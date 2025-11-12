package checks

import (
	"bib/internal/capcheck"
	"context"
	"os/exec"
)

type ContainerEcosystemChecker struct{}

func (c ContainerEcosystemChecker) ID() capcheck.CapabilityID { return "container_ecosystem" }
func (c ContainerEcosystemChecker) Description() string {
	return "Detects auxiliary container build/pull tooling (docker, podman, nerdctl, buildx)."
}

func (c ContainerEcosystemChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      c.ID(),
		Name:    "Container ecosystem",
		Details: map[string]any{},
	}
	tools := []string{"docker", "podman", "nerdctl", "buildx"}
	found := []string{}
	for _, t := range tools {
		if _, err := exec.LookPath(t); err == nil {
			found = append(found, t)
		}
	}
	res.Details["found"] = found
	if len(found) > 0 {
		res.Supported = true
	} else {
		res.Error = "no container ecosystem tools found"
	}
	return res
}
