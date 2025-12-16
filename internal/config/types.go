package config

import (
	"time"
)

// LogConfig holds logging configuration shared by both bib and bibd
type LogConfig struct {
	Level           string   `mapstructure:"level"`              // debug, info, warn, error
	Format          string   `mapstructure:"format"`             // text, json, pretty
	Output          string   `mapstructure:"output"`             // stdout, stderr, or file path
	FilePath        string   `mapstructure:"file_path"`          // path to log file (in addition to output)
	MaxSizeMB       int      `mapstructure:"max_size_mb"`        // max size in MB before rotation
	MaxBackups      int      `mapstructure:"max_backups"`        // max number of old log files to keep
	MaxAgeDays      int      `mapstructure:"max_age_days"`       // max days to retain old log files
	EnableCaller    bool     `mapstructure:"enable_caller"`      // include source file/line in logs
	NoColor         bool     `mapstructure:"no_color"`           // disable colored output (pretty format only)
	AuditPath       string   `mapstructure:"audit_path"`         // path to audit log file
	AuditMaxAgeDays int      `mapstructure:"audit_max_age_days"` // max days to retain audit logs
	RedactFields    []string `mapstructure:"redact_fields"`      // field names to redact from logs
}

// IdentityConfig holds identity/authentication configuration
type IdentityConfig struct {
	Name  string `mapstructure:"name"`
	Email string `mapstructure:"email"`
	Key   string `mapstructure:"key"` // can be a path or secret reference
}

// OutputConfig holds output formatting options (bib CLI only)
type OutputConfig struct {
	Format string `mapstructure:"format"` // text, json, yaml, table
	Color  bool   `mapstructure:"color"`
}

// ServerConfig holds daemon server configuration (bibd only)
type ServerConfig struct {
	Host    string    `mapstructure:"host"`
	Port    int       `mapstructure:"port"`
	TLS     TLSConfig `mapstructure:"tls"`
	PIDFile string    `mapstructure:"pid_file"`
	DataDir string    `mapstructure:"data_dir"`
}

// TLSConfig holds TLS/SSL configuration
type TLSConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
}

// P2PConfig holds P2P networking configuration for the daemon
type P2PConfig struct {
	// Enabled controls whether P2P networking is active
	Enabled bool `mapstructure:"enabled"`

	// Mode controls the node operation mode: "proxy", "selective", or "full"
	// - proxy: no local storage, forwards requests to peers (default)
	// - selective: subscribe to specific topics/datasets on-demand
	// - full: replicate all data from connected peers
	Mode string `mapstructure:"mode"`

	// Identity configuration
	Identity P2PIdentityConfig `mapstructure:"identity"`

	// Listen addresses in multiaddr format
	// Defaults: ["/ip4/0.0.0.0/tcp/4001", "/ip4/0.0.0.0/udp/4001/quic-v1"]
	ListenAddresses []string `mapstructure:"listen_addresses"`

	// Connection manager settings
	ConnManager ConnManagerConfig `mapstructure:"connection_manager"`

	// Bootstrap node configuration
	Bootstrap BootstrapConfig `mapstructure:"bootstrap"`

	// mDNS local discovery configuration
	MDNS MDNSConfig `mapstructure:"mdns"`

	// DHT configuration
	DHT DHTConfig `mapstructure:"dht"`

	// Peer store configuration
	PeerStore PeerStoreConfig `mapstructure:"peer_store"`

	// Full replica mode configuration
	FullReplica FullReplicaConfig `mapstructure:"full_replica"`

	// Selective mode configuration
	Selective SelectiveConfig `mapstructure:"selective"`

	// Proxy mode configuration
	Proxy ProxyConfig `mapstructure:"proxy"`
}

// P2PIdentityConfig holds node P2P identity configuration
type P2PIdentityConfig struct {
	// KeyPath is the path to the PEM-encoded Ed25519 private key file.
	// If empty, defaults to the config directory + "/identity.pem"
	KeyPath string `mapstructure:"key_path"`
}

// ConnManagerConfig holds connection manager settings
type ConnManagerConfig struct {
	// LowWatermark is the minimum number of connections to maintain.
	LowWatermark int `mapstructure:"low_watermark"`

	// HighWatermark is the maximum number of connections before pruning begins.
	HighWatermark int `mapstructure:"high_watermark"`

	// GracePeriod is the duration a new connection is protected from pruning.
	GracePeriod time.Duration `mapstructure:"grace_period"`
}

