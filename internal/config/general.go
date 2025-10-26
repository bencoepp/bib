package config

import "github.com/spf13/viper"

type GeneralConfig struct {
	LogLevel                 string `mapstructure:"log_level"` // debug|info|warn|error
	Theme                    string `mapstructure:"theme"`     // auto|light|dark
	AutoRegisterCapabilities bool   `mapstructure:"auto_register_capabilities"`
}

// DefaultsGeneral sets defaults for shared "general" settings.
func DefaultsGeneral(v *viper.Viper) {
	v.SetDefault("general.log_level", "info")
	v.SetDefault("general.theme", "auto")
	v.SetDefault("general.auto_register_capabilities", true)
}
