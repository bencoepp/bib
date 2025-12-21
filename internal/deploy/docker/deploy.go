package docker

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DeployConfig contains configuration for Docker deployment
type DeployConfig struct {
	// ComposeConfig is the compose configuration
	ComposeConfig *ComposeConfig

	// OutputDir is the output directory for generated files
	OutputDir string

	// AutoStart automatically starts containers after generation
	AutoStart bool

	// PullImages pulls the latest images before starting
	PullImages bool

	// WaitForHealthy waits for containers to be healthy before returning
	WaitForHealthy bool

	// HealthTimeout is the timeout for waiting for containers to be healthy
	HealthTimeout time.Duration

	// Verbose enables verbose output
	Verbose bool
}

// DefaultDeployConfig returns a default deployment configuration
func DefaultDeployConfig() *DeployConfig {
	return &DeployConfig{
		ComposeConfig:  DefaultComposeConfig(),
		OutputDir:      "./bibd-docker",
		AutoStart:      true,
		PullImages:     true,
		WaitForHealthy: true,
		HealthTimeout:  120 * time.Second,
		Verbose:        false,
	}
}

// Deployer handles Docker deployment operations
type Deployer struct {
	Config     *DeployConfig
	DockerInfo *DockerInfo
	Generator  *ComposeGenerator
}

// NewDeployer creates a new Docker deployer
func NewDeployer(config *DeployConfig) *Deployer {
	if config == nil {
		config = DefaultDeployConfig()
	}
	if config.ComposeConfig == nil {
		config.ComposeConfig = DefaultComposeConfig()
	}
	config.ComposeConfig.OutputDir = config.OutputDir

	return &Deployer{
		Config:    config,
		Generator: NewComposeGenerator(config.ComposeConfig),
	}
}

// DeployResult contains the result of a deployment
type DeployResult struct {
	// Success indicates if deployment was successful
	Success bool

	// OutputDir is the directory where files were generated
	OutputDir string

	// FilesGenerated is the list of generated files
	FilesGenerated []string

	// ContainersStarted indicates if containers were started
	ContainersStarted bool

	// ContainersHealthy indicates if containers are healthy
	ContainersHealthy bool

	// Error contains any error message
	Error string

	// Logs contains deployment logs
	Logs []string
}

// Deploy performs the full Docker deployment
func (d *Deployer) Deploy(ctx context.Context) (*DeployResult, error) {
	result := &DeployResult{
		OutputDir: d.Config.OutputDir,
		Logs:      make([]string, 0),
	}

	// Step 1: Detect Docker
	d.log(result, "🔍 Detecting Docker...")
	detector := NewDetector()
	d.DockerInfo = detector.Detect(ctx)

	if !d.DockerInfo.IsUsable() {
		result.Error = "Docker is not available or not usable"
		if d.DockerInfo.Error != "" {
			result.Error = d.DockerInfo.Error
		}
		return result, fmt.Errorf("%s", result.Error)
	}
	d.log(result, fmt.Sprintf("   ✓ Docker %s with %s", d.DockerInfo.Version, d.DockerInfo.ComposeCommand))

	// Step 2: Generate files
	d.log(result, "📝 Generating deployment files...")
	files, err := d.Generator.Generate()
	if err != nil {
		result.Error = fmt.Sprintf("Failed to generate files: %v", err)
		return result, err
	}

	// Step 3: Create output directory
	if err := os.MkdirAll(d.Config.OutputDir, 0755); err != nil {
		result.Error = fmt.Sprintf("Failed to create output directory: %v", err)
		return result, err
	}

	// Step 4: Write files
	for filename := range files.Files {
		result.FilesGenerated = append(result.FilesGenerated, filename)
	}

	if err := files.WriteToDir(d.Config.OutputDir); err != nil {
		result.Error = fmt.Sprintf("Failed to write files: %v", err)
		return result, err
	}
	d.log(result, fmt.Sprintf("   ✓ Generated %d files in %s", len(result.FilesGenerated), d.Config.OutputDir))

	// Step 5: Generate identity key if not exists
	identityKeyPath := filepath.Join(d.Config.OutputDir, "config", "identity.pem")
	if _, err := os.Stat(identityKeyPath); os.IsNotExist(err) {
		d.log(result, "🔑 Generating identity key...")
		if err := d.generateIdentityKey(identityKeyPath); err != nil {
			d.log(result, fmt.Sprintf("   ⚠️ Failed to generate identity key: %v", err))
		} else {
			d.log(result, "   ✓ Identity key generated")
		}
	}

	// If not auto-starting, we're done
	if !d.Config.AutoStart {
		result.Success = true
		d.log(result, "\n✅ Files generated successfully!")
		d.log(result, d.Generator.FormatStartInstructions(d.DockerInfo))
		return result, nil
	}

	// Step 6: Pull images (if enabled)
	if d.Config.PullImages {
		d.log(result, "📥 Pulling images...")
		if err := d.pullImages(ctx); err != nil {
			d.log(result, fmt.Sprintf("   ⚠️ Failed to pull images: %v (continuing anyway)", err))
		} else {
			d.log(result, "   ✓ Images pulled")
		}
	}

	// Step 7: Start containers
	d.log(result, "🚀 Starting containers...")
	if err := d.startContainers(ctx); err != nil {
		result.Error = fmt.Sprintf("Failed to start containers: %v", err)
		return result, err
	}
	result.ContainersStarted = true
	d.log(result, "   ✓ Containers started")

	// Step 8: Wait for healthy (if enabled)
	if d.Config.WaitForHealthy {
		d.log(result, "⏳ Waiting for containers to be healthy...")
		if err := d.waitForHealthy(ctx); err != nil {
			d.log(result, fmt.Sprintf("   ⚠️ Health check timeout: %v", err))
		} else {
			result.ContainersHealthy = true
			d.log(result, "   ✓ All containers healthy")
		}
	}

	// Step 9: Show status
	d.log(result, "📊 Container status:")
	status, _ := d.getContainerStatus(ctx)
	for _, line := range strings.Split(status, "\n") {
		if strings.TrimSpace(line) != "" {
			d.log(result, "   "+line)
		}
	}

	result.Success = true
	d.log(result, "\n✅ Deployment complete!")

	return result, nil
}

