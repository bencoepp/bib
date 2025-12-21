package kubernetes

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

// DeployConfig contains configuration for Kubernetes deployment
type DeployConfig struct {
	// ManifestConfig is the manifest configuration
	ManifestConfig *ManifestConfig

	// OutputDir is the output directory for generated files
	OutputDir string

	// AutoApply automatically applies manifests after generation
	AutoApply bool

	// WaitForReady waits for pods to be ready before returning
	WaitForReady bool

	// WaitTimeout is the timeout for waiting for pods
	WaitTimeout time.Duration

	// Verbose enables verbose output
	Verbose bool
}

// DefaultDeployConfig returns a default deployment configuration
func DefaultDeployConfig() *DeployConfig {
	return &DeployConfig{
		ManifestConfig: DefaultManifestConfig(),
		OutputDir:      "./bibd-kubernetes",
		AutoApply:      true,
		WaitForReady:   true,
		WaitTimeout:    300 * time.Second,
		Verbose:        false,
	}
}

// Deployer handles Kubernetes deployment operations
type Deployer struct {
	Config    *DeployConfig
	KubeInfo  *KubeInfo
	Generator *ManifestGenerator
}

// NewDeployer creates a new Kubernetes deployer
func NewDeployer(config *DeployConfig) *Deployer {
	if config == nil {
		config = DefaultDeployConfig()
	}
	if config.ManifestConfig == nil {
		config.ManifestConfig = DefaultManifestConfig()
	}
	config.ManifestConfig.OutputDir = config.OutputDir

	return &Deployer{
		Config:    config,
		Generator: NewManifestGenerator(config.ManifestConfig),
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

	// ManifestsApplied indicates if manifests were applied
	ManifestsApplied bool

	// PodsReady indicates if pods are ready
	PodsReady bool

	// ExternalIP is the external IP (for LoadBalancer services)
	ExternalIP string

	// IngressURL is the ingress URL if configured
	IngressURL string

	// Error contains any error message
	Error string

	// Logs contains deployment logs
	Logs []string
}

// Deploy performs the full Kubernetes deployment
func (d *Deployer) Deploy(ctx context.Context) (*DeployResult, error) {
	result := &DeployResult{
		OutputDir: d.Config.OutputDir,
		Logs:      make([]string, 0),
	}

	// Step 1: Detect Kubernetes
	d.log(result, "🔍 Detecting Kubernetes...")
	detector := NewDetector()
	d.KubeInfo = detector.Detect(ctx)

	if !d.KubeInfo.Available {
		result.Error = "kubectl is not available"
		if d.KubeInfo.Error != "" {
			result.Error = d.KubeInfo.Error
		}
		return result, fmt.Errorf("%s", result.Error)
	}

	if !d.KubeInfo.ClusterReachable {
		result.Error = "Kubernetes cluster is not reachable"
		if d.KubeInfo.Error != "" {
			result.Error = d.KubeInfo.Error
		}
		return result, fmt.Errorf("%s", result.Error)
	}

	d.log(result, fmt.Sprintf("   ✓ Context: %s", d.KubeInfo.CurrentContext))
	d.log(result, fmt.Sprintf("   ✓ Cluster: %s (%s)", d.KubeInfo.ClusterName, d.KubeInfo.ServerVersion))

	// Check for CloudNativePG if using that mode
	if d.Config.ManifestConfig.StorageBackend == "postgres" &&
		d.Config.ManifestConfig.PostgresMode == "cloudnativepg" &&
		!d.KubeInfo.CloudNativePGAvailable {
		d.log(result, "   ⚠️ CloudNativePG operator not detected")
		d.log(result, "      Install it with: kubectl apply -f https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-1.22/releases/cnpg-1.22.0.yaml")
	}

	// Step 2: Generate manifests
	d.log(result, "📝 Generating Kubernetes manifests...")
	files, err := d.Generator.Generate()
	if err != nil {
		result.Error = fmt.Sprintf("Failed to generate manifests: %v", err)
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

	// Make scripts executable
	for _, script := range []string{"apply.sh", "delete.sh"} {
		scriptPath := filepath.Join(d.Config.OutputDir, script)
		os.Chmod(scriptPath, 0755)
	}

	d.log(result, fmt.Sprintf("   ✓ Generated %d files in %s", len(result.FilesGenerated), d.Config.OutputDir))

	// If not auto-applying, we're done
	if !d.Config.AutoApply {
		result.Success = true
		d.log(result, "\n✅ Manifests generated successfully!")
		d.log(result, d.Generator.FormatDeployInstructions(d.KubeInfo))
		return result, nil
	}

	// Step 5: Apply manifests
	d.log(result, "☸️  Applying manifests...")
	if err := d.applyManifests(ctx); err != nil {
		result.Error = fmt.Sprintf("Failed to apply manifests: %v", err)
		return result, err
	}
	result.ManifestsApplied = true
	d.log(result, "   ✓ Manifests applied")

	// Step 6: Wait for pods to be ready
	if d.Config.WaitForReady {
		d.log(result, "⏳ Waiting for pods to be ready...")
		if err := d.waitForReady(ctx); err != nil {
			d.log(result, fmt.Sprintf("   ⚠️ Timeout waiting for pods: %v", err))
		} else {
			result.PodsReady = true
			d.log(result, "   ✓ All pods ready")
		}
	}

	// Step 7: Get external access info
	if d.Config.ManifestConfig.ServiceType == "LoadBalancer" {
		d.log(result, "🌐 Getting external IP...")
		if ip, err := d.getExternalIP(ctx); err == nil && ip != "" {
			result.ExternalIP = ip
			d.log(result, fmt.Sprintf("   ✓ External IP: %s", ip))
		} else {
			d.log(result, "   ⚠️ External IP not yet assigned (check later with kubectl get svc)")
		}
	}

	if d.Config.ManifestConfig.IngressHost != "" {
		result.IngressURL = fmt.Sprintf("http://%s", d.Config.ManifestConfig.IngressHost)
		if d.Config.ManifestConfig.IngressTLS {
			result.IngressURL = fmt.Sprintf("https://%s", d.Config.ManifestConfig.IngressHost)
		}
		d.log(result, fmt.Sprintf("   ✓ Ingress URL: %s", result.IngressURL))
	}

	// Step 8: Show status
	d.log(result, "\n📊 Deployment status:")
	status, _ := d.getPodsStatus(ctx)
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

// applyManifests applies the Kubernetes manifests
func (d *Deployer) applyManifests(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-k", d.Config.OutputDir)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v: %s", err, stderr.String())
	}

	return nil
}

// waitForReady waits for all pods to be ready
func (d *Deployer) waitForReady(ctx context.Context) error {
	namespace := d.Config.ManifestConfig.Namespace

	// Wait for bibd deployment
	cmd := exec.CommandContext(ctx, "kubectl", "rollout", "status",
		"-n", namespace, "deployment/bibd",
		"--timeout", fmt.Sprintf("%ds", int(d.Config.WaitTimeout.Seconds())))

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bibd deployment not ready: %v: %s", err, stderr.String())
	}

	// Wait for postgres if using statefulset
	if d.Config.ManifestConfig.StorageBackend == "postgres" &&
		d.Config.ManifestConfig.PostgresMode == "statefulset" {

		cmd = exec.CommandContext(ctx, "kubectl", "rollout", "status",
			"-n", namespace, "statefulset/postgres",
			"--timeout", fmt.Sprintf("%ds", int(d.Config.WaitTimeout.Seconds())))

		cmd.Stderr = &stderr
		stderr.Reset()

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("postgres statefulset not ready: %v: %s", err, stderr.String())
		}
	}

	return nil
}

