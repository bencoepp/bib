package postsetup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// DockerStatus contains the status of Docker deployment
type DockerStatus struct {
	// Containers contains status of each container
	Containers []ContainerStatus

	// AllRunning indicates if all containers are running
	AllRunning bool

	// AllHealthy indicates if all containers are healthy
	AllHealthy bool

	// BibdReachable indicates if bibd is reachable
	BibdReachable bool

	// BibdAddress is the bibd address
	BibdAddress string

	// Error contains any error
	Error string
}

// ContainerStatus contains status of a single container
type ContainerStatus struct {
	// Name is the container name
	Name string

	// ID is the container ID
	ID string

	// Image is the container image
	Image string

	// Status is the container status
	Status string

	// State is the container state (running, exited, etc.)
	State string

	// Health is the health status
	Health string

	// Ports are the exposed ports
	Ports string

	// Running indicates if container is running
	Running bool

	// Healthy indicates if container is healthy
	Healthy bool
}

// DockerVerifier verifies Docker deployment
type DockerVerifier struct {
	// ProjectName is the docker compose project name
	ProjectName string

	// ComposeFile is the path to docker-compose.yaml
	ComposeFile string

	// BibdPort is the bibd port
	BibdPort int

	// Timeout is the verification timeout
	Timeout time.Duration
}

// NewDockerVerifier creates a new Docker verifier
func NewDockerVerifier(projectName, composeFile string) *DockerVerifier {
	return &DockerVerifier{
		ProjectName: projectName,
		ComposeFile: composeFile,
		BibdPort:    4000,
		Timeout:     30 * time.Second,
	}
}

// Verify checks the Docker deployment status
func (v *DockerVerifier) Verify(ctx context.Context) *DockerStatus {
	status := &DockerStatus{
		Containers: make([]ContainerStatus, 0),
	}

	// Get container status using docker compose ps
	containers, err := v.getContainerStatus(ctx)
	if err != nil {
		status.Error = err.Error()
		return status
	}

	status.Containers = containers

	// Check if all running and healthy
	allRunning := true
	allHealthy := true
	for _, c := range containers {
		if !c.Running {
			allRunning = false
		}
		if !c.Healthy && c.Health != "" && c.Health != "none" {
			allHealthy = false
		}
	}
	status.AllRunning = allRunning
	status.AllHealthy = allHealthy

	// Check bibd connectivity
	address := fmt.Sprintf("localhost:%d", v.BibdPort)
	status.BibdAddress = address

	verifier := NewLocalVerifier(address)
	localStatus := verifier.Verify(ctx)
	status.BibdReachable = localStatus.Running

	return status
}

// getContainerStatus gets status of all containers in the project
func (v *DockerVerifier) getContainerStatus(ctx context.Context) ([]ContainerStatus, error) {
	args := []string{"compose"}
	if v.ComposeFile != "" {
		args = append(args, "-f", v.ComposeFile)
	}
	if v.ProjectName != "" {
		args = append(args, "-p", v.ProjectName)
	}
	args = append(args, "ps", "--format", "json")

	cmd := exec.CommandContext(ctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Try without --format json (older docker compose)
		return v.getContainerStatusLegacy(ctx)
	}

	var containers []ContainerStatus

	// Parse JSON output (each line is a JSON object)
	for _, line := range strings.Split(stdout.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var data struct {
			Name    string `json:"Name"`
			ID      string `json:"ID"`
			Image   string `json:"Image"`
			Status  string `json:"Status"`
			State   string `json:"State"`
			Health  string `json:"Health"`
			Ports   string `json:"Ports"`
			Service string `json:"Service"`
		}

		if err := json.Unmarshal([]byte(line), &data); err != nil {
			continue
		}

		container := ContainerStatus{
			Name:    data.Name,
			ID:      data.ID,
			Image:   data.Image,
			Status:  data.Status,
			State:   data.State,
			Health:  data.Health,
			Ports:   data.Ports,
			Running: data.State == "running",
			Healthy: data.Health == "healthy" || data.Health == "",
		}
		containers = append(containers, container)
	}

	return containers, nil
}

// getContainerStatusLegacy gets container status for older docker versions
func (v *DockerVerifier) getContainerStatusLegacy(ctx context.Context) ([]ContainerStatus, error) {
	args := []string{"compose"}
	if v.ComposeFile != "" {
		args = append(args, "-f", v.ComposeFile)
	}
	if v.ProjectName != "" {
		args = append(args, "-p", v.ProjectName)
	}
	args = append(args, "ps")

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("docker compose ps failed: %w", err)
	}

	var containers []ContainerStatus

	lines := strings.Split(string(output), "\n")
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue // Skip header
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		container := ContainerStatus{
			Name:    fields[0],
			Status:  strings.Join(fields[3:], " "),
			Running: strings.Contains(line, "Up"),
			Healthy: strings.Contains(line, "healthy") || !strings.Contains(line, "unhealthy"),
		}
		containers = append(containers, container)
	}

	return containers, nil
}

// WaitForHealthy waits for all containers to be healthy
func (v *DockerVerifier) WaitForHealthy(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		status := v.Verify(ctx)
		if status.AllRunning && status.AllHealthy {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
			continue
		}
	}

	return fmt.Errorf("timeout waiting for containers to be healthy")
}

// FormatDockerStatus formats Docker status for display
func FormatDockerStatus(status *DockerStatus) string {
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
	if status.AllRunning && status.AllHealthy {
		sb.WriteString("🟢 All containers running and healthy\n")
	} else if status.AllRunning {
		sb.WriteString("🟡 All containers running (some not healthy)\n")
	} else {
		sb.WriteString("🔴 Some containers not running\n")
	}
	sb.WriteString("\n")

	// Container details
	sb.WriteString("Containers:\n")
	for _, c := range status.Containers {
		var icon string
		if c.Running && c.Healthy {
			icon = "✓"
		} else if c.Running {
			icon = "⏳"
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

// GetDockerManagementCommands returns Docker management commands
func GetDockerManagementCommands(composeFile string) []string {
	prefix := "docker compose"
	if composeFile != "" {
		prefix = fmt.Sprintf("docker compose -f %s", composeFile)
	}

	return []string{
		fmt.Sprintf("%s ps", prefix),
		fmt.Sprintf("%s logs -f", prefix),
		fmt.Sprintf("%s up -d", prefix),
		fmt.Sprintf("%s down", prefix),
		fmt.Sprintf("%s restart", prefix),
	}
}
