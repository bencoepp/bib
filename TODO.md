# BIB Development Roadmap

> A distributed research, analysis and management tool for handling all types of research data.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              BIB ECOSYSTEM                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│   ┌──────────────┐         ┌──────────────────────────────────────────┐     │
│   │   bib CLI    │◄───────►│              bibd daemon                 │     │
│   │  (bubbletea) │  gRPC   │                                          │     │
│   └──────────────┘  mTLS   │  ┌────────────┐  ┌───────────────────┐   │     │
│                            │  │  P2P Layer │  │    Scheduler      │   │     │
│   ┌──────────────┐         │  │  (libp2p)  │  │  (CEL + Workers)  │   │     │
│   │  Wish SSH    │◄───────►│  └────────────┘  └───────────────────┘   │     │
│   │   Server     │         │                                          │     │
│   └──────────────┘         │  ┌────────────┐  ┌───────────────────┐   │     │
│                            │  │  Storage   │  │   Web Interface   │   │     │
│   ┌──────────────┐         │  │  Manager   │  │   (Read + More)   │   │     │
│   │  Bootstrap   │         │  └─────┬──────┘  └───────────────────┘   │     │
│   │   bib.dev    │         │        │                                 │     │
│   └──────────────┘         └────────┼─────────────────────────────────┘     │
│                                     │                                        │
│                    ┌────────────────┴─────────────────┐                     │
│                    │      SECURITY BOUNDARY           │                     │
│                    │  ┌─────────────────────────────┐ │                     │
│                    │  │  Managed PostgreSQL         │ │                     │
│                    │  │  (Container/K8s Pod)        │ │                     │
│                    │  │  - No external access       │ │                     │
│                    │  │  - Auto-managed credentials │ │                     │
│                    │  │  - Full audit logging       │ │                     │
│                    │  └─────────────────────────────┘ │                     │
│                    │           OR (limited mode)      │                     │
│                    │  ┌─────────────────────────────┐ │                     │
│                    │  │  SQLite (Proxy/Cache Only)  │ │                     │
│                    │  │  - No authoritative data    │ │                     │
│                    │  │  - Cannot distribute data   │ │                     │
│                    │  └─────────────────────────────┘ │                     │
│                    └──────────────────────────────────┘                     │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Node Modes

| Mode | Description | Storage | Data Sync | Required Backend |
|------|-------------|---------|-----------|------------------|
| **Full Replica** | Replicates all topics/datasets from configured base nodes | Full local | Continuous | PostgreSQL (managed) |
| **Selective** | Choose which datasets/topics to sync on-demand | Partial local | On-demand | PostgreSQL preferred, SQLite (cache only) |
| **Proxy** | No local storage, forwards requests to other nodes | None/Cache | Pass-through | SQLite or PostgreSQL |

> **Note**: SQLite nodes cannot be authoritative data distributors. Only PostgreSQL-backed nodes 
> running in `full` or `selective` mode can serve as trusted data sources in the P2P network.

---

## Phase 1: P2P Networking Foundation

### 1.1 Core libp2p Integration
- [x] **P2P-001**: Set up libp2p host with identity management
  - Generate/persist node identity (Ed25519 keypair)
  - Configure listen addresses (TCP, QUIC, WebSocket)
  - Store identity in config/secrets
- [x] **P2P-002**: Implement transport layer
  - TCP transport with Noise encryption
  - QUIC transport for improved performance
  - Optional WebSocket for browser-based access (future)
- [x] **P2P-003**: Configure multiplexing
  - Yamux multiplexer
  - Connection manager with limits

### 1.2 Node Discovery & Bootstrap
- [x] **P2P-004**: Bootstrap node configuration
  - Hardcode bib.dev as default bootstrap
  - Allow additional bootstrap nodes in config
  - Implement bootstrap connection logic
- [x] **P2P-005**: mDNS local discovery
  - Enable for local network discovery
  - Configurable on/off
- [x] **P2P-006**: DHT implementation
  - Kademlia DHT for peer discovery
  - Content routing for data location
  - Provider records for dataset availability
- [x] **P2P-007**: Peer store & connection management
  - Persistent peer store (badger/sqlite)
  - Connection pruning strategies
  - Peer reputation/scoring (basic)

### 1.3 Node Modes Implementation
- [x] **P2P-008**: Node mode configuration
  - Config option: `mode: full | selective | proxy`
  - Mode-specific initialization
  - Hot config reload for runtime mode switching
- [x] **P2P-009**: Full replica mode
  - Topic discovery protocol
  - Automatic subscription to all topics
  - Full dataset replication logic (periodic polling)
- [x] **P2P-010**: Selective mode
  - Topic/dataset catalog sync
  - On-demand subscription mechanism
  - Partial replication with metadata
  - Persistent subscriptions across restarts
- [x] **P2P-011**: Proxy mode
  - Request forwarding to known peers
  - Response caching (configurable TTL, in-memory)
  - Favorite peers configuration
  - No persistent storage

### 1.4 P2P Protocols
- [x] **P2P-012**: Define custom protocols
  - `/bib/discovery/1.0.0` - Node & dataset discovery
  - `/bib/data/1.0.0` - Data transfer protocol
  - `/bib/jobs/1.0.0` - Job distribution protocol (placeholder)
  - `/bib/sync/1.0.0` - State synchronization
  - Multi-version support
  - Protobuf message definitions
- [x] **P2P-013**: PubSub for real-time updates
  - GossipSub for topic-based messaging
  - Topic structure: `/bib/global`, `/bib/nodes`, `/bib/topics/<topic-id>`
  - Message validation & signing
  - Timestamp freshness validation
- [x] **P2P-014**: Data transfer
  - Chunked transfer for large datasets
  - Resumable downloads with bitmap tracking
  - Integrity verification (SHA-256)
  - Parallel downloads from multiple peers
  - Configurable chunk size

### 1.5 Raft Consensus (Optional HA Mode)
- [x] **P2P-015**: Raft integration for HA clusters
  - etcd/raft for consensus
  - Leader election for coordinator nodes
  - Log replication for critical metadata (catalog, jobs, config)
  - SQLite-backed storage for logs/snapshots
  - FSM for catalog, job, and config replication
- [x] **P2P-016**: Cluster membership
  - Join/leave cluster operations via `bib setup --cluster` and `--cluster-join`
  - Join token generation with 24h expiry
  - Voters + Non-voters support (minimum 3 voters)
  - Snapshot & restore (automatic and manual)
  - Simple split-brain protection (leader steps down without quorum)

---

## Phase 2: Storage Layer

> **Security Architecture**: The database is a critical security boundary. bibd MUST be the sole manager 
> of its database. No external access is permitted by default. All data operations flow through bibd's 
> gRPC API, ensuring audit trails, access control, and data integrity.

### 2.0 Storage Mode Constraints

| Storage Backend | Allowed Node Modes | Rationale |
|-----------------|-------------------|-----------|
| **SQLite** | `proxy`, `selective` (cache only) | SQLite cannot be hardened; node cannot be trusted data distributor |
| **PostgreSQL (managed)** | `full`, `selective`, `proxy` | Fully hardened, audited, secure |

- [x] **DB-000**: Storage mode enforcement
  - SQLite mode restricts node to proxy/selective-cache only
  - SQLite nodes marked as `untrusted-storage` in peer metadata
  - Full replica mode REQUIRES managed PostgreSQL
  - Startup validation: reject `mode: full` with SQLite backend
  - Clear error messages explaining security rationale

