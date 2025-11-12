package checks

import (
	"bib/internal/capcheck"
	"context"
	"os"
	"path/filepath"
	"time"
)

type DiskPerformanceChecker struct{}

func (d DiskPerformanceChecker) ID() capcheck.CapabilityID { return "disk_performance" }
func (d DiskPerformanceChecker) Description() string {
	return "Performs a tiny temp file write/read timing to estimate disk responsiveness."
}

func (d DiskPerformanceChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      d.ID(),
		Name:    "Disk performance",
		Details: map[string]any{},
	}
	dir := os.TempDir()
	path := filepath.Join(dir, ".__cap_disk_test")
	payload := []byte("0123456789abcdefghijklmnopqrstuvwxyz0123456789")
	startW := time.Now()
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		res.Error = "write failed: " + err.Error()
		return res
	}
	writeDur := time.Since(startW)
	startR := time.Now()
	data, err := os.ReadFile(path)
	readDur := time.Since(startR)
	_ = os.Remove(path)
	if err != nil {
		res.Error = "read failed: " + err.Error()
		return res
	}
	if len(data) != len(payload) {
		res.Error = "data length mismatch"
		return res
	}
	res.Supported = true
	res.Details["write_ns"] = writeDur.Nanoseconds()
	res.Details["read_ns"] = readDur.Nanoseconds()
	return res
}
