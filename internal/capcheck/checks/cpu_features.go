package checks

import (
	"bib/internal/capcheck"
	"context"
	"runtime"
)

type CPUFeaturesChecker struct{}

func (c CPUFeaturesChecker) ID() capcheck.CapabilityID { return "cpu_features" }
func (c CPUFeaturesChecker) Description() string {
	return "Reports basic CPU concurrency and architecture (no advanced flags without external libs)."
}

func (c CPUFeaturesChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      c.ID(),
		Name:    "CPU features",
		Details: map[string]any{},
	}
	res.Details["arch"] = runtime.GOARCH
	res.Details["cores_logical"] = runtime.NumCPU()
	// Advanced features (AVX/...) need cpuid; not implemented here.
	res.Supported = true
	return res
}
