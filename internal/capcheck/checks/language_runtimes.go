package checks

import (
	"bib/internal/capcheck"
	"context"
	"os/exec"
	"strings"
)

type LanguageRuntimesChecker struct{}

func (l LanguageRuntimesChecker) ID() capcheck.CapabilityID { return "language_runtimes" }
func (l LanguageRuntimesChecker) Description() string {
	return "Detects versions of common language runtimes (node, python, java, go) if available."
}

func (l LanguageRuntimesChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      l.ID(),
		Name:    "Language runtimes",
		Details: map[string]any{},
	}
	type tool struct {
		Name string
		Args []string
		Key  string
	}
	candidates := []tool{
		{"node", []string{"--version"}, "node"},
		{"python", []string{"--version"}, "python"},
		{"python3", []string{"--version"}, "python3"},
		{"java", []string{"-version"}, "java"},
		{"go", []string{"version"}, "go"},
	}
	for _, c := range candidates {
		cmd := exec.CommandContext(ctx, c.Name, c.Args...)
		out, err := cmd.CombinedOutput()
		if err == nil {
			res.Details[c.Key] = strings.TrimSpace(string(out))
		}
	}
	if len(res.Details) > 0 {
		res.Supported = true
	} else {
		res.Error = "no runtimes found"
	}
	return res
}
