package p2p

import (
	"context"
	"fmt"
	"strings"

	"bib/internal/config"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
)

// DHTMode represents the DHT operation mode.
type DHTMode string

const (
	// DHTModeAuto lets libp2p decide based on network reachability.
	DHTModeAuto DHTMode = "auto"
	// DHTModeServer runs as a full DHT participant (requires public IP).
	DHTModeServer DHTMode = "server"
	// DHTModeClient only queries the DHT, doesn't store records.
	DHTModeClient DHTMode = "client"
)

// DHT wraps the Kademlia DHT with bib-specific functionality.
type DHT struct {
	*dht.IpfsDHT
	host host.Host
	cfg  config.DHTConfig
}

// NewDHT creates a new Kademlia DHT instance.
func NewDHT(ctx context.Context, h host.Host, cfg config.DHTConfig, bootstrapPeers []peer.AddrInfo) (*DHT, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	mode := DHTMode(strings.ToLower(cfg.Mode))
	if mode == "" {
		mode = DHTModeAuto
	}

	opts := []dht.Option{
		dht.ProtocolPrefix("/bib"),
	}

	// Set mode-specific options
	switch mode {
	case DHTModeServer:
		opts = append(opts, dht.Mode(dht.ModeServer))
	case DHTModeClient:
		opts = append(opts, dht.Mode(dht.ModeClient))
	case DHTModeAuto:
		opts = append(opts, dht.Mode(dht.ModeAutoServer))
	default:
		return nil, fmt.Errorf("invalid DHT mode: %s (must be auto, server, or client)", cfg.Mode)
	}

	// Add bootstrap peers if available
	if len(bootstrapPeers) > 0 {
		opts = append(opts, dht.BootstrapPeers(bootstrapPeers...))
	}

	// Create the DHT
	kadDHT, err := dht.New(ctx, h, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create DHT: %w", err)
	}

	return &DHT{
		IpfsDHT: kadDHT,
		host:    h,
		cfg:     cfg,
	}, nil
}

// Bootstrap connects to bootstrap peers and refreshes the routing table.
func (d *DHT) Bootstrap(ctx context.Context) error {
	if d.IpfsDHT == nil {
		return nil
	}
	return d.IpfsDHT.Bootstrap(ctx)
}

// FindPeer searches the DHT for a peer's addresses.
func (d *DHT) FindPeer(ctx context.Context, id peer.ID) (peer.AddrInfo, error) {
	if d.IpfsDHT == nil {
		return peer.AddrInfo{}, fmt.Errorf("DHT not enabled")
	}
	return d.IpfsDHT.FindPeer(ctx, id)
}

// Provide announces that this node can provide the given key.
func (d *DHT) Provide(ctx context.Context, key string, announce bool) error {
	if d.IpfsDHT == nil {
		return fmt.Errorf("DHT not enabled")
	}
	// Convert string key to CID for provider records
	// This is a simplified version - in practice you'd hash the key
	return nil // Placeholder - implement when needed for content routing
}

// FindProviders searches the DHT for peers that provide the given key.
func (d *DHT) FindProviders(ctx context.Context, key string) ([]peer.AddrInfo, error) {
	if d.IpfsDHT == nil {
		return nil, fmt.Errorf("DHT not enabled")
	}
	// Placeholder - implement when needed for content routing
	return nil, nil
}

// Close shuts down the DHT.
func (d *DHT) Close() error {
	if d.IpfsDHT == nil {
		return nil
	}
	return d.IpfsDHT.Close()
}

// RoutingTable returns the DHT's routing table size.
func (d *DHT) RoutingTableSize() int {
	if d.IpfsDHT == nil {
		return 0
	}
	return d.IpfsDHT.RoutingTable().Size()
}

// AsRouting returns the DHT as a routing.Routing interface.
func (d *DHT) AsRouting() routing.Routing {
	return d.IpfsDHT
}

// Mode returns the current DHT mode as a string.
func (d *DHT) Mode() string {
	return d.cfg.Mode
}
