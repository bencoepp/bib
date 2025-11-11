package p2p

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	lpdisc "github.com/libp2p/go-libp2p/core/discovery"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	rd "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	discutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
)

type DiscoveryConfig struct {
	Rendezvous            string
	EnableMDNS            bool
	MDNSServiceTag        string
	DHTServer             bool
	BootstrapPeers        []string
	OnPeer                func(ai peer.AddrInfo, source string)
	AdvertiseInterval     time.Duration
	SkipMDNSIfNoMulticast bool
	RequireMDNS           bool
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

	opts := []dht.Option{
		dht.ProtocolPrefix("/bib"),
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
			dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			err := h.Connect(dialCtx, ai)
			if err != nil {
				// Differentiate swarm closed
				if strings.Contains(err.Error(), "swarm closed") {
					log.Warn("p2p: bootstrap dial aborted (swarm closed)", "peer", ai.ID, "err", err)
				} else {
					log.Warn("p2p: bootstrap dial failed", "peer", ai.ID, "err", err)
				}
				return
			}
			log.Info("p2p: bootstrap connected", "peer", ai.ID)
		}(ai)
	}

	if err := kad.Bootstrap(ctx); err != nil {
		cancel()
		_ = kad.Close()
		return nil, fmt.Errorf("p2p: DHT bootstrap: %w", err)
	}

	rDiscovery := rd.NewRoutingDiscovery(kad)

	go func() {
		for {
			discutil.Advertise(ctx, rDiscovery, cfg.Rendezvous)
			select {
			case <-time.After(cfg.AdvertiseInterval):
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		backoff := time.Second
		for {
			peerCh, err := rDiscovery.FindPeers(ctx, cfg.Rendezvous, lpdisc.Limit(50))
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
			notifier := &mdnsNotifier{
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
			svc := mdns.NewMdnsService(h, cfg.MDNSServiceTag, notifier)
			mdnsSvc = svc
		}
	}

	return &Discovery{
		cancel:  cancel,
		mdnsSvc: mdnsSvc,
		dht:     kad,
	}, nil
}

type mdnsNotifier struct {
	onPeer func(peer.AddrInfo)
}

func (n *mdnsNotifier) HandlePeerFound(ai peer.AddrInfo) {
	if n.onPeer != nil {
		n.onPeer(ai)
	}
}
