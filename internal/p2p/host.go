package p2p

import (
	"bib/internal/contexts"
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	quic "github.com/libp2p/go-libp2p/p2p/transport/quic"
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

	var multiAddress []multiaddr.Multiaddr
	if len(cfg.ListenAddresses) == 0 {
		tcp, _ := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/0")
		multiAddress = append(multiAddress, tcp)
		if cfg.EnableQUIC {
			quicMa, _ := multiaddr.NewMultiaddr("/ip4/0.0.0.0/udp/0/quic-v1")
			multiAddress = append(multiAddress, quicMa)
		}
	} else {
		for _, s := range cfg.ListenAddresses {
			ma, err := multiaddr.NewMultiaddr(s)
			if err != nil {
				return nil, ErrInvalidListenAddress
			}
			multiAddress = append(multiAddress, ma)
		}
	}

	opts := []libp2p.Option{
		libp2p.Identity(lpPrivateKey),
		libp2p.ListenAddrs(multiAddress...),
	}

	if cfg.EnableQUIC {
		opts = append(opts, libp2p.Transport(quic.NewTransport))
	}
	if cfg.NATPortMap {
		opts = append(opts, libp2p.NATPortMap())
	}

	h, err := libp2p.New(opts...)
	if err != nil {
		return nil, ErrBuildHostFailed
	}
	return h, nil
}
