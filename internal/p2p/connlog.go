package p2p

import (
	"time"

	"github.com/charmbracelet/log"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
)

func AttachConnLoggerWithIngest(h host.Host, store *PeerStore) func() {
	b := &network.NotifyBundle{
		ConnectedF: func(n network.Network, c network.Conn) {
			peerID := c.RemotePeer()
			// Log
			log.Info("p2p connected",
				"peer", peerID.String(),
				"remote", c.RemoteMultiaddr().String(),
				"local", c.LocalMultiaddr().String(),
				"dir", c.Stat().Direction.String(),
			)

			// Ingest into app store
			if store != nil {
				// Collect addresses: start with the connection's remote addr, then add known peerstore addrs.
				seen := map[string]struct{}{
					c.RemoteMultiaddr().String(): {},
				}
				var addrs []string
				addrs = append(addrs, c.RemoteMultiaddr().String())
				for _, ma := range h.Peerstore().Addrs(peerID) {
					s := ma.String()
					if _, ok := seen[s]; ok {
						continue
					}
					seen[s] = struct{}{}
					addrs = append(addrs, s)
				}
				store.UpsertFromCandidate(Candidate{
					PeerID:     peerID.String(),
					Multiaddrs: addrs,
					Source:     "conn",
					Discovered: time.Now(),
				})
			}
		},
		DisconnectedF: func(n network.Network, c network.Conn) {
			log.Info("p2p disconnected",
				"peer", c.RemotePeer().String(),
				"remote", c.RemoteMultiaddr().String(),
				"local", c.LocalMultiaddr().String(),
				"dir", c.Stat().Direction.String(),
			)
		},
	}
	h.Network().Notify(b)
	return func() { h.Network().StopNotify(b) }
}
