// Package deploy provides deployment detection and deployment utilities.
package deploy

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// TargetType represents a deployment target type
type TargetType string

const (
	TargetLocal      TargetType = "local"
	TargetDocker     TargetType = "docker"
	TargetPodman     TargetType = "podman"
	TargetKubernetes TargetType = "kubernetes"
)

// TargetInfo contains information about a deployment target
type TargetInfo struct {
	// Type is the target type
	Type TargetType

	// Available indicates if the target is available on this system
	Available bool

	// Version is the version string of the tool (if available)
	Version string

	// Status is a human-readable status message
	Status string

	// Error contains any error encountered during detection
	Error string

	// Details contains additional target-specific information
	Details map[string]string
}

// TargetDetector detects available deployment targets
type TargetDetector struct {
	// Timeout for detection commands
	Timeout time.Duration
}

// NewTargetDetector creates a new target detector
func NewTargetDetector() *TargetDetector {
	return &TargetDetector{
		Timeout: 5 * time.Second,
	}
}

// WithTimeout sets the detection timeout
func (d *TargetDetector) WithTimeout(timeout time.Duration) *TargetDetector {
	d.Timeout = timeout
	return d
}

// DetectAll detects all available deployment targets
func (d *TargetDetector) DetectAll(ctx context.Context) []*TargetInfo {
	results := make([]*TargetInfo, 4)

	// Detect all targets in parallel
	done := make(chan struct{})
	go func() {
		results[0] = d.DetectLocal(ctx)
		done <- struct{}{}
	}()
	go func() {
		results[1] = d.DetectDocker(ctx)
		done <- struct{}{}
	}()
	go func() {
		results[2] = d.DetectPodman(ctx)
		done <- struct{}{}
	}()
	go func() {
		results[3] = d.DetectKubernetes(ctx)
		done <- struct{}{}
	}()

	// Wait for all detections to complete
	for i := 0; i < 4; i++ {
		<-done
	}

	return results
}

// DetectLocal checks if local deployment is available
func (d *TargetDetector) DetectLocal(ctx context.Context) *TargetInfo {
	info := &TargetInfo{
		Type:      TargetLocal,
		Available: true, // Local is always available
		Status:    "Available",
		Details:   make(map[string]string),
	}

	// Add OS info
	info.Details["os"] = runtime.GOOS
	info.Details["arch"] = runtime.GOARCH

	// Check for init system on Linux
	if runtime.GOOS == "linux" {
		if d.commandExists(ctx, "systemctl") {
			info.Details["init"] = "systemd"
		} else if d.commandExists(ctx, "service") {
			info.Details["init"] = "sysvinit"
		}
	} else if runtime.GOOS == "darwin" {
		info.Details["init"] = "launchd"
	} else if runtime.GOOS == "windows" {
		info.Details["init"] = "windows-service"
	}

	info.Status = fmt.Sprintf("Available (%s/%s)", runtime.GOOS, runtime.GOARCH)

	return info
}

// DetectDocker checks if Docker is available
func (d *TargetDetector) DetectDocker(ctx context.Context) *TargetInfo {
	info := &TargetInfo{
		Type:    TargetDocker,
		Details: make(map[string]string),
	}

	// Check if docker command exists
	if !d.commandExists(ctx, "docker") {
		info.Available = false
		info.Status = "Not installed"
		info.Error = "docker command not found"
		return info
	}

	// Get docker version
	version, err := d.runCommand(ctx, "docker", "version", "--format", "{{.Server.Version}}")
	if err != nil {
		// Docker command exists but daemon might not be running
		info.Available = false
		info.Status = "Daemon not running"
		info.Error = err.Error()
		return info
	}

	info.Available = true
	info.Version = strings.TrimSpace(version)
	info.Status = fmt.Sprintf("Available (v%s)", info.Version)

	// Get additional info
	if apiVersion, err := d.runCommand(ctx, "docker", "version", "--format", "{{.Server.APIVersion}}"); err == nil {
		info.Details["api_version"] = strings.TrimSpace(apiVersion)
	}

	// Check docker compose
	if d.commandExists(ctx, "docker-compose") {
		info.Details["compose"] = "docker-compose"
	} else {
		// Check for docker compose plugin
		if _, err := d.runCommand(ctx, "docker", "compose", "version"); err == nil {
			info.Details["compose"] = "docker compose"
		}
	}

	return info
}

// DetectPodman checks if Podman is available
func (d *TargetDetector) DetectPodman(ctx context.Context) *TargetInfo {
	info := &TargetInfo{
		Type:    TargetPodman,
		Details: make(map[string]string),
	}

	// Check if podman command exists
	if !d.commandExists(ctx, "podman") {
		info.Available = false
		info.Status = "Not installed"
		info.Error = "podman command not found"
		return info
	}

	// Get podman version
	version, err := d.runCommand(ctx, "podman", "version", "--format", "{{.Version}}")
	if err != nil {
		// Try alternative format
		version, err = d.runCommand(ctx, "podman", "--version")
		if err != nil {
			info.Available = false
			info.Status = "Error getting version"
			info.Error = err.Error()
			return info
		}
		// Parse "podman version X.Y.Z"
		parts := strings.Fields(version)
		if len(parts) >= 3 {
			version = parts[2]
		}
	}

	info.Available = true
	info.Version = strings.TrimSpace(version)
	info.Status = fmt.Sprintf("Available (v%s)", info.Version)

	// Detect rootful vs rootless
	if uid, err := d.runCommand(ctx, "podman", "info", "--format", "{{.Host.Security.Rootless}}"); err == nil {
		rootless := strings.TrimSpace(uid)
		if rootless == "true" {
			info.Details["mode"] = "rootless"
		} else {
			info.Details["mode"] = "rootful"
		}
	}

	// Check for podman-compose
	if d.commandExists(ctx, "podman-compose") {
		info.Details["compose"] = "podman-compose"
	}

	return info
}