// BootstrapConfig holds bootstrap node configuration
type BootstrapConfig struct {
	// Peers is a list of bootstrap peer multiaddrs
	// Default includes bib.dev bootstrap node
	Peers []string `mapstructure:"peers"`

	// MinPeers is the minimum number of bootstrap peers to connect to before continuing
	MinPeers int `mapstructure:"min_peers"`

	// RetryInterval is the initial retry interval for failed connections
	RetryInterval time.Duration `mapstructure:"retry_interval"`

	// MaxRetryInterval is the maximum retry interval (exponential backoff cap)
	MaxRetryInterval time.Duration `mapstructure:"max_retry_interval"`
}

// MDNSConfig holds mDNS local discovery configuration
type MDNSConfig struct {
	// Enabled controls whether mDNS discovery is active
	Enabled bool `mapstructure:"enabled"`

	// ServiceName is the mDNS service name to advertise/discover
	// Default: "bib.local"
	ServiceName string `mapstructure:"service_name"`
}

// DHTConfig holds Kademlia DHT configuration
type DHTConfig struct {
	// Enabled controls whether DHT is active
	Enabled bool `mapstructure:"enabled"`

	// Mode controls the DHT operation mode: "auto", "server", or "client"
	// - auto: libp2p decides based on reachability
	// - server: full DHT participant, stores records (requires public IP)
	// - client: queries only, doesn't store records (works behind NAT)
	Mode string `mapstructure:"mode"`
}

// PeerStoreConfig holds peer store configuration
type PeerStoreConfig struct {
	// Path is the path to the SQLite peer store database
	// If empty, defaults to config directory + "/peers.db"
	Path string `mapstructure:"path"`
}

// FullReplicaConfig holds configuration for full replica mode
type FullReplicaConfig struct {
	// SyncInterval is how often to poll peers for new data
	SyncInterval time.Duration `mapstructure:"sync_interval"`
}

// SelectiveConfig holds configuration for selective mode
type SelectiveConfig struct {
	// Subscriptions is a list of topic patterns to subscribe to
	// Persisted across restarts
	Subscriptions []string `mapstructure:"subscriptions"`

	// SubscriptionStorePath is where to persist subscriptions
	// If empty, defaults to config directory + "/subscriptions.json"
	SubscriptionStorePath string `mapstructure:"subscription_store_path"`
}

// ProxyConfig holds configuration for proxy mode
type ProxyConfig struct {
	// CacheTTL is how long to cache query results
	CacheTTL time.Duration `mapstructure:"cache_ttl"`

	// MaxCacheSize is the maximum number of cached entries
	MaxCacheSize int `mapstructure:"max_cache_size"`

	// FavoritePeers is a list of preferred peers for forwarding requests
	// If empty, forwards to any discovered peer
	FavoritePeers []string `mapstructure:"favorite_peers"`
}

// ClusterConfig holds HA cluster configuration using Raft consensus
type ClusterConfig struct {
	// Enabled controls whether clustering/HA mode is active
	// Disabled by default - single node operation
	Enabled bool `mapstructure:"enabled"`

	// NodeID is a unique identifier for this node within the cluster
	// Auto-generated if empty
	NodeID string `mapstructure:"node_id"`

	// ClusterName is a unique name for the cluster (used in DHT discovery)
	ClusterName string `mapstructure:"cluster_name"`

	// DataDir is where Raft data (logs, snapshots) are stored
	// Defaults to <config_dir>/raft
	DataDir string `mapstructure:"data_dir"`

	// ListenAddr is the address for Raft inter-node communication
	// Defaults to 0.0.0.0:4002
	ListenAddr string `mapstructure:"listen_addr"`

	// AdvertiseAddr is the address advertised to other nodes
	// Defaults to ListenAddr
	AdvertiseAddr string `mapstructure:"advertise_addr"`

	// IsVoter controls whether this node can vote in leader elections
	// Non-voters replicate data but don't participate in consensus
	IsVoter bool `mapstructure:"is_voter"`

	// Bootstrap indicates this is the initial cluster node
	// Only set for the first node when initializing a new cluster
	Bootstrap bool `mapstructure:"bootstrap"`

	// JoinToken is used to join an existing cluster
	// Generated by the leader node during cluster init
	JoinToken string `mapstructure:"join_token"`

	// JoinAddrs is a list of existing cluster member addresses to join
	// Used as alternative to JoinToken
	JoinAddrs []string `mapstructure:"join_addrs"`

	// EnableDHTDiscovery allows automatic cluster discovery via DHT
	EnableDHTDiscovery bool `mapstructure:"enable_dht_discovery"`

	// Raft-specific settings
	Raft RaftConfig `mapstructure:"raft"`

	// Snapshot settings
	Snapshot SnapshotConfig `mapstructure:"snapshot"`
}

