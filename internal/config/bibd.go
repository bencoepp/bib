package config

import (
	"github.com/spf13/viper"
)

type BibDaemonConfig struct {
	General GeneralConfig `yaml:"general"`
}

func LoadBibDaemonConfig(path string) (*BibDaemonConfig, error) {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg BibDaemonConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
