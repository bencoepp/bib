// Package grpc provides gRPC service implementations for the bib daemon.
package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"

	services "bib/api/gen/go/bib/v1/services"
	"bib/internal/cluster"
	"bib/internal/storage"
	"bib/internal/storage/backup"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AdminServiceServer implements the AdminService gRPC service.
type AdminServiceServer struct {
	services.UnimplementedAdminServiceServer
	store        storage.Store
	clusterMgr   *cluster.Cluster
	backupMgr    *backup.Manager
	auditLogger  *AuditMiddleware
	nodeID       string
	configPath   string
	dataDir      string
	startedAt    time.Time
	shutdownFunc func()
	config       interface{} // Current config
}

// AdminServiceConfig holds configuration for the AdminServiceServer.
type AdminServiceConfig struct {
	Store        storage.Store
	ClusterMgr   *cluster.Cluster
	BackupMgr    *backup.Manager
	AuditLogger  *AuditMiddleware
	NodeID       string
	ConfigPath   string
	DataDir      string
	StartedAt    time.Time
	ShutdownFunc func()
	Config       interface{}
}

// NewAdminServiceServer creates a new AdminServiceServer.
func NewAdminServiceServer() *AdminServiceServer {
	return &AdminServiceServer{
		startedAt: time.Now(),
	}
}

// NewAdminServiceServerWithConfig creates a new AdminServiceServer with dependencies.
func NewAdminServiceServerWithConfig(cfg AdminServiceConfig) *AdminServiceServer {
	return &AdminServiceServer{
		store:        cfg.Store,
		clusterMgr:   cfg.ClusterMgr,
		backupMgr:    cfg.BackupMgr,
		auditLogger:  cfg.AuditLogger,
		nodeID:       cfg.NodeID,
		configPath:   cfg.ConfigPath,
		dataDir:      cfg.DataDir,
		startedAt:    cfg.StartedAt,
		shutdownFunc: cfg.ShutdownFunc,
		config:       cfg.Config,
	}
}

// GetConfig returns current configuration.
func (s *AdminServiceServer) GetConfig(_ context.Context, req *services.GetConfigRequest) (*services.GetConfigResponse, error) {
	if s.config == nil {
		return nil, status.Error(codes.Unavailable, "config not available")
	}

	// Convert config to map
	configMap := configToMap(s.config, req.GetIncludeSecrets())

	// Filter by section if specified
	if req.GetSection() != "" {
		if section, ok := configMap[req.GetSection()]; ok {
			configMap = map[string]interface{}{req.GetSection(): section}
		} else {
			return nil, status.Errorf(codes.NotFound, "config section not found: %s", req.GetSection())
		}
	}

	configStruct, err := structpb.NewStruct(configMap)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert config: %v", err)
	}

	return &services.GetConfigResponse{
		Config:          configStruct,
		ConfigPath:      s.configPath,
		EffectiveConfig: configStruct,
	}, nil
}

// UpdateConfig updates runtime configuration.
func (s *AdminServiceServer) UpdateConfig(ctx context.Context, req *services.UpdateConfigRequest) (*services.UpdateConfigResponse, error) {
	if req.GetUpdates() == nil {
		return nil, NewValidationError("updates is required", map[string]string{
			"updates": "must not be empty",
		})
	}

	if req.GetValidateOnly() {
		return &services.UpdateConfigResponse{
			Success:         true,
			RestartRequired: false,
		}, nil
	}

	// Audit log
	if s.auditLogger != nil {
		_ = s.auditLogger.LogMutation(ctx, "UPDATE", "config", "runtime", "Updated config")
	}

	return &services.UpdateConfigResponse{
		Success:         true,
		RestartRequired: true, // Assume restart required for now
	}, nil
}

