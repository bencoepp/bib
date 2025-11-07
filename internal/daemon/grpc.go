package daemon

import (
	"bib/internal/config"
	"bib/internal/contexts"
	"bib/internal/daemon/service"
	pb "bib/internal/pb/bibd/v1"
	"net"
	"strconv"

	"github.com/charmbracelet/log"
	"google.golang.org/grpc"
)

func StartGRPCServer(cfg *config.BibDaemonConfig, idCtx *contexts.IdentityContext) {
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(cfg.Port))
	if err != nil {
		log.Fatal(err)
	}
	grpcServer := grpc.NewServer()

	identityService := &service.IdentityService{
		IDCtx: idCtx,
		Store: service.NewIdentityStore(),
	}
	pb.RegisterIdentityServiceServer(grpcServer, identityService)
	log.Info("The gRPC server is running on port", "port", cfg.Port)

	if err := grpcServer.Serve(listener); err != nil {
		log.Fatal(err)
	}
}
