package checks

import (
	"bib/internal/capcheck"
	"context"
	"os/exec"
	"strings"
	"time"
)

type KubernetesDeepChecker struct{}

func (k KubernetesDeepChecker) ID() capcheck.CapabilityID { return "kubernetes_deep" }
func (k KubernetesDeepChecker) Description() string {
	return "Attempts to query kubectl version (client only) if kubectl present."
}

func (k KubernetesDeepChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      k.ID(),
		Name:    "Kubernetes deep",
		Details: map[string]any{},
	}
	path, err := exec.LookPath("kubectl")
	if err != nil {
		res.Error = "kubectl not found"
		return res
	}
	res.Details["kubectl_path"] = path
	subCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(subCtx, "kubectl", "version", "--client", "--short")
	out, err := cmd.CombinedOutput()
	if err == nil {
		res.Details["kubectl_client_version"] = strings.TrimSpace(string(out))
		res.Supported = true
	} else {
		res.Error = "kubectl version failed: " + err.Error()
	}
	return res
}
