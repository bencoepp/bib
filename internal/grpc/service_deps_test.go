package grpc

import (
	"testing"
	"time"
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

func TestServiceServers_SetHealthProvider(t *testing.T) {
	servers := NewServiceServers()

	// Create a mock provider (uses mockHealthProvider from health_service_test.go)
	provider := &mockHealthProvider{
		running: true,
		nodeID:  "test-node",
	}

	servers.SetHealthProvider(provider)

	// The health service should now have the provider
	if servers.Health.provider == nil {
		t.Error("Health provider should be set")
	}
}
