// Package p2p provides peer-to-peer networking functionality for bib.
package p2p

import (
	"context"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

// NodeManager provides a high-level interface for P2P node operations.
// This abstraction allows gRPC services to interact with the P2P layer
// without direct dependency on libp2p internals.
type NodeManager interface {
	// Self returns information about this node.
	GetSelfInfo() (*NodeManagerInfo, error)

	// GetPeerInfo returns information about a specific peer.
	GetPeerInfo(peerID peer.ID) (*NodeManagerInfo, error)

	// ListConnectedPeers returns all currently connected peers.
	ListConnectedPeers() ([]*NodeManagerInfo, error)

	// ListKnownPeers returns all known peers (connected or not).
	ListKnownPeers() ([]*NodeManagerInfo, error)

	// Connect connects to a peer by multiaddr.
	Connect(ctx context.Context, addr multiaddr.Multiaddr) (*NodeManagerInfo, error)

	// Disconnect disconnects from a peer.
	Disconnect(peerID peer.ID) error

	// GetNetworkStats returns network statistics.
	GetNetworkStats() (*NetworkStats, error)

	// SubscribeNodeEvents subscribes to node join/leave events.
	// The returned channel will receive events until the context is cancelled.
	SubscribeNodeEvents(ctx context.Context, bufferSize int) (<-chan NodeEvent, error)

	// BanPeer bans a peer. Duration of 0 means permanent.
	BanPeer(peerID peer.ID, reason string, duration time.Duration) error

	// UnbanPeer removes a ban from a peer.
	UnbanPeer(peerID peer.ID) error

	// IsBanned checks if a peer is currently banned.
	IsBanned(peerID peer.ID) bool

	// ListBannedPeers returns all banned peers.
	ListBannedPeers() ([]*BannedPeerInfo, error)

	// GRPCClient returns the P2P gRPC client for calling remote peers.
	// Returns nil if gRPC-over-P2P is not enabled.
	GRPCClient() *GRPCClient
}

// NodeManagerInfo contains information about a node.
type NodeManagerInfo struct {
	// PeerID is the libp2p peer ID.
	PeerID peer.ID

	// Addresses are the multiaddrs for this peer.
	Addresses []multiaddr.Multiaddr

	// Mode is the node's operation mode (full, selective, proxy).
	Mode string

	// Version is the node's software version.
	Version string

	// AgentVersion is the libp2p agent version string.
	AgentVersion string

	// Protocols are the protocols supported by this node.
	Protocols []string

	// Connected indicates if we're currently connected.
	Connected bool

	// LatencyMs is the connection latency in milliseconds.
	LatencyMs int64

	// IsBootstrap indicates if this is a bootstrap node.
	IsBootstrap bool

	// StorageType is the storage backend (postgres, sqlite).
	StorageType string

	// IsAuthoritative indicates if this node can be a trusted data source.
	IsAuthoritative bool

	// DatasetCount is the number of datasets this node has.
	DatasetCount int64

	// Reputation is the peer reputation score (0-100).
	Reputation int32

	// DiscoveredAt is when we first discovered this node.
	DiscoveredAt time.Time

	// LastSeen is when we last saw this node.
	LastSeen time.Time

	// Metadata holds additional node information.
	Metadata map[string]string
}

// NetworkStats contains network statistics.
type NetworkStats struct {
	// ConnectedPeers is the number of connected peers.
	ConnectedPeers int32

	// KnownPeers is the number of known peers.
	KnownPeers int32

	// TotalBytesSent is the total bytes sent.
	TotalBytesSent int64

	// TotalBytesReceived is the total bytes received.
	TotalBytesReceived int64

	// ActiveStreams is the number of active streams.
	ActiveStreams int32

	// DHTSize is the DHT routing table size.
	DHTSize int32

	// BootstrapConnected indicates if we're connected to bootstrap nodes.
	BootstrapConnected bool

	// ProtocolStats contains per-protocol statistics.
	ProtocolStats map[string]*ProtocolStats

	// Bandwidth contains bandwidth information.
	Bandwidth *BandwidthStats
}

// ProtocolStats contains per-protocol statistics.
type ProtocolStats struct {
	// ProtocolID is the protocol identifier.
	ProtocolID string

	// StreamCount is the number of active streams.
	StreamCount int32

	// MessagesSent is the number of messages sent.
	MessagesSent int64

	// MessagesReceived is the number of messages received.
	MessagesReceived int64

	// BytesSent is the bytes sent for this protocol.
	BytesSent int64

	// BytesReceived is the bytes received for this protocol.
	BytesReceived int64
}

// BandwidthStats contains bandwidth information.
type BandwidthStats struct {
	// RateIn is the current inbound rate (bytes/sec).
	RateIn float64

	// RateOut is the current outbound rate (bytes/sec).
	RateOut float64

	// TotalIn is the total bytes received.
	TotalIn int64

	// TotalOut is the total bytes sent.
	TotalOut int64
}

// NodeEventType represents the type of node event.
type NodeEventType string

const (
	NodeEventTypeJoin   NodeEventType = "join"
	NodeEventTypeLeave  NodeEventType = "leave"
	NodeEventTypeUpdate NodeEventType = "update"
)

// NodeEvent represents a node event.
type NodeEvent struct {
	// Type is the event type.
	Type NodeEventType

	// Node is the node information.
	Node *NodeManagerInfo

	// Timestamp is when the event occurred.
	Timestamp time.Time

	// Details contains additional event details.
	Details map[string]string
}

// BannedPeerInfo contains information about a banned peer.
type BannedPeerInfo struct {
	// PeerID is the banned peer's ID.
	PeerID peer.ID

	// Reason is the reason for the ban.
	Reason string

	// BannedAt is when the peer was banned.
	BannedAt time.Time

	// ExpiresAt is when the ban expires (nil = permanent).
	ExpiresAt *time.Time
}

// =============================================================================
// Default Node Manager Implementation
// =============================================================================

// DefaultNodeManagerConfig holds configuration for the default node manager.
type DefaultNodeManagerConfig struct {
	// StatsCacheInterval is how often to refresh cached stats.
	StatsCacheInterval time.Duration

	// EventBufferSize is the default buffer size for event channels.
	EventBufferSize int
}

// DefaultNodeManagerConfigDefaults returns default configuration.
func DefaultNodeManagerConfigDefaults() DefaultNodeManagerConfig {
	return DefaultNodeManagerConfig{
		StatsCacheInterval: 30 * time.Second,
		EventBufferSize:    100,
	}
}

// defaultNodeManager implements NodeManager using a Host.
type defaultNodeManager struct {
	host *Host
	cfg  DefaultNodeManagerConfig

	// gRPC client for P2P calls
	grpcClient *GRPCClient

	// Cached stats
	statsMu        sync.RWMutex
	cachedStats    *NetworkStats
	statsUpdatedAt time.Time

	// Event subscribers
	eventMu     sync.RWMutex
	subscribers map[chan<- NodeEvent]struct{}

	// In-memory ban list (supplemented by database)
	banMu   sync.RWMutex
	banList map[peer.ID]*BannedPeerInfo
}

// NewNodeManager creates a new NodeManager wrapping a Host.
func NewNodeManager(host *Host, cfg DefaultNodeManagerConfig) NodeManager {
	nm := &defaultNodeManager{
		host:        host,
		cfg:         cfg,
		subscribers: make(map[chan<- NodeEvent]struct{}),
		banList:     make(map[peer.ID]*BannedPeerInfo),
	}

	// Start background stats refresh
	go nm.statsRefreshLoop()

	return nm
}

// GetSelfInfo returns information about this node.
func (nm *defaultNodeManager) GetSelfInfo() (*NodeManagerInfo, error) {
	h := nm.host.Host

	addrs := h.Addrs()
	multiaddrs := make([]multiaddr.Multiaddr, len(addrs))
	copy(multiaddrs, addrs)

	protocols := h.Mux().Protocols()
	protoStrings := make([]string, len(protocols))
	for i, p := range protocols {
		protoStrings[i] = string(p)
	}

	return &NodeManagerInfo{
		PeerID:       h.ID(),
		Addresses:    multiaddrs,
		Protocols:    protoStrings,
		Connected:    true,
		AgentVersion: "bib/1.0.0", // TODO: Get from version package
		Metadata:     make(map[string]string),
	}, nil
}

// GetPeerInfo returns information about a specific peer.
func (nm *defaultNodeManager) GetPeerInfo(peerID peer.ID) (*NodeManagerInfo, error) {
	h := nm.host.Host
	ps := h.Peerstore()

	addrs := ps.Addrs(peerID)
	multiaddrs := make([]multiaddr.Multiaddr, len(addrs))
	copy(multiaddrs, addrs)

	protocols, _ := ps.GetProtocols(peerID)
	protoStrings := make([]string, len(protocols))
	for i, p := range protocols {
		protoStrings[i] = string(p)
	}

	connected := h.Network().Connectedness(peerID) == 2 // Connected

	var latencyMs int64
	if latency := ps.LatencyEWMA(peerID); latency > 0 {
		latencyMs = latency.Milliseconds()
	}

	agentVersion, _ := ps.Get(peerID, "AgentVersion")
	agentVersionStr, _ := agentVersion.(string)

	return &NodeManagerInfo{
		PeerID:       peerID,
		Addresses:    multiaddrs,
		Protocols:    protoStrings,
		Connected:    connected,
		LatencyMs:    latencyMs,
		AgentVersion: agentVersionStr,
		Metadata:     make(map[string]string),
	}, nil
}

// ListConnectedPeers returns all currently connected peers.
func (nm *defaultNodeManager) ListConnectedPeers() ([]*NodeManagerInfo, error) {
	h := nm.host.Host
	peers := h.Network().Peers()

	result := make([]*NodeManagerInfo, 0, len(peers))
	for _, p := range peers {
		info, err := nm.GetPeerInfo(p)
		if err != nil {
			continue
		}
		if info.Connected {
			result = append(result, info)
		}
	}

	return result, nil
}

// ListKnownPeers returns all known peers.
func (nm *defaultNodeManager) ListKnownPeers() ([]*NodeManagerInfo, error) {
	h := nm.host.Host
	ps := h.Peerstore()
	peers := ps.Peers()

	result := make([]*NodeManagerInfo, 0, len(peers))
	for _, p := range peers {
		if p == h.ID() {
			continue // Skip self
		}
		info, err := nm.GetPeerInfo(p)
		if err != nil {
			continue
		}
		result = append(result, info)
	}

	return result, nil
}

// Connect connects to a peer by multiaddr.
func (nm *defaultNodeManager) Connect(ctx context.Context, addr multiaddr.Multiaddr) (*NodeManagerInfo, error) {
	h := nm.host.Host

	// Extract peer ID from multiaddr
	peerInfo, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		return nil, err
	}

	// Check if banned
	if nm.IsBanned(peerInfo.ID) {
		return nil, ErrPeerBanned
	}

	// Connect
	if err := h.Connect(ctx, *peerInfo); err != nil {
		return nil, err
	}

	// Emit event
	nm.emitEvent(NodeEvent{
		Type:      NodeEventTypeJoin,
		Timestamp: time.Now(),
		Details: map[string]string{
			"peer_id": peerInfo.ID.String(),
			"addr":    addr.String(),
		},
	})

	return nm.GetPeerInfo(peerInfo.ID)
}

