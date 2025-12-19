// Package p2p provides peer-to-peer networking functionality for bib.
package p2p

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"bib/internal/logger"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCClientConfig holds configuration for the P2P gRPC client.
type GRPCClientConfig struct {
	// Host is the libp2p host.
	Host host.Host

	// DialTimeout is the timeout for establishing connections.
	DialTimeout time.Duration

	// MaxConnsPerPeer is the maximum number of gRPC connections per peer.
	MaxConnsPerPeer int

	// IdleTimeout is how long idle connections are kept before closing.
	IdleTimeout time.Duration

	// TCPFallbackEnabled enables fallback to direct TCP when P2P fails.
	TCPFallbackEnabled bool

	// TCPFallbackTimeout is the timeout for TCP fallback connections.
	TCPFallbackTimeout time.Duration

	// DialOptions are additional gRPC dial options.
	DialOptions []grpc.DialOption
}

// DefaultGRPCClientConfig returns the default client configuration.
func DefaultGRPCClientConfig() GRPCClientConfig {
	return GRPCClientConfig{
		DialTimeout:        30 * time.Second,
		MaxConnsPerPeer:    2,
		IdleTimeout:        5 * time.Minute,
		TCPFallbackEnabled: false,
		TCPFallbackTimeout: 10 * time.Second,
	}
}

// GRPCClient provides gRPC client connections over P2P.
type GRPCClient struct {
	host   host.Host
	cfg    GRPCClientConfig
	dialer *p2pDialer
	log    *logger.Logger

	// Connection pool
	mu       sync.RWMutex
	connPool map[peer.ID]*pooledConn
}

// pooledConn holds a pooled gRPC connection.
type pooledConn struct {
	conn        *grpc.ClientConn
	lastUsed    time.Time
	mu          sync.Mutex
	useFallback bool   // true if using TCP fallback
	tcpAddr     string // TCP address if using fallback
}

// NewGRPCClient creates a new P2P gRPC client.
func NewGRPCClient(cfg GRPCClientConfig) (*GRPCClient, error) {
	if cfg.Host == nil {
		return nil, fmt.Errorf("host is required")
	}

	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = DefaultGRPCClientConfig().DialTimeout
	}
	if cfg.MaxConnsPerPeer == 0 {
		cfg.MaxConnsPerPeer = DefaultGRPCClientConfig().MaxConnsPerPeer
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = DefaultGRPCClientConfig().IdleTimeout
	}

	client := &GRPCClient{
		host:     cfg.Host,
		cfg:      cfg,
		dialer:   newP2PDialer(cfg.Host, cfg.DialTimeout),
		log:      getLogger("grpc_client"),
		connPool: make(map[peer.ID]*pooledConn),
	}

	// Start cleanup goroutine
	go client.cleanupLoop()

	return client, nil
}

// GetConnection returns a gRPC client connection to the specified peer.
// The connection is pooled and reused for subsequent calls.
func (c *GRPCClient) GetConnection(ctx context.Context, peerID peer.ID) (*grpc.ClientConn, error) {
	// Check pool first
	c.mu.RLock()
	pc, ok := c.connPool[peerID]
	c.mu.RUnlock()

	if ok {
		pc.mu.Lock()
		pc.lastUsed = time.Now()
		conn := pc.conn
		pc.mu.Unlock()

		// Verify connection is still valid
		if conn != nil && conn.GetState().String() != "SHUTDOWN" {
			return conn, nil
		}

		// Connection is dead, remove from pool
		c.mu.Lock()
		delete(c.connPool, peerID)
		c.mu.Unlock()
	}

	// Create new connection
	conn, err := c.dial(ctx, peerID)
	if err != nil {
		return nil, err
	}

	// Add to pool
	c.mu.Lock()
	c.connPool[peerID] = &pooledConn{
		conn:     conn,
		lastUsed: time.Now(),
	}
	c.mu.Unlock()

	return conn, nil
}

// GetConnectionByAddr returns a gRPC client connection to a peer identified by peer ID string.
func (c *GRPCClient) GetConnectionByAddr(ctx context.Context, peerIDStr string) (*grpc.ClientConn, error) {
	peerID, err := peer.Decode(peerIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid peer ID: %w", err)
	}
	return c.GetConnection(ctx, peerID)
}

// GetConnectionWithFallback returns a gRPC client connection, with TCP fallback.
// If P2P connection fails and tcpAddr is provided, it will try to connect via TCP.
func (c *GRPCClient) GetConnectionWithFallback(ctx context.Context, peerID peer.ID, tcpAddr string) (*grpc.ClientConn, error) {
	if !c.cfg.TCPFallbackEnabled || tcpAddr == "" {
		return c.GetConnection(ctx, peerID)
	}

	// Try P2P first
	conn, err := c.GetConnection(ctx, peerID)
	if err == nil {
		return conn, nil
	}

	c.log.Debug("P2P connection failed, trying TCP fallback",
		"peer_id", peerID.String(),
		"tcp_addr", tcpAddr,
		"error", err,
	)

	// Fall back to TCP
	return c.dialTCP(ctx, peerID, tcpAddr)
}

