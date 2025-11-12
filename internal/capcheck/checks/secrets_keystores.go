package checks

import (
	"bib/internal/capcheck"
	"context"
	"os"
	"path/filepath"
)

type SecretsKeyStoresChecker struct{}

func (s SecretsKeyStoresChecker) ID() capcheck.CapabilityID { return "secrets_keystores" }
func (s SecretsKeyStoresChecker) Description() string {
	return "Detects presence of common secret storage config files (best-effort)."
}

func (s SecretsKeyStoresChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      s.ID(),
		Name:    "Secrets / Key stores",
		Details: map[string]any{},
	}
	paths := []string{}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths,
			filepath.Join(home, ".dockercfg"),
			filepath.Join(home, ".docker", "config.json"),
			filepath.Join(home, ".netrc"),
			filepath.Join(home, ".ssh", "config"),
		)
	}
	found := []string{}
	for _, p := range paths {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			found = append(found, p)
		}
	}
	res.Details["found"] = found
	if len(found) > 0 {
		res.Supported = true
	} else {
		res.Error = "no known secret store files present"
	}
	return res
}