// log adds a log message to the result
func (d *Deployer) log(result *DeployResult, msg string) {
	result.Logs = append(result.Logs, msg)
	if d.Config.Verbose {
		fmt.Println(msg)
	}
}

// generateIdentityKey generates an identity key file
func (d *Deployer) generateIdentityKey(path string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	// Generate a placeholder key (in production, this would use crypto/ed25519)
	// For now, we'll create a placeholder that bibd will regenerate on first start
	placeholder := `# Placeholder identity key
# bibd will generate a real key on first start
# You can also use 'bibd keygen' to generate one
`
	return os.WriteFile(path, []byte(placeholder), 0600)
}

// pullImages pulls the required Docker images
func (d *Deployer) pullImages(ctx context.Context) error {
	composeCmd := d.DockerInfo.GetComposeCommand()
	args := append(composeCmd[1:], "pull")

	cmd := exec.CommandContext(ctx, composeCmd[0], args...)
	cmd.Dir = d.Config.OutputDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v: %s", err, stderr.String())
	}
	return nil
}

// startContainers starts the Docker containers
func (d *Deployer) startContainers(ctx context.Context) error {
	composeCmd := d.DockerInfo.GetComposeCommand()
	args := append(composeCmd[1:], "up", "-d")

	cmd := exec.CommandContext(ctx, composeCmd[0], args...)
	cmd.Dir = d.Config.OutputDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v: %s", err, stderr.String())
	}
	return nil
}