// DetectKubernetes checks if Kubernetes access is available
func (d *TargetDetector) DetectKubernetes(ctx context.Context) *TargetInfo {
	info := &TargetInfo{
		Type:    TargetKubernetes,
		Details: make(map[string]string),
	}

	// Check if kubectl command exists
	if !d.commandExists(ctx, "kubectl") {
		info.Available = false
		info.Status = "kubectl not installed"
		info.Error = "kubectl command not found"
		return info
	}

	// Get kubectl version
	version, err := d.runCommand(ctx, "kubectl", "version", "--client", "--short")
	if err != nil {
		// Try new format
		version, err = d.runCommand(ctx, "kubectl", "version", "--client", "-o", "json")
		if err != nil {
			info.Available = false
			info.Status = "Error getting version"
			info.Error = err.Error()
			return info
		}
	}
	info.Version = extractKubectlVersion(version)

	// Get current context
	currentContext, err := d.runCommand(ctx, "kubectl", "config", "current-context")
	if err != nil {
		info.Available = false
		info.Status = "No cluster context"
		info.Error = "no kubernetes context configured"
		return info
	}
	info.Details["context"] = strings.TrimSpace(currentContext)

	// Try to connect to cluster
	_, err = d.runCommand(ctx, "kubectl", "cluster-info")
	if err != nil {
		info.Available = false
		info.Status = fmt.Sprintf("Cannot connect to cluster (%s)", info.Details["context"])
		info.Error = err.Error()
		return info
	}

	info.Available = true
	info.Status = fmt.Sprintf("Available (%s)", info.Details["context"])

	// Get server version
	if serverVersion, err := d.runCommand(ctx, "kubectl", "version", "--short"); err == nil {
		if lines := strings.Split(serverVersion, "\n"); len(lines) >= 2 {
			info.Details["server_version"] = strings.TrimPrefix(strings.TrimSpace(lines[1]), "Server Version: ")
		}
	}

	// Check for CloudNativePG operator
	if _, err := d.runCommand(ctx, "kubectl", "get", "crd", "clusters.postgresql.cnpg.io"); err == nil {
		info.Details["cloudnativepg"] = "installed"
	}

	return info
}

// commandExists checks if a command exists in PATH
func (d *TargetDetector) commandExists(ctx context.Context, name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// runCommand runs a command and returns its output
func (d *TargetDetector) runCommand(ctx context.Context, name string, args ...string) (string, error) {
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

// extractKubectlVersion extracts version from kubectl output
func extractKubectlVersion(output string) string {
	output = strings.TrimSpace(output)

	// Try to extract from "Client Version: vX.Y.Z" format
	if strings.HasPrefix(output, "Client Version:") {
		parts := strings.Split(output, "\n")
		if len(parts) > 0 {
			return strings.TrimPrefix(strings.TrimSpace(parts[0]), "Client Version: ")
		}
	}

	// Try JSON format
	if strings.Contains(output, "clientVersion") {
		// Simple extraction - look for major.minor
		if idx := strings.Index(output, `"gitVersion"`); idx != -1 {
			rest := output[idx+13:]
			if end := strings.Index(rest, `"`); end != -1 {
				return rest[:end]
			}
		}
	}

	return output
}

// FormatTargetInfo formats a TargetInfo for display
func FormatTargetInfo(info *TargetInfo) string {
	var icon string
	switch info.Type {
	case TargetLocal:
		icon = "🖥️"
	case TargetDocker:
		icon = "🐳"
	case TargetPodman:
		icon = "🦭"
	case TargetKubernetes:
		icon = "☸️"
	}

	var statusIcon string
	if info.Available {
		statusIcon = "✓"
	} else {
		statusIcon = "✗"
	}

	name := TargetDisplayName(info.Type)
	return fmt.Sprintf("%s %s %s - %s", icon, statusIcon, name, info.Status)
}

// TargetDisplayName returns a human-readable name for a target
func TargetDisplayName(t TargetType) string {
	switch t {
	case TargetLocal:
		return "Local Installation"
	case TargetDocker:
		return "Docker"
	case TargetPodman:
		return "Podman"
	case TargetKubernetes:
		return "Kubernetes"
	default:
		return string(t)
	}
}

// TargetDescription returns a description for a target
func TargetDescription(t TargetType) string {
	switch t {
	case TargetLocal:
		return "Install bibd directly on this machine as a system service"
	case TargetDocker:
		return "Run bibd in Docker containers with docker-compose"
	case TargetPodman:
		return "Run bibd in Podman containers (rootful or rootless)"
	case TargetKubernetes:
		return "Deploy bibd to a Kubernetes cluster"
	default:
		return ""
	}
}
