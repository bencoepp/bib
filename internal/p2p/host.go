package p2p

import (
	"context"
	"fmt"
	"time"

	"bib/internal/config"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	libp2pquic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/multiformats/go-multiaddr"
)

// Host wraps a libp2p host with bib-specific functionality.
type Host struct {
	host.Host
	cfg config.P2PConfig
}

// NewHost creates a new libp2p host with the given configuration.
// The configDir is used to resolve the identity key path if not explicitly set.
func NewHost(ctx context.Context, cfg config.P2PConfig, configDir string) (*Host, error) {
	// Load or generate identity
	identity, err := LoadOrGenerateIdentity(cfg.Identity.KeyPath, configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load identity: %w", err)
	}

	// Parse listen addresses
	listenAddrs, err := parseMultiaddrs(cfg.ListenAddresses)
	if err != nil {
		return nil, fmt.Errorf("failed to parse listen addresses: %w", err)
	}

	// Create connection manager
	connMgr, err := connmgr.NewConnManager(
		cfg.ConnManager.LowWatermark,
		cfg.ConnManager.HighWatermark,
		connmgr.WithGracePeriod(cfg.ConnManager.GracePeriod),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection manager: %w", err)
	}

	// Build libp2p host options
	opts := []libp2p.Option{
		// Identity
		libp2p.Identity(identity.PrivKey),

		// Listen addresses
		libp2p.ListenAddrs(listenAddrs...),

		// Transports: TCP and QUIC
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.Transport(libp2pquic.NewTransport),

		// Security: Noise protocol
		libp2p.Security(noise.ID, noise.New),

		// Multiplexing: Yamux is the default in go-libp2p

		// Connection manager
		libp2p.ConnectionManager(connMgr),

		// Enable NAT port mapping (UPnP/NAT-PMP)
		libp2p.NATPortMap(),

		// Enable hole punching for NAT traversal
		libp2p.EnableHolePunching(),
	}

	// Create the libp2p host
	h, err := libp2p.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	return &Host{
		Host: h,
		cfg:  cfg,
	}, nil
}

// PeerID returns the peer ID of this host.
func (h *Host) PeerID() peer.ID {
	return h.Host.ID()
}

// ListenAddrs returns the addresses this host is listening on.
func (h *Host) ListenAddrs() []multiaddr.Multiaddr {
	return h.Host.Addrs()
}

// FullAddrs returns the full multiaddrs (including peer ID) for this host.
func (h *Host) FullAddrs() []multiaddr.Multiaddr {
	hostAddr, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/p2p/%s", h.PeerID()))

	var addrs []multiaddr.Multiaddr
	for _, addr := range h.ListenAddrs() {
		addrs = append(addrs, addr.Encapsulate(hostAddr))
	}
	return addrs
}

// Close shuts down the host.
func (h *Host) Close() error {
	return h.Host.Close()
}

// parseMultiaddrs parses a slice of multiaddr strings.
func parseMultiaddrs(addrs []string) ([]multiaddr.Multiaddr, error) {
	result := make([]multiaddr.Multiaddr, 0, len(addrs))
	for _, addr := range addrs {
		ma, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid multiaddr %q: %w", addr, err)
		}
		result = append(result, ma)
	}
	return result, nil
}

// WaitForReady waits for the host to be ready with a timeout.
func (h *Host) WaitForReady(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Check if we have at least one listen address
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for host to be ready: %w", ctx.Err())
		case <-ticker.C:
			if len(h.ListenAddrs()) > 0 {
				return nil
			}
		}
	}
}
