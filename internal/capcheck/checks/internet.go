package checks

import (
	"bib/internal/capcheck"
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

type InternetAccessChecker struct {
	HTTPURL string
}

func (i InternetAccessChecker) ID() capcheck.CapabilityID { return "internet_access" }
func (i InternetAccessChecker) Description() string {
	return "Checks DNS resolution and HTTPS reachability to determine internet access"
}

func (i InternetAccessChecker) Check(ctx context.Context) capcheck.CheckResult {
	url := i.HTTPURL
	if url == "" {
		url = "https://example.com"
	}
	res := capcheck.CheckResult{
		ID:      i.ID(),
		Name:    "Internet access",
		Details: map[string]any{"url": url},
	}

	// DNS
	dnsCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	_, dnsErr := net.DefaultResolver.LookupIPAddr(dnsCtx, "example.com")
	if dnsErr != nil {
		res.Details["dns_error"] = dnsErr.Error()
	}

	// HTTPS HEAD
	client := &http.Client{
		Timeout: 800 * time.Millisecond,
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			TLSHandshakeTimeout: 500 * time.Millisecond,
			TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
			IdleConnTimeout:     300 * time.Millisecond,
		},
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	httpResp, httpErr := client.Do(req)
	if httpResp != nil {
		httpResp.Body.Close()
	}

	if httpErr == nil && httpResp.StatusCode < 500 {
		res.Supported = true
	} else {
		res.Supported = false
		if httpErr != nil {
			res.Error = httpErr.Error()
		} else {
			res.Error = httpResp.Status
		}
	}
	return res
}
