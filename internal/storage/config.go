package storage

import (
	"time"
)

// Config holds the storage configuration.
type Config struct {
	// Backend is the storage backend type: "sqlite" or "postgres"
	Backend BackendType `mapstructure:"backend"`

	// SQLite configuration (used when Backend is "sqlite")
	SQLite SQLiteConfig `mapstructure:"sqlite"`

	// Postgres configuration (used when Backend is "postgres")
	Postgres PostgresConfig `mapstructure:"postgres"`

	// Audit configuration
	Audit AuditConfig `mapstructure:"audit"`

	// BreakGlass emergency access configuration
	BreakGlass BreakGlassConfig `mapstructure:"break_glass"`
}

// SQLiteConfig holds SQLite-specific configuration.
type SQLiteConfig struct {
	// Path is the path to the SQLite database file.
	// Defaults to <data_dir>/cache.db
	Path string `mapstructure:"path"`

	// MaxOpenConns is the maximum number of open connections.
	MaxOpenConns int `mapstructure:"max_open_conns"`

	// CacheTTL is the TTL for cached data.
	CacheTTL time.Duration `mapstructure:"cache_ttl"`

	// MaxCacheSizeMB is the maximum cache size in megabytes.
	MaxCacheSizeMB int `mapstructure:"max_cache_size_mb"`

	// VacuumInterval is how often to run VACUUM.
	VacuumInterval time.Duration `mapstructure:"vacuum_interval"`
}

// PostgresConfig holds PostgreSQL-specific configuration.
// Note: bibd manages the PostgreSQL instance; no external connection strings.
type PostgresConfig struct {
	// Managed indicates whether bibd manages the PostgreSQL lifecycle.
	// When true, bibd provisions and manages a PostgreSQL container.
	Managed bool `mapstructure:"managed"`

	// ContainerRuntime is the container runtime to use: "docker", "podman", or "kubernetes"
	// If empty, auto-detected (docker > podman > kubernetes)
	ContainerRuntime string `mapstructure:"container_runtime"`

	// SocketPath is the path to the container runtime socket
	// Auto-detected if empty
	SocketPath string `mapstructure:"socket_path"`

	// KubeconfigPath is the path to kubeconfig file (for Kubernetes runtime)
	KubeconfigPath string `mapstructure:"kubeconfig_path"`

	// Image is the PostgreSQL container image.
	// Defaults to "postgres:16-alpine"
	Image string `mapstructure:"image"`

	// DataDir is where PostgreSQL data is stored.
	// Defaults to <data_dir>/postgres
	DataDir string `mapstructure:"data_dir"`

	// Port is the PostgreSQL port (internal to the container network).
	// Defaults to 5432 (not exposed externally)
	Port int `mapstructure:"port"`

	// MaxConnections is the maximum number of connections.
	MaxConnections int `mapstructure:"max_connections"`

	// SharedBuffers is the PostgreSQL shared_buffers setting.
	SharedBuffers string `mapstructure:"shared_buffers"`

	// EffectiveCacheSize is the PostgreSQL effective_cache_size setting.
	EffectiveCacheSize string `mapstructure:"effective_cache_size"`

	// CredentialRotationInterval is how often to rotate database credentials.
	CredentialRotationInterval time.Duration `mapstructure:"credential_rotation_interval"`

	// SSLMode is the SSL mode for connections.
	// Options: "disable", "require", "verify-ca", "verify-full"
	SSLMode string `mapstructure:"ssl_mode"`

	// ResourceLimits for the container.
	Resources ContainerResources `mapstructure:"resources"`

	// Network configuration
	Network NetworkConfig `mapstructure:"network"`

	// Health check configuration
	Health HealthConfig `mapstructure:"health"`

	// TLS configuration for PostgreSQL connections
	TLS TLSConfig `mapstructure:"tls"`

	// Advanced allows manual connection configuration (for debugging only).
	// When set, Managed must be false.
	Advanced *AdvancedPostgresConfig `mapstructure:"advanced,omitempty"`
}

// NetworkConfig holds network configuration for managed PostgreSQL.
type NetworkConfig struct {
	// UseBridgeNetwork creates a private bridge network for isolation
	UseBridgeNetwork bool `mapstructure:"use_bridge_network"`

	// BridgeNetworkName is the name of the bridge network
	BridgeNetworkName string `mapstructure:"bridge_network_name"`

	// UseUnixSocket uses Unix socket only (no TCP)
	UseUnixSocket bool `mapstructure:"use_unix_socket"`

	// BindAddress is the address to bind to (default: 127.0.0.1)
	BindAddress string `mapstructure:"bind_address"`
}

