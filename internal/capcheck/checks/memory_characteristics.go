package checks

import (
	"bib/internal/capcheck"
	"context"
	"runtime"
)

type MemoryCharacteristicsChecker struct{}

func (m MemoryCharacteristicsChecker) ID() capcheck.CapabilityID { return "memory_characteristics" }
func (m MemoryCharacteristicsChecker) Description() string {
	return "Reports Go memory stats (heap) as a runtime characteristic (not total system)."
}

func (m MemoryCharacteristicsChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      m.ID(),
		Name:    "Memory characteristics",
		Details: map[string]any{},
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	res.Details["alloc_bytes"] = ms.Alloc
	res.Details["sys_bytes"] = ms.Sys
	res.Details["heap_alloc"] = ms.HeapAlloc
	res.Details["heap_sys"] = ms.HeapSys
	res.Details["heap_idle"] = ms.HeapIdle
	res.Details["heap_inuse"] = ms.HeapInuse
	res.Supported = true
	return res
}
