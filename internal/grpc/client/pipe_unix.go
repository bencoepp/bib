//go:build !windows
// +build !windows

package client

import (
	"context"
	"fmt"
	"net"
)

// dialNamedPipe is a no-op on non-Windows platforms.
func dialNamedPipe(ctx context.Context, addr string) (net.Conn, error) {
	return nil, fmt.Errorf("named pipes not supported on this platform")
}
