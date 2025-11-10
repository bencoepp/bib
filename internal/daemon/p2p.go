package daemon

import (
	"bib/internal/contexts"
	"bib/internal/p2p"
	"context"
	"time"

	"github.com/charmbracelet/log"
	"github.com/libp2p/go-libp2p/core/host"
	"google.golang.org/grpc"
)

func StartP2P(ctx context.Context, identity contexts.IdentityContext, host host.Host, peerStore *p2p.PeerStore, register func(*grpc.Server)) {
	log.Info("Starting P2P host...")

	ingestSink := peerStore.Sink()

	onPeer := p2p.NewPeerIngester(ctx, host, ingestSink, p2p.IngestOptions{
		AddrTTL: 2 * time.Hour, // optional; tweak via config later
	})

	disc, err := p2p.StartDiscovery(ctx, host, p2p.DiscoveryConfig{
		Rendezvous:            "/bib/default",
		EnableMDNS:            true,
		MDNSServiceTag:        "bib-mdns",
		SkipMDNSIfNoMulticast: true, // quiet skip if running in container lacking multicast
		// RequireMDNS:        false,   // set true only if you insist mDNS must run
		DHTServer:      false,
		BootstrapPeers: nil,
		OnPeer:         onPeer,
	})
	if err != nil {
		log.Fatalf("failed to start discovery: %v", err)
	}

	prefs := p2p.Preferences{
		// PreferRegion: "us-east-1",
		// PreferTags:   []string{"low-latency"},
	}

	sampler := p2p.StartRTTSampler(ctx, host, peerStore, p2p.RTTSamplerOptions{
		Interval:           1 * time.Minute,
		PerPeerMinInterval: 2 * time.Minute,
		Concurrency:        4,
		PingsPerPeer:       3,
		ConnectTimeout:     5 * time.Second,
		PingTimeout:        3 * time.Second,
		// TCPPing:         nil, // can be provided later when you map peerID -> TCP addr
		Preferences: prefs,
	})
	defer sampler.Stop()

	// Start a second gRPC server over libp2p streams.
	// Reuse the same service registrations as your TCP gRPC server.
	grpcP2P, err := p2p.StartGRPCOverP2P(ctx, host, p2p.DefaultGRPCProtocolID, register)
	if err != nil {
		log.Fatal("failed to start gRPC over libp2p: %v", err)
	}
	defer grpcP2P.Stop()

	log.Infof("gRPC over libp2p ready: protocol=%q peer=%s", p2p.DefaultGRPCProtocolID, host.ID())

	defer func() { _ = disc.Close() }()

	defer func() {
		_ = host.Close()
	}()

	log.Info("P2P host started", "id", host.ID().String(), "addresses", host.Addrs())
}
