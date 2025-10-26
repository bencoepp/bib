//go:build !windows

package checks

import (
	"bib/internal/capcheck"
	"context"
	"errors"
	"net"
	"os"
	"time"
)

func dialUnix(ctx context.Context, socket string, timeout time.Duration) error {
	dialer := net.Dialer{Timeout: timeout}
	if d, ok := ctx.Deadline(); ok {
		if rem := time.Until(d); rem < timeout {
			dialer.Timeout = rem
		}
	}
	conn, err := dialer.DialContext(ctx, "unix", socket)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}

func (c ContainerRuntimeChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      c.ID(),
		Name:    "Container runtime",
		Details: map[string]any{},
	}

	candidates := []struct {
		Name   string
		Socket string
	}{
		{"docker", "/var/run/docker.sock"},
		{"containerd", "/run/containerd/containerd.sock"},
		{"cri-o", "/var/run/crio/crio.sock"},
	}

	found := false
	available := []string{}
	errorsMap := map[string]string{}

	for _, cand := range candidates {
		if _, err := os.Stat(cand.Socket); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			errorsMap[cand.Name] = err.Error()
			continue
		}
		if err := dialUnix(ctx, cand.Socket, 300*time.Millisecond); err != nil {
			errorsMap[cand.Name] = "socket present but not responsive: " + err.Error()
			continue
		}
		found = true
		available = append(available, cand.Name)
	}

	res.Supported = found
	if !found && len(errorsMap) > 0 {
		res.Error = "no responsive container runtime socket found"
	}
	res.Details["available"] = available
	if len(errorsMap) > 0 {
		res.Details["errors"] = errorsMap
	}
	return res
}
