package config

import "github.com/spf13/viper"

type BibdConfig struct {
	General GeneralConfig `mapstructure:"general"`
}

func DefaultsBibd(v *viper.Viper) {
	DefaultsGeneral(v)
}

func LoadBibd(v *viper.Viper) (*BibdConfig, error) {
	var cfg BibdConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