// RaftConfig holds Raft consensus algorithm settings
type RaftConfig struct {
	// HeartbeatTimeout is the interval for leader heartbeats
	HeartbeatTimeout time.Duration `mapstructure:"heartbeat_timeout"`

	// ElectionTimeout is the timeout for leader elections
	ElectionTimeout time.Duration `mapstructure:"election_timeout"`

	// CommitTimeout is the timeout for log commits
	CommitTimeout time.Duration `mapstructure:"commit_timeout"`

	// MaxAppendEntries is the maximum entries per append RPC
	MaxAppendEntries int `mapstructure:"max_append_entries"`

	// TrailingLogs is the number of logs to keep after snapshot
	TrailingLogs uint64 `mapstructure:"trailing_logs"`

	// MaxInflight is the maximum in-flight append entries
	MaxInflight int `mapstructure:"max_inflight"`
}

// SnapshotConfig holds snapshot configuration
type SnapshotConfig struct {
	// Interval is how often to take automatic snapshots
	// Set to 0 to disable automatic snapshots
	Interval time.Duration `mapstructure:"interval"`

	// Threshold is the number of log entries before taking a snapshot
	Threshold uint64 `mapstructure:"threshold"`

	// RetainCount is how many snapshots to retain
	RetainCount int `mapstructure:"retain_count"`
}

// BibConfig is the complete configuration for the bib CLI
type BibConfig struct {
	Log      LogConfig      `mapstructure:"log"`
	Identity IdentityConfig `mapstructure:"identity"`
	Output   OutputConfig   `mapstructure:"output"`
	Server   string         `mapstructure:"server"` // bibd server address to connect to
}

// BibdConfig is the complete configuration for the bibd daemon
type BibdConfig struct {
	Log      LogConfig      `mapstructure:"log"`
	Identity IdentityConfig `mapstructure:"identity"`
	Server   ServerConfig   `mapstructure:"server"`
	P2P      P2PConfig      `mapstructure:"p2p"`
	Cluster  ClusterConfig  `mapstructure:"cluster"`
	Database DatabaseConfig `mapstructure:"database"`
}

// DatabaseConfig holds storage layer configuration
type DatabaseConfig struct {
	// Backend is the storage backend type: "sqlite" or "postgres"
	Backend string `mapstructure:"backend"`

	// SQLite configuration (used when Backend is "sqlite")
	SQLite SQLiteDatabaseConfig `mapstructure:"sqlite"`

	// Postgres configuration (used when Backend is "postgres")
	Postgres PostgresDatabaseConfig `mapstructure:"postgres"`

	// Audit configuration
	Audit AuditDatabaseConfig `mapstructure:"audit"`
}

// SQLiteDatabaseConfig holds SQLite-specific configuration
type SQLiteDatabaseConfig struct {
	// Path is the path to the SQLite database file
	// Defaults to <data_dir>/cache.db
	Path string `mapstructure:"path"`

	// MaxOpenConns is the maximum number of open connections
	MaxOpenConns int `mapstructure:"max_open_conns"`
}

// PostgresDatabaseConfig holds PostgreSQL-specific configuration
type PostgresDatabaseConfig struct {
	// Managed indicates whether bibd manages the PostgreSQL lifecycle
	Managed bool `mapstructure:"managed"`

	// ContainerRuntime is the container runtime: "docker" or "podman"
	ContainerRuntime string `mapstructure:"container_runtime"`

	// Image is the PostgreSQL container image
	Image string `mapstructure:"image"`

	// DataDir is where PostgreSQL data is stored
	DataDir string `mapstructure:"data_dir"`

	// Port is the PostgreSQL port (internal)
	Port int `mapstructure:"port"`

	// MaxConnections is the maximum number of connections
	MaxConnections int `mapstructure:"max_connections"`

	// MemoryMB is the memory limit for the container
	MemoryMB int `mapstructure:"memory_mb"`

	// CPUCores is the CPU limit for the container
	CPUCores float64 `mapstructure:"cpu_cores"`

	// Advanced allows manual connection (for debugging only)
	Advanced *PostgresAdvancedConfig `mapstructure:"advanced,omitempty"`
}

