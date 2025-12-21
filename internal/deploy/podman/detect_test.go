package podman

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

	if detector.Timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", detector.Timeout)
	}
}

func TestDetector_WithTimeout(t *testing.T) {
	detector := NewDetector().WithTimeout(5 * time.Second)

	if detector.Timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", detector.Timeout)
	}
}

func TestDetector_Detect(t *testing.T) {
	detector := NewDetector().WithTimeout(5 * time.Second)
	ctx := context.Background()

	info := detector.Detect(ctx)

	if info == nil {
		t.Fatal("info is nil")
	}

	// We can't know if Podman is installed, but we can check the structure
	if !info.Available && info.Error == "" {
		t.Error("if not available, should have error message")
	}

	// If available, should have version
	if info.Available && info.Version == "" {
		t.Error("if available, should have version")
	}
}

func TestPodmanInfo_IsUsable(t *testing.T) {
	tests := []struct {
		name     string
		info     *PodmanInfo
		expected bool
	}{
		{
			name: "with compose",
			info: &PodmanInfo{
				Available:        true,
				ComposeAvailable: true,
			},
			expected: true,
		},
		{
			name: "with kube play",
			info: &PodmanInfo{
				Available:         true,
				KubePlayAvailable: true,
			},
			expected: true,
		},
		{
			name: "with both",
			info: &PodmanInfo{
				Available:         true,
				ComposeAvailable:  true,
				KubePlayAvailable: true,
			},
			expected: true,
		},
		{
			name: "not available",
			info: &PodmanInfo{
				Available: false,
			},
			expected: false,
		},
		{
			name: "no compose or kube",
			info: &PodmanInfo{
				Available:         true,
				ComposeAvailable:  false,
				KubePlayAvailable: false,
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

func TestPodmanInfo_GetComposeCommand(t *testing.T) {
	tests := []struct {
		name     string
		info     *PodmanInfo
		expected []string
	}{
		{
			name: "podman compose plugin",
			info: &PodmanInfo{
				ComposeCommand: "podman compose",
			},
			expected: []string{"podman", "compose"},
		},
		{
			name: "podman-compose standalone",
			info: &PodmanInfo{
				ComposeCommand: "podman-compose",
			},
			expected: []string{"podman-compose"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.info.GetComposeCommand()
			if len(result) != len(tt.expected) {
				t.Errorf("GetComposeCommand() = %v, expected %v", result, tt.expected)
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("GetComposeCommand()[%d] = %q, expected %q", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestPodmanInfo_PreferredDeployMethod(t *testing.T) {
	tests := []struct {
		name     string
		info     *PodmanInfo
		expected string
	}{
		{
			name: "prefers kube",
			info: &PodmanInfo{
				ComposeAvailable:  true,
				KubePlayAvailable: true,
			},
			expected: "kube",
		},
		{
			name: "compose only",
			info: &PodmanInfo{
				ComposeAvailable:  true,
				KubePlayAvailable: false,
			},
			expected: "compose",
		},
		{
			name: "kube only",
			info: &PodmanInfo{
				ComposeAvailable:  false,
				KubePlayAvailable: true,
			},
			expected: "kube",
		},
		{
			name: "neither",
			info: &PodmanInfo{
				ComposeAvailable:  false,
				KubePlayAvailable: false,
			},
			expected: "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.info.PreferredDeployMethod()
			if result != tt.expected {
				t.Errorf("PreferredDeployMethod() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestFormatPodmanInfo(t *testing.T) {
	tests := []struct {
		name     string
		info     *PodmanInfo
		contains []string
	}{
		{
			name: "not available",
			info: &PodmanInfo{
				Available: false,
				Error:     "podman command not found",
			},
			contains: []string{"✗", "Not installed", "podman command not found"},
		},
		{
			name: "rootless mode",
			info: &PodmanInfo{
				Available:        true,
				Version:          "4.7.0",
				Rootless:         true,
				RootlessUID:      1000,
				ComposeAvailable: true,
				ComposeCommand:   "podman compose",
				ComposeVersion:   "1.0.0",
			},
			contains: []string{"✓", "Available", "4.7.0", "Rootless", "1000", "Compose"},
		},
		{
			name: "rootful mode",
			info: &PodmanInfo{
				Available:         true,
				Version:           "4.7.0",
				Rootless:          false,
				KubePlayAvailable: true,
			},
			contains: []string{"✓", "4.7.0", "Rootful", "Kube Play"},
		},
		{
			name: "with machine",
			info: &PodmanInfo{
				Available:      true,
				Version:        "4.7.0",
				MachineRunning: true,
				MachineName:    "podman-machine-default",
			},
			contains: []string{"Machine", "podman-machine-default", "running"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatPodmanInfo(tt.info)
			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("FormatPodmanInfo() missing %q in:\n%s", s, result)
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

func TestDetector_isRootless(t *testing.T) {
	detector := NewDetector()

	// Just verify it doesn't panic
	result := detector.isRootless()

	// On most test systems, we should be rootless
	// (unless running as root)
	_ = result
}

func TestPodmanInfo_Fields(t *testing.T) {
	info := PodmanInfo{
		Available:              true,
		Version:                "4.7.0",
		APIVersion:             "4.7.0",
		Rootless:               true,
		RootlessUID:            1000,
		SocketPath:             "unix:///run/user/1000/podman/podman.sock",
		MachineRunning:         true,
		MachineName:            "default",
		ComposeAvailable:       true,
		ComposeVersion:         "1.0.0",
		ComposeCommand:         "podman compose",
		PodmanComposeAvailable: true,
		KubePlayAvailable:      true,
		Error:                  "",
	}

	if !info.Available {
		t.Error("Available should be true")
	}
	if info.Version != "4.7.0" {
		t.Error("Version mismatch")
	}
	if !info.Rootless {
		t.Error("Rootless should be true")
	}
	if info.RootlessUID != 1000 {
		t.Error("RootlessUID mismatch")
	}
	if !info.KubePlayAvailable {
		t.Error("KubePlayAvailable should be true")
	}
}
