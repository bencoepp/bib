package setup

import (
	"context"
	"testing"
	"time"

	"bib/internal/deploy/docker"
)

// TestDockerDetection tests Docker detection
func TestDockerDetection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	detector := docker.NewDetector()
	info := detector.Detect(ctx)

	// Info should not be nil
	if info == nil {
		t.Fatal("docker info is nil")
	}

	// Log available info
	t.Logf("Docker info: %+v", info)
}

// TestDockerComposeConfig tests Docker Compose config defaults
func TestDockerComposeConfig(t *testing.T) {
	cfg := docker.DefaultComposeConfig()

	if cfg == nil {
		t.Fatal("default config is nil")
	}

	// Verify some defaults exist
	if cfg.P2PMode == "" {
		t.Error("P2P mode should have a default")
	}
}

// TestDockerComposeGeneratorCreation tests compose generator creation
func TestDockerComposeGeneratorCreation(t *testing.T) {
	cfg := docker.DefaultComposeConfig()
	generator := docker.NewComposeGenerator(cfg)

	if generator == nil {
		t.Fatal("generator is nil")
	}
}