// Disconnect disconnects from a peer.
func (nm *defaultNodeManager) Disconnect(peerID peer.ID) error {
	h := nm.host.Host

	if err := h.Network().ClosePeer(peerID); err != nil {
		return err
	}

	// Emit event
	nm.emitEvent(NodeEvent{
		Type:      NodeEventTypeLeave,
		Timestamp: time.Now(),
		Details: map[string]string{
			"peer_id": peerID.String(),
			"reason":  "manual_disconnect",
		},
	})

	return nil
}

// GetNetworkStats returns network statistics.
func (nm *defaultNodeManager) GetNetworkStats() (*NetworkStats, error) {
	nm.statsMu.RLock()
	if nm.cachedStats != nil && time.Since(nm.statsUpdatedAt) < nm.cfg.StatsCacheInterval {
		stats := nm.cachedStats
		nm.statsMu.RUnlock()
		return stats, nil
	}
	nm.statsMu.RUnlock()

	// Refresh stats
	return nm.refreshStats()
}

// refreshStats updates the cached network stats.
func (nm *defaultNodeManager) refreshStats() (*NetworkStats, error) {
	h := nm.host.Host

	connectedPeers := len(h.Network().Peers())
	knownPeers := len(h.Peerstore().Peers())

	stats := &NetworkStats{
		ConnectedPeers: int32(connectedPeers),
		KnownPeers:     int32(knownPeers),
		ActiveStreams:  0, // TODO: Count active streams
		ProtocolStats:  make(map[string]*ProtocolStats),
	}

	// Get bandwidth stats if available
	if nm.host.bandwidthCounter != nil {
		bwStats := nm.host.bandwidthCounter.GetBandwidthTotals()
		stats.TotalBytesSent = bwStats.TotalOut
		stats.TotalBytesReceived = bwStats.TotalIn
		stats.Bandwidth = &BandwidthStats{
			RateIn:   bwStats.RateIn,
			RateOut:  bwStats.RateOut,
			TotalIn:  bwStats.TotalIn,
			TotalOut: bwStats.TotalOut,
		}
	}

	nm.statsMu.Lock()
	nm.cachedStats = stats
	nm.statsUpdatedAt = time.Now()
	nm.statsMu.Unlock()

	return stats, nil
}

