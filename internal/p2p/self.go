package p2p

import (
	"time"

	"github.com/libp2p/go-libp2p/core/host"
)

func RegisterSelf(store *PeerStore, h host.Host) {
	if store == nil || h == nil {
		return
	}
	addresses := make([]string, 0, len(h.Addrs()))
	for _, ma := range h.Addrs() {
		addresses = append(addresses, ma.String())
	}
	store.UpsertFromCandidate(Candidate{
		PeerID:       h.ID().String(),
		MultiAddress: addresses,
		Source:       "self",
		Discovered:   time.Now(),
	})
}
