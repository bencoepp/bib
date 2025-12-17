package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// SupportedFormats lists the config file formats we support
var SupportedFormats = []string{"yaml", "toml", "json"}

// GenerateConfig creates a default configuration file for the specified app
func GenerateConfig(appName, format string) (string, error) {
	if !isValidFormat(format) {
		return "", fmt.Errorf("unsupported format %q, supported: %v", format, SupportedFormats)
	}

	// Get the user config directory
	configDir, err := UserConfigDir(appName)
	if err != nil {
		return "", err
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, fmt.Sprintf("config.%s", format))

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		return configPath, fmt.Errorf("config file already exists: %s", configPath)
	}

	// Get default config
	var defaultCfg interface{}
	switch appName {
	case AppBib:
		cfg := DefaultBibConfig()
		defaultCfg = &cfg
	case AppBibd:
		cfg := DefaultBibdConfig()
		defaultCfg = &cfg
	default:
		return "", fmt.Errorf("unknown app: %s", appName)
	}

	// Create viper instance with the config
	v := NewViperFromConfig(appName, defaultCfg)
	v.SetConfigType(format)

	// Write the config file
	if err := v.WriteConfigAs(configPath); err != nil {
		return "", fmt.Errorf("failed to write config file: %w", err)
	}

	return configPath, nil
}

// GenerateConfigIfNotExists creates a default config file if one doesn't exist
// Returns the path to the config file (existing or newly created) and whether it was created
func GenerateConfigIfNotExists(appName, format string) (string, bool, error) {
	configDir, err := UserConfigDir(appName)
	if err != nil {
		return "", false, err
	}

	// Check for existing config files in any format
	for _, ext := range SupportedFormats {
		path := filepath.Join(configDir, fmt.Sprintf("config.%s", ext))
		if _, err := os.Stat(path); err == nil {
			return path, false, nil
		}
	}

	// No config exists, generate one
	path, err := GenerateConfig(appName, format)
	if err != nil {
		return "", false, err
	}

	return path, true, nil
}

func isValidFormat(format string) bool {
	for _, f := range SupportedFormats {
		if f == format {
			return true
		}
	}
	return false
}
