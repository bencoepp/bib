package checks

import (
	"bib/internal/capcheck"
	"context"
	"os"
	"path/filepath"
)

type TempCacheDirsChecker struct{}

func (t TempCacheDirsChecker) ID() capcheck.CapabilityID { return "temp_cache_dirs" }
func (t TempCacheDirsChecker) Description() string {
	return "Ensures temp/cache directories are writable."
}

func (t TempCacheDirsChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      t.ID(),
		Name:    "Temp & cache dirs",
		Details: map[string]any{},
	}
	dirs := []string{os.TempDir()}
	if cache := os.Getenv("XDG_CACHE_HOME"); cache != "" {
		dirs = append(dirs, cache)
	}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".cache"))
	}
	type entry struct {
		Path     string `json:"path"`
		Writable bool   `json:"writable"`
		Error    string `json:"error,omitempty"`
	}
	results := []entry{}
	for _, d := range dirs {
		testFile := filepath.Join(d, ".cap_writable_test")
		e := entry{Path: d}
		err := os.WriteFile(testFile, []byte("x"), 0o600)
		if err == nil {
			e.Writable = true
			os.Remove(testFile)
		} else {
			e.Error = err.Error()
		}
		results = append(results, e)
	}
	res.Details["dirs"] = results
	writable := 0
	for _, r := range results {
		if r.Writable {
			writable++
		}
	}
	if writable > 0 {
		res.Supported = true
	} else {
		res.Error = "no writable temp/cache dirs"
	}
	return res
}
