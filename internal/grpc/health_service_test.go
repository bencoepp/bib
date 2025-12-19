package grpc

import (
	"context"
	"testing"
	"time"

	services "bib/api/gen/go/bib/v1/services"
	"bib/internal/cluster"
	"bib/internal/p2p"
	"bib/internal/storage"
)

// mockHealthProvider is a mock implementation of HealthProvider for testing.
type mockHealthProvider struct {
	running         bool
	startedAt       time.Time
	nodeMode        string
	nodeID          string
	listenAddresses []string
	config          HealthProviderConfig
	store           storage.Store
	p2pHost         *p2p.Host
	p2pDiscovery    *p2p.Discovery
	clusterInstance *cluster.Cluster
}

func (m *mockHealthProvider) IsRunning() bool                    { return m.running }
func (m *mockHealthProvider) StartedAt() time.Time               { return m.startedAt }
func (m *mockHealthProvider) NodeMode() string                   { return m.nodeMode }
func (m *mockHealthProvider) NodeID() string                     { return m.nodeID }
func (m *mockHealthProvider) ListenAddresses() []string          { return m.listenAddresses }
func (m *mockHealthProvider) HealthConfig() HealthProviderConfig { return m.config }
func (m *mockHealthProvider) Store() storage.Store               { return m.store }
func (m *mockHealthProvider) P2PHost() *p2p.Host                 { return m.p2pHost }
func (m *mockHealthProvider) P2PDiscovery() *p2p.Discovery       { return m.p2pDiscovery }
func (m *mockHealthProvider) Cluster() *cluster.Cluster          { return m.clusterInstance }

func TestHealthServiceServer_Ping(t *testing.T) {
	server := NewHealthServiceServer()

	ctx := context.Background()
	req := &services.PingRequest{
		Payload: []byte("hello"),
	}

	resp, err := server.Ping(ctx, req)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	if resp.Timestamp == nil {
		t.Error("expected timestamp in response")
	}

	if string(resp.Payload) != "hello" {
		t.Errorf("expected payload 'hello', got '%s'", string(resp.Payload))
	}
}

func TestHealthServiceServer_Check_NoProvider(t *testing.T) {
	server := NewHealthServiceServer()

	ctx := context.Background()
	req := &services.HealthCheckRequest{}

	resp, err := server.Check(ctx, req)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if resp.Status != services.ServingStatus_SERVING_STATUS_UNKNOWN {
		t.Errorf("expected UNKNOWN status without provider, got %v", resp.Status)
	}
}

func TestHealthServiceServer_Check_NotRunning(t *testing.T) {
	server := NewHealthServiceServer()
	provider := &mockHealthProvider{
		running:   false,
		startedAt: time.Now(),
		nodeMode:  "proxy",
		nodeID:    "test-node",
	}
	server.SetProvider(provider)

	ctx := context.Background()
	req := &services.HealthCheckRequest{}

	resp, err := server.Check(ctx, req)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if resp.Status != services.ServingStatus_SERVING_STATUS_NOT_SERVING {
		t.Errorf("expected NOT_SERVING status when not running, got %v", resp.Status)
	}
}

func TestHealthServiceServer_Check_Running_NoStorage(t *testing.T) {
	server := NewHealthServiceServer()
	provider := &mockHealthProvider{
		running:   true,
		startedAt: time.Now(),
		nodeMode:  "proxy",
		nodeID:    "test-node",
		config: HealthProviderConfig{
			P2PEnabled:     false,
			ClusterEnabled: false,
		},
	}
	server.SetProvider(provider)

	ctx := context.Background()
	req := &services.HealthCheckRequest{}

	resp, err := server.Check(ctx, req)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	// Storage is nil, so should be NOT_SERVING
	if resp.Status != services.ServingStatus_SERVING_STATUS_NOT_SERVING {
		t.Errorf("expected NOT_SERVING status without storage, got %v", resp.Status)
	}

	// Check that storage component is reported
	if _, ok := resp.Components["storage"]; !ok {
		t.Error("expected storage component in response")
	}
}

func TestHealthServiceServer_GetNodeInfo(t *testing.T) {
	server := NewHealthServiceServer()
	startTime := time.Now().Add(-1 * time.Hour)
	provider := &mockHealthProvider{
		running:         true,
		startedAt:       startTime,
		nodeMode:        "selective",
		nodeID:          "test-node-123",
		listenAddresses: []string{"127.0.0.1:9090"},
		config: HealthProviderConfig{
			P2PEnabled:        true,
			ClusterEnabled:    false,
			BandwidthMetering: true,
		},
	}
	server.SetProvider(provider)

	ctx := context.Background()
	req := &services.GetNodeInfoRequest{
		IncludeComponents: false,
		IncludeNetwork:    false,
		IncludeStorage:    false,
	}

	resp, err := server.GetNodeInfo(ctx, req)
	if err != nil {
		t.Fatalf("GetNodeInfo failed: %v", err)
	}

	if resp.NodeId != "test-node-123" {
		t.Errorf("expected node ID 'test-node-123', got '%s'", resp.NodeId)
	}

	if resp.Mode != "selective" {
		t.Errorf("expected mode 'selective', got '%s'", resp.Mode)
	}

	if resp.Version == "" {
		t.Error("expected version to be set")
	}

	if resp.Uptime == nil || resp.Uptime.AsDuration() < time.Hour {
		t.Error("expected uptime to be at least 1 hour")
	}
}

func TestHealthServiceServer_SetProvider(t *testing.T) {
	server := NewHealthServiceServer()

	provider := &mockHealthProvider{
		running: true,
		nodeID:  "test",
	}

	server.SetProvider(provider)

	// Verify provider is set by making a call
	ctx := context.Background()
	req := &services.HealthCheckRequest{}

	resp, err := server.Check(ctx, req)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	// Should not be UNKNOWN since provider is set
	if resp.Status == services.ServingStatus_SERVING_STATUS_UNKNOWN {
		t.Error("expected status to not be UNKNOWN after setting provider")
	}
}
