// Package docker provides Docker deployment utilities for bibd.
package docker

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// DockerInfo contains information about the Docker installation
type DockerInfo struct {
	// Available indicates if Docker is available
	Available bool

	// DaemonRunning indicates if the Docker daemon is running
	DaemonRunning bool

	// Version is the Docker version
	Version string

	// APIVersion is the Docker API version
	APIVersion string

	// ComposeAvailable indicates if docker-compose or docker compose is available
	ComposeAvailable bool

	// ComposeVersion is the docker-compose version
	ComposeVersion string

	// ComposeCommand is the command to use for compose ("docker-compose" or "docker compose")
	ComposeCommand string

	// ServerOS is the server operating system
	ServerOS string

	// Error contains any error message
	Error string
}

// Detector detects Docker installation and capabilities
type Detector struct {
	// Timeout for detection commands
	Timeout time.Duration
}

// NewDetector creates a new Docker detector
func NewDetector() *Detector {
	return &Detector{
		Timeout: 10 * time.Second,
	}
}

// WithTimeout sets the detection timeout
func (d *Detector) WithTimeout(timeout time.Duration) *Detector {
	d.Timeout = timeout
	return d
}

// Detect detects Docker installation and capabilities
func (d *Detector) Detect(ctx context.Context) *DockerInfo {
	info := &DockerInfo{}

	// Check if docker command exists
	if !d.commandExists("docker") {
		info.Available = false
		info.Error = "docker command not found"
		return info
	}
	info.Available = true

	// Check Docker version
	version, err := d.runCommand(ctx, "docker", "version", "--format", "{{.Server.Version}}")
	if err != nil {
		// Docker command exists but daemon might not be running
		info.DaemonRunning = false
		info.Error = fmt.Sprintf("Docker daemon not running: %v", err)

		// Try to get client version at least
		if clientVersion, err := d.runCommand(ctx, "docker", "version", "--format", "{{.Client.Version}}"); err == nil {
			info.Version = strings.TrimSpace(clientVersion) + " (client only)"
		}
		return info
	}

	info.DaemonRunning = true
	info.Version = strings.TrimSpace(version)

	// Get API version
	if apiVersion, err := d.runCommand(ctx, "docker", "version", "--format", "{{.Server.APIVersion}}"); err == nil {
		info.APIVersion = strings.TrimSpace(apiVersion)
	}

	// Get server OS
	if serverOS, err := d.runCommand(ctx, "docker", "version", "--format", "{{.Server.Os}}"); err == nil {
		info.ServerOS = strings.TrimSpace(serverOS)
	}

	// Check for docker-compose (standalone)
	if d.commandExists("docker-compose") {
		info.ComposeAvailable = true
		info.ComposeCommand = "docker-compose"
		if composeVersion, err := d.runCommand(ctx, "docker-compose", "version", "--short"); err == nil {
			info.ComposeVersion = strings.TrimSpace(composeVersion)
		}
	}

	// Check for docker compose (plugin) - preferred
	if _, err := d.runCommand(ctx, "docker", "compose", "version"); err == nil {
		info.ComposeAvailable = true
		info.ComposeCommand = "docker compose"
		if composeVersion, err := d.runCommand(ctx, "docker", "compose", "version", "--short"); err == nil {
			info.ComposeVersion = strings.TrimSpace(composeVersion)
		}
	}

	return info
}

// commandExists checks if a command exists in PATH
func (d *Detector) commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// runCommand runs a command and returns its output
func (d *Detector) runCommand(ctx context.Context, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, d.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// FormatDockerInfo formats Docker info for display
func FormatDockerInfo(info *DockerInfo) string {
	var sb strings.Builder

	if !info.Available {
		sb.WriteString("🐳 Docker: ✗ Not installed\n")
		if info.Error != "" {
			sb.WriteString(fmt.Sprintf("   Error: %s\n", info.Error))
		}
		return sb.String()
	}

	if !info.DaemonRunning {
		sb.WriteString("🐳 Docker: ⚠ Daemon not running\n")
		if info.Version != "" {
			sb.WriteString(fmt.Sprintf("   Version: %s\n", info.Version))
		}
		if info.Error != "" {
			sb.WriteString(fmt.Sprintf("   Error: %s\n", info.Error))
		}
		return sb.String()
	}

	sb.WriteString("🐳 Docker: ✓ Available\n")
	sb.WriteString(fmt.Sprintf("   Version: %s\n", info.Version))
	if info.APIVersion != "" {
		sb.WriteString(fmt.Sprintf("   API Version: %s\n", info.APIVersion))
	}
	if info.ServerOS != "" {
		sb.WriteString(fmt.Sprintf("   Server OS: %s\n", info.ServerOS))
	}

	if info.ComposeAvailable {
		sb.WriteString(fmt.Sprintf("   Compose: ✓ %s (v%s)\n", info.ComposeCommand, info.ComposeVersion))
	} else {
		sb.WriteString("   Compose: ✗ Not available\n")
	}

	return sb.String()
}

// IsUsable returns true if Docker can be used for deployment
func (info *DockerInfo) IsUsable() bool {
	return info.Available && info.DaemonRunning && info.ComposeAvailable
}

// GetComposeCommand returns the compose command parts
func (info *DockerInfo) GetComposeCommand() []string {
	if info.ComposeCommand == "docker compose" {
		return []string{"docker", "compose"}
	}
	return []string{"docker-compose"}
}