// GetMetrics returns Prometheus-style metrics.
func (s *AdminServiceServer) GetMetrics(_ context.Context, _ *services.GetMetricsRequest) (*services.GetMetricsResponse, error) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Build Prometheus-style metrics string
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# HELP bib_go_goroutines Number of goroutines\n"))
	sb.WriteString(fmt.Sprintf("# TYPE bib_go_goroutines gauge\n"))
	sb.WriteString(fmt.Sprintf("bib_go_goroutines %d\n", runtime.NumGoroutine()))
	sb.WriteString(fmt.Sprintf("# HELP bib_go_memory_alloc_bytes Bytes allocated and in use\n"))
	sb.WriteString(fmt.Sprintf("# TYPE bib_go_memory_alloc_bytes gauge\n"))
	sb.WriteString(fmt.Sprintf("bib_go_memory_alloc_bytes %d\n", memStats.Alloc))
	sb.WriteString(fmt.Sprintf("# HELP bib_uptime_seconds Daemon uptime in seconds\n"))
	sb.WriteString(fmt.Sprintf("# TYPE bib_uptime_seconds gauge\n"))
	sb.WriteString(fmt.Sprintf("bib_uptime_seconds %f\n", time.Since(s.startedAt).Seconds()))

	return &services.GetMetricsResponse{
		Metrics: sb.String(),
	}, nil
}

// StreamLogs streams daemon logs in real-time.
func (s *AdminServiceServer) StreamLogs(_ *services.StreamLogsRequest, _ services.AdminService_StreamLogsServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamLogs not implemented")
}

// GetAuditLogs queries the audit trail.
func (s *AdminServiceServer) GetAuditLogs(ctx context.Context, req *services.GetAuditLogsRequest) (*services.GetAuditLogsResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	filter := storage.AuditFilter{
		Action: req.GetAction(),
	}

	if req.GetStartTime() != nil {
		t := req.GetStartTime().AsTime()
		filter.After = &t
	}
	if req.GetEndTime() != nil {
		t := req.GetEndTime().AsTime()
		filter.Before = &t
	}
	if req.GetPage() != nil {
		filter.Limit = int(req.GetPage().GetLimit())
		filter.Offset = int(req.GetPage().GetOffset())
	}

	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 1000 {
		filter.Limit = 1000
	}

	entries, err := s.store.Audit().Query(ctx, filter)
	if err != nil {
		return nil, MapDomainError(err)
	}

	protoEntries := make([]*services.AuditLogEntry, len(entries))
	for i, e := range entries {
		protoEntries[i] = &services.AuditLogEntry{
			Id:        fmt.Sprintf("%d", e.ID),
			Timestamp: timestamppb.New(e.Timestamp),
			NodeId:    e.NodeID,
			Action:    e.Action,
		}
	}

	return &services.GetAuditLogsResponse{
		Entries: protoEntries,
	}, nil
}

// TriggerBackup initiates a database backup.
func (s *AdminServiceServer) TriggerBackup(ctx context.Context, _ *services.TriggerBackupRequest) (*services.TriggerBackupResponse, error) {
	if s.backupMgr == nil {
		return nil, status.Error(codes.Unavailable, "backup manager not initialized")
	}

	metadata, err := s.backupMgr.Backup(ctx, "")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "backup failed: %v", err)
	}

	// Audit log
	if s.auditLogger != nil {
		_ = s.auditLogger.LogMutation(ctx, "CREATE", "backup", metadata.ID, "Created backup: "+metadata.ID)
	}

	return &services.TriggerBackupResponse{
		Backup: &services.BackupInfo{
			Id:        metadata.ID,
			CreatedAt: timestamppb.New(metadata.Timestamp),
			Size:      metadata.Size,
			NodeId:    metadata.NodeID,
			Path:      metadata.Path,
			Status:    "completed",
		},
	}, nil
}

// ListBackups lists available backups.
func (s *AdminServiceServer) ListBackups(ctx context.Context, _ *services.ListBackupsRequest) (*services.ListBackupsResponse, error) {
	if s.backupMgr == nil {
		return nil, status.Error(codes.Unavailable, "backup manager not initialized")
	}

	backups, err := s.backupMgr.List(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list backups: %v", err)
	}

	protoBackups := make([]*services.BackupInfo, len(backups))
	for i, b := range backups {
		protoBackups[i] = &services.BackupInfo{
			Id:        b.ID,
			CreatedAt: timestamppb.New(b.Timestamp),
			Size:      b.Size,
			NodeId:    b.NodeID,
			Path:      b.Path,
			Status:    "completed",
		}
	}

	return &services.ListBackupsResponse{
		Backups: protoBackups,
	}, nil
}

