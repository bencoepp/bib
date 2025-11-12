package checks

import (
	"bib/internal/capcheck"
	"context"
	"os"
	"runtime"
)

type OSKernelFeaturesChecker struct{}

func (o OSKernelFeaturesChecker) ID() capcheck.CapabilityID { return "os_kernel_features" }
func (o OSKernelFeaturesChecker) Description() string {
	return "Reports basic kernel / feature indicators (cgroups, namespaces, security modules)."
}

func (o OSKernelFeaturesChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      o.ID(),
		Name:    "OS / Kernel features",
		Details: map[string]any{},
	}
	res.Details["goos"] = runtime.GOOS
	if runtime.GOOS == "linux" {
		if _, err := os.Stat("/sys/fs/cgroup"); err == nil {
			res.Details["cgroups"] = true
		}
		if _, err := os.Stat("/sys/fs/selinux"); err == nil {
			res.Details["selinux"] = true
		}
		// Simple AppArmor indicator
		if _, err := os.Stat("/sys/module/apparmor"); err == nil {
			res.Details["apparmor"] = true
		}
	}
	res.Supported = true
	return res
}
