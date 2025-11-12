package checks

import (
	"bib/internal/capcheck"
	"context"
	"os"
	"path/filepath"
	"runtime"
)

type DiskStorageChecker struct{}

func (d DiskStorageChecker) ID() capcheck.CapabilityID { return "disk_storage" }
func (d DiskStorageChecker) Description() string {
	return "Reports free space (best-effort) and mount characteristics of key paths."
}

func (d DiskStorageChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      d.ID(),
		Name:    "Disk / Storage",
		Details: map[string]any{},
	}

	paths := []string{"/", os.TempDir()}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, home)
	}

	type statResult struct {
		Path        string  `json:"path"`
		BytesFree   uint64  `json:"bytes_free"`
		BytesTotal  uint64  `json:"bytes_total"`
		PercentFree float64 `json:"percent_free"`
		Err         string  `json:"err,omitempty"`
	}

	stats := []statResult{}
	if runtime.GOOS == "windows" {
		// Placeholder: deeper implementation uses syscall.GetDiskFreeSpaceEx
		res.Error = "windows disk stats minimal (not implemented)"
		for _, p := range paths {
			if ctx.Err() != nil {
				break
			}
			stats = append(stats, statResult{Path: p})
		}
	} else {
		for _, p := range paths {
			if ctx.Err() != nil {
				break
			}
			abs, _ := filepath.Abs(p)
			st, err := statFS(abs)
			if err != nil {
				stats = append(stats, statResult{Path: abs, Err: err.Error()})
				continue
			}
			free := st.Free
			total := st.Total
			pct := 0.0
			if total > 0 {
				pct = float64(free) / float64(total)
			}
			stats = append(stats, statResult{Path: abs, BytesFree: free, BytesTotal: total, PercentFree: pct})
		}
	}

	res.Details["paths"] = stats
	res.Supported = true // even partial info is useful
	return res
}

// minimal portable struct
type fsStats struct {
	Free  uint64
	Total uint64
}
