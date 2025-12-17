package p2p

import (
	"context"
	"fmt"
	"sync"
	"time"

	"bib/internal/config"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// Discovery manages all peer discovery mechanisms.
type Discovery struct {
	host         host.Host
	cfg          config.P2PConfig
	configDir    string
	bootstrapper *Bootstrapper
	mdns         *MDNSDiscovery
	dht          *DHT
	peerStore    *PeerStore

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewDiscovery creates a new discovery manager.
func NewDiscovery(h host.Host, cfg config.P2PConfig, configDir string) (*Discovery, error) {
	discLog := getLogger("discovery")

	discLog.Debug("creating discovery manager",
		"mdns_enabled", cfg.MDNS.Enabled,
		"dht_enabled", cfg.DHT.Enabled,
		"bootstrap_peers", len(cfg.Bootstrap.Peers),
	)

	ctx, cancel := context.WithCancel(context.Background())

	d := &Discovery{
		host:      h,
		cfg:       cfg,
		configDir: configDir,
		ctx:       ctx,
		cancel:    cancel,
	}

	// Initialize peer store
	peerStore, err := NewPeerStore(cfg.PeerStore, configDir)
	if err != nil {
		cancel()
		discLog.Error("failed to create peer store", "error", err)
		return nil, fmt.Errorf("failed to create peer store: %w", err)
	}
	d.peerStore = peerStore
	discLog.Debug("peer store initialized")

	// Initialize bootstrapper
	bootstrapper, err := NewBootstrapper(h, cfg.Bootstrap)
	if err != nil {
		peerStore.Close()
		cancel()
		discLog.Error("failed to create bootstrapper", "error", err)
		return nil, fmt.Errorf("failed to create bootstrapper: %w", err)
	}
	d.bootstrapper = bootstrapper
	discLog.Debug("bootstrapper initialized", "peer_count", len(cfg.Bootstrap.Peers))

	// Initialize mDNS
	d.mdns = NewMDNSDiscovery(h, cfg.MDNS)

	// Setup connection notifier for peer store updates
	h.Network().Notify(&network.NotifyBundle{
		ConnectedF: func(n network.Network, conn network.Conn) {
			d.onPeerConnected(conn.RemotePeer())
		},
		DisconnectedF: func(n network.Network, conn network.Conn) {
			d.onPeerDisconnected(conn.RemotePeer())
		},
	})

	discLog.Info("discovery manager created")
	return d, nil
}

// Start begins all discovery mechanisms.
func (d *Discovery) Start(ctx context.Context) error {
	discLog := getLogger("discovery")

	discLog.Info("starting discovery mechanisms")

	// Start mDNS discovery
	if d.cfg.MDNS.Enabled {
		discLog.Debug("starting mDNS discovery", "service_name", d.cfg.MDNS.ServiceName)
		if err := d.mdns.Start(); err != nil {
			discLog.Error("failed to start mDNS", "error", err)
			return fmt.Errorf("failed to start mDNS: %w", err)
		}
		discLog.Info("mDNS discovery started")
	}

	// Start bootstrap connections
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		discLog.Debug("starting bootstrap connections")
		if err := d.bootstrapper.Start(ctx); err != nil {
			discLog.Warn("bootstrap connection error", "error", err)
			// Log error but don't fail - we might find peers through other means
		}
	}()

	// Initialize DHT after bootstrap connections are established
	if d.cfg.DHT.Enabled {
		discLog.Debug("waiting for bootstrap before DHT init")
		// Wait a bit for bootstrap connections
		time.Sleep(2 * time.Second)

		discLog.Debug("initializing DHT", "mode", d.cfg.DHT.Mode)
		bootstrapPeers := d.bootstrapper.peers
		dht, err := NewDHT(d.ctx, d.host, d.cfg.DHT, bootstrapPeers)
		if err != nil {
			discLog.Error("failed to create DHT", "error", err)
			return fmt.Errorf("failed to create DHT: %w", err)
		}
		d.dht = dht
		discLog.Info("DHT initialized", "mode", d.cfg.DHT.Mode)

		// Bootstrap the DHT
		discLog.Debug("bootstrapping DHT")
		if err := d.dht.Bootstrap(ctx); err != nil {
			discLog.Warn("DHT bootstrap error", "error", err)
			// Log but don't fail
		}
	}

	// Start periodic peer store maintenance
	d.wg.Add(1)
	go d.maintenanceLoop()

	discLog.Info("discovery started successfully")
	return nil
}

// Stop stops all discovery mechanisms.
func (d *Discovery) Stop() error {
	discLog := getLogger("discovery")
	discLog.Info("stopping discovery mechanisms")

	d.cancel()
	d.wg.Wait()

	var errs []error

	if d.bootstrapper != nil {
		discLog.Debug("stopping bootstrapper")
		d.bootstrapper.Stop()
	}

	if d.mdns != nil {
		discLog.Debug("stopping mDNS")
		if err := d.mdns.Stop(); err != nil {
			discLog.Error("error stopping mDNS", "error", err)
			errs = append(errs, err)
		}
	}

	if d.dht != nil {
		discLog.Debug("closing DHT")
		if err := d.dht.Close(); err != nil {
			discLog.Error("error closing DHT", "error", err)
			errs = append(errs, err)
		}
	}

	if d.peerStore != nil {
		discLog.Debug("closing peer store")
		if err := d.peerStore.Close(); err != nil {
			discLog.Error("error closing peer store", "error", err)
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		discLog.Warn("discovery stopped with errors", "error_count", len(errs))
		return fmt.Errorf("errors stopping discovery: %v", errs)
	}

	discLog.Info("discovery stopped successfully")
	return nil
}

// onPeerConnected is called when a peer connects.
func (d *Discovery) onPeerConnected(id peer.ID) {
	discLog := getLogger("discovery")
	isBootstrap := d.bootstrapper != nil && d.bootstrapper.IsBootstrapPeer(id)

	discLog.Debug("peer connected",
		"peer_id", id.String(),
		"is_bootstrap", isBootstrap,
	)

	addrs := d.host.Peerstore().Addrs(id)
	info := peer.AddrInfo{ID: id, Addrs: addrs}

	// Add to peer store
	if err := d.peerStore.AddPeer(info, isBootstrap); err != nil {
		discLog.Warn("failed to add peer to store", "peer_id", id.String(), "error", err)
	}

	// Record successful connection
	// Note: We don't have latency info here, would need to measure separately
	if err := d.peerStore.RecordConnection(id, true, 0); err != nil {
		discLog.Warn("failed to record connection", "peer_id", id.String(), "error", err)
	}
}

// onPeerDisconnected is called when a peer disconnects.
func (d *Discovery) onPeerDisconnected(id peer.ID) {
	discLog := getLogger("discovery")
	discLog.Debug("peer disconnected", "peer_id", id.String())
	// Could update peer store here if needed
}

// maintenanceLoop periodically maintains the peer store.
func (d *Discovery) maintenanceLoop() {
	defer d.wg.Done()
	discLog := getLogger("discovery")

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			discLog.Debug("running peer store maintenance")
			// Prune peers not seen in 30 days
			pruned, err := d.peerStore.PruneOldPeers(30 * 24 * time.Hour)
			if err != nil {
				discLog.Warn("failed to prune old peers", "error", err)
			} else if pruned > 0 {
				discLog.Info("pruned old peers", "count", pruned)
			}
		}
	}
}

