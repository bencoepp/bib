package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

const (
	AppBib  = "bib"
	AppBibd = "bibd"
)

// configSearchPaths returns the paths to search for config files in order of precedence
// (later paths have higher priority in Viper)
func configSearchPaths(appName string) []string {
	paths := []string{}

	// System-wide (lowest priority)
	paths = append(paths, filepath.Join("/etc", appName))

	// User-specific
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", appName))
	}

	// Current directory (highest priority for files)
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, cwd)
	}

	return paths
}

// UserConfigDir returns the user-specific config directory for the app
func UserConfigDir(appName string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".config", appName), nil
}

// newViper creates and configures a new Viper instance for the given app
func newViper(appName string) *viper.Viper {
	v := viper.New()

	// Config file settings
	v.SetConfigName("config")
	v.SetConfigType("yaml") // default, but will auto-detect

	// Add search paths
	for _, path := range configSearchPaths(appName) {
		v.AddConfigPath(path)
	}

	// Environment variable settings
	v.SetEnvPrefix(strings.ToUpper(appName))
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	return v
}

// LoadBib loads the configuration for the bib CLI
func LoadBib(cfgFile string) (*BibConfig, error) {
	v := newViper(AppBib)

	// Set defaults
	defaults := DefaultBibConfig()
	setViperDefaults(v, defaults)

	// Load config file
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	}

	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
		// Config file not found; use defaults + env vars
	}

	var cfg BibConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Resolve any secrets
	if err := resolveSecrets(&cfg); err != nil {
		return nil, fmt.Errorf("failed to resolve secrets: %w", err)
	}

	return &cfg, nil
}

// LoadBibd loads the configuration for the bibd daemon
func LoadBibd(cfgFile string) (*BibdConfig, error) {
	v := newViper(AppBibd)

	// Set defaults
	defaults := DefaultBibdConfig()
	setViperDefaults(v, defaults)

	// Load config file
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	}

	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
		// Config file not found; use defaults + env vars
	}

	var cfg BibdConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Resolve any secrets
	if err := resolveSecrets(&cfg); err != nil {
		return nil, fmt.Errorf("failed to resolve secrets: %w", err)
	}

	return &cfg, nil
}

