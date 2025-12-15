package p2p

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"bib/internal/config"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

// Bootstrapper handles connecting to bootstrap nodes with exponential backoff.
type Bootstrapper struct {
	host   host.Host
	cfg    config.BootstrapConfig
	peers  []peer.AddrInfo
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// mu protects connected
	mu        sync.RWMutex
	connected map[peer.ID]bool
}

// NewBootstrapper creates a new bootstrapper with the given configuration.
func NewBootstrapper(h host.Host, cfg config.BootstrapConfig) (*Bootstrapper, error) {
	peers, err := parseBootstrapPeers(cfg.Peers)
	if err != nil {
		return nil, fmt.Errorf("failed to parse bootstrap peers: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Bootstrapper{
		host:      h,
		cfg:       cfg,
		peers:     peers,
		ctx:       ctx,
		cancel:    cancel,
		connected: make(map[peer.ID]bool),
	}, nil
}

// Start begins the bootstrap process, connecting to peers with exponential backoff.
// It returns after at least minPeers are connected, or the context is cancelled.
func (b *Bootstrapper) Start(ctx context.Context) error {
	if len(b.peers) == 0 {
		return nil // No bootstrap peers configured
	}

	// Start connection attempts for each peer
	for _, peerInfo := range b.peers {
		b.wg.Add(1)
		go b.connectWithBackoff(peerInfo)
	}

	// Wait for minimum peers or context cancellation
	return b.waitForMinPeers(ctx)
}

// Stop stops the bootstrapper and all connection attempts.
func (b *Bootstrapper) Stop() {
	b.cancel()
	b.wg.Wait()
}

// ConnectedPeers returns the number of connected bootstrap peers.
func (b *Bootstrapper) ConnectedPeers() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	count := 0
	for _, connected := range b.connected {
		if connected {
			count++
		}
	}
	return count
}

// IsBootstrapPeer returns true if the given peer ID is a bootstrap peer.
func (b *Bootstrapper) IsBootstrapPeer(id peer.ID) bool {
	for _, p := range b.peers {
		if p.ID == id {
			return true
		}
	}
	return false
}

// connectWithBackoff attempts to connect to a peer with exponential backoff.
func (b *Bootstrapper) connectWithBackoff(peerInfo peer.AddrInfo) {
	defer b.wg.Done()

	retryInterval := b.cfg.RetryInterval
	if retryInterval == 0 {
		retryInterval = 5 * time.Second
	}
	maxInterval := b.cfg.MaxRetryInterval
	if maxInterval == 0 {
		maxInterval = time.Hour
	}

	attempt := 0
	for {
		select {
		case <-b.ctx.Done():
			return
		default:
		}

		err := b.host.Connect(b.ctx, peerInfo)
		if err == nil {
			b.mu.Lock()
			b.connected[peerInfo.ID] = true
			b.mu.Unlock()

			// Stay connected - monitor and reconnect if disconnected
			b.monitorConnection(peerInfo)

			// Reset backoff on successful reconnection
			attempt = 0
			retryInterval = b.cfg.RetryInterval
			continue
		}

		// Connection failed, apply exponential backoff
		attempt++
		backoff := time.Duration(float64(retryInterval) * math.Pow(2, float64(attempt-1)))
		if backoff > maxInterval {
			backoff = maxInterval
		}

		select {
		case <-b.ctx.Done():
			return
		case <-time.After(backoff):
		}
	}
}

// monitorConnection monitors a connection and returns when disconnected.
func (b *Bootstrapper) monitorConnection(peerInfo peer.AddrInfo) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			// Check if still connected
			if b.host.Network().Connectedness(peerInfo.ID) != 1 { // 1 = Connected
				b.mu.Lock()
				b.connected[peerInfo.ID] = false
				b.mu.Unlock()
				return // Trigger reconnection
			}
		}
	}
}

// waitForMinPeers waits until at least minPeers are connected.
func (b *Bootstrapper) waitForMinPeers(ctx context.Context) error {
	if b.cfg.MinPeers <= 0 {
		return nil // No minimum required
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-b.ctx.Done():
			return b.ctx.Err()
		case <-ticker.C:
			if b.ConnectedPeers() >= b.cfg.MinPeers {
				return nil
			}
		}
	}
}

// parseBootstrapPeers parses multiaddr strings into peer.AddrInfo.
// Supports both full multiaddrs with peer ID and DNS-only addresses.
func parseBootstrapPeers(addrs []string) ([]peer.AddrInfo, error) {
	var peers []peer.AddrInfo
	seen := make(map[peer.ID]bool)

	for _, addr := range addrs {
		ma, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid multiaddr %q: %w", addr, err)
		}

		// Try to extract peer ID from multiaddr
		addrInfo, err := peer.AddrInfoFromP2pAddr(ma)
		if err != nil {
			// No peer ID in address - store as address-only
			// This will be used for DNS-based discovery where peer ID isn't known yet
			// For now, we'll skip these until we implement peer ID discovery
			continue
		}

		// Deduplicate by peer ID, merging addresses
		if seen[addrInfo.ID] {
			// Find and merge with existing
			for i, p := range peers {
				if p.ID == addrInfo.ID {
					peers[i].Addrs = append(peers[i].Addrs, addrInfo.Addrs...)
					break
				}
			}
		} else {
			seen[addrInfo.ID] = true
			peers = append(peers, *addrInfo)
		}
	}

	return peers, nil
}

// DefaultBootstrapPeers returns the default bootstrap peer addresses.
// Note: These require a peer ID to be useful. The actual bib.dev peer ID
// should be configured or discovered.
func DefaultBootstrapPeers() []string {
	return []string{
		"/dns4/bib.dev/tcp/4001",
		"/dns4/bib.dev/udp/4001/quic-v1",
	}
}
