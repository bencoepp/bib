package checks

import (
	"bib/internal/capcheck"
	"context"
	"net/http"
	"os"
)

type ProxyChecker struct{}

func (p ProxyChecker) ID() capcheck.CapabilityID { return "proxy" }
func (p ProxyChecker) Description() string {
	return "Detects proxy environment variables and validates preferred proxy for HTTPS."
}

func (p ProxyChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      p.ID(),
		Name:    "Proxy / Egress",
		Details: map[string]any{},
	}
	envs := []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "no_proxy"}
	for _, e := range envs {
		if val := os.Getenv(e); val != "" {
			res.Details[e] = val
		}
	}
	// Minimal validation: ask http.ProxyFromEnvironment for a test URL.
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://example.com", nil)
	url, err := http.ProxyFromEnvironment(req)
	if err == nil && url != nil {
		res.Details["proxy_detected"] = url.String()
		res.Supported = true
	} else {
		res.Error = "no proxy configured or detection failed"
	}
	return res
}
