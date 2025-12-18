# Database Lifecycle Management

This document describes the complete lifecycle management for bibd's database backends, including initialization, backup/recovery, and graceful shutdown.

---

## Overview

The database lifecycle in bibd consists of three main phases:

1. **Initialization** - Setting up the database and preparing it for use
2. **Operations** - Normal runtime operations with backup management
3. **Shutdown** - Graceful cleanup ensuring data consistency

All lifecycle operations are designed to:
- Fail fast on configuration errors
- Provide clear error messages
- Ensure data consistency
- Support both SQLite and PostgreSQL backends

---

## Phase 1: Initialization Workflow (DB-022)

### Startup Sequence

When `bibd` starts, the initialization follows this sequence:

```
┌─────────────────────────────────────────────────────────────┐
│                 Database Initialization Flow                 │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  1. Load Configuration                                       │
│     ├── Validate database.backend (sqlite|postgres)         │
│     ├── Validate mode compatibility (full requires PG)      │
│     └── Check runtime requirements (Docker/Podman/K8s)      │
│                                                              │
│  2. Initialize Storage Backend                               │
│     ├── SQLite:                                              │
│     │   ├── Create data directory                           │
│     │   ├── Open database file                              │
│     │   └── Ping to verify                                  │
│     │                                                        │
│     └── PostgreSQL (Managed):                               │
│         ├── Create lifecycle manager                        │
│         ├── Generate/load credentials                       │
│         ├── Generate TLS certificates (if enabled)          │
│         ├── Provision container/pod                         │
│         └── Start asynchronously (don't wait)               │
│                                                              │
│  3. Start Other Components (P2P, Cluster)                    │
│     └── These start in parallel with PostgreSQL provisioning│
│                                                              │
│  4. Wait for Storage Ready                                   │
│     ├── If PostgreSQL: wait for health checks to pass       │
│     ├── Connect to database                                 │
│     ├── Run pending migrations                              │
│     └── Verify with final ping                              │
│                                                              │
│  5. Mark Node as Ready                                       │
│     └── Begin accepting requests                            │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Configuration Validation

Before any resources are created, the configuration is validated:

```yaml
# Example configuration
database:
  backend: postgres  # or sqlite
  postgres:
    managed: true    # bibd manages PostgreSQL lifecycle
    container_runtime: ""  # Auto-detect (docker > podman > kubernetes)
    image: "postgres:16-alpine"
    port: 5432
```

**Validation Rules:**

| Mode | Backend | Valid | Notes |
|------|---------|-------|-------|
| full | postgres | ✅ Yes | Preferred configuration |
| full | sqlite | ❌ No | SQLite cannot be authoritative |
| selective | postgres | ✅ Yes | Full features available |
| selective | sqlite | ⚠️  Limited | Cache-only, no data distribution |
| proxy | postgres | ✅ Yes | Works but wasteful |
| proxy | sqlite | ✅ Yes | Minimal resource usage |

### Error Handling

Initialization follows a **fail-fast** approach:

```go
// Example error handling
if err := cfg.Validate(); err != nil {
    log.Error("invalid configuration", "error", err)
    os.Exit(1)
}

if err := ValidateModeBackend(mode, backend); err != nil {
    log.Error("incompatible mode and backend", "error", err)
    os.Exit(1)
}
```

**Common Errors:**

1. **Invalid backend**: Unknown backend type specified
2. **Mode mismatch**: Full mode with SQLite backend
3. **Runtime unavailable**: Container runtime not found
4. **Port conflict**: PostgreSQL port already in use
5. **Permission denied**: Cannot create data directory

### Migration Execution

After the database is connected, migrations run automatically:

```
Migrations Flow:
├── Check current schema version
├── Lock migrations table (prevent concurrent runs)
├── For each pending migration:
│   ├── Begin transaction
│   ├── Execute migration SQL
│   ├── Update schema version
│   └── Commit transaction
└── Verify final checksum
```

**Migration Safety:**

- All migrations run in transactions (PostgreSQL)
- Failed migrations marked as "dirty" and auto-recovered
- Checksums verify migration integrity
- Downgrade migrations preserve data

### Health Checks

PostgreSQL readiness is determined by health checks:

```go
// Health check parameters
Interval:       5 * time.Second
Timeout:        5 * time.Second
StartupTimeout: 60 * time.Second
MaxRetries:     5
RetryBackoff:   10 * time.Second
```

Health check validates:
- Container is running
- PostgreSQL is accepting connections
- Database `bibd` exists
- Basic query succeeds

---

## Phase 2: Backup & Recovery (DB-023)

### Backup System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Backup Architecture                      │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Backup Manager                                              │
│  ├── Configuration (schedule, retention, location)          │
│  ├── Metadata Store (.metadata/*.json)                      │
│  └── Storage Backend                                         │
│      ├── Local Filesystem (default)                         │
│      └── S3-Compatible Storage (optional)                   │
│                                                              │
│  Backup Types                                                │
│  ├── PostgreSQL                                              │
│  │   ├── Full Backup (pg_dump)                              │
│  │   ├── WAL Archiving (PITR)                               │
│  │   └── Snapshot                                            │
│  └── SQLite                                                  │
│      └── File Copy (backup API)                             │
│                                                              │
│  Features                                                    │
│  ├── Compression (gzip, configurable)                       │
│  ├── Encryption (node identity-based)                       │
│  ├── Integrity Verification (SHA-256)                       │
│  └── Automatic Cleanup (retention policy)                   │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Backup Configuration

```yaml
database:
  backup:
    enabled: true
    schedule: "0 2 * * *"  # Daily at 2 AM (cron format)
    location: local        # or s3
    local_path: ""         # Defaults to <data_dir>/backups
    compression: true
    encryption: true
    retention_days: 30
    max_backups: 7
    wal_archiving: false   # PostgreSQL PITR
    verify_after_backup: true
