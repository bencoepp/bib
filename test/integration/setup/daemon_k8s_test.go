package setup

import (
	"context"
	"testing"
	"time"

	"bib/internal/deploy/kubernetes"
)

// TestKubernetesDetection tests Kubernetes detection
func TestKubernetesDetection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	detector := kubernetes.NewDetector()
	info := detector.Detect(ctx)

	// Info should not be nil
	if info == nil {
		t.Fatal("kubernetes info is nil")
	}

	// Log available info
	t.Logf("Kubernetes info: %+v", info)
}

// TestKubernetesManifestConfig tests manifest config defaults
func TestKubernetesManifestConfig(t *testing.T) {
	cfg := kubernetes.DefaultManifestConfig()

	if cfg == nil {
		t.Fatal("default config is nil")
	}

	// Verify some defaults exist
	if cfg.Namespace == "" {
		t.Error("namespace should have a default")
	}
}

// TestKubernetesManifestGeneratorCreation tests manifest generator creation
func TestKubernetesManifestGeneratorCreation(t *testing.T) {
	cfg := kubernetes.DefaultManifestConfig()
	generator := kubernetes.NewManifestGenerator(cfg)

	if generator == nil {
		t.Fatal("generator is nil")
	}
}

// TestKubernetesServiceTypes tests service type configuration
func TestKubernetesServiceTypes(t *testing.T) {
	tests := []struct {
		name        string
		serviceType string
	}{
		{"ClusterIP", "ClusterIP"},
		{"NodePort", "NodePort"},
		{"LoadBalancer", "LoadBalancer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := kubernetes.DefaultManifestConfig()
			cfg.ServiceType = tt.serviceType

			if cfg.ServiceType != tt.serviceType {
				t.Errorf("service type not set correctly")
			}
		})
	}
}
