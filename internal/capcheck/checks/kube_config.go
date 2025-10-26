package checks

import (
	"bib/internal/capcheck"
	"context"
	"os"
	"path/filepath"
)

type KubernetesConfigChecker struct{}

func (k KubernetesConfigChecker) ID() capcheck.CapabilityID { return "kubernetes_config" }
func (k KubernetesConfigChecker) Description() string {
	return "Detects availability of Kubernetes configuration (kubeconfig or in-cluster SA token)"
}

func (k KubernetesConfigChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      k.ID(),
		Name:    "Kubernetes config",
		Details: map[string]any{},
	}

	// Find kubeconfig paths
	var kubeconfigPaths []string
	if env := os.Getenv("KUBECONFIG"); env != "" {
		for _, p := range filepath.SplitList(env) {
			kubeconfigPaths = append(kubeconfigPaths, p)
		}
	} else if home, err := os.UserHomeDir(); err == nil {
		kubeconfigPaths = append(kubeconfigPaths, filepath.Join(home, ".kube", "config"))
	}

	kubeconfigExists := false
	for _, p := range kubeconfigPaths {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			kubeconfigExists = true
			break
		}
	}

	// In-cluster token path (platform-agnostic)
	inClusterToken := filepath.Clean("/var/run/secrets/kubernetes.io/serviceaccount/token")
	_, tokenErr := os.Stat(inClusterToken)

	res.Details["kubeconfig_paths"] = kubeconfigPaths
	res.Details["in_cluster_token_path"] = inClusterToken

	if kubeconfigExists || tokenErr == nil {
		res.Supported = true
	} else {
		res.Supported = false
		res.Error = "no kubeconfig file and no in-cluster token found"
	}
	return res
}
