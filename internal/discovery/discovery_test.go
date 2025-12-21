package discovery

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts.Timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", opts.Timeout)
	}

	if len(opts.LocalPorts) != 2 {
		t.Errorf("expected 2 local ports, got %d", len(opts.LocalPorts))
	}

	if !opts.EnableMDNS {
		t.Error("expected mDNS to be enabled by default")
	}

	if opts.EnableP2P {
		t.Error("expected P2P to be disabled by default")
	}

	if !opts.MeasureLatency {
		t.Error("expected latency measurement to be enabled by default")
	}
}

func TestNewDiscoverer(t *testing.T) {
	opts := DefaultOptions()
	d := New(opts)

	if d == nil {
		t.Fatal("discoverer is nil")
	}

	if d.opts.Timeout != opts.Timeout {
		t.Error("options not set correctly")
	}
}

func TestNewWithDefaults(t *testing.T) {
	d := NewWithDefaults()

	if d == nil {
		t.Fatal("discoverer is nil")
	}

	if d.opts.Timeout != 10*time.Second {
		t.Errorf("expected default timeout, got %v", d.opts.Timeout)
	}
}

func TestMeasureLatency(t *testing.T) {
	// Start a test server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer listener.Close()

	// Accept connections in background
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	// Measure latency
	ctx := context.Background()
	latency, err := measureLatency(ctx, listener.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatalf("failed to measure latency: %v", err)
	}

	if latency <= 0 {
		t.Error("latency should be positive")
	}

	if latency > 1*time.Second {
		t.Errorf("latency to localhost should be very low, got %v", latency)
	}
}

func TestMeasureLatency_Timeout(t *testing.T) {
	ctx := context.Background()
	// Try to connect to a non-existent address
	_, err := measureLatency(ctx, "192.0.2.1:4000", 100*time.Millisecond)
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestSortNodesByLatency(t *testing.T) {
	nodes := []DiscoveredNode{
		{Address: "a", Latency: 100 * time.Millisecond},
		{Address: "b", Latency: 0}, // No latency measured
		{Address: "c", Latency: 50 * time.Millisecond},
		{Address: "d", Latency: 200 * time.Millisecond},
	}

	sortNodesByLatency(nodes)

	// Nodes with latency should come first, sorted by latency
	if nodes[0].Address != "c" {
		t.Errorf("expected 'c' first (50ms), got %s", nodes[0].Address)
	}
	if nodes[1].Address != "a" {
		t.Errorf("expected 'a' second (100ms), got %s", nodes[1].Address)
	}
	if nodes[2].Address != "d" {
		t.Errorf("expected 'd' third (200ms), got %s", nodes[2].Address)
	}
	// Node without latency should be last
	if nodes[3].Address != "b" {
		t.Errorf("expected 'b' last (no latency), got %s", nodes[3].Address)
	}
}

func TestDiscoveredNodeFields(t *testing.T) {
	node := DiscoveredNode{
		Address:      "localhost:4000",
		Method:       MethodLocal,
		Latency:      5 * time.Millisecond,
		DiscoveredAt: time.Now(),
		NodeInfo: &NodeInfo{
			Name:    "Test Node",
			Version: "1.0.0",
			Mode:    "proxy",
		},
	}

	if node.Address != "localhost:4000" {
		t.Error("address not set correctly")
	}

	if node.Method != MethodLocal {
		t.Error("method not set correctly")
	}

	if node.NodeInfo == nil {
		t.Fatal("node info is nil")
	}

	if node.NodeInfo.Name != "Test Node" {
		t.Error("node info name not set correctly")
	}
}

func TestDiscoveryMethods(t *testing.T) {
	if MethodLocal != "local" {
		t.Errorf("expected 'local', got %s", MethodLocal)
	}
	if MethodMDNS != "mdns" {
		t.Errorf("expected 'mdns', got %s", MethodMDNS)
	}
	if MethodP2P != "p2p" {
		t.Errorf("expected 'p2p', got %s", MethodP2P)
	}
	if MethodManual != "manual" {
		t.Errorf("expected 'manual', got %s", MethodManual)
	}
	if MethodPublic != "public" {
		t.Errorf("expected 'public', got %s", MethodPublic)
	}
}

func TestCheckAddress(t *testing.T) {
	// Start a test server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	d := NewWithDefaults()
	ctx := context.Background()

	node, err := d.CheckAddress(ctx, listener.Addr().String())
	if err != nil {
		t.Fatalf("failed to check address: %v", err)
	}

	if node == nil {
		t.Fatal("node is nil")
	}

	if node.Method != MethodManual {
		t.Errorf("expected method 'manual', got %s", node.Method)
	}

	if node.Latency <= 0 {
		t.Error("expected positive latency")
	}
}

func TestCheckAddress_Failed(t *testing.T) {
	d := New(DiscoveryOptions{
		LatencyTimeout: 100 * time.Millisecond,
	})
	ctx := context.Background()

	_, err := d.CheckAddress(ctx, "192.0.2.1:4000")
	if err == nil {
		t.Error("expected error for unreachable address")
	}
}

func TestScanPorts(t *testing.T) {
	// Start a test server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test server: %v", err)
	}
	defer listener.Close()

	// Get the port
	port := listener.Addr().(*net.TCPAddr).Port

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	ctx := context.Background()
	openPorts := ScanPorts(ctx, "127.0.0.1", []int{port, port + 1, port + 2}, 1*time.Second)

	if len(openPorts) != 1 {
		t.Errorf("expected 1 open port, got %d", len(openPorts))
	}

	if len(openPorts) > 0 && openPorts[0] != port {
		t.Errorf("expected port %d, got %d", port, openPorts[0])
	}
}

