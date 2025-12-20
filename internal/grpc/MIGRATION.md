# gRPC Package Migration Guide

This document describes the new structure of the `internal/grpc` package after the refactoring.

## New Structure

```
internal/grpc/
├── client/              # gRPC client implementation
├── errors/              # Error handling utilities
│   └── errors.go        # MapDomainError, NewValidationError, etc.
├── interfaces/          # Shared interfaces
│   └── health_provider.go  # HealthProvider, AuditLogger interfaces
├── middleware/          # gRPC interceptors
│   ├── audit.go         # Audit logging interceptor
│   ├── interceptors.go  # Request ID, logging, recovery, rate limiting
│   └── rbac.go          # Role-based access control
├── services/            # Service implementations
│   ├── admin/           # AdminService
│   │   └── server.go
│   ├── auth/            # AuthService
│   │   └── server.go
│   ├── breakglass/      # BreakGlassService
│   │   └── server.go
│   ├── dataset/         # DatasetService
│   │   └── server.go
│   ├── health/          # HealthService
│   │   └── server.go
│   ├── job/             # JobService
│   │   └── server.go
│   ├── node/            # NodeService
│   │   └── server.go
│   ├── query/           # QueryService
│   │   └── server.go
│   ├── topic/           # TopicService
│   │   ├── server.go
│   │   └── membership.go
│   └── user/            # UserService
│       ├── server.go
│       └── converters.go
├── doc.go               # Package documentation
├── server.go            # ServiceServers and registration
├── server_lifecycle.go  # Server lifecycle management
├── service_deps.go      # Dependency injection
└── service_deps_test.go # Tests
```

## Key Changes

### 1. Service Packages
Each gRPC service is now in its own package under `services/`:
- Clear separation of concerns
- Independent testing
- Easier navigation

### 2. Middleware Package
All interceptors moved to `middleware/`:
- `RequestIDUnaryInterceptor`, `RequestIDStreamInterceptor`
- `LoggingUnaryInterceptor`, `LoggingStreamInterceptor`
- `RecoveryUnaryInterceptor`, `RecoveryStreamInterceptor`
- `RateLimitUnaryInterceptor`, `RateLimitStreamInterceptor`
- `AuditUnaryInterceptor`, `AuditStreamInterceptor`
- `RBACConfig`, `RBACUnaryInterceptor`

### 3. Errors Package
Error utilities moved to `errors/`:
- `MapDomainError()` - Maps domain errors to gRPC status
- `NewValidationError()` - Creates validation errors
- `NewResourceNotFoundError()` - Creates not-found errors
- `NewPermissionDeniedError()` - Creates permission errors
- `NewPreconditionError()` - Creates precondition errors

### 4. Interfaces Package
Shared interfaces in `interfaces/`:
- `HealthProvider` - Health check information provider
- `HealthProviderConfig` - Health configuration
- `AuditLogger` - Audit logging interface
- `ComponentHealthStatus` - Health status representation

## Usage

### Creating Services
```go
import (
    "bib/internal/grpc/services/user"
    "bib/internal/grpc/services/topic"
)

// Create with default (empty) config
userServer := user.NewServer()

// Create with dependencies
userServer := user.NewServerWithConfig(user.Config{
    Store:       store,
    AuditLogger: auditMiddleware,
})
```

### Using Middleware
```go
import "bib/internal/grpc/middleware"

// Create interceptors
unaryInterceptors := []grpc.UnaryServerInterceptor{
    middleware.RecoveryUnaryInterceptor(),
    middleware.RequestIDUnaryInterceptor(),
    middleware.LoggingUnaryInterceptor(),
}
```

### Using Error Utilities
```go
import grpcerrors "bib/internal/grpc/errors"

// Map domain error to gRPC status
return nil, grpcerrors.MapDomainError(err)

// Create validation error
return nil, grpcerrors.NewValidationError("invalid request", map[string]string{
    "field": "error message",
})
```

## Migration Complete

All legacy files have been removed. The new structure is now the only implementation.
