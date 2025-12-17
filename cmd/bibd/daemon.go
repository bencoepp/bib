// Package main provides the bibd daemon.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"bib/internal/cluster"
	"bib/internal/config"
	"bib/internal/logger"
	"bib/internal/p2p"
	"bib/internal/storage"
	pglifecycle "bib/internal/storage/postgres/lifecycle"

	// Import storage backends to register factories
	_ "bib/internal/storage/postgres"
	_ "bib/internal/storage/sqlite"
)

// Daemon manages all bibd components and their lifecycle.
type Daemon struct {
	cfg       *config.BibdConfig
	configDir string
	log       *logger.Logger
	auditLog  *logger.AuditLogger

	store       storage.Store
	pgLifecycle *pglifecycle.Manager // PostgreSQL lifecycle manager (nil if not using managed postgres)
	p2pHost     *p2p.Host
	p2pDisc     *p2p.Discovery
	p2pMode     *p2p.ModeManager
	cluster     *cluster.Cluster

	mu      sync.Mutex
	running bool
}

// NewDaemon creates a new daemon instance.
func NewDaemon(cfg *config.BibdConfig, configDir string, log *logger.Logger, auditLog *logger.AuditLogger) *Daemon {
	// Set logger for all components
	storage.SetLogger(log)
	p2p.SetLogger(log)
	cluster.SetLogger(log)

	return &Daemon{
		cfg:       cfg,
		configDir: configDir,
		log:       log,
		auditLog:  auditLog,
	}
}

// Start initializes and starts all daemon components in the correct order.
// Order: Storage -> P2P -> Cluster
func (d *Daemon) Start(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running {
		return fmt.Errorf("daemon already running")
	}

	d.log.Info("starting daemon components")

	// 1. Write PID file
	if err := d.writePIDFile(); err != nil {
		d.log.Warn("failed to write PID file", "error", err, "path", d.cfg.Server.PIDFile)
		// Non-fatal, continue
	}

	// 2. Initialize storage
	if err := d.startStorage(ctx); err != nil {
		return fmt.Errorf("failed to start storage: %w", err)
	}

	// 3. Initialize P2P networking
	if d.cfg.P2P.Enabled {
		if err := d.startP2P(ctx); err != nil {
			d.stopStorage()
			return fmt.Errorf("failed to start P2P: %w", err)
		}
	}

	// 4. Initialize cluster (requires P2P for DHT discovery)
	if d.cfg.Cluster.Enabled {
		if err := d.startCluster(ctx); err != nil {
			d.stopP2P()
			d.stopStorage()
			return fmt.Errorf("failed to start cluster: %w", err)
		}
	}

	// 5. Wait for storage to be ready (includes waiting for managed PostgreSQL)
	if err := d.waitForStorageReady(ctx); err != nil {
		d.stopCluster()
		d.stopP2P()
		d.stopStorage()
		return fmt.Errorf("storage failed to become ready: %w", err)
	}

	d.running = true
	d.log.Info("daemon started successfully")

	return nil
}

// Stop gracefully shuts down all daemon components in reverse order.
// Order: Cluster -> P2P -> Storage
func (d *Daemon) Stop(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return nil
	}

	d.log.Info("stopping daemon components")

	var errs []error

	// 1. Stop cluster first
	if err := d.stopCluster(); err != nil {
		errs = append(errs, fmt.Errorf("cluster: %w", err))
	}

	// 2. Stop P2P
	if err := d.stopP2P(); err != nil {
		errs = append(errs, fmt.Errorf("p2p: %w", err))
	}

	// 3. Stop storage last
	if err := d.stopStorage(); err != nil {
		errs = append(errs, fmt.Errorf("storage: %w", err))
	}

	// 4. Remove PID file
	if err := d.removePIDFile(); err != nil {
		d.log.Warn("failed to remove PID file", "error", err)
	}

	d.running = false

	if len(errs) > 0 {
		d.log.Error("daemon stopped with errors", "errors", errs)
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	d.log.Info("daemon stopped successfully")
	return nil
}

