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

func StartGRPCServer(config *config.BibDaemonConfig) {
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(config.Port))
	if err != nil {
		log.Fatal(err)
	}
	grpcServer := grpc.NewServer()

	identityService := &service.IdentityService{}
	pb.RegisterIdentityServiceServer(grpcServer, identityService)
	log.Info("The gRPC server is running on port", "port", config.Port)

	if err := grpcServer.Serve(listener); err != nil {
		log.Fatal(err)
	}
}
