//go:build !linux && !windows && !darwin && !freebsd && !openbsd && !netbsd && !dragonfly

package checks

import (
	"bib/internal/capcheck"
	"context"
	"runtime"
	"time"
)

type ResourcesChecker struct{}

func (r ResourcesChecker) ID() capcheck.CapabilityID { return "resources" }
func (r ResourcesChecker) Description() string {
	return "Reports available CPU and memory (generic fallback)"
}

func (r ResourcesChecker) Check(ctx context.Context) capcheck.CheckResult {
	start := time.Now()
	res := capcheck.CheckResult{
		ID:      r.ID(),
		Name:    "Resources",
		Details: map[string]any{},
	}

	res.Details["cpu_cores_effective"] = float64(runtime.NumCPU())
	res.Details["cpu_detection_method"] = "host_numcpu"

	// Unknown total memory on this platform with stdlib only
	res.Details["memory_bytes_effective"] = uint64(0)
	res.Details["memory_detection_method"] = "unknown"
	res.Supported = false
	res.Error = "memory detection not implemented for this platform"

	res.Duration = time.Since(start)
	return res
}