### 2.1 Database Abstraction
- [x] **DB-001**: Database interface design
  - Define repository interfaces with permission contexts
  - Support both SQLite (limited) and PostgreSQL (full)
  - Connection pooling (pgx for Postgres)
  - All queries tagged with job/operation context for audit
- [x] **DB-002**: SQLite embedded mode (Proxy/Cache Only)
  - Single-file database for cache/metadata only
  - WAL mode for concurrency
  - Auto-vacuum configuration
  - **Limitations enforced:**
    - No authoritative data storage
    - Cache TTL with automatic expiration
    - Cannot serve data to other peers (pass-through only)
    - Marked as non-authoritative in DHT provider records
- [x] **DB-003**: PostgreSQL managed mode (Preferred)
  - bibd manages PostgreSQL lifecycle entirely
  - No external connection strings accepted by default
  - mTLS between bibd and PostgreSQL
  - Health checks & automatic recovery

### 2.2 Managed PostgreSQL (Container/Kubernetes)

> **Principle**: bibd owns and operates its PostgreSQL instance. External management is prohibited.
> The database is an internal implementation detail, not an externally accessible service.

- [x] **DB-004**: Container-managed PostgreSQL (Docker/Podman)
  - bibd automatically provisions PostgreSQL container on startup
  - Container naming: `bibd-postgres-<node-id-short>`
  - Automatic container lifecycle (start/stop/restart with bibd)
  - Volume management for persistence (`<data_dir>/postgres/`)
  - Container health monitoring and auto-restart
  - PostgreSQL version pinning with security updates
  - Resource limits (memory, CPU) configurable
  - Automatic cleanup on `bibd cleanup` command
  
- [x] **DB-005**: Kubernetes PostgreSQL deployment
  - bibd creates/manages PostgreSQL StatefulSet
  - Runs in same namespace as bibd (configurable)
  - PersistentVolumeClaim for data durability
  - Pod anti-affinity with bibd for resilience
  - Automatic backup CronJob creation
  - NetworkPolicy restricting access to bibd pod only
  - ServiceAccount with minimal RBAC permissions
  - In-cluster and out-of-cluster support with auto-detection
  - Service type (ClusterIP/NodePort) auto-detection
  - CloudNativePG operator support (planned)
  - Automatic fallback to Docker/Podman if Kubernetes fails
  - Comprehensive configuration options for security and resources

### 2.3 Database Security & Hardening

> **Zero Trust Database Access**: The PostgreSQL instance is invisible to everything except bibd.
> All credentials are generated, rotated, and never exposed. Every query is audited.
>
> 📋 **Documentation**: See [docs/database-security.md](docs/database-security.md)

- [x] **DB-006**: Credential management
  - bibd generates all PostgreSQL credentials at initialization
  - Superuser password: 64-char random, never logged, never exposed
  - Credentials stored encrypted in `<config_dir>/secrets/db.enc`
  - Encryption key derived from node identity (Ed25519)
  - Automatic credential rotation (configurable interval, default 7 days)
  - Rotation is zero-downtime (create new role, migrate, drop old)
  - Credentials never appear in config files, logs, or error messages

- [x] **DB-007**: Role-based database access (per job type)
  ```sql
  -- bibd creates these roles automatically
  -- Superuser (bibd internal only, never exposed)
  CREATE ROLE bibd_admin WITH LOGIN SUPERUSER PASSWORD '<generated>';
  
  -- Job-specific roles with minimal permissions
  CREATE ROLE bibd_scrape WITH LOGIN PASSWORD '<generated>';
  GRANT INSERT ON datasets, chunks TO bibd_scrape;
  GRANT SELECT ON topics TO bibd_scrape;  -- Read topic config only
  
  CREATE ROLE bibd_query WITH LOGIN PASSWORD '<generated>';
  GRANT SELECT ON datasets, chunks, topics TO bibd_query;
  
  CREATE ROLE bibd_transform WITH LOGIN PASSWORD '<generated>';
  GRANT SELECT, INSERT, UPDATE ON datasets, chunks TO bibd_transform;
  
  CREATE ROLE bibd_admin_jobs WITH LOGIN PASSWORD '<generated>';
  GRANT ALL ON ALL TABLES TO bibd_admin_jobs;  -- For migrations, maintenance
  
  CREATE ROLE bibd_audit WITH LOGIN PASSWORD '<generated>';
  GRANT INSERT ON audit_log TO bibd_audit;
  GRANT SELECT ON audit_log TO bibd_audit;  -- For audit queries
  ```
  - Each job execution uses appropriate role
  - Role selection based on job type at runtime
  - Connection pool per role for isolation

- [x] **DB-008**: Network isolation
  - **Docker/Podman:**
    - PostgreSQL binds to Unix socket only (no TCP by default)
    - If TCP required: bind to `127.0.0.1` only
    - Custom Docker network with no external access
    - Container has no published ports
  - **Kubernetes:**
    - NetworkPolicy: ingress only from bibd pod label
    - No Service exposure (ClusterIP: None or headless)
    - Pod-to-pod communication via pod IP only
    - Optional: Istio/Linkerd mTLS sidecar
  - Firewall rules managed by bibd where possible

- [x] **DB-009**: PostgreSQL hardening configuration
  ```ini
  # bibd generates and manages postgresql.conf
  listen_addresses = ''  # Unix socket only, or '127.0.0.1' if needed
  ssl = on
  ssl_cert_file = '/var/lib/bibd/certs/server.crt'
  ssl_key_file = '/var/lib/bibd/certs/server.key'
  ssl_ca_file = '/var/lib/bibd/certs/ca.crt'
  
  # Authentication
  password_encryption = scram-sha-256
  
  # Logging for audit
  log_statement = 'all'
  log_connections = on
  log_disconnections = on
  log_duration = on
  
  # Restrict dangerous operations
  shared_preload_libraries = 'pg_stat_statements'
  ```
  - pg_hba.conf generated to only allow bibd roles
  - No `trust` authentication ever
  - Certificate-based auth for all connections

- [x] **DB-010**: Encryption at rest
  - PostgreSQL data directory encryption
  - Option 1: LUKS/dm-crypt volume (Linux)
  - Option 2: PostgreSQL TDE extension (pgcrypto)
  - Encryption key managed by bibd, derived from node identity
  - Key escrow/backup mechanism for disaster recovery

### 2.4 Database Audit & Monitoring

- [X] **DB-011**: Comprehensive audit logging
  - Every query logged with:
    - Timestamp (UTC, microsecond precision)
    - Job ID / Operation ID
    - Role used
    - Query text (with parameter values redacted)
    - Rows affected
    - Execution time
    - Source (which bibd component)
  - Audit log stored in separate table, append-only
  - Audit log replicated to separate storage (optional)
  - Tamper detection via hash chains
  - Retention policy (configurable, default 90 days)

- [X] **DB-012**: Audit log schema
  ```sql
  CREATE TABLE audit_log (
    id              BIGSERIAL PRIMARY KEY,
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    node_id         TEXT NOT NULL,
    job_id          UUID,
    operation_id    UUID NOT NULL,
    role_used       TEXT NOT NULL,
    action          TEXT NOT NULL,  -- SELECT, INSERT, UPDATE, DELETE, DDL
    table_name      TEXT,
    query_hash      TEXT,           -- Hash of query for grouping
    rows_affected   INTEGER,
    duration_ms     INTEGER,
    source_component TEXT,          -- scheduler, grpc, p2p, etc.
    metadata        JSONB,
    prev_hash       TEXT,           -- Hash chain for tamper detection
    entry_hash      TEXT NOT NULL   -- Hash of this entry
  );
  
  -- Append-only enforced via trigger
  CREATE OR REPLACE FUNCTION audit_no_modify() RETURNS TRIGGER AS $$
  BEGIN
    RAISE EXCEPTION 'Audit log is append-only';
  END;
  $$ LANGUAGE plpgsql;
  
  CREATE TRIGGER audit_immutable
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION audit_no_modify();
  ```

