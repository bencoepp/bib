package checks

import (
	"bib/internal/capcheck"
	"context"
	"runtime"
)

type SystemLimitsChecker struct{}

func (s SystemLimitsChecker) ID() capcheck.CapabilityID { return "system_limits" }
func (s SystemLimitsChecker) Description() string {
	return "Reports basic process/environment limits (partial cross-platform)."
}

func (s SystemLimitsChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      s.ID(),
		Name:    "System limits",
		Details: map[string]any{},
	}
	res.Details["goos"] = runtime.GOOS
	// Portable "ulimit" retrieval is not available in stdlib; add placeholders.
	res.Error = "detailed limits not implemented (needs syscall specific to platform)"
	res.Supported = false
	return res
}
