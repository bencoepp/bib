package p2p

import (
	"context"
	"fmt"
	"time"

	"bib/internal/config"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/metrics"
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
	cfg              config.P2PConfig
	bandwidthCounter *metrics.BandwidthCounter
}

// NewHost creates a new libp2p host with the given configuration.
// The configDir is used to resolve the identity key path if not explicitly set.
func NewHost(ctx context.Context, cfg config.P2PConfig, configDir string) (*Host, error) {
	hostLog := getLogger("host")

	hostLog.Debug("creating P2P host",
		"listen_addresses", cfg.ListenAddresses,
		"conn_low_watermark", cfg.ConnManager.LowWatermark,
		"conn_high_watermark", cfg.ConnManager.HighWatermark,
		"bandwidth_metering", cfg.Metrics.BandwidthMetering,
	)

	// Load or generate identity
	identity, err := LoadOrGenerateIdentity(cfg.Identity.KeyPath, configDir)
	if err != nil {
		hostLog.Error("failed to load identity", "error", err)
		return nil, fmt.Errorf("failed to load identity: %w", err)
	}

	peerID, err := peer.IDFromPrivateKey(identity.PrivKey)
	if err != nil {
		hostLog.Error("failed to get peer ID from identity", "error", err)
		return nil, fmt.Errorf("failed to get peer ID: %w", err)
	}
	hostLog.Debug("loaded P2P identity", "peer_id", peerID.String())

	// Parse listen addresses
	listenAddrs, err := parseMultiaddrs(cfg.ListenAddresses)
	if err != nil {
		hostLog.Error("failed to parse listen addresses", "error", err)
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

	// Add bandwidth metering if enabled
	var bwCounter *metrics.BandwidthCounter
	if cfg.Metrics.BandwidthMetering {
		bwCounter = metrics.NewBandwidthCounter()
		opts = append(opts, libp2p.BandwidthReporter(bwCounter))
		hostLog.Debug("bandwidth metering enabled")
	}

	// Create the libp2p host
	h, err := libp2p.New(opts...)
	if err != nil {
		hostLog.Error("failed to create libp2p host", "error", err)
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	hostLog.Info("P2P host created",
		"peer_id", h.ID().String(),
		"addrs", h.Addrs(),
	)

	return &Host{
		Host:             h,
		cfg:              cfg,
		bandwidthCounter: bwCounter,
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

// BandwidthStats returns bandwidth statistics if metering is enabled.
// Returns (bytesSent, bytesReceived, enabled).
func (h *Host) BandwidthStats() (int64, int64, bool) {
	if h.bandwidthCounter == nil {
		return 0, 0, false
	}
	stats := h.bandwidthCounter.GetBandwidthTotals()
	return stats.TotalOut, stats.TotalIn, true
}

// ConnectedPeersCount returns the number of currently connected peers.
func (h *Host) ConnectedPeersCount() int {
	return len(h.Host.Network().Peers())
}

// ActiveStreamsCount returns the number of active streams.
func (h *Host) ActiveStreamsCount() int {
	count := 0
	for _, conn := range h.Host.Network().Conns() {
		count += conn.Stat().NumStreams
	}
	return count
}

// Config returns the P2P configuration.
func (h *Host) Config() config.P2PConfig {
	return h.cfg
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