// FindPeer attempts to find a peer using available discovery mechanisms.
func (d *Discovery) FindPeer(ctx context.Context, id peer.ID) (peer.AddrInfo, error) {
	// Check peer store first
	info, _, err := d.peerStore.GetPeer(id)
	if err == nil && info != nil && len(info.Addrs) > 0 {
		return *info, nil
	}

	// Try DHT
	if d.dht != nil {
		return d.dht.FindPeer(ctx, id)
	}

	return peer.AddrInfo{}, fmt.Errorf("peer not found: %s", id)
}

// ConnectedPeers returns the number of connected peers.
func (d *Discovery) ConnectedPeers() int {
	return len(d.host.Network().Peers())
}

// PeerCount returns the number of known peers in the store.
func (d *Discovery) PeerCount() (int, error) {
	return d.peerStore.Count()
}

// DHT returns the DHT instance, if enabled.
func (d *Discovery) DHT() *DHT {
	return d.dht
}

// PeerStore returns the peer store.
func (d *Discovery) PeerStore() *PeerStore {
	return d.peerStore
}

// GetPeersForPruning returns peers to disconnect when at high watermark.
// Priority: Keep bootstrap peers, then best scored, then most recently active.
func (d *Discovery) GetPeersForPruning(count int) []peer.ID {
	connected := d.host.Network().Peers()
	if len(connected) == 0 {
		return nil
	}

	type scoredPeer struct {
		id    peer.ID
		score float64
	}

	var scored []scoredPeer
	for _, id := range connected {
		// Skip bootstrap peers
		if d.bootstrapper != nil && d.bootstrapper.IsBootstrapPeer(id) {
			continue
		}

		_, peerScore, err := d.peerStore.GetPeer(id)
		if err != nil || peerScore == nil {
			// Unknown peer, give low score
			scored = append(scored, scoredPeer{id: id, score: 0})
		} else {
			scored = append(scored, scoredPeer{id: id, score: peerScore.Score()})
		}
	}

	// Sort by score (ascending - worst first)
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[i].score > scored[j].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Return the worst peers to prune
	result := make([]peer.ID, 0, count)
	for i := 0; i < len(scored) && i < count; i++ {
		result = append(result, scored[i].id)
	}

	return result
}
