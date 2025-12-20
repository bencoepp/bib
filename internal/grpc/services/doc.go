// Package services provides gRPC service implementations for the bib daemon.
//
// Each service is in its own subpackage with a consistent structure:
//
//	services/
//	├── admin/      - AdminService (config, system info, cluster, backup)
//	├── auth/       - AuthService (authentication, sessions)
//	├── breakglass/ - BreakGlassService (emergency access)
//	├── dataset/    - DatasetService (dataset CRUD, blob storage)
//	├── health/     - HealthService (health checks, readiness)
//	├── job/        - JobService (background job management)
//	├── node/       - NodeService (P2P node discovery, status)
//	├── query/      - QueryService (data queries)
//	├── topic/      - TopicService (topics, subscriptions, membership)
//	└── user/       - UserService (user CRUD, sessions, preferences)
//
// Each service package exports:
//   - Server struct implementing the gRPC service interface
//   - Config struct for dependency injection
//   - NewServer() constructor (creates server with nil dependencies)
//   - NewServerWithConfig(cfg Config) constructor (creates server with dependencies)
//
// Example usage:
//
//	import "bib/internal/grpc/services/user"
//
//	// Create with dependencies
//	server := user.NewServerWithConfig(user.Config{
//	    Store:       store,
//	    AuditLogger: auditMiddleware,
//	})
package services

// This file serves as package documentation.
// Individual services are implemented in their respective subpackages.
