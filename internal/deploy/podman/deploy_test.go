package podman

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

	if config.OutputDir != "./bibd-podman" {
		t.Errorf("expected output dir './bibd-podman', got %q", config.OutputDir)
	}

	if !config.AutoStart {
		t.Error("expected AutoStart to be true")
	}

	if !config.PullImages {
		t.Error("expected PullImages to be true")
	}

	if !config.WaitForRunning {
		t.Error("expected WaitForRunning to be true")
	}

	if config.WaitTimeout != 120*time.Second {
		t.Errorf("expected WaitTimeout 120s, got %v", config.WaitTimeout)
	}
}

func TestNewDeployer(t *testing.T) {
	t.Run("with config", func(t *testing.T) {
		config := &DeployConfig{
			OutputDir: "/custom/path",
			PodConfig: &PodConfig{
				PodName: "test",
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
		if deployer.Config.OutputDir != "./bibd-podman" {
			t.Error("should have default output dir")
		}
	})

	t.Run("nil pod config", func(t *testing.T) {
		config := &DeployConfig{
			OutputDir: "/custom/path",
		}
		deployer := NewDeployer(config)

		if deployer.Config.PodConfig == nil {
			t.Error("pod config should be initialized")
		}
	})
}

func TestDeployResult_Fields(t *testing.T) {
	result := DeployResult{
		Success:           true,
		OutputDir:         "/path/to/output",
		FilesGenerated:    []string{"pod.yaml", ".env"},
		ContainersStarted: true,
		ContainersRunning: true,
		DeployStyle:       "pod",
		Error:             "",
		Logs:              []string{"Step 1", "Step 2"},
	}

	if !result.Success {
		t.Error("success should be true")
	}
	if len(result.FilesGenerated) != 2 {
		t.Error("files generated count mismatch")
	}
	if result.DeployStyle != "pod" {
		t.Error("deploy style mismatch")
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
			Deployed:    true,
			Running:     true,
			DeployStyle: "pod",
			Containers: []ContainerStatus{
				{Name: "bibd", Status: "Up 5 minutes"},
				{Name: "postgres", Status: "Up 5 minutes"},
			},
		}

		formatted := status.FormatStatus()
		if !strings.Contains(formatted, "Running") {
			t.Error("should indicate running")
		}
		if !strings.Contains(formatted, "bibd") {
			t.Error("should list bibd container")
		}
		if !strings.Contains(formatted, "pod") {
			t.Error("should show deploy style")
		}
	})

	t.Run("stopped", func(t *testing.T) {
		status := &DeploymentStatus{
			Deployed:    true,
			Running:     false,
			DeployStyle: "compose",
			Containers: []ContainerStatus{
				{Name: "bibd", Status: "Exited (0)"},
			},
		}

		formatted := status.FormatStatus()
		if !strings.Contains(formatted, "Stopped") {
			t.Error("should indicate stopped")
		}
		if !strings.Contains(formatted, "✗") {
			t.Error("should show exited icon")
		}
	})
}

func TestContainerStatus_Fields(t *testing.T) {
	cs := ContainerStatus{
		Name:   "bibd-bibd",
		Status: "Up 10 minutes",
	}

	if cs.Name != "bibd-bibd" {
		t.Error("name mismatch")
	}
	if cs.Status != "Up 10 minutes" {
		t.Error("status mismatch")
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

	// Check file has content
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
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
		OutputDir:      tmpDir,
		AutoStart:      false, // Don't try to start containers in test
		PullImages:     false,
		WaitForRunning: false,
		PodConfig: &PodConfig{
			PodName:        "test",
			Name:           "Test Node",
			Email:          "test@example.com",
			StorageBackend: "sqlite",
			DeployStyle:    "pod",
		},
	}

	deployer := NewDeployer(config)
	ctx := context.Background()

	result, err := deployer.Deploy(ctx)

	// If Podman is not available, the deployment will fail early
	if err != nil {
		if strings.Contains(err.Error(), "Podman is not available") ||
			strings.Contains(err.Error(), "podman command not found") {
			t.Skip("Podman not available, skipping deployment test")
		}
		if strings.Contains(err.Error(), "neither compose nor kube") {
			t.Skip("Podman available but no compose/kube, skipping")
		}
	}

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
		PodConfig: &PodConfig{
			PodName: "test",
		},
		OutputDir:      "/custom/output",
		AutoStart:      false,
		PullImages:     false,
		WaitForRunning: false,
		WaitTimeout:    60 * time.Second,
		Verbose:        true,
	}

	if config.OutputDir != "/custom/output" {
		t.Error("OutputDir mismatch")
	}
	if config.AutoStart {
		t.Error("AutoStart should be false")
	}
	if config.WaitTimeout != 60*time.Second {
		t.Error("WaitTimeout mismatch")
	}
}

func TestDeploymentStatus_Fields(t *testing.T) {
	status := DeploymentStatus{
		Deployed:    true,
		Running:     true,
		DeployStyle: "pod",
		Containers: []ContainerStatus{
			{Name: "bibd", Status: "Running"},
		},
	}

	if !status.Deployed {
		t.Error("Deployed should be true")
	}
	if !status.Running {
		t.Error("Running should be true")
	}
	if status.DeployStyle != "pod" {
		t.Error("DeployStyle mismatch")
	}
	if len(status.Containers) != 1 {
		t.Error("Containers count mismatch")
	}
}
