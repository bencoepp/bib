package checks

import (
	"bib/internal/capcheck"
	"context"
	"runtime"
)

type PerformanceHealthChecker struct{}

func (p PerformanceHealthChecker) ID() capcheck.CapabilityID { return "performance_health" }
func (p PerformanceHealthChecker) Description() string {
	return "Captures a quick snapshot (goroutines, GC pause, load placeholders)."
}

func (p PerformanceHealthChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      p.ID(),
		Name:    "Performance health",
		Details: map[string]any{},
	}
	res.Details["goroutines"] = runtime.NumGoroutine()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	res.Details["last_gc_unix"] = ms.LastGC / 1e9
	res.Details["pause_total_ns"] = ms.PauseTotalNs
	res.Details["num_gc"] = ms.NumGC
	// Load average (portable) not available; placeholder:
	res.Details["load_avg_supported"] = false
	res.Supported = true
	return res
}
