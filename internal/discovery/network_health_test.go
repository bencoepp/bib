package discovery

import (
	"context"
	"testing"
	"time"
)

func TestNewNetworkHealthChecker(t *testing.T) {
	checker := NewNetworkHealthChecker()

	if checker == nil {
		t.Fatal("checker is nil")
	}

	if checker.Timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", checker.Timeout)
	}
}

func TestNetworkHealthChecker_WithTimeout(t *testing.T) {
	checker := NewNetworkHealthChecker().WithTimeout(5 * time.Second)

	if checker.Timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", checker.Timeout)
	}
}

func TestNetworkHealthStatus_Constants(t *testing.T) {
	tests := []struct {
		status   NetworkHealthStatus
		expected string
	}{
		{NetworkHealthGood, "good"},
		{NetworkHealthDegraded, "degraded"},
		{NetworkHealthPoor, "poor"},
		{NetworkHealthOffline, "offline"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.expected {
			t.Errorf("expected %q, got %q", tt.expected, string(tt.status))
		}
	}
}

func TestNetworkHealthChecker_CheckHealth_ConnectionError(t *testing.T) {
	checker := NewNetworkHealthChecker().WithTimeout(1 * time.Second)
	ctx := context.Background()

	// Test against a port that should be closed
	result := checker.CheckHealth(ctx, "127.0.0.1:59999")

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Address != "127.0.0.1:59999" {
		t.Errorf("expected address '127.0.0.1:59999', got %q", result.Address)
	}

	if result.Status != NetworkHealthOffline {
		t.Errorf("expected status %q, got %q", NetworkHealthOffline, result.Status)
	}

	if result.Error == "" {
		t.Error("expected error message")
	}

	if result.Duration == 0 {
		t.Error("expected duration to be set")
	}

	if result.TestedAt.IsZero() {
		t.Error("expected TestedAt to be set")
	}
}

func TestNetworkHealthChecker_CheckHealthMultiple(t *testing.T) {
	checker := NewNetworkHealthChecker().WithTimeout(100 * time.Millisecond)
	ctx := context.Background()

	addresses := []string{
		"127.0.0.1:59997",
		"127.0.0.1:59998",
		"127.0.0.1:59999",
	}

	results := checker.CheckHealthMultiple(ctx, addresses)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

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

func TestNetworkHealthChecker_DetermineHealthStatus(t *testing.T) {
	checker := NewNetworkHealthChecker()

	tests := []struct {
		name     string
		stats    *NetworkStats
		expected NetworkHealthStatus
	}{
		{
			name:     "nil stats",
			stats:    nil,
			expected: NetworkHealthOffline,
		},
		{
			name: "good - peers and bootstrap",
			stats: &NetworkStats{
				ConnectedPeers:     5,
				BootstrapConnected: true,
			},
			expected: NetworkHealthGood,
		},
		{
			name: "degraded - peers but no bootstrap",
			stats: &NetworkStats{
				ConnectedPeers:     3,
				BootstrapConnected: false,
			},
			expected: NetworkHealthDegraded,
		},
		{
			name: "degraded - bootstrap but no peers",
			stats: &NetworkStats{
				ConnectedPeers:     0,
				BootstrapConnected: true,
			},
			expected: NetworkHealthDegraded,
		},
		{
			name: "poor - no peers, no bootstrap",
			stats: &NetworkStats{
				ConnectedPeers:     0,
				BootstrapConnected: false,
			},
			expected: NetworkHealthPoor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := checker.determineHealthStatus(tt.stats)
			if status != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, status)
			}
		})
	}
}

func TestNetworkHealthChecker_GetSummary(t *testing.T) {
	checker := NewNetworkHealthChecker()

	results := []*NetworkHealthResult{
		{
			Status: NetworkHealthGood,
			Network: &NetworkStats{
				ConnectedPeers:     5,
				BootstrapConnected: true,
			},
		},
		{
			Status: NetworkHealthDegraded,
			Network: &NetworkStats{
				ConnectedPeers:     2,
				BootstrapConnected: false,
			},
		},
		{
			Status: NetworkHealthOffline,
		},
	}

	summary := checker.GetSummary(results)

	if summary.TotalNodes != 3 {
		t.Errorf("expected 3 total nodes, got %d", summary.TotalNodes)
	}

	if summary.HealthyNodes != 1 {
		t.Errorf("expected 1 healthy node, got %d", summary.HealthyNodes)
	}

	if summary.DegradedNodes != 1 {
		t.Errorf("expected 1 degraded node, got %d", summary.DegradedNodes)
	}

	if summary.OfflineNodes != 1 {
		t.Errorf("expected 1 offline node, got %d", summary.OfflineNodes)
	}

	if summary.TotalConnectedPeers != 7 {
		t.Errorf("expected 7 total peers, got %d", summary.TotalConnectedPeers)
	}

	if !summary.BootstrapConnected {
		t.Error("expected BootstrapConnected to be true")
	}

	// 2 nodes with peers (healthy + degraded), so average should be 3.5
	expectedAvg := 3.5
	if summary.AverageConnectedPeers != expectedAvg {
		t.Errorf("expected average %.1f, got %.1f", expectedAvg, summary.AverageConnectedPeers)
	}

	// Overall should be degraded because we have offline nodes
	if summary.OverallStatus != NetworkHealthDegraded {
		t.Errorf("expected overall status %q, got %q", NetworkHealthDegraded, summary.OverallStatus)
	}
}

