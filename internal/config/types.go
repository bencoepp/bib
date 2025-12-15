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
		},
	}
}