// getExternalIP gets the external IP for LoadBalancer service
func (d *Deployer) getExternalIP(ctx context.Context) (string, error) {
	namespace := d.Config.ManifestConfig.Namespace

	// Try multiple times as LoadBalancer IP might take a while
	for i := 0; i < 10; i++ {
		cmd := exec.CommandContext(ctx, "kubectl", "get", "svc", "bibd",
			"-n", namespace,
			"-o", "jsonpath={.status.loadBalancer.ingress[0].ip}")

		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			return string(output), nil
		}

		// Try hostname (some cloud providers use hostname instead of IP)
		cmd = exec.CommandContext(ctx, "kubectl", "get", "svc", "bibd",
			"-n", namespace,
			"-o", "jsonpath={.status.loadBalancer.ingress[0].hostname}")

		output, err = cmd.Output()
		if err == nil && len(output) > 0 {
			return string(output), nil
		}

		time.Sleep(3 * time.Second)
	}

	return "", fmt.Errorf("external IP not assigned")
}

// getPodsStatus gets the current pods status
func (d *Deployer) getPodsStatus(ctx context.Context) (string, error) {
	namespace := d.Config.ManifestConfig.Namespace

	cmd := exec.CommandContext(ctx, "kubectl", "get", "pods",
		"-n", namespace,
		"-l", "app.kubernetes.io/name=bibd",
		"-o", "wide")

	output, err := cmd.Output()
	return string(output), err
}

