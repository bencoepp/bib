package daemon

import (
	"bib/internal/config"
	"bib/internal/p2p"
	"context"
	"time"

	"github.com/charmbracelet/log"
	"github.com/libp2p/go-libp2p/core/host"
	"google.golang.org/grpc"
)

func StartP2P(ctx context.Context, host host.Host, cfg *config.BibDaemonConfig, peerStore *p2p.PeerStore, register func(*grpc.Server)) {
	log.Info("Starting P2P host...")

	ingestSink := peerStore.Sink()

	onPeer := p2p.NewPeerIngester(ctx, host, ingestSink, p2p.IngestOptions{
		AddrTTL: 2 * time.Hour,
	})

	disc, err := p2p.StartDiscovery(ctx, host, p2p.DiscoveryConfig{
		Rendezvous:            cfg.P2P.Discovery.Rendezvous,
		EnableMDNS:            cfg.P2P.Discovery.EnableMDNS,
		MDNSServiceTag:        cfg.P2P.Discovery.MDNSServiceTag,
		SkipMDNSIfNoMulticast: cfg.P2P.Discovery.SkipMDNSIfNoMulticast, // quiet skip if running in container lacking multicast
		RequireMDNS:           cfg.P2P.Discovery.RequireMDNS,
		DHTServer:             cfg.P2P.Discovery.DHTServer,
		BootstrapPeers:        cfg.P2P.Discovery.BootstrapPeers,
		OnPeer:                onPeer,
	})
	if err != nil {
		log.Fatalf("failed to start discovery: %v", err)
	}

	prefs := p2p.Preferences{
		PreferRegion:     cfg.P2P.Preferences.Region,
		PreferTags:       cfg.P2P.Preferences.Tags,
		WeightLatency:    cfg.P2P.Preferences.WeightLatency,
		WeightLoad:       cfg.P2P.Preferences.WeightLoad,
		WeightRegion:     cfg.P2P.Preferences.WeightRegion,
		WeightTags:       cfg.P2P.Preferences.WeightTags,
		MinSamplesForRTT: cfg.P2P.Preferences.MinSamplesForRTT,
	}

	sampler := p2p.StartRTTSampler(ctx, host, peerStore, p2p.RTTSamplerOptions{
		Interval:           time.Duration(cfg.P2P.RTT.Interval) * time.Second,
		PerPeerMinInterval: 2 * time.Minute,
		Concurrency:        cfg.P2P.RTT.Concurrency,
		PingsPerPeer:       cfg.P2P.RTT.PingsPerPeer,
		ConnectTimeout:     time.Duration(cfg.P2P.RTT.ConnectTimeout) * time.Second,
		PingTimeout:        time.Duration(cfg.P2P.RTT.PingTimeout) * time.Second,
		Preferences:        prefs,
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
