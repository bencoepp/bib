package config

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
	}
}