- [X] **DB-013**: Real-time audit streaming
  - Audit events published to internal channel
  - Optional export to external SIEM (Splunk, ELK, etc.)
  - Alerts for suspicious patterns:
    - Unusual query patterns
    - Failed authentication attempts
    - Schema modification attempts
    - Bulk data access

### 2.5 Break Glass Emergency Access

> **Emergency Access**: For disaster recovery and debugging, a controlled break-glass procedure 
> exists. It requires explicit action, is heavily audited, and auto-expires.

- [x] **DB-014**: Break glass configuration
  ```yaml
  # config.yaml - disabled by default
  database:
    break_glass:
      enabled: false                    # Must be explicitly enabled
      require_restart: true             # bibd restart required to enable
      max_duration: 1h                  # Auto-disable after duration
      allowed_users:                    # Pre-configured emergency users
        - name: "emergency_admin"
          public_key: "ssh-ed25519 AAAA..."  # SSH key for auth
      audit_level: "paranoid"           # Log everything including data
      notification:
        webhook: "https://..."          # Alert when break glass activated
        email: "security@..."
  ```

- [x] **DB-015**: Break glass procedure
  1. `bib admin break-glass enable --reason "description" --duration 1h`
  2. Requires confirmation with node admin key
  3. Creates time-limited PostgreSQL user with restricted access
  4. All actions logged with `break_glass: true` flag
  5. Notification sent to configured endpoints
  6. User can connect via `bib admin break-glass connect`
  7. Session recorded (terminal recording if SSH)
  8. Auto-expires after duration, credentials invalidated
  9. Summary report generated after session ends
  10. `bib admin break-glass disable` for manual early termination

- [x] **DB-016**: Break glass audit trail
  - Separate audit category for break glass sessions
  - Full query logging (no redaction)
  - Terminal session recording (if applicable)
  - Post-session review required (configurable)
  - Compliance report generation

### 2.6 Schema & Migrations
- [x] **DB-017**: Migration framework
  - golang-migrate integration
  - Migrations executed by appropriate role
  - Up/down migrations with checksums
  - Version tracking in migrations table managed by golang-migrate
  - Migration audit logging via golang-migrate
  - Embedded migrations using go:embed
  - Migration locking (PostgreSQL advisory locks, SQLite application-level)
  - Checksum verification on startup (configurable)
  - Safe down migrations that preserve data
- [x] **DB-018**: Core schema design
  - All tables include audit columns and constraints
  - Nodes, topics, datasets, dataset_versions, chunks tables
  - Jobs and job_results tables
  - Blob storage stub (for Phase 2.7)
  - Row-Level Security (RLS) policies for PostgreSQL
  - CHECK constraints for data validation
  - Foreign key constraints with appropriate CASCADE behavior
- [x] **DB-019**: Initial migrations
  - Create all core tables with proper constraints
  - Create audit_log table and triggers
  - Indexes for common queries
  - Helper functions and triggers (updated_at, counts)
  - Row-level security policies
  - Separate migrations for PostgreSQL and SQLite
  - Admin-only rollback with confirmation requirement

### 2.7 Blob Storage
- [x] **DB-020**: Local blob storage
  - Content-addressed storage (CAS)
  - Directory structure: `<data_dir>/blobs/<hash[0:2]>/<hash[2:4]>/<hash>`
  - Integrity verification on read
  - Garbage collection for orphaned blobs
  - Blob access logged to audit trail
- [x] **DB-021**: Optional S3-compatible storage
  - MinIO/S3 integration
  - Configurable backend
  - Tiered storage (hot/cold)
  - S3 credentials managed by bibd (same security model)

### 2.8 Database Lifecycle Management
- [x] **DB-022**: Initialization workflow
  1. `bibd` starts, checks for existing PostgreSQL
  2. If none: provision container/pod with generated config
  3. Wait for PostgreSQL ready (health check)
  4. Connect as superuser, create roles and schema
  5. Run pending migrations
  6. Switch to least-privilege role for normal operations
  7. Begin accepting requests

- [x] **DB-023**: Backup & recovery
  - Automatic daily backups (pg_dump)
  - Backup encryption with node key
  - Backup integrity verification
  - Point-in-time recovery (WAL archiving)
  - Backup to local storage or S3
  - `bib admin backup create` and `bib admin backup list` commands
  - `bib admin restore` command
  - Disaster recovery documentation

- [x] **DB-024**: Graceful shutdown
  - Drain active connections
  - Complete in-flight transactions
  - Checkpoint and sync
  - Stop PostgreSQL container/pod
  - Verify clean shutdown in logs
  - `bib cleanup` command for resource cleanup



---

## Phase 3: Scheduler & Job System

### 3.1 CEL Integration
- [ ] **SCHED-001**: CEL environment setup
  - google/cel-go integration
  - Custom functions for data operations
  - Type declarations for bib data model
- [ ] **SCHED-002**: CEL standard library
  - Data access functions: `dataset()`, `topic()`, `query()`
  - Transformation: `map()`, `filter()`, `reduce()`, `aggregate()`
  - I/O: `fetch()`, `scrape()`, `parse()`
  - ML: `train()`, `predict()`, `embed()`
- [ ] **SCHED-003**: CEL validation & compilation
  - Syntax validation
  - Type checking
  - Resource estimation

### 3.2 Job Types
- [ ] **SCHED-004**: Job type definitions
  ```go
  type JobType string
  const (
    JobTypeScrape     JobType = "scrape"      // Web scraping
    JobTypeTransform  JobType = "transform"   // Data transformation
    JobTypeClean      JobType = "clean"       // Data cleaning
    JobTypeAnalyze    JobType = "analyze"     // Data analysis
    JobTypeML         JobType = "ml"          // Machine learning
    JobTypeETL        JobType = "etl"         // Extract-Transform-Load
    JobTypeCustom     JobType = "custom"      // Custom CEL job
  )
  ```
- [ ] **SCHED-005**: Job lifecycle
  - States: pending → queued → running → completed/failed
  - Retry policies
  - Timeout handling
  - Cancellation support

### 3.3 Scheduler Core
- [ ] **SCHED-006**: Job queue implementation
  - Priority queue with multiple levels
  - Fair scheduling across topics
  - Resource-aware scheduling
- [ ] **SCHED-007**: Worker pool
  - Configurable worker count
  - Worker health monitoring
  - Graceful shutdown
- [ ] **SCHED-008**: Job distribution (P2P)
  - Distribute jobs to capable nodes
  - Data locality optimization
  - Load balancing across cluster

### 3.4 Built-in Job Templates
- [ ] **SCHED-009**: Web scraping jobs
  - HTTP/HTTPS fetching
  - HTML parsing (goquery)
  - Rate limiting & politeness
  - Robots.txt compliance
- [ ] **SCHED-010**: Data cleaning jobs
  - Deduplication
  - Schema validation
  - Missing value handling
  - Outlier detection
- [ ] **SCHED-011**: ML integration
  - ONNX runtime for inference
  - Embedding generation
  - Model versioning
  - Training job orchestration (external)

### 3.5 Data Pipeline
- [ ] **SCHED-012**: Pipeline definition
  - DAG-based pipeline structure
  - Step dependencies
  - Checkpoint/resume
