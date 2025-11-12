package checks

import (
	"bib/internal/capcheck"
	"context"
	"net"
	"time"
)

type DNSResolverChecker struct{}

func (d DNSResolverChecker) ID() capcheck.CapabilityID { return "dns_resolver" }
func (d DNSResolverChecker) Description() string {
	return "Examines basic DNS resolution latency for a set of hostnames."
}

func (d DNSResolverChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      d.ID(),
		Name:    "DNS resolver",
		Details: map[string]any{},
	}
	hosts := []string{"example.com", "github.com", "golang.org"}
	type r struct {
		Host     string   `json:"host"`
		Success  bool     `json:"success"`
		IPs      []string `json:"ips,omitempty"`
		LatencyM int64    `json:"latency_ms"`
		Error    string   `json:"error,omitempty"`
	}
	results := []r{}
	for _, h := range hosts {
		if ctx.Err() != nil {
			break
		}
		start := time.Now()
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, h)
		lat := time.Since(start).Milliseconds()
		entry := r{Host: h, LatencyM: lat}
		if err != nil {
			entry.Success = false
			entry.Error = err.Error()
		} else {
			entry.Success = true
			for _, ip := range ips {
				entry.IPs = append(entry.IPs, ip.IP.String())
			}
		}
		results = append(results, entry)
	}
	res.Details["queries"] = results
	successes := 0
	for _, q := range results {
		if q.Success {
			successes++
		}
	}
	if successes == len(results) {
		res.Supported = true
	} else {
		res.Error = "one or more DNS queries failed"
	}
	return res
}
