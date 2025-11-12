package checks

import (
	"bib/internal/capcheck"
	"context"
	"os"
	"os/exec"
	"path/filepath"
)

type CLIToolchainChecker struct{}

func (c CLIToolchainChecker) ID() capcheck.CapabilityID { return "cli_toolchain" }
func (c CLIToolchainChecker) Description() string {
	return "Detects presence of core CLI tools (git, ssh, tar, gzip)."
}

func (c CLIToolchainChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      c.ID(),
		Name:    "CLI toolchain",
		Details: map[string]any{},
	}

	tools := []string{"git", "ssh", "tar", "gzip"}
	found := []string{}
	missing := []string{}
	for _, t := range tools {
		p, err := exec.LookPath(t)
		if err != nil {
			missing = append(missing, t)
		} else {
			found = append(found, filepath.Clean(p))
		}
	}
	res.Details["found"] = found
	res.Details["missing"] = missing
	if len(found) > 0 {
		res.Supported = true
	}
	if home, err := os.UserHomeDir(); err == nil {
		res.Details["home"] = home
	}
	return res
}
