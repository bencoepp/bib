package config

type UpdateConfig struct {
	Enabled          bool   `mapstructure:"enabled" yaml:"enabled"`
	GitHubOwner      string `mapstructure:"github_owner" yaml:"github_owner"`
	GitHubRepo       string `mapstructure:"github_repo" yaml:"github_repo"`
	AllowPrerelease  bool   `mapstructure:"allow_prerelease" yaml:"allow_prerelease"`
	HTTPTimeoutInSec int    `mapstructure:"http_timeout_in_sec" yaml:"http_timeout_in_sec"`
}