// startStorage initializes the storage backend asynchronously.
// For managed PostgreSQL, this starts the container but does NOT wait for it to be ready.
// Readiness is checked later by waitForStorageReady() before the node becomes ready.
func (d *Daemon) startStorage(ctx context.Context) error {
	d.log.Debug("initializing storage",
		"backend", d.cfg.Database.Backend,
		"data_dir", d.cfg.Server.DataDir,
	)

	// Convert config types
	storageCfg := d.convertStorageConfig()

	// Validate configuration early (fail fast)
	if err := storageCfg.Validate(); err != nil {
		d.log.Error("invalid storage configuration", "error", err)
		return fmt.Errorf("invalid storage configuration: %w", err)
	}

	// Determine node ID for storage
	nodeID := d.cfg.Cluster.NodeID
	if nodeID == "" {
		nodeID = "standalone"
	}

	// Validate mode/backend compatibility early (fail fast)
	if err := storage.ValidateModeBackend(d.cfg.P2P.Mode, storageCfg.Backend); err != nil {
		d.log.Error("incompatible mode and backend", "error", err, "mode", d.cfg.P2P.Mode, "backend", storageCfg.Backend)
		return fmt.Errorf("incompatible mode and backend: %w", err)
	}

	// Handle managed PostgreSQL lifecycle
	if storageCfg.Backend == storage.BackendPostgres && storageCfg.Postgres.Managed {
		if err := d.startManagedPostgres(ctx, storageCfg); err != nil {
			return err
		}
		// Don't wait here - we'll wait in waitForStorageReady()
		d.log.Info("managed PostgreSQL lifecycle started (waiting for readiness)")
		return nil
	}

	// For non-managed backends (SQLite or external Postgres), open immediately
	store, err := storage.Open(ctx, storageCfg, d.cfg.Server.DataDir, nodeID, d.cfg.P2P.Mode)
	if err != nil {
		d.log.Error("failed to open storage", "error", err)
		return fmt.Errorf("failed to open storage: %w", err)
	}

	// Initial ping to verify connectivity (fail fast)
	if err := store.Ping(ctx); err != nil {
		store.Close()
		d.log.Error("storage ping failed", "error", err)
		return fmt.Errorf("storage ping failed: %w", err)
	}

	d.store = store

	d.log.Info("storage initialized",
		"backend", store.Backend(),
		"authoritative", store.IsAuthoritative(),
	)

	return nil
}

// startManagedPostgres initializes and starts a managed PostgreSQL instance.
// This starts the container/cluster but does NOT wait for readiness.
func (d *Daemon) startManagedPostgres(ctx context.Context, storageCfg storage.Config) error {
	d.log.Debug("starting managed PostgreSQL lifecycle")

	// Create lifecycle config from storage config
	lifecycleCfg := d.convertToLifecycleConfig(storageCfg.Postgres)

	// Determine node ID
	nodeID := d.cfg.Cluster.NodeID
	if nodeID == "" {
		nodeID = "standalone"
	}

	// Create lifecycle manager
	mgr, err := pglifecycle.NewManager(lifecycleCfg, nodeID, d.cfg.Server.DataDir)
	if err != nil {
		d.log.Error("failed to create PostgreSQL lifecycle manager", "error", err)
		return fmt.Errorf("failed to create PostgreSQL lifecycle manager: %w", err)
	}

	// Validate detected runtime
	runtime := mgr.Runtime()
	d.log.Debug("detected container runtime", "runtime", runtime)

	// Start PostgreSQL (asynchronous - container/pod starts but we don't wait)
	if err := mgr.Start(ctx); err != nil {
		// If Kubernetes fails, check if we can fallback to container runtime
		if runtime == "kubernetes" {
			d.log.Warn("Kubernetes deployment failed, checking for container runtime fallback", "error", err)

			// Try to detect Docker or Podman as fallback
			fallbackMgr, fallbackErr := pglifecycle.NewManager(lifecycleCfg, nodeID, d.cfg.Server.DataDir)
			if fallbackErr == nil && (fallbackMgr.Runtime() == "docker" || fallbackMgr.Runtime() == "podman") {
				d.log.Info("falling back to container runtime", "runtime", fallbackMgr.Runtime())
				if startErr := fallbackMgr.Start(ctx); startErr == nil {
					d.pgLifecycle = fallbackMgr
					d.log.Info("managed PostgreSQL started with fallback",
						"runtime", fallbackMgr.Runtime(),
						"container", lifecycleCfg.ContainerName,
					)
					return nil
				}
			}
		}

		d.log.Error("failed to start managed PostgreSQL", "error", err)
		return fmt.Errorf("failed to start managed PostgreSQL: %w", err)
	}

	d.pgLifecycle = mgr
	d.log.Info("managed PostgreSQL started",
		"runtime", runtime,
		"identifier", lifecycleCfg.ContainerName,
	)

	return nil
}

