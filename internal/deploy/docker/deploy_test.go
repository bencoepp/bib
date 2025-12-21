package docker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultDeployConfig(t *testing.T) {
	config := DefaultDeployConfig()

	if config == nil {
		t.Fatal("config is nil")
	}

	if config.OutputDir != "./bibd-docker" {
		t.Errorf("expected output dir './bibd-docker', got %q", config.OutputDir)
	}

	if !config.AutoStart {
		t.Error("expected AutoStart to be true")
	}

	if !config.PullImages {
		t.Error("expected PullImages to be true")
	}

	if !config.WaitForHealthy {
		t.Error("expected WaitForHealthy to be true")
	}

	if config.HealthTimeout != 120*time.Second {
		t.Errorf("expected HealthTimeout 120s, got %v", config.HealthTimeout)
	}
}

func TestNewDeployer(t *testing.T) {
	t.Run("with config", func(t *testing.T) {
		config := &DeployConfig{
			OutputDir: "/custom/path",
			ComposeConfig: &ComposeConfig{
				ProjectName: "test",
			},
		}
		deployer := NewDeployer(config)

		if deployer.Config.OutputDir != "/custom/path" {
			t.Error("config not set correctly")
		}
		if deployer.Generator == nil {
			t.Error("generator should be initialized")
		}
	})

	t.Run("nil config", func(t *testing.T) {
		deployer := NewDeployer(nil)

		if deployer.Config == nil {
			t.Error("should use default config when nil")
		}
		if deployer.Config.OutputDir != "./bibd-docker" {
			t.Error("should have default output dir")
		}
	})

	t.Run("nil compose config", func(t *testing.T) {
		config := &DeployConfig{
			OutputDir: "/custom/path",
		}
		deployer := NewDeployer(config)

		if deployer.Config.ComposeConfig == nil {
			t.Error("compose config should be initialized")
		}
	})
}

func TestDeployResult_Fields(t *testing.T) {
	result := DeployResult{
		Success:           true,
		OutputDir:         "/path/to/output",
		FilesGenerated:    []string{"docker-compose.yaml", ".env"},
		ContainersStarted: true,
		ContainersHealthy: true,
		Error:             "",
		Logs:              []string{"Step 1", "Step 2"},
	}

	if !result.Success {
		t.Error("success should be true")
	}
	if len(result.FilesGenerated) != 2 {
		t.Error("files generated count mismatch")
	}
	if len(result.Logs) != 2 {
		t.Error("logs count mismatch")
	}
}

func TestDeploymentStatus_FormatStatus(t *testing.T) {
	t.Run("not deployed", func(t *testing.T) {
		status := &DeploymentStatus{
			Deployed: false,
		}

		formatted := status.FormatStatus()
		if !strings.Contains(formatted, "Not deployed") {
			t.Error("should indicate not deployed")
		}
	})

	t.Run("running", func(t *testing.T) {
		status := &DeploymentStatus{
			Deployed: true,
			Running:  true,
			Containers: []ContainerStatus{
				{Name: "bibd", Status: "Up 5 minutes", Health: "healthy"},
				{Name: "postgres", Status: "Up 5 minutes", Health: "healthy"},
			},
		}

		formatted := status.FormatStatus()
		if !strings.Contains(formatted, "Running") {
			t.Error("should indicate running")
		}
		if !strings.Contains(formatted, "bibd") {
			t.Error("should list bibd container")
		}
		if !strings.Contains(formatted, "postgres") {
			t.Error("should list postgres container")
		}
	})

	t.Run("stopped", func(t *testing.T) {
		status := &DeploymentStatus{
			Deployed: true,
			Running:  false,
			Containers: []ContainerStatus{
				{Name: "bibd", Status: "Exited (0)", Health: ""},
			},
		}

		formatted := status.FormatStatus()
		if !strings.Contains(formatted, "Stopped") {
			t.Error("should indicate stopped")
		}
	})

	t.Run("unhealthy", func(t *testing.T) {
		status := &DeploymentStatus{
			Deployed: true,
			Running:  true,
			Containers: []ContainerStatus{
				{Name: "bibd", Status: "Up 5 minutes", Health: "unhealthy"},
			},
		}

		formatted := status.FormatStatus()
		if !strings.Contains(formatted, "✗") {
			t.Error("should show unhealthy icon")
		}
	})
}

func TestContainerStatus_Fields(t *testing.T) {
	cs := ContainerStatus{
		Name:   "bibd-bibd",
		Status: "Up 10 minutes",
		Health: "healthy",
	}

	if cs.Name != "bibd-bibd" {
		t.Error("name mismatch")
	}
	if cs.Status != "Up 10 minutes" {
		t.Error("status mismatch")
	}
	if cs.Health != "healthy" {
		t.Error("health mismatch")
	}
}

func TestDeployer_generateIdentityKey(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "config", "identity.pem")

	deployer := NewDeployer(nil)

	err := deployer.generateIdentityKey(keyPath)
	if err != nil {
		t.Fatalf("generateIdentityKey failed: %v", err)
	}

	// Check file exists
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Error("identity key file not created")
	}

	// Check file permissions (on Unix-like systems)
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	// File should be created (permissions check is platform-specific)
	if info.Size() == 0 {
		t.Error("identity key file is empty")
	}
}

func TestDeployer_log(t *testing.T) {
	deployer := NewDeployer(&DeployConfig{
		Verbose: false,
	})

	result := &DeployResult{
		Logs: make([]string, 0),
	}

	deployer.log(result, "Test message 1")
	deployer.log(result, "Test message 2")

	if len(result.Logs) != 2 {
		t.Errorf("expected 2 logs, got %d", len(result.Logs))
	}

	if result.Logs[0] != "Test message 1" {
		t.Error("log message mismatch")
	}
}

func TestDeployer_Deploy_GeneratesFiles(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	config := &DeployConfig{
		OutputDir:  tmpDir,
		AutoStart:  false, // Don't try to start containers in test
		PullImages: false,
		ComposeConfig: &ComposeConfig{
			ProjectName:    "test",
			Name:           "Test Node",
			Email:          "test@example.com",
			StorageBackend: "sqlite",
		},
	}

	deployer := NewDeployer(config)
	ctx := context.Background()

	result, err := deployer.Deploy(ctx)

	// If Docker is not available, the deployment will fail early
	// That's okay for this test - we just want to verify the logic
	if err != nil {
		if strings.Contains(err.Error(), "Docker is not available") ||
			strings.Contains(err.Error(), "docker command not found") {
			t.Skip("Docker not available, skipping deployment test")
		}
		// For other errors in CI where docker might be installed but not running
		if strings.Contains(err.Error(), "daemon") {
			t.Skip("Docker daemon not running, skipping deployment test")
		}
	}

	// If we got here and result is nil, something went wrong
	if result == nil {
		t.Fatal("result is nil")
	}

	// Check that files were marked as generated
	if len(result.FilesGenerated) == 0 {
		t.Error("no files were generated")
	}
}

func TestDeployConfig_Fields(t *testing.T) {
	config := DeployConfig{
		ComposeConfig: &ComposeConfig{
			ProjectName: "test",
		},
		OutputDir:      "/custom/output",
		AutoStart:      false,
		PullImages:     false,
		WaitForHealthy: false,
		HealthTimeout:  60 * time.Second,
		Verbose:        true,
	}

	if config.OutputDir != "/custom/output" {
		t.Error("OutputDir mismatch")
	}
	if config.AutoStart {
		t.Error("AutoStart should be false")
	}
	if config.HealthTimeout != 60*time.Second {
		t.Error("HealthTimeout mismatch")
	}
}
