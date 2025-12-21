// Package kubernetes provides Kubernetes deployment utilities for bibd.
package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// KubeInfo contains information about the Kubernetes installation
type KubeInfo struct {
	// Available indicates if kubectl is available
	Available bool

	// ClusterReachable indicates if the cluster is reachable
	ClusterReachable bool

	// CurrentContext is the current kubectl context
	CurrentContext string

	// ClusterName is the name of the current cluster
	ClusterName string

	// ServerURL is the Kubernetes API server URL
	ServerURL string

	// ServerVersion is the Kubernetes server version
	ServerVersion string

	// ClientVersion is the kubectl client version
	ClientVersion string

	// Namespace is the current default namespace
	Namespace string

	// CloudNativePGAvailable indicates if CloudNativePG operator is installed
	CloudNativePGAvailable bool

	// CloudNativePGVersion is the CloudNativePG operator version
	CloudNativePGVersion string

	// IngressControllerAvailable indicates if an ingress controller is available
	IngressControllerAvailable bool

	// IngressClassName is the default ingress class name
	IngressClassName string

	// StorageClasses lists available storage classes
	StorageClasses []string

	// DefaultStorageClass is the default storage class
	DefaultStorageClass string

	// Error contains any error message
	Error string
}

// Detector detects Kubernetes installation and capabilities
type Detector struct {
	// Timeout for detection commands
	Timeout time.Duration

	// Kubeconfig is an optional path to kubeconfig file
	Kubeconfig string

	// Context is an optional context to use
	Context string
}

// NewDetector creates a new Kubernetes detector
func NewDetector() *Detector {
	return &Detector{
		Timeout: 15 * time.Second,
	}
}

// WithTimeout sets the detection timeout
func (d *Detector) WithTimeout(timeout time.Duration) *Detector {
	d.Timeout = timeout
	return d
}

// WithKubeconfig sets the kubeconfig path
func (d *Detector) WithKubeconfig(path string) *Detector {
	d.Kubeconfig = path
	return d
}

// WithContext sets the context to use
func (d *Detector) WithContext(ctx string) *Detector {
	d.Context = ctx
	return d
}

// Detect detects Kubernetes installation and capabilities
func (d *Detector) Detect(ctx context.Context) *KubeInfo {
	info := &KubeInfo{}

	// Check if kubectl command exists
	if !d.commandExists("kubectl") {
		info.Available = false
		info.Error = "kubectl command not found"
		return info
	}
	info.Available = true

	// Get client version
	if version, err := d.runKubectl(ctx, "version", "--client", "-o", "json"); err == nil {
		var versionInfo struct {
			ClientVersion struct {
				GitVersion string `json:"gitVersion"`
			} `json:"clientVersion"`
		}
		if json.Unmarshal([]byte(version), &versionInfo) == nil {
			info.ClientVersion = versionInfo.ClientVersion.GitVersion
		}
	}

	// Get current context
	if currentCtx, err := d.runKubectl(ctx, "config", "current-context"); err == nil {
		info.CurrentContext = strings.TrimSpace(currentCtx)
	}

	// Test cluster connectivity
	if _, err := d.runKubectl(ctx, "cluster-info"); err != nil {
		info.ClusterReachable = false
		info.Error = fmt.Sprintf("cluster not reachable: %v", err)
		return info
	}
	info.ClusterReachable = true

	// Get server version
	if version, err := d.runKubectl(ctx, "version", "-o", "json"); err == nil {
		var versionInfo struct {
			ServerVersion struct {
				GitVersion string `json:"gitVersion"`
			} `json:"serverVersion"`
		}
		if json.Unmarshal([]byte(version), &versionInfo) == nil {
			info.ServerVersion = versionInfo.ServerVersion.GitVersion
		}
	}

	// Get cluster info from config
	d.getClusterInfo(ctx, info)

	// Get current namespace
	if ns, err := d.runKubectl(ctx, "config", "view", "--minify", "-o", "jsonpath={..namespace}"); err == nil {
		info.Namespace = strings.TrimSpace(ns)
		if info.Namespace == "" {
			info.Namespace = "default"
		}
	}

	// Check for CloudNativePG
	d.detectCloudNativePG(ctx, info)

	// Check for ingress controller
	d.detectIngressController(ctx, info)

	// Get storage classes
	d.detectStorageClasses(ctx, info)

	return info
}

// getClusterInfo gets cluster name and server URL from config
func (d *Detector) getClusterInfo(ctx context.Context, info *KubeInfo) {
	// Get server URL
	if serverURL, err := d.runKubectl(ctx, "config", "view", "--minify", "-o", "jsonpath={.clusters[0].cluster.server}"); err == nil {
		info.ServerURL = strings.TrimSpace(serverURL)
	}

	// Get cluster name
	if clusterName, err := d.runKubectl(ctx, "config", "view", "--minify", "-o", "jsonpath={.clusters[0].name}"); err == nil {
		info.ClusterName = strings.TrimSpace(clusterName)
	}
}

// detectCloudNativePG checks for CloudNativePG operator
func (d *Detector) detectCloudNativePG(ctx context.Context, info *KubeInfo) {
	// Check if the CRD exists
	if _, err := d.runKubectl(ctx, "get", "crd", "clusters.postgresql.cnpg.io", "--ignore-not-found"); err == nil {
		// Check if there's actually a CRD (non-empty output would mean it exists)
		output, _ := d.runKubectl(ctx, "get", "crd", "clusters.postgresql.cnpg.io", "-o", "name")
		if strings.TrimSpace(output) != "" {
			info.CloudNativePGAvailable = true

			// Try to get version from operator deployment
			if version, err := d.runKubectl(ctx, "get", "deployment", "-n", "cnpg-system", "cnpg-controller-manager",
				"-o", "jsonpath={.spec.template.spec.containers[0].image}"); err == nil {
				parts := strings.Split(strings.TrimSpace(version), ":")
				if len(parts) > 1 {
					info.CloudNativePGVersion = parts[len(parts)-1]
				}
			}
		}
	}
}

