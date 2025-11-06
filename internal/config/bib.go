package config

import (
	"bib/internal/config/util"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type BibConfig struct {
	General GeneralConfig `mapstructure:"general" yaml:"general"`
	Update  UpdateConfig  `mapstructure:"update" yaml:"update"`
}

func ApplyBibDefaults(v *viper.Viper) {
	path, err := os.UserHomeDir()
	if err != nil {
		path = "."
	}

	v.SetDefault("general", map[string]interface{}{
		"theme":              "auto",
		"check_capabilities": true,
		"identity_path":      filepath.Join(path, ".bib", "identity.json"),
		"retrieve_location":  true,
		"use_passphrase":     true,
		"use_second_factor":  true,
	})

	v.SetDefault("update", map[string]interface{}{
		"enabled":             true,
		"github_owner":        "bencoepp",
		"github_repo":         "bib",
		"allow_prerelease":    false,
		"http_timeout_in_sec": 30,
	})
}

// SaveBibConfig saves the configuration for the Bib application using the extracted SaveConfig logic.
func SaveBibConfig() (string, error) {
	viper.SetConfigType("yaml")
	ApplyBibDefaults(viper.GetViper())

	// Use SaveConfig from save_config.go
	return util.SaveConfig("bib")
}

// LoadBibConfig loads the configuration for the Bib application.
func LoadBibConfig(path string) (*BibConfig, error) {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	ApplyBibDefaults(viper.GetViper())

	var cfg BibConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
