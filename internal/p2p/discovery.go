package p2p

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	lpdisc "github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	mdns "github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	rd "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	discutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
)

type DiscoveryConfig struct {
	// Rendezvous should be a string like "/bib/<cluster>".
	Rendezvous string

	// mDNS for LAN peer discovery. ServiceTag can be empty to default to "bib-mdns".
	EnableMDNS     bool
	MDNSServiceTag string

	// DHT mode: if true, participate as a server (provide/routing table maintenance).
	// If false, run as a lightweight client.
	DHTServer bool

	// Bootstrap peer multi address (e.g., /dns4/host/tcp/4001/p2p/12D3K...).
	BootstrapPeers []string

	// Optional callback when a peer is discovered. Source: "dht" or "mdns".
	OnPeer func(ai peer.AddrInfo, source string)

	// If zero, defaults to 30 minutes.
	AdvertiseInterval time.Duration

	// Added: if true, we will silently skip starting mDNS when no multicast-capable interface is found.
	SkipMDNSIfNoMulticast bool

	// Added: if true, we force attempting mDNS even if no multicast interface appears; warnings may occur.
	RequireMDNS bool
}

type Discovery struct {
	cancel  context.CancelFunc
	mdnsSvc mdns.Service
	dht     *dht.IpfsDHT
}

func (d *Discovery) Close() error {
	if d.cancel != nil {
		d.cancel()
	}
	if d.mdnsSvc != nil {
		_ = d.mdnsSvc.Close()
	}
	if d.dht != nil {
		_ = d.dht.Close()
	}
	return nil
}

// StartDiscovery configures the DHT, optionally starts mDNS, and runs advertise/find loops.
// Minimal side-effect: discovered peers are announced via cfg.OnPeer (if provided) and logged.
// Peer ingestion into an internal store comes in Phase 4.
func StartDiscovery(parent context.Context, h host.Host, cfg DiscoveryConfig) (*Discovery, error) {
	if cfg.Rendezvous == "" {
		return nil, fmt.Errorf("p2p: discovery requires Rendezvous string like /bib/<cluster>")
	}
	if cfg.MDNSServiceTag == "" {
		cfg.MDNSServiceTag = "bib-mdns"
	}
	if cfg.AdvertiseInterval <= 0 {
		cfg.AdvertiseInterval = 30 * time.Minute
	}

	ctx, cancel := context.WithCancel(parent)

	bootstrapInfos, err := ParseBootstrapPeers(cfg.BootstrapPeers)
	if err != nil {
		cancel()
		return nil, err
	}

	// Isolate the DHT protocol to avoid interfering with other vendors.
	opts := []dht.Option{
		dht.ProtocolPrefix(protocol.ID("/bib")),
	}
	if cfg.DHTServer {
		opts = append(opts, dht.Mode(dht.ModeServer))
	} else {
		opts = append(opts, dht.Mode(dht.ModeClient))
	}
	if len(bootstrapInfos) > 0 {
		opts = append(opts, dht.BootstrapPeers(bootstrapInfos...))
	}

	kad, err := dht.New(ctx, h, opts...)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("p2p: create DHT: %w", err)
	}

	// Best-effort connect to bootstraps to seed the DHT.
	for _, ai := range bootstrapInfos {
		go func(ai peer.AddrInfo) {
			_ = h.Connect(ctx, ai)
		}(ai)
	}

	if err := kad.Bootstrap(ctx); err != nil {
		cancel()
		_ = kad.Close()
		return nil, fmt.Errorf("p2p: DHT bootstrap: %w", err)
	}

	// Routing discovery built on the DHT.
	rdisc := rd.NewRoutingDiscovery(kad)

	// Periodic advertisement of our presence on the rendezvous string.
	go func() {
		// Re-advertise periodically; util handles TTL backoff.
		for {
			// In current libp2p versions, discutil.Advertise returns no TTL.
			// We re-announce on a fixed interval instead.
			discutil.Advertise(ctx, rdisc, cfg.Rendezvous)
			select {
			case <-time.After(cfg.AdvertiseInterval):
			case <-ctx.Done():
				return
			}
		}
	}()

	// Continuous peer discovery on the rendezvous string.
	go func() {
		backoff := time.Second
		for {
			peerCh, err := rdisc.FindPeers(ctx, cfg.Rendezvous, lpdisc.Limit(50))
			if err != nil {
				log.Printf("p2p: find peers error: %v", err)
				select {
				case <-time.After(backoff):
				case <-ctx.Done():
					return
				}
				// simple backoff capping
				if backoff < 30*time.Second {
					backoff *= 2
				}
				continue
			}
			// Reset backoff on successful stream
			backoff = time.Second

			for p := range peerCh {
				if p.ID == "" || p.ID == h.ID() {
					continue
				}
				if cfg.OnPeer != nil {
					cfg.OnPeer(p, "dht")
				} else {
					log.Printf("p2p: discovered (dht): %s addrs=%d", p.ID, len(p.Addrs))
				}
			}
			// The channel closes; loop and re-issue FindPeers to keep discovering.
			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
				return
			}
		}
	}()

	var mdnsSvc mdns.Service
	if cfg.EnableMDNS {
		// Multicast viability check
		if !SupportsMulticast() && cfg.SkipMDNSIfNoMulticast && !cfg.RequireMDNS {
			log.Printf("p2p: mDNS skipped (no multicast-capable interface detected)")
		} else {
			notifee := &mdnsNotifee{
				onPeer: func(ai peer.AddrInfo) {
					if ai.ID == "" || ai.ID == h.ID() {
						return
					}
					if cfg.OnPeer != nil {
						cfg.OnPeer(ai, "mdns")
					} else {
						log.Printf("p2p: discovered (mdns): %s addrs=%d", ai.ID, len(ai.Addrs))
					}
				},
			}
			svc := mdns.NewMdnsService(h, cfg.MDNSServiceTag, notifee)
			mdnsSvc = svc
		}
	}

	return &Discovery{
		cancel:  cancel,
		mdnsSvc: mdnsSvc,
		dht:     kad,
	}, nil
}

type mdnsNotifee struct {
	onPeer func(peer.AddrInfo)
}

func (n *mdnsNotifee) HandlePeerFound(ai peer.AddrInfo) {
	if n.onPeer != nil {
		n.onPeer(ai)
	}
}
