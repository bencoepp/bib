package p2p

import (
	"bytes"
	"crypto/ed25519"
	"fmt"
	"net"

	lpcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

func PrivateKeyToP2PKey(k ed25519.PrivateKey) (lpcrypto.PrivKey, error) {
	switch len(k) {
	case ed25519.PrivateKeySize:
		return lpcrypto.UnmarshalEd25519PrivateKey(k)
	case ed25519.SeedSize:
		r := bytes.NewReader(k)
		lpPrivy, _, err := lpcrypto.GenerateEd25519Key(r)
		return lpPrivy, err
	default:
		return nil, fmt.Errorf("unexpected ed25519 key length %d; want 32 or 64", len(k))
	}
}

func ParseBootstrapPeers(addresses []string) ([]peer.AddrInfo, error) {
	var out []peer.AddrInfo
	for _, s := range addresses {
		ma, err := multiaddr.NewMultiaddr(s)
		if err != nil {
			return nil, fmt.Errorf("p2p: invalid bootstrap addr %q: %w", s, err)
		}
		ai, err := peer.AddrInfoFromP2pAddr(ma)
		if err != nil {
			return nil, fmt.Errorf("p2p: bootstrap addr missing /p2p component %q: %w", s, err)
		}
		out = append(out, *ai)
	}
	return out, nil
}

func SupportsMulticast() bool {
	ifaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	for _, ifc := range ifaces {
		if (ifc.Flags&net.FlagUp) != 0 &&
			(ifc.Flags&net.FlagMulticast) != 0 &&
			(ifc.Flags&net.FlagLoopback) == 0 {
			return true
		}
	}
	return false
}

func DedupeStrings(in []string, base []string) []string {
	if len(in) == 0 && len(base) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(base)+len(in))
	out := make([]string, 0, len(base)+len(in))
	for _, s := range base {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func NormalizePreferences(p Preferences) Preferences {
	if p.WeightLatency == 0 {
		p.WeightLatency = 1.0
	}
	if p.WeightLoad == 0 {
		p.WeightLoad = 0.5
	}
	if p.WeightRegion == 0 {
		p.WeightRegion = 0.3
	}
	if p.WeightTags == 0 {
		p.WeightTags = 0.2
	}
	if p.MinSamplesForRTT <= 0 {
		p.MinSamplesForRTT = 3
	}
	return p
}
