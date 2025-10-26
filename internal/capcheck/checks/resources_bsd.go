//go:build freebsd || openbsd || netbsd || dragonfly

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
	return "Reports available CPU and memory (BSD)"
}

func sysctlUint64Fallback(names ...string) (uint64, error) {
	var lastErr error
	for _, n := range names {
		if v, err := unix.SysctlUint64(n); err == nil {
			return v, nil
		} else {
			lastErr = err
		}
	}
	return 0, lastErr
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

	// Try common BSD sysctls
	// FreeBSD: hw.physmem
	// OpenBSD: hw.physmem64
	// NetBSD: hw.physmem64
	if mem, err := sysctlUint64Fallback("hw.physmem64", "hw.physmem"); err == nil {
		res.Details["memory_bytes_effective"] = mem
		res.Details["memory_detection_method"] = "sysctl(hw.physmem*)"
		res.Supported = true
	} else {
		res.Details["memory_bytes_effective"] = uint64(0)
		res.Details["memory_detection_method"] = "unknown"
		res.Supported = false
		res.Error = "failed to query physmem via sysctl"
	}

	res.Duration = time.Since(start)
	return res
}
