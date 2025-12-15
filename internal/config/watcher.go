package config

import (
	"fmt"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// ConfigWatcher watches for configuration file changes and triggers callbacks.
type ConfigWatcher struct {
	v           *viper.Viper
	appName     string
	cfgFile     string
	mu          sync.RWMutex
	callbacks   []func(interface{})
	lastConfig  interface{}
	stopChan    chan struct{}
	stoppedChan chan struct{}
}

// NewConfigWatcher creates a new configuration watcher.
func NewConfigWatcher(appName, cfgFile string) (*ConfigWatcher, error) {
	v := newViper(appName)

	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	}

	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if err, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil, fmt.Errorf("config file not found: %w", err)
		}
		_ = configFileNotFoundError
	}

	return &ConfigWatcher{
		v:           v,
		appName:     appName,
		cfgFile:     cfgFile,
		callbacks:   []func(interface{}){},
		stopChan:    make(chan struct{}),
		stoppedChan: make(chan struct{}),
	}, nil
}

// OnChange registers a callback to be called when configuration changes.
// The callback receives the new configuration struct.
func (cw *ConfigWatcher) OnChange(callback func(interface{})) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.callbacks = append(cw.callbacks, callback)
}

// Start begins watching for configuration changes.
func (cw *ConfigWatcher) Start() error {
	cw.v.OnConfigChange(func(e fsnotify.Event) {
		cw.handleChange()
	})

	cw.v.WatchConfig()

	return nil
}

// Stop stops watching for configuration changes.
func (cw *ConfigWatcher) Stop() {
	close(cw.stopChan)
	<-cw.stoppedChan
}

// handleChange is called when configuration changes.
func (cw *ConfigWatcher) handleChange() {
	cw.mu.RLock()
	callbacks := make([]func(interface{}), len(cw.callbacks))
	copy(callbacks, cw.callbacks)
	cw.mu.RUnlock()

	// Load the new configuration
	var cfg interface{}
	var err error

	switch cw.appName {
	case AppBib:
		cfg, err = cw.loadBibConfig()
	case AppBibd:
		cfg, err = cw.loadBibdConfig()
	}

	if err != nil {
		// Log error but don't crash
		return
	}

	// Call all callbacks
	for _, cb := range callbacks {
		cb(cfg)
	}

	cw.mu.Lock()
	cw.lastConfig = cfg
	cw.mu.Unlock()
}

// loadBibConfig loads BibConfig from viper.
func (cw *ConfigWatcher) loadBibConfig() (*BibConfig, error) {
	defaults := DefaultBibConfig()
	setViperDefaults(cw.v, defaults)

	var cfg BibConfig
	if err := cw.v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := resolveSecrets(&cfg); err != nil {
		return nil, fmt.Errorf("failed to resolve secrets: %w", err)
	}

	return &cfg, nil
}

// loadBibdConfig loads BibdConfig from viper.
func (cw *ConfigWatcher) loadBibdConfig() (*BibdConfig, error) {
	defaults := DefaultBibdConfig()
	setViperDefaults(cw.v, defaults)

	var cfg BibdConfig
	if err := cw.v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := resolveSecrets(&cfg); err != nil {
		return nil, fmt.Errorf("failed to resolve secrets: %w", err)
	}

	return &cfg, nil
}

// CurrentConfig returns the last loaded configuration.
func (cw *ConfigWatcher) CurrentConfig() interface{} {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	return cw.lastConfig
}

// Reload forces a configuration reload.
func (cw *ConfigWatcher) Reload() error {
	if err := cw.v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to reload config: %w", err)
	}
	cw.handleChange()
	return nil
}