func TestNetworkHealthChecker_GetSummary_AllOffline(t *testing.T) {
	checker := NewNetworkHealthChecker()

	results := []*NetworkHealthResult{
		{Status: NetworkHealthOffline},
		{Status: NetworkHealthOffline},
	}

	summary := checker.GetSummary(results)

	if summary.OverallStatus != NetworkHealthOffline {
		t.Errorf("expected overall status %q, got %q", NetworkHealthOffline, summary.OverallStatus)
	}
}

func TestNetworkHealthResult_Fields(t *testing.T) {
	result := NetworkHealthResult{
		Address: "localhost:4000",
		Status:  NetworkHealthGood,
		NodeInfo: &NodeInfo{
			Version: "1.0.0",
			Mode:    "full",
			PeerID:  "Qm123",
		},
		Network: &NetworkStats{
			ConnectedPeers:      5,
			KnownPeers:          10,
			BootstrapConnected:  true,
			DHTRoutingTableSize: 20,
			ActiveStreams:       3,
			BytesSent:           1000,
			BytesReceived:       2000,
		},
		Duration: 50 * time.Millisecond,
		TestedAt: time.Now(),
	}

	if result.Address != "localhost:4000" {
		t.Error("address mismatch")
	}
	if result.Network.ConnectedPeers != 5 {
		t.Error("connected peers mismatch")
	}
	if result.Network.KnownPeers != 10 {
		t.Error("known peers mismatch")
	}
	if !result.Network.BootstrapConnected {
		t.Error("bootstrap connected should be true")
	}
}

func TestFormatNetworkHealthResult(t *testing.T) {
	t.Run("good health", func(t *testing.T) {
		result := &NetworkHealthResult{
			Address:  "localhost:4000",
			Status:   NetworkHealthGood,
			Duration: 50 * time.Millisecond,
			NodeInfo: &NodeInfo{
				Mode: "full",
			},
			Network: &NetworkStats{
				ConnectedPeers:      5,
				BootstrapConnected:  true,
				DHTRoutingTableSize: 20,
			},
		}
		output := FormatNetworkHealthResult(result)

		if !containsStr(output, "✓") {
			t.Error("expected checkmark for good status")
		}
		if !containsStr(output, "localhost:4000") {
			t.Error("expected address in output")
		}
		if !containsStr(output, "5 peers") {
			t.Error("expected peer count in output")
		}
		if !containsStr(output, "bootstrap") {
			t.Error("expected bootstrap in output")
		}
		if !containsStr(output, "DHT:20") {
			t.Error("expected DHT size in output")
		}
		if !containsStr(output, "[full]") {
			t.Error("expected mode in output")
		}
	})

	t.Run("degraded health", func(t *testing.T) {
		result := &NetworkHealthResult{
			Address:  "localhost:4000",
			Status:   NetworkHealthDegraded,
			Duration: 50 * time.Millisecond,
			Network: &NetworkStats{
				ConnectedPeers:     3,
				BootstrapConnected: false,
			},
		}
		output := FormatNetworkHealthResult(result)

		if !containsStr(output, "⚠") {
			t.Error("expected warning for degraded status")
		}
		if !containsStr(output, "no-bootstrap") {
			t.Error("expected no-bootstrap in output")
		}
	})

	t.Run("offline", func(t *testing.T) {
		result := &NetworkHealthResult{
			Address: "localhost:4000",
			Status:  NetworkHealthOffline,
			Error:   "connection refused",
		}
		output := FormatNetworkHealthResult(result)

		if !containsStr(output, "⊘") {
			t.Error("expected offline icon")
		}
		if !containsStr(output, "offline") {
			t.Error("expected 'offline' in output")
		}
		if !containsStr(output, "connection refused") {
			t.Error("expected error in output")
		}
	})
}

