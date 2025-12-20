// Package grpc provides gRPC service implementations for the bib daemon.
package grpc

import (
	services "bib/api/gen/go/bib/v1/services"

	"bib/internal/grpc/services/admin"
	authsvc "bib/internal/grpc/services/auth"
	"bib/internal/grpc/services/breakglass"
	"bib/internal/grpc/services/dataset"
	"bib/internal/grpc/services/health"
	"bib/internal/grpc/services/job"
	"bib/internal/grpc/services/node"
	"bib/internal/grpc/services/query"
	"bib/internal/grpc/services/topic"
	"bib/internal/grpc/services/user"

	"google.golang.org/grpc"
)

// RegisterAllServices registers all bib gRPC services with the given server.
func RegisterAllServices(s *grpc.Server) {
	services.RegisterHealthServiceServer(s, health.NewServer())
	services.RegisterAuthServiceServer(s, authsvc.NewServer())
	services.RegisterUserServiceServer(s, user.NewServer())
	services.RegisterNodeServiceServer(s, node.NewServer())
	services.RegisterTopicServiceServer(s, topic.NewServer())
	services.RegisterDatasetServiceServer(s, dataset.NewServer())
	services.RegisterAdminServiceServer(s, admin.NewServer())
	services.RegisterQueryServiceServer(s, query.NewServer())
	services.RegisterJobServiceServer(s, job.NewServer())
	services.RegisterBreakGlassServiceServer(s, breakglass.NewServer())
}

// ServiceServers holds all the service server instances.
// Use this when you need to inject dependencies into the services.
type ServiceServers struct {
	Health     *health.Server
	Auth       *authsvc.Server
	User       *user.Server
	Node       *node.Server
	Topic      *topic.Server
	Dataset    *dataset.Server
	Admin      *admin.Server
	Query      *query.Server
	Job        *job.Server
	BreakGlass *breakglass.Server
}

// NewServiceServers creates all service server instances.
func NewServiceServers() *ServiceServers {
	return &ServiceServers{
		Health:     health.NewServer(),
		Auth:       authsvc.NewServer(),
		User:       user.NewServer(),
		Node:       node.NewServer(),
		Topic:      topic.NewServer(),
		Dataset:    dataset.NewServer(),
		Admin:      admin.NewServer(),
		Query:      query.NewServer(),
		Job:        job.NewServer(),
		BreakGlass: breakglass.NewServer(),
	}
}

// Register registers all services with the given gRPC server.
func (ss *ServiceServers) Register(s *grpc.Server) {
	services.RegisterHealthServiceServer(s, ss.Health)
	services.RegisterAuthServiceServer(s, ss.Auth)
	services.RegisterUserServiceServer(s, ss.User)
	services.RegisterNodeServiceServer(s, ss.Node)
	services.RegisterTopicServiceServer(s, ss.Topic)
	services.RegisterDatasetServiceServer(s, ss.Dataset)
	services.RegisterAdminServiceServer(s, ss.Admin)
	services.RegisterQueryServiceServer(s, ss.Query)
	services.RegisterJobServiceServer(s, ss.Job)
	services.RegisterBreakGlassServiceServer(s, ss.BreakGlass)
}