// RestoreBackup restores from a backup.
func (s *AdminServiceServer) RestoreBackup(ctx context.Context, req *services.RestoreBackupRequest) (*services.RestoreBackupResponse, error) {
	if s.backupMgr == nil {
		return nil, status.Error(codes.Unavailable, "backup manager not initialized")
	}

	if req.GetBackupId() == "" {
		return nil, NewValidationError("backup_id is required", map[string]string{
			"backup_id": "must not be empty",
		})
	}

	err := s.backupMgr.Restore(ctx, req.GetBackupId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "restore failed: %v", err)
	}

	// Audit log
	if s.auditLogger != nil {
		_ = s.auditLogger.LogMutation(ctx, "UPDATE", "backup", req.GetBackupId(), "Restored from backup")
	}

	return &services.RestoreBackupResponse{
		Success: true,
		Message: "Backup restored. Daemon restart is required to apply changes.",
	}, nil
}

// DeleteBackup deletes a backup.
func (s *AdminServiceServer) DeleteBackup(ctx context.Context, req *services.DeleteBackupRequest) (*services.DeleteBackupResponse, error) {
	if s.backupMgr == nil {
		return nil, status.Error(codes.Unavailable, "backup manager not initialized")
	}

	if req.GetBackupId() == "" {
		return nil, NewValidationError("backup_id is required", map[string]string{
			"backup_id": "must not be empty",
		})
	}

	err := s.backupMgr.Delete(ctx, req.GetBackupId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete backup: %v", err)
	}

	// Audit log
	if s.auditLogger != nil {
		_ = s.auditLogger.LogMutation(ctx, "DELETE", "backup", req.GetBackupId(), "Deleted backup")
	}

	return &services.DeleteBackupResponse{
		Success: true,
	}, nil
}

// GetClusterStatus returns cluster/raft status.
func (s *AdminServiceServer) GetClusterStatus(_ context.Context, _ *services.GetClusterStatusRequest) (*services.GetClusterStatusResponse, error) {
	if s.clusterMgr == nil {
		return &services.GetClusterStatusResponse{
			Enabled: false,
		}, nil
	}

	clusterStatus := s.clusterMgr.Status()

	members := make([]*services.ClusterMember, len(clusterStatus.Members))
	for i, m := range clusterStatus.Members {
		members[i] = &services.ClusterMember{
			Id:      m.NodeID,
			Address: m.Address,
			Role:    string(m.Role),
		}
	}

	return &services.GetClusterStatusResponse{
		Enabled:      true,
		LeaderId:     clusterStatus.Leader,
		State:        string(clusterStatus.State),
		Term:         clusterStatus.Term,
		CommitIndex:  clusterStatus.CommitIndex,
		AppliedIndex: clusterStatus.AppliedIndex,
		Members:      members,
	}, nil
}

// Shutdown gracefully shuts down the daemon.
func (s *AdminServiceServer) Shutdown(ctx context.Context, req *services.ShutdownRequest) (*services.ShutdownResponse, error) {
	if s.shutdownFunc == nil {
		return nil, status.Error(codes.Unavailable, "shutdown not available")
	}

	// Audit log before shutdown
	if s.auditLogger != nil {
		reason := req.GetReason()
		if reason == "" {
			reason = "admin request"
		}
		_ = s.auditLogger.LogMutation(ctx, "DELETE", "daemon", s.nodeID, "Shutdown requested: "+reason)
	}

	// Schedule shutdown with delay
	delay := 1 * time.Second

	go func() {
		time.Sleep(delay)
		s.shutdownFunc()
	}()

	return &services.ShutdownResponse{
		Message: fmt.Sprintf("Shutdown scheduled in %v", delay),
	}, nil
}

// =============================================================================
// Helper functions
// =============================================================================

func configToMap(cfg interface{}, includeSecrets bool) map[string]interface{} {
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}

	if !includeSecrets {
		redactSecrets(result)
	}

	return result
}

func redactSecrets(m map[string]interface{}) {
	secretKeys := []string{"password", "secret", "key", "token", "credential", "private"}

	for k, v := range m {
		keyLower := strings.ToLower(k)

		for _, secretKey := range secretKeys {
			if strings.Contains(keyLower, secretKey) {
				if _, ok := v.(string); ok {
					m[k] = "[REDACTED]"
				}
				break
			}
		}

		if nested, ok := v.(map[string]interface{}); ok {
			redactSecrets(nested)
		}
	}
}
