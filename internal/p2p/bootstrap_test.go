package p2p

import (
	"testing"
	"time"

	"bib/internal/config"
)

func TestParseBootstrapPeers(t *testing.T) {
	tests := []struct {
		name    string
		addrs   []string
		wantLen int
		wantErr bool
	}{
		{
			name:    "empty",
			addrs:   []string{},
			wantLen: 0,
			wantErr: false,
		},
		{
			name: "valid with peer ID",
			addrs: []string{
				"/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWDXHortdoEeRiLx7FAtQ2ebEvS25R4ZKmVzpiz2sa2JWG",
			},
			wantLen: 1,
			wantErr: false,
		},
		{
			name: "without peer ID (skipped)",
			addrs: []string{
				"/dns4/bib.dev/tcp/4001",
			},
			wantLen: 0, // Skipped because no peer ID
			wantErr: false,
		},
		{
			name: "multiple addrs same peer",
			addrs: []string{
				"/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWDXHortdoEeRiLx7FAtQ2ebEvS25R4ZKmVzpiz2sa2JWG",
				"/ip4/127.0.0.1/udp/4001/quic-v1/p2p/12D3KooWDXHortdoEeRiLx7FAtQ2ebEvS25R4ZKmVzpiz2sa2JWG",
			},
			wantLen: 1, // Same peer ID, merged
			wantErr: false,
		},
		{
			name: "invalid multiaddr",
			addrs: []string{
				"not-a-multiaddr",
			},
			wantLen: 0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			peers, err := parseBootstrapPeers(tt.addrs)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseBootstrapPeers() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(peers) != tt.wantLen {
				t.Errorf("parseBootstrapPeers() got %d peers, want %d", len(peers), tt.wantLen)
			}
		})
	}
}

func TestBootstrapConfig(t *testing.T) {
	cfg := config.BootstrapConfig{
		Peers:            []string{},
		MinPeers:         1,
		RetryInterval:    5 * time.Second,
		MaxRetryInterval: 1 * time.Hour,
	}

	if cfg.RetryInterval != 5*time.Second {
		t.Errorf("expected 5s retry interval, got %v", cfg.RetryInterval)
	}
	if cfg.MaxRetryInterval != 1*time.Hour {
		t.Errorf("expected 1h max retry interval, got %v", cfg.MaxRetryInterval)
	}
}

func TestDefaultBootstrapPeers(t *testing.T) {
	peers := DefaultBootstrapPeers()
	if len(peers) != 2 {
		t.Errorf("expected 2 default bootstrap peers, got %d", len(peers))
	}

	// Should contain bib.dev addresses
	found := false
	for _, p := range peers {
		if p == "/dns4/bib.dev/tcp/4001" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected bib.dev in default bootstrap peers")
	}
}