// waitForStorageReady waits for storage to become healthy before the node becomes ready.
// This is called after all components are started but before marking the node as ready.
func (d *Daemon) waitForStorageReady(ctx context.Context) error {
	d.log.Debug("waiting for storage to become ready")

	// If we have a PostgreSQL lifecycle manager, wait for it to be ready
	if d.pgLifecycle != nil {
		d.log.Debug("waiting for managed PostgreSQL to be ready")

		// The lifecycle manager's Start() already waits, but we check again for safety
		if !d.pgLifecycle.IsReady() {
			return fmt.Errorf("managed PostgreSQL is not ready")
		}

		// Now that PostgreSQL is ready, connect to it
		storageCfg := d.convertStorageConfig()
		nodeID := d.cfg.Cluster.NodeID
		if nodeID == "" {
			nodeID = "standalone"
		}

		// Open the store with the managed connection
		store, err := d.openManagedPostgresStore(ctx, storageCfg.Postgres, nodeID)
		if err != nil {
			return fmt.Errorf("failed to connect to managed PostgreSQL: %w", err)
		}

		d.store = store
		d.log.Info("connected to managed PostgreSQL",
			"backend", store.Backend(),
			"authoritative", store.IsAuthoritative(),
		)
	}

	// Verify storage is responding
	if d.store == nil {
		return fmt.Errorf("storage is not initialized")
	}

	// Final health check with ping
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := d.store.Ping(pingCtx); err != nil {
		d.log.Error("storage health check failed", "error", err)
		return fmt.Errorf("storage health check failed: %w", err)
	}

	d.log.Info("storage is ready and healthy", "backend", d.store.Backend())
	return nil
}

// openManagedPostgresStore opens a PostgreSQL store connected to a managed instance.
func (d *Daemon) openManagedPostgresStore(ctx context.Context, pgCfg storage.PostgresConfig, nodeID string) (storage.Store, error) {
	d.log.Debug("connecting to managed PostgreSQL", "node_id", nodeID)

	// Use the storage.Open with the modified config
	// We need to construct an AdvancedPostgresConfig from the connection string
	modifiedCfg := storage.Config{
		Backend:  storage.BackendPostgres,
		Postgres: pgCfg,
	}

	// For managed PostgreSQL, we use Advanced config to connect
	// Set Managed to false to pass validation (we're just connecting, not managing)
	modifiedCfg.Postgres.Managed = false

	// Parse connection string to populate Advanced config
	// Format: "host=X port=Y user=Z password=W dbname=D sslmode=S"
	// For simplicity, we'll pass the connection components
	if modifiedCfg.Postgres.Advanced == nil {
		modifiedCfg.Postgres.Advanced = &storage.AdvancedPostgresConfig{}
	}

	// Extract components from lifecycle manager

	// For Kubernetes, get connection info from the manager
	runtime := d.pgLifecycle.Runtime()
	if runtime == "kubernetes" {
		// Use the connection string method which handles Kubernetes properly
		connStr := d.pgLifecycle.ConnectionString()
		d.log.Debug("using Kubernetes connection", "connection_string_format", "parsed")

		// Parse the connection string to populate Advanced config
		// This is a simple parser - in production you might want to use url.Parse
		parts := make(map[string]string)
		for _, part := range splitConnectionString(connStr) {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				parts[kv[0]] = kv[1]
			}
		}

		if host, ok := parts["host"]; ok {
			modifiedCfg.Postgres.Advanced.Host = host
		}
		if port, ok := parts["port"]; ok {
			if p, err := strconv.Atoi(port); err == nil {
				modifiedCfg.Postgres.Advanced.Port = p
			}
		}
		if user, ok := parts["user"]; ok {
			modifiedCfg.Postgres.Advanced.User = user
		}
		if password, ok := parts["password"]; ok {
			modifiedCfg.Postgres.Advanced.Password = password
		}
		if dbname, ok := parts["dbname"]; ok {
			modifiedCfg.Postgres.Advanced.Database = dbname
		}
		if sslmode, ok := parts["sslmode"]; ok {
			modifiedCfg.Postgres.Advanced.SSLMode = sslmode
		}
	} else {
		// For Docker/Podman, parse the connection string from the lifecycle manager
		// The lifecycle manager has already applied Windows-specific fixes (forcing TCP on Windows)
		connStr := d.pgLifecycle.ConnectionString()
		d.log.Debug("using Docker/Podman connection", "connection_string_format", "parsed")

		// Parse the connection string to populate Advanced config
		parts := make(map[string]string)
		for _, part := range splitConnectionString(connStr) {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				parts[kv[0]] = kv[1]
			}
		}

		if host, ok := parts["host"]; ok {
			modifiedCfg.Postgres.Advanced.Host = host
		}
		if port, ok := parts["port"]; ok {
			if p, err := strconv.Atoi(port); err == nil {
				modifiedCfg.Postgres.Advanced.Port = p
			}
		}
		if user, ok := parts["user"]; ok {
			modifiedCfg.Postgres.Advanced.User = user
		}
		if password, ok := parts["password"]; ok {
			modifiedCfg.Postgres.Advanced.Password = password
		}
		if dbname, ok := parts["dbname"]; ok {
			modifiedCfg.Postgres.Advanced.Database = dbname
		}
		if sslmode, ok := parts["sslmode"]; ok {
			modifiedCfg.Postgres.Advanced.SSLMode = sslmode
		}
	}

	// Open with the advanced config
	store, err := storage.Open(ctx, modifiedCfg, d.cfg.Server.DataDir, nodeID, d.cfg.P2P.Mode)
	if err != nil {
		return nil, err
	}

	return store, nil
}

