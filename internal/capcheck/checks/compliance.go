package checks

import (
	"bib/internal/capcheck"
	"context"
	"os"
	"runtime"
)

type ComplianceChecker struct{}

func (c ComplianceChecker) ID() capcheck.CapabilityID { return "compliance" }
func (c ComplianceChecker) Description() string {
	return "Heuristic compliance flags (FIPS mode, crypto policies - limited)."
}

func (c ComplianceChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      c.ID(),
		Name:    "Compliance",
		Details: map[string]any{},
	}
	res.Details["goos"] = runtime.GOOS
	// FIPS mode heuristic (Linux)
	if runtime.GOOS == "linux" {
		if _, err := os.Stat("/proc/sys/crypto/fips_enabled"); err == nil {
			data, _ := os.ReadFile("/proc/sys/crypto/fips_enabled")
			res.Details["fips_enabled_raw"] = string(data)
		}
	}
	res.Error = "heuristic only"
	res.Supported = false
	return res
}
