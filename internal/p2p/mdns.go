package p2p

import (
	"context"
	"sync"

	"bib/internal/config"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
)

// MDNSDiscovery handles local network peer discovery using mDNS.
type MDNSDiscovery struct {
	host    host.Host
	cfg     config.MDNSConfig
	service mdns.Service
	ctx     context.Context
	cancel  context.CancelFunc

	// mu protects discovered peers
	mu         sync.RWMutex
	discovered map[peer.ID]peer.AddrInfo

	// peerHandler is called when a new peer is discovered
	peerHandler func(peer.AddrInfo)
}

// NewMDNSDiscovery creates a new mDNS discovery service.
func NewMDNSDiscovery(h host.Host, cfg config.MDNSConfig) *MDNSDiscovery {
	ctx, cancel := context.WithCancel(context.Background())

	serviceName := cfg.ServiceName
	if serviceName == "" {
		serviceName = "bib.local"
	}

	return &MDNSDiscovery{
		host:       h,
		cfg:        config.MDNSConfig{Enabled: cfg.Enabled, ServiceName: serviceName},
		ctx:        ctx,
		cancel:     cancel,
		discovered: make(map[peer.ID]peer.AddrInfo),
	}
}

// Start begins mDNS discovery. It returns immediately.
func (m *MDNSDiscovery) Start() error {
	if !m.cfg.Enabled {
		return nil
	}

	service := mdns.NewMdnsService(m.host, m.cfg.ServiceName, m)
	if err := service.Start(); err != nil {
		return err
	}
	m.service = service

	return nil
}

// Stop stops the mDNS discovery service.
func (m *MDNSDiscovery) Stop() error {
	m.cancel()
	if m.service != nil {
		return m.service.Close()
	}
	return nil
}

// HandlePeerFound is called by the mDNS service when a peer is discovered.
// It implements the mdns.Notifee interface.
func (m *MDNSDiscovery) HandlePeerFound(pi peer.AddrInfo) {
	// Don't connect to ourselves
	if pi.ID == m.host.ID() {
		return
	}

	// Store the discovered peer
	m.mu.Lock()
	m.discovered[pi.ID] = pi
	m.mu.Unlock()

	// Call the peer handler if set
	if m.peerHandler != nil {
		m.peerHandler(pi)
	}

	// Attempt to connect to the discovered peer
	go func() {
		if err := m.host.Connect(m.ctx, pi); err != nil {
			// Connection failed, but we still have the peer info stored
			// for future connection attempts
		}
	}()
}

// SetPeerHandler sets a callback to be called when a new peer is discovered.
func (m *MDNSDiscovery) SetPeerHandler(handler func(peer.AddrInfo)) {
	m.peerHandler = handler
}

// DiscoveredPeers returns all peers discovered via mDNS.
func (m *MDNSDiscovery) DiscoveredPeers() []peer.AddrInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	peers := make([]peer.AddrInfo, 0, len(m.discovered))
	for _, pi := range m.discovered {
		peers = append(peers, pi)
	}
	return peers
}

// IsDiscovered returns true if the peer was discovered via mDNS.
func (m *MDNSDiscovery) IsDiscovered(id peer.ID) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.discovered[id]
	return ok
}
