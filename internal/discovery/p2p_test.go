package discovery

import (
	"context"
	"testing"
	"time"
)

func TestDefaultP2PDiscoveryConfig(t *testing.T) {
	config := DefaultP2PDiscoveryConfig()

	if config.Timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", config.Timeout)
	}

	if config.MaxPeers != 50 {
		t.Errorf("expected max peers 50, got %d", config.MaxPeers)
	}

	if !config.MeasureLatency {
		t.Error("expected MeasureLatency to be true")
	}

	if len(config.BootstrapPeers) == 0 {
		t.Error("expected at least one bootstrap peer")
	}
}

func TestParseMultiaddr(t *testing.T) {
	tests := []struct {
		name         string
		multiaddr    string
		expectedHost string
		expectedPort int
		expectError  bool
	}{
		{
			name:         "ip4 tcp",
			multiaddr:    "/ip4/192.168.1.1/tcp/4001/p2p/QmTest",
			expectedHost: "192.168.1.1",
			expectedPort: 4001,
			expectError:  false,
		},
		{
			name:         "dns4 tcp",
			multiaddr:    "/dns4/bootstrap.bib.dev/tcp/4001/p2p/12D3KooW",
			expectedHost: "bootstrap.bib.dev",
			expectedPort: 4001,
			expectError:  false,
		},
		{
			name:         "ip6 tcp",
			multiaddr:    "/ip6/::1/tcp/4001/p2p/QmTest",
			expectedHost: "::1",
			expectedPort: 4001,
			expectError:  false,
		},
		{
			name:        "invalid",
			multiaddr:   "invalid",
			expectError: true,
		},
		{
			name:        "too short",
			multiaddr:   "/ip4/1.2.3.4",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := ParseMultiaddr(tt.multiaddr)

			if tt.expectError {
				if err == nil {
					t.Error("expected error")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if host != tt.expectedHost {
				t.Errorf("expected host %q, got %q", tt.expectedHost, host)
			}

			if port != tt.expectedPort {
				t.Errorf("expected port %d, got %d", tt.expectedPort, port)
			}
		})
	}
}

func TestMultiaddrsToAddresses(t *testing.T) {
	multiaddrs := []string{
		"/ip4/192.168.1.1/tcp/4001/p2p/QmTest1",
		"/ip4/192.168.1.2/tcp/4001/p2p/QmTest2",
		"/dns4/bootstrap.bib.dev/tcp/4001/p2p/QmTest3",
		"invalid",
		"/ip4/192.168.1.1/tcp/4001/p2p/QmTest1Duplicate", // Duplicate
	}

	addresses := MultiaddrsToAddresses(multiaddrs)

	// Should have 3 unique addresses (duplicate removed, invalid skipped)
	if len(addresses) != 3 {
		t.Errorf("expected 3 addresses, got %d: %v", len(addresses), addresses)
	}

	expected := map[string]bool{
		"192.168.1.1:4001":       true,
		"192.168.1.2:4001":       true,
		"bootstrap.bib.dev:4001": true,
	}

	for _, addr := range addresses {
		if !expected[addr] {
			t.Errorf("unexpected address: %s", addr)
		}
		delete(expected, addr)
	}

	if len(expected) > 0 {
		t.Errorf("missing addresses: %v", expected)
	}
}

func TestIsPublicBootstrapPeer(t *testing.T) {
	tests := []struct {
		address  string
		expected bool
	}{
		{"bootstrap.bib.dev:4001", true},
		{"bib.dev:4001", true},
		{"node1.bib.dev:4001", true},
		{"192.168.1.1:4001", false},
		{"localhost:4001", false},
		{"example.com:4001", false},
		{"bootstrap.bib.dev", true}, // Without port
	}

	for _, tt := range tests {
		t.Run(tt.address, func(t *testing.T) {
			result := IsPublicBootstrapPeer(tt.address)
			if result != tt.expected {
				t.Errorf("IsPublicBootstrapPeer(%q) = %v, expected %v", tt.address, result, tt.expected)
			}
		})
	}
}

func TestP2PDiscoveryConfig_Fields(t *testing.T) {
	config := P2PDiscoveryConfig{
		BootstrapPeers: []string{"peer1:4001", "peer2:4001"},
		Timeout:        5 * time.Second,
		MaxPeers:       100,
		MeasureLatency: true,
		LatencyTimeout: 3 * time.Second,
	}

	if len(config.BootstrapPeers) != 2 {
		t.Error("BootstrapPeers mismatch")
	}

	if config.Timeout != 5*time.Second {
		t.Error("Timeout mismatch")
	}

	if config.MaxPeers != 100 {
		t.Error("MaxPeers mismatch")
	}

	if config.LatencyTimeout != 3*time.Second {
		t.Error("LatencyTimeout mismatch")
	}
}

func TestP2PNodeInfo_Fields(t *testing.T) {
	info := P2PNodeInfo{
		PeerID:       "12D3KooWTest",
		Multiaddrs:   []string{"/ip4/1.2.3.4/tcp/4001"},
		Protocols:    []string{"/bib/1.0.0", "/bib/identity/1.0.0"},
		AgentVersion: "bibd/1.0.0",
	}

	if info.PeerID != "12D3KooWTest" {
		t.Error("PeerID mismatch")
	}

	if len(info.Multiaddrs) != 1 {
		t.Error("Multiaddrs mismatch")
	}

	if len(info.Protocols) != 2 {
		t.Error("Protocols mismatch")
	}

	if info.AgentVersion != "bibd/1.0.0" {
		t.Error("AgentVersion mismatch")
	}
}

func TestDiscoverP2P(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	config := P2PDiscoveryConfig{
		BootstrapPeers: []string{},
		Timeout:        1 * time.Second,
		MeasureLatency: false,
	}

	nodes, err := DiscoverP2P(ctx, config)

	// Should not error (may return empty or nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Result may be nil or empty, both are acceptable
	_ = nodes
}

func TestDiscoverFromBootstrapPeers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Use localhost which likely won't respond
	peers := []string{"127.0.0.1:19999"}

	nodes, err := DiscoverFromBootstrapPeers(ctx, peers, 1*time.Second)

	// Should not error (may return empty or nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Result may be nil or empty, both are acceptable
	_ = nodes
}

func TestDiscoverer_discoverP2P(t *testing.T) {
	d := New(DiscoveryOptions{
		EnableP2P:      true,
		P2PTimeout:     2 * time.Second,
		MeasureLatency: false,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	nodes, err := d.discoverP2P(ctx)

	// May fail if network is unavailable, that's OK
	if err != nil {
		t.Logf("P2P discovery returned error (may be expected): %v", err)
	}

	// Result may be nil or empty, both are acceptable
	_ = nodes
}
