package daemon

import (
	"bib/internal/config"
	"bib/internal/daemon/service"
	pb "bib/internal/pb/bibd/v1"
	"net"
	"strconv"

	"github.com/charmbracelet/log"
	"google.golang.org/grpc"
)

func StartGRPCServer(cfg *config.BibDaemonConfig, register func(*grpc.Server)) {
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(cfg.Port))
	if err != nil {
		log.Fatal(err)
	}
	srv := grpc.NewServer()
	if register != nil {
		register(srv)
	}

	log.Info("The gRPC server is running on port", "port", cfg.Port)

	if err := srv.Serve(listener); err != nil {
		log.Fatal(err)
	}
}

func RegisterBibServices(
	s *grpc.Server,
	identitySvc *service.IdentityService,
	discoverySvc *service.DiscoveryService,
) {
	if identitySvc != nil {
		pb.RegisterIdentityServiceServer(s, identitySvc)
	}

	if discoverySvc != nil {
		pb.RegisterDiscoveryServer(s, discoverySvc)
	}
}
