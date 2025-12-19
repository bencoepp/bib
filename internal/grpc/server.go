// Package grpc provides gRPC service implementations for the bib daemon.
package grpc

import (
	services "bib/api/gen/go/bib/v1/services"

	"google.golang.org/grpc"
)

// RegisterAllServices registers all bib gRPC services with the given server.
func RegisterAllServices(s *grpc.Server) {
	services.RegisterHealthServiceServer(s, NewHealthServiceServer())
	services.RegisterAuthServiceServer(s, NewAuthServiceServer())
	services.RegisterUserServiceServer(s, NewUserServiceServer())
	services.RegisterNodeServiceServer(s, NewNodeServiceServer())
	services.RegisterTopicServiceServer(s, NewTopicServiceServer())
	services.RegisterDatasetServiceServer(s, NewDatasetServiceServer())
	services.RegisterAdminServiceServer(s, NewAdminServiceServer())
	services.RegisterQueryServiceServer(s, NewQueryServiceServer())
	services.RegisterJobServiceServer(s, NewJobServiceServer())
	services.RegisterBreakGlassServiceServer(s, NewBreakGlassServiceServer())
}

// ServiceServers holds all the service server instances.
// Use this when you need to inject dependencies into the services.
type ServiceServers struct {
	Health     *HealthServiceServer
	Auth       *AuthServiceServer
	User       *UserServiceServer
	Node       *NodeServiceServer
	Topic      *TopicServiceServer
	Dataset    *DatasetServiceServer
	Admin      *AdminServiceServer
	Query      *QueryServiceServer
	Job        *JobServiceServer
	BreakGlass *BreakGlassServiceServer
}

// NewServiceServers creates all service server instances.
func NewServiceServers() *ServiceServers {
	return &ServiceServers{
		Health:     NewHealthServiceServer(),
		Auth:       NewAuthServiceServer(),
		User:       NewUserServiceServer(),
		Node:       NewNodeServiceServer(),
		Topic:      NewTopicServiceServer(),
		Dataset:    NewDatasetServiceServer(),
		Admin:      NewAdminServiceServer(),
		Query:      NewQueryServiceServer(),
		Job:        NewJobServiceServer(),
		BreakGlass: NewBreakGlassServiceServer(),
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
