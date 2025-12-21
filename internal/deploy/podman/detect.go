// Package podman provides Podman deployment utilities for bibd.
package podman

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strings"
	"time"
)

// PodmanInfo contains information about the Podman installation
type PodmanInfo struct {
	// Available indicates if Podman is available
	Available bool

	// Version is the Podman version
	Version string

	// APIVersion is the Podman API version
	APIVersion string

	// Rootless indicates if running in rootless mode
	Rootless bool

	// RootlessUID is the UID in rootless mode
	RootlessUID int

	// SocketPath is the path to the Podman socket
	SocketPath string

	// MachineRunning indicates if Podman machine is running (macOS/Windows)
	MachineRunning bool

	// MachineName is the name of the running Podman machine
	MachineName string

	// ComposeAvailable indicates if podman-compose is available
	ComposeAvailable bool

	// ComposeVersion is the podman-compose version
	ComposeVersion string

	// ComposeCommand is the command to use for compose
	ComposeCommand string

	// PodmanComposeAvailable indicates if 'podman compose' plugin is available
	PodmanComposeAvailable bool

	// KubePlayAvailable indicates if 'podman kube play' is available
	KubePlayAvailable bool

	// Error contains any error message
	Error string
}

// Detector detects Podman installation and capabilities
type Detector struct {
	// Timeout for detection commands
	Timeout time.Duration
}

// NewDetector creates a new Podman detector
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

// Detect detects Podman installation and capabilities
func (d *Detector) Detect(ctx context.Context) *PodmanInfo {
	info := &PodmanInfo{}

	// Check if podman command exists
	if !d.commandExists("podman") {
		info.Available = false
		info.Error = "podman command not found"
		return info
	}
	info.Available = true

	// Check Podman version
	version, err := d.runCommand(ctx, "podman", "version", "--format", "{{.Client.Version}}")
	if err != nil {
		info.Error = fmt.Sprintf("Failed to get Podman version: %v", err)
		return info
	}
	info.Version = strings.TrimSpace(version)

	// Get API version
	if apiVersion, err := d.runCommand(ctx, "podman", "version", "--format", "{{.Client.APIVersion}}"); err == nil {
		info.APIVersion = strings.TrimSpace(apiVersion)
	}

	// Check if rootless
	info.Rootless = d.isRootless()
	if info.Rootless {
		if currentUser, err := user.Current(); err == nil {
			fmt.Sscanf(currentUser.Uid, "%d", &info.RootlessUID)
		}
	}

	// Check for Podman machine (macOS/Windows)
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		d.detectMachine(ctx, info)
	}

	// Get socket path
	info.SocketPath = d.getSocketPath()

	// Check for podman-compose
	d.detectCompose(ctx, info)

	// Check for podman compose plugin
	d.detectPodmanCompose(ctx, info)

	// Check for podman kube play
	d.detectKubePlay(ctx, info)

	return info
}

// isRootless checks if running in rootless mode
func (d *Detector) isRootless() bool {
	// Check if running as root
	if os.Geteuid() == 0 {
		return false
	}

	// On Linux, check for rootless indicators
	if runtime.GOOS == "linux" {
		// Check XDG_RUNTIME_DIR
		if xdgRuntime := os.Getenv("XDG_RUNTIME_DIR"); xdgRuntime != "" {
			return true
		}
	}

	// Default to rootless for non-root users
	return true
}

// getSocketPath returns the Podman socket path
func (d *Detector) getSocketPath() string {
	// Check for explicit socket path
	if socket := os.Getenv("CONTAINER_HOST"); socket != "" {
		return socket
	}

	// Check for rootless socket
	if d.isRootless() {
		if xdgRuntime := os.Getenv("XDG_RUNTIME_DIR"); xdgRuntime != "" {
			return fmt.Sprintf("unix://%s/podman/podman.sock", xdgRuntime)
		}
		if home, err := os.UserHomeDir(); err == nil {
			return fmt.Sprintf("unix://%s/.local/share/containers/podman/machine/podman.sock", home)
		}
	}

	// Default root socket
	return "unix:///run/podman/podman.sock"
}

