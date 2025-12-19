// Package p2p provides peer-to-peer networking functionality for bib.
package p2p

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

func TestStreamConn_Implements_NetConn(t *testing.T) {
	// Verify that streamConn implements net.Conn interface
	var _ net.Conn = (*streamConn)(nil)
}

func TestPeerAddr_Implements_NetAddr(t *testing.T) {
	// Verify that peerAddr implements net.Addr interface
	var _ net.Addr = (*peerAddr)(nil)
}

func TestPeerAddr_Network(t *testing.T) {
	addr := &peerAddr{id: "test-peer-id"}

	if got := addr.Network(); got != "libp2p" {
		t.Errorf("Network() = %q, want %q", got, "libp2p")
	}
}

func TestPeerAddr_String(t *testing.T) {
	// peer.ID has its own encoding, so we just verify the round-trip
	testID := peer.ID("test-peer-id")
	addr := &peerAddr{id: testID}

	// String() should return the peer ID's string representation
	if got := addr.String(); got != testID.String() {
		t.Errorf("String() = %q, want %q", got, testID.String())
	}
}

func TestP2PListener_Implements_NetListener(t *testing.T) {
	// Verify that p2pListener implements net.Listener interface
	var _ net.Listener = (*p2pListener)(nil)
}

// mockStream is a minimal mock of network.Stream for testing
type mockReadWriteCloser struct {
	readData  []byte
	readPos   int
	writeData []byte
	closed    bool
}

func (m *mockReadWriteCloser) Read(b []byte) (int, error) {
	if m.closed {
		return 0, io.EOF
	}
	if m.readPos >= len(m.readData) {
		return 0, io.EOF
	}
	n := copy(b, m.readData[m.readPos:])
	m.readPos += n
	return n, nil
}

func (m *mockReadWriteCloser) Write(b []byte) (int, error) {
	if m.closed {
		return 0, ErrStreamClosed
	}
	m.writeData = append(m.writeData, b...)
	return len(b), nil
}

func (m *mockReadWriteCloser) Close() error {
	m.closed = true
	return nil
}

func TestErrStreamClosed(t *testing.T) {
	if ErrStreamClosed.Error() != "stream closed" {
		t.Errorf("ErrStreamClosed.Error() = %q, want %q", ErrStreamClosed.Error(), "stream closed")
	}
}

func TestDefaultGRPCClientConfig(t *testing.T) {
	cfg := DefaultGRPCClientConfig()

	if cfg.DialTimeout != 30*time.Second {
		t.Errorf("DialTimeout = %v, want %v", cfg.DialTimeout, 30*time.Second)
	}

	if cfg.MaxConnsPerPeer != 2 {
		t.Errorf("MaxConnsPerPeer = %d, want %d", cfg.MaxConnsPerPeer, 2)
	}

	if cfg.IdleTimeout != 5*time.Minute {
		t.Errorf("IdleTimeout = %v, want %v", cfg.IdleTimeout, 5*time.Minute)
	}

	if cfg.TCPFallbackEnabled != false {
		t.Errorf("TCPFallbackEnabled = %v, want %v", cfg.TCPFallbackEnabled, false)
	}

	if cfg.TCPFallbackTimeout != 10*time.Second {
		t.Errorf("TCPFallbackTimeout = %v, want %v", cfg.TCPFallbackTimeout, 10*time.Second)
	}
}

func TestDefaultRateLimitConfig(t *testing.T) {
	cfg := DefaultRateLimitConfig()

	if cfg.Enabled != true {
		t.Errorf("Enabled = %v, want %v", cfg.Enabled, true)
	}

	if cfg.RequestsPerSecond != 100 {
		t.Errorf("RequestsPerSecond = %v, want %v", cfg.RequestsPerSecond, 100.0)
	}

	if cfg.BurstSize != 200 {
		t.Errorf("BurstSize = %d, want %d", cfg.BurstSize, 200)
	}

	if cfg.CleanupInterval != 5*time.Minute {
		t.Errorf("CleanupInterval = %v, want %v", cfg.CleanupInterval, 5*time.Minute)
	}
}

func TestGRPCClient_NewWithNilHost(t *testing.T) {
	cfg := GRPCClientConfig{
		Host: nil,
	}

	_, err := NewGRPCClient(cfg)
	if err == nil {
		t.Error("Expected error when creating GRPCClient with nil host")
	}
}

func TestGRPCClientStats(t *testing.T) {
	stats := GRPCClientStats{
		TotalConnections:       5,
		P2PConnections:         3,
		TCPFallbackConnections: 2,
	}

	if stats.TotalConnections != 5 {
		t.Errorf("TotalConnections = %d, want %d", stats.TotalConnections, 5)
	}

	if stats.P2PConnections != 3 {
		t.Errorf("P2PConnections = %d, want %d", stats.P2PConnections, 3)
	}

	if stats.TCPFallbackConnections != 2 {
		t.Errorf("TCPFallbackConnections = %d, want %d", stats.TCPFallbackConnections, 2)
	}
}

func TestProtocolGRPC(t *testing.T) {
	expected := "/bib/grpc/1.0.0"
	if string(ProtocolGRPC) != expected {
		t.Errorf("ProtocolGRPC = %q, want %q", ProtocolGRPC, expected)
	}
}

func TestSupportedProtocols_ContainsGRPC(t *testing.T) {
	protocols := SupportedProtocols()

	found := false
	for _, p := range protocols {
		if p == ProtocolGRPC {
			found = true
			break
		}
	}

	if !found {
		t.Error("SupportedProtocols() should contain ProtocolGRPC")
	}
}

func TestPeerIDFromContext_Empty(t *testing.T) {
	ctx := context.Background()

	_, ok := PeerIDFromContext(ctx)
	if ok {
		t.Error("PeerIDFromContext should return false for empty context")
	}
}