```

### Creating Backups

**Manual Backup:**

```bash
# Create a backup with notes
bib admin backup create --notes "Before upgrade to v0.2.0"

# Create without verification (faster)
bib admin backup create --no-verify
```

**Automatic Backups:**

Scheduled backups run automatically when enabled:

```
Daily Backup Flow:
├── Trigger at configured time (cron)
├── Generate unique backup ID
├── Execute backend-specific backup
│   ├── PostgreSQL: pg_dump with custom format
│   └── SQLite: Backup API with file copy
├── Compress backup file (if enabled)
├── Encrypt backup file (if enabled)
├── Calculate SHA-256 hash
├── Save metadata
├── Verify integrity
└── Clean up old backups (retention policy)
```

### Backup Metadata

Each backup stores comprehensive metadata:

```json
{
  "id": "1734540120000000000",
  "timestamp": "2025-12-18T14:22:00Z",
  "backend": "postgres",
  "format": "pg_dump",
  "size": 10485760,
  "compressed": true,
  "encrypted": true,
  "node_id": "node-abc123",
  "version": "PostgreSQL 16.1",
  "wal_position": "0/1234ABCD",
  "location": "local",
  "path": "/data/backups/node-abc123_1734540120_20251218_142200.sql.gz",
  "integrity_hash": "sha256:abc123...",
  "notes": "Before upgrade to v0.2.0"
}
```

### Listing Backups

```bash
# List all available backups
bib admin backup list

# Output:
# ID                  TIMESTAMP           BACKEND    FORMAT    SIZE      NOTES
# --                  ---------           -------    ------    ----      -----
# 1734540120000000000 2025-12-18 14:22    postgres   pg_dump   10.0 MB   Before upgrade
# 1734453720000000000 2025-12-17 14:22    postgres   pg_dump   9.8 MB    Daily backup
# 1734367320000000000 2025-12-16 14:22    postgres   pg_dump   9.5 MB    Daily backup
```

### Restoring Backups

**Prerequisites:**
- Stop the bibd daemon before restoring
- Ensure enough disk space
- Have admin access

**Restore Command:**

```bash
# Restore from a specific backup (with confirmation)
bib admin restore 1734540120000000000

# Force restore without confirmation
bib admin restore 1734540120000000000 --force

# Verify backup integrity before restore
bib admin restore 1734540120000000000 --verify

# Point-in-time recovery (PostgreSQL with WAL archiving)
bib admin restore 1734540120000000000 --target-time "2025-12-18T14:30:00Z"
```

**Restore Flow:**

```
Restore Process:
├── Load backup metadata
├── Verify backup integrity (if --verify)
├── Check if database has data
│   └── Require --force if data exists
├── Stop accepting connections
├── Execute backend-specific restore
│   ├── PostgreSQL: pg_restore --clean --if-exists
│   └── SQLite: File copy with atomic rename
├── Verify restore success
└── Prompt user to restart bibd
```

### Backup Retention

Backups are automatically cleaned up based on retention policy:

```go
// Cleanup rules (both must be satisfied)
RetentionDays: 30   // Delete backups older than 30 days
MaxBackups: 7       // Keep only the 7 most recent backups
```

Cleanup runs:
- After each successful backup
- During `bib admin backup list` (lazy cleanup)
- Can be triggered manually with `bib admin cleanup --backups`

### Point-in-Time Recovery (PITR)

For PostgreSQL with WAL archiving enabled:

```yaml
database:
  backup:
    wal_archiving: true
