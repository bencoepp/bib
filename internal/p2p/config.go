// Package p2p provides libp2p-based peer-to-peer networking for bib.
package p2p

import (
	"bib/internal/config"
	"time"
)

// DefaultListenAddresses returns the default listen addresses for P2P.
func DefaultListenAddresses() []string {
	return []string{
		"/ip4/0.0.0.0/tcp/4001",
		"/ip4/0.0.0.0/udp/4001/quic-v1",
	}
}

// DefaultConnManagerConfig returns default connection manager settings.
func DefaultConnManagerConfig() config.ConnManagerConfig {
	return config.ConnManagerConfig{
		LowWatermark:  100,
		HighWatermark: 400,
		GracePeriod:   30 * time.Second,
	}
}
