package docker

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

	// We can't know if Docker is installed, but we can check the structure
	// If Docker is not installed, Available should be false
	if !info.Available && info.Error == "" {
		t.Error("if not available, should have error message")
	}

	// If available and daemon running, should have version
	if info.Available && info.DaemonRunning && info.Version == "" {
		t.Error("if daemon running, should have version")
	}
}

func TestDockerInfo_IsUsable(t *testing.T) {
	tests := []struct {
		name     string
		info     *DockerInfo
		expected bool
	}{
		{
			name: "fully usable",
			info: &DockerInfo{
				Available:        true,
				DaemonRunning:    true,
				ComposeAvailable: true,
			},
			expected: true,
		},
		{
			name: "not available",
			info: &DockerInfo{
				Available: false,
			},
			expected: false,
		},
		{
			name: "daemon not running",
			info: &DockerInfo{
				Available:     true,
				DaemonRunning: false,
			},
			expected: false,
		},
		{
			name: "no compose",
			info: &DockerInfo{
				Available:        true,
				DaemonRunning:    true,
				ComposeAvailable: false,
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

func TestDockerInfo_GetComposeCommand(t *testing.T) {
	tests := []struct {
		name     string
		info     *DockerInfo
		expected []string
	}{
		{
			name: "docker compose plugin",
			info: &DockerInfo{
				ComposeCommand: "docker compose",
			},
			expected: []string{"docker", "compose"},
		},
		{
			name: "docker-compose standalone",
			info: &DockerInfo{
				ComposeCommand: "docker-compose",
			},
			expected: []string{"docker-compose"},
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

func TestFormatDockerInfo(t *testing.T) {
	tests := []struct {
		name     string
		info     *DockerInfo
		contains []string
	}{
		{
			name: "not available",
			info: &DockerInfo{
				Available: false,
				Error:     "docker command not found",
			},
			contains: []string{"✗", "Not installed", "docker command not found"},
		},
		{
			name: "daemon not running",
			info: &DockerInfo{
				Available:     true,
				DaemonRunning: false,
				Version:       "24.0.0 (client only)",
				Error:         "daemon not running",
			},
			contains: []string{"⚠", "Daemon not running", "24.0.0"},
		},
		{
			name: "fully available",
			info: &DockerInfo{
				Available:        true,
				DaemonRunning:    true,
				Version:          "24.0.0",
				APIVersion:       "1.43",
				ServerOS:         "linux",
				ComposeAvailable: true,
				ComposeCommand:   "docker compose",
				ComposeVersion:   "2.20.0",
			},
			contains: []string{"✓", "Available", "24.0.0", "1.43", "linux", "Compose", "2.20.0"},
		},
		{
			name: "no compose",
			info: &DockerInfo{
				Available:        true,
				DaemonRunning:    true,
				Version:          "24.0.0",
				ComposeAvailable: false,
			},
			contains: []string{"✓", "24.0.0", "Compose: ✗"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDockerInfo(tt.info)
			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("FormatDockerInfo() missing %q in:\n%s", s, result)
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
