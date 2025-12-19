// Package grpc provides gRPC service implementations for the bib daemon.
package grpc

import (
	"context"

	services "bib/api/gen/go/bib/v1/services"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AdminServiceServer implements the AdminService gRPC service.
type AdminServiceServer struct {
	services.UnimplementedAdminServiceServer
}

// NewAdminServiceServer creates a new AdminServiceServer.
func NewAdminServiceServer() *AdminServiceServer {
	return &AdminServiceServer{}
}

// GetConfig returns current configuration.
func (s *AdminServiceServer) GetConfig(ctx context.Context, req *services.GetConfigRequest) (*services.GetConfigResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetConfig not implemented")
}

// UpdateConfig updates runtime configuration.
func (s *AdminServiceServer) UpdateConfig(ctx context.Context, req *services.UpdateConfigRequest) (*services.UpdateConfigResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateConfig not implemented")
}

// GetMetrics returns Prometheus-style metrics.
func (s *AdminServiceServer) GetMetrics(ctx context.Context, req *services.GetMetricsRequest) (*services.GetMetricsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetMetrics not implemented")
}

// StreamLogs streams daemon logs in real-time.
func (s *AdminServiceServer) StreamLogs(req *services.StreamLogsRequest, stream services.AdminService_StreamLogsServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamLogs not implemented")
}

// GetAuditLogs queries the audit trail.
func (s *AdminServiceServer) GetAuditLogs(ctx context.Context, req *services.GetAuditLogsRequest) (*services.GetAuditLogsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetAuditLogs not implemented")
}

// TriggerBackup initiates a database backup.
func (s *AdminServiceServer) TriggerBackup(ctx context.Context, req *services.TriggerBackupRequest) (*services.TriggerBackupResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method TriggerBackup not implemented")
}

// ListBackups lists available backups.
func (s *AdminServiceServer) ListBackups(ctx context.Context, req *services.ListBackupsRequest) (*services.ListBackupsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListBackups not implemented")
}

// RestoreBackup restores from a backup.
func (s *AdminServiceServer) RestoreBackup(ctx context.Context, req *services.RestoreBackupRequest) (*services.RestoreBackupResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RestoreBackup not implemented")
}

// DeleteBackup deletes a backup.
func (s *AdminServiceServer) DeleteBackup(ctx context.Context, req *services.DeleteBackupRequest) (*services.DeleteBackupResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteBackup not implemented")
}

// GetClusterStatus returns cluster/raft status.
func (s *AdminServiceServer) GetClusterStatus(ctx context.Context, req *services.GetClusterStatusRequest) (*services.GetClusterStatusResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetClusterStatus not implemented")
}

// TriggerSnapshot triggers a Raft snapshot.
func (s *AdminServiceServer) TriggerSnapshot(ctx context.Context, req *services.TriggerSnapshotRequest) (*services.TriggerSnapshotResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method TriggerSnapshot not implemented")
}

// TransferLeadership transfers Raft leadership.
func (s *AdminServiceServer) TransferLeadership(ctx context.Context, req *services.TransferLeadershipRequest) (*services.TransferLeadershipResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method TransferLeadership not implemented")
}

// Shutdown gracefully shuts down the daemon.
func (s *AdminServiceServer) Shutdown(ctx context.Context, req *services.ShutdownRequest) (*services.ShutdownResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Shutdown not implemented")
}

// GetSystemInfo returns system information.
func (s *AdminServiceServer) GetSystemInfo(ctx context.Context, req *services.GetSystemInfoRequest) (*services.GetSystemInfoResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetSystemInfo not implemented")
}

// RunMaintenance runs maintenance tasks.
func (s *AdminServiceServer) RunMaintenance(ctx context.Context, req *services.RunMaintenanceRequest) (*services.RunMaintenanceResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RunMaintenance not implemented")
}
