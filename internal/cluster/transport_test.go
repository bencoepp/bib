package cluster

import (
	"testing"
	"time"

	"bib/internal/config"
)

func TestNewTransport(t *testing.T) {
	cfg := config.ClusterConfig{
		ListenAddr: "127.0.0.1:0", // Random port
	}

	transport, err := NewTransport(cfg, "test-node")
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}
	defer transport.Close()

	// Verify local address is set
	addr := transport.LocalAddr()
	if addr == "" {
		t.Error("expected local address to be set")
	}
}

func TestTransportConnectDisconnect(t *testing.T) {
	cfg1 := config.ClusterConfig{
		ListenAddr: "127.0.0.1:0",
	}
	cfg2 := config.ClusterConfig{
		ListenAddr: "127.0.0.1:0",
	}

	// Create two transports
	t1, err := NewTransport(cfg1, "node-1")
	if err != nil {
		t.Fatalf("failed to create transport 1: %v", err)
	}
	defer t1.Close()

	t2, err := NewTransport(cfg2, "node-2")
	if err != nil {
		t.Fatalf("failed to create transport 2: %v", err)
	}
	defer t2.Close()

	// Connect t1 to t2
	err = t1.Connect("node-2", t2.LocalAddr())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	// Wait a moment for connection to establish
	time.Sleep(100 * time.Millisecond)

	// Disconnect
	err = t1.Disconnect("node-2")
	if err != nil {
		t.Fatalf("failed to disconnect: %v", err)
	}
}

func TestRaftMessage(t *testing.T) {
	msg := &RaftMessage{
		Type:     MsgTypeRequestVote,
		From:     "node-1",
		To:       "node-2",
		Term:     5,
		LogIndex: 100,
		LogTerm:  4,
	}

	// Test encode
	data, err := encodeMessage(msg)
	if err != nil {
		t.Fatalf("failed to encode message: %v", err)
	}

	// Test decode
	decoded, err := decodeMessage(data)
	if err != nil {
		t.Fatalf("failed to decode message: %v", err)
	}

	if decoded.Type != msg.Type {
		t.Errorf("type mismatch: expected %d, got %d", msg.Type, decoded.Type)
	}
	if decoded.From != msg.From {
		t.Errorf("from mismatch: expected %s, got %s", msg.From, decoded.From)
	}
	if decoded.To != msg.To {
		t.Errorf("to mismatch: expected %s, got %s", msg.To, decoded.To)
	}
	if decoded.Term != msg.Term {
		t.Errorf("term mismatch: expected %d, got %d", msg.Term, decoded.Term)
	}
	if decoded.LogIndex != msg.LogIndex {
		t.Errorf("log index mismatch: expected %d, got %d", msg.LogIndex, decoded.LogIndex)
	}
}
