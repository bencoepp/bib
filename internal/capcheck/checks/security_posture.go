package checks

import (
	"bib/internal/capcheck"
	"context"
	"os"
	"runtime"
)

type SecurityPostureChecker struct{}

func (s SecurityPostureChecker) ID() capcheck.CapabilityID { return "security_posture" }
func (s SecurityPostureChecker) Description() string {
	return "Heuristic security posture: presence of SELinux/AppArmor (Linux), firewall placeholder."
}

func (s SecurityPostureChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      s.ID(),
		Name:    "Security posture",
		Details: map[string]any{},
	}
	res.Details["goos"] = runtime.GOOS
	if runtime.GOOS == "linux" {
		if _, err := os.Stat("/sys/fs/selinux"); err == nil {
			res.Details["selinux_present"] = true
		}
		if _, err := os.Stat("/sys/module/apparmor"); err == nil {
			res.Details["apparmor_present"] = true
		}
	}
	res.Error = "limited heuristic only"
	res.Supported = false
	return res
}
