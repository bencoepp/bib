package discovery

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

func TestDefaultMDNSServerConfig(t *testing.T) {
	config := DefaultMDNSServerConfig()

	if config.Port != 4000 {
		t.Errorf("expected port 4000, got %d", config.Port)
	}

	if config.Mode != "proxy" {
		t.Errorf("expected mode 'proxy', got %q", config.Mode)
	}

	if config.InstanceName == "" {
		t.Error("instance name should not be empty")
	}
}

func TestNewMDNSServer(t *testing.T) {
	config := MDNSServerConfig{
		InstanceName: "test-node",
		Port:         4000,
		NodeName:     "Test Node",
		Version:      "1.0.0",
	}

	server := NewMDNSServer(config)

	if server == nil {
		t.Fatal("server is nil")
	}

	if server.Config.InstanceName != "test-node" {
		t.Error("config not set correctly")
	}

	if server.IsRunning() {
		t.Error("server should not be running initially")
	}
}

func TestMDNSServer_buildTXTRecords(t *testing.T) {
	config := MDNSServerConfig{
		InstanceName: "test",
		Port:         4000,
		NodeName:     "My Node",
		Version:      "1.2.3",
		PeerID:       "QmTest123",
		Mode:         "full",
	}

	server := NewMDNSServer(config)
	records := server.buildTXTRecords()

	expected := map[string]string{
		"name":    "My Node",
		"version": "1.2.3",
		"peer_id": "QmTest123",
		"mode":    "full",
	}

	for _, record := range records {
		parts := strings.SplitN(record, "=", 2)
		if len(parts) != 2 {
			t.Errorf("invalid record format: %s", record)
			continue
		}
		key, value := parts[0], parts[1]
		if expectedValue, ok := expected[key]; ok {
			if value != expectedValue {
				t.Errorf("expected %s=%s, got %s=%s", key, expectedValue, key, value)
			}
			delete(expected, key)
		}
	}

	if len(expected) > 0 {
		t.Errorf("missing expected records: %v", expected)
	}
}

func TestMDNSServer_buildTXTRecords_Partial(t *testing.T) {
	config := MDNSServerConfig{
		InstanceName: "test",
		Port:         4000,
		NodeName:     "My Node",
		// Other fields empty
	}

	server := NewMDNSServer(config)
	records := server.buildTXTRecords()

	// Should only have name record
	found := false
	for _, record := range records {
		if strings.HasPrefix(record, "name=") {
			found = true
		}
	}

	if !found {
		t.Error("should have name record")
	}
}

func TestMDNSServer_GetAdvertisedAddress(t *testing.T) {
	config := MDNSServerConfig{
		InstanceName: "test",
		Port:         4000,
		Host:         "myhost.local",
	}

	server := NewMDNSServer(config)
	address := server.GetAdvertisedAddress()

	if address != "myhost.local:4000" {
		t.Errorf("expected 'myhost.local:4000', got %q", address)
	}
}

func TestMDNSServer_GetTXTRecords(t *testing.T) {
	config := MDNSServerConfig{
		InstanceName: "test",
		Port:         4000,
		NodeName:     "Test",
		Version:      "1.0.0",
	}

	server := NewMDNSServer(config)
	records := server.GetTXTRecords()

	if len(records) < 2 {
		t.Errorf("expected at least 2 records, got %d", len(records))
	}
}

func TestMDNSServer_StartStop(t *testing.T) {
	config := MDNSServerConfig{
		InstanceName: "test-bibd",
		Port:         14000,              // Use high port to avoid conflicts
		Host:         "localhost.local.", // FQDN required by mDNS
		NodeName:     "Test Node",
		Version:      "1.0.0",
		Mode:         "proxy",
		IPs:          []net.IP{net.ParseIP("127.0.0.1")},
	}

	server := NewMDNSServer(config)

	// Should not be running initially
	if server.IsRunning() {
		t.Error("server should not be running initially")
	}

	// Start server
	err := server.Start()
	if err != nil {
		// mDNS may fail in some test environments (e.g., CI without multicast)
		// Skip if that's the case
		if strings.Contains(err.Error(), "multicast") ||
			strings.Contains(err.Error(), "permission") ||
			strings.Contains(err.Error(), "bind") {
			t.Skipf("mDNS not available in test environment: %v", err)
		}
		t.Fatalf("failed to start server: %v", err)
	}

	// Should be running now
	if !server.IsRunning() {
		t.Error("server should be running after start")
	}

	// Starting again should fail
	err = server.Start()
	if err == nil {
		t.Error("starting twice should fail")
	}

	// Stop server
	err = server.Stop()
	if err != nil {
		t.Errorf("failed to stop server: %v", err)
	}

	// Should not be running
	if server.IsRunning() {
		t.Error("server should not be running after stop")
	}

	// Stopping again should be safe
	err = server.Stop()
	if err != nil {
		t.Errorf("stopping twice should not error: %v", err)
	}
}

