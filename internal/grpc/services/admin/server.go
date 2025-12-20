// Package admin implements the AdminService gRPC service.
package admin

import (
	"context"
	"encoding/json"
	"runtime"
	"sync"
	"time"

	services "bib/api/gen/go/bib/v1/services"
	"bib/internal/cluster"
	"bib/internal/grpc/interfaces"
	"bib/internal/storage"
	"bib/internal/storage/backup"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

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

	for _, ch := range b.listeners {
		select {
		case ch <- entry:
		default:
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

// Config holds configuration for the admin service server.
type Config struct {
	Store        storage.Store
	ClusterMgr   *cluster.Cluster
	BackupMgr    *backup.Manager
	AuditLogger  interfaces.AuditLogger
	NodeID       string
	ConfigPath   string
	DataDir      string
	StartedAt    time.Time
	ShutdownFunc func()
	Config       interface{}
	LogBuffer    *LogRingBuffer
}

// Server implements the AdminService gRPC service.
type Server struct {
	services.UnimplementedAdminServiceServer
	store        storage.Store
	clusterMgr   *cluster.Cluster
	backupMgr    *backup.Manager
	auditLogger  interfaces.AuditLogger
	nodeID       string
	configPath   string
	dataDir      string
	startedAt    time.Time
	shutdownFunc func()
	config       interface{}
	logBuffer    *LogRingBuffer
}

// NewServer creates a new admin service server.
func NewServer() *Server {
	return &Server{
		startedAt: time.Now(),
		logBuffer: NewLogRingBuffer(1000),
	}
}

// NewServerWithConfig creates a new admin service server with dependencies.
func NewServerWithConfig(cfg Config) *Server {
	logBuffer := cfg.LogBuffer
	if logBuffer == nil {
		logBuffer = NewLogRingBuffer(1000)
	}
	return &Server{
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
func (s *Server) GetConfig(_ context.Context, req *services.GetConfigRequest) (*services.GetConfigResponse, error) {
	if s.config == nil {
		return nil, status.Error(codes.Unavailable, "config not available")
	}

	configMap := configToMap(s.config, req.GetIncludeSecrets())

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
		Config:     configStruct,
		ConfigPath: s.configPath,
	}, nil
}

// GetSystemInfo returns system information.
func (s *Server) GetSystemInfo(_ context.Context, _ *services.GetSystemInfoRequest) (*services.GetSystemInfoResponse, error) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return &services.GetSystemInfoResponse{
		Os:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		NumCpu:       int32(runtime.NumCPU()),
		GoVersion:    runtime.Version(),
		NumGoroutine: int32(runtime.NumGoroutine()),
		StartedAt:    timestamppb.New(s.startedAt),
		Uptime:       durationpb.New(time.Since(s.startedAt)),
		HeapAlloc:    int64(memStats.Alloc),
		TotalMemory:  int64(memStats.Sys),
		UsedMemory:   int64(memStats.Alloc),
	}, nil
}

// Shutdown initiates a graceful shutdown.
func (s *Server) Shutdown(ctx context.Context, req *services.ShutdownRequest) (*services.ShutdownResponse, error) {
	if s.shutdownFunc == nil {
		return nil, status.Error(codes.Unavailable, "shutdown not available")
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogServiceAction(ctx, "DDL", "system", "shutdown", map[string]interface{}{
			"reason": req.GetReason(),
		})
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		s.shutdownFunc()
	}()

	return &services.ShutdownResponse{
		Accepted: true,
		Message:  "Shutdown initiated",
	}, nil
}

// GetClusterStatus returns cluster status.
func (s *Server) GetClusterStatus(_ context.Context, _ *services.GetClusterStatusRequest) (*services.GetClusterStatusResponse, error) {
	if s.clusterMgr == nil {
		return &services.GetClusterStatusResponse{
			Enabled: false,
		}, nil
	}

	clusterStatus := s.clusterMgr.Status()

	members := make([]*services.ClusterMember, len(clusterStatus.Members))
	for i, m := range clusterStatus.Members {
		health := "healthy"
		if !m.IsHealthy {
			health = "unhealthy"
		}
		members[i] = &services.ClusterMember{
			Id:       m.NodeID,
			Address:  m.Address,
			Role:     "voter",
			IsLeader: m.NodeID == clusterStatus.Leader,
			Health:   health,
		}
	}

	return &services.GetClusterStatusResponse{
		Enabled:  true,
		State:    string(clusterStatus.State),
		LeaderId: clusterStatus.Leader,
		Term:     clusterStatus.Term,
		Members:  members,
	}, nil
}

// TriggerBackup triggers a backup operation.
func (s *Server) TriggerBackup(ctx context.Context, req *services.TriggerBackupRequest) (*services.TriggerBackupResponse, error) {
	if s.backupMgr == nil {
		return nil, status.Error(codes.Unavailable, "backup manager not available")
	}

	meta, err := s.backupMgr.Backup(ctx, req.GetName())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to trigger backup: %v", err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogServiceAction(ctx, "CREATE", "backup", meta.ID, nil)
	}

	return &services.TriggerBackupResponse{
		Backup: &services.BackupInfo{
			Id:        meta.ID,
			Name:      meta.Notes,
			CreatedAt: timestamppb.New(meta.Timestamp),
			Size:      meta.Size,
			Status:    "completed",
		},
	}, nil
}

// ListBackups lists available backups.
func (s *Server) ListBackups(ctx context.Context, _ *services.ListBackupsRequest) (*services.ListBackupsResponse, error) {
	if s.backupMgr == nil {
		return nil, status.Error(codes.Unavailable, "backup manager not available")
	}

	backups, err := s.backupMgr.List(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list backups: %v", err)
	}

	protoBackups := make([]*services.BackupInfo, len(backups))
	for i, b := range backups {
		protoBackups[i] = &services.BackupInfo{
			Id:        b.ID,
			Name:      b.Notes,
			CreatedAt: timestamppb.New(b.Timestamp),
			Size:      b.Size,
			Status:    "completed",
		}
	}

	return &services.ListBackupsResponse{
		Backups: protoBackups,
	}, nil
}

// Helper functions

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
	secretKeys := []string{"password", "secret", "token", "key", "credential", "private"}

	for k, v := range m {
		for _, secret := range secretKeys {
			if containsIgnoreCase(k, secret) {
				m[k] = "[REDACTED]"
				break
			}
		}
		if nested, ok := v.(map[string]interface{}); ok {
			redactSecrets(nested)
		}
	}
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr)))
}
