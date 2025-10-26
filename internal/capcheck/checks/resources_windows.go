//go:build windows

package checks

import (
	"bib/internal/capcheck"
	"context"
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type ResourcesChecker struct{}

func (r ResourcesChecker) ID() capcheck.CapabilityID { return "resources" }
func (r ResourcesChecker) Description() string {
	return "Reports available CPU and memory (Windows)"
}

type memoryStatusEx struct {
	cbSize                  uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

var (
	kernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	procGlobalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
)

func globalMemoryStatusEx(msx *memoryStatusEx) error {
	msx.cbSize = uint32(unsafe.Sizeof(*msx))
	r1, _, err := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(msx)))
	if r1 == 0 {
		return err
	}
	return nil
}

func (r ResourcesChecker) Check(ctx context.Context) capcheck.CheckResult {
	start := time.Now()
	res := capcheck.CheckResult{
		ID:      r.ID(),
		Name:    "Resources",
		Details: map[string]any{},
	}

	// CPU cores (no easy generic container limit detection on Windows)
	cores := runtime.NumCPU()
	res.Details["cpu_cores_effective"] = float64(cores)
	res.Details["cpu_detection_method"] = "host_numcpu"

	// Memory via GlobalMemoryStatusEx (total physical)
	var msx memoryStatusEx
	if err := globalMemoryStatusEx(&msx); err == nil {
		res.Details["memory_bytes_effective"] = msx.ullTotalPhys
		res.Details["memory_detection_method"] = "GlobalMemoryStatusEx"
		res.Supported = true
	} else {
		res.Details["memory_bytes_effective"] = uint64(0)
		res.Details["memory_detection_method"] = "unknown"
		res.Supported = false
		res.Error = "failed to query memory via GlobalMemoryStatusEx"
	}

	res.Duration = time.Since(start)
	return res
}