```

This enables:
- Continuous WAL archiving to backup location
- Restore to any point in time between backups
- Minimal data loss in disaster scenarios

**PITR Workflow:**

```
PITR Setup:
├── Enable WAL archiving in PostgreSQL config
├── Configure archive_command to copy WAL files
├── Backups include base backup + WAL position
├── WAL files continuously archived
└── Restore replays WAL files to target time
```

### Disaster Recovery

**Complete System Failure:**

1. Install fresh bibd on new system
2. Copy backup files to new system
3. Restore from backup:
   ```bash
   bib admin restore <backup-id> --force
   ```
4. Start bibd daemon
5. Verify data integrity

**Backup Verification:**

```bash
# List backups and check integrity
bib admin backup list

# Each backup shows:
# - Integrity hash (SHA-256)
# - Size
# - Location

# Backups are automatically verified on restore
```

---

## Phase 3: Graceful Shutdown (DB-024)

### Shutdown Sequence

When bibd receives a shutdown signal (SIGTERM, SIGINT), it follows this sequence:

```
┌─────────────────────────────────────────────────────────────┐
│                   Graceful Shutdown Flow                     │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  1. Receive Shutdown Signal                                  │
│     ├── SIGTERM (default)                                    │
│     ├── SIGINT (Ctrl+C)                                      │
│     └── OS shutdown                                          │
│                                                              │
│  2. Stop Accepting New Requests                              │
│     ├── Mark node as shutting down                          │
│     ├── Return 503 Service Unavailable to new requests      │
│     └── Keep existing connections open                       │
│                                                              │
│  3. Drain Active Connections                                 │
│     ├── Wait for in-flight operations to complete           │
│     ├── Timeout: 30 seconds (configurable)                  │
│     └── Log warning for operations forced to stop           │
│                                                              │
│  4. Stop Components (Reverse Order)                          │
│     ├── Stop cluster consensus (if enabled)                 │
│     ├── Stop P2P networking                                 │
│     └── Stop storage backend                                │
│                                                              │
│  5. Storage Shutdown                                         │
│     ├── Close database connections cleanly                  │
│     ├── Complete pending transactions                       │
│     ├── Perform checkpoint (PostgreSQL)                     │
│     └── Sync data to disk                                   │
│                                                              │
│  6. PostgreSQL Lifecycle Cleanup                             │
│     ├── Send CHECKPOINT command                             │
│     ├── Stop PostgreSQL gracefully (30s timeout)            │
│     ├── Verify clean shutdown                               │
│     └── Leave container running (for manual inspection)     │
│         OR stop container (based on config)                 │
│                                                              │
│  7. Cleanup & Exit                                           │
│     ├── Remove PID file                                      │
│     ├── Flush logs                                           │
│     ├── Report final status                                 │
│     └── Exit with status code                               │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Configuration

```yaml
server:
  shutdown_timeout: 30s       # Max time to wait for graceful shutdown
  drain_timeout: 10s          # Max time to drain connections

database:
  postgres:
    stop_on_shutdown: false   # Keep container running (false) or stop (true)
    checkpoint_on_shutdown: true  # Run CHECKPOINT before stop
```

### Shutdown Types

**Normal Shutdown (SIGTERM):**
```bash
# Graceful shutdown with full cleanup
kill <bibd-pid>

# Or via systemd
systemctl stop bibd
```

**Forced Shutdown (SIGKILL):**
```bash
# Immediate termination (not recommended)
kill -9 <bibd-pid>
```

**Emergency Shutdown:**
```bash
# Faster shutdown, skip some cleanup
bib admin shutdown --force
```

### Connection Draining

During shutdown, bibd drains active connections:

```go
// Connection draining logic
func (d *Daemon) drainConnections() {
    timeout := d.cfg.Server.DrainTimeout
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    
    // Wait for active operations
    d.waitForOperations(ctx)
    
    // Close idle connections
    d.closeIdleConnections()
}
```

**Behavior:**
- gRPC connections: Return `Unavailable` status
- HTTP connections: Return `503 Service Unavailable`
- Active operations: Complete with timeout
- New requests: Immediately rejected

### PostgreSQL Checkpoint

Before stopping PostgreSQL, a CHECKPOINT is performed:

```sql
-- Checkpoint command (issued by bibd)
CHECKPOINT;

-- This ensures:
-- - All dirty buffers written to disk
-- - WAL is synchronized
-- - Data consistency guaranteed
```

**Checkpoint Timeout:**
- Default: 5 seconds
- If timeout: Log warning and proceed
- Non-fatal: Shutdown continues even if checkpoint fails

### Container Management

**Keep Container Running (default):**
```yaml
database:
  postgres:
    stop_on_shutdown: false
```

Benefits:
- Manual inspection possible
- Faster restart
- Troubleshooting easier

