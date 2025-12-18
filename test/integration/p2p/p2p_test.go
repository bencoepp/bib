//go:build integration

// Package p2p_test contains integration tests for the P2P layer.
package p2p_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"bib/internal/config"
	"bib/internal/p2p"
	"bib/test/testutil"
	"bib/test/testutil/helpers"
)

// TestP2PHost_Integration tests P2P host creation and lifecycle.
func TestP2PHost_Integration(t *testing.T) {
	ctx := testutil.TestContext(t)
	dataDir := testutil.TempDir(t, "p2p")

	cfg := config.P2PConfig{
		Enabled:     true,
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
		Mode:        "full",
		EnableMDNS:  false,
	}

	host, err := p2p.NewHost(ctx, cfg, dataDir)
	if err != nil {
		t.Fatalf("failed to create host: %v", err)
	}
	defer host.Close()

	// Verify host has an ID
	if host.ID() == "" {
		t.Error("expected host to have an ID")
	}

	// Verify host is listening
	addrs := host.Addrs()
	if len(addrs) == 0 {
		t.Error("expected host to have listen addresses")
	}

	t.Logf("Host ID: %s", host.ID())
	t.Logf("Listen addresses: %v", addrs)
}

// TestP2PDiscovery_Integration tests peer discovery between two hosts.
func TestP2PDiscovery_Integration(t *testing.T) {
	ctx := testutil.TestContext(t)

	// Create two hosts
	host1 := createP2PHost(t, ctx, "host1")
	defer host1.Close()

	host2 := createP2PHost(t, ctx, "host2")
	defer host2.Close()

	// Get host1's address
	host1Addrs := host1.Addrs()
	if len(host1Addrs) == 0 {
		t.Fatal("host1 has no addresses")
	}

	// Connect host2 to host1
	host1FullAddr := fmt.Sprintf("%s/p2p/%s", host1Addrs[0], host1.ID())
	if err := host2.Connect(ctx, host1FullAddr); err != nil {
		t.Fatalf("failed to connect host2 to host1: %v", err)
	}

	// Verify connection
	helpers.Eventually(t, 5*time.Second, func() bool {
		return len(host1.Peers()) > 0 && len(host2.Peers()) > 0
	}, "hosts should discover each other")

	t.Logf("Host1 peers: %v", host1.Peers())
	t.Logf("Host2 peers: %v", host2.Peers())
}

// TestP2PMultiNode_Integration tests a multi-node P2P network.
func TestP2PMultiNode_Integration(t *testing.T) {
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	const numNodes = 5

	// Create multiple hosts
	hosts := make([]*p2p.Host, numNodes)
	for i := 0; i < numNodes; i++ {
		host := createP2PHost(t, ctx, fmt.Sprintf("host%d", i))
		hosts[i] = host
		defer host.Close()
	}

	// Connect hosts in a chain: 0 -> 1 -> 2 -> 3 -> 4
	for i := 1; i < numNodes; i++ {
		prevHost := hosts[i-1]
		currentHost := hosts[i]

		prevAddrs := prevHost.Addrs()
		if len(prevAddrs) == 0 {
			t.Fatalf("host %d has no addresses", i-1)
		}

		fullAddr := fmt.Sprintf("%s/p2p/%s", prevAddrs[0], prevHost.ID())
		if err := currentHost.Connect(ctx, fullAddr); err != nil {
			t.Fatalf("failed to connect host %d to host %d: %v", i, i-1, err)
		}
	}

	// Wait for connections to establish
	time.Sleep(2 * time.Second)

	// Log network topology
	for i, host := range hosts {
		t.Logf("Host %d (%s) peers: %v", i, host.ID()[:8], host.Peers())
	}
}

// TestP2PPubSub_Integration tests publish/subscribe functionality.
func TestP2PPubSub_Integration(t *testing.T) {
	ctx := testutil.TestContext(t)

	// Create two connected hosts
	host1 := createP2PHost(t, ctx, "pubsub1")
	defer host1.Close()

	host2 := createP2PHost(t, ctx, "pubsub2")
	defer host2.Close()

	// Connect hosts
	host1Addrs := host1.Addrs()
	fullAddr := fmt.Sprintf("%s/p2p/%s", host1Addrs[0], host1.ID())
	if err := host2.Connect(ctx, fullAddr); err != nil {
		t.Fatalf("failed to connect hosts: %v", err)
	}

	// Wait for connection
	time.Sleep(time.Second)

	// Create pubsub for both hosts
	ps1, err := p2p.NewPubSub(ctx, host1)
	if err != nil {
		t.Fatalf("failed to create pubsub for host1: %v", err)
	}

	ps2, err := p2p.NewPubSub(ctx, host2)
	if err != nil {
		t.Fatalf("failed to create pubsub for host2: %v", err)
	}

	// Subscribe host2 to a topic
	topicName := "test-topic"
	sub, err := ps2.Subscribe(topicName)
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	// Wait for subscription to propagate
	time.Sleep(time.Second)

	// Publish from host1
	testMsg := []byte("hello from host1")
	if err := ps1.Publish(topicName, testMsg); err != nil {
		t.Fatalf("failed to publish: %v", err)
	}

	// Wait for message
	msgCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	msg, err := sub.Next(msgCtx)
	if err != nil {
		t.Fatalf("failed to receive message: %v", err)
	}

	if string(msg.Data) != string(testMsg) {
		t.Errorf("expected message %q, got %q", testMsg, msg.Data)
	}
}

// createP2PHost creates a P2P host for testing.
func createP2PHost(t *testing.T, ctx context.Context, name string) *p2p.Host {
	t.Helper()

	dataDir := testutil.TempDir(t, "p2p-"+name)
	cfg := config.P2PConfig{
		Enabled:     true,
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
		Mode:        "full",
		EnableMDNS:  false,
	}

	host, err := p2p.NewHost(ctx, cfg, dataDir)
	if err != nil {
		t.Fatalf("failed to create host %s: %v", name, err)
	}

	return host
}
