//go:build windows

package grpc

import (
	"net"

	"github.com/Microsoft/go-winio"
)

// listenNamedPipe creates a Windows named pipe listener.
func listenNamedPipe(pipeName string) (net.Listener, error) {
	// Configure the named pipe with appropriate security
	cfg := &winio.PipeConfig{
		SecurityDescriptor: "", // Use default security (current user + Administrators)
		MessageMode:        false,
		InputBufferSize:    65536,
		OutputBufferSize:   65536,
	}

	return winio.ListenPipe(pipeName, cfg)
}

// removeSocketFile is a no-op on Windows since named pipes don't leave files.
func removeSocketFile(_ string) error {
	return nil
}
