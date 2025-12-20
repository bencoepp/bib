//go:build !windows

package grpc

import (
	"net"
	"os"
)

// listenNamedPipe creates a listener for Unix sockets on non-Windows platforms.
// On Unix, this is just a regular Unix socket.
func listenNamedPipe(path string) (net.Listener, error) {
	return net.Listen("unix", path)
}

// removeSocketFile removes a Unix socket file if it exists.
func removeSocketFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return os.Remove(path)
	}
	return nil
}
