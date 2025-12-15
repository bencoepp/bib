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
	}

	return v
}
