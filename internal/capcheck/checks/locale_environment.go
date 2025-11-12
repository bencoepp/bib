package checks

import (
	"bib/internal/capcheck"
	"context"
	"os"
)

type LocaleEnvironmentChecker struct{}

func (l LocaleEnvironmentChecker) ID() capcheck.CapabilityID { return "locale_environment" }
func (l LocaleEnvironmentChecker) Description() string {
	return "Reports locale-related environment variables."
}

func (l LocaleEnvironmentChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      l.ID(),
		Name:    "Locale / Environment",
		Details: map[string]any{},
	}
	vars := []string{"LANG", "LC_ALL", "LC_CTYPE", "TZ"}
	for _, v := range vars {
		if val := os.Getenv(v); val != "" {
			res.Details[v] = val
		}
	}
	if len(res.Details) > 0 {
		res.Supported = true
	} else {
		res.Error = "no locale env vars set"
	}
	return res
}
