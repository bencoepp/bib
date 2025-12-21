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

// KubernetesStatus contains the status of Kubernetes deployment
type KubernetesStatus struct {
	// Namespace is the deployment namespace
	Namespace string

	// Pods contains status of each pod
	Pods []PodStatus

	// AllReady indicates if all pods are ready
	AllReady bool

	// ExternalIP is the external IP (for LoadBalancer)
	ExternalIP string

	// NodePort is the NodePort (for NodePort service)
	NodePort int

	// IngressHost is the ingress hostname
	IngressHost string

	// BibdReachable indicates if bibd is reachable
	BibdReachable bool

	// BibdAddress is the bibd address
	BibdAddress string

	// Error contains any error
	Error string
}

// PodStatus contains status of a single pod
type PodStatus struct {
	// Name is the pod name
	Name string

	// Phase is the pod phase
	Phase string

	// Ready indicates if pod is ready
	Ready bool

	// Restarts is the number of restarts
	Restarts int

	// Age is the pod age
	Age string
}

// KubernetesVerifier verifies Kubernetes deployment
type KubernetesVerifier struct {
	// Namespace is the deployment namespace
	Namespace string

	// BibdPort is the bibd port
	BibdPort int

	// Timeout is the verification timeout
	Timeout time.Duration
}

// NewKubernetesVerifier creates a new Kubernetes verifier
func NewKubernetesVerifier(namespace string) *KubernetesVerifier {
	if namespace == "" {
		namespace = "bibd"
	}
	return &KubernetesVerifier{
		Namespace: namespace,
		BibdPort:  4000,
		Timeout:   60 * time.Second,
	}
}

// Verify checks the Kubernetes deployment status
func (v *KubernetesVerifier) Verify(ctx context.Context) *KubernetesStatus {
	status := &KubernetesStatus{
		Namespace: v.Namespace,
		Pods:      make([]PodStatus, 0),
	}

	// Get pod status
	pods, err := v.getPodStatus(ctx)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.Pods = pods

	// Check if all ready
	allReady := len(pods) > 0
	for _, p := range pods {
		if !p.Ready {
			allReady = false
		}
	}
	status.AllReady = allReady

	// Get external access info
	v.getExternalAccess(ctx, status)

	// Check bibd connectivity if we have an address
	if status.ExternalIP != "" {
		address := fmt.Sprintf("%s:%d", status.ExternalIP, v.BibdPort)
		status.BibdAddress = address

		verifier := NewLocalVerifier(address)
		localStatus := verifier.Verify(ctx)
		status.BibdReachable = localStatus.Running
	}

	return status
}

