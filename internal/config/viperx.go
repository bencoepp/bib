package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Init sets up a Viper instance for the given app name and optional explicit config file path.
// It configures default search paths, AutomaticEnv with a prefix, and reads the config file if present.
// Returns the viper instance, the config file actually used (if any), and an error if reading fails for reasons other than "not found".
func Init(appName string, explicitFile string) (*viper.Viper, string, error) {
	v := viper.New()

	v.SetConfigType("yaml")
	v.SetEnvPrefix(strings.ToUpper(appName)) // e.g., BIB, BIBD
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	if explicitFile != "" {
		v.SetConfigFile(explicitFile)
	} else {
		// Search locations, in order:
		// $XDG_CONFIG_HOME/<app>/
		// $HOME/.config/<app>/
		// $HOME/.<app>/
		// ./
		v.SetConfigName(appName)
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			v.AddConfigPath(filepath.Join(xdg, appName))
		}
		if home, err := os.UserHomeDir(); err == nil {
			v.AddConfigPath(filepath.Join(home, ".config", appName))
			v.AddConfigPath(filepath.Join(home, "."+appName))
		}
		v.AddConfigPath(".")
	}

	// ReadInConfig if a file is specified or found, but ignore "not found" errors
	if err := v.ReadInConfig(); err != nil {
		var cfgUsed string
		if _, notFound := err.(viper.ConfigFileNotFoundError); notFound {
			return v, cfgUsed, nil
		}
		return v, "", err
	}

	return v, v.ConfigFileUsed(), nil
}
