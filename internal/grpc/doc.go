// Package grpc provides gRPC service implementations for the bib daemon.
//
// Package structure:
//
//	grpc/
//	├── client/         - gRPC client library
//	├── errors/         - Error handling utilities
//	├── interfaces/     - Shared interfaces (HealthProvider, AuditLogger)
//	├── middleware/     - Interceptors (RBAC, audit, logging, recovery, rate limiting)
//	├── services/       - Service implementations
//	│   ├── admin/      - AdminService (config, system info, cluster, backup)
//	│   ├── auth/       - AuthService (authentication, sessions)
//	│   ├── breakglass/ - BreakGlassService (emergency access)
//	│   ├── dataset/    - DatasetService (dataset CRUD)
//	│   ├── health/     - HealthService (health checks)
//	│   ├── job/        - JobService (background jobs)
//	│   ├── node/       - NodeService (P2P node management)
//	│   ├── query/      - QueryService (data queries)
//	│   ├── topic/      - TopicService (topics, subscriptions, membership)
//	│   └── user/       - UserService (user management)
//	├── server.go           - ServiceServers and registration
//	├── server_lifecycle.go - Server lifecycle management
//	└── service_deps.go     - Dependency injection
//
// Usage:
//
//	import "bib/internal/grpc/middleware"
//	import "bib/internal/grpc/interfaces"
//	import "bib/internal/grpc/services/health"
//	import grpcerrors "bib/internal/grpc/errors"
package grpc
