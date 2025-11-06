package config

import (
	"bib/internal/config/util"

	"github.com/spf13/viper"
)

type BibDaemonConfig struct {
	General GeneralConfig `yaml:"general"`
	Update  UpdateConfig  `yaml:"update"`
}

func ApplyBibDaemonDefaults(v *viper.Viper) {
	v.SetDefault("general", map[string]any{
		"check_capabilities": true,
	})

	v.SetDefault("update", map[string]any{
		"enabled":             true,
		"github_owner":        "bencoepp",
		"github_repo":         "bib",
		"allow_prerelease":    false,
		"http_timeout_in_sec": 30,
	})
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