- [ ] **SCHED-013**: Data warehouse features
  - Materialized views
  - Incremental processing
  - Partitioning support

---

## Phase 4: gRPC API

> **Architecture**: The gRPC API serves two purposes:
> 1. **CLI ↔ Daemon**: The `bib` CLI communicates with `bibd` via gRPC over mTLS
> 2. **Daemon ↔ Daemon**: Other `bibd` nodes access the API via P2P (libp2p streams wrapping gRPC)
>
> **Identity Model**: Users authenticate using their SSH key (Ed25519/RSA). The same key is used
> for both CLI gRPC authentication and Wish SSH sessions, ensuring consistent identity across interfaces.
>
> **Local-Only Commands**: Some CLI commands work without a daemon connection:
> - `bib setup` - Initial configuration
> - `bib config` - View/edit local config
> - `bib cert` - Certificate management
> - `bib version` - Version information

### 4.0 Prerequisites & Code Generation

- [x] **GRPC-000**: Proto tooling setup
  - [x] Install buf CLI (`go install github.com/bufbuild/buf/cmd/buf@latest`)
  - [x] Add protoc-gen-go and protoc-gen-go-grpc plugins
  - [x] Validate buf.gen.yaml configuration
  - [x] Create `make proto` target for code generation
  - [x] Add generated code handling in `.gitignore` (committed by default)
  - [x] Create `tools/tools.go` for tracking tool dependencies
  - [x] Create `docs/development/proto-development.md` guide

### 4.1 Protocol Buffer Reorganization

> **Goal**: Separate P2P protocol messages from gRPC service definitions for clarity.

- [x] **GRPC-001**: Proto file structure reorganization
  ```
  api/proto/bib/v1/
  ├── common.proto           # Shared types (existing, keep)
  │
  ├── p2p/                   # P2P protocol messages (libp2p streams)
  │   ├── discovery.proto    # /bib/discovery/1.0.0 (move existing)
  │   ├── data.proto         # /bib/data/1.0.0 (move existing)
  │   ├── sync.proto         # /bib/sync/1.0.0 (move existing)
  │   ├── pubsub.proto       # GossipSub messages (move existing)
  │   └── jobs.proto         # /bib/jobs/1.0.0 (move existing)
  │
  └── services/              # gRPC service definitions
      ├── auth.proto         # AuthService (enhance existing)
      ├── user.proto         # UserService (enhance existing)
      ├── node.proto         # NodeService (new)
      ├── topic.proto        # TopicService (new)
      ├── dataset.proto      # DatasetService (new)
      ├── job.proto          # JobService (new - placeholder until Phase 3)
      ├── query.proto        # QueryService (new)
      ├── admin.proto        # AdminService (new)
      ├── breakglass.proto   # BreakGlassService (move existing)
      └── health.proto       # HealthService (new)
  ```

- [x] **GRPC-002**: Common types enhancement
  - [x] Add pagination message: `PageRequest { int32 limit, int32 offset, string cursor }`
  - [x] Add pagination response: `PageInfo { int32 total_count, string next_cursor, bool has_more }`
  - [x] Add sort options: `SortOrder { string field, bool descending }`
  - [x] Add filter operators for flexible queries
  - [x] Add `OperationMetadata` for request tracking (request_id, timestamp, node_id)
  - [x] Ensure all existing types in common.proto are compatible

- [x] **GRPC-003**: Service definitions overview
  | Service | Priority | Description | Depends On |
  |---------|----------|-------------|------------|
  | HealthService | P0 | Health checks, readiness | None |
  | AuthService | P0 | Authentication, sessions | HealthService |
  | UserService | P0 | User management | AuthService |
  | NodeService | P1 | Node info, peer management | AuthService |
  | TopicService | P1 | Topic CRUD, subscriptions | AuthService |
  | DatasetService | P1 | Dataset CRUD, versions | TopicService |
  | AdminService | P2 | Config, metrics, logs | AuthService |
  | BreakGlassService | P2 | Emergency access | AuthService |
  | QueryService | P2 | CEL queries | DatasetService |
  | JobService | P3 | Job management | Phase 3 Scheduler |

### 4.2 Service Proto Definitions

#### 4.2.1 HealthService (P0)
- [x] **GRPC-004**: HealthService proto definition
  ```protobuf
  service HealthService {
    // Check performs a health check (standard gRPC health check)
    rpc Check(HealthCheckRequest) returns (HealthCheckResponse);
    
    // Watch streams health status changes
    rpc Watch(HealthCheckRequest) returns (stream HealthCheckResponse);
    
    // GetNodeInfo returns detailed node information
    rpc GetNodeInfo(GetNodeInfoRequest) returns (GetNodeInfoResponse);
    
    // Ping is a simple connectivity check
    rpc Ping(PingRequest) returns (PingResponse);
  }
  ```
  - Include service status (SERVING, NOT_SERVING, UNKNOWN)
  - Include component health (storage, p2p, cluster)
  - Include version info, uptime, node mode

#### 4.2.2 AuthService Enhancement (P0)
- [x] **GRPC-005**: AuthService enhancement (existing proto)
  - Review existing `AuthService` in auth.proto
  - Ensure Challenge/VerifyChallenge flow works for SSH keys
  - Add `GetPublicKeyInfo` RPC to get key fingerprint/type from bytes
  - Add session token format specification (JWT or opaque)
  - Add `ListMySessions` RPC for current user
  - Document authentication flow:
    1. Client sends `Challenge(public_key)`
    2. Server returns challenge bytes + challenge_id
    3. Client signs challenge with private key
    4. Client sends `VerifyChallenge(challenge_id, signature)`
    5. Server returns session token + user info

#### 4.2.3 UserService Enhancement (P0)
- [x] **GRPC-006**: UserService enhancement (existing proto)
  - Review existing `UserService` in user.proto
  - Ensure admin-only RPCs are marked (CreateUser, DeleteUser, SuspendUser, etc.)
  - Add `SearchUsers` RPC with text search
  - Add user preferences storage (theme, locale, etc.)

#### 4.2.4 NodeService (P1)
- [x] **GRPC-007**: NodeService proto definition
  ```protobuf
  service NodeService {
    // GetNode returns information about a specific node
    rpc GetNode(GetNodeRequest) returns (GetNodeResponse);
    
    // ListNodes lists known nodes in the network
    rpc ListNodes(ListNodesRequest) returns (ListNodesResponse);
    
    // GetSelfNode returns this node's information
    rpc GetSelfNode(GetSelfNodeRequest) returns (GetSelfNodeResponse);
    
    // ConnectPeer manually connects to a peer
    rpc ConnectPeer(ConnectPeerRequest) returns (ConnectPeerResponse);
    
    // DisconnectPeer disconnects from a peer
    rpc DisconnectPeer(DisconnectPeerRequest) returns (DisconnectPeerResponse);
    
    // GetNetworkStats returns network statistics
    rpc GetNetworkStats(GetNetworkStatsRequest) returns (GetNetworkStatsResponse);
    
    // StreamNodeEvents streams node join/leave events
    rpc StreamNodeEvents(StreamNodeEventsRequest) returns (stream NodeEvent);
  }
  ```

