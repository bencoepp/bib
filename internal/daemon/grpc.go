package daemon

import (
	"bib/internal/config"
	"bib/internal/daemon/service"
	pb "bib/internal/pb/bibd/v1"
	"context"
	"net"
	"strconv"

	"github.com/charmbracelet/log"
	"google.golang.org/grpc"
)

func StartGRPCServer(ctx context.Context, cfg *config.BibDaemonConfig, register func(*grpc.Server)) {
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(cfg.Port))
	if err != nil {
		log.Fatal(err)
	}
	srv := grpc.NewServer()
	if register != nil {
		register(srv)
	}

	go func() {
		<-ctx.Done()
		srv.GracefulStop()
	}()

	go func() {
		log.Info("The gRPC server is running on port", "port", cfg.Port)

		if err := srv.Serve(listener); err != nil {
			log.Fatal(err)
		}
	}()
}

func RegisterBibServices(
	s *grpc.Server,
	discoverySvc *service.DiscoveryService,
) {

	if discoverySvc != nil {
		pb.RegisterDiscoveryServer(s, discoverySvc)
	}
}
