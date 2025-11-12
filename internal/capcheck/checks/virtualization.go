package checks

import (
	"bib/internal/capcheck"
	"context"
	"os"
	"runtime"
	"strings"
)

type VirtualizationChecker struct{}

func (v VirtualizationChecker) ID() capcheck.CapabilityID { return "virtualization" }
func (v VirtualizationChecker) Description() string {
	return "Heuristically detects if running inside a container or VM."
}

func (v VirtualizationChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      v.ID(),
		Name:    "Virtualization",
		Details: map[string]any{},
	}
	res.Details["goos"] = runtime.GOOS

	// Container heuristics (Linux)
	if runtime.GOOS == "linux" {
		if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
			txt := string(data)
			if strings.Contains(txt, "docker") || strings.Contains(txt, "kubepods") || strings.Contains(txt, "containerd") {
				res.Details["container"] = true
			}
		}
		if _, err := os.Stat("/.dockerenv"); err == nil {
			res.Details["docker_env"] = true
		}
	}

	// VM heuristic: dmesg/hypervisor flags would require privileged read; minimal env / CPU vendor absent
	// Skipping deep detection due to stdlib constraints.

	res.Supported = true
	return res
}
