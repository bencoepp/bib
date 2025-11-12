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
	HttpUrl string
}

func (i InternetAccessChecker) ID() capcheck.CapabilityID { return "internet_access" }
func (i InternetAccessChecker) Description() string {
	return "Checks DNS resolution and HTTPS reachability (with latency + TLS info)."
}

func (i InternetAccessChecker) Check(ctx context.Context) capcheck.CheckResult {
	url := i.HttpUrl
	if url == "" {
		url = "https://example.com"
	}
	res := capcheck.CheckResult{
		ID:      i.ID(),
		Name:    "Internet access",
		Details: map[string]any{"url": url},
	}

	// DNS
	dnsStart := time.Now()
	dnsCtx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()
	ips, dnsErr := net.DefaultResolver.LookupIPAddr(dnsCtx, "example.com")
	if dnsErr != nil {
		res.Details["dns_error"] = dnsErr.Error()
		res.Details["dns_success"] = false
	} else {
		res.Details["dns_success"] = true
		res.Details["resolved_ips"] = ipAddrToStrings(ips)
	}
	res.Details["dns_latency_ms"] = time.Since(dnsStart).Milliseconds()

	// HTTPS HEAD
	httpStart := time.Now()
	client := &http.Client{
		Timeout: 1500 * time.Millisecond,
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			TLSHandshakeTimeout: 800 * time.Millisecond,
			TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
			IdleConnTimeout:     500 * time.Millisecond,
		},
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	httpResp, httpErr := client.Do(req)
	if httpResp != nil {
		res.Details["http_status_code"] = httpResp.StatusCode
		if httpResp.TLS != nil {
			res.Details["tls_version"] = tlsVersionName(httpResp.TLS.Version)
		}
		err := httpResp.Body.Close()
		if err != nil {
			return capcheck.CheckResult{}
		}
	}
	res.Details["http_latency_ms"] = time.Since(httpStart).Milliseconds()

	if httpErr == nil && httpResp != nil && httpResp.StatusCode < 500 {
		res.Supported = true
		res.Details["http_success"] = true
	} else {
		res.Supported = false
		res.Details["http_success"] = false
		if httpErr != nil {
			res.Error = httpErr.Error()
		} else if httpResp != nil {
			res.Error = httpResp.Status
		} else {
			res.Error = "no http response"
		}
	}
	return res
}

func ipAddrToStrings(addresses []net.IPAddr) []string {
	out := make([]string, 0, len(addresses))
	for _, a := range addresses {
		out = append(out, a.IP.String())
	}
	return out
}

func tlsVersionName(v uint16) string {
	switch v {
	case tls.VersionTLS13:
		return "TLS1.3"
	case tls.VersionTLS12:
		return "TLS1.2"
	default:
		return "UNKNOWN"
	}
}
