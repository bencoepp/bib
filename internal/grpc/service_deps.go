// Package grpc provides gRPC service implementations for the bib daemon.
package grpc

import (
	"time"

	internalauth "bib/internal/auth"
	"bib/internal/cluster"
	"bib/internal/config"
	"bib/internal/grpc/interfaces"
	"bib/internal/grpc/middleware"
	"bib/internal/grpc/services/admin"
	authsvc "bib/internal/grpc/services/auth"
	"bib/internal/grpc/services/breakglass"
	"bib/internal/grpc/services/dataset"
	"bib/internal/grpc/services/job"
	"bib/internal/grpc/services/node"
	"bib/internal/grpc/services/query"
	"bib/internal/grpc/services/topic"
	"bib/internal/grpc/services/user"
	"bib/internal/p2p"
	"bib/internal/storage"
	"bib/internal/storage/backup"
	"bib/internal/storage/blob"
	breakglassmgr "bib/internal/storage/breakglass"
)

// ServiceDependencies holds all dependencies needed by gRPC services.
// This struct is designed to be extended as new services are added.
type ServiceDependencies struct {
	// Core dependencies
	Store     storage.Store
	BlobStore blob.Store

	// Authentication
	AuthService *internalauth.Service
	AuthConfig  config.AuthConfig

	// P2P networking
	NodeManager p2p.NodeManager
	P2PHost     *p2p.Host

	// Cluster
	ClusterMgr *cluster.Cluster

	// Backup
	BackupMgr *backup.Manager

	// Break glass emergency access
	BreakGlassMgr *breakglassmgr.Manager

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
	AuditMiddleware *middleware.AuditMiddleware
}

// ConfigureServices configures all service servers with the provided dependencies.
// This should be called after creating the Server but before starting it.
func (ss *ServiceServers) ConfigureServices(deps ServiceDependencies) {
	// Configure AuthService
	if deps.AuthService != nil {
		ss.Auth = authsvc.NewServerWithConfig(authsvc.Config{
			AuthService: deps.AuthService,
			AuthConfig:  deps.AuthConfig,
			NodeID:      deps.NodeID,
			NodeMode:    deps.NodeMode,
			Version:     deps.Version,
		})
	}

	// Configure UserService
	if deps.Store != nil {
		ss.User = user.NewServerWithConfig(user.Config{
			Store:       deps.Store,
			AuditLogger: deps.AuditMiddleware,
		})
	}

	// Configure NodeService
	if deps.NodeManager != nil {
		ss.Node = node.NewServerWithConfig(node.Config{
			NodeManager:     deps.NodeManager,
			Store:           deps.Store,
			AuditLogger:     deps.AuditMiddleware,
			EventBufferSize: 100,
		})
	}

	// Configure TopicService
	if deps.Store != nil {
		ss.Topic = topic.NewServerWithConfig(topic.Config{
			Store:       deps.Store,
			AuditLogger: deps.AuditMiddleware,
			NodeMode:    deps.NodeMode,
		})
	}

	// Configure DatasetService
	if deps.Store != nil {
		ss.Dataset = dataset.NewServerWithConfig(dataset.Config{
			Store:       deps.Store,
			BlobStore:   deps.BlobStore,
			AuditLogger: deps.AuditMiddleware,
			NodeMode:    deps.NodeMode,
		})
	}

	// Configure AdminService
	ss.Admin = admin.NewServerWithConfig(admin.Config{
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
		ss.Query = query.NewServerWithConfig(query.Config{
			Store:       deps.Store,
			AuditLogger: deps.AuditMiddleware,
		})
	}

	// Configure JobService
	ss.Job = job.NewServer()

	// Configure BreakGlassService
	if deps.BreakGlassMgr != nil {
		ss.BreakGlass = breakglass.NewServerWithConfig(breakglass.Config{
			Manager:     deps.BreakGlassMgr,
			AuditLogger: deps.AuditMiddleware,
			NodeID:      deps.NodeID,
		})
	}
}

// SetHealthProvider sets the health provider for the health service.
// This is separate because the Daemon itself implements HealthProvider.
func (ss *ServiceServers) SetHealthProvider(provider interfaces.HealthProvider) {
	if ss.Health != nil {
		ss.Health.SetProvider(provider)
	}
}
