package checks

import (
	"bib/internal/capcheck"
	"context"
	"os"
	"path/filepath"
	"runtime"
)

type GPUChecker struct{}

func (g GPUChecker) ID() capcheck.CapabilityID { return "gpu" }
func (g GPUChecker) Description() string {
	return "Detects presence of basic GPU/accelerator indicators (NVIDIA devices, env hints)."
}

func (g GPUChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      g.ID(),
		Name:    "GPU / Accelerators",
		Details: map[string]any{},
	}

	// Simple heuristics: presence of /dev/nvidia* (Unix), or CUDA env vars.
	if runtime.GOOS != "windows" {
		matches := []string{}
		filepath.WalkDir("/dev", func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			base := filepath.Base(path)
			if d.Type().IsRegular() && (base == "nvidiactl" || base == "nvidia-uvm" || (len(base) > 6 && base[:6] == "nvidia")) {
				matches = append(matches, path)
			}
			return nil
		})
		if len(matches) > 0 {
			res.Supported = true
			res.Details["nvidia_devices"] = matches
		}
	}

	if cudaPath := os.Getenv("CUDA_HOME"); cudaPath != "" {
		res.Details["cuda_home"] = cudaPath
		res.Supported = true
	}
	if toolkit := os.Getenv("NVIDIA_VISIBLE_DEVICES"); toolkit != "" {
		res.Details["nvidia_visible_devices"] = toolkit
		res.Supported = true
	}

	if !res.Supported {
		res.Error = "no GPU indicators detected"
	}
	return res
}
