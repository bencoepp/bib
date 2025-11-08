package util

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/viper"
)

var ErrNoDefaultConfigPath = errors.New("no default config path available")

// GetDefaultConfigPath resolves the default configuration file path for the given
// app name without writing anything to disk. It ensures the directory exists and
// is writable, and returns the full path to config.yaml or an error if no
// suitable location is available.
//
// Resolution order:
// 1. User's home directory: ~/.<appname>/config.yaml
// 2. System-wide directory:
//   - /etc/<appname>/config.yaml (Linux/Unix)
//   - ~/Library/Application Support/<AppName>/config.yaml (macOS)
//   - %PROGRAMDATA%\<AppName>\config.yaml (Windows)
func GetDefaultConfigPath(appName string) (string, error) {
	if appName == "" {
		return "", errors.New("app name must be provided")
	}

	// Try user's home directory under `.appName`
	if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
		homeAppDir := filepath.Join(homeDir, "."+appName)
		if ensureWritableDir(homeAppDir) {
			return filepath.Join(homeAppDir, "config.yaml"), nil
		}
	}

	// Fall back to system-wide directory
	systemDir := getSystemConfigDir(appName)
	if ensureWritableDir(systemDir) {
		return filepath.Join(systemDir, "config.yaml"), nil
	}

	return "", ErrNoDefaultConfigPath
}

// SaveConfig saves the current viper configuration to the resolved default path.
// It uses GetDefaultConfigPath to resolve the path and then writes via viper.
func SaveConfig(appName string) (string, error) {
	if appName == "" {
		return "", errors.New("app name must be provided")
	}

	viper.SetConfigType("yaml")

	configPath, err := GetDefaultConfigPath(appName)
	if err != nil {
		return "", err
	}

	viper.SetConfigFile(configPath)
	return writeConfig(configPath)
}

// writeConfig writes the configuration to the given path.
func writeConfig(path string) (string, error) {
	if err := viper.WriteConfig(); err != nil {
		return "", err
	}
	return path, nil
}

// getSystemConfigDir returns the appropriate system-wide configuration directory based on the OS.
func getSystemConfigDir(appName string) string {
	switch runtime.GOOS {
	case "darwin":
		// macOS: ~/Library/Application Support/<AppName>/
		if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
			return filepath.Join(homeDir, "Library", "Application Support", appName)
		}
	case "windows":
		// Windows: %PROGRAMDATA%\<AppName>\
		if programData := os.Getenv("PROGRAMDATA"); programData != "" {
			return filepath.Join(programData, appName)
		}
	default:
		// Linux/Unix: /etc/<appname>/
		return filepath.Join(string(filepath.Separator), "etc", appName)
	}

	return ""
}

// ensureWritableDir ensures the directory exists and is writable.
func ensureWritableDir(path string) bool {
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return false
	}
	return isWritable(path)
}

// isWritable checks if a directory is writable.
func isWritable(path string) bool {
	testFile := filepath.Join(path, ".test")
	if err := os.WriteFile(testFile, []byte{}, 0644); err != nil {
		return false
	}
	_ = os.Remove(testFile) // Cleanup the test file
	return true
}
