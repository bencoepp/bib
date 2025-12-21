package postsetup

import (
	"context"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNewLocalVerifier(t *testing.T) {
	t.Run("with address", func(t *testing.T) {
		v := NewLocalVerifier("localhost:8080")
		if v.Address != "localhost:8080" {
			t.Errorf("expected address 'localhost:8080', got %q", v.Address)
		}
	})

	t.Run("empty address", func(t *testing.T) {
		v := NewLocalVerifier("")
		if v.Address != "localhost:4000" {
			t.Errorf("expected default address 'localhost:4000', got %q", v.Address)
		}
	})
}

func TestLocalVerifier_Verify(t *testing.T) {
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

	v := NewLocalVerifier(listener.Addr().String())
	ctx := context.Background()

	status := v.Verify(ctx)

	if !status.Running {
		t.Error("expected Running to be true")
	}

	if status.Address != listener.Addr().String() {
		t.Error("address mismatch")
	}
}

func TestLocalVerifier_Verify_NotRunning(t *testing.T) {
	v := NewLocalVerifier("127.0.0.1:59999") // Unlikely to be open
	ctx := context.Background()

	status := v.Verify(ctx)

	if status.Running {
		t.Error("expected Running to be false")
	}

	if status.Error == "" {
		t.Error("expected error message")
	}
}

func TestFormatLocalStatus(t *testing.T) {
	t.Run("running", func(t *testing.T) {
		status := &LocalStatus{
			Running:      true,
			Address:      "localhost:4000",
			Version:      "1.0.0",
			Uptime:       5 * time.Minute,
			Mode:         "proxy",
			Healthy:      true,
			HealthStatus: "ok",
		}

		formatted := FormatLocalStatus(status)

		if !strings.Contains(formatted, "🟢") {
			t.Error("should show green indicator for running")
		}
		if !strings.Contains(formatted, "localhost:4000") {
			t.Error("should show address")
		}
		if !strings.Contains(formatted, "1.0.0") {
			t.Error("should show version")
		}
	})

	t.Run("not running", func(t *testing.T) {
		status := &LocalStatus{
			Running: false,
			Address: "localhost:4000",
			Error:   "connection refused",
		}

		formatted := FormatLocalStatus(status)

		if !strings.Contains(formatted, "🔴") {
			t.Error("should show red indicator for not running")
		}
		if !strings.Contains(formatted, "connection refused") {
			t.Error("should show error")
		}
	})
}

func TestServiceStatus_Fields(t *testing.T) {
	status := ServiceStatus{
		Installed:   true,
		Running:     true,
		Enabled:     true,
		ServiceName: "bibd",
		Error:       "",
	}

	if !status.Installed {
		t.Error("Installed should be true")
	}
	if !status.Running {
		t.Error("Running should be true")
	}
	if status.ServiceName != "bibd" {
		t.Error("ServiceName mismatch")
	}
}

func TestFormatServiceStatus(t *testing.T) {
	t.Run("installed and running", func(t *testing.T) {
		status := &ServiceStatus{
			Installed:   true,
			Running:     true,
			Enabled:     true,
			ServiceName: "bibd",
		}

		formatted := FormatServiceStatus(status)

		if !strings.Contains(formatted, "bibd") {
			t.Error("should show service name")
		}
		if !strings.Contains(formatted, "🟢") {
			t.Error("should show green indicator")
		}
	})

	t.Run("not installed", func(t *testing.T) {
		status := &ServiceStatus{
			Installed:   false,
			ServiceName: "bibd",
		}

		formatted := FormatServiceStatus(status)

		if !strings.Contains(formatted, "not installed") {
			t.Error("should indicate not installed")
		}
	})
}

func TestGetLocalManagementCommands(t *testing.T) {
	commands := GetLocalManagementCommands()

	if len(commands) == 0 {
		t.Error("should return some commands")
	}

	// Verify platform-specific commands
	switch runtime.GOOS {
	case "linux":
		found := false
		for _, cmd := range commands {
			if strings.Contains(cmd, "systemctl") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Linux should have systemctl commands")
		}
	case "darwin":
		found := false
		for _, cmd := range commands {
			if strings.Contains(cmd, "launchctl") {
				found = true
				break
			}
		}
		if !found {
			t.Error("macOS should have launchctl commands")
		}
	case "windows":
		found := false
		for _, cmd := range commands {
			if strings.Contains(cmd, "sc") {
				found = true
				break
			}
		}
		if !found {
			t.Error("Windows should have sc commands")
		}
	}
}

func TestDockerStatus_Fields(t *testing.T) {
	status := DockerStatus{
		Containers: []ContainerStatus{
			{Name: "bibd", Running: true},
			{Name: "postgres", Running: true},
		},
		AllRunning:    true,
		AllHealthy:    true,
		BibdReachable: true,
		BibdAddress:   "localhost:4000",
	}

	if len(status.Containers) != 2 {
		t.Error("Containers count mismatch")
	}
	if !status.AllRunning {
		t.Error("AllRunning should be true")
	}
}

func TestFormatDockerStatus(t *testing.T) {
	status := &DockerStatus{
		Containers: []ContainerStatus{
			{Name: "bibd", Running: true, Healthy: true, Status: "Up 5 minutes"},
			{Name: "postgres", Running: true, Healthy: true, Status: "Up 5 minutes"},
		},
		AllRunning:    true,
		AllHealthy:    true,
		BibdReachable: true,
		BibdAddress:   "localhost:4000",
	}

	formatted := FormatDockerStatus(status)

	if !strings.Contains(formatted, "🟢") {
		t.Error("should show green indicator")
	}
	if !strings.Contains(formatted, "bibd") {
		t.Error("should show container names")
	}
}

func TestGetDockerManagementCommands(t *testing.T) {
	commands := GetDockerManagementCommands("docker-compose.yaml")

	if len(commands) == 0 {
		t.Error("should return some commands")
	}

	found := false
	for _, cmd := range commands {
		if strings.Contains(cmd, "docker compose") {
			found = true
			break
		}
	}
	if !found {
		t.Error("should have docker compose commands")
	}
}

func TestPodmanStatus_Fields(t *testing.T) {
	status := PodmanStatus{
		DeployStyle: "pod",
		PodName:     "bibd",
		Containers: []ContainerStatus{
			{Name: "bibd", Running: true},
		},
		AllRunning:    true,
		BibdReachable: true,
		BibdAddress:   "localhost:4000",
	}

	if status.DeployStyle != "pod" {
		t.Error("DeployStyle mismatch")
	}
	if status.PodName != "bibd" {
		t.Error("PodName mismatch")
	}
}

func TestFormatPodmanStatus(t *testing.T) {
	status := &PodmanStatus{
		DeployStyle: "pod",
		PodName:     "bibd",
		Containers: []ContainerStatus{
			{Name: "bibd", Running: true, Status: "Up 5 minutes"},
		},
		AllRunning:    true,
		BibdReachable: true,
		BibdAddress:   "localhost:4000",
	}

	formatted := FormatPodmanStatus(status)

	if !strings.Contains(formatted, "🟢") {
		t.Error("should show green indicator")
	}
	if !strings.Contains(formatted, "bibd") {
		t.Error("should show pod name")
	}
}

func TestGetPodmanManagementCommands(t *testing.T) {
	t.Run("pod style", func(t *testing.T) {
		commands := GetPodmanManagementCommands("pod", "bibd", "")

		found := false
		for _, cmd := range commands {
			if strings.Contains(cmd, "podman pod") {
				found = true
				break
			}
		}
		if !found {
			t.Error("should have podman pod commands")
		}
	})

	t.Run("compose style", func(t *testing.T) {
		commands := GetPodmanManagementCommands("compose", "", "podman-compose.yaml")

		found := false
		for _, cmd := range commands {
			if strings.Contains(cmd, "podman-compose") {
				found = true
				break
			}
		}
		if !found {
			t.Error("should have podman-compose commands")
		}
	})
}

func TestKubernetesStatus_Fields(t *testing.T) {
	status := KubernetesStatus{
		Namespace: "bibd",
		Pods: []PodStatus{
			{Name: "bibd-abc123", Phase: "Running", Ready: true},
		},
		AllReady:      true,
		ExternalIP:    "34.56.78.90",
		BibdReachable: true,
		BibdAddress:   "34.56.78.90:4000",
	}

	if status.Namespace != "bibd" {
		t.Error("Namespace mismatch")
	}
	if !status.AllReady {
		t.Error("AllReady should be true")
	}
}

func TestFormatKubernetesStatus(t *testing.T) {
	status := &KubernetesStatus{
		Namespace: "bibd",
		Pods: []PodStatus{
			{Name: "bibd-abc123", Phase: "Running", Ready: true, Restarts: 0, Age: "5m"},
		},
		AllReady:      true,
		ExternalIP:    "34.56.78.90",
		BibdReachable: true,
		BibdAddress:   "34.56.78.90:4000",
	}

	formatted := FormatKubernetesStatus(status)

	if !strings.Contains(formatted, "🟢") {
		t.Error("should show green indicator")
	}
	if !strings.Contains(formatted, "bibd") {
		t.Error("should show namespace")
	}
	if !strings.Contains(formatted, "34.56.78.90") {
		t.Error("should show external IP")
	}
}

func TestGetKubernetesManagementCommands(t *testing.T) {
	commands := GetKubernetesManagementCommands("bibd")

	if len(commands) == 0 {
		t.Error("should return some commands")
	}

	found := false
	for _, cmd := range commands {
		if strings.Contains(cmd, "kubectl") && strings.Contains(cmd, "bibd") {
			found = true
			break
		}
	}
	if !found {
		t.Error("should have kubectl commands with namespace")
	}
}

func TestCLIStatus_Fields(t *testing.T) {
	status := CLIStatus{
		Nodes: []NodeStatus{
			{Address: "localhost:4000", Connected: true, Authenticated: true},
		},
		AllConnected:     true,
		AllAuthenticated: true,
		NetworkHealth:    NetworkHealthGood,
	}

	if !status.AllConnected {
		t.Error("AllConnected should be true")
	}
	if status.NetworkHealth != NetworkHealthGood {
		t.Error("NetworkHealth should be good")
	}
}

func TestNewCLIVerifier(t *testing.T) {
	nodes := []NodeConfig{
		{Address: "localhost:4000", Alias: "local"},
		{Address: "node1.example.com:4000", Alias: "remote"},
	}

	v := NewCLIVerifier(nodes)

	if len(v.Nodes) != 2 {
		t.Error("should have 2 nodes")
	}
}

func TestCLIVerifier_Verify(t *testing.T) {
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

	nodes := []NodeConfig{
		{Address: listener.Addr().String(), Alias: "test"},
	}

	v := NewCLIVerifier(nodes)
	ctx := context.Background()

	status := v.Verify(ctx)

	if !status.AllConnected {
		t.Error("AllConnected should be true")
	}

	if status.NetworkHealth != NetworkHealthGood {
		t.Errorf("NetworkHealth should be good, got %s", status.NetworkHealth)
	}
}

func TestFormatCLIStatus(t *testing.T) {
	status := &CLIStatus{
		Nodes: []NodeStatus{
			{Address: "localhost:4000", Alias: "local", Connected: true, Authenticated: true, Latency: 5 * time.Millisecond},
		},
		AllConnected:     true,
		AllAuthenticated: true,
		NetworkHealth:    NetworkHealthGood,
	}

	formatted := FormatCLIStatus(status)

	if !strings.Contains(formatted, "🟢") {
		t.Error("should show green indicator for good health")
	}
	if !strings.Contains(formatted, "local") {
		t.Error("should show node alias")
	}
	if !strings.Contains(formatted, "connected") {
		t.Error("should show connected status")
	}
}

func TestGetCLINextSteps(t *testing.T) {
	steps := GetCLINextSteps()

	if len(steps) == 0 {
		t.Error("should return some steps")
	}
}

func TestGetCLIHelpfulCommands(t *testing.T) {
	commands := GetCLIHelpfulCommands()

	if len(commands) == 0 {
		t.Error("should return some commands")
	}
}

func TestNetworkHealth(t *testing.T) {
	tests := []struct {
		health   NetworkHealth
		expected string
	}{
		{NetworkHealthGood, "good"},
		{NetworkHealthDegraded, "degraded"},
		{NetworkHealthPoor, "poor"},
		{NetworkHealthOffline, "offline"},
	}

	for _, tt := range tests {
		if string(tt.health) != tt.expected {
			t.Errorf("NetworkHealth %v should be %q", tt.health, tt.expected)
		}
	}
}