// statsRefreshLoop periodically refreshes network stats.
func (nm *defaultNodeManager) statsRefreshLoop() {
	ticker := time.NewTicker(nm.cfg.StatsCacheInterval)
	defer ticker.Stop()

	for range ticker.C {
		nm.refreshStats()
	}
}

// SubscribeNodeEvents subscribes to node events.
func (nm *defaultNodeManager) SubscribeNodeEvents(ctx context.Context, bufferSize int) (<-chan NodeEvent, error) {
	if bufferSize <= 0 {
		bufferSize = nm.cfg.EventBufferSize
	}

	ch := make(chan NodeEvent, bufferSize)

	nm.eventMu.Lock()
	nm.subscribers[ch] = struct{}{}
	nm.eventMu.Unlock()

	// Clean up when context is done
	go func() {
		<-ctx.Done()
		nm.eventMu.Lock()
		delete(nm.subscribers, ch)
		close(ch)
		nm.eventMu.Unlock()
	}()

	return ch, nil
}

// emitEvent sends an event to all subscribers.
func (nm *defaultNodeManager) emitEvent(event NodeEvent) {
	nm.eventMu.RLock()
	defer nm.eventMu.RUnlock()

	for ch := range nm.subscribers {
		select {
		case ch <- event:
		default:
			// Channel full, skip
		}
	}
}