// Delete deletes the Kubernetes deployment
func (d *Deployer) Delete(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "kubectl", "delete", "-k", d.Config.OutputDir)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v: %s", err, stderr.String())
	}

	return nil
}

// Logs returns the pod logs
func (d *Deployer) Logs(ctx context.Context, follow bool, tail int) (string, error) {
	namespace := d.Config.ManifestConfig.Namespace

	args := []string{"logs", "-n", namespace, "deployment/bibd"}
	if follow {
		args = append(args, "-f")
	}
	if tail > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", tail))
	}

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	output, err := cmd.Output()
	return string(output), err
}

// Status returns the current deployment status
func (d *Deployer) Status(ctx context.Context) (*DeploymentStatus, error) {
	status := &DeploymentStatus{
		Namespace: d.Config.ManifestConfig.Namespace,
		Pods:      make([]PodStatus, 0),
	}

	// Check if output directory exists
	if _, err := os.Stat(d.Config.OutputDir); os.IsNotExist(err) {
		status.Generated = false
		return status, nil
	}
	status.Generated = true

	// Check if namespace exists
	cmd := exec.CommandContext(ctx, "kubectl", "get", "namespace",
		d.Config.ManifestConfig.Namespace, "-o", "name")
	if output, err := cmd.Output(); err == nil && len(output) > 0 {
		status.Deployed = true
	}

	if !status.Deployed {
		return status, nil
	}

	// Get pod status
	cmd = exec.CommandContext(ctx, "kubectl", "get", "pods",
		"-n", d.Config.ManifestConfig.Namespace,
		"-o", "custom-columns=NAME:.metadata.name,STATUS:.status.phase,READY:.status.conditions[?(@.type=='Ready')].status",
		"--no-headers")

	output, err := cmd.Output()
	if err != nil {
		return status, nil
	}

	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 {
			ps := PodStatus{
				Name:  fields[0],
				Phase: fields[1],
				Ready: fields[2] == "True",
			}
			status.Pods = append(status.Pods, ps)

			if ps.Phase == "Running" && ps.Ready {
				status.Running = true
			}
		}
	}

	return status, nil
}

// DeploymentStatus contains the current deployment status
type DeploymentStatus struct {
	// Namespace is the deployment namespace
	Namespace string

	// Generated indicates if manifests have been generated
	Generated bool

	// Deployed indicates if manifests have been applied
	Deployed bool

	// Running indicates if pods are running
	Running bool

	// Pods contains the status of each pod
	Pods []PodStatus
}

// PodStatus contains the status of a single pod
type PodStatus struct {
	Name  string
	Phase string
	Ready bool
}

// FormatStatus formats the deployment status for display
func (s *DeploymentStatus) FormatStatus() string {
	var sb strings.Builder

	if !s.Generated {
		sb.WriteString("📦 Not generated\n")
		return sb.String()
	}

	if !s.Deployed {
		sb.WriteString("📋 Generated (not deployed)\n")
		return sb.String()
	}

	if s.Running {
		sb.WriteString("🟢 Running\n")
	} else {
		sb.WriteString("🟡 Deployed (pods not ready)\n")
	}

	sb.WriteString(fmt.Sprintf("Namespace: %s\n", s.Namespace))
	sb.WriteString("\nPods:\n")

	for _, p := range s.Pods {
		var icon string
		if p.Ready {
			icon = "✓"
		} else if p.Phase == "Running" {
			icon = "⏳"
		} else {
			icon = "✗"
		}
		sb.WriteString(fmt.Sprintf("  %s %s: %s (ready: %v)\n", icon, p.Name, p.Phase, p.Ready))
	}

	return sb.String()
}
