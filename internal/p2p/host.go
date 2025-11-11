package p2p

import (
	"bib/internal/contexts"
	"context"
	"fmt"
	"strings"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	quic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	tcp "github.com/libp2p/go-libp2p/p2p/transport/tcp"

	"github.com/multiformats/go-multiaddr"
)

type Config struct {
	Identity        contexts.IdentityContext
	ListenAddresses []string
	EnableQUIC      bool
	NATPortMap      bool
}

func BuildHost(ctx context.Context, cfg Config) (host.Host, error) {
	if len(cfg.Identity.KeyMaterial.PrivateKey) == 0 {
		return nil, ErrIdentityNoPrivateKey
	}

	lpPrivateKey, err := PrivateKeyToP2PKey(cfg.Identity.KeyMaterial.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("p2p: convert ed25519 key: %w", err)
	}

	var listenAddrs []multiaddr.Multiaddr

	// Determine needed transports.
	needTCP := false
	needQUIC := false

	if len(cfg.ListenAddresses) == 0 {
		// Defaults: always TCP, optionally QUIC.
		tcpMa, _ := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/0")
		listenAddrs = append(listenAddrs, tcpMa)
		needTCP = true

		if cfg.EnableQUIC {
			quicMa, _ := multiaddr.NewMultiaddr("/ip4/0.0.0.0/udp/0/quic-v1")
			listenAddrs = append(listenAddrs, quicMa)
			needQUIC = true
		}
	} else {
		for _, addrStr := range cfg.ListenAddresses {
			// Guard: listen addresses must NOT include /p2p/<peerID>.
			if strings.Contains(addrStr, "/p2p/") || strings.HasSuffix(addrStr, "/p2p") {
				return nil, fmt.Errorf("p2p: invalid listen address %q: contains /p2p peer component; listen addrs must be transport-only (e.g., /ip4/0.0.0.0/tcp/4001 or /ip4/0.0.0.0/udp/4002/quic-v1)", addrStr)
			}

			ma, err := multiaddr.NewMultiaddr(addrStr)
			if err != nil {
				return nil, fmt.Errorf("p2p: invalid listen address %q: %w", addrStr, ErrInvalidListenAddress)
			}
			listenAddrs = append(listenAddrs, ma)

			if strings.Contains(addrStr, "/tcp/") {
				needTCP = true
			}
			if strings.Contains(addrStr, "/quic-v1") || strings.Contains(addrStr, "/quic/") {
				needQUIC = true
			}
		}
	}

	opts := []libp2p.Option{
		libp2p.Identity(lpPrivateKey),
		libp2p.ListenAddrs(listenAddrs...),
	}

	// IMPORTANT: Specifying Transport replaces defaults; list all needed.
	if needTCP {
		opts = append(opts, libp2p.Transport(tcp.NewTCPTransport))
	}
	if needQUIC {
		opts = append(opts, libp2p.Transport(quic.NewTransport))
	}

	if cfg.NATPortMap {
		opts = append(opts, libp2p.NATPortMap())
	}

	h, err := libp2p.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("p2p: build host: %w", err)
	}
	return h, nil
}
