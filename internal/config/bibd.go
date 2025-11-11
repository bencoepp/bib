package config

import (
	"bib/internal/config/util"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type BibDaemonConfig struct {
	General GeneralConfig `mapstructure:"general" yaml:"general"`
	Update  UpdateConfig  `mapstructure:"update" yaml:"update"`
	P2P     P2PConfig     `mapstructure:"p2p" yaml:"p2p"`
	Port    int           `mapstructure:"port" yaml:"port"`
}

func ApplyBibDaemonDefaults(v *viper.Viper) {
	path, err := os.UserHomeDir()
	if err != nil {
		path = "."
	}

	v.SetDefault("general", map[string]interface{}{
		"theme":              "auto",
		"check_capabilities": true,
		"identity_path":      filepath.Join(path, ".bibd"),
		"retrieve_location":  true,
		"use_passphrase":     false,
		"use_second_factor":  false,
	})

	v.SetDefault("update", map[string]any{
		"enabled":             true,
		"github_owner":        "bencoepp",
		"github_repo":         "bib",
		"allow_prerelease":    false,
		"http_timeout_in_sec": 30,
	})

	v.SetDefault("p2p", map[string]any{
		"listen_addresses": []string{
			"/ip4/0.0.0.0/tcp/4001",
			"/ip4/0.0.0.0/udp/4002/quic-v1",
		},

		"discovery": map[string]any{
			"rendezvous":                "bib-network",
			"enable_mdns":               true,
			"mdns_service_tag":          "bib-mdns",
			"dht_server":                false,
			"advertise_interval":        300,
			"skip_mdns_if_no_multicast": false,
			"require_mdns":              false,
		},
		"rtt": map[string]any{
			"enable_rtt_probing": true,
			"interval":           300,
			"concurrency":        3,
			"pings_per_peer":     5,
			"connect_timeout":    10,
			"ping_timeout":       5,
		},
		"preferences": map[string]any{
			"region":              "auto",
			"tags":                []string{},
			"weight_latency":      0.5,
			"weight_load":         0.3,
			"weight_region":       0.1,
			"weight_tags":         0.1,
			"min_samples_for_rtt": 5,
		},
	})

	v.SetDefault("port", 50051)
	v.SetDefault("enable_local_grpc", true)
	v.SetDefault("enable_reflection", true)
}

// SaveBibDaemonConfig saves the configuration for the Bib Daemon using the extracted SaveConfig logic.
func SaveBibDaemonConfig() (string, error) {
	viper.SetConfigType("yaml")
	ApplyBibDaemonDefaults(viper.GetViper())

	// Use SaveConfig from save_config.go
	return util.SaveConfig("bibd")
}

// LoadBibDaemonConfig loads the configuration for the Bib Daemon.
func LoadBibDaemonConfig(path string) (*BibDaemonConfig, error) {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	ApplyBibDaemonDefaults(viper.GetViper())

	var cfg BibDaemonConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
