// Package main provides the bibd daemon.
package main

import (
	"bib/internal/certs"
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"bib/internal/cluster"
	"bib/internal/config"
	grpcpkg "bib/internal/grpc"
	"bib/internal/grpc/interfaces"
	"bib/internal/grpc/middleware"
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
	p2pIdentity *p2p.Identity        // P2P identity (also used for CA key encryption)
	p2pHost     *p2p.Host
	p2pDisc     *p2p.Discovery
	p2pMode     *p2p.ModeManager
	cluster     *cluster.Cluster
	certMgr     *certs.Manager  // TLS certificate manager
	grpcServer  *grpcpkg.Server // gRPC server

	mu        sync.Mutex
	running   bool
	startedAt time.Time
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
// Order: Identity -> Certificates -> Storage -> P2P -> Cluster
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

	// 2. Load or generate P2P identity (needed for certificate encryption)
	if err := d.loadIdentity(); err != nil {
		return fmt.Errorf("failed to load identity: %w", err)
	}

	// 3. Initialize TLS certificates (blocks until ready if auto-generate is enabled)
	if d.cfg.Server.TLS.Enabled || d.cfg.Server.TLS.AutoGenerate {
		if err := d.initCertificates(); err != nil {
			return fmt.Errorf("failed to initialize certificates: %w", err)
		}
	}

	// 4. Initialize storage
	if err := d.startStorage(ctx); err != nil {
		d.stopCertificates()
		return fmt.Errorf("failed to start storage: %w", err)
	}

	// 5. Initialize P2P networking
	if d.cfg.P2P.Enabled {
		if err := d.startP2P(ctx); err != nil {
			d.stopStorage()
			d.stopCertificates()
			return fmt.Errorf("failed to start P2P: %w", err)
		}
	}

	// 6. Initialize cluster (requires P2P for DHT discovery)
	if d.cfg.Cluster.Enabled {
		if err := d.startCluster(ctx); err != nil {
			d.stopP2P()
			d.stopStorage()
			d.stopCertificates()
			return fmt.Errorf("failed to start cluster: %w", err)
		}
	}

	// 7. Wait for storage to be ready (includes waiting for managed PostgreSQL)
	if err := d.waitForStorageReady(ctx); err != nil {
		d.stopCluster()
		d.stopP2P()
		d.stopStorage()
		d.stopCertificates()
		return fmt.Errorf("storage failed to become ready: %w", err)
	}

	// 8. Initialize gRPC server (after storage and P2P are ready)
	if d.cfg.Server.GRPC.Enabled {
		if err := d.startGRPCServer(ctx); err != nil {
			d.stopCluster()
			d.stopP2P()
			d.stopStorage()
			d.stopCertificates()
			return fmt.Errorf("failed to start gRPC server: %w", err)
		}
	}

	d.running = true
	d.startedAt = time.Now()
	d.log.Info("daemon started successfully")

	return nil
}

// Stop gracefully shuts down all daemon components in reverse order.
// Order: gRPC -> Cluster -> P2P -> Storage -> Certificates
func (d *Daemon) Stop(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return nil
	}

	d.log.Info("stopping daemon components")

	var errs []error

	// 1. Stop gRPC server first (drain connections)
	if err := d.stopGRPCServer(ctx); err != nil {
		errs = append(errs, fmt.Errorf("grpc: %w", err))
	}

	// 2. Stop cluster
	if err := d.stopCluster(); err != nil {
		errs = append(errs, fmt.Errorf("cluster: %w", err))
	}

	// 3. Stop P2P
	if err := d.stopP2P(); err != nil {
		errs = append(errs, fmt.Errorf("p2p: %w", err))
	}

	// 4. Stop storage
	if err := d.stopStorage(); err != nil {
		errs = append(errs, fmt.Errorf("storage: %w", err))
	}

	// 5. Stop certificate manager
	if err := d.stopCertificates(); err != nil {
		errs = append(errs, fmt.Errorf("certs: %w", err))
	}

	// 6. Remove PID file
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

