// Package grpc provides gRPC service implementations for the bib daemon.
package grpc

import (
	"context"

	services "bib/api/gen/go/bib/v1/services"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// HealthServiceServer implements the HealthService gRPC service.
type HealthServiceServer struct {
	services.UnimplementedHealthServiceServer
}

// NewHealthServiceServer creates a new HealthServiceServer.
func NewHealthServiceServer() *HealthServiceServer {
	return &HealthServiceServer{}
}

// Check performs a health check.
func (s *HealthServiceServer) Check(ctx context.Context, req *services.HealthCheckRequest) (*services.HealthCheckResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Check not implemented")
}

// Watch streams health status changes.
func (s *HealthServiceServer) Watch(req *services.HealthCheckRequest, stream services.HealthService_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "method Watch not implemented")
}

// GetNodeInfo returns detailed node information.
func (s *HealthServiceServer) GetNodeInfo(ctx context.Context, req *services.GetNodeInfoRequest) (*services.GetNodeInfoResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetNodeInfo not implemented")
}

// Ping is a simple connectivity check.
func (s *HealthServiceServer) Ping(ctx context.Context, req *services.PingRequest) (*services.PingResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Ping not implemented")
}
