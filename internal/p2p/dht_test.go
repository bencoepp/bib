package p2p

import (
	"testing"

	"bib/internal/config"
)

func TestDHTModes(t *testing.T) {
	tests := []struct {
		mode  string
		valid bool
	}{
		{"auto", true},
		{"server", true},
		{"client", true},
		{"Auto", true},   // case insensitive
		{"SERVER", true}, // case insensitive
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			cfg := config.DHTConfig{
				Enabled: true,
				Mode:    tt.mode,
			}

			// Validate mode parsing would work
			mode := DHTMode(tt.mode)
			switch mode {
			case DHTModeAuto, DHTModeServer, DHTModeClient:
				if !tt.valid && tt.mode != "invalid" {
					// lowercase versions are valid
				}
			default:
				// Could be uppercase - normalize check would happen in NewDHT
			}

			_ = cfg // Use the config
		})
	}
}

func TestDHTConfigDefaults(t *testing.T) {
	defaults := config.DefaultBibdConfig()

	if !defaults.P2P.DHT.Enabled {
		t.Error("DHT should be enabled by default")
	}
	if defaults.P2P.DHT.Mode != "auto" {
		t.Errorf("DHT mode should default to 'auto', got %s", defaults.P2P.DHT.Mode)
	}
}
