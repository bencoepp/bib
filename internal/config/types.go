package config

import (
	"runtime"
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

// FavoriteNode represents a preferred node for connection
type FavoriteNode struct {
	// ID is the unique node identifier (peer ID)
	ID string `mapstructure:"id"`

	// Alias is a human-friendly name for this node
	Alias string `mapstructure:"alias"`

	// Priority determines connection preference (lower = higher priority)
	Priority int `mapstructure:"priority"`

	// Address is an optional direct address (multiaddr or host:port)
	Address string `mapstructure:"address,omitempty"`
}

// ConnectionConfig holds settings for connecting to bibd nodes
type ConnectionConfig struct {
	// FavoriteNodes is a list of preferred nodes for connection
	FavoriteNodes []FavoriteNode `mapstructure:"favorite_nodes"`

	// AutoDetect enables automatic node discovery
	AutoDetect bool `mapstructure:"auto_detect"`

	// Timeout is the connection timeout
	Timeout string `mapstructure:"timeout"`

	// RetryAttempts is the number of connection retry attempts
	RetryAttempts int `mapstructure:"retry_attempts"`
}

// BibConfig is the complete configuration for the bib CLI
type BibConfig struct {
	Log        LogConfig        `mapstructure:"log"`
	Identity   IdentityConfig   `mapstructure:"identity"`
	Output     OutputConfig     `mapstructure:"output"`
	Locale     string           `mapstructure:"locale"`     // UI locale (en, de, fr, ru, zh-tw). Empty = auto-detect from system
	Server     string           `mapstructure:"server"`     // bibd server address to connect to (legacy)
	Connection ConnectionConfig `mapstructure:"connection"` // Connection settings with favorite nodes
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

	// BreakGlass holds emergency access configuration
	BreakGlass BreakGlassConfig `mapstructure:"break_glass"`
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

	// ContainerRuntime is the container runtime: "docker", "podman", or "kubernetes"
	ContainerRuntime string `mapstructure:"container_runtime"`

	// SocketPath is the path to the container runtime socket (auto-detected if empty)
	SocketPath string `mapstructure:"socket_path"`

	// KubeconfigPath is the path to kubeconfig file (for Kubernetes runtime)
	KubeconfigPath string `mapstructure:"kubeconfig_path"`

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

	// SSLMode is the SSL mode for connections
	SSLMode string `mapstructure:"ssl_mode"`

	// CredentialRotationInterval is how often to rotate database credentials
	CredentialRotationInterval time.Duration `mapstructure:"credential_rotation_interval"`

	// Network configuration
	Network PostgresNetworkConfig `mapstructure:"network"`

	// Health check configuration
	Health PostgresHealthConfig `mapstructure:"health"`

	// TLS configuration
	TLS PostgresTLSConfig `mapstructure:"tls"`

	// Advanced allows manual connection (for debugging only)
	Advanced *PostgresAdvancedConfig `mapstructure:"advanced,omitempty"`
}

// PostgresNetworkConfig holds network configuration for managed PostgreSQL
type PostgresNetworkConfig struct {
	// UseBridgeNetwork creates a private bridge network for isolation
	UseBridgeNetwork bool `mapstructure:"use_bridge_network"`

	// BridgeNetworkName is the name of the bridge network
	BridgeNetworkName string `mapstructure:"bridge_network_name"`

	// UseUnixSocket uses Unix socket only (no TCP)
	UseUnixSocket bool `mapstructure:"use_unix_socket"`

	// BindAddress is the address to bind to (default: 127.0.0.1)
	BindAddress string `mapstructure:"bind_address"`
}

// PostgresHealthConfig holds health check configuration for managed PostgreSQL
type PostgresHealthConfig struct {
	// Interval is how often to check health
	Interval time.Duration `mapstructure:"interval"`

	// Timeout is the timeout for each health check
	Timeout time.Duration `mapstructure:"timeout"`

	// StartupTimeout is how long to wait for initial startup
	StartupTimeout time.Duration `mapstructure:"startup_timeout"`

	// Action defines what happens on repeated failures: "shutdown", "retry_always", "retry_limit"
	Action string `mapstructure:"action"`

	// MaxRetries is the maximum retries (for "retry_limit" action)
	MaxRetries int `mapstructure:"max_retries"`

	// RetryBackoff is the backoff duration between retries
	RetryBackoff time.Duration `mapstructure:"retry_backoff"`
}

// PostgresTLSConfig holds TLS configuration for PostgreSQL connections
type PostgresTLSConfig struct {
	// Enabled controls whether mTLS is enabled (always true for managed)
	Enabled bool `mapstructure:"enabled"`

	// CertDir is where certificates are stored
	CertDir string `mapstructure:"cert_dir"`

	// AutoGenerate automatically generates certificates from node identity
	AutoGenerate bool `mapstructure:"auto_generate"`
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

// BreakGlassConfig holds emergency access configuration.
// Break glass provides controlled emergency access to the database
// for disaster recovery and debugging scenarios.
type BreakGlassConfig struct {
	// Enabled controls whether break glass access is available.
	// Must be explicitly enabled and requires bibd restart.
	Enabled bool `mapstructure:"enabled"`

	// RequireRestart requires bibd restart to enable/disable break glass.
	// This prevents runtime configuration changes for security.
	RequireRestart bool `mapstructure:"require_restart"`

	// MaxDuration is the maximum allowed duration for a break glass session.
	// Sessions auto-expire after this duration.
	MaxDuration time.Duration `mapstructure:"max_duration"`

	// DefaultAccessLevel is the default access level for break glass sessions.
	// Can be "readonly" or "readwrite".
	DefaultAccessLevel string `mapstructure:"default_access_level"`

	// AllowedUsers is a list of pre-configured emergency access users.
	AllowedUsers []BreakGlassUser `mapstructure:"allowed_users"`

	// AuditLevel controls the verbosity of break glass audit logging.
	// "normal" redacts sensitive values, "paranoid" logs everything.
	AuditLevel string `mapstructure:"audit_level"`

	// Notification holds notification configuration for break glass events.
	Notification BreakGlassNotification `mapstructure:"notification"`

	// RequireAcknowledgment requires admin acknowledgment after session ends.
	RequireAcknowledgment bool `mapstructure:"require_acknowledgment"`

	// SessionRecording enables terminal session recording.
	SessionRecording bool `mapstructure:"session_recording"`

	// RecordingPath is where session recordings are stored.
	// Defaults to the same location as audit logs.
	RecordingPath string `mapstructure:"recording_path"`
}

// BreakGlassUser represents a pre-configured emergency access user.
type BreakGlassUser struct {
	// Name is the username for the emergency user.
	Name string `mapstructure:"name"`

	// PublicKey is the SSH Ed25519 public key for authentication.
	// Format: "ssh-ed25519 AAAA..."
	PublicKey string `mapstructure:"public_key"`

	// AccessLevel overrides the default access level for this user.
	// Can be "readonly" or "readwrite". Empty means use default.
	AccessLevel string `mapstructure:"access_level,omitempty"`
}

// BreakGlassNotification holds notification configuration for break glass events.
type BreakGlassNotification struct {
	// Webhook is the URL to send webhook notifications to.
	Webhook string `mapstructure:"webhook"`

	// Email is the email address to send notifications to.
	Email string `mapstructure:"email"`
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
			Format: "table",
			Color:  true,
		},
		Server: "localhost:8080",
		Connection: ConnectionConfig{
			FavoriteNodes: []FavoriteNode{},
			AutoDetect:    true,
			Timeout:       "30s",
			RetryAttempts: 3,
		},
	}
}

