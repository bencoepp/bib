package checks

import "bib/internal/capcheck"

type ContainerRuntimeChecker struct{}

func (c ContainerRuntimeChecker) ID() capcheck.CapabilityID { return "container_runtime" }
func (c ContainerRuntimeChecker) Description() string {
	return "Detects availability of common container runtimes (Docker, containerd, CRI-O)"
}