// loadIdentity loads or generates the P2P identity.
// This is needed early for certificate key encryption.
func (d *Daemon) loadIdentity() error {
	d.log.Debug("loading P2P identity")

	identity, err := p2p.LoadOrGenerateIdentity(d.cfg.P2P.Identity.KeyPath, d.configDir)
	if err != nil {
		return fmt.Errorf("failed to load or generate identity: %w", err)
	}

	d.p2pIdentity = identity
	d.log.Info("P2P identity loaded", "key_path", p2p.KeyPath(d.cfg.P2P.Identity.KeyPath, d.configDir))

	return nil
}

// initCertificates initializes the TLS certificate infrastructure.
// This blocks until certificates are ready (generates CA/server cert on first run).
func (d *Daemon) initCertificates() error {
	d.log.Debug("initializing TLS certificates")

	// Get the raw identity key for CA key encryption
	identityKey, err := d.p2pIdentity.RawPrivateKey()
	if err != nil {
		return fmt.Errorf("failed to get identity key: %w", err)
	}

	// Determine node ID and peer ID
	nodeID := d.cfg.Cluster.NodeID
	if nodeID == "" {
		nodeID = "standalone"
	}

	var peerID string
	if d.p2pIdentity != nil {
		// Get peer ID from identity
		pid, err := d.p2pIdentity.PrivKey.GetPublic().Raw()
		if err == nil {
			peerID = fmt.Sprintf("%x", pid[:8]) // Short form
		}
	}

	// Create certificate manager config
	certCfg := certs.ManagerConfig{
		ConfigDir:              d.configDir,
		NodeID:                 nodeID,
		PeerID:                 peerID,
		P2PIdentityKey:         identityKey,
		ListenAddresses:        d.cfg.P2P.ListenAddresses,
		CAValidityYears:        d.cfg.Server.TLS.CAValidityYears,
		ServerCertValidityDays: d.cfg.Server.TLS.ServerCertValidityDays,
		ClientCertValidityDays: d.cfg.Server.TLS.ClientCertValidityDays,
		RenewalThresholdDays:   d.cfg.Server.TLS.RenewalThresholdDays,
	}

	// Create certificate manager
	certMgr, err := certs.NewManager(certCfg)
	if err != nil {
		return fmt.Errorf("failed to create certificate manager: %w", err)
	}

	// Initialize (generates CA/certs if needed, blocks until ready)
	if err := certMgr.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize certificates: %w", err)
	}

	d.certMgr = certMgr

	d.log.Info("TLS certificates initialized",
		"ca_fingerprint", certMgr.CAFingerprint()[:16]+"...",
		"server_fingerprint", certMgr.ServerFingerprint()[:16]+"...",
	)

	return nil
}

