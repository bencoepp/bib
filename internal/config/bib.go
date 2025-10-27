package config

import (
	"time"

	"github.com/spf13/viper"
)

type BibConfig struct {
	General GeneralConfig `mapstructure:"general" yaml:"general"`
	Update  UpdateConfig  `mapstructure:"update" yaml:"update"`
}

func ApplyBibDefaults(v *viper.Viper) {
	v.SetDefault("general", map[string]any{
		"theme": "auto",
	})

	v.SetDefault("update", map[string]any{
		"enabled":             true,
		"github_owner":        "bencoepp",
		"github_repo":         "bib",
		"allow_prerelease":    false,
		"http_timeout_in_sec": 30 * time.Second,
	})
}

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
