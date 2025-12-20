//go:build windows
// +build windows

package client

import (
	"context"
	"net"
	"time"

	"github.com/Microsoft/go-winio"
)

// dialNamedPipe connects to a Windows named pipe.
func dialNamedPipe(ctx context.Context, addr string) (net.Conn, error) {
	// Extract timeout from context
	deadline, ok := ctx.Deadline()
	var timeout time.Duration
	if ok {
		timeout = time.Until(deadline)
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
	} else {
		timeout = 30 * time.Second
	}

	return winio.DialPipe(addr, &timeout)
}
