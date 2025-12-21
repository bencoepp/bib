package podman

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

// DeployConfig contains configuration for Podman deployment
type DeployConfig struct {
	// PodConfig is the pod configuration
	PodConfig *PodConfig

	// OutputDir is the output directory for generated files
	OutputDir string

	// AutoStart automatically starts containers after generation
	AutoStart bool

	// PullImages pulls the latest images before starting
	PullImages bool

	// WaitForRunning waits for containers to be running before returning
	WaitForRunning bool

	// WaitTimeout is the timeout for waiting for containers
	WaitTimeout time.Duration

	// Verbose enables verbose output
	Verbose bool
}

// DefaultDeployConfig returns a default deployment configuration
func DefaultDeployConfig() *DeployConfig {
	return &DeployConfig{
		PodConfig:      DefaultPodConfig(),
		OutputDir:      "./bibd-podman",
		AutoStart:      true,
		PullImages:     true,
		WaitForRunning: true,
		WaitTimeout:    120 * time.Second,
		Verbose:        false,
	}
}

// Deployer handles Podman deployment operations
type Deployer struct {
	Config     *DeployConfig
	PodmanInfo *PodmanInfo
	Generator  *PodGenerator
}

// NewDeployer creates a new Podman deployer
func NewDeployer(config *DeployConfig) *Deployer {
	if config == nil {
		config = DefaultDeployConfig()
	}
	if config.PodConfig == nil {
		config.PodConfig = DefaultPodConfig()
	}
	config.PodConfig.OutputDir = config.OutputDir

	return &Deployer{
		Config:    config,
		Generator: NewPodGenerator(config.PodConfig),
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

	// ContainersRunning indicates if containers are running
	ContainersRunning bool

	// DeployStyle is the deployment style used (pod or compose)
	DeployStyle string

	// Error contains any error message
	Error string

	// Logs contains deployment logs
	Logs []string
}

// Deploy performs the full Podman deployment
func (d *Deployer) Deploy(ctx context.Context) (*DeployResult, error) {
	result := &DeployResult{
		OutputDir:   d.Config.OutputDir,
		DeployStyle: d.Config.PodConfig.DeployStyle,
		Logs:        make([]string, 0),
	}

	// Step 1: Detect Podman
	d.log(result, "🔍 Detecting Podman...")
	detector := NewDetector()
	d.PodmanInfo = detector.Detect(ctx)

	if !d.PodmanInfo.Available {
		result.Error = "Podman is not available"
		if d.PodmanInfo.Error != "" {
			result.Error = d.PodmanInfo.Error
		}
		return result, fmt.Errorf("%s", result.Error)
	}

	// Check for required capabilities
	if !d.PodmanInfo.IsUsable() {
		result.Error = "Podman is available but neither compose nor kube play is available"
		return result, fmt.Errorf("%s", result.Error)
	}

	d.log(result, fmt.Sprintf("   ✓ Podman %s", d.PodmanInfo.Version))
	if d.PodmanInfo.Rootless {
		d.log(result, fmt.Sprintf("   ✓ Rootless mode (UID %d)", d.PodmanInfo.RootlessUID))
		d.Config.PodConfig.Rootless = true
	} else {
		d.log(result, "   ✓ Rootful mode")
		d.Config.PodConfig.Rootless = false
	}

	// Select deploy style based on capabilities
	if d.Config.PodConfig.DeployStyle == "" {
		d.Config.PodConfig.DeployStyle = d.PodmanInfo.PreferredDeployMethod()
	}
	result.DeployStyle = d.Config.PodConfig.DeployStyle
	d.log(result, fmt.Sprintf("   ✓ Deploy style: %s", d.Config.PodConfig.DeployStyle))

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
		d.log(result, "🔑 Generating identity key placeholder...")
		if err := d.generateIdentityKey(identityKeyPath); err != nil {
			d.log(result, fmt.Sprintf("   ⚠️ Failed to generate identity key: %v", err))
		} else {
			d.log(result, "   ✓ Identity key placeholder generated")
		}
	}

	// If not auto-starting, we're done
	if !d.Config.AutoStart {
		result.Success = true
		d.log(result, "\n✅ Files generated successfully!")
		d.log(result, d.Generator.FormatStartInstructions(d.PodmanInfo))
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

	// Step 8: Wait for running (if enabled)
	if d.Config.WaitForRunning {
		d.log(result, "⏳ Waiting for containers to be running...")
		if err := d.waitForRunning(ctx); err != nil {
			d.log(result, fmt.Sprintf("   ⚠️ Wait timeout: %v", err))
		} else {
			result.ContainersRunning = true
			d.log(result, "   ✓ All containers running")
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

// generateIdentityKey generates an identity key file placeholder
func (d *Deployer) generateIdentityKey(path string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	placeholder := `# Placeholder identity key
# bibd will generate a real key on first start
# You can also use 'bibd keygen' to generate one
`
	return os.WriteFile(path, []byte(placeholder), 0600)
}

// pullImages pulls the required container images
func (d *Deployer) pullImages(ctx context.Context) error {
	images := []string{
		fmt.Sprintf("%s:%s", d.Config.PodConfig.BibdImage, d.Config.PodConfig.BibdTag),
	}

	if d.Config.PodConfig.StorageBackend == "postgres" {
		images = append(images, fmt.Sprintf("%s:%s",
			d.Config.PodConfig.PostgresImage, d.Config.PodConfig.PostgresTag))
	}

	for _, image := range images {
		cmd := exec.CommandContext(ctx, "podman", "pull", image)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to pull %s: %v: %s", image, err, stderr.String())
		}
	}

	return nil
}

// startContainers starts the Podman containers
func (d *Deployer) startContainers(ctx context.Context) error {
	if d.Config.PodConfig.DeployStyle == "pod" {
		return d.startPod(ctx)
	}
	return d.startCompose(ctx)
}

// startPod starts using podman kube play
func (d *Deployer) startPod(ctx context.Context) error {
	podYamlPath := filepath.Join(d.Config.OutputDir, "pod.yaml")

	cmd := exec.CommandContext(ctx, "podman", "kube", "play", podYamlPath)
	cmd.Dir = d.Config.OutputDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v: %s", err, stderr.String())
	}

	return nil
}

// startCompose starts using podman-compose or podman compose
func (d *Deployer) startCompose(ctx context.Context) error {
	composeCmd := d.PodmanInfo.GetComposeCommand()
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

// waitForRunning waits for all containers to be running
func (d *Deployer) waitForRunning(ctx context.Context) error {
	deadline := time.Now().Add(d.Config.WaitTimeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		running, err := d.checkRunning(ctx)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		if running {
			return nil
		}
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("timeout waiting for containers to be running")
}

// checkRunning checks if containers are running
func (d *Deployer) checkRunning(ctx context.Context) (bool, error) {
	var cmd *exec.Cmd

	if d.Config.PodConfig.DeployStyle == "pod" {
		cmd = exec.CommandContext(ctx, "podman", "pod", "ps",
			"--filter", fmt.Sprintf("name=%s", d.Config.PodConfig.PodName),
			"--format", "{{.Status}}")
	} else {
		composeCmd := d.PodmanInfo.GetComposeCommand()
		args := append(composeCmd[1:], "ps", "--format", "{{.State}}")
		cmd = exec.CommandContext(ctx, composeCmd[0], args...)
		cmd.Dir = d.Config.OutputDir
	}

	output, err := cmd.Output()
	if err != nil {
		return false, err
	}

	outputStr := strings.ToLower(string(output))
	return strings.Contains(outputStr, "running") || strings.Contains(outputStr, "up"), nil
}

// getContainerStatus returns the current container status
func (d *Deployer) getContainerStatus(ctx context.Context) (string, error) {
	var cmd *exec.Cmd

	if d.Config.PodConfig.DeployStyle == "pod" {
		cmd = exec.CommandContext(ctx, "podman", "ps",
			"--filter", fmt.Sprintf("pod=%s", d.Config.PodConfig.PodName))
	} else {
		composeCmd := d.PodmanInfo.GetComposeCommand()
		args := append(composeCmd[1:], "ps")
		cmd = exec.CommandContext(ctx, composeCmd[0], args...)
		cmd.Dir = d.Config.OutputDir
	}

	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(output), nil
}

// Stop stops the Podman containers
func (d *Deployer) Stop(ctx context.Context) error {
	if d.PodmanInfo == nil {
		detector := NewDetector()
		d.PodmanInfo = detector.Detect(ctx)
	}

	if d.Config.PodConfig.DeployStyle == "pod" {
		podYamlPath := filepath.Join(d.Config.OutputDir, "pod.yaml")
		cmd := exec.CommandContext(ctx, "podman", "kube", "down", podYamlPath)
		cmd.Dir = d.Config.OutputDir
		return cmd.Run()
	}

	composeCmd := d.PodmanInfo.GetComposeCommand()
	args := append(composeCmd[1:], "down")

	cmd := exec.CommandContext(ctx, composeCmd[0], args...)
	cmd.Dir = d.Config.OutputDir

	return cmd.Run()
}

// Logs returns the container logs
func (d *Deployer) Logs(ctx context.Context, follow bool, tail int) (string, error) {
	if d.PodmanInfo == nil {
		detector := NewDetector()
		d.PodmanInfo = detector.Detect(ctx)
	}

	var cmd *exec.Cmd

	if d.Config.PodConfig.DeployStyle == "pod" {
		args := []string{"logs"}
		if follow {
			args = append(args, "-f")
		}
		if tail > 0 {
			args = append(args, "--tail", fmt.Sprintf("%d", tail))
		}
		args = append(args, fmt.Sprintf("%s-bibd", d.Config.PodConfig.PodName))
		cmd = exec.CommandContext(ctx, "podman", args...)
	} else {
		composeCmd := d.PodmanInfo.GetComposeCommand()
		args := append(composeCmd[1:], "logs")
		if follow {
			args = append(args, "-f")
		}
		if tail > 0 {
			args = append(args, "--tail", fmt.Sprintf("%d", tail))
		}
		args = append(args, "bibd")
		cmd = exec.CommandContext(ctx, composeCmd[0], args...)
		cmd.Dir = d.Config.OutputDir
	}

	output, err := cmd.Output()
	return string(output), err
}

// Status returns the current deployment status
func (d *Deployer) Status(ctx context.Context) (*DeploymentStatus, error) {
	if d.PodmanInfo == nil {
		detector := NewDetector()
		d.PodmanInfo = detector.Detect(ctx)
	}

	status := &DeploymentStatus{
		DeployStyle: d.Config.PodConfig.DeployStyle,
		Containers:  make([]ContainerStatus, 0),
	}

	// Check if output directory exists
	if _, err := os.Stat(d.Config.OutputDir); os.IsNotExist(err) {
		status.Deployed = false
		return status, nil
	}
	status.Deployed = true

	// Get container status
	var cmd *exec.Cmd
	if d.Config.PodConfig.DeployStyle == "pod" {
		cmd = exec.CommandContext(ctx, "podman", "ps",
			"--filter", fmt.Sprintf("pod=%s", d.Config.PodConfig.PodName),
			"--format", "{{.Names}}\t{{.Status}}")
	} else {
		composeCmd := d.PodmanInfo.GetComposeCommand()
		args := append(composeCmd[1:], "ps", "--format", "{{.Name}}\t{{.Status}}")
		cmd = exec.CommandContext(ctx, composeCmd[0], args...)
		cmd.Dir = d.Config.OutputDir
	}

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
			status.Containers = append(status.Containers, cs)

			if strings.Contains(strings.ToLower(parts[1]), "up") ||
				strings.Contains(strings.ToLower(parts[1]), "running") {
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

	// DeployStyle is the deployment style (pod or compose)
	DeployStyle string

	// Containers contains the status of each container
	Containers []ContainerStatus
}

// ContainerStatus contains the status of a single container
type ContainerStatus struct {
	Name   string
	Status string
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

	sb.WriteString(fmt.Sprintf("Deploy Style: %s\n", s.DeployStyle))
	sb.WriteString("\nContainers:\n")
	for _, c := range s.Containers {
		var icon string
		statusLower := strings.ToLower(c.Status)
		if strings.Contains(statusLower, "up") || strings.Contains(statusLower, "running") {
			icon = "✓"
		} else if strings.Contains(statusLower, "exited") {
			icon = "✗"
		} else {
			icon = "-"
		}
		sb.WriteString(fmt.Sprintf("  %s %s: %s\n", icon, c.Name, c.Status))
	}

	return sb.String()
}
