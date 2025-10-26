//go:build darwin

package checks

type ResourcesChecker struct{}

func (r ResourcesChecker) ID() capcheck.CapabilityID { return "resources" }
func (r ResourcesChecker) Description() string {
	return "Reports available CPU and memory (macOS)"
}

func sysctlUint64(name string) (uint64, error) {
	val, err := unix.SysctlRaw(name)
	if err != nil {
		return 0, err
	}
	// sysctl returns native endian uint64
	return *(*uint64)(unsafe.Pointer(&val[0])), nil
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

	// hw.memsize is total physical memory
	if mem, err := sysctlUint64("hw.memsize"); err == nil {
		res.Details["memory_bytes_effective"] = mem
		res.Details["memory_detection_method"] = "sysctl(hw.memsize)"
		res.Supported = true
	} else {
		res.Details["memory_bytes_effective"] = uint64(0)
		res.Details["memory_detection_method"] = "unknown"
		res.Supported = false
		res.Error = "failed to query hw.memsize via sysctl"
	}

	res.Duration = time.Since(start)
	return res
}