// getDefaultDataDir returns a platform-appropriate default data directory
func getDefaultDataDir() string {
	if runtime.GOOS == "windows" {
		// On Windows, use %LOCALAPPDATA%\bibd or fallback to ~\.bibd\data
		return "~/AppData/Local/bibd"
	}
	// On Unix-like systems, use XDG Base Directory specification
	return "~/.local/share/bibd"
}

// DefaultBibdConfig returns the default bibd configuration.
func DefaultBibdConfig() BibdConfig {
	// On Windows, we can't use Unix sockets for PostgreSQL connections from host to Docker container
	useUnixSocket := runtime.GOOS != "windows"

	return BibdConfig{
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
			PIDFile: "~/bibd.pid",
			DataDir: getDefaultDataDir(),
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
				Managed:                    true,
				ContainerRuntime:           "", // Auto-detect
				SocketPath:                 "", // Auto-detect
				KubeconfigPath:             "",
				Image:                      "postgres:16-alpine",
				DataDir:                    "", // defaults to <data_dir>/postgres
				Port:                       5432,
				MaxConnections:             100,
				MemoryMB:                   512,
				CPUCores:                   1.0,
				SSLMode:                    "require",
				CredentialRotationInterval: 7 * 24 * time.Hour, // 7 days
				Network: PostgresNetworkConfig{
					UseBridgeNetwork:  true,
					BridgeNetworkName: "bibd-network",
					UseUnixSocket:     useUnixSocket,
					BindAddress:       "127.0.0.1",
				},
				Health: PostgresHealthConfig{
					Interval:       5 * time.Second,
					Timeout:        5 * time.Second,
					StartupTimeout: 60 * time.Second,
					Action:         "retry_limit",
					MaxRetries:     5,
					RetryBackoff:   10 * time.Second,
				},
				TLS: PostgresTLSConfig{
					Enabled:      true,
					CertDir:      "", // defaults to <data_dir>/postgres/certs
					AutoGenerate: true,
				},
			},
			Audit: AuditDatabaseConfig{
				Enabled:       true,
				RetentionDays: 90,
				HashChain:     true,
			},
			BreakGlass: BreakGlassConfig{
				Enabled:               false, // Disabled by default for security
				RequireRestart:        true,  // Must restart bibd to enable
				MaxDuration:           1 * time.Hour,
				DefaultAccessLevel:    "readonly",
				AllowedUsers:          []BreakGlassUser{},
				AuditLevel:            "paranoid", // Log everything during break glass
				RequireAcknowledgment: true,
				SessionRecording:      true,
				RecordingPath:         "", // Defaults to audit log path
				Notification: BreakGlassNotification{
					Webhook: "",
					Email:   "",
				},
			},
		},
	}
}
