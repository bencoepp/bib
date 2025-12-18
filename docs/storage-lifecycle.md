# Storage Lifecycle Management

This document describes how the bibd daemon manages storage backends (SQLite and PostgreSQL) through their complete lifecycle.

## Overview

The bibd daemon supports two storage backends:

1. **SQLite** - Simple, file-based storage ideal for single nodes and development
2. **PostgreSQL** - Managed, container-based PostgreSQL for production and high-availability setups

The storage lifecycle is designed to:
- Start asynchronously (don't block daemon startup)
- Fail fast on configuration errors
- Wait for readiness before marking the node as operational
- Validate configurations early
- Provide clear error messages for unsupported features

## Storage Startup Flow

### Phase 1: Initialization (Non-blocking)

When `daemon.Start()` is called, storage initialization happens in `startStorage()`:

```
startStorage()
├── Validate configuration (fail fast)
├── Validate mode/backend compatibility
├── If managed PostgreSQL:
│   ├── Validate runtime support (fail fast if Kubernetes)
│   ├── Create lifecycle manager
│   ├── Start container (async - don't wait)
│   └── Return immediately
└── If SQLite or external Postgres:
    ├── Open connection
    ├── Ping to verify (fail fast)
    └── Store is ready
```

**Key Points:**
- Configuration validation happens immediately
- Kubernetes runtime returns clear "not implemented" error
- Managed PostgreSQL container starts but we don't wait for readiness
- SQLite and external PostgreSQL are ready immediately after successful ping

### Phase 2: Component Startup

After storage initialization, other components start:
1. P2P networking (if enabled)
2. Raft cluster (if enabled)

These components do NOT depend on storage being fully ready.

### Phase 3: Readiness Check (Blocking)

Before the node becomes operational, `waitForStorageReady()` is called:

```
waitForStorageReady()
├── If PostgreSQL lifecycle manager exists:
│   ├── Wait for container to be ready (health checks pass)
│   ├── Extract connection info
│   ├── Open store connection
│   └── Run migrations
├── Verify store is not nil
└── Final health check (ping with timeout)
```

**Key Points:**
- This is the ONLY blocking step in storage startup
- We wait for PostgreSQL container health checks to pass
- Migrations run after connection is established
- Final ping ensures end-to-end connectivity

## Configuration

### SQLite Configuration

```yaml
database:
  backend: sqlite
  sqlite:
    path: ""  # Defaults to <data_dir>/cache.db
    max_open_conns: 10
```

### Managed PostgreSQL Configuration

```yaml
database:
  backend: postgres
  postgres:
    managed: true
    container_runtime: ""  # Auto-detect (docker > podman > kubernetes)
    socket_path: ""        # Auto-detect
    kubeconfig_path: ""    # For Kubernetes (not yet implemented)
    image: "postgres:16-alpine"
    data_dir: ""           # Defaults to <data_dir>/postgres
    port: 5432
    max_connections: 100
    memory_mb: 512
    cpu_cores: 1.0
    ssl_mode: "require"
    credential_rotation_interval: 168h  # 7 days
    
    # Network configuration
    network:
      use_bridge_network: true
      bridge_network_name: "bibd-network"
      use_unix_socket: true
      bind_address: "127.0.0.1"
    
    # Health check configuration
    health:
      interval: 5s
      timeout: 5s
      startup_timeout: 60s
      action: "retry_limit"  # shutdown, retry_always, retry_limit
      max_retries: 5
      retry_backoff: 10s
    
    # TLS configuration
    tls:
      enabled: true
      cert_dir: ""      # Defaults to <data_dir>/postgres/certs
      auto_generate: true
```

### External PostgreSQL Configuration (Testing Only)

```yaml
database:
  backend: postgres
  postgres:
    managed: false
    advanced:
      host: "localhost"
      port: 5432
      database: "bibd"
      user: "postgres"
      password: "secret"  # WARNING: Plaintext password
      ssl_mode: "disable"
```

## Container Runtime Detection

When `managed: true`, the lifecycle manager auto-detects the container runtime:

1. **Docker** (preferred)
   - Checks `/var/run/docker.sock`
   - Verifies with `docker info`

2. **Podman** (fallback)
   - Checks common socket locations
   - Verifies with `podman info`

3. **Kubernetes** (not implemented)
   - Checks `KUBERNETES_SERVICE_HOST` environment variable
   - Returns clear error: "kubernetes runtime is not yet implemented"

You can explicitly set the runtime:
```yaml
container_runtime: "docker"  # or "podman"
```

## Health Checks

The lifecycle manager continuously monitors PostgreSQL health:

### Startup Health Checks
- Interval: Every 1 second during startup
- Timeout: Configured `health.startup_timeout` (default 60s)
- Command: `pg_isready -U postgres`

### Runtime Health Checks
- Interval: Configured `health.interval` (default 5s)
- Timeout: Configured `health.timeout` (default 5s)
- Action on failure: Configured `health.action`

### Health Actions

1. **shutdown** - Shutdown bibd immediately on health check failure
2. **retry_always** - Keep retrying forever, restart container if needed
3. **retry_limit** - Retry up to `max_retries` times, then give up

## Error Handling

### Fail Fast Scenarios

The daemon fails immediately on:
- Invalid configuration (missing required fields, invalid values)
- Incompatible mode/backend combinations (e.g., full mode with SQLite)
- Kubernetes runtime (not yet implemented)
- Storage ping failure (for non-managed backends)

### Clear Error Messages

```
✗ kubernetes runtime is not yet implemented; please use 'docker' or 'podman'
✗ incompatible mode and backend: full replica mode requires PostgreSQL backend
✗ invalid storage configuration: port must be between 1024 and 65535
✗ storage ping failed: connection refused
```

## Shutdown Flow

When `daemon.Stop()` is called, storage shuts down in reverse order:

```
stopStorage()
├── Close store connection
└── If PostgreSQL lifecycle manager exists:
    ├── Stop container
    └── Clean up resources
```

**Key Points:**
- Store connection closes before container stops
- Clean shutdown prevents data corruption
- Timeout of 30 seconds for PostgreSQL shutdown

## Mode/Backend Compatibility

| Node Mode  | SQLite | PostgreSQL |
|------------|--------|------------|
| proxy      | ✓      | ✓          |
| selective  | ✓      | ✓          |
| full       | ✗      | ✓          |

**Note:** SQLite cannot be authoritative, so full replica mode requires PostgreSQL.

## Security

> 📖 For comprehensive security documentation, see [Database Security & Hardening](database-security.md).

### Credential Management

For managed PostgreSQL:
- Passwords are generated with 64 bytes of cryptographic randomness
- Separate roles for different permissions (admin, scrape, query, transform, audit, readonly)
- Automatic credential rotation (configurable interval, default 7 days)
- Credentials encrypted at rest using node identity key
- Three encryption methods: X25519, HKDF, or Hybrid (default)
- Zero-downtime rotation with dual credential sets

### Role-Based Access Control

bibd uses per-transaction role switching for minimal privilege:

| Role | Purpose | Key Permissions |
|------|---------|-----------------|
| `bibd_admin` | Connection pool | Can SET ROLE to all others |
| `bibd_scrape` | Data ingestion | INSERT on datasets, chunks |
| `bibd_query` | Read operations | SELECT on all tables |
| `bibd_transform` | Data transformation | SELECT, INSERT, UPDATE |
| `bibd_audit` | Audit logging | INSERT, SELECT on audit_log |
| `bibd_readonly` | Cache/proxy | SELECT only |

### Network Security

- **Unix Socket Mode** (default on Linux): No network exposure, socket-only access
- **TCP Mode** (macOS/Windows): Binds to localhost only, mTLS required
- **Bridge Network**: Isolated Docker network with `--internal` flag
- **Kubernetes**: NetworkPolicy restricts access to bibd pods only

### Encryption at Rest

Multiple encryption options supported:
- **Application-level**: AES-256-GCM encryption of sensitive fields
- **LUKS** (Linux): Full disk encryption with dm-crypt
- **Shamir's Secret Sharing**: Key recovery with threshold shares

### Certificate Management

When `tls.auto_generate: true`:
- CA certificate generated from node identity
- Server certificates signed by CA
- Client certificates for mTLS authentication
- Automatic rotation before expiry
- Certificates stored in `<data_dir>/postgres/certs`

## Troubleshooting

### Container Won't Start

```bash
# Check Docker/Podman is running
docker info
# or
podman info

# Check container logs
docker logs bibd-postgres-<node-id>
# or
podman logs bibd-postgres-<node-id>

# Check if port is already in use
netstat -an | grep 5432
```

### Health Checks Failing

```yaml
# Increase startup timeout
health:
  startup_timeout: 120s  # 2 minutes
  
# Check container is running
docker ps | grep bibd-postgres
```

### Connection Issues

```bash
# Test Unix socket connection
psql -h /path/to/data/postgres/run -U postgres -d bibd

# Test TCP connection
psql -h 127.0.0.1 -p 5432 -U postgres -d bibd
```

## Future Enhancements

### Kubernetes Support

Planned support for Kubernetes StatefulSets:
- Auto-generated PersistentVolumeClaims
- Service discovery via DNS
- Rolling updates with zero downtime
- Integration with Kubernetes secrets

### ~~Credential Encryption~~ ✅ Implemented

~~Planned enhancement to encrypt credentials at rest:~~
- ✅ Use node identity key for encryption
- ✅ Support multiple encryption methods (X25519, HKDF, Hybrid)
- ✅ Automatic credential rotation
- 🔄 External KMS support (AWS KMS, HashiCorp Vault) - Future

### ~~Role-Based Access~~ ✅ Implemented

~~Planned enhancement for fine-grained access control:~~
- ✅ Per-transaction role switching
- ✅ Minimal privilege roles (scrape, query, transform, audit, readonly)
- ✅ SET LOCAL ROLE per transaction

### ~~Encryption at Rest~~ ✅ Implemented

~~Planned enhancement for data protection:~~
- ✅ Application-level field encryption (AES-256-GCM)
- ✅ LUKS volume encryption (Linux)
- ✅ Shamir's Secret Sharing for key recovery

### Backup and Recovery

Planned features:
- Automatic periodic backups
- Point-in-time recovery
- Backup to S3/object storage
- Backup encryption

## See Also

- [Database Security & Hardening](database-security.md) - Comprehensive security documentation
- [Configuration Guide](configuration.md) - Complete configuration reference
- [Node Modes](node-modes.md) - Understanding mode/storage requirements
- [Developer Guide](developer-guide.md) - Storage package development guide
- [Phase 2.3 Implementation Outline](phase-2.3-implementation-outline.md) - Technical implementation details