// detectMachine detects Podman machine status
func (d *Detector) detectMachine(ctx context.Context, info *PodmanInfo) {
	output, err := d.runCommand(ctx, "podman", "machine", "list", "--format", "{{.Name}}\t{{.Running}}")
	if err != nil {
		return
	}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) >= 2 {
			name := parts[0]
			running := strings.ToLower(parts[1]) == "true"
			if running {
				info.MachineRunning = true
				info.MachineName = name
				break
			}
		}
	}
}

// detectCompose detects podman-compose availability
func (d *Detector) detectCompose(ctx context.Context, info *PodmanInfo) {
	if !d.commandExists("podman-compose") {
		return
	}

	info.ComposeAvailable = true
	info.ComposeCommand = "podman-compose"

	if version, err := d.runCommand(ctx, "podman-compose", "version"); err == nil {
		// Parse version from output
		lines := strings.Split(version, "\n")
		for _, line := range lines {
			if strings.Contains(line, "version") || strings.Contains(line, "Version") {
				info.ComposeVersion = strings.TrimSpace(line)
				break
			}
		}
		if info.ComposeVersion == "" {
			info.ComposeVersion = strings.TrimSpace(lines[0])
		}
	}
}

// detectPodmanCompose detects 'podman compose' plugin
func (d *Detector) detectPodmanCompose(ctx context.Context, info *PodmanInfo) {
	if _, err := d.runCommand(ctx, "podman", "compose", "version"); err == nil {
		info.PodmanComposeAvailable = true
		// Prefer podman compose over podman-compose
		info.ComposeAvailable = true
		info.ComposeCommand = "podman compose"

		if version, err := d.runCommand(ctx, "podman", "compose", "version", "--short"); err == nil {
			info.ComposeVersion = strings.TrimSpace(version)
		}
	}
}

// detectKubePlay detects 'podman kube play' availability
func (d *Detector) detectKubePlay(ctx context.Context, info *PodmanInfo) {
	// Check if podman kube play is available
	if _, err := d.runCommand(ctx, "podman", "kube", "play", "--help"); err == nil {
		info.KubePlayAvailable = true
	}
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

// FormatPodmanInfo formats Podman info for display
func FormatPodmanInfo(info *PodmanInfo) string {
	var sb strings.Builder

	if !info.Available {
		sb.WriteString("🦭 Podman: ✗ Not installed\n")
		if info.Error != "" {
			sb.WriteString(fmt.Sprintf("   Error: %s\n", info.Error))
		}
		return sb.String()
	}

	sb.WriteString("🦭 Podman: ✓ Available\n")
	sb.WriteString(fmt.Sprintf("   Version: %s\n", info.Version))
	if info.APIVersion != "" {
		sb.WriteString(fmt.Sprintf("   API Version: %s\n", info.APIVersion))
	}

	if info.Rootless {
		sb.WriteString(fmt.Sprintf("   Mode: Rootless (UID %d)\n", info.RootlessUID))
	} else {
		sb.WriteString("   Mode: Rootful\n")
	}

	if info.MachineRunning {
		sb.WriteString(fmt.Sprintf("   Machine: %s (running)\n", info.MachineName))
	}

	if info.ComposeAvailable {
		sb.WriteString(fmt.Sprintf("   Compose: ✓ %s (%s)\n", info.ComposeCommand, info.ComposeVersion))
	} else {
		sb.WriteString("   Compose: ✗ Not available\n")
	}

	if info.KubePlayAvailable {
		sb.WriteString("   Kube Play: ✓ Available\n")
	}

	return sb.String()
}

// IsUsable returns true if Podman can be used for deployment
func (info *PodmanInfo) IsUsable() bool {
	return info.Available && (info.ComposeAvailable || info.KubePlayAvailable)
}

// GetComposeCommand returns the compose command parts
func (info *PodmanInfo) GetComposeCommand() []string {
	if info.ComposeCommand == "podman compose" {
		return []string{"podman", "compose"}
	}
	return []string{"podman-compose"}
}

// PreferredDeployMethod returns the preferred deployment method
func (info *PodmanInfo) PreferredDeployMethod() string {
	if info.KubePlayAvailable {
		return "kube"
	}
	if info.ComposeAvailable {
		return "compose"
	}
	return "none"
}
