package discovery

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// P2PDiscoveryConfig contains configuration for P2P discovery
type P2PDiscoveryConfig struct {
	// BootstrapPeers are the initial peers to connect to
	BootstrapPeers []string

	// Timeout is the maximum time for discovery
	Timeout time.Duration

	// MaxPeers is the maximum number of peers to return
	MaxPeers int

	// MeasureLatency enables latency measurement
	MeasureLatency bool

	// LatencyTimeout is the timeout for latency measurement
	LatencyTimeout time.Duration
}

// DefaultP2PDiscoveryConfig returns a default P2P discovery configuration
func DefaultP2PDiscoveryConfig() P2PDiscoveryConfig {
	return P2PDiscoveryConfig{
		BootstrapPeers: []string{
			"bootstrap.bib.dev:4001",
		},
		Timeout:        10 * time.Second,
		MaxPeers:       50,
		MeasureLatency: true,
		LatencyTimeout: 2 * time.Second,
	}
}

// discoverP2P discovers bibd nodes using P2P bootstrap peers
func (d *Discoverer) discoverP2P(ctx context.Context) ([]DiscoveredNode, error) {
	timeout := d.opts.P2PTimeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	// Create a context with timeout
	p2pCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var nodes []DiscoveredNode
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Try to connect to bootstrap peers
	bootstrapPeers := []string{
		"bootstrap.bib.dev:4001",
	}

	for _, peer := range bootstrapPeers {
		wg.Add(1)
		go func(peerAddr string) {
			defer wg.Done()

			node, err := d.probeP2PPeer(p2pCtx, peerAddr)
			if err != nil {
				return
			}

			mu.Lock()
			nodes = append(nodes, *node)
			mu.Unlock()
		}(peer)
	}

	wg.Wait()

	return nodes, nil
}

// probeP2PPeer probes a P2P peer to check if it's a bibd node
func (d *Discoverer) probeP2PPeer(ctx context.Context, address string) (*DiscoveredNode, error) {
	// Try to resolve the address
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}

	// Resolve DNS if needed
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve %s: %w", host, err)
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("no IPs found for %s", host)
	}

	// Try to connect to the first IP
	resolvedAddress := fmt.Sprintf("%s:%s", ips[0].String(), port)

	// Measure latency if enabled
	var latency time.Duration
	if d.opts.MeasureLatency {
		if l, err := measureLatency(ctx, resolvedAddress, d.opts.LatencyTimeout); err == nil {
			latency = l
		} else {
			// If we can't connect, the peer isn't available
			return nil, fmt.Errorf("peer not reachable: %w", err)
		}
	}

	return &DiscoveredNode{
		Address:      address,
		Method:       MethodP2P,
		Latency:      latency,
		NodeInfo:     nil, // P2P info requires protocol exchange
		DiscoveredAt: time.Now(),
	}, nil
}

// DiscoverP2P is a convenience function for P2P discovery
func DiscoverP2P(ctx context.Context, config P2PDiscoveryConfig) ([]DiscoveredNode, error) {
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}

	d := New(DiscoveryOptions{
		EnableP2P:      true,
		P2PTimeout:     config.Timeout,
		MeasureLatency: config.MeasureLatency,
		LatencyTimeout: config.LatencyTimeout,
	})

	return d.discoverP2P(ctx)
}

// DiscoverFromBootstrapPeers discovers nodes from specific bootstrap peers
func DiscoverFromBootstrapPeers(ctx context.Context, peers []string, timeout time.Duration) ([]DiscoveredNode, error) {
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var nodes []DiscoveredNode
	var mu sync.Mutex
	var wg sync.WaitGroup

	d := New(DiscoveryOptions{
		MeasureLatency: true,
		LatencyTimeout: 2 * time.Second,
	})

	for _, peer := range peers {
		wg.Add(1)
		go func(peerAddr string) {
			defer wg.Done()

			node, err := d.probeP2PPeer(ctx, peerAddr)
			if err != nil {
				return
			}

			mu.Lock()
			nodes = append(nodes, *node)
			mu.Unlock()
		}(peer)
	}

	wg.Wait()

	// Sort by latency
	sortNodesByLatency(nodes)

	return nodes, nil
}

// P2PNodeInfo represents information about a P2P peer
type P2PNodeInfo struct {
	// PeerID is the libp2p peer ID
	PeerID string

	// Multiaddrs are the multiaddresses of the peer
	Multiaddrs []string

	// Protocols are the supported protocols
	Protocols []string

	// AgentVersion is the agent version string
	AgentVersion string
}

// ParseMultiaddr parses a multiaddr string and extracts address info
func ParseMultiaddr(multiaddr string) (host string, port int, err error) {
	// Simple parser for common multiaddr formats
	// /ip4/1.2.3.4/tcp/4001/p2p/QmHash
	// /dns4/hostname/tcp/4001/p2p/QmHash

	parts := strings.Split(multiaddr, "/")
	if len(parts) < 5 {
		return "", 0, fmt.Errorf("invalid multiaddr: %s", multiaddr)
	}

	for i := 0; i < len(parts)-1; i++ {
		switch parts[i] {
		case "ip4", "ip6", "dns4", "dns6":
			if i+1 < len(parts) {
				host = parts[i+1]
			}
		case "tcp", "udp":
			if i+1 < len(parts) {
				fmt.Sscanf(parts[i+1], "%d", &port)
			}
		}
	}

	if host == "" || port == 0 {
		return "", 0, fmt.Errorf("could not parse host/port from multiaddr: %s", multiaddr)
	}

	return host, port, nil
}

// MultiaddrsToAddresses converts multiaddrs to host:port addresses
func MultiaddrsToAddresses(multiaddrs []string) []string {
	var addresses []string
	seen := make(map[string]bool)

	for _, ma := range multiaddrs {
		host, port, err := ParseMultiaddr(ma)
		if err != nil {
			continue
		}

		addr := fmt.Sprintf("%s:%d", host, port)
		if !seen[addr] {
			seen[addr] = true
			addresses = append(addresses, addr)
		}
	}

	return addresses
}

// IsPublicBootstrapPeer checks if a peer is a known public bootstrap peer
func IsPublicBootstrapPeer(address string) bool {
	publicPeers := []string{
		"bootstrap.bib.dev",
		"bib.dev",
	}

	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}

	for _, peer := range publicPeers {
		if host == peer || strings.HasSuffix(host, "."+peer) {
			return true
		}
	}

	return false
}