#### 4.2.5 TopicService (P1)
- [x] **GRPC-008**: TopicService proto definition
  ```protobuf
  service TopicService {
    // CreateTopic creates a new topic
    rpc CreateTopic(CreateTopicRequest) returns (CreateTopicResponse);
    
    // GetTopic retrieves a topic by ID or name
    rpc GetTopic(GetTopicRequest) returns (GetTopicResponse);
    
    // ListTopics lists topics with filtering
    rpc ListTopics(ListTopicsRequest) returns (ListTopicsResponse);
    
    // UpdateTopic updates topic metadata
    rpc UpdateTopic(UpdateTopicRequest) returns (UpdateTopicResponse);
    
    // DeleteTopic soft-deletes a topic
    rpc DeleteTopic(DeleteTopicRequest) returns (DeleteTopicResponse);
    
    // Subscribe subscribes to a topic (selective mode)
    rpc Subscribe(SubscribeRequest) returns (SubscribeResponse);
    
    // Unsubscribe unsubscribes from a topic
    rpc Unsubscribe(UnsubscribeRequest) returns (UnsubscribeResponse);
    
    // ListSubscriptions lists current subscriptions
    rpc ListSubscriptions(ListSubscriptionsRequest) returns (ListSubscriptionsResponse);
    
    // StreamTopicUpdates streams topic changes in real-time
    rpc StreamTopicUpdates(StreamTopicUpdatesRequest) returns (stream TopicUpdate);
  }
  ```

#### 4.2.6 DatasetService (P1)
- [x] **GRPC-009**: DatasetService proto definition
  ```protobuf
  service DatasetService {
    // CreateDataset creates a new dataset
    rpc CreateDataset(CreateDatasetRequest) returns (CreateDatasetResponse);
    
    // GetDataset retrieves dataset metadata
    rpc GetDataset(GetDatasetRequest) returns (GetDatasetResponse);
    
    // ListDatasets lists datasets with filtering
    rpc ListDatasets(ListDatasetsRequest) returns (ListDatasetsResponse);
    
    // UpdateDataset updates dataset metadata
    rpc UpdateDataset(UpdateDatasetRequest) returns (UpdateDatasetResponse);
    
    // DeleteDataset soft-deletes a dataset
    rpc DeleteDataset(DeleteDatasetRequest) returns (DeleteDatasetResponse);
    
    // UploadDataset uploads dataset content (streaming)
    rpc UploadDataset(stream UploadDatasetRequest) returns (UploadDatasetResponse);
    
    // DownloadDataset downloads dataset content (streaming)
    rpc DownloadDataset(DownloadDatasetRequest) returns (stream DownloadDatasetResponse);
    
    // GetDatasetVersions lists versions of a dataset
    rpc GetDatasetVersions(GetDatasetVersionsRequest) returns (GetDatasetVersionsResponse);
    
    // GetChunk retrieves a specific chunk
    rpc GetChunk(GetChunkRequest) returns (GetChunkResponse);
    
    // VerifyDataset verifies dataset integrity
    rpc VerifyDataset(VerifyDatasetRequest) returns (VerifyDatasetResponse);
  }
  ```

#### 4.2.7 AdminService (P2)
- [x] **GRPC-010**: AdminService proto definition
  ```protobuf
  service AdminService {
    // GetConfig returns current configuration (redacted secrets)
    rpc GetConfig(GetConfigRequest) returns (GetConfigResponse);
    
    // UpdateConfig updates runtime configuration
    rpc UpdateConfig(UpdateConfigRequest) returns (UpdateConfigResponse);
    
    // GetMetrics returns Prometheus-style metrics
    rpc GetMetrics(GetMetricsRequest) returns (GetMetricsResponse);
    
    // StreamLogs streams daemon logs in real-time
    rpc StreamLogs(StreamLogsRequest) returns (stream LogEntry);
    
    // GetAuditLogs queries audit trail
    rpc GetAuditLogs(GetAuditLogsRequest) returns (GetAuditLogsResponse);
    
    // TriggerBackup initiates a database backup
    rpc TriggerBackup(TriggerBackupRequest) returns (TriggerBackupResponse);
    
    // ListBackups lists available backups
    rpc ListBackups(ListBackupsRequest) returns (ListBackupsResponse);
    
    // GetClusterStatus returns cluster/raft status
    rpc GetClusterStatus(GetClusterStatusRequest) returns (GetClusterStatusResponse);
    
    // Shutdown gracefully shuts down the daemon (admin only)
    rpc Shutdown(ShutdownRequest) returns (ShutdownResponse);
  }
  ```

#### 4.2.8 QueryService (P2)
- [x] **GRPC-011**: QueryService proto definition
  ```protobuf
  service QueryService {
    // Execute runs a CEL query and returns results
    rpc Execute(ExecuteQueryRequest) returns (ExecuteQueryResponse);
    
    // ExecuteStream runs a query and streams results
    rpc ExecuteStream(ExecuteQueryRequest) returns (stream QueryResult);
    
    // ValidateQuery validates a CEL expression without executing
    rpc ValidateQuery(ValidateQueryRequest) returns (ValidateQueryResponse);
    
    // ExplainQuery explains query execution plan
    rpc ExplainQuery(ExplainQueryRequest) returns (ExplainQueryResponse);
  }
  ```
  - Note: Full implementation depends on Phase 3 CEL integration

#### 4.2.9 JobService (P3 - Placeholder)
- [x] **GRPC-012**: JobService proto definition (placeholder)
  ```protobuf
  service JobService {
    // CreateJob creates a new job
    rpc CreateJob(CreateJobRequest) returns (CreateJobResponse);
    
    // GetJob retrieves job status
    rpc GetJob(GetJobRequest) returns (GetJobResponse);
    
    // ListJobs lists jobs with filtering
    rpc ListJobs(ListJobsRequest) returns (ListJobsResponse);
    
    // CancelJob cancels a running job
    rpc CancelJob(CancelJobRequest) returns (CancelJobResponse);
    
    // RetryJob retries a failed job
    rpc RetryJob(RetryJobRequest) returns (RetryJobResponse);
    
    // StreamJobLogs streams job output in real-time
    rpc StreamJobLogs(StreamJobLogsRequest) returns (stream JobLogEntry);
    
    // StreamJobStatus streams job status updates
    rpc StreamJobStatus(StreamJobStatusRequest) returns (stream JobStatusUpdate);
  }
  ```
  - Note: Implementation deferred until Phase 3 completion

### 4.3 TLS & Certificate Management

> **Trust Model**: 
> - bibd auto-generates a self-signed CA on first startup
> - CLI uses Trust-On-First-Use (TOFU) for daemon connections
> - Users can pin certificates for security-sensitive deployments
> - P2P connections use libp2p's Noise/TLS built-in security

#### 4.3.1 Certificate Infrastructure
- [x] **GRPC-013**: CA generation and management
  - Auto-generate CA on first `bibd` startup if none exists
  - Store CA key encrypted in `<config_dir>/secrets/ca.key.enc`
  - Store CA cert in `<config_dir>/certs/ca.crt`
  - CA validity: 10 years (configurable)
  - Log CA fingerprint on startup for verification

- [x] **GRPC-014**: Server certificate generation
  - Generate server cert signed by CA on startup
  - Include node's listen addresses as SANs
  - Include node's peer ID as SAN (for P2P identification)
  - Server cert validity: 1 year (configurable)
  - Auto-renewal 30 days before expiry
  - Store in `<config_dir>/certs/server.{crt,key}`

- [x] **GRPC-015**: Client certificate generation
  - `bib cert generate` creates client cert signed by daemon CA
  - Client cert tied to user's SSH public key fingerprint
  - Include user ID in certificate subject
  - Client cert validity: 90 days (configurable)
  - Store in `<config_dir>/certs/client.{crt,key}`

