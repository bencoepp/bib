package p2p

import (
	"context"
	"os"
	"testing"
	"time"

	"bib/internal/config"
)

func TestNewHost(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-p2p-host-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := config.P2PConfig{
		Enabled:  true,
		Identity: config.P2PIdentityConfig{},
		ListenAddresses: []string{
			"/ip4/127.0.0.1/tcp/0", // Use port 0 to let OS assign
			"/ip4/127.0.0.1/udp/0/quic-v1",
		},
		ConnManager: config.ConnManagerConfig{
			LowWatermark:  10,
			HighWatermark: 40,
			GracePeriod:   time.Second,
		},
	}

	ctx := context.Background()
	host, err := NewHost(ctx, cfg, tmpDir)
	if err != nil {
		t.Fatalf("failed to create host: %v", err)
	}
	defer host.Close()

	// Wait for host to be ready
	if err := host.WaitForReady(ctx, 5*time.Second); err != nil {
		t.Fatalf("host not ready: %v", err)
	}

	// Verify host has addresses
	addrs := host.ListenAddrs()
	if len(addrs) == 0 {
		t.Fatal("host has no listen addresses")
	}

	// Verify we have a peer ID
	peerID := host.PeerID()
	if peerID == "" {
		t.Fatal("host has no peer ID")
	}

	t.Logf("Host created with peer ID: %s", peerID)
	for _, addr := range host.FullAddrs() {
		t.Logf("  Listening on: %s", addr)
	}
}

func TestHostPersistentIdentity(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-p2p-host-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := config.P2PConfig{
		Enabled:  true,
		Identity: config.P2PIdentityConfig{},
		ListenAddresses: []string{
			"/ip4/127.0.0.1/tcp/0",
		},
		ConnManager: config.ConnManagerConfig{
			LowWatermark:  10,
			HighWatermark: 40,
			GracePeriod:   time.Second,
		},
	}

	ctx := context.Background()

	// Create first host
	host1, err := NewHost(ctx, cfg, tmpDir)
	if err != nil {
		t.Fatalf("failed to create first host: %v", err)
	}
	peerID1 := host1.PeerID()
	host1.Close()

	// Create second host with same config dir - should have same peer ID
	host2, err := NewHost(ctx, cfg, tmpDir)
	if err != nil {
		t.Fatalf("failed to create second host: %v", err)
	}
	peerID2 := host2.PeerID()
	host2.Close()

	if peerID1 != peerID2 {
		t.Fatalf("peer IDs should be the same: %s != %s", peerID1, peerID2)
	}

	t.Logf("Persistent peer ID verified: %s", peerID1)
}