// stopCertificates cleans up the certificate manager.
func (d *Daemon) stopCertificates() error {
	if d.certMgr != nil {
		if err := d.certMgr.Close(); err != nil {
			d.log.Error("error closing certificate manager", "error", err)
			return err
		}
		d.certMgr = nil
	}
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

// stopStorage shuts down the storage backend and lifecycle manager gracefully.
func (d *Daemon) stopStorage() error {
	var errs []error

	// 1. Drain active connections (if store supports it)
	if d.store != nil {
		d.log.Debug("draining storage connections")
		// TODO: Implement connection draining in store interface
		// For now, we'll give a brief grace period for active operations
		time.Sleep(1 * time.Second)
	}

	// 2. Close the store connection (this completes in-flight transactions)
	if d.store != nil {
		d.log.Debug("closing storage connection")
		if err := d.store.Close(); err != nil {
			d.log.Error("error closing storage", "error", err)
			errs = append(errs, fmt.Errorf("close store: %w", err))
		} else {
			d.log.Info("storage connection closed cleanly")
		}
		d.store = nil
	}

	// 3. Gracefully stop PostgreSQL lifecycle manager if present
	if d.pgLifecycle != nil {
		d.log.Debug("stopping managed PostgreSQL gracefully")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Perform checkpoint before shutdown (PostgreSQL specific)
		d.log.Debug("requesting PostgreSQL checkpoint")
		// The lifecycle manager's Stop will trigger a CHECKPOINT

		if err := d.pgLifecycle.Stop(ctx); err != nil {
			d.log.Error("error stopping managed PostgreSQL", "error", err)
			errs = append(errs, fmt.Errorf("stop lifecycle: %w", err))
		} else {
			d.log.Info("managed PostgreSQL stopped cleanly")
		}
		d.pgLifecycle = nil
	}

	if len(errs) > 0 {
		d.log.Warn("storage shutdown completed with errors", "error_count", len(errs))
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

// startGRPCServer initializes and starts the gRPC server.
func (d *Daemon) startGRPCServer(ctx context.Context) error {
	d.log.Debug("initializing gRPC server",
		"host", d.cfg.Server.GRPC.Host,
		"port", d.cfg.Server.GRPC.Port,
		"unix_socket", d.cfg.Server.GRPC.UnixSocket,
	)

	// Get TLS config from certificate manager
	var tlsConfig *tls.Config
	if d.certMgr != nil {
		tlsConfig = d.certMgr.TLSConfig()
	}

	// Create gRPC server configuration
	serverCfg := grpcpkg.ServerConfig{
		GRPCConfig:     d.cfg.Server.GRPC,
		ServerHost:     d.cfg.Server.Host,
		TLSConfig:      tlsConfig,
		HealthProvider: d, // Daemon implements HealthProvider
	}

	// Create audit middleware if audit logging is enabled
	if d.cfg.Database.Audit.Enabled && d.store != nil {
		auditRepo := d.store.Audit()
		if auditRepo != nil {
			serverCfg.AuditMiddleware = middleware.NewAuditMiddleware(auditRepo, middleware.AuditConfig{
				Enabled:             true,
				LogFailedOperations: true,
				NodeID:              d.NodeID(),
			})
		}
	}

	// Create the server
	server, err := grpcpkg.NewServer(serverCfg)
	if err != nil {
		d.log.Error("failed to create gRPC server", "error", err)
		return fmt.Errorf("failed to create gRPC server: %w", err)
	}

	// Start the server
	if err := server.Start(ctx); err != nil {
		d.log.Error("failed to start gRPC server", "error", err)
		return fmt.Errorf("failed to start gRPC server: %w", err)
	}

	d.grpcServer = server

	d.log.Info("gRPC server started",
		"address", server.Address(),
		"metrics_enabled", d.cfg.Server.GRPC.Metrics.Enabled,
	)

	return nil
}

// stopGRPCServer shuts down the gRPC server gracefully.
func (d *Daemon) stopGRPCServer(ctx context.Context) error {
	if d.grpcServer == nil {
		return nil
	}

	d.log.Debug("stopping gRPC server")

	if err := d.grpcServer.Stop(ctx); err != nil {
		d.log.Error("error stopping gRPC server", "error", err)
		return err
	}

	d.grpcServer = nil
	d.log.Debug("gRPC server shut down")
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

// P2PDiscovery returns the P2P discovery manager.
func (d *Daemon) P2PDiscovery() *p2p.Discovery {
	return d.p2pDisc
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

// StartedAt returns when the daemon started.
func (d *Daemon) StartedAt() time.Time {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.startedAt
}

// NodeMode returns the P2P mode.
func (d *Daemon) NodeMode() string {
	return d.cfg.P2P.Mode
}

// NodeID returns the node identifier.
func (d *Daemon) NodeID() string {
	// Prefer cluster node ID if available
	if d.cfg.Cluster.NodeID != "" {
		return d.cfg.Cluster.NodeID
	}
	// Fall back to P2P peer ID
	if d.p2pHost != nil {
		return d.p2pHost.PeerID().String()
	}
	return "standalone"
}

// ListenAddresses returns the gRPC server listen addresses.
func (d *Daemon) ListenAddresses() []string {
	addr := fmt.Sprintf("%s:%d", d.cfg.Server.Host, d.cfg.Server.Port)
	return []string{addr}
}

// HealthConfig returns configuration relevant to health reporting.
func (d *Daemon) HealthConfig() interfaces.HealthProviderConfig {
	return interfaces.HealthProviderConfig{
		P2PEnabled:        d.cfg.P2P.Enabled,
		ClusterEnabled:    d.cfg.Cluster.Enabled,
		BandwidthMetering: d.cfg.P2P.Metrics.BandwidthMetering,
	}
}