- [x] **GRPC-016**: Trust-On-First-Use (TOFU) for CLI
  - On first connection, CLI prompts to trust server cert
  - Store trusted cert fingerprint in `<config_dir>/trusted_nodes/<node_id>.fingerprint`
  - Warn if certificate changes (possible MITM)
  - `bib trust add <node_id> --fingerprint <fp>` for manual trust
  - `bib trust list` to show trusted nodes
  - `bib trust remove <node_id>` to untrust

#### 4.3.2 Certificate CLI Commands
- [x] **GRPC-017**: Certificate CLI commands (local, no daemon required)
  - `bib cert init` - Initialize CA (for self-hosted scenarios)
  - `bib cert generate --name <name>` - Generate client cert
  - `bib cert list` - List certificates
  - `bib cert info <cert-file>` - Show certificate details
  - `bib cert export --format pem|p12` - Export certificates
  - `bib cert revoke <fingerprint>` - Add to revocation list

### 4.4 gRPC-over-P2P Transport

> **Goal**: Allow bibd nodes to call each other's gRPC APIs over libp2p streams,
> enabling distributed operations without separate TCP connections.

- [x] **GRPC-018**: libp2p gRPC transport
  - Create custom `grpc.DialOption` that uses libp2p streams
  - Protocol: `/bib/grpc/1.0.0`
  - Multiplex multiple gRPC calls over single libp2p connection
  - Use libp2p's built-in authentication (peer ID verification)
  - Map libp2p peer ID to bibd node identity
  - **Implemented in**: `internal/p2p/grpc_transport.go`, `internal/p2p/grpc_server.go`

- [x] **GRPC-019**: P2P client wrapper
  - `p2p.GRPCClient(peerID)` returns standard gRPC `ClientConn`
  - Automatic peer discovery via DHT
  - Connection pooling per peer
  - Fallback to direct TCP if P2P unavailable (configurable)
  - **Implemented in**: `internal/p2p/grpc_client.go`

- [x] **GRPC-020**: P2P authorization
  - Verify calling peer's node ID against allowed nodes list
  - Allowed peers stored in database with config bootstrap
  - Rate limiting per peer (in-memory, configurable)
  - Silently drop unauthorized connections (secure)
  - Admin/BreakGlass services blocked over P2P
  - **Implemented in**: `internal/p2p/grpc_auth.go`, `internal/storage/*/allowed_peers.go`

### 4.5 Server Implementation

#### 4.5.1 gRPC Server Setup
- [ ] **GRPC-021**: gRPC server infrastructure
  - Create `internal/grpc/server.go` with server lifecycle
  - Configure TLS from generated certificates
  - Listen on configurable port (default: 4000)
  - Optional Unix socket for local CLI (faster, no TLS overhead)
  - Graceful shutdown with connection draining

- [ ] **GRPC-022**: gRPC interceptors
  - **Logging interceptor**: Log all RPC calls with timing
  - **Auth interceptor**: Validate session token, extract user context
  - **Recovery interceptor**: Catch panics, return proper errors
  - **Audit interceptor**: Write to audit log for sensitive operations
  - **Rate limit interceptor**: Per-user rate limiting
  - **Request ID interceptor**: Add/propagate request IDs
  - **Metrics interceptor**: Prometheus metrics for all RPCs

- [ ] **GRPC-023**: Error handling
  - Map domain errors to gRPC status codes
  - Include error details in `google.rpc.Status`
  - Localized error messages (using i18n)
  - Don't leak internal error details to clients
  - Log full error context server-side

- [ ] **GRPC-024**: gRPC reflection & debugging
  - Enable reflection in development mode
  - Disable in production by default (configurable)
  - Health check endpoint for load balancers

#### 4.5.2 Service Implementations

- [ ] **GRPC-025**: Implement HealthService
  - `internal/grpc/services/health.go`
  - Check storage connectivity
  - Check P2P host status
  - Check cluster membership (if enabled)
  - Return aggregate health status

- [ ] **GRPC-026**: Implement AuthService
  - `internal/grpc/services/auth.go`
  - Integrate with existing `internal/auth/service.go`
  - Challenge generation with crypto/rand
  - Challenge verification with SSH key
  - Session token generation (JWT with node signing key)
  - Session storage in database

- [ ] **GRPC-027**: Implement UserService
  - `internal/grpc/services/user.go`
  - Delegate to `storage.UserRepository`
  - RBAC checks for admin operations
  - Input validation

- [ ] **GRPC-028**: Implement NodeService
  - `internal/grpc/services/node.go`
  - Integrate with `internal/p2p/host.go`
  - Expose peer store data
  - Manual peer connection/disconnection

- [ ] **GRPC-029**: Implement TopicService
  - `internal/grpc/services/topic.go`
  - Delegate to `storage.TopicRepository`
  - Subscription management via `internal/p2p/mode_selective.go`
  - Real-time updates via gRPC streams

- [ ] **GRPC-030**: Implement DatasetService
  - `internal/grpc/services/dataset.go`
  - Delegate to `storage.DatasetRepository`
  - Chunked upload/download streaming
  - Integrate with blob storage

- [ ] **GRPC-031**: Implement AdminService
  - `internal/grpc/services/admin.go`
  - Config read (redact secrets)
  - Metrics aggregation
  - Log streaming
  - Backup triggers

- [ ] **GRPC-032**: Implement QueryService (partial)
  - `internal/grpc/services/query.go`
  - Basic query validation
  - Simple queries without CEL (field filters)
  - Full CEL support deferred to Phase 3

- [ ] **GRPC-033**: Implement JobService (stub)
  - `internal/grpc/services/job.go`
  - Return "not implemented" until Phase 3
  - Define interface for future implementation

- [ ] **GRPC-034**: Integrate BreakGlassService
  - Move existing proto to `services/` folder
  - Implement service delegating to `internal/storage/breakglass`

### 4.6 Daemon Integration

- [ ] **GRPC-035**: Add gRPC server to daemon lifecycle
  - Update `cmd/bibd/daemon.go` to start gRPC server
  - Start after storage and P2P initialization
  - Stop before storage shutdown
  - Log gRPC server address on startup

- [ ] **GRPC-036**: Configuration for gRPC
  ```yaml
  # config.yaml additions
  grpc:
    enabled: true
    host: "0.0.0.0"
    port: 4000
    unix_socket: ""                    # Optional: /var/run/bibd/grpc.sock
    max_recv_msg_size: 16777216        # 16MB
    max_send_msg_size: 16777216        # 16MB
    keepalive:
      time: 30s
      timeout: 10s
    reflection: false                  # Enable for debugging
    rate_limit:
      enabled: true
      requests_per_second: 100
      burst: 200
  ```

### 4.7 Client Library & CLI Integration

#### 4.7.1 Go Client Library
- [ ] **GRPC-037**: Client library structure
  ```
  internal/grpc/client/
  ├── client.go        # Main client with connection management
  ├── auth.go          # Auth-related helpers
  ├── options.go       # Connection options (TLS, retry, timeout)
  ├── interceptors.go  # Client-side interceptors
  └── errors.go        # Error handling helpers
  ```

- [ ] **GRPC-038**: Client connection management
  - Support multiple connection targets (direct, P2P, Unix socket)
  - Connection pooling with health checks
  - Automatic reconnection with backoff
  - Context propagation (request ID, user info)

- [ ] **GRPC-039**: Client authentication
  - Load user's SSH key from config or SSH agent
  - Perform Challenge/VerifyChallenge flow
  - Cache session token with refresh
  - Token storage in `<config_dir>/session.token`

#### 4.7.2 CLI Integration
- [ ] **GRPC-040**: CLI client initialization
  - Create gRPC client in `cmd/bib/cmd/root.go`
  - Skip for local-only commands (setup, config, cert, version)
  - Lazy connection on first RPC call
  - Handle connection errors gracefully

