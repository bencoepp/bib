package checks

import (
	"bib/internal/capcheck"
	"context"
	"os"
	"path/filepath"
	"strings"
)

type KubernetesConfigChecker struct{}

func (k KubernetesConfigChecker) ID() capcheck.CapabilityID { return "kubernetes_config" }
func (k KubernetesConfigChecker) Description() string {
	return "Detects availability of Kubernetes configuration (kubeconfig or in-cluster SA token) and parses current context."
}

func (k KubernetesConfigChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      k.ID(),
		Name:    "Kubernetes config",
		Details: map[string]any{},
	}

	var kubeconfigPaths []string
	if env := os.Getenv("KUBECONFIG"); env != "" {
		for _, p := range filepath.SplitList(env) {
			kubeconfigPaths = append(kubeconfigPaths, p)
		}
	} else if home, err := os.UserHomeDir(); err == nil {
		kubeconfigPaths = append(kubeconfigPaths, filepath.Join(home, ".kube", "config"))
	}

	kubeconfigExists := false
	selectedPath := ""
	for _, p := range kubeconfigPaths {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			kubeconfigExists = true
			selectedPath = p
			break
		}
	}

	inClusterToken := filepath.Clean("/var/run/secrets/kubernetes.io/serviceaccount/token")
	_, tokenErr := os.Stat(inClusterToken)
	inClusterTokenExists := tokenErr == nil

	// Lightweight kubeconfig parsing (no external libs)
	var currentContext string
	var contextsList []string
	var contextToCluster = map[string]string{}
	var currentContextCluster string

	if kubeconfigExists && selectedPath != "" {
		if data, err := os.ReadFile(selectedPath); err == nil {
			lines := strings.Split(string(data), "\n")
			section := ""
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				if strings.HasPrefix(line, "current-context:") {
					currentContext = strings.TrimSpace(strings.TrimPrefix(line, "current-context:"))
				}
				if strings.HasPrefix(line, "contexts:") {
					section = "contexts"
					continue
				}
				if strings.HasPrefix(line, "clusters:") {
					section = "clusters"
					continue
				}
				if strings.HasPrefix(line, "- ") {
					// YAML list item, we try to capture context or cluster name
					item := strings.TrimSpace(strings.TrimPrefix(line, "- "))
					if section == "contexts" && strings.HasPrefix(item, "name:") {
						n := strings.TrimSpace(strings.TrimPrefix(item, "name:"))
						contextsList = append(contextsList, n)
					}
				}
				if section == "contexts" && strings.Contains(line, "cluster:") {
					clusterName := strings.TrimSpace(strings.TrimPrefix(line, "cluster:"))
					if currentContext != "" && strings.Contains(line, currentContext) {
						currentContextCluster = clusterName
					}
				}
			}
			// fallback attempt: if currentContextCluster still empty, guess first cluster
			if currentContextCluster == "" && len(contextToCluster) > 0 {
				if cand, ok := contextToCluster[currentContext]; ok {
					currentContextCluster = cand
				}
			}
		}
	}

	res.Details["kubeconfig_paths"] = kubeconfigPaths
	res.Details["kubeconfig_exists"] = kubeconfigExists
	res.Details["in_cluster_token_path"] = inClusterToken
	res.Details["in_cluster_token_exists"] = inClusterTokenExists
	res.Details["current_context"] = currentContext
	res.Details["current_context_cluster"] = currentContextCluster
	res.Details["available_contexts"] = contextsList

	if kubeconfigExists || inClusterTokenExists {
		res.Supported = true
	} else {
		res.Supported = false
		res.Error = "no kubeconfig file and no in-cluster token found"
	}
	return res
}
