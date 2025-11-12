package checks

import (
	"bib/internal/capcheck"
	"context"
	"net"
	"time"
)

type SourceControlConnectivityChecker struct{}

func (s SourceControlConnectivityChecker) ID() capcheck.CapabilityID {
	return "source_control_connectivity"
}
func (s SourceControlConnectivityChecker) Description() string {
	return "Checks TCP connectivity to GitHub over SSH (22) and HTTPS (443)."
}

func (s SourceControlConnectivityChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      s.ID(),
		Name:    "Source control connectivity",
		Details: map[string]any{},
	}

	type hostPort struct {
		Host string
		Port string
		Key  string
	}
	targets := []hostPort{
		{"github.com", "22", "ssh"},
		{"github.com", "443", "https"},
	}

	dialer := net.Dialer{Timeout: 500 * time.Millisecond}
	success := 0
	for _, t := range targets {
		start := time.Now()
		conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(t.Host, t.Port))
		lat := time.Since(start).Milliseconds()
		keyLatency := t.Key + "_latency_ms"
		if err != nil {
			res.Details[t.Key+"_error"] = err.Error()
		} else {
			res.Details[t.Key+"_ok"] = true
			res.Details[keyLatency] = lat
			success++
			conn.Close()
		}
	}
	if success == len(targets) {
		res.Supported = true
	} else {
		res.Error = "one or more endpoints unreachable"
	}
	return res
}
