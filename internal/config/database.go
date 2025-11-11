package config

type DatabaseConfig struct {
	Host               string `mapstructure:"host" yaml:"host"`
	Port               int    `mapstructure:"port" yaml:"port"`
	User               string `mapstructure:"user" yaml:"user"`
	Password           string `mapstructure:"password" yaml:"password"`
	DBName             string `mapstructure:"dbname" yaml:"dbname"`
	SSLMode            string `mapstructure:"sslmode" yaml:"sslmode"`
	MaxOpenConnections int    `mapstructure:"max_open_connections" yaml:"max_open_connections"`
	MaxIdleConnections int    `mapstructure:"max_idle_connections" yaml:"max_idle_connections"`
}