// detectIngressController checks for ingress controller
func (d *Detector) detectIngressController(ctx context.Context, info *KubeInfo) {
	// Check for ingress classes
	if output, err := d.runKubectl(ctx, "get", "ingressclass", "-o", "json"); err == nil {
		var ingressClasses struct {
			Items []struct {
				Metadata struct {
					Name        string            `json:"name"`
					Annotations map[string]string `json:"annotations"`
				} `json:"metadata"`
			} `json:"items"`
		}
		if json.Unmarshal([]byte(output), &ingressClasses) == nil && len(ingressClasses.Items) > 0 {
			info.IngressControllerAvailable = true
			// Find default ingress class
			for _, ic := range ingressClasses.Items {
				if ic.Metadata.Annotations["ingressclass.kubernetes.io/is-default-class"] == "true" {
					info.IngressClassName = ic.Metadata.Name
					break
				}
			}
			// If no default, use first one
			if info.IngressClassName == "" && len(ingressClasses.Items) > 0 {
				info.IngressClassName = ingressClasses.Items[0].Metadata.Name
			}
		}
	}
}

// detectStorageClasses gets available storage classes
func (d *Detector) detectStorageClasses(ctx context.Context, info *KubeInfo) {
	if output, err := d.runKubectl(ctx, "get", "storageclass", "-o", "json"); err == nil {
		var storageClasses struct {
			Items []struct {
				Metadata struct {
					Name        string            `json:"name"`
					Annotations map[string]string `json:"annotations"`
				} `json:"metadata"`
			} `json:"items"`
		}
		if json.Unmarshal([]byte(output), &storageClasses) == nil {
			for _, sc := range storageClasses.Items {
				info.StorageClasses = append(info.StorageClasses, sc.Metadata.Name)
				if sc.Metadata.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
					info.DefaultStorageClass = sc.Metadata.Name
				}
			}
		}
	}
}

// commandExists checks if a command exists in PATH
func (d *Detector) commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// runKubectl runs a kubectl command and returns its output
func (d *Detector) runKubectl(ctx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, d.Timeout)
	defer cancel()

	// Build command with optional kubeconfig and context
	cmdArgs := make([]string, 0, len(args)+4)
	if d.Kubeconfig != "" {
		cmdArgs = append(cmdArgs, "--kubeconfig", d.Kubeconfig)
	}
	if d.Context != "" {
		cmdArgs = append(cmdArgs, "--context", d.Context)
	}
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.CommandContext(ctx, "kubectl", cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// FormatKubeInfo formats Kubernetes info for display
func FormatKubeInfo(info *KubeInfo) string {
	var sb strings.Builder

	if !info.Available {
		sb.WriteString("☸️  Kubernetes: ✗ kubectl not installed\n")
		if info.Error != "" {
			sb.WriteString(fmt.Sprintf("   Error: %s\n", info.Error))
		}
		return sb.String()
	}

	if !info.ClusterReachable {
		sb.WriteString("☸️  Kubernetes: ⚠ Cluster not reachable\n")
		sb.WriteString(fmt.Sprintf("   Context: %s\n", info.CurrentContext))
		sb.WriteString(fmt.Sprintf("   Client: %s\n", info.ClientVersion))
		if info.Error != "" {
			sb.WriteString(fmt.Sprintf("   Error: %s\n", info.Error))
		}
		return sb.String()
	}

	sb.WriteString("☸️  Kubernetes: ✓ Available\n")
	sb.WriteString(fmt.Sprintf("   Context: %s\n", info.CurrentContext))
	sb.WriteString(fmt.Sprintf("   Cluster: %s\n", info.ClusterName))
	sb.WriteString(fmt.Sprintf("   Server: %s (%s)\n", info.ServerURL, info.ServerVersion))
	sb.WriteString(fmt.Sprintf("   Namespace: %s\n", info.Namespace))

	if info.CloudNativePGAvailable {
		sb.WriteString(fmt.Sprintf("   CloudNativePG: ✓ %s\n", info.CloudNativePGVersion))
	}

	if info.IngressControllerAvailable {
		sb.WriteString(fmt.Sprintf("   Ingress: ✓ %s\n", info.IngressClassName))
	} else {
		sb.WriteString("   Ingress: ✗ No controller found\n")
	}

	if len(info.StorageClasses) > 0 {
		sb.WriteString(fmt.Sprintf("   Storage: %s", strings.Join(info.StorageClasses, ", ")))
		if info.DefaultStorageClass != "" {
			sb.WriteString(fmt.Sprintf(" (default: %s)", info.DefaultStorageClass))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// IsUsable returns true if Kubernetes can be used for deployment
func (info *KubeInfo) IsUsable() bool {
	return info.Available && info.ClusterReachable
}

// GetContexts returns all available contexts
func (d *Detector) GetContexts(ctx context.Context) ([]string, error) {
	output, err := d.runKubectl(ctx, "config", "get-contexts", "-o", "name")
	if err != nil {
		return nil, err
	}

	var contexts []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			contexts = append(contexts, line)
		}
	}
	return contexts, nil
}

// GetNamespaces returns all namespaces
func (d *Detector) GetNamespaces(ctx context.Context) ([]string, error) {
	output, err := d.runKubectl(ctx, "get", "namespaces", "-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return nil, err
	}

	return strings.Fields(output), nil
}
