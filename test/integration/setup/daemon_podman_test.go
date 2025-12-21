package setup

import (
	"context"
	"testing"
	"time"

	"bib/internal/deploy/podman"
)

// TestPodmanDetection tests Podman detection
func TestPodmanDetection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	detector := podman.NewDetector()
	info := detector.Detect(ctx)

	// Info should not be nil
	if info == nil {
		t.Fatal("podman info is nil")
	}

	// Log available info
	t.Logf("Podman info: %+v", info)
}

// TestPodmanPodConfig tests pod config defaults
func TestPodmanPodConfig(t *testing.T) {
	cfg := podman.DefaultPodConfig()

	if cfg == nil {
		t.Fatal("default config is nil")
	}

	// Verify some defaults exist
	if cfg.P2PMode == "" {
		t.Error("P2P mode should have a default")
	}
}

// TestPodmanPodGeneratorCreation tests pod generator creation
func TestPodmanPodGeneratorCreation(t *testing.T) {
	cfg := podman.DefaultPodConfig()
	generator := podman.NewPodGenerator(cfg)

	if generator == nil {
		t.Fatal("generator is nil")
	}
}
