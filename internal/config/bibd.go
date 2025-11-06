package config

import (
	"bib/internal/config/util"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type BibDaemonConfig struct {
	General GeneralConfig `yaml:"general"`
	Update  UpdateConfig  `yaml:"update"`
	Port    int           `yaml:"port"`
}

func ApplyBibDaemonDefaults(v *viper.Viper) {
	path, err := os.UserHomeDir()
	if err != nil {
		path = "."
	}

	v.SetDefault("general", map[string]interface{}{
		"theme":              "auto",
		"check_capabilities": true,
		"identity_path":      filepath.Join(path, ".bib", "identity.json"),
		"retrieve_location":  true,
		"use_passphrase":     false,
		"use_second_factor":  false,
	})

	v.SetDefault("update", map[string]any{
		"enabled":             true,
		"github_owner":        "bencoepp",
		"github_repo":         "bib",
		"allow_prerelease":    false,
		"http_timeout_in_sec": 30,
	})

	v.SetDefault("port", 50051)
}

// SaveBibDaemonConfig saves the configuration for the Bib Daemon using the extracted SaveConfig logic.
func SaveBibDaemonConfig() (string, error) {
	viper.SetConfigType("yaml")
	ApplyBibDaemonDefaults(viper.GetViper())

	// Use SaveConfig from save_config.go
	return util.SaveConfig("bibd")
}

// LoadBibDaemonConfig loads the configuration for the Bib Daemon.
func LoadBibDaemonConfig(path string) (*BibDaemonConfig, error) {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	ApplyBibDaemonDefaults(viper.GetViper())

	var cfg BibDaemonConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
