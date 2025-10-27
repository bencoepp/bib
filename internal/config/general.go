package config

type GeneralConfig struct {
	CheckCapabilities bool   `mapstructure:"check_capabilities" yaml:"check_capabilities"`
	IdentityPath      string `mapstructure:"identity_path" yaml:"identity_path"`
}
