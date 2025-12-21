package kubernetes

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNewDetector(t *testing.T) {
	detector := NewDetector()

	if detector == nil {
		t.Fatal("detector is nil")
	}

	if detector.Timeout != 15*time.Second {
		t.Errorf("expected timeout 15s, got %v", detector.Timeout)
	}
}

func TestDetector_WithTimeout(t *testing.T) {
	detector := NewDetector().WithTimeout(5 * time.Second)

	if detector.Timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", detector.Timeout)
	}
}

func TestDetector_WithKubeconfig(t *testing.T) {
	detector := NewDetector().WithKubeconfig("/path/to/kubeconfig")

	if detector.Kubeconfig != "/path/to/kubeconfig" {
		t.Errorf("expected kubeconfig '/path/to/kubeconfig', got %q", detector.Kubeconfig)
	}
}

func TestDetector_WithContext(t *testing.T) {
	detector := NewDetector().WithContext("my-context")

	if detector.Context != "my-context" {
		t.Errorf("expected context 'my-context', got %q", detector.Context)
	}
}

func TestDetector_Detect(t *testing.T) {
	detector := NewDetector().WithTimeout(5 * time.Second)
	ctx := context.Background()

	info := detector.Detect(ctx)

	if info == nil {
		t.Fatal("info is nil")
	}

	// We can't know if kubectl is installed, but we can check the structure
	if !info.Available && info.Error == "" {
		t.Error("if not available, should have error message")
	}

	// If available and reachable, should have server info
	if info.Available && info.ClusterReachable && info.ServerVersion == "" {
		t.Error("if cluster reachable, should have server version")
	}
}

func TestKubeInfo_IsUsable(t *testing.T) {
	tests := []struct {
		name     string
		info     *KubeInfo
		expected bool
	}{
		{
			name: "fully usable",
			info: &KubeInfo{
				Available:        true,
				ClusterReachable: true,
			},
			expected: true,
		},
		{
			name: "not available",
			info: &KubeInfo{
				Available: false,
			},
			expected: false,
		},
		{
			name: "cluster not reachable",
			info: &KubeInfo{
				Available:        true,
				ClusterReachable: false,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.info.IsUsable()
			if result != tt.expected {
				t.Errorf("IsUsable() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestFormatKubeInfo(t *testing.T) {
	tests := []struct {
		name     string
		info     *KubeInfo
		contains []string
	}{
		{
			name: "not available",
			info: &KubeInfo{
				Available: false,
				Error:     "kubectl command not found",
			},
			contains: []string{"✗", "kubectl not installed", "kubectl command not found"},
		},
		{
			name: "cluster not reachable",
			info: &KubeInfo{
				Available:        true,
				ClusterReachable: false,
				CurrentContext:   "my-context",
				ClientVersion:    "v1.28.0",
				Error:            "connection refused",
			},
			contains: []string{"⚠", "not reachable", "my-context", "v1.28.0"},
		},
		{
			name: "fully available",
			info: &KubeInfo{
				Available:                  true,
				ClusterReachable:           true,
				CurrentContext:             "production",
				ClusterName:                "prod-cluster",
				ServerURL:                  "https://k8s.example.com:6443",
				ServerVersion:              "v1.28.0",
				Namespace:                  "default",
				CloudNativePGAvailable:     true,
				CloudNativePGVersion:       "1.20.0",
				IngressControllerAvailable: true,
				IngressClassName:           "nginx",
				StorageClasses:             []string{"standard", "fast"},
				DefaultStorageClass:        "standard",
			},
			contains: []string{"✓", "Available", "production", "prod-cluster", "v1.28.0", "CloudNativePG", "nginx", "standard"},
		},
		{
			name: "no ingress",
			info: &KubeInfo{
				Available:                  true,
				ClusterReachable:           true,
				CurrentContext:             "test",
				ClusterName:                "test-cluster",
				ServerURL:                  "https://localhost:6443",
				ServerVersion:              "v1.28.0",
				IngressControllerAvailable: false,
			},
			contains: []string{"Ingress: ✗"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatKubeInfo(tt.info)
			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("FormatKubeInfo() missing %q in:\n%s", s, result)
				}
			}
		})
	}
}

func TestDetector_commandExists(t *testing.T) {
	detector := NewDetector()

	// Test with a command that should exist on all platforms
	var cmdToTest string
	if runtime.GOOS == "windows" {
		cmdToTest = "cmd"
	} else {
		cmdToTest = "sh"
	}

	if !detector.commandExists(cmdToTest) {
		t.Errorf("expected %s to exist", cmdToTest)
	}

	// Test with a command that shouldn't exist
	if detector.commandExists("this-command-definitely-does-not-exist-12345") {
		t.Error("expected non-existent command to return false")
	}
}

func TestKubeInfo_Fields(t *testing.T) {
	info := KubeInfo{
		Available:                  true,
		ClusterReachable:           true,
		CurrentContext:             "prod",
		ClusterName:                "production",
		ServerURL:                  "https://k8s.example.com:6443",
		ServerVersion:              "v1.28.0",
		ClientVersion:              "v1.28.0",
		Namespace:                  "bibd",
		CloudNativePGAvailable:     true,
		CloudNativePGVersion:       "1.20.0",
		IngressControllerAvailable: true,
		IngressClassName:           "nginx",
		StorageClasses:             []string{"standard", "fast", "slow"},
		DefaultStorageClass:        "standard",
		Error:                      "",
	}

	if !info.Available {
		t.Error("Available should be true")
	}
	if info.CurrentContext != "prod" {
		t.Error("CurrentContext mismatch")
	}
	if info.ServerVersion != "v1.28.0" {
		t.Error("ServerVersion mismatch")
	}
	if !info.CloudNativePGAvailable {
		t.Error("CloudNativePGAvailable should be true")
	}
	if len(info.StorageClasses) != 3 {
		t.Error("StorageClasses count mismatch")
	}
	if info.DefaultStorageClass != "standard" {
		t.Error("DefaultStorageClass mismatch")
	}
}
