// Package grpc provides gRPC service implementations for the bib daemon.
package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"sync"
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
	logBuffer    *LogRingBuffer
}

// LogRingBuffer is a thread-safe ring buffer for log entries.
type LogRingBuffer struct {
	entries   []LogEntry
	size      int
	head      int
	count     int
	mu        sync.RWMutex
	listeners map[int64]chan LogEntry
	nextID    int64
}

// LogEntry represents a single log entry.
type LogEntry struct {
	Timestamp time.Time
	Level     string
	Message   string
	Fields    map[string]string
}

// NewLogRingBuffer creates a new log ring buffer with the specified capacity.
func NewLogRingBuffer(capacity int) *LogRingBuffer {
	if capacity <= 0 {
		capacity = 1000
	}
	return &LogRingBuffer{
		entries:   make([]LogEntry, capacity),
		size:      capacity,
		listeners: make(map[int64]chan LogEntry),
	}
}

// Add adds a log entry to the buffer and notifies all listeners.
func (b *LogRingBuffer) Add(entry LogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.entries[b.head] = entry
	b.head = (b.head + 1) % b.size
	if b.count < b.size {
		b.count++
	}

	// Notify listeners (non-blocking)
	for _, ch := range b.listeners {
		select {
		case ch <- entry:
		default:
			// Drop if listener is slow
		}
	}
}

// Recent returns the most recent n entries.
func (b *LogRingBuffer) Recent(n int) []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if n <= 0 || b.count == 0 {
		return nil
	}
	if n > b.count {
		n = b.count
	}

	result := make([]LogEntry, n)
	start := (b.head - n + b.size) % b.size
	for i := 0; i < n; i++ {
		result[i] = b.entries[(start+i)%b.size]
	}
	return result
}

// Subscribe creates a new subscription for log entries.
// Returns a channel and an unsubscribe function.
func (b *LogRingBuffer) Subscribe() (<-chan LogEntry, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.nextID
	b.nextID++

	ch := make(chan LogEntry, 100)
	b.listeners[id] = ch

	unsubscribe := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.listeners, id)
		close(ch)
	}

	return ch, unsubscribe
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
	LogBuffer    *LogRingBuffer
}

// NewAdminServiceServer creates a new AdminServiceServer.
func NewAdminServiceServer() *AdminServiceServer {
	return &AdminServiceServer{
		startedAt: time.Now(),
		logBuffer: NewLogRingBuffer(1000),
	}
}

// NewAdminServiceServerWithConfig creates a new AdminServiceServer with dependencies.
func NewAdminServiceServerWithConfig(cfg AdminServiceConfig) *AdminServiceServer {
	logBuffer := cfg.LogBuffer
	if logBuffer == nil {
		logBuffer = NewLogRingBuffer(1000)
	}
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
		logBuffer:    logBuffer,
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
func (s *AdminServiceServer) StreamLogs(req *services.StreamLogsRequest, stream services.AdminService_StreamLogsServer) error {
	if s.logBuffer == nil {
		return status.Error(codes.Unavailable, "log buffer not initialized")
	}

	// Parse level filter
	levelFilter := strings.ToLower(req.GetLevel())

	// First, send historical entries if requested
	if req.GetIncludeHistory() {
		historyCount := int(req.GetHistoryCount())
		if historyCount <= 0 {
			historyCount = 100 // Default
		}

		recent := s.logBuffer.Recent(historyCount)
		for _, entry := range recent {
			if levelFilter != "" && !matchesLevel(entry.Level, levelFilter) {
				continue
			}

			logEntry := logEntryToProto(entry)
			if err := stream.Send(logEntry); err != nil {
				return err
			}
		}
	}

	// Subscribe for real-time updates (always stream after history)
	logCh, unsubscribe := s.logBuffer.Subscribe()
	defer unsubscribe()

	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case entry, ok := <-logCh:
			if !ok {
				return nil // Channel closed
			}

			if levelFilter != "" && !matchesLevel(entry.Level, levelFilter) {
				continue
			}

			logEntry := logEntryToProto(entry)
			if err := stream.Send(logEntry); err != nil {
				return err
			}
		}
	}
}

// LogMessage adds a log message to the buffer (for use by the daemon).
func (s *AdminServiceServer) LogMessage(level, message string, fields map[string]string) {
	if s.logBuffer == nil {
		return
	}
	s.logBuffer.Add(LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Fields:    fields,
	})
}

// GetLogBuffer returns the log buffer for external access.
func (s *AdminServiceServer) GetLogBuffer() *LogRingBuffer {
	return s.logBuffer
}

func logEntryToProto(entry LogEntry) *services.LogEntry {
	return &services.LogEntry{
		Timestamp: timestamppb.New(entry.Timestamp),
		Level:     entry.Level,
		Message:   entry.Message,
		Fields:    entry.Fields,
	}
}

func matchesLevel(entryLevel, filterLevel string) bool {
	levels := map[string]int{
		"debug": 0,
		"info":  1,
		"warn":  2,
		"error": 3,
		"fatal": 4,
	}

	entryPriority, ok1 := levels[strings.ToLower(entryLevel)]
	filterPriority, ok2 := levels[strings.ToLower(filterLevel)]

	if !ok1 || !ok2 {
		return true // Unknown level, include it
	}

	return entryPriority >= filterPriority
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