- [ ] **GRPC-041**: CLI daemon connection configuration
  ```yaml
  # bib config.yaml
  connection:
    default_node: ""                   # Default node to connect to
    timeout: 30s
    retry_attempts: 3
    tls:
      skip_verify: false               # Dangerous: disable TLS verification
      ca_file: ""                      # Custom CA file
      cert_file: ""                    # Client certificate
      key_file: ""                     # Client key
  ```

- [ ] **GRPC-042**: CLI node selection
  - `bib --node <addr>` flag for explicit node selection
  - Auto-discover local node via mDNS
  - Use favorite nodes from config
  - `bib connect <addr>` to set default node

### 4.8 Testing & Documentation

- [ ] **GRPC-043**: gRPC integration tests
  - Test each service with mock storage
  - Test authentication flow
  - Test streaming endpoints
  - Test error handling

- [ ] **GRPC-044**: P2P gRPC transport tests
  - Test gRPC calls between two bibd instances
  - Test connection failures and recovery
  - Test concurrent calls

- [ ] **GRPC-045**: API documentation
  - Generate proto documentation (buf docs or protoc-gen-doc)
  - Document authentication flow
  - Document error codes
  - Provide example requests/responses

- [ ] **GRPC-046**: Client SDK documentation
  - Usage examples
  - Connection options
  - Error handling best practices

---

## Phase 5: TUI (Bubbletea + Wish)

### 5.1 Core TUI Framework
- [ ] **TUI-001**: Bubbletea application structure
  - Main model with screen navigation
  - Shared styles (lipgloss)
  - Key bindings configuration
- [ ] **TUI-002**: Component library
  - Tables (bubbles/table)
  - Lists (bubbles/list)
  - Text inputs (bubbles/textinput)
  - Viewports (bubbles/viewport)
  - Progress bars (bubbles/progress)
  - Spinners (bubbles/spinner)
- [ ] **TUI-003**: Theme system
  - Light/dark themes
  - Custom color schemes
  - Configurable via config file

### 5.2 Dashboard View
- [ ] **TUI-004**: Real-time dashboard
  - Node status overview
  - Active jobs count
  - Data transfer rates
  - Resource usage (CPU, memory, disk)
- [ ] **TUI-005**: Live updates
  - gRPC streaming integration
  - Auto-refresh intervals
  - Event-driven updates

### 5.3 Data Browser
- [ ] **TUI-006**: Topic browser
  - Hierarchical topic navigation
  - Topic metadata display
  - Subscribe/unsubscribe actions
- [ ] **TUI-007**: Dataset browser
  - List datasets with filtering
  - Dataset details view
  - Preview data (first N rows)
- [ ] **TUI-008**: Query interface
  - CEL expression input
  - Syntax highlighting (if possible)
  - Result display with pagination

### 5.4 Job Management
- [ ] **TUI-009**: Job list view
  - Filter by status, type, topic
  - Sort by various fields
  - Bulk actions
- [ ] **TUI-010**: Job creation wizard
  - Job type selection
  - Parameter input
  - CEL expression editor
  - Preview & submit
- [ ] **TUI-011**: Job detail view
  - Real-time status updates
  - Log streaming
  - Result preview
  - Retry/cancel actions

### 5.5 Interactive Analysis
- [ ] **TUI-012**: Analysis mode
  - REPL-like CEL interface
  - Result visualization (ASCII charts)
  - History navigation
- [ ] **TUI-013**: Data exploration
  - Schema viewer
  - Sample data viewer
  - Statistical summaries

### 5.6 Wish SSH Server
- [ ] **TUI-014**: Wish server integration
  - SSH server in bibd
  - Key-based authentication
  - Session management
- [ ] **TUI-015**: Remote TUI access
  - Full TUI over SSH
  - Read-only mode enforcement
  - Session timeout
- [ ] **TUI-016**: SSH tunneling
  - Port forwarding for web UI
  - Secure remote access

---

## Phase 6: Web Interface

### 6.1 Technology Decision
> **Recommendation**: Use **Templ + HTMX + Alpine.js** for the best Go-native experience with modern interactivity.

| Option | Pros | Cons |
|--------|------|------|
| **Templ + HTMX** | Type-safe templates, Go-native, minimal JS, SSR | Less ecosystem, newer |
| **Go templates + HTMX** | Simple, stdlib, no build step | Verbose, no type safety |
| **Embedded SPA (Vue/React)** | Rich interactivity, ecosystem | Build complexity, larger bundle |

- [ ] **WEB-001**: Choose and setup web framework
  - Templ for type-safe templates
  - HTMX for dynamic updates
  - Alpine.js for client-side state
  - Tailwind CSS for styling

### 6.2 Core Web Infrastructure
- [ ] **WEB-002**: HTTP server setup
  - Chi or stdlib mux
  - Middleware (logging, recovery, auth)
  - Static file serving (embedded)
- [ ] **WEB-003**: Authentication
  - Session-based auth
  - mTLS client certificates
  - OAuth2 (optional, future)
- [ ] **WEB-004**: WebSocket support
  - Real-time updates
  - SSE alternative
  - Connection management

### 6.3 Dashboard Pages
- [ ] **WEB-005**: Home dashboard
  - System overview cards
  - Health status indicators
  - Quick stats charts
- [ ] **WEB-006**: Node management page
  - Connected nodes list
  - Node details modal
  - Network topology visualization
- [ ] **WEB-007**: Real-time metrics
  - Chart.js or similar
  - Time-series data
  - Auto-updating charts

### 6.4 Data Visualization
- [ ] **WEB-008**: Topic explorer
  - Tree/list view
  - Search and filter
  - Topic details panel
- [ ] **WEB-009**: Dataset viewer
  - Grid/table view with virtual scrolling
  - Column sorting/filtering
  - Export functionality
- [ ] **WEB-010**: Data preview
  - JSON/YAML/Table views
  - Schema visualization
  - Relationship diagrams
- [ ] **WEB-011**: Query playground
  - CEL editor with syntax highlighting (CodeMirror)
  - Query execution
  - Result visualization
  - Query history

### 6.5 Job Interface
- [ ] **WEB-012**: Job list page
  - Filterable/sortable table
  - Status badges
  - Bulk operations
- [ ] **WEB-013**: Job detail page
  - Status timeline
  - Log viewer
  - Result explorer
- [ ] **WEB-014**: Job creation form
  - Step-by-step wizard
  - Template library
  - Validation feedback

### 6.6 Administration
- [ ] **WEB-015**: Configuration viewer
  - Display current config
  - Config diff view
- [ ] **WEB-016**: Logs viewer
  - Real-time log streaming
  - Log level filtering
  - Search functionality
- [ ] **WEB-017**: Audit trail
  - Audit log table
  - Filtering by action/actor
  - Export capability

---

## Phase 7: CLI Commands

### 7.1 Node Management
- [ ] **CLI-001**: `bib node list` - List known nodes
- [ ] **CLI-002**: `bib node info <id>` - Node details
- [ ] **CLI-003**: `bib node connect <addr>` - Connect to node
- [ ] **CLI-004**: `bib node disconnect <id>` - Disconnect from node

### 7.2 Topic Commands
- [ ] **CLI-005**: `bib topic list` - List topics
- [ ] **CLI-006**: `bib topic create <name>` - Create topic
- [ ] **CLI-007**: `bib topic subscribe <name>` - Subscribe to topic
- [ ] **CLI-008**: `bib topic unsubscribe <name>` - Unsubscribe

