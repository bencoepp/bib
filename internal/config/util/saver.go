package util

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/viper"
)

var ErrNoDefaultConfigPath = errors.New("no default config path available")

// SaveConfig saves the configuration to the user's home directory or system-wide directory, based on the OS.
// 1. User's home directory: ~/.<appname>/config.yaml
// 2. System-wide directory:
//   - /etc/<appname>/config.yaml (Linux/Unix)
//   - /Library/Application\ Support/<AppName>/config.yaml (macOS)
//   - %PROGRAMDATA%\<AppName>\config.yaml (Windows)
func SaveConfig(appName string) (string, error) {
	if appName == "" {
		return "", errors.New("app name must be provided")
	}

	viper.SetConfigType("yaml")

	// Attempt to save to the user's home directory under `.appName`
	if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
		homeAppDir := filepath.Join(homeDir, "."+appName)
		if ensureWritableDir(homeAppDir) {
			configPath := filepath.Join(homeAppDir, "config.yaml")
			viper.SetConfigFile(configPath)
			return writeConfig(configPath)
		}
	}

	// If saving to the home directory fails, attempt to save to the system-wide directory
	systemDir := getSystemConfigDir(appName)
	if ensureWritableDir(systemDir) {
		configPath := filepath.Join(systemDir, "config.yaml")
		viper.SetConfigFile(configPath)
		return writeConfig(configPath)
	}

	return "", ErrNoDefaultConfigPath
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
