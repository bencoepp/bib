package postsetup

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// PodmanStatus contains the status of Podman deployment
type PodmanStatus struct {
	// DeployStyle is "pod" or "compose"
	DeployStyle string

	// PodName is the pod name (for pod style)
	PodName string

	// Containers contains status of each container
	Containers []ContainerStatus

	// AllRunning indicates if all containers are running
	AllRunning bool

	// BibdReachable indicates if bibd is reachable
	BibdReachable bool

	// BibdAddress is the bibd address
	BibdAddress string

	// Error contains any error
	Error string
}

// PodmanVerifier verifies Podman deployment
type PodmanVerifier struct {
	// DeployStyle is "pod" or "compose"
	DeployStyle string

	// PodName is the pod name (for pod style)
	PodName string

	// ComposeFile is the path to podman-compose.yaml
	ComposeFile string

	// BibdPort is the bibd port
	BibdPort int

	// Timeout is the verification timeout
	Timeout time.Duration
}

// NewPodmanVerifier creates a new Podman verifier
func NewPodmanVerifier(deployStyle, podName, composeFile string) *PodmanVerifier {
	return &PodmanVerifier{
		DeployStyle: deployStyle,
		PodName:     podName,
		ComposeFile: composeFile,
		BibdPort:    4000,
		Timeout:     30 * time.Second,
	}
}

// Verify checks the Podman deployment status
func (v *PodmanVerifier) Verify(ctx context.Context) *PodmanStatus {
	status := &PodmanStatus{
		DeployStyle: v.DeployStyle,
		PodName:     v.PodName,
		Containers:  make([]ContainerStatus, 0),
	}

	var containers []ContainerStatus
	var err error

	if v.DeployStyle == "pod" {
		containers, err = v.getPodContainerStatus(ctx)
	} else {
		containers, err = v.getComposeContainerStatus(ctx)
	}

	if err != nil {
		status.Error = err.Error()
		return status
	}

	status.Containers = containers

	// Check if all running
	allRunning := true
	for _, c := range containers {
		if !c.Running {
			allRunning = false
		}
	}
	status.AllRunning = allRunning

	// Check bibd connectivity
	address := fmt.Sprintf("localhost:%d", v.BibdPort)
	status.BibdAddress = address

	verifier := NewLocalVerifier(address)
	localStatus := verifier.Verify(ctx)
	status.BibdReachable = localStatus.Running

	return status
}

// getPodContainerStatus gets container status for pod deployment
func (v *PodmanVerifier) getPodContainerStatus(ctx context.Context) ([]ContainerStatus, error) {
	// Get pod status
	cmd := exec.CommandContext(ctx, "podman", "pod", "ps", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("podman pod ps failed: %w", err)
	}

	var pods []struct {
		Name   string `json:"Name"`
		Status string `json:"Status"`
	}

	if err := json.Unmarshal(output, &pods); err != nil {
		return nil, fmt.Errorf("failed to parse pod status: %w", err)
	}

	// Get containers in pod
	cmd = exec.CommandContext(ctx, "podman", "ps", "-a", "--filter", fmt.Sprintf("pod=%s", v.PodName), "--format", "json")
	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("podman ps failed: %w", err)
	}

	var containerData []struct {
		Names  []string `json:"Names"`
		ID     string   `json:"Id"`
		Image  string   `json:"Image"`
		Status string   `json:"Status"`
		State  string   `json:"State"`
	}

	if err := json.Unmarshal(output, &containerData); err != nil {
		return nil, fmt.Errorf("failed to parse container status: %w", err)
	}

	var containers []ContainerStatus
	for _, c := range containerData {
		name := ""
		if len(c.Names) > 0 {
			name = c.Names[0]
		}

		container := ContainerStatus{
			Name:    name,
			ID:      c.ID,
			Image:   c.Image,
			Status:  c.Status,
			State:   c.State,
			Running: c.State == "running",
		}
		containers = append(containers, container)
	}

	return containers, nil
}

// getComposeContainerStatus gets container status for compose deployment
func (v *PodmanVerifier) getComposeContainerStatus(ctx context.Context) ([]ContainerStatus, error) {
	// Try podman-compose first
	cmd := exec.CommandContext(ctx, "podman-compose", "-f", v.ComposeFile, "ps")
	output, err := cmd.Output()
	if err != nil {
		// Try podman compose
		cmd = exec.CommandContext(ctx, "podman", "compose", "-f", v.ComposeFile, "ps")
		output, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("podman compose ps failed: %w", err)
		}
	}

	var containers []ContainerStatus

	lines := strings.Split(string(output), "\n")
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue // Skip header
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		container := ContainerStatus{
			Name:    fields[0],
			Status:  strings.Join(fields[1:], " "),
			Running: strings.Contains(line, "Up") || strings.Contains(line, "running"),
		}
		containers = append(containers, container)
	}

	return containers, nil
}

// WaitForRunning waits for all containers to be running
func (v *PodmanVerifier) WaitForRunning(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		status := v.Verify(ctx)
		if status.AllRunning {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
			continue
		}
	}

	return fmt.Errorf("timeout waiting for containers to be running")
}

// FormatPodmanStatus formats Podman status for display
func FormatPodmanStatus(status *PodmanStatus) string {
	var sb strings.Builder

	if status.Error != "" {
		sb.WriteString(fmt.Sprintf("❌ Error: %s\n", status.Error))
		return sb.String()
	}

	if len(status.Containers) == 0 {
		sb.WriteString("📦 No containers found\n")
		return sb.String()
	}

	// Overall status
	if status.AllRunning {
		sb.WriteString("🟢 All containers running\n")
	} else {
		sb.WriteString("🔴 Some containers not running\n")
	}

	if status.DeployStyle == "pod" {
		sb.WriteString(fmt.Sprintf("   Pod: %s\n", status.PodName))
	}
	sb.WriteString("\n")

	// Container details
	sb.WriteString("Containers:\n")
	for _, c := range status.Containers {
		var icon string
		if c.Running {
			icon = "✓"
		} else {
			icon = "✗"
		}
		sb.WriteString(fmt.Sprintf("  %s %s: %s\n", icon, c.Name, c.Status))
	}

	// bibd connectivity
	sb.WriteString("\n")
	if status.BibdReachable {
		sb.WriteString(fmt.Sprintf("🌐 bibd reachable at %s\n", status.BibdAddress))
	} else {
		sb.WriteString(fmt.Sprintf("⚠️  bibd not reachable at %s\n", status.BibdAddress))
	}

	return sb.String()
}

// GetPodmanManagementCommands returns Podman management commands
func GetPodmanManagementCommands(deployStyle, podName, composeFile string) []string {
	if deployStyle == "pod" {
		return []string{
			fmt.Sprintf("podman pod ps"),
			fmt.Sprintf("podman pod logs %s", podName),
			fmt.Sprintf("podman kube play pod.yaml"),
			fmt.Sprintf("podman kube down pod.yaml"),
			fmt.Sprintf("podman pod restart %s", podName),
		}
	}

	// Compose style
	return []string{
		fmt.Sprintf("podman-compose -f %s ps", composeFile),
		fmt.Sprintf("podman-compose -f %s logs -f", composeFile),
		fmt.Sprintf("podman-compose -f %s up -d", composeFile),
		fmt.Sprintf("podman-compose -f %s down", composeFile),
		fmt.Sprintf("podman-compose -f %s restart", composeFile),
	}
}