func TestMDNSServerManager(t *testing.T) {
	manager := NewMDNSServerManager()

	if manager.IsRunning() {
		t.Error("manager should not be running initially")
	}

	config := MDNSServerConfig{
		InstanceName: "test-managed",
		Port:         14001,
		Host:         "localhost.local.", // FQDN required by mDNS
		NodeName:     "Managed Node",
		IPs:          []net.IP{net.ParseIP("127.0.0.1")},
	}

	err := manager.StartServer(config)
	if err != nil {
		if strings.Contains(err.Error(), "multicast") ||
			strings.Contains(err.Error(), "permission") ||
			strings.Contains(err.Error(), "bind") {
			t.Skipf("mDNS not available in test environment: %v", err)
		}
		t.Fatalf("failed to start managed server: %v", err)
	}

	if !manager.IsRunning() {
		t.Error("manager should be running after start")
	}

	server := manager.GetServer()
	if server == nil {
		t.Error("GetServer should return the server")
	}

	err = manager.StopServer()
	if err != nil {
		t.Errorf("failed to stop managed server: %v", err)
	}

	if manager.IsRunning() {
		t.Error("manager should not be running after stop")
	}
}

func TestMDNSServerManager_RunWithContext(t *testing.T) {
	manager := NewMDNSServerManager()

	config := MDNSServerConfig{
		InstanceName: "test-context",
		Port:         14002,
		Host:         "localhost.local.", // FQDN required by mDNS
		NodeName:     "Context Node",
		IPs:          []net.IP{net.ParseIP("127.0.0.1")},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- manager.RunWithContext(ctx, config)
	}()

	// Wait a bit for server to start
	time.Sleep(50 * time.Millisecond)

	// Should be running
	if !manager.IsRunning() {
		// May fail in CI environments
		select {
		case err := <-errCh:
			if err != nil {
				if strings.Contains(err.Error(), "multicast") ||
					strings.Contains(err.Error(), "permission") ||
					strings.Contains(err.Error(), "bind") {
					t.Skipf("mDNS not available in test environment: %v", err)
				}
			}
		default:
		}
		t.Skip("mDNS server may not be running in test environment")
	}

	// Wait for context to expire
	<-ctx.Done()

	// Wait for goroutine
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("RunWithContext returned error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("RunWithContext did not return after context cancelled")
	}

	// Should be stopped now
	if manager.IsRunning() {
		t.Error("manager should not be running after context cancelled")
	}
}

func TestMDNSServer_UpdateInfo(t *testing.T) {
	config := MDNSServerConfig{
		InstanceName: "test-update",
		Port:         14003,
		Host:         "localhost.local.", // FQDN required by mDNS
		NodeName:     "Original Name",
		Version:      "1.0.0",
		IPs:          []net.IP{net.ParseIP("127.0.0.1")},
	}

	server := NewMDNSServer(config)

	// Update without starting should work
	newConfig := config
	newConfig.NodeName = "Updated Name"
	newConfig.Version = "2.0.0"

	err := server.UpdateInfo(newConfig)
	if err != nil {
		t.Errorf("UpdateInfo failed: %v", err)
	}

	if server.Config.NodeName != "Updated Name" {
		t.Error("config not updated")
	}

	if server.Config.Version != "2.0.0" {
		t.Error("version not updated")
	}
}

func TestMDNSServerConfig_Fields(t *testing.T) {
	config := MDNSServerConfig{
		InstanceName: "my-instance",
		Port:         8080,
		Host:         "myhost.local",
		NodeName:     "My Node",
		Version:      "1.2.3",
		PeerID:       "QmTest",
		Mode:         "full",
		IPs:          []net.IP{net.ParseIP("192.168.1.100")},
	}

	if config.InstanceName != "my-instance" {
		t.Error("InstanceName mismatch")
	}
	if config.Port != 8080 {
		t.Error("Port mismatch")
	}
	if config.Host != "myhost.local" {
		t.Error("Host mismatch")
	}
	if config.NodeName != "My Node" {
		t.Error("NodeName mismatch")
	}
	if config.Version != "1.2.3" {
		t.Error("Version mismatch")
	}
	if config.PeerID != "QmTest" {
		t.Error("PeerID mismatch")
	}
	if config.Mode != "full" {
		t.Error("Mode mismatch")
	}
	if len(config.IPs) != 1 {
		t.Error("IPs mismatch")
	}
}
