// Package main provides the bibd daemon.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"bib/internal/cluster"
	"bib/internal/config"
	"bib/internal/logger"
	"bib/internal/p2p"
	"bib/internal/storage"

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

	store   storage.Store
	p2pHost *p2p.Host
	p2pDisc *p2p.Discovery
	p2pMode *p2p.ModeManager
	cluster *cluster.Cluster

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

// startStorage initializes the storage backend.
func (d *Daemon) startStorage(ctx context.Context) error {
	d.log.Debug("initializing storage",
		"backend", d.cfg.Database.Backend,
		"data_dir", d.cfg.Server.DataDir,
	)

	// Convert config types
	storageCfg := d.convertStorageConfig()

	// Determine node ID for storage
	nodeID := d.cfg.Cluster.NodeID
	if nodeID == "" {
		nodeID = "standalone"
	}

	// Open storage
	store, err := storage.Open(ctx, storageCfg, d.cfg.Server.DataDir, nodeID, d.cfg.P2P.Mode)
	if err != nil {
		d.log.Error("failed to open storage", "error", err)
		return err
	}

	// Ping to verify connectivity
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

// stopStorage shuts down the storage backend.
func (d *Daemon) stopStorage() error {
	if d.store == nil {
		return nil
	}

	d.log.Debug("shutting down storage")

	if err := d.store.Close(); err != nil {
		d.log.Error("error closing storage", "error", err)
		return err
	}

	d.store = nil
	d.log.Debug("storage shut down")
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
			Managed:          d.cfg.Database.Postgres.Managed,
			ContainerRuntime: d.cfg.Database.Postgres.ContainerRuntime,
			Image:            d.cfg.Database.Postgres.Image,
			DataDir:          d.cfg.Database.Postgres.DataDir,
			Port:             d.cfg.Database.Postgres.Port,
			MaxConnections:   d.cfg.Database.Postgres.MaxConnections,
			Resources: storage.ContainerResources{
				MemoryMB: d.cfg.Database.Postgres.MemoryMB,
				CPUCores: d.cfg.Database.Postgres.CPUCores,
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
