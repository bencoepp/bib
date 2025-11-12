package checks

import (
	"bib/internal/capcheck"
	"context"
	"net"
	"time"
)

type NetworkReachabilityChecker struct{}

func (n NetworkReachabilityChecker) ID() capcheck.CapabilityID { return "network_reachability" }
func (n NetworkReachabilityChecker) Description() string {
	return "Attempts outbound TCP connections to common ports (53, 80, 443) to infer basic reachability and IPv6 support."
}

func (n NetworkReachabilityChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      n.ID(),
		Name:    "Network reachability",
		Details: map[string]any{},
	}

	targets := []struct {
		Host string
		Port string
	}{
		{"1.1.1.1", "53"},
		{"8.8.8.8", "53"},
		{"1.1.1.1", "80"},
		{"1.1.1.1", "443"},
		{"8.8.8.8", "443"},
	}
	type attempt struct {
		Host     string `json:"host"`
		Port     string `json:"port"`
		Success  bool   `json:"success"`
		LatencyM int64  `json:"latency_ms"`
		Error    string `json:"error,omitempty"`
	}
	attempts := []attempt{}
	dialer := net.Dialer{Timeout: 400 * time.Millisecond}
	for _, t := range targets {
		if ctx.Err() != nil {
			break
		}
		addr := net.JoinHostPort(t.Host, t.Port)
		start := time.Now()
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		lat := time.Since(start).Milliseconds()
		a := attempt{Host: t.Host, Port: t.Port, LatencyM: lat}
		if err != nil {
			a.Success = false
			a.Error = err.Error()
		} else {
			a.Success = true
			conn.Close()
		}
		attempts = append(attempts, a)
	}
	res.Details["attempts"] = attempts

	// IPv6 quick test (Google DNS)
	ipv6Conn, err6 := dialer.DialContext(ctx, "udp", "[2001:4860:4860::8888]:53")
	if err6 == nil {
		res.Details["ipv6"] = true
		ipv6Conn.Close()
	} else {
		res.Details["ipv6"] = false
		res.Details["ipv6_error"] = err6.Error()
	}

	// success if at least one 53 and one 443 succeeded
	var ok53, ok443 bool
	for _, a := range attempts {
		if a.Success && a.Port == "53" {
			ok53 = true
		}
		if a.Success && a.Port == "443" {
			ok443 = true
		}
	}
	if ok53 && ok443 {
		res.Supported = true
	} else {
		res.Error = "insufficient outbound success (need DNS and HTTPS)"
	}
	return res
}
