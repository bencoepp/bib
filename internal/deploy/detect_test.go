package deploy

import (
	"context"
	"testing"
	"time"
)

func TestNewTargetDetector(t *testing.T) {
	detector := NewTargetDetector()

	if detector == nil {
		t.Fatal("detector is nil")
	}

	if detector.Timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", detector.Timeout)
	}
}

func TestTargetDetector_WithTimeout(t *testing.T) {
	detector := NewTargetDetector().WithTimeout(10 * time.Second)

	if detector.Timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", detector.Timeout)
	}
}

func TestTargetType_Constants(t *testing.T) {
	tests := []struct {
		target   TargetType
		expected string
	}{
		{TargetLocal, "local"},
		{TargetDocker, "docker"},
		{TargetPodman, "podman"},
		{TargetKubernetes, "kubernetes"},
	}

	for _, tt := range tests {
		if string(tt.target) != tt.expected {
			t.Errorf("expected %q, got %q", tt.expected, string(tt.target))
		}
	}
}

func TestTargetDetector_DetectLocal(t *testing.T) {
	detector := NewTargetDetector()
	ctx := context.Background()

	info := detector.DetectLocal(ctx)

	if info == nil {
		t.Fatal("info is nil")
	}

	if info.Type != TargetLocal {
		t.Errorf("expected type %q, got %q", TargetLocal, info.Type)
	}

	// Local should always be available
	if !info.Available {
		t.Error("local should always be available")
	}

	if info.Details == nil {
		t.Error("details should not be nil")
	}

	// Should have OS info
	if info.Details["os"] == "" {
		t.Error("should have os in details")
	}

	if info.Details["arch"] == "" {
		t.Error("should have arch in details")
	}
}

func TestTargetDetector_DetectDocker(t *testing.T) {
	detector := NewTargetDetector()
	ctx := context.Background()

	info := detector.DetectDocker(ctx)

	if info == nil {
		t.Fatal("info is nil")
	}

	if info.Type != TargetDocker {
		t.Errorf("expected type %q, got %q", TargetDocker, info.Type)
	}

	// Status should be set
	if info.Status == "" {
		t.Error("status should be set")
	}

	// If not available, should have error
	if !info.Available && info.Error == "" {
		t.Error("if not available, should have error")
	}
}

func TestTargetDetector_DetectPodman(t *testing.T) {
	detector := NewTargetDetector()
	ctx := context.Background()

	info := detector.DetectPodman(ctx)

	if info == nil {
		t.Fatal("info is nil")
	}

	if info.Type != TargetPodman {
		t.Errorf("expected type %q, got %q", TargetPodman, info.Type)
	}

	if info.Status == "" {
		t.Error("status should be set")
	}
}

func TestTargetDetector_DetectKubernetes(t *testing.T) {
	detector := NewTargetDetector()
	ctx := context.Background()

	info := detector.DetectKubernetes(ctx)

	if info == nil {
		t.Fatal("info is nil")
	}

	if info.Type != TargetKubernetes {
		t.Errorf("expected type %q, got %q", TargetKubernetes, info.Type)
	}

	if info.Status == "" {
		t.Error("status should be set")
	}
}

func TestTargetDetector_DetectAll(t *testing.T) {
	detector := NewTargetDetector().WithTimeout(2 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results := detector.DetectAll(ctx)

	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	// Check each result has expected type
	types := make(map[TargetType]bool)
	for _, r := range results {
		if r == nil {
			t.Error("result is nil")
			continue
		}
		types[r.Type] = true
	}

	expectedTypes := []TargetType{TargetLocal, TargetDocker, TargetPodman, TargetKubernetes}
	for _, et := range expectedTypes {
		if !types[et] {
			t.Errorf("missing result for type %q", et)
		}
	}
}

func TestTargetInfo_Fields(t *testing.T) {
	info := TargetInfo{
		Type:      TargetDocker,
		Available: true,
		Version:   "24.0.0",
		Status:    "Available (v24.0.0)",
		Details: map[string]string{
			"compose": "docker compose",
		},
	}

	if info.Type != TargetDocker {
		t.Error("type mismatch")
	}
	if !info.Available {
		t.Error("available should be true")
	}
	if info.Version != "24.0.0" {
		t.Error("version mismatch")
	}
	if info.Details["compose"] != "docker compose" {
		t.Error("details compose mismatch")
	}
}

func TestFormatTargetInfo(t *testing.T) {
	tests := []struct {
		name     string
		info     *TargetInfo
		contains []string
	}{
		{
			name: "available docker",
			info: &TargetInfo{
				Type:      TargetDocker,
				Available: true,
				Status:    "Available (v24.0.0)",
			},
			contains: []string{"🐳", "✓", "Docker", "Available"},
		},
		{
			name: "unavailable kubernetes",
			info: &TargetInfo{
				Type:      TargetKubernetes,
				Available: false,
				Status:    "kubectl not installed",
			},
			contains: []string{"☸️", "✗", "Kubernetes", "not installed"},
		},
		{
			name: "local",
			info: &TargetInfo{
				Type:      TargetLocal,
				Available: true,
				Status:    "Available (linux/amd64)",
			},
			contains: []string{"🖥️", "✓", "Local"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := FormatTargetInfo(tt.info)
			for _, s := range tt.contains {
				if !containsStr(output, s) {
					t.Errorf("expected output to contain %q, got %q", s, output)
				}
			}
		})
	}
}

func TestTargetDisplayName(t *testing.T) {
	tests := []struct {
		target   TargetType
		expected string
	}{
		{TargetLocal, "Local Installation"},
		{TargetDocker, "Docker"},
		{TargetPodman, "Podman"},
		{TargetKubernetes, "Kubernetes"},
	}

	for _, tt := range tests {
		name := TargetDisplayName(tt.target)
		if name != tt.expected {
			t.Errorf("expected %q, got %q", tt.expected, name)
		}
	}
}

func TestTargetDescription(t *testing.T) {
	tests := []struct {
		target TargetType
	}{
		{TargetLocal},
		{TargetDocker},
		{TargetPodman},
		{TargetKubernetes},
	}

	for _, tt := range tests {
		desc := TargetDescription(tt.target)
		if desc == "" {
			t.Errorf("expected non-empty description for %q", tt.target)
		}
	}
}

func TestExtractKubectlVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "Client Version: v1.28.0",
			expected: "v1.28.0",
		},
		{
			input:    "Client Version: v1.28.0\nServer Version: v1.27.0",
			expected: "v1.28.0",
		},
		{
			input:    "simple version",
			expected: "simple version",
		},
	}

	for _, tt := range tests {
		result := extractKubectlVersion(tt.input)
		if result != tt.expected {
			t.Errorf("extractKubectlVersion(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