// dial creates a new gRPC connection over P2P.
func (c *GRPCClient) dial(ctx context.Context, peerID peer.ID) (*grpc.ClientConn, error) {
	// Create dial options
	opts := append([]grpc.DialOption{}, c.cfg.DialOptions...)

	// Use our custom P2P dialer
	opts = append(opts,
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return c.dialer.DialContext(ctx, peerID)
		}),
		// We use insecure credentials because libp2p's Noise protocol handles encryption
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	// The target address is just the peer ID since we use a custom dialer
	// Use passthrough scheme to bypass resolver
	target := "passthrough:///" + peerID.String()

	c.log.Debug("dialing peer over P2P", "peer_id", peerID.String())

	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial peer %s: %w", peerID, err)
	}

	return conn, nil
}

// dialTCP creates a gRPC connection over TCP (fallback).
func (c *GRPCClient) dialTCP(ctx context.Context, peerID peer.ID, tcpAddr string) (*grpc.ClientConn, error) {
	// Check if we already have a TCP fallback connection
	c.mu.RLock()
	pc, ok := c.connPool[peerID]
	c.mu.RUnlock()

	if ok && pc.useFallback {
		pc.mu.Lock()
		pc.lastUsed = time.Now()
		conn := pc.conn
		pc.mu.Unlock()

		if conn != nil && conn.GetState().String() != "SHUTDOWN" {
			return conn, nil
		}
	}

	// Create dial options for TCP
	opts := append([]grpc.DialOption{}, c.cfg.DialOptions...)
	opts = append(opts,
		// For TCP fallback, you may want to use TLS - this depends on your setup
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	c.log.Debug("dialing peer over TCP fallback", "peer_id", peerID.String(), "addr", tcpAddr)

	// Note: We still want a timeout for the connection, but NewClient is lazy
	// The timeout will apply to the actual connection attempt
	_ = ctx // ctx is available for future use with connection timeout

	conn, err := grpc.NewClient(tcpAddr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial peer %s via TCP: %w", peerID, err)
	}

	// Add to pool with fallback flag
	c.mu.Lock()
	c.connPool[peerID] = &pooledConn{
		conn:        conn,
		lastUsed:    time.Now(),
		useFallback: true,
		tcpAddr:     tcpAddr,
	}
	c.mu.Unlock()

	return conn, nil
}

// DiscoverPeer attempts to discover a peer via DHT and returns their addresses.
func (c *GRPCClient) DiscoverPeer(ctx context.Context, peerID peer.ID) ([]multiaddr.Multiaddr, error) {
	// Check if already connected
	if c.host.Network().Connectedness(peerID) == network.Connected {
		return c.host.Peerstore().Addrs(peerID), nil
	}

	// Try to find the peer via DHT (if DHT is available)
	// This requires access to the DHT instance
	peerInfo, err := c.findPeer(ctx, peerID)
	if err != nil {
		return nil, fmt.Errorf("failed to discover peer: %w", err)
	}

	// Connect to the peer
	if err := c.host.Connect(ctx, peerInfo); err != nil {
		return nil, fmt.Errorf("failed to connect to peer: %w", err)
	}

	return peerInfo.Addrs, nil
}

// findPeer attempts to find a peer via the host's peer store.
// In a real implementation, this would use the DHT.
func (c *GRPCClient) findPeer(ctx context.Context, peerID peer.ID) (peer.AddrInfo, error) {
	addrs := c.host.Peerstore().Addrs(peerID)
	if len(addrs) > 0 {
		return peer.AddrInfo{
			ID:    peerID,
			Addrs: addrs,
		}, nil
	}

	// Peer not found in peerstore
	return peer.AddrInfo{}, fmt.Errorf("peer not found: %s", peerID)
}

// cleanupLoop periodically cleans up idle connections.
func (c *GRPCClient) cleanupLoop() {
	ticker := time.NewTicker(c.cfg.IdleTimeout / 2)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

// cleanup removes idle connections.
func (c *GRPCClient) cleanup() {
	threshold := time.Now().Add(-c.cfg.IdleTimeout)

	c.mu.Lock()
	defer c.mu.Unlock()

	for peerID, pc := range c.connPool {
		pc.mu.Lock()
		if pc.lastUsed.Before(threshold) {
			c.log.Debug("closing idle connection", "peer_id", peerID.String())
			if pc.conn != nil {
				pc.conn.Close()
			}
			delete(c.connPool, peerID)
		}
		pc.mu.Unlock()
	}
}

// Close closes all pooled connections and stops the client.
func (c *GRPCClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for peerID, pc := range c.connPool {
		if pc.conn != nil {
			pc.conn.Close()
		}
		delete(c.connPool, peerID)
	}

	return nil
}

// ConnectedPeers returns a list of peers with active gRPC connections.
func (c *GRPCClient) ConnectedPeers() []peer.ID {
	c.mu.RLock()
	defer c.mu.RUnlock()

	peers := make([]peer.ID, 0, len(c.connPool))
	for peerID := range c.connPool {
		peers = append(peers, peerID)
	}
	return peers
}

// Stats returns statistics about the client connections.
type GRPCClientStats struct {
	// TotalConnections is the number of active connections.
	TotalConnections int

	// P2PConnections is the number of P2P connections.
	P2PConnections int

	// TCPFallbackConnections is the number of TCP fallback connections.
	TCPFallbackConnections int
}

// Stats returns current client statistics.
func (c *GRPCClient) Stats() GRPCClientStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := GRPCClientStats{
		TotalConnections: len(c.connPool),
	}

	for _, pc := range c.connPool {
		if pc.useFallback {
			stats.TCPFallbackConnections++
		} else {
			stats.P2PConnections++
		}
	}

	return stats
}
