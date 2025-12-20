# BIB API Documentation

This directory contains documentation for the BIB gRPC API.

## Overview

BIB (Bibliotheca) provides a comprehensive gRPC API for distributed research data management. The API is organized into several services, each handling a specific domain.

## Services

| Service | Description | Authentication |
|---------|-------------|----------------|
| [HealthService](./health-service.md) | Health checks and node info | Public |
| [AuthService](./auth-service.md) | Authentication and session management | Partial |
| [UserService](./user-service.md) | User management | Required |
| [TopicService](./topic-service.md) | Topic management | Required |
| [DatasetService](./dataset-service.md) | Dataset management | Required |
| [JobService](./job-service.md) | Job and task management | Required |
| [QueryService](./query-service.md) | Data querying | Required |
| [NodeService](./node-service.md) | P2P node management | Required |
| [AdminService](./admin-service.md) | Administrative operations | Admin only |
| [BreakGlassService](./breakglass-service.md) | Emergency access | Admin only |

## Authentication

BIB uses SSH key-based authentication with challenge-response:

1. Client sends public key to `/bib.v1.services.AuthService/Challenge`
2. Server returns a cryptographic challenge
3. Client signs the challenge with private key
4. Client sends signature to `/bib.v1.services.AuthService/VerifyChallenge`
5. Server validates and returns a session token

See [Authentication Flow](./auth-flow.md) for detailed documentation.

## Error Codes

BIB uses standard gRPC status codes with rich error details:

| Code | Meaning | Common Causes |
|------|---------|---------------|
| `OK` (0) | Success | - |
| `INVALID_ARGUMENT` (3) | Invalid request | Missing required fields, validation errors |
| `NOT_FOUND` (5) | Resource not found | Topic, dataset, user doesn't exist |
| `ALREADY_EXISTS` (6) | Duplicate resource | User with same key exists |
| `PERMISSION_DENIED` (7) | Access denied | Insufficient role, not owner |
| `UNAUTHENTICATED` (16) | Not authenticated | Missing/invalid session token |
| `UNAVAILABLE` (14) | Service unavailable | Dependency not ready |

See [Error Codes](./error-codes.md) for the complete reference.

## Protocol Buffers

Proto definitions are located in `api/proto/bib/v1/`.

### Generating Code

```bash
# Generate Go code
buf generate

# Lint proto files
buf lint

# Check breaking changes
buf breaking --against '.git#branch=main'
```

## Connection Options

### Local Connection (Unix Socket / Named Pipe)

```
# Linux/macOS
unix:///var/run/bibd.sock

# Windows
\\.\pipe\bibd
```

### TCP Connection

```
# Local
127.0.0.1:9090

# Remote with TLS
bibd.example.com:9090
```

### P2P Connection

```
# By peer ID
p2p:12D3KooWDpJ7As7BWAwRMfu1VU2WCqNjvq387JEYKDBj4kx6nXTN
```

## Version Information

- API Version: v1
- Minimum Client Version: 1.0.0
- Protocol: gRPC with Protocol Buffers 3

