//go:build windows

package checks

import (
	"bib/internal/capcheck"
	"context"
	"time"

	"github.com/Microsoft/go-winio"
)

func (c ContainerRuntimeChecker) Check(ctx context.Context) capcheck.CheckResult {
	res := capcheck.CheckResult{
		ID:      c.ID(),
		Name:    "Container runtime",
		Details: map[string]any{},
	}

	type pipeCand struct {
		Name string
		Pipe string
	}

	candidates := []pipeCand{
		{"docker", `\\.\pipe\docker_engine`},
		{"containerd", `\\.\pipe\containerd-containerd`},
	}

	found := false
	available := []string{}
	errorsMap := map[string]string{}

	for _, cand := range candidates {
		d := 300 * time.Millisecond
		if deadline, ok := ctx.Deadline(); ok {
			if rem := time.Until(deadline); rem < d {
				d = rem
			}
		}
		conn, err := winio.DialPipe(cand.Pipe, &d)
		if err != nil {
			errorsMap[cand.Name] = err.Error()
			continue
		}
		_ = conn.Close()
		found = true
		available = append(available, cand.Name)
	}

	res.Supported = found
	if !found && len(errorsMap) > 0 {
		res.Error = "no responsive container runtime pipe found"
	}
	res.Details["available"] = available
	if len(errorsMap) > 0 {
		res.Details["errors"] = errorsMap
	}
	return res
}