// splitConnectionString splits a PostgreSQL connection string into parts.
func splitConnectionString(connStr string) []string {
	// Simple space-based split, respecting quoted values
	var parts []string
	var current strings.Builder
	inQuote := false

	for _, ch := range connStr {
		switch ch {
		case '\'':
			inQuote = !inQuote
			current.WriteRune(ch)
		case ' ':
			if inQuote {
				current.WriteRune(ch)
			} else if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// stopStorage shuts down the storage backend and lifecycle manager.
func (d *Daemon) stopStorage() error {
	var errs []error

	// Close the store connection first
	if d.store != nil {
		d.log.Debug("closing storage connection")
		if err := d.store.Close(); err != nil {
			d.log.Error("error closing storage", "error", err)
			errs = append(errs, fmt.Errorf("close store: %w", err))
		}
		d.store = nil
	}

	// Stop PostgreSQL lifecycle manager if present
	if d.pgLifecycle != nil {
		d.log.Debug("stopping managed PostgreSQL")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := d.pgLifecycle.Stop(ctx); err != nil {
			d.log.Error("error stopping managed PostgreSQL", "error", err)
			errs = append(errs, fmt.Errorf("stop lifecycle: %w", err))
		}
		d.pgLifecycle = nil
	}

	if len(errs) > 0 {
		d.log.Debug("storage shutdown completed with errors")
		return fmt.Errorf("storage shutdown errors: %v", errs)
	}

	d.log.Debug("storage shut down successfully")
	return nil
}

// startP2P initializes P2P networking components.
func (d *Daemon) startP2P(ctx context.Context) error {
	d.log.Debug("initializing P2P networking",
		"mode", d.cfg.P2P.Mode,
		"listen_addresses", d.cfg.P2P.ListenAddresses,
	)

	// Create P2P host
	host, err := p2p.NewHost(ctx, d.cfg.P2P, d.configDir)
	if err != nil {
		d.log.Error("failed to create P2P host", "error", err)
		return err
	}
	d.p2pHost = host

	d.log.Info("P2P host created",
		"peer_id", host.PeerID().String(),
		"listen_addrs", host.ListenAddrs(),
	)

	// Create discovery manager
	discovery, err := p2p.NewDiscovery(host.Host, d.cfg.P2P, d.configDir)
	if err != nil {
		host.Close()
		d.log.Error("failed to create P2P discovery", "error", err)
		return err
	}
	d.p2pDisc = discovery

	// Start discovery
	if err := discovery.Start(ctx); err != nil {
		discovery.Stop()
		host.Close()
		d.log.Error("failed to start P2P discovery", "error", err)
		return err
	}

	d.log.Debug("P2P discovery started",
		"mdns_enabled", d.cfg.P2P.MDNS.Enabled,
		"dht_enabled", d.cfg.P2P.DHT.Enabled,
		"bootstrap_peers", len(d.cfg.P2P.Bootstrap.Peers),
	)

	// Create and start mode manager
	modeManager, err := p2p.NewModeManager(host.Host, discovery, d.cfg.P2P, d.configDir)
	if err != nil {
		discovery.Stop()
		host.Close()
		d.log.Error("failed to create mode manager", "error", err)
		return err
	}
	d.p2pMode = modeManager

	if err := modeManager.Start(ctx); err != nil {
		discovery.Stop()
		host.Close()
		d.log.Error("failed to start mode manager", "error", err)
		return err
	}

	d.log.Info("P2P networking initialized",
		"mode", d.cfg.P2P.Mode,
		"peer_id", host.PeerID().String(),
	)

	// Log full addresses for debugging
	for _, addr := range host.FullAddrs() {
		d.log.Debug("P2P listening", "addr", addr.String())
	}

	return nil
}

// stopP2P shuts down P2P networking.
func (d *Daemon) stopP2P() error {
	var errs []error

	if d.p2pMode != nil {
		d.log.Debug("stopping P2P mode manager")
		if err := d.p2pMode.Stop(); err != nil {
			d.log.Error("error stopping mode manager", "error", err)
			errs = append(errs, err)
		}
		d.p2pMode = nil
	}

	if d.p2pDisc != nil {
		d.log.Debug("stopping P2P discovery")
		if err := d.p2pDisc.Stop(); err != nil {
			d.log.Error("error stopping discovery", "error", err)
			errs = append(errs, err)
		}
		d.p2pDisc = nil
	}

	if d.p2pHost != nil {
		d.log.Debug("stopping P2P host")
		if err := d.p2pHost.Close(); err != nil {
			d.log.Error("error closing P2P host", "error", err)
			errs = append(errs, err)
		}
		d.p2pHost = nil
	}

	d.log.Debug("P2P networking shut down")

	if len(errs) > 0 {
		return fmt.Errorf("P2P shutdown errors: %v", errs)
	}
	return nil
}

// startCluster initializes the Raft cluster.
func (d *Daemon) startCluster(ctx context.Context) error {
	d.log.Debug("initializing cluster",
		"cluster_name", d.cfg.Cluster.ClusterName,
		"is_voter", d.cfg.Cluster.IsVoter,
		"bootstrap", d.cfg.Cluster.Bootstrap,
	)

	clusterInstance, err := cluster.New(d.cfg.Cluster, d.configDir)
	if err != nil {
		d.log.Error("failed to create cluster", "error", err)
		return err
	}

	if clusterInstance == nil {
		// Cluster not enabled (already checked above, but being defensive)
		return nil
	}

	if err := clusterInstance.Start(ctx); err != nil {
		d.log.Error("failed to start cluster", "error", err)
		return err
	}
	d.cluster = clusterInstance

	// Set up callbacks for cluster events
	clusterInstance.OnLeaderChange(func(leaderID string) {
		d.log.Info("cluster leader changed", "leader", leaderID)
	})

	clusterInstance.OnMemberChange(func(members []cluster.ClusterMember) {
		d.log.Info("cluster membership changed", "member_count", len(members))
		for _, m := range members {
			d.log.Debug("cluster member",
				"node_id", m.NodeID,
				"role", m.Role,
				"state", m.State,
				"healthy", m.IsHealthy,
			)
		}
	})

	d.log.Info("cluster initialized",
		"node_id", clusterInstance.NodeID(),
		"state", clusterInstance.State(),
	)

	return nil
}

// stopCluster shuts down the Raft cluster.
func (d *Daemon) stopCluster() error {
	if d.cluster == nil {
		return nil
	}

	d.log.Debug("shutting down cluster")

	if err := d.cluster.Stop(); err != nil {
		d.log.Error("error stopping cluster", "error", err)
		return err
	}

	d.cluster = nil
	d.log.Debug("cluster shut down")
	return nil
}

// convertStorageConfig converts the bibd config to storage config.
func (d *Daemon) convertStorageConfig() storage.Config {
	cfg := storage.Config{
		Backend: storage.BackendType(d.cfg.Database.Backend),
		SQLite: storage.SQLiteConfig{
			Path:         d.cfg.Database.SQLite.Path,
			MaxOpenConns: d.cfg.Database.SQLite.MaxOpenConns,
		},
		Postgres: storage.PostgresConfig{
			Managed:                    d.cfg.Database.Postgres.Managed,
			ContainerRuntime:           d.cfg.Database.Postgres.ContainerRuntime,
			SocketPath:                 d.cfg.Database.Postgres.SocketPath,
			KubeconfigPath:             d.cfg.Database.Postgres.KubeconfigPath,
			Image:                      d.cfg.Database.Postgres.Image,
			DataDir:                    d.cfg.Database.Postgres.DataDir,
			Port:                       d.cfg.Database.Postgres.Port,
			MaxConnections:             d.cfg.Database.Postgres.MaxConnections,
			SSLMode:                    d.cfg.Database.Postgres.SSLMode,
			CredentialRotationInterval: d.cfg.Database.Postgres.CredentialRotationInterval,
			Resources: storage.ContainerResources{
				MemoryMB: d.cfg.Database.Postgres.MemoryMB,
				CPUCores: d.cfg.Database.Postgres.CPUCores,
			},
			Network: storage.NetworkConfig{
				UseBridgeNetwork:  d.cfg.Database.Postgres.Network.UseBridgeNetwork,
				BridgeNetworkName: d.cfg.Database.Postgres.Network.BridgeNetworkName,
				UseUnixSocket:     d.cfg.Database.Postgres.Network.UseUnixSocket,
				BindAddress:       d.cfg.Database.Postgres.Network.BindAddress,
			},
			Health: storage.HealthConfig{
				Interval:       d.cfg.Database.Postgres.Health.Interval,
				Timeout:        d.cfg.Database.Postgres.Health.Timeout,
				StartupTimeout: d.cfg.Database.Postgres.Health.StartupTimeout,
				Action:         d.cfg.Database.Postgres.Health.Action,
				MaxRetries:     d.cfg.Database.Postgres.Health.MaxRetries,
				RetryBackoff:   d.cfg.Database.Postgres.Health.RetryBackoff,
			},
			TLS: storage.TLSConfig{
				Enabled:      d.cfg.Database.Postgres.TLS.Enabled,
				CertDir:      d.cfg.Database.Postgres.TLS.CertDir,
				AutoGenerate: d.cfg.Database.Postgres.TLS.AutoGenerate,
			},
		},
		Audit: storage.AuditConfig{
			Enabled:       d.cfg.Database.Audit.Enabled,
			RetentionDays: d.cfg.Database.Audit.RetentionDays,
			HashChain:     d.cfg.Database.Audit.HashChain,
		},
	}

	// Handle advanced postgres config if present
	if d.cfg.Database.Postgres.Advanced != nil {
		cfg.Postgres.Advanced = &storage.AdvancedPostgresConfig{
			Host:     d.cfg.Database.Postgres.Advanced.Host,
			Port:     d.cfg.Database.Postgres.Advanced.Port,
			Database: d.cfg.Database.Postgres.Advanced.Database,
			User:     d.cfg.Database.Postgres.Advanced.User,
			Password: d.cfg.Database.Postgres.Advanced.Password,
			SSLMode:  d.cfg.Database.Postgres.Advanced.SSLMode,
		}
	}

	return cfg
}

// convertToLifecycleConfig converts PostgreSQL config to lifecycle manager config.
func (d *Daemon) convertToLifecycleConfig(pgCfg storage.PostgresConfig) pglifecycle.LifecycleConfig {
	lifecycleCfg := pglifecycle.DefaultLifecycleConfig()

	// Override defaults with user configuration
	if pgCfg.ContainerRuntime != "" {
		lifecycleCfg.Runtime = pglifecycle.RuntimeType(pgCfg.ContainerRuntime)
	}
	if pgCfg.SocketPath != "" {
		lifecycleCfg.SocketPath = pgCfg.SocketPath
	}
	if pgCfg.KubeconfigPath != "" {
		lifecycleCfg.KubeconfigPath = pgCfg.KubeconfigPath
	}
	if pgCfg.Image != "" {
		lifecycleCfg.Image = pgCfg.Image
	}
	if pgCfg.DataDir != "" {
		lifecycleCfg.DataDir = pgCfg.DataDir
	}
	if pgCfg.Port != 0 {
		lifecycleCfg.Port = pgCfg.Port
	}

	// Network configuration
	lifecycleCfg.Network = pglifecycle.NetworkConfig{
		UseBridgeNetwork:  pgCfg.Network.UseBridgeNetwork,
		BridgeNetworkName: pgCfg.Network.BridgeNetworkName,
		UseUnixSocket:     pgCfg.Network.UseUnixSocket,
		BindAddress:       pgCfg.Network.BindAddress,
	}

	// Resource limits
	lifecycleCfg.Resources = pglifecycle.ResourceConfig{
		MemoryMB: pgCfg.Resources.MemoryMB,
		CPUCores: pgCfg.Resources.CPUCores,
	}

	// Health check configuration
	if pgCfg.Health.Interval > 0 {
		lifecycleCfg.Health.Interval = pgCfg.Health.Interval
	}
	if pgCfg.Health.Timeout > 0 {
		lifecycleCfg.Health.Timeout = pgCfg.Health.Timeout
	}
	if pgCfg.Health.StartupTimeout > 0 {
		lifecycleCfg.Health.StartupTimeout = pgCfg.Health.StartupTimeout
	}
	if pgCfg.Health.Action != "" {
		lifecycleCfg.Health.Action = pglifecycle.HealthAction(pgCfg.Health.Action)
	}
	if pgCfg.Health.MaxRetries > 0 {
		lifecycleCfg.Health.MaxRetries = pgCfg.Health.MaxRetries
	}
	if pgCfg.Health.RetryBackoff > 0 {
		lifecycleCfg.Health.RetryBackoff = pgCfg.Health.RetryBackoff
	}

	// TLS configuration
	lifecycleCfg.TLS = pglifecycle.TLSConfig{
		Enabled:      pgCfg.TLS.Enabled,
		CertDir:      pgCfg.TLS.CertDir,
		AutoGenerate: pgCfg.TLS.AutoGenerate,
	}

	// Credentials configuration
	if pgCfg.CredentialRotationInterval > 0 {
		lifecycleCfg.Credentials.RotationInterval = pgCfg.CredentialRotationInterval
	}

	// Kubernetes configuration
	lifecycleCfg.Kubernetes = pgCfg.Kubernetes

	return lifecycleCfg
}

// writePIDFile writes the daemon's PID to a file.
func (d *Daemon) writePIDFile() error {
	if d.cfg.Server.PIDFile == "" {
		return nil
	}

	// Expand the path
	pidFile := d.cfg.Server.PIDFile
	if pidFile[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to expand home dir: %w", err)
		}
		pidFile = filepath.Join(home, pidFile[1:])
	}

	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(pidFile), 0755); err != nil {
		return fmt.Errorf("failed to create PID file directory: %w", err)
	}

	// Write PID
	pid := os.Getpid()
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	d.log.Debug("wrote PID file", "path", pidFile, "pid", pid)
	return nil
}

// removePIDFile removes the PID file.
func (d *Daemon) removePIDFile() error {
	if d.cfg.Server.PIDFile == "" {
		return nil
	}

	pidFile := d.cfg.Server.PIDFile
	if pidFile[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		pidFile = filepath.Join(home, pidFile[1:])
	}

	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// Store returns the storage instance for use by other components.
func (d *Daemon) Store() storage.Store {
	return d.store
}

// P2PHost returns the P2P host for use by other components.
func (d *Daemon) P2PHost() *p2p.Host {
	return d.p2pHost
}

// Cluster returns the cluster instance for use by other components.
func (d *Daemon) Cluster() *cluster.Cluster {
	return d.cluster
}

// IsRunning returns true if the daemon is running.
func (d *Daemon) IsRunning() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.running
}