// PostgresAdvancedConfig allows manual PostgreSQL configuration (testing only)
type PostgresAdvancedConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Database string `mapstructure:"database"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	SSLMode  string `mapstructure:"ssl_mode"`
}

// AuditDatabaseConfig holds audit logging configuration
type AuditDatabaseConfig struct {
	// Enabled controls whether audit logging is active
	Enabled bool `mapstructure:"enabled"`

	// RetentionDays is how long to keep audit logs
	RetentionDays int `mapstructure:"retention_days"`

	// HashChain enables hash chain for tamper detection
	HashChain bool `mapstructure:"hash_chain"`
}

// DefaultBibConfig returns sensible defaults for bib CLI
func DefaultBibConfig() *BibConfig {
	return &BibConfig{
		Log: LogConfig{
			Level:           "info",
			Format:          "text",
			Output:          "stderr",
			FilePath:        "",
			MaxSizeMB:       100,
			MaxBackups:      3,
			MaxAgeDays:      28,
			EnableCaller:    false,
			AuditPath:       "",
			AuditMaxAgeDays: 365,
			RedactFields:    []string{"password", "token", "key", "secret", "credential", "auth"},
		},
		Identity: IdentityConfig{},
		Output: OutputConfig{
			Format: "text",
			Color:  true,
		},
		Server: "localhost:8080",
	}
}

// DefaultBibdConfig returns sensible defaults for bibd daemon
func DefaultBibdConfig() *BibdConfig {
	return &BibdConfig{
		Log: LogConfig{
			Level:           "info",
			Format:          "pretty",
			Output:          "stdout",
			FilePath:        "",
			MaxSizeMB:       100,
			MaxBackups:      3,
			MaxAgeDays:      28,
			EnableCaller:    true,
			NoColor:         false,
			AuditPath:       "",
			AuditMaxAgeDays: 365,
			RedactFields:    []string{"password", "token", "key", "secret", "credential", "auth"},
		},
		Identity: IdentityConfig{},
		Server: ServerConfig{
			Host:    "0.0.0.0",
			Port:    8080,
			PIDFile: "/var/run/bibd.pid",
			DataDir: "~/.local/share/bibd",
			TLS: TLSConfig{
				Enabled: false,
			},
		},
		P2P: P2PConfig{
			Enabled:  true,
			Mode:     "proxy", // Default to proxy mode
			Identity: P2PIdentityConfig{},
			ListenAddresses: []string{
				"/ip4/0.0.0.0/tcp/4001",
				"/ip4/0.0.0.0/udp/4001/quic-v1",
			},
			ConnManager: ConnManagerConfig{
				LowWatermark:  100,
				HighWatermark: 400,
				GracePeriod:   30 * time.Second,
			},
			Bootstrap: BootstrapConfig{
				Peers: []string{
					// bib.dev bootstrap node - peer ID will be discovered via DNS
					"/dns4/bib.dev/tcp/4001",
					"/dns4/bib.dev/udp/4001/quic-v1",
				},
				MinPeers:         1,
				RetryInterval:    5 * time.Second,
				MaxRetryInterval: 1 * time.Hour,
			},
			MDNS: MDNSConfig{
				Enabled:     true,
				ServiceName: "bib.local",
			},
			DHT: DHTConfig{
				Enabled: true,
				Mode:    "auto",
			},
			PeerStore: PeerStoreConfig{
				Path: "", // defaults to config dir + "/peers.db"
			},
			FullReplica: FullReplicaConfig{
				SyncInterval: 5 * time.Minute,
			},
			Selective: SelectiveConfig{
				Subscriptions:         []string{},
				SubscriptionStorePath: "", // defaults to config dir + "/subscriptions.json"
			},
			Proxy: ProxyConfig{
				CacheTTL:      2 * time.Minute,
				MaxCacheSize:  1000,
				FavoritePeers: []string{},
			},
		},
		Cluster: ClusterConfig{
			Enabled:            false, // Disabled by default - single node mode
			NodeID:             "",    // Auto-generated if empty
			ClusterName:        "bib-cluster",
			DataDir:            "", // defaults to config dir + "/raft"
			ListenAddr:         "0.0.0.0:4002",
			AdvertiseAddr:      "",   // defaults to ListenAddr
			IsVoter:            true, // Default to voter
			Bootstrap:          false,
			JoinToken:          "",
			JoinAddrs:          []string{},
			EnableDHTDiscovery: false,
			Raft: RaftConfig{
				HeartbeatTimeout: 1 * time.Second,
				ElectionTimeout:  5 * time.Second,
				CommitTimeout:    50 * time.Millisecond,
				MaxAppendEntries: 64,
				TrailingLogs:     10000,
				MaxInflight:      256,
			},
			Snapshot: SnapshotConfig{
				Interval:    30 * time.Minute, // Automatic snapshots every 30 minutes
				Threshold:   8192,             // Also snapshot after 8192 log entries
				RetainCount: 3,
			},
		},
		Database: DatabaseConfig{
			Backend: "sqlite", // Default to SQLite for easy onboarding
			SQLite: SQLiteDatabaseConfig{
				Path:         "", // defaults to <data_dir>/cache.db
				MaxOpenConns: 10,
			},
			Postgres: PostgresDatabaseConfig{
				Managed:          true,
				ContainerRuntime: "docker",
				Image:            "postgres:16-alpine",
				DataDir:          "", // defaults to <data_dir>/postgres
				Port:             5432,
				MaxConnections:   100,
				MemoryMB:         512,
				CPUCores:         1.0,
			},
			Audit: AuditDatabaseConfig{
				Enabled:       true,
				RetentionDays: 90,
				HashChain:     true,
			},
		},
	}
}