**Stop Container on Shutdown:**
```yaml
database:
  postgres:
    stop_on_shutdown: true
```

Benefits:
- Clean environment
- Resource release
- Consistent state

### Cleanup Command

For complete removal of bibd resources:

```bash
# Remove everything (interactive confirmation)
bib cleanup --all

# Remove only PostgreSQL containers
bib cleanup --postgres

# Remove backups and logs
bib cleanup --backups --logs

# Force cleanup without confirmation
bib cleanup --all --force

# Clean up specific container
bib cleanup --container bibd-postgres-abc123
```

**Cleanup Phases:**

```
Cleanup Process:
├── Stop bibd daemon (if running)
├── Stop and remove PostgreSQL containers
├── Remove PostgreSQL data volumes
├── Delete backup files
├── Delete log files
├── Delete cache files (SQLite)
├── Remove configuration files (with --all)
└── Remove data directory (with --all)
```

### Recovery from Unclean Shutdown

If bibd crashes or is force-killed:

**PostgreSQL (Managed):**
- Container continues running
- PostgreSQL auto-recovery on restart
- WAL replay ensures consistency
- No manual intervention needed

**SQLite:**
- WAL mode enabled by default
- Automatic journal recovery
- Rare cases may need `PRAGMA integrity_check`

**Detection:**

```bash
# Check for dirty migrations
bib admin status

# Output may show:
# Migration State: dirty (version 5)
# Action: Run migrations to auto-recover
```

**Recovery:**

```bash
# Automatic recovery on restart
bibd

# Or manual migration reset
bib admin migrate --reset
```

---

## Monitoring & Troubleshooting

### Health Checks

```bash
# Check overall daemon health
bib admin health

# Check database connectivity
bib admin db ping

# View current operations
bib admin ops list
```

### Logs

```bash
# View daemon logs
tail -f /var/log/bibd/bibd.log

# View audit logs
tail -f /var/log/bibd/audit.log

# Filter for storage events
grep "storage" /var/log/bibd/bibd.log
```

### Common Issues

**Issue: Initialization hangs**
- Check PostgreSQL container status: `docker ps | grep bibd-postgres`
- Check logs: `docker logs bibd-postgres-<node-id>`
- Increase `StartupTimeout` in config

**Issue: Shutdown timeout**
- Check for long-running operations
- Increase `shutdown_timeout`
- Use `--force` for emergency shutdown

**Issue: Backup fails**
- Check disk space: `df -h`
- Verify PostgreSQL connectivity
- Check backup directory permissions

**Issue: Restore fails**
- Ensure daemon is stopped
- Verify backup integrity: `--verify`
- Check backup format matches backend

---

## Best Practices

### Initialization
- ✅ Use managed PostgreSQL for production
- ✅ Enable TLS for all connections
- ✅ Run migrations automatically
- ✅ Verify health checks pass
- ❌ Don't use SQLite for full replica mode

### Backup & Recovery
- ✅ Enable automated backups
- ✅ Test restore process regularly
- ✅ Store backups off-system (S3)
- ✅ Enable WAL archiving for PostgreSQL
- ✅ Document disaster recovery procedure
- ❌ Don't rely solely on local backups
- ❌ Don't skip backup verification

### Graceful Shutdown
- ✅ Use SIGTERM (not SIGKILL)
- ✅ Allow sufficient drain timeout
- ✅ Monitor shutdown logs
- ✅ Enable checkpoint on shutdown
- ❌ Don't force kill unless necessary
- ❌ Don't skip cleanup steps

---

## Implementation Status

| Feature | Status | Notes |
|---------|--------|-------|
| DB-022: Initialization | ✅ Complete | Fully implemented and tested |
| DB-023: Backup & Recovery | ✅ Complete | CLI commands and manager implemented |
| DB-024: Graceful Shutdown | ✅ Complete | Enhanced with draining and checkpoint |
| Automatic backups | 🚧 Partial | Scheduler integration pending |
| WAL archiving | 📋 Planned | PostgreSQL PITR support |
| S3 backup storage | 📋 Planned | S3-compatible backend |

---

## Future Enhancements

1. **Automated Backup Scheduling**
   - Cron-based scheduler
   - Integration with system cron or internal scheduler

2. **Point-in-Time Recovery**
   - WAL archiving to S3/local
   - Replay to specific timestamp

3. **Backup Compression Options**
   - Multiple compression algorithms (gzip, zstd, lz4)
   - Configurable compression levels

4. **Incremental Backups**
   - Delta backups for large databases
   - Reduced storage requirements

5. **Backup Replication**
   - Multi-region backup copies
   - Cross-cloud replication

6. **Automated Recovery Testing**
   - Periodic restore verification
   - Backup health scoring

---

*Last Updated: 2025-12-18*
*Version: 0.1.0*

