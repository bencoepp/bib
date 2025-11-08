package config

import (
	"bib/internal/config/util"
	"errors"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
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

func DefaultBibConfig() *BibConfig {
	viper.SetConfigType("yaml")
	ApplyBibDefaults(viper.GetViper())

	var cfg BibConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		return &BibConfig{}
	}

	return &cfg
}

func SaveBibConfigTo(path string, cfg *BibConfig) error {
	if cfg == nil {
		return errors.New("config cannot be nil")
	}
	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	// Restrictive perms on Unix; Windows ACLs are used on Windows.
	return os.WriteFile(path, data, 0o600)
}

func SaveUpdatedBibConfig(cfg *BibConfig) (string, error) {
	if cfg == nil {
		return "", errors.New("config cannot be nil")
	}

	// Prefer the file already associated with the current viper instance (e.g., via LoadBibConfig).
	if existing := viper.ConfigFileUsed(); existing != "" {
		return existing, SaveBibConfigTo(existing, cfg)
	}

	// Otherwise, resolve the default config path and write there.
	path, err := util.GetDefaultConfigPath("bib")
	if err != nil {
		return "", err
	}
	return path, SaveBibConfigTo(path, cfg)
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
