package discovery

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestNewConnectionTester(t *testing.T) {
	tester := NewConnectionTester()

	if tester == nil {
		t.Fatal("tester is nil")
	}

	if tester.Timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", tester.Timeout)
	}

	if tester.UseTLS {
		t.Error("expected UseTLS to be false by default")
	}

	if !tester.SkipTLSVerify {
		t.Error("expected SkipTLSVerify to be true by default")
	}
}

func TestConnectionTester_WithTimeout(t *testing.T) {
	tester := NewConnectionTester().WithTimeout(5 * time.Second)

	if tester.Timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", tester.Timeout)
	}
}

func TestConnectionTester_WithTLS(t *testing.T) {
	tester := NewConnectionTester().WithTLS(true)

	if !tester.UseTLS {
		t.Error("expected UseTLS to be true")
	}
}

func TestConnectionTester_WithSkipTLSVerify(t *testing.T) {
	tester := NewConnectionTester().WithSkipTLSVerify(false)

	if tester.SkipTLSVerify {
		t.Error("expected SkipTLSVerify to be false")
	}
}

func TestConnectionStatus_Constants(t *testing.T) {
	tests := []struct {
		status   ConnectionStatus
		expected string
	}{
		{StatusConnected, "connected"},
		{StatusDisconnected, "disconnected"},
		{StatusTimeout, "timeout"},
		{StatusRefused, "refused"},
		{StatusUnreachable, "unreachable"},
		{StatusAuthFailed, "auth_failed"},
		{StatusTLSError, "tls_error"},
		{StatusUnknown, "unknown"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.expected {
			t.Errorf("expected %q, got %q", tt.expected, string(tt.status))
		}
	}
}

func TestConnectionTester_TestConnection_Refused(t *testing.T) {
	tester := NewConnectionTester().WithTimeout(1 * time.Second)
	ctx := context.Background()

	// Test connection to a port that should be closed
	result := tester.TestConnection(ctx, "127.0.0.1:59999")

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Address != "127.0.0.1:59999" {
		t.Errorf("expected address '127.0.0.1:59999', got %q", result.Address)
	}

	if result.Status == StatusConnected {
		t.Error("expected non-connected status for closed port")
	}

	if result.TestedAt.IsZero() {
		t.Error("expected TestedAt to be set")
	}
}

func TestConnectionTester_TestConnection_Timeout(t *testing.T) {
	tester := NewConnectionTester().WithTimeout(100 * time.Millisecond)
	ctx := context.Background()

	// Test connection to a non-routable address (will timeout)
	result := tester.TestConnection(ctx, "192.0.2.1:4000")

	if result == nil {
		t.Fatal("result is nil")
	}

	// Should be timeout or unreachable
	if result.Status == StatusConnected {
		t.Error("expected non-connected status for unreachable address")
	}
}

func TestConnectionTester_TestConnections_Parallel(t *testing.T) {
	tester := NewConnectionTester().WithTimeout(100 * time.Millisecond)
	ctx := context.Background()

	addresses := []string{
		"127.0.0.1:59997",
		"127.0.0.1:59998",
		"127.0.0.1:59999",
	}

	results := tester.TestConnections(ctx, addresses)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// All should be tested
	for i, r := range results {
		if r == nil {
			t.Errorf("result[%d] is nil", i)
			continue
		}
		if r.Address != addresses[i] {
			t.Errorf("result[%d] address mismatch: expected %q, got %q", i, addresses[i], r.Address)
		}
	}
}

func TestConnectionTester_TestNodes(t *testing.T) {
	tester := NewConnectionTester().WithTimeout(100 * time.Millisecond)
	ctx := context.Background()

	nodes := []DiscoveredNode{
		{Address: "127.0.0.1:59997", Method: MethodLocal},
		{Address: "127.0.0.1:59998", Method: MethodMDNS},
	}

	results := tester.TestNodes(ctx, nodes)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestConnectionTester_ClassifyError(t *testing.T) {
	tester := NewConnectionTester()

	tests := []struct {
		name     string
		errMsg   string
		expected ConnectionStatus
	}{
		{"nil error", "", StatusConnected},
		{"connection refused", "dial tcp 127.0.0.1:4000: connection refused", StatusRefused},
		{"network unreachable", "dial tcp: network is unreachable", StatusUnreachable},
		{"no route", "dial tcp: no route to host", StatusUnreachable},
		{"tls error", "tls: first record does not look like a TLS handshake", StatusTLSError},
		{"certificate error", "x509: certificate signed by unknown authority", StatusTLSError},
		{"unauthenticated", "rpc error: code = Unauthenticated", StatusAuthFailed},
		{"unknown", "some random error", StatusDisconnected},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.errMsg != "" {
				err = &testError{msg: tt.errMsg}
			}
			status := tester.classifyError(err)
			if status != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, status)
			}
		})
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestFingerprintCert(t *testing.T) {
	// Test with some dummy data
	data := []byte("test certificate data")
	fp := fingerprintCert(data)

	if fp == "" {
		t.Error("fingerprint is empty")
	}

	if !containsStr(fp, "SHA256:") {
		t.Errorf("expected fingerprint to start with 'SHA256:', got %q", fp)
	}

	// Same input should produce same output
	fp2 := fingerprintCert(data)
	if fp != fp2 {
		t.Error("fingerprints should be deterministic")
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"hello world", "world", true},
		{"HELLO WORLD", "world", true},
		{"hello world", "WORLD", true},
		{"hello", "xyz", false},
		{"", "xyz", false},
		{"hello", "", true},
	}

	for _, tt := range tests {
		result := contains(tt.s, tt.substr)
		if result != tt.expected {
			t.Errorf("contains(%q, %q) = %v, expected %v", tt.s, tt.substr, result, tt.expected)
		}
	}
}