### 7.3 Dataset Commands
- [ ] **CLI-009**: `bib dataset list` - List datasets
- [ ] **CLI-010**: `bib dataset get <id>` - Get dataset info
- [ ] **CLI-011**: `bib dataset upload <file>` - Upload dataset
- [ ] **CLI-012**: `bib dataset download <id>` - Download dataset
- [ ] **CLI-013**: `bib dataset query <expr>` - Query with CEL

### 7.4 Job Commands
- [ ] **CLI-014**: `bib job list` - List jobs
- [ ] **CLI-015**: `bib job create` - Create job (interactive or flags)
- [ ] **CLI-016**: `bib job status <id>` - Job status
- [ ] **CLI-017**: `bib job logs <id>` - Stream job logs
- [ ] **CLI-018**: `bib job cancel <id>` - Cancel job
- [ ] **CLI-019**: `bib job retry <id>` - Retry failed job

### 7.5 Certificate Commands
- [ ] **CLI-020**: `bib cert init` - Initialize CA
- [ ] **CLI-021**: `bib cert generate` - Generate client cert
- [ ] **CLI-022**: `bib cert list` - List certificates
- [ ] **CLI-023**: `bib cert revoke <id>` - Revoke certificate

### 7.6 Interactive Mode
- [ ] **CLI-024**: `bib` (no args) - Launch TUI
- [ ] **CLI-025**: `bib shell` - Interactive CEL shell

---

## Phase 8: DevOps & Deployment

### 8.1 Build System
- [ ] **DEV-001**: Makefile improvements
  - Build targets for all binaries
  - Cross-compilation
  - Version injection via ldflags
- [ ] **DEV-002**: GoReleaser configuration
  - Multi-platform releases
  - Homebrew tap
  - Docker images

### 8.2 Containerization
- [ ] **DEV-003**: Dockerfile for bibd
  - Multi-stage build
  - Minimal runtime image
  - Health check
- [ ] **DEV-004**: Docker Compose for dev
  - bibd + PostgreSQL
  - Volume mounts
  - Network configuration

### 8.3 Kubernetes
- [ ] **DEV-005**: Helm chart
  - Configurable values
  - StatefulSet for bibd
  - Service & Ingress
  - ConfigMap & Secrets
- [ ] **DEV-006**: Kubernetes operator (future)
  - CRD for BibCluster
  - Automated scaling
  - Backup scheduling

### 8.4 Observability
- [ ] **DEV-007**: Prometheus metrics
  - `/metrics` endpoint
  - Custom metrics (jobs, nodes, data)
  - Grafana dashboards
- [ ] **DEV-008**: Distributed tracing
  - OpenTelemetry integration
  - Trace context propagation
  - Jaeger/Zipkin export

---

## Phase 9: Testing & Quality

### 9.1 Unit Tests
- [ ] **TEST-001**: Config package tests
- [ ] **TEST-002**: Logger package tests
- [ ] **TEST-003**: P2P layer tests (mocked)
- [ ] **TEST-004**: Scheduler tests
- [ ] **TEST-005**: Storage layer tests

### 9.2 Integration Tests
- [ ] **TEST-006**: gRPC API tests
- [ ] **TEST-007**: P2P network tests (multiple nodes)
- [ ] **TEST-008**: Database integration tests
- [ ] **TEST-009**: End-to-end job execution tests

### 9.3 CI/CD
- [ ] **TEST-010**: GitHub Actions workflow
  - Lint (golangci-lint)
  - Test with coverage
  - Build verification
- [ ] **TEST-011**: Release automation
  - Tag-based releases
  - Changelog generation
  - Asset publishing

---

## Appendix: Package Structure (Proposed)

```
bib/
├── cmd/
│   ├── bib/           # CLI application
│   │   ├── main.go
│   │   └── cmd/       # Cobra commands
│   └── bibd/          # Daemon
│       └── main.go
├── api/
│   └── proto/         # Protocol buffer definitions
│       └── bib/v1/
├── internal/
│   ├── config/        # Configuration (✓ exists)
│   ├── logger/        # Logging (✓ exists)
│   ├── p2p/           # libp2p networking
│   │   ├── host.go
│   │   ├── discovery.go
│   │   ├── protocols.go
│   │   └── pubsub.go
│   ├── storage/       # Data storage
│   │   ├── db/        # SQL repositories
│   │   ├── blob/      # Blob storage
│   │   └── migrations/
│   ├── scheduler/     # Job scheduling
│   │   ├── queue.go
│   │   ├── worker.go
│   │   ├── cel/       # CEL integration
│   │   └── jobs/      # Built-in job types
│   ├── grpc/          # gRPC server
│   │   ├── server.go
│   │   ├── interceptors/
│   │   └── services/
│   ├── tui/           # Bubbletea UI
│   │   ├── app.go
│   │   ├── views/
│   │   └── components/
│   └── web/           # Web interface
│       ├── server.go
│       ├── handlers/
│       ├── templates/
│       └── static/
├── pkg/               # Public packages
│   ├── client/        # Go client SDK
│   └── cel/           # CEL extensions
├── web/               # Web assets (if SPA)
├── deployments/
│   ├── docker/
│   └── kubernetes/
└── docs/
```

---

## Dependency Summary

### Core Dependencies (Add to go.mod)

```go
// P2P
github.com/libp2p/go-libp2p
github.com/libp2p/go-libp2p-kad-dht
github.com/libp2p/go-libp2p-pubsub

// Database
github.com/jackc/pgx/v5          // PostgreSQL
modernc.org/sqlite               // Pure Go SQLite
github.com/golang-migrate/migrate/v4

// Scheduler & CEL
github.com/google/cel-go

// gRPC
google.golang.org/grpc
google.golang.org/protobuf
github.com/grpc-ecosystem/go-grpc-middleware/v2

// TUI
github.com/charmbracelet/bubbletea
github.com/charmbracelet/bubbles
github.com/charmbracelet/lipgloss  // ✓ exists
github.com/charmbracelet/wish

// Web
github.com/a-h/templ
github.com/go-chi/chi/v5

// Observability
go.opentelemetry.io/otel
github.com/prometheus/client_golang

// Consensus (optional)
github.com/hashicorp/raft
```

---

## Priority Order

1. **Phase 1**: P2P Networking Foundation ← START HERE
2. **Phase 2**: Storage Layer (parallel with P2P)
3. **Phase 3**: Scheduler & Job System
4. **Phase 4**: gRPC API
5. **Phase 5**: TUI
6. **Phase 6**: Web Interface
7. **Phase 7**: CLI Commands (incremental, parallel with other phases)
8. **Phase 8**: DevOps & Deployment
9. **Phase 9**: Testing & Quality (continuous)

---

## Notes & Recommendations

### Node Discovery Strategy
For bib.dev bootstrap + distributed discovery:
1. On startup, connect to `bib.dev` bootstrap node
2. Use DHT to discover additional peers
3. Use mDNS for local network discovery (optional, configurable)
4. Maintain peer list in local database for faster reconnection

### Web Framework Recommendation
**Templ + HTMX + Alpine.js** is recommended because:
- Type-safe Go templates with Templ
- HTMX provides SPA-like experience with server rendering
- Alpine.js handles client-side interactions
- Tailwind CSS for rapid styling
- No complex JS build pipeline
- Excellent for data visualization with Chart.js

### Raft/HA Considerations
- Keep Raft optional and off by default
- Use for: leader election, metadata consistency, configuration sync
- NOT for: data replication (use libp2p protocols instead)
- Consider etcd as alternative if you need more features

---

*Last Updated: 2025-12-16*
*Version: 0.1.0-planning*

