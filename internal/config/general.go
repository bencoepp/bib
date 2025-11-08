package config

type GeneralConfig struct {
	Theme             string `mapstructure:"theme" yaml:"theme"`
	CheckCapabilities bool   `mapstructure:"check_capabilities" yaml:"check_capabilities"`
	CheckLocation     bool   `mapstructure:"check_location" yaml:"check_location"`
	IdentityPath      string `mapstructure:"identity_path" yaml:"identity_path"`
	RetrieveLocation  bool   `mapstructure:"retrieve_location" yaml:"retrieve_location"`
	UsePassphrase     bool   `mapstructure:"use_passphrase" yaml:"use_passphrase"`
	UseSecondFactor   bool   `mapstructure:"use_second_factor" yaml:"use_second_factor"`
}