// HealthConfig holds health check configuration for managed PostgreSQL.
type HealthConfig struct {
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

// TLSConfig holds TLS configuration for PostgreSQL connections.
type TLSConfig struct {
	// Enabled controls whether mTLS is enabled (always true for managed)
	Enabled bool `mapstructure:"enabled"`

	// CertDir is where certificates are stored
	CertDir string `mapstructure:"cert_dir"`

	// AutoGenerate automatically generates certificates from node identity
	AutoGenerate bool `mapstructure:"auto_generate"`
}

// ContainerResources defines resource limits for the PostgreSQL container.
type ContainerResources struct {
	// MemoryMB is the memory limit in megabytes.
	MemoryMB int `mapstructure:"memory_mb"`

	// CPUCores is the CPU limit (can be fractional).
	CPUCores float64 `mapstructure:"cpu_cores"`
}

// AdvancedPostgresConfig allows manual PostgreSQL configuration.
// This should only be used for debugging and testing.
type AdvancedPostgresConfig struct {
	// Host is the PostgreSQL host.
	Host string `mapstructure:"host"`

	// Port is the PostgreSQL port.
	Port int `mapstructure:"port"`

	// Database is the database name.
	Database string `mapstructure:"database"`

	// User is the database user.
	User string `mapstructure:"user"`

	// Password is the database password.
	// WARNING: This is stored in plaintext. Use only for testing.
	Password string `mapstructure:"password"`

	// SSLMode is the SSL mode.
	SSLMode string `mapstructure:"ssl_mode"`
}

// AuditConfig holds audit logging configuration.
type AuditConfig struct {
	// Enabled controls whether audit logging is active.
	Enabled bool `mapstructure:"enabled"`

	// RetentionDays is how long to keep audit logs.
	RetentionDays int `mapstructure:"retention_days"`

	// HashChain enables hash chain for tamper detection.
	HashChain bool `mapstructure:"hash_chain"`

	// StreamToExternal enables streaming to external SIEM.
	StreamToExternal bool `mapstructure:"stream_to_external"`

	// ExternalEndpoint is the endpoint for external streaming.
	ExternalEndpoint string `mapstructure:"external_endpoint,omitempty"`
}

// BreakGlassConfig holds emergency access configuration.
type BreakGlassConfig struct {
	// Enabled allows break glass access to be activated.
	// Must be explicitly enabled in config.
	Enabled bool `mapstructure:"enabled"`

	// RequireRestart requires bibd restart to enable break glass.
	RequireRestart bool `mapstructure:"require_restart"`

	// MaxDuration is the maximum duration for a break glass session.
	MaxDuration time.Duration `mapstructure:"max_duration"`

	// AllowedUsers are pre-configured emergency access users.
	AllowedUsers []BreakGlassUser `mapstructure:"allowed_users"`

	// AuditLevel is the audit level during break glass ("normal" or "paranoid").
	AuditLevel string `mapstructure:"audit_level"`

	// NotificationWebhook is called when break glass is activated.
	NotificationWebhook string `mapstructure:"notification_webhook,omitempty"`

	// NotificationEmail is emailed when break glass is activated.
	NotificationEmail string `mapstructure:"notification_email,omitempty"`
}

// BreakGlassUser represents an allowed emergency access user.
type BreakGlassUser struct {
	// Name is the user name.
	Name string `mapstructure:"name"`

	// PublicKey is the SSH public key for authentication.
	PublicKey string `mapstructure:"public_key"`
}

// DefaultConfig returns the default storage configuration.
func DefaultConfig() Config {
	return Config{
		Backend: BackendSQLite, // Default to SQLite for easy onboarding
		SQLite: SQLiteConfig{
			Path:           "", // Defaults to <data_dir>/cache.db
			MaxOpenConns:   10,
			CacheTTL:       5 * time.Minute,
			MaxCacheSizeMB: 500,
			VacuumInterval: 24 * time.Hour,
		},
		Postgres: PostgresConfig{
			Managed:                    true,
			ContainerRuntime:           "", // Auto-detect
			SocketPath:                 "", // Auto-detect
			Image:                      "postgres:16-alpine",
			DataDir:                    "", // Defaults to <data_dir>/postgres
			Port:                       5432,
			MaxConnections:             100,
			SharedBuffers:              "256MB",
			EffectiveCacheSize:         "1GB",
			CredentialRotationInterval: 7 * 24 * time.Hour, // 7 days
			SSLMode:                    "require",
			Resources: ContainerResources{
				MemoryMB: 512,
				CPUCores: 1.0,
			},
			Network: NetworkConfig{
				UseBridgeNetwork:  true,
				BridgeNetworkName: "bibd-network",
				UseUnixSocket:     true,
				BindAddress:       "127.0.0.1",
			},
			Health: HealthConfig{
				Interval:       5 * time.Second,
				Timeout:        5 * time.Second,
				StartupTimeout: 60 * time.Second,
				Action:         "retry_limit",
				MaxRetries:     5,
				RetryBackoff:   10 * time.Second,
			},
			TLS: TLSConfig{
				Enabled:      true,
				AutoGenerate: true,
			},
		},
		Audit: AuditConfig{
			Enabled:          true,
			RetentionDays:    90,
			HashChain:        true,
			StreamToExternal: false,
		},
		BreakGlass: BreakGlassConfig{
			Enabled:        false,
			RequireRestart: true,
			MaxDuration:    1 * time.Hour,
			AuditLevel:     "paranoid",
		},
	}
}

// Validate validates the storage configuration.
func (c *Config) Validate() error {
	switch c.Backend {
	case BackendSQLite:
		// SQLite config validation
	case BackendPostgres:
		if c.Postgres.Advanced != nil && c.Postgres.Managed {
			return ErrInvalidInput
		}
	default:
		return ErrInvalidInput
	}
	return nil
}
