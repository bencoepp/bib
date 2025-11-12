package checks

import (
	"bib/internal/capcheck"
	"context"
	"os"
	"runtime"
)

type VPNChecker struct{}

func (v VPNChecker) ID() capcheck.CapabilityID { return "vpn" }
func (v VPNChecker) Description() string {
	return "Heuristic VPN detection via common environment or network interface hints."
}

func (v VPNChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      v.ID(),
		Name:    "VPN",
		Details: map[string]any{},
	}
	res.Details["goos"] = runtime.GOOS
	// Very naive: environment variables sometimes set
	if os.Getenv("VPN") != "" {
		res.Details["env_vpn"] = true
		res.Supported = true
	}
	// Real detection would parse interfaces; skipping for brevity
	// TODO : implement network interface checks
	if !res.Supported {
		res.Error = "no vpn indicators"
	}
	return res
}
