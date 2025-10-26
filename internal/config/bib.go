package config

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/viper"
)

type BibConfig struct {
	Theme string `mapstructure:"theme"` // auto|light|dark
}

func DefaultsBib(v *viper.Viper) {
	v.SetDefault("theme", "auto")
}

func LoadBib(v *viper.Viper) (*BibConfig, error) {
	var cfg BibConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func defaultDataDir(app string) string {
	// Respect XDG_DATA_HOME on Unix, fallback to ~/.local/share/<app>
	// On Windows, use %AppData%\<app>\data
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("AppData"); appData != "" {
			return filepath.Join(appData, app, "data")
		}
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, app)
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share", app)
	}
	return "." // fallback
}
