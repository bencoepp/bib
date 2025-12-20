package grpc

import (
	"testing"
	"time"

	"bib/internal/cluster"
	"bib/internal/grpc/interfaces"
	"bib/internal/p2p"
	"bib/internal/storage"
)

func TestServiceServers_New(t *testing.T) {
	servers := NewServiceServers()

	if servers.Health == nil {
		t.Error("Health service should be initialized")
	}
	if servers.Auth == nil {
		t.Error("Auth service should be initialized")
	}
	if servers.User == nil {
		t.Error("User service should be initialized")
	}
	if servers.Node == nil {
		t.Error("Node service should be initialized")
	}
	if servers.Topic == nil {
		t.Error("Topic service should be initialized")
	}
	if servers.Dataset == nil {
		t.Error("Dataset service should be initialized")
	}
	if servers.Admin == nil {
		t.Error("Admin service should be initialized")
	}
	if servers.Query == nil {
		t.Error("Query service should be initialized")
	}
	if servers.Job == nil {
		t.Error("Job service should be initialized")
	}
	if servers.BreakGlass == nil {
		t.Error("BreakGlass service should be initialized")
	}
}

func TestServiceDependencies_ConfigureServices(t *testing.T) {
	servers := NewServiceServers()

	deps := ServiceDependencies{
		NodeID:    "test-node",
		NodeMode:  "full",
		Version:   "1.0.0",
		StartedAt: time.Now(),
	}

	// This should not panic
	servers.ConfigureServices(deps)

	// Services should still be set
	if servers.Health == nil {
		t.Error("Health service should remain initialized")
	}
}

// testHealthProvider is a mock health provider for tests.
type testHealthProvider struct {
	running   bool
	nodeID    string
	startedAt time.Time
}

func (p *testHealthProvider) IsRunning() bool              { return p.running }
func (p *testHealthProvider) NodeID() string               { return p.nodeID }
func (p *testHealthProvider) NodeMode() string             { return "full" }
func (p *testHealthProvider) StartedAt() time.Time         { return p.startedAt }
func (p *testHealthProvider) Store() storage.Store         { return nil }
func (p *testHealthProvider) P2PHost() *p2p.Host           { return nil }
func (p *testHealthProvider) P2PDiscovery() *p2p.Discovery { return nil }
func (p *testHealthProvider) Cluster() *cluster.Cluster    { return nil }
func (p *testHealthProvider) ListenAddresses() []string    { return []string{"127.0.0.1:9090"} }
func (p *testHealthProvider) HealthConfig() interfaces.HealthProviderConfig {
	return interfaces.HealthProviderConfig{}
}

func TestServiceServers_SetHealthProvider(t *testing.T) {
	servers := NewServiceServers()

	provider := &testHealthProvider{
		running:   true,
		nodeID:    "test-node",
		startedAt: time.Now(),
	}

	// This should not panic
	servers.SetHealthProvider(provider)

	// We can't check the internal provider field since it's unexported,
	// but the function should complete without error
}
