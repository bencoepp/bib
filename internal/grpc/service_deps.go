// Package grpc provides gRPC service implementations for the bib daemon.
package grpc

import (
	"bib/internal/auth"
	"bib/internal/cluster"
	"bib/internal/config"
	"bib/internal/p2p"
	"bib/internal/storage"
	"bib/internal/storage/backup"
	"bib/internal/storage/blob"
	"bib/internal/storage/breakglass"
	"time"
)

// ServiceDependencies holds all dependencies needed by gRPC services.
// This struct is designed to be extended as new services are added.
type ServiceDependencies struct {
	// Core dependencies
	Store     storage.Store
	BlobStore blob.Store

	// Authentication
	AuthService *auth.Service
	AuthConfig  config.AuthConfig

	// P2P networking
	NodeManager p2p.NodeManager
	P2PHost     *p2p.Host

	// Cluster
	ClusterMgr *cluster.Cluster

	// Backup
	BackupMgr *backup.Manager

	// Break glass emergency access
	BreakGlassMgr *breakglass.Manager

	// Configuration
	Config     interface{} // Current config for admin service
	ConfigPath string

	// Node information
	NodeID   string
	NodeMode string
	DataDir  string
	Version  string

	// Lifecycle
	StartedAt    time.Time
	ShutdownFunc func()

	// Audit
	AuditMiddleware *AuditMiddleware
}

// ConfigureServices configures all service servers with the provided dependencies.
// This should be called after creating the Server but before starting it.
func (ss *ServiceServers) ConfigureServices(deps ServiceDependencies) {
	// Configure HealthService
	// Health service uses HealthProvider interface, configured separately via SetProvider

	// Configure AuthService
	if deps.AuthService != nil {
		ss.Auth = NewAuthServiceServerWithConfig(AuthServiceConfig{
			AuthService: deps.AuthService,
			AuthConfig:  deps.AuthConfig,
			NodeID:      deps.NodeID,
			NodeMode:    deps.NodeMode,
			Version:     deps.Version,
		})
	}

	// Configure UserService
	if deps.Store != nil {
		ss.User = NewUserServiceServerWithConfig(UserServiceConfig{
			Store:       deps.Store,
			AuditLogger: deps.AuditMiddleware,
		})
	}

	// Configure NodeService
	if deps.NodeManager != nil {
		ss.Node = NewNodeServiceServerWithConfig(NodeServiceConfig{
			NodeManager:     deps.NodeManager,
			Store:           deps.Store,
			AuditLogger:     deps.AuditMiddleware,
			EventBufferSize: 100,
		})
	}

	// Configure TopicService
	if deps.Store != nil {
		ss.Topic = NewTopicServiceServerWithConfig(TopicServiceConfig{
			Store:       deps.Store,
			AuditLogger: deps.AuditMiddleware,
			NodeMode:    deps.NodeMode,
		})
	}

	// Configure DatasetService
	if deps.Store != nil {
		ss.Dataset = NewDatasetServiceServerWithConfig(DatasetServiceConfig{
			Store:       deps.Store,
			BlobStore:   deps.BlobStore,
			AuditLogger: deps.AuditMiddleware,
			NodeMode:    deps.NodeMode,
		})
	}

	// Configure AdminService
	ss.Admin = NewAdminServiceServerWithConfig(AdminServiceConfig{
		Store:        deps.Store,
		ClusterMgr:   deps.ClusterMgr,
		BackupMgr:    deps.BackupMgr,
		AuditLogger:  deps.AuditMiddleware,
		NodeID:       deps.NodeID,
		ConfigPath:   deps.ConfigPath,
		DataDir:      deps.DataDir,
		StartedAt:    deps.StartedAt,
		ShutdownFunc: deps.ShutdownFunc,
		Config:       deps.Config,
	})

	// Configure QueryService
	if deps.Store != nil {
		ss.Query = NewQueryServiceServerWithConfig(QueryServiceConfig{
			Store:       deps.Store,
			AuditLogger: deps.AuditMiddleware,
		})
	}

	// Configure JobService (stub for now, awaiting Phase 3)
	ss.Job = NewJobServiceServer()

	// Configure BreakGlassService
	if deps.BreakGlassMgr != nil {
		ss.BreakGlass = NewBreakGlassServiceServerWithConfig(BreakGlassServiceConfig{
			Manager:     deps.BreakGlassMgr,
			AuditLogger: deps.AuditMiddleware,
			NodeID:      deps.NodeID,
		})
	}
}

// SetHealthProvider sets the health provider for the health service.
// This is separate because the Daemon itself implements HealthProvider.
func (ss *ServiceServers) SetHealthProvider(provider HealthProvider) {
	if ss.Health != nil {
		ss.Health.SetProvider(provider)
	}
}