// setViperDefaults sets default values in Viper from a config struct
func setViperDefaults(v *viper.Viper, cfg interface{}) {
	switch c := cfg.(type) {
	case *BibConfig:
		v.SetDefault("log.level", c.Log.Level)
		v.SetDefault("log.format", c.Log.Format)
		v.SetDefault("log.output", c.Log.Output)
		v.SetDefault("identity.name", c.Identity.Name)
		v.SetDefault("identity.email", c.Identity.Email)
		v.SetDefault("identity.key", c.Identity.Key)
		v.SetDefault("output.format", c.Output.Format)
		v.SetDefault("output.color", c.Output.Color)
		v.SetDefault("server", c.Server)
	case *BibdConfig:
		v.SetDefault("log.level", c.Log.Level)
		v.SetDefault("log.format", c.Log.Format)
		v.SetDefault("log.output", c.Log.Output)
		v.SetDefault("identity.name", c.Identity.Name)
		v.SetDefault("identity.email", c.Identity.Email)
		v.SetDefault("identity.key", c.Identity.Key)
		v.SetDefault("server.host", c.Server.Host)
		v.SetDefault("server.port", c.Server.Port)
		v.SetDefault("server.tls.enabled", c.Server.TLS.Enabled)
		v.SetDefault("server.tls.cert_file", c.Server.TLS.CertFile)
		v.SetDefault("server.tls.key_file", c.Server.TLS.KeyFile)
		v.SetDefault("server.pid_file", c.Server.PIDFile)
		v.SetDefault("server.data_dir", c.Server.DataDir)
		// P2P defaults
		v.SetDefault("p2p.enabled", c.P2P.Enabled)
		v.SetDefault("p2p.mode", c.P2P.Mode)
		v.SetDefault("p2p.identity.key_path", c.P2P.Identity.KeyPath)
		v.SetDefault("p2p.listen_addresses", c.P2P.ListenAddresses)
		v.SetDefault("p2p.connection_manager.low_watermark", c.P2P.ConnManager.LowWatermark)
		v.SetDefault("p2p.connection_manager.high_watermark", c.P2P.ConnManager.HighWatermark)
		v.SetDefault("p2p.connection_manager.grace_period", c.P2P.ConnManager.GracePeriod)
		// Bootstrap defaults
		v.SetDefault("p2p.bootstrap.peers", c.P2P.Bootstrap.Peers)
		v.SetDefault("p2p.bootstrap.min_peers", c.P2P.Bootstrap.MinPeers)
		v.SetDefault("p2p.bootstrap.retry_interval", c.P2P.Bootstrap.RetryInterval)
		v.SetDefault("p2p.bootstrap.max_retry_interval", c.P2P.Bootstrap.MaxRetryInterval)
		// mDNS defaults
		v.SetDefault("p2p.mdns.enabled", c.P2P.MDNS.Enabled)
		v.SetDefault("p2p.mdns.service_name", c.P2P.MDNS.ServiceName)
		// DHT defaults
		v.SetDefault("p2p.dht.enabled", c.P2P.DHT.Enabled)
		v.SetDefault("p2p.dht.mode", c.P2P.DHT.Mode)
		// Peer store defaults
		v.SetDefault("p2p.peer_store.path", c.P2P.PeerStore.Path)
		// Full replica mode defaults
		v.SetDefault("p2p.full_replica.sync_interval", c.P2P.FullReplica.SyncInterval)
		// Selective mode defaults
		v.SetDefault("p2p.selective.subscriptions", c.P2P.Selective.Subscriptions)
		v.SetDefault("p2p.selective.subscription_store_path", c.P2P.Selective.SubscriptionStorePath)
		// Proxy mode defaults
		v.SetDefault("p2p.proxy.cache_ttl", c.P2P.Proxy.CacheTTL)
		v.SetDefault("p2p.proxy.max_cache_size", c.P2P.Proxy.MaxCacheSize)
		v.SetDefault("p2p.proxy.favorite_peers", c.P2P.Proxy.FavoritePeers)
		// Cluster defaults
		v.SetDefault("cluster.enabled", c.Cluster.Enabled)
		v.SetDefault("cluster.node_id", c.Cluster.NodeID)
		v.SetDefault("cluster.cluster_name", c.Cluster.ClusterName)
		v.SetDefault("cluster.data_dir", c.Cluster.DataDir)
		v.SetDefault("cluster.listen_addr", c.Cluster.ListenAddr)
		v.SetDefault("cluster.advertise_addr", c.Cluster.AdvertiseAddr)
		v.SetDefault("cluster.is_voter", c.Cluster.IsVoter)
		v.SetDefault("cluster.bootstrap", c.Cluster.Bootstrap)
		v.SetDefault("cluster.join_token", c.Cluster.JoinToken)
		v.SetDefault("cluster.join_addrs", c.Cluster.JoinAddrs)
		v.SetDefault("cluster.enable_dht_discovery", c.Cluster.EnableDHTDiscovery)
		v.SetDefault("cluster.raft.heartbeat_timeout", c.Cluster.Raft.HeartbeatTimeout)
		v.SetDefault("cluster.raft.election_timeout", c.Cluster.Raft.ElectionTimeout)
		v.SetDefault("cluster.raft.commit_timeout", c.Cluster.Raft.CommitTimeout)
		v.SetDefault("cluster.raft.max_append_entries", c.Cluster.Raft.MaxAppendEntries)
		v.SetDefault("cluster.raft.trailing_logs", c.Cluster.Raft.TrailingLogs)
		v.SetDefault("cluster.raft.max_inflight", c.Cluster.Raft.MaxInflight)
		v.SetDefault("cluster.snapshot.interval", c.Cluster.Snapshot.Interval)
		v.SetDefault("cluster.snapshot.threshold", c.Cluster.Snapshot.Threshold)
		v.SetDefault("cluster.snapshot.retain_count", c.Cluster.Snapshot.RetainCount)
		// Database defaults
		v.SetDefault("database.backend", c.Database.Backend)
		// SQLite defaults
		v.SetDefault("database.sqlite.path", c.Database.SQLite.Path)
		v.SetDefault("database.sqlite.max_open_conns", c.Database.SQLite.MaxOpenConns)
		// PostgreSQL defaults
		v.SetDefault("database.postgres.managed", c.Database.Postgres.Managed)
		v.SetDefault("database.postgres.container_runtime", c.Database.Postgres.ContainerRuntime)
		v.SetDefault("database.postgres.socket_path", c.Database.Postgres.SocketPath)
		v.SetDefault("database.postgres.kubeconfig_path", c.Database.Postgres.KubeconfigPath)
		v.SetDefault("database.postgres.image", c.Database.Postgres.Image)
		v.SetDefault("database.postgres.data_dir", c.Database.Postgres.DataDir)
		v.SetDefault("database.postgres.port", c.Database.Postgres.Port)
		v.SetDefault("database.postgres.max_connections", c.Database.Postgres.MaxConnections)
		v.SetDefault("database.postgres.memory_mb", c.Database.Postgres.MemoryMB)
		v.SetDefault("database.postgres.cpu_cores", c.Database.Postgres.CPUCores)
		v.SetDefault("database.postgres.ssl_mode", c.Database.Postgres.SSLMode)
		v.SetDefault("database.postgres.credential_rotation_interval", c.Database.Postgres.CredentialRotationInterval)
		// PostgreSQL network defaults
		v.SetDefault("database.postgres.network.use_bridge_network", c.Database.Postgres.Network.UseBridgeNetwork)
		v.SetDefault("database.postgres.network.bridge_network_name", c.Database.Postgres.Network.BridgeNetworkName)
		v.SetDefault("database.postgres.network.use_unix_socket", c.Database.Postgres.Network.UseUnixSocket)
		v.SetDefault("database.postgres.network.bind_address", c.Database.Postgres.Network.BindAddress)
		// PostgreSQL health defaults
		v.SetDefault("database.postgres.health.interval", c.Database.Postgres.Health.Interval)
		v.SetDefault("database.postgres.health.timeout", c.Database.Postgres.Health.Timeout)
		v.SetDefault("database.postgres.health.startup_timeout", c.Database.Postgres.Health.StartupTimeout)
		v.SetDefault("database.postgres.health.action", c.Database.Postgres.Health.Action)
		v.SetDefault("database.postgres.health.max_retries", c.Database.Postgres.Health.MaxRetries)
		v.SetDefault("database.postgres.health.retry_backoff", c.Database.Postgres.Health.RetryBackoff)
		// PostgreSQL TLS defaults
		v.SetDefault("database.postgres.tls.enabled", c.Database.Postgres.TLS.Enabled)
		v.SetDefault("database.postgres.tls.cert_dir", c.Database.Postgres.TLS.CertDir)
		v.SetDefault("database.postgres.tls.auto_generate", c.Database.Postgres.TLS.AutoGenerate)
		// Audit defaults
		v.SetDefault("database.audit.enabled", c.Database.Audit.Enabled)
		v.SetDefault("database.audit.retention_days", c.Database.Audit.RetentionDays)
		v.SetDefault("database.audit.hash_chain", c.Database.Audit.HashChain)
	}
}