func TestParseMDNSTxtRecords(t *testing.T) {
	fields := []string{
		"name=My Node",
		"version=1.2.3",
		"mode=proxy",
		"peer_id=Qm123abc",
	}

	info := parseMDNSTxtRecords(fields)

	if info == nil {
		t.Fatal("info is nil")
	}

	if info.Name != "My Node" {
		t.Errorf("expected name 'My Node', got %s", info.Name)
	}

	if info.Version != "1.2.3" {
		t.Errorf("expected version '1.2.3', got %s", info.Version)
	}

	if info.Mode != "proxy" {
		t.Errorf("expected mode 'proxy', got %s", info.Mode)
	}

	if info.PeerID != "Qm123abc" {
		t.Errorf("expected peer_id 'Qm123abc', got %s", info.PeerID)
	}
}

func TestParseMDNSTxtRecords_Empty(t *testing.T) {
	info := parseMDNSTxtRecords(nil)
	if info != nil {
		t.Error("expected nil for empty fields")
	}

	info = parseMDNSTxtRecords([]string{})
	if info != nil {
		t.Error("expected nil for empty slice")
	}
}

func TestFormatDiscoveryResult(t *testing.T) {
	result := &DiscoveryResult{
		Nodes: []DiscoveredNode{
			{
				Address: "localhost:4000",
				Method:  MethodLocal,
				Latency: 5 * time.Millisecond,
			},
			{
				Address: "192.168.1.50:4000",
				Method:  MethodMDNS,
				Latency: 10 * time.Millisecond,
				NodeInfo: &NodeInfo{
					Name: "Remote Node",
				},
			},
		},
		Duration: 500 * time.Millisecond,
	}

	output := FormatDiscoveryResult(result)

	if output == "" {
		t.Error("output is empty")
	}

	if !containsString(output, "2 node(s)") {
		t.Error("expected '2 node(s)' in output")
	}

	if !containsString(output, "localhost:4000") {
		t.Error("expected 'localhost:4000' in output")
	}

	if !containsString(output, "Remote Node") {
		t.Error("expected 'Remote Node' in output")
	}
}

func TestDiscoverySummary(t *testing.T) {
	result := &DiscoveryResult{
		Nodes: []DiscoveredNode{
			{Address: "a", Method: MethodLocal},
			{Address: "b", Method: MethodLocal},
			{Address: "c", Method: MethodMDNS},
		},
	}

	summary := DiscoverySummary(result)

	if !containsString(summary, "3 nodes") {
		t.Errorf("expected '3 nodes' in summary, got: %s", summary)
	}

	if !containsString(summary, "2 local") {
		t.Errorf("expected '2 local' in summary, got: %s", summary)
	}

	if !containsString(summary, "1 mDNS") {
		t.Errorf("expected '1 mDNS' in summary, got: %s", summary)
	}
}

func TestDiscoverySummary_Empty(t *testing.T) {
	result := &DiscoveryResult{
		Nodes: []DiscoveredNode{},
	}

	summary := DiscoverySummary(result)

	if summary != "No nodes found" {
		t.Errorf("expected 'No nodes found', got: %s", summary)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) && (containsSubstr(s, substr))))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