func TestFormatConnectionResult(t *testing.T) {
	t.Run("connected", func(t *testing.T) {
		result := &ConnectionTestResult{
			Address: "localhost:4000",
			Status:  StatusConnected,
			Latency: 5 * time.Millisecond,
			NodeInfo: &NodeInfo{
				Version: "1.0.0",
				Mode:    "full",
			},
		}
		output := FormatConnectionResult(result)
		if !containsStr(output, "✓") {
			t.Error("expected checkmark for connected status")
		}
		if !containsStr(output, "localhost:4000") {
			t.Error("expected address in output")
		}
		if !containsStr(output, "v1.0.0") {
			t.Error("expected version in output")
		}
	})

	t.Run("refused", func(t *testing.T) {
		result := &ConnectionTestResult{
			Address: "localhost:4000",
			Status:  StatusRefused,
			Error:   "connection refused",
		}
		output := FormatConnectionResult(result)
		if !containsStr(output, "✗") {
			t.Error("expected X for refused status")
		}
		if !containsStr(output, "refused") {
			t.Error("expected status in output")
		}
	})

	t.Run("with TLS", func(t *testing.T) {
		result := &ConnectionTestResult{
			Address: "localhost:4000",
			Status:  StatusConnected,
			Latency: 5 * time.Millisecond,
			TLSInfo: &TLSInfo{Enabled: true},
		}
		output := FormatConnectionResult(result)
		if !containsStr(output, "🔒") {
			t.Error("expected lock icon for TLS")
		}
	})
}

func TestFormatConnectionResults(t *testing.T) {
	results := []*ConnectionTestResult{
		{Address: "localhost:4000", Status: StatusConnected, Latency: 5 * time.Millisecond},
		{Address: "localhost:4001", Status: StatusRefused, Error: "refused"},
		{Address: "localhost:4002", Status: StatusConnected, Latency: 10 * time.Millisecond},
	}

	output := FormatConnectionResults(results)

	if !containsStr(output, "2 connected") {
		t.Error("expected '2 connected' in output")
	}
	if !containsStr(output, "1 failed") {
		t.Error("expected '1 failed' in output")
	}
}

func TestConnectionTestResult_Fields(t *testing.T) {
	result := ConnectionTestResult{
		Address: "test:4000",
		Status:  StatusConnected,
		Latency: 5 * time.Millisecond,
		NodeInfo: &NodeInfo{
			Name:    "Test Node",
			Version: "1.0.0",
			PeerID:  "Qm123",
			Mode:    "full",
		},
		TLSInfo: &TLSInfo{
			Enabled:     true,
			Fingerprint: "SHA256:abc",
			Subject:     "CN=test",
			Issuer:      "CN=issuer",
			NotAfter:    time.Now().Add(365 * 24 * time.Hour),
			Trusted:     true,
		},
		Error:    "",
		TestedAt: time.Now(),
	}

	if result.Address != "test:4000" {
		t.Error("address mismatch")
	}
	if result.NodeInfo.Name != "Test Node" {
		t.Error("node info name mismatch")
	}
	if result.TLSInfo.Fingerprint != "SHA256:abc" {
		t.Error("TLS fingerprint mismatch")
	}
}

func TestConnectionTester_TCPConnection(t *testing.T) {
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

	tester := NewConnectionTester().WithTimeout(2 * time.Second)
	ctx := context.Background()

	// Test TCP connection should succeed
	conn, err := tester.testTCPConnection(ctx, listener.Addr().String())
	if err != nil {
		t.Fatalf("expected TCP connection to succeed: %v", err)
	}
	conn.Close()
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
