package main

import (
	"bib/internal/config"
	"bib/internal/config/util"
	"bib/internal/contexts"
	"bib/internal/daemon"
	"bib/internal/daemon/service"
	"bib/internal/p2p"
	"context"

	"github.com/charmbracelet/log"
	"google.golang.org/grpc"
)

func main() {
	configPath, err := util.FindConfigPath(util.Options{AppName: "bibd", FileNames: []string{"config.yaml", "config.yml", "bib-daemon.yaml", "bib-daemon.yml"}, AlsoCheckCWD: true})

	cfg, err := config.LoadBibDaemonConfig(configPath)
	if err != nil {
		log.Error(err)
		log.Info("You need to create the bib daemon configuration file before running the daemon. We recommend you use the cli to create it. Run 'bib setup --daemon' to create a configuration file for the daemon.")
		return
	}

	log.Info("Starting bib daemon...")
	log.Info("It might take a while to index your library the first time. Please be patient.")

	identity, err := contexts.RegisterDaemonIdentity(cfg, Version)
	if err != nil {
		log.Fatal("Failed to register daemon identity:", "error", err)
	}
	log.Info("Daemon identity registered", "id", identity.ID)

	ctx := context.Background()
	peerStore := p2p.NewPeerStore()

	host, err := p2p.BuildHost(ctx, p2p.Config{
		Identity:        *identity,
		EnableQUIC:      true,
		NATPortMap:      true,
		ListenAddresses: cfg.P2P.Discovery.BootstrapPeers,
	})
	if err != nil {
		log.Fatal(err)
	}
	p2p.RegisterSelf(peerStore, host)

	identitySvc := &service.IdentityService{
		IDCtx: identity,
		Store: service.NewIdentityStore(),
	}

	discoverySvc := &service.DiscoveryService{
		PeerStore: peerStore,
		Host:      host,
	}

	daemon.StartP2P(ctx, host, cfg, peerStore, func(s *grpc.Server) {
		daemon.RegisterBibServices(s, identitySvc, discoverySvc)
	})
	daemon.StartCapabilityChecks(cfg)
	daemon.StartScheduler()
	daemon.StartGRPCServer(cfg, func(s *grpc.Server) {
		daemon.RegisterBibServices(s, identitySvc, discoverySvc)
	})
}
