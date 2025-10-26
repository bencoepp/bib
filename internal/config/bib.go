package config

import "github.com/spf13/viper"

type BibConfig struct {
	General GeneralConfig `yaml:"general"`
}

func LoadBibConfig(path string) (*BibConfig, error) {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg BibConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
