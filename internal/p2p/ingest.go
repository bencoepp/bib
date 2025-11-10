package p2p

import (
	"context"
	"time"

	"github.com/charmbracelet/log"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

type Candidate struct {
	PeerID     string
	Multiaddrs []string
	Source     string // "dht" or "mdns" (or future sources)
	Discovered time.Time
}

type IngestOptions struct {
	// How long to keep discovered addresses in libp2p's Peerstore.
	// If zero, defaults to 2 hours.
	AddrTTL time.Duration
}

// NewPeerIngester returns a callback compatible with DiscoveryConfig.OnPeer.
// It updates the libp2p Peerstore and forwards a Candidate to the provided sink.
func NewPeerIngester(
	ctx context.Context,
	h host.Host,
	sink func(context.Context, Candidate),
	opts IngestOptions,
) func(ai peer.AddrInfo, source string) {
	if opts.AddrTTL <= 0 {
		opts.AddrTTL = 2 * time.Hour
	}
	return func(ai peer.AddrInfo, source string) {
		if ai.ID == "" || ai.ID == h.ID() {
			return
		}
		// Dedup addrs and add to peerstore with TTL.
		seen := make(map[string]struct{}, len(ai.Addrs))
		var list []multiaddr.Multiaddr
		for _, ma := range ai.Addrs {
			if ma == nil {
				continue
			}
			s := ma.String()
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			list = append(list, ma)
		}
		if len(list) > 0 {
			h.Peerstore().AddAddrs(ai.ID, list, opts.AddrTTL)
		}

		// Build candidate for the application store.
		var sAddrs []string
		for _, ma := range list {
			sAddrs = append(sAddrs, ma.String())
		}
		c := Candidate{
			PeerID:     ai.ID.String(),
			Multiaddrs: sAddrs,
			Source:     source,
			Discovered: time.Now(),
		}

		// Forward to sink in a goroutine so discovery loop never blocks.
		if sink != nil {
			go func() {
				defer func() {
					// Avoid panicking the discovery loop if sink crashes.
					if r := recover(); r != nil {
						log.Printf("p2p: ingestion sink panic recovered: %v", r)
					}
				}()
				sink(ctx, c)
			}()
		}
	}
}
