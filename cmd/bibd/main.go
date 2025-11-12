package main

import (
	"bib/internal/config"
	"bib/internal/config/util"
	"bib/internal/contexts"
	"bib/internal/daemon"
	"bib/internal/daemon/service"
	"bib/internal/p2p"
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

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

	// Long-lived context that cancels on SIGINT/SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := daemon.OpenDatabase(cfg)
	if err != nil {
		log.Fatal("Failed to open database:", "error", err)
	}

	log.Info("Database connection established")

	peerStore := p2p.NewPeerStore()

	host, err := p2p.BuildHost(p2p.Config{
		Identity:        *identity,
		EnableQUIC:      true,
		NATPortMap:      true,
		ListenAddresses: cfg.P2P.ListenAddresses, // pinned ports if configured
	})
	if err != nil {
		log.Fatal(err)
	}

	cancelConnLog := p2p.AttachConnLoggerWithIngest(host, peerStore)
	defer cancelConnLog()

	p2p.RegisterSelf(peerStore, host)

	discoverySvc := &service.DiscoveryService{
		PeerStore: peerStore,
		Host:      host,
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		daemon.StartP2P(ctx, host, cfg, peerStore, func(s *grpc.Server) {
			daemon.RegisterBibServices(s, discoverySvc)
		})
	}()

	watcher := daemon.StartCapabilityChecks(cfg)
	daemon.StartScheduler(watcher)

	wg.Add(1)
	go func() {
		defer wg.Done()
		daemon.StartGRPCServer(ctx, cfg, func(s *grpc.Server) {
			daemon.RegisterBibServices(s, discoverySvc)
		})
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	log.Info("Shutting down...")

	wg.Wait()

	_ = host.Close()
	if err := daemon.CloseDb(db); err != nil {
		log.Error("Failed to close database:", "error", err)
	} else {
		log.Info("Database connection closed")
	}
	watcher.Stop()
	log.Info("Shutdown complete")
}