// getPodStatus gets status of all pods
func (v *KubernetesVerifier) getPodStatus(ctx context.Context) ([]PodStatus, error) {
	cmd := exec.CommandContext(ctx, "kubectl", "get", "pods",
		"-n", v.Namespace,
		"-l", "app.kubernetes.io/name=bibd",
		"-o", "json")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("kubectl get pods failed: %w", err)
	}

	var podList struct {
		Items []struct {
			Metadata struct {
				Name              string    `json:"name"`
				CreationTimestamp time.Time `json:"creationTimestamp"`
			} `json:"metadata"`
			Status struct {
				Phase      string `json:"phase"`
				Conditions []struct {
					Type   string `json:"type"`
					Status string `json:"status"`
				} `json:"conditions"`
				ContainerStatuses []struct {
					RestartCount int `json:"restartCount"`
				} `json:"containerStatuses"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := json.Unmarshal(output, &podList); err != nil {
		return nil, fmt.Errorf("failed to parse pod status: %w", err)
	}

	var pods []PodStatus
	for _, p := range podList.Items {
		ready := false
		for _, cond := range p.Status.Conditions {
			if cond.Type == "Ready" && cond.Status == "True" {
				ready = true
				break
			}
		}

		restarts := 0
		for _, cs := range p.Status.ContainerStatuses {
			restarts += cs.RestartCount
		}

		age := time.Since(p.Metadata.CreationTimestamp).Round(time.Minute).String()

		pod := PodStatus{
			Name:     p.Metadata.Name,
			Phase:    p.Status.Phase,
			Ready:    ready,
			Restarts: restarts,
			Age:      age,
		}
		pods = append(pods, pod)
	}

	return pods, nil
}

// getExternalAccess gets external access information
func (v *KubernetesVerifier) getExternalAccess(ctx context.Context, status *KubernetesStatus) {
	// Get service info
	cmd := exec.CommandContext(ctx, "kubectl", "get", "svc", "bibd",
		"-n", v.Namespace, "-o", "json")

	output, err := cmd.Output()
	if err != nil {
		return
	}

	var svc struct {
		Spec struct {
			Type  string `json:"type"`
			Ports []struct {
				NodePort int `json:"nodePort"`
			} `json:"ports"`
		} `json:"spec"`
		Status struct {
			LoadBalancer struct {
				Ingress []struct {
					IP       string `json:"ip"`
					Hostname string `json:"hostname"`
				} `json:"ingress"`
			} `json:"loadBalancer"`
		} `json:"status"`
	}

	if err := json.Unmarshal(output, &svc); err != nil {
		return
	}

	// Get LoadBalancer IP/hostname
	if len(svc.Status.LoadBalancer.Ingress) > 0 {
		ingress := svc.Status.LoadBalancer.Ingress[0]
		if ingress.IP != "" {
			status.ExternalIP = ingress.IP
		} else if ingress.Hostname != "" {
			status.ExternalIP = ingress.Hostname
		}
	}

	// Get NodePort
	if svc.Spec.Type == "NodePort" && len(svc.Spec.Ports) > 0 {
		status.NodePort = svc.Spec.Ports[0].NodePort
	}

	// Get Ingress info
	cmd = exec.CommandContext(ctx, "kubectl", "get", "ingress", "bibd",
		"-n", v.Namespace, "-o", "jsonpath={.spec.rules[0].host}")

	output, err = cmd.Output()
	if err == nil && len(output) > 0 {
		status.IngressHost = string(output)
	}
}

// WaitForReady waits for all pods to be ready
func (v *KubernetesVerifier) WaitForReady(ctx context.Context, timeout time.Duration) error {
	cmd := exec.CommandContext(ctx, "kubectl", "rollout", "status",
		"-n", v.Namespace, "deployment/bibd",
		"--timeout", fmt.Sprintf("%ds", int(timeout.Seconds())))

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rollout status failed: %s", stderr.String())
	}

	return nil
}

// PortForward starts a port-forward to bibd
func (v *KubernetesVerifier) PortForward(ctx context.Context, localPort int) (*exec.Cmd, error) {
	if localPort == 0 {
		localPort = 14000 // Default local port
	}

	cmd := exec.CommandContext(ctx, "kubectl", "port-forward",
		"-n", v.Namespace, "svc/bibd",
		fmt.Sprintf("%d:%d", localPort, v.BibdPort))

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start port-forward: %w", err)
	}

	return cmd, nil
}

// FormatKubernetesStatus formats Kubernetes status for display
func FormatKubernetesStatus(status *KubernetesStatus) string {
	var sb strings.Builder

	if status.Error != "" {
		sb.WriteString(fmt.Sprintf("❌ Error: %s\n", status.Error))
		return sb.String()
	}

	if len(status.Pods) == 0 {
		sb.WriteString("📦 No pods found\n")
		return sb.String()
	}

	// Overall status
	if status.AllReady {
		sb.WriteString("🟢 All pods ready\n")
	} else {
		sb.WriteString("🟡 Some pods not ready\n")
	}
	sb.WriteString(fmt.Sprintf("   Namespace: %s\n", status.Namespace))
	sb.WriteString("\n")

	// Pod details
	sb.WriteString("Pods:\n")
	for _, p := range status.Pods {
		var icon string
		if p.Ready {
			icon = "✓"
		} else if p.Phase == "Running" {
			icon = "⏳"
		} else {
			icon = "✗"
		}
		sb.WriteString(fmt.Sprintf("  %s %s: %s (restarts: %d, age: %s)\n",
			icon, p.Name, p.Phase, p.Restarts, p.Age))
	}

	// External access
	sb.WriteString("\n")
	if status.ExternalIP != "" {
		sb.WriteString(fmt.Sprintf("🌐 External IP: %s\n", status.ExternalIP))
	}
	if status.NodePort > 0 {
		sb.WriteString(fmt.Sprintf("🔌 NodePort: %d\n", status.NodePort))
	}
	if status.IngressHost != "" {
		sb.WriteString(fmt.Sprintf("🔗 Ingress: %s\n", status.IngressHost))
	}

	// bibd connectivity
	if status.BibdAddress != "" {
		if status.BibdReachable {
			sb.WriteString(fmt.Sprintf("✓ bibd reachable at %s\n", status.BibdAddress))
		} else {
			sb.WriteString(fmt.Sprintf("⚠️  bibd not reachable at %s\n", status.BibdAddress))
		}
	}

	return sb.String()
}

// GetKubernetesManagementCommands returns Kubernetes management commands
func GetKubernetesManagementCommands(namespace string) []string {
	return []string{
		fmt.Sprintf("kubectl -n %s get pods", namespace),
		fmt.Sprintf("kubectl -n %s get svc", namespace),
		fmt.Sprintf("kubectl -n %s logs -f deployment/bibd", namespace),
		fmt.Sprintf("kubectl -n %s describe deployment bibd", namespace),
		fmt.Sprintf("kubectl -n %s port-forward svc/bibd 4000:4000", namespace),
		fmt.Sprintf("kubectl -n %s rollout restart deployment bibd", namespace),
	}
}