// BanPeer bans a peer.
func (nm *defaultNodeManager) BanPeer(peerID peer.ID, reason string, duration time.Duration) error {
	nm.banMu.Lock()
	defer nm.banMu.Unlock()

	ban := &BannedPeerInfo{
		PeerID:   peerID,
		Reason:   reason,
		BannedAt: time.Now(),
	}

	if duration > 0 {
		expiresAt := time.Now().Add(duration)
		ban.ExpiresAt = &expiresAt
	}

	nm.banList[peerID] = ban

	// Disconnect immediately
	nm.host.Host.Network().ClosePeer(peerID)

	return nil
}

// UnbanPeer removes a ban.
func (nm *defaultNodeManager) UnbanPeer(peerID peer.ID) error {
	nm.banMu.Lock()
	defer nm.banMu.Unlock()

	delete(nm.banList, peerID)
	return nil
}

// IsBanned checks if a peer is banned.
func (nm *defaultNodeManager) IsBanned(peerID peer.ID) bool {
	nm.banMu.RLock()
	defer nm.banMu.RUnlock()

	ban, exists := nm.banList[peerID]
	if !exists {
		return false
	}

	// Check if expired
	if ban.ExpiresAt != nil && time.Now().After(*ban.ExpiresAt) {
		return false
	}

	return true
}

// ListBannedPeers returns all banned peers.
func (nm *defaultNodeManager) ListBannedPeers() ([]*BannedPeerInfo, error) {
	nm.banMu.RLock()
	defer nm.banMu.RUnlock()

	result := make([]*BannedPeerInfo, 0, len(nm.banList))
	for _, ban := range nm.banList {
		// Skip expired bans
		if ban.ExpiresAt != nil && time.Now().After(*ban.ExpiresAt) {
			continue
		}
		result = append(result, ban)
	}

	return result, nil
}

// GRPCClient returns the P2P gRPC client.
func (nm *defaultNodeManager) GRPCClient() *GRPCClient {
	return nm.grpcClient
}

// SetGRPCClient sets the P2P gRPC client.
// This should be called during initialization.
func (nm *defaultNodeManager) SetGRPCClient(client *GRPCClient) {
	nm.grpcClient = client
}

// ErrPeerBanned is returned when trying to connect to a banned peer.
var ErrPeerBanned = &PeerBannedError{}

// PeerBannedError indicates a peer is banned.
type PeerBannedError struct{}

func (e *PeerBannedError) Error() string {
	return "peer is banned"
}
