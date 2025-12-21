package kubernetes

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

	if config.OutputDir != "./bibd-kubernetes" {
		t.Errorf("expected output dir './bibd-kubernetes', got %q", config.OutputDir)
	}

	if !config.AutoApply {
		t.Error("expected AutoApply to be true")
	}

	if !config.WaitForReady {
		t.Error("expected WaitForReady to be true")
	}

	if config.WaitTimeout != 300*time.Second {
		t.Errorf("expected WaitTimeout 300s, got %v", config.WaitTimeout)
	}
}

func TestNewDeployer(t *testing.T) {
	t.Run("with config", func(t *testing.T) {
		config := &DeployConfig{
			OutputDir: "/custom/path",
			ManifestConfig: &ManifestConfig{
				Namespace: "test",
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
		if deployer.Config.OutputDir != "./bibd-kubernetes" {
			t.Error("should have default output dir")
		}
	})

	t.Run("nil manifest config", func(t *testing.T) {
		config := &DeployConfig{
			OutputDir: "/custom/path",
		}
		deployer := NewDeployer(config)

		if deployer.Config.ManifestConfig == nil {
			t.Error("manifest config should be initialized")
		}
	})
}

func TestDeployResult_Fields(t *testing.T) {
	result := DeployResult{
		Success:          true,
		OutputDir:        "/path/to/output",
		FilesGenerated:   []string{"namespace.yaml", "deployment.yaml"},
		ManifestsApplied: true,
		PodsReady:        true,
		ExternalIP:       "1.2.3.4",
		IngressURL:       "https://bibd.example.com",
		Error:            "",
		Logs:             []string{"Step 1", "Step 2"},
	}

	if !result.Success {
		t.Error("success should be true")
	}
	if len(result.FilesGenerated) != 2 {
		t.Error("files generated count mismatch")
	}
	if result.ExternalIP != "1.2.3.4" {
		t.Error("external IP mismatch")
	}
	if len(result.Logs) != 2 {
		t.Error("logs count mismatch")
	}
}

func TestDeploymentStatus_FormatStatus(t *testing.T) {
	t.Run("not generated", func(t *testing.T) {
		status := &DeploymentStatus{
			Generated: false,
		}

		formatted := status.FormatStatus()
		if !strings.Contains(formatted, "Not generated") {
			t.Error("should indicate not generated")
		}
	})

	t.Run("generated not deployed", func(t *testing.T) {
		status := &DeploymentStatus{
			Generated: true,
			Deployed:  false,
		}

		formatted := status.FormatStatus()
		if !strings.Contains(formatted, "Generated") {
			t.Error("should indicate generated")
		}
		if !strings.Contains(formatted, "not deployed") {
			t.Error("should indicate not deployed")
		}
	})

	t.Run("running", func(t *testing.T) {
		status := &DeploymentStatus{
			Generated: true,
			Deployed:  true,
			Running:   true,
			Namespace: "bibd",
			Pods: []PodStatus{
				{Name: "bibd-abc123", Phase: "Running", Ready: true},
			},
		}

		formatted := status.FormatStatus()
		if !strings.Contains(formatted, "Running") {
			t.Error("should indicate running")
		}
		if !strings.Contains(formatted, "bibd") {
			t.Error("should show namespace")
		}
		if !strings.Contains(formatted, "bibd-abc123") {
			t.Error("should list pod")
		}
	})

	t.Run("not ready", func(t *testing.T) {
		status := &DeploymentStatus{
			Generated: true,
			Deployed:  true,
			Running:   false,
			Namespace: "bibd",
			Pods: []PodStatus{
				{Name: "bibd-abc123", Phase: "Pending", Ready: false},
			},
		}

		formatted := status.FormatStatus()
		if !strings.Contains(formatted, "not ready") {
			t.Error("should indicate not ready")
		}
	})
}

func TestPodStatus_Fields(t *testing.T) {
	ps := PodStatus{
		Name:  "bibd-abc123",
		Phase: "Running",
		Ready: true,
	}

	if ps.Name != "bibd-abc123" {
		t.Error("name mismatch")
	}
	if ps.Phase != "Running" {
		t.Error("phase mismatch")
	}
	if !ps.Ready {
		t.Error("ready should be true")
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
		OutputDir:    tmpDir,
		AutoApply:    false, // Don't try to apply in test
		WaitForReady: false,
		ManifestConfig: &ManifestConfig{
			Namespace:      "test",
			Name:           "Test Node",
			Email:          "test@example.com",
			StorageBackend: "sqlite",
		},
	}

	deployer := NewDeployer(config)
	ctx := context.Background()

	result, err := deployer.Deploy(ctx)

	// If kubectl is not available, the deployment will fail early
	if err != nil {
		if strings.Contains(err.Error(), "kubectl is not available") ||
			strings.Contains(err.Error(), "kubectl command not found") {
			t.Skip("kubectl not available, skipping deployment test")
		}
		if strings.Contains(err.Error(), "cluster") {
			t.Skip("Kubernetes cluster not available, skipping")
		}
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	// Check that files were marked as generated
	if len(result.FilesGenerated) == 0 {
		t.Error("no files were generated")
	}

	// Verify files exist on disk
	for _, f := range result.FilesGenerated {
		path := filepath.Join(tmpDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("file %s was not written to disk", f)
		}
	}
}

func TestDeployConfig_Fields(t *testing.T) {
	config := DeployConfig{
		ManifestConfig: &ManifestConfig{
			Namespace: "test",
		},
		OutputDir:    "/custom/output",
		AutoApply:    false,
		WaitForReady: false,
		WaitTimeout:  60 * time.Second,
		Verbose:      true,
	}

	if config.OutputDir != "/custom/output" {
		t.Error("OutputDir mismatch")
	}
	if config.AutoApply {
		t.Error("AutoApply should be false")
	}
	if config.WaitTimeout != 60*time.Second {
		t.Error("WaitTimeout mismatch")
	}
}