func TestFormatNetworkHealthResults(t *testing.T) {
	results := []*NetworkHealthResult{
		{
			Address:  "localhost:4000",
			Status:   NetworkHealthGood,
			Duration: 50 * time.Millisecond,
			Network: &NetworkStats{
				ConnectedPeers:     5,
				BootstrapConnected: true,
			},
		},
		{
			Address:  "localhost:4001",
			Status:   NetworkHealthDegraded,
			Duration: 100 * time.Millisecond,
			Network: &NetworkStats{
				ConnectedPeers:     2,
				BootstrapConnected: false,
			},
		},
		{
			Address: "localhost:4002",
			Status:  NetworkHealthOffline,
			Error:   "timeout",
		},
	}

	output := FormatNetworkHealthResults(results)

	if !containsStr(output, "1 healthy") {
		t.Error("expected '1 healthy' in output")
	}
	if !containsStr(output, "1 degraded") {
		t.Error("expected '1 degraded' in output")
	}
	if !containsStr(output, "1 offline") {
		t.Error("expected '1 offline' in output")
	}
}

func TestFormatNetworkHealthSummary(t *testing.T) {
	summary := &NetworkHealthSummary{
		TotalNodes:            3,
		HealthyNodes:          1,
		DegradedNodes:         1,
		OfflineNodes:          1,
		TotalConnectedPeers:   7,
		AverageConnectedPeers: 3.5,
		BootstrapConnected:    true,
		OverallStatus:         NetworkHealthDegraded,
	}

	output := FormatNetworkHealthSummary(summary)

	if !containsStr(output, "⚠") {
		t.Error("expected warning icon for degraded status")
	}
	if !containsStr(output, "degraded") {
		t.Error("expected 'degraded' in output")
	}
	if !containsStr(output, "3 total") {
		t.Error("expected node count in output")
	}
	if !containsStr(output, "7 connected") {
		t.Error("expected peer count in output")
	}
	if !containsStr(output, "3.5") {
		t.Error("expected average in output")
	}
	if !containsStr(output, "✓ connected") {
		t.Error("expected bootstrap connected in output")
	}
}

func TestNetworkHealthStatusIcon(t *testing.T) {
	tests := []struct {
		status   NetworkHealthStatus
		expected string
	}{
		{NetworkHealthGood, "✓"},
		{NetworkHealthDegraded, "⚠"},
		{NetworkHealthPoor, "✗"},
		{NetworkHealthOffline, "⊘"},
	}

	for _, tt := range tests {
		icon := NetworkHealthStatusIcon(tt.status)
		if icon != tt.expected {
			t.Errorf("expected icon %q for %q, got %q", tt.expected, tt.status, icon)
		}
	}
}

func TestNetworkHealthBrief(t *testing.T) {
	results := []*NetworkHealthResult{
		{
			Status: NetworkHealthGood,
			Network: &NetworkStats{
				ConnectedPeers: 5,
			},
		},
		{
			Status: NetworkHealthDegraded,
			Network: &NetworkStats{
				ConnectedPeers: 3,
			},
		},
		{
			Status: NetworkHealthOffline,
		},
	}

	brief := NetworkHealthBrief(results)

	if !containsStr(brief, "2/3 nodes healthy") {
		t.Errorf("expected '2/3 nodes healthy' in brief, got %q", brief)
	}
	if !containsStr(brief, "8 peers connected") {
		t.Errorf("expected '8 peers connected' in brief, got %q", brief)
	}
}

func TestNetworkStats_Fields(t *testing.T) {
	stats := NetworkStats{
		ConnectedPeers:      5,
		KnownPeers:          10,
		BootstrapConnected:  true,
		DHTRoutingTableSize: 20,
		ActiveStreams:       3,
		BytesSent:           1000,
		BytesReceived:       2000,
	}

	if stats.ConnectedPeers != 5 {
		t.Error("connected peers mismatch")
	}
	if stats.KnownPeers != 10 {
		t.Error("known peers mismatch")
	}
	if !stats.BootstrapConnected {
		t.Error("bootstrap connected should be true")
	}
	if stats.DHTRoutingTableSize != 20 {
		t.Error("DHT size mismatch")
	}
	if stats.ActiveStreams != 3 {
		t.Error("active streams mismatch")
	}
	if stats.BytesSent != 1000 {
		t.Error("bytes sent mismatch")
	}
	if stats.BytesReceived != 2000 {
		t.Error("bytes received mismatch")
	}
}

func TestNetworkHealthSummary_Fields(t *testing.T) {
	summary := NetworkHealthSummary{
		TotalNodes:            5,
		HealthyNodes:          3,
		DegradedNodes:         1,
		OfflineNodes:          1,
		TotalConnectedPeers:   15,
		AverageConnectedPeers: 3.75,
		BootstrapConnected:    true,
		OverallStatus:         NetworkHealthGood,
	}

	if summary.TotalNodes != 5 {
		t.Error("total nodes mismatch")
	}
	if summary.HealthyNodes != 3 {
		t.Error("healthy nodes mismatch")
	}
	if summary.AverageConnectedPeers != 3.75 {
		t.Error("average peers mismatch")
	}
	if summary.OverallStatus != NetworkHealthGood {
		t.Error("overall status mismatch")
	}
}
