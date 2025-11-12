package checks

import (
	"bib/internal/capcheck"
	"context"
	"time"
)

type TimeNTPChecker struct{}

func (t TimeNTPChecker) ID() capcheck.CapabilityID { return "time_ntp" }
func (t TimeNTPChecker) Description() string {
	return "Reports local clock and monotonic resolution (no real NTP query; placeholder)."
}

func (t TimeNTPChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      t.ID(),
		Name:    "Time / NTP",
		Details: map[string]any{},
	}
	now := time.Now()
	res.Details["wall_time"] = now.UTC().Format(time.RFC3339Nano)
	// Monotonic delta measurement
	start := time.Now()
	time.Sleep(2 * time.Millisecond)
	diff := time.Since(start)
	res.Details["monotonic_test_ns"] = diff.Nanoseconds()
	res.Supported = true
	return res
}