// waitForHealthy waits for all containers to be healthy
func (d *Deployer) waitForHealthy(ctx context.Context) error {
	deadline := time.Now().Add(d.Config.HealthTimeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		healthy, err := d.checkHealth(ctx)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		if healthy {
			return nil
		}
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("timeout waiting for containers to be healthy")
}

// checkHealth checks if all containers are healthy
func (d *Deployer) checkHealth(ctx context.Context) (bool, error) {
	composeCmd := d.DockerInfo.GetComposeCommand()
	args := append(composeCmd[1:], "ps", "--format", "json")

	cmd := exec.CommandContext(ctx, composeCmd[0], args...)
	cmd.Dir = d.Config.OutputDir

	output, err := cmd.Output()
	if err != nil {
		return false, err
	}

	// Simple check: no "unhealthy" or "starting" in output
	outputStr := string(output)
	if strings.Contains(outputStr, "unhealthy") {
		return false, nil
	}
	if strings.Contains(outputStr, "starting") {
		return false, nil
	}

	return true, nil
}

// getContainerStatus returns the current container status
func (d *Deployer) getContainerStatus(ctx context.Context) (string, error) {
	composeCmd := d.DockerInfo.GetComposeCommand()
	args := append(composeCmd[1:], "ps")

	cmd := exec.CommandContext(ctx, composeCmd[0], args...)
	cmd.Dir = d.Config.OutputDir

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}

// Stop stops the Docker containers
func (d *Deployer) Stop(ctx context.Context) error {
	if d.DockerInfo == nil {
		detector := NewDetector()
		d.DockerInfo = detector.Detect(ctx)
	}

	composeCmd := d.DockerInfo.GetComposeCommand()
	args := append(composeCmd[1:], "down")

	cmd := exec.CommandContext(ctx, composeCmd[0], args...)
	cmd.Dir = d.Config.OutputDir

	return cmd.Run()
}

// Logs returns the container logs
func (d *Deployer) Logs(ctx context.Context, follow bool, tail int) (string, error) {
	if d.DockerInfo == nil {
		detector := NewDetector()
		d.DockerInfo = detector.Detect(ctx)
	}

	composeCmd := d.DockerInfo.GetComposeCommand()
	args := append(composeCmd[1:], "logs")

	if follow {
		args = append(args, "-f")
	}
	if tail > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", tail))
	}
	args = append(args, "bibd")

	cmd := exec.CommandContext(ctx, composeCmd[0], args...)
	cmd.Dir = d.Config.OutputDir

	output, err := cmd.Output()
	return string(output), err
}

// Status returns the current deployment status
func (d *Deployer) Status(ctx context.Context) (*DeploymentStatus, error) {
	if d.DockerInfo == nil {
		detector := NewDetector()
		d.DockerInfo = detector.Detect(ctx)
	}

	status := &DeploymentStatus{
		Containers: make([]ContainerStatus, 0),
	}

	// Check if output directory exists
	if _, err := os.Stat(d.Config.OutputDir); os.IsNotExist(err) {
		status.Deployed = false
		return status, nil
	}
	status.Deployed = true

	// Get container status
	composeCmd := d.DockerInfo.GetComposeCommand()
	args := append(composeCmd[1:], "ps", "--format", "{{.Name}}\t{{.Status}}\t{{.Health}}")

	cmd := exec.CommandContext(ctx, composeCmd[0], args...)
	cmd.Dir = d.Config.OutputDir

	output, err := cmd.Output()
	if err != nil {
		return status, nil
	}

	for _, line := range strings.Split(string(output), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) >= 2 {
			cs := ContainerStatus{
				Name:   parts[0],
				Status: parts[1],
			}
			if len(parts) >= 3 {
				cs.Health = parts[2]
			}
			status.Containers = append(status.Containers, cs)

			if strings.Contains(parts[1], "Up") {
				status.Running = true
			}
		}
	}

	return status, nil
}

// DeploymentStatus contains the current deployment status
type DeploymentStatus struct {
	// Deployed indicates if files have been deployed
	Deployed bool

	// Running indicates if containers are running
	Running bool

	// Containers contains the status of each container
	Containers []ContainerStatus
}

// ContainerStatus contains the status of a single container
type ContainerStatus struct {
	Name   string
	Status string
	Health string
}

// FormatStatus formats the deployment status for display
func (s *DeploymentStatus) FormatStatus() string {
	var sb strings.Builder

	if !s.Deployed {
		sb.WriteString("📦 Not deployed\n")
		return sb.String()
	}

	if s.Running {
		sb.WriteString("🟢 Running\n")
	} else {
		sb.WriteString("🔴 Stopped\n")
	}

	sb.WriteString("\nContainers:\n")
	for _, c := range s.Containers {
		var healthIcon string
		switch {
		case strings.Contains(c.Health, "unhealthy"):
			healthIcon = "✗"
		case strings.Contains(c.Health, "healthy"):
			healthIcon = "✓"
		case strings.Contains(c.Health, "starting"):
			healthIcon = "⏳"
		default:
			healthIcon = "-"
		}
		sb.WriteString(fmt.Sprintf("  %s %s: %s\n", healthIcon, c.Name, c.Status))
	}

	return sb.String()
}