// ConfigFileUsed returns the config file path that was loaded, if any
func ConfigFileUsed(appName string) string {
	v := newViper(appName)
	_ = v.ReadInConfig()
	return v.ConfigFileUsed()
}

// NewViperFromConfig creates a viper instance populated with values from a config struct
func NewViperFromConfig(appName string, cfg interface{}) *viper.Viper {
	v := viper.New()

	switch c := cfg.(type) {
	case *BibConfig:
		v.Set("log.level", c.Log.Level)
		v.Set("log.format", c.Log.Format)
		v.Set("log.output", c.Log.Output)
		v.Set("identity.name", c.Identity.Name)
		v.Set("identity.email", c.Identity.Email)
		v.Set("identity.key", c.Identity.Key)
		v.Set("output.format", c.Output.Format)
		v.Set("output.color", c.Output.Color)
		v.Set("server", c.Server)
	case *BibdConfig:
		v.Set("log.level", c.Log.Level)
		v.Set("log.format", c.Log.Format)
		v.Set("log.output", c.Log.Output)
		v.Set("identity.name", c.Identity.Name)
		v.Set("identity.email", c.Identity.Email)
		v.Set("identity.key", c.Identity.Key)
		v.Set("server.host", c.Server.Host)
		v.Set("server.port", c.Server.Port)
		v.Set("server.tls.enabled", c.Server.TLS.Enabled)
		v.Set("server.tls.cert_file", c.Server.TLS.CertFile)
		v.Set("server.tls.key_file", c.Server.TLS.KeyFile)
		v.Set("server.pid_file", c.Server.PIDFile)
		v.Set("server.data_dir", c.Server.DataDir)
		// P2P settings
		v.Set("p2p.enabled", c.P2P.Enabled)
		v.Set("p2p.mode", c.P2P.Mode)
		v.Set("p2p.identity.key_path", c.P2P.Identity.KeyPath)
		v.Set("p2p.listen_addresses", c.P2P.ListenAddresses)
		v.Set("p2p.connection_manager.low_watermark", c.P2P.ConnManager.LowWatermark)
		v.Set("p2p.connection_manager.high_watermark", c.P2P.ConnManager.HighWatermark)
		v.Set("p2p.connection_manager.grace_period", c.P2P.ConnManager.GracePeriod)
		// Bootstrap settings
		v.Set("p2p.bootstrap.peers", c.P2P.Bootstrap.Peers)
		v.Set("p2p.bootstrap.min_peers", c.P2P.Bootstrap.MinPeers)
		v.Set("p2p.bootstrap.retry_interval", c.P2P.Bootstrap.RetryInterval)
		v.Set("p2p.bootstrap.max_retry_interval", c.P2P.Bootstrap.MaxRetryInterval)
		// mDNS settings
		v.Set("p2p.mdns.enabled", c.P2P.MDNS.Enabled)
		v.Set("p2p.mdns.service_name", c.P2P.MDNS.ServiceName)
		// DHT settings
		v.Set("p2p.dht.enabled", c.P2P.DHT.Enabled)
		v.Set("p2p.dht.mode", c.P2P.DHT.Mode)
		// Peer store settings
		v.Set("p2p.peer_store.path", c.P2P.PeerStore.Path)
		// Full replica mode settings
		v.Set("p2p.full_replica.sync_interval", c.P2P.FullReplica.SyncInterval)
		// Selective mode settings
		v.Set("p2p.selective.subscriptions", c.P2P.Selective.Subscriptions)
		v.Set("p2p.selective.subscription_store_path", c.P2P.Selective.SubscriptionStorePath)
		// Proxy mode settings
		v.Set("p2p.proxy.cache_ttl", c.P2P.Proxy.CacheTTL)
		v.Set("p2p.proxy.max_cache_size", c.P2P.Proxy.MaxCacheSize)
		v.Set("p2p.proxy.favorite_peers", c.P2P.Proxy.FavoritePeers)
		// Cluster settings
		v.Set("cluster.enabled", c.Cluster.Enabled)
		v.Set("cluster.node_id", c.Cluster.NodeID)
		v.Set("cluster.cluster_name", c.Cluster.ClusterName)
		v.Set("cluster.data_dir", c.Cluster.DataDir)
		v.Set("cluster.listen_addr", c.Cluster.ListenAddr)
		v.Set("cluster.advertise_addr", c.Cluster.AdvertiseAddr)
		v.Set("cluster.is_voter", c.Cluster.IsVoter)
		v.Set("cluster.bootstrap", c.Cluster.Bootstrap)
		v.Set("cluster.join_token", c.Cluster.JoinToken)
		v.Set("cluster.join_addrs", c.Cluster.JoinAddrs)
		v.Set("cluster.enable_dht_discovery", c.Cluster.EnableDHTDiscovery)
		v.Set("cluster.raft.heartbeat_timeout", c.Cluster.Raft.HeartbeatTimeout)
		v.Set("cluster.raft.election_timeout", c.Cluster.Raft.ElectionTimeout)
		v.Set("cluster.raft.commit_timeout", c.Cluster.Raft.CommitTimeout)
		v.Set("cluster.raft.max_append_entries", c.Cluster.Raft.MaxAppendEntries)
		v.Set("cluster.raft.trailing_logs", c.Cluster.Raft.TrailingLogs)
		v.Set("cluster.raft.max_inflight", c.Cluster.Raft.MaxInflight)
		v.Set("cluster.snapshot.interval", c.Cluster.Snapshot.Interval)
		v.Set("cluster.snapshot.threshold", c.Cluster.Snapshot.Threshold)
		v.Set("cluster.snapshot.retain_count", c.Cluster.Snapshot.RetainCount)
		// Database settings
		v.Set("database.backend", c.Database.Backend)
		// SQLite settings
		v.Set("database.sqlite.path", c.Database.SQLite.Path)
		v.Set("database.sqlite.max_open_conns", c.Database.SQLite.MaxOpenConns)
		// PostgreSQL settings
		v.Set("database.postgres.managed", c.Database.Postgres.Managed)
		v.Set("database.postgres.container_runtime", c.Database.Postgres.ContainerRuntime)
		v.Set("database.postgres.socket_path", c.Database.Postgres.SocketPath)
		v.Set("database.postgres.kubeconfig_path", c.Database.Postgres.KubeconfigPath)
		v.Set("database.postgres.image", c.Database.Postgres.Image)
		v.Set("database.postgres.data_dir", c.Database.Postgres.DataDir)
		v.Set("database.postgres.port", c.Database.Postgres.Port)
		v.Set("database.postgres.max_connections", c.Database.Postgres.MaxConnections)
		v.Set("database.postgres.memory_mb", c.Database.Postgres.MemoryMB)
		v.Set("database.postgres.cpu_cores", c.Database.Postgres.CPUCores)
		v.Set("database.postgres.ssl_mode", c.Database.Postgres.SSLMode)
		v.Set("database.postgres.credential_rotation_interval", c.Database.Postgres.CredentialRotationInterval)
		// PostgreSQL network settings
		v.Set("database.postgres.network.use_bridge_network", c.Database.Postgres.Network.UseBridgeNetwork)
		v.Set("database.postgres.network.bridge_network_name", c.Database.Postgres.Network.BridgeNetworkName)
		v.Set("database.postgres.network.use_unix_socket", c.Database.Postgres.Network.UseUnixSocket)
		v.Set("database.postgres.network.bind_address", c.Database.Postgres.Network.BindAddress)
		// PostgreSQL health settings
		v.Set("database.postgres.health.interval", c.Database.Postgres.Health.Interval)
		v.Set("database.postgres.health.timeout", c.Database.Postgres.Health.Timeout)
		v.Set("database.postgres.health.startup_timeout", c.Database.Postgres.Health.StartupTimeout)
		v.Set("database.postgres.health.action", c.Database.Postgres.Health.Action)
		v.Set("database.postgres.health.max_retries", c.Database.Postgres.Health.MaxRetries)
		v.Set("database.postgres.health.retry_backoff", c.Database.Postgres.Health.RetryBackoff)
		// PostgreSQL TLS settings
		v.Set("database.postgres.tls.enabled", c.Database.Postgres.TLS.Enabled)
		v.Set("database.postgres.tls.cert_dir", c.Database.Postgres.TLS.CertDir)
		v.Set("database.postgres.tls.auto_generate", c.Database.Postgres.TLS.AutoGenerate)
		// Audit settings
		v.Set("database.audit.enabled", c.Database.Audit.Enabled)
		v.Set("database.audit.retention_days", c.Database.Audit.RetentionDays)
		v.Set("database.audit.hash_chain", c.Database.Audit.HashChain)
	}

	return v
}
