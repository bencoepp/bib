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
  
- [ ] **DB-005**: Kubernetes PostgreSQL deployment
  - bibd creates/manages PostgreSQL StatefulSet
  - Runs in same namespace as bibd
  - PersistentVolumeClaim for data durability
  - Pod anti-affinity with bibd for resilience
  - Automatic backup CronJob creation
  - NetworkPolicy restricting access to bibd pod only
  - ServiceAccount with minimal RBAC permissions

### 2.3 Database Security & Hardening

> **Zero Trust Database Access**: The PostgreSQL instance is invisible to everything except bibd.
> All credentials are generated, rotated, and never exposed. Every query is audited.

- [ ] **DB-006**: Credential management
  - bibd generates all PostgreSQL credentials at initialization
  - Superuser password: 64-char random, never logged, never exposed
  - Credentials stored encrypted in `<config_dir>/secrets/db.enc`
  - Encryption key derived from node identity (Ed25519)
  - Automatic credential rotation (configurable interval, default 7 days)
  - Rotation is zero-downtime (create new role, migrate, drop old)
  - Credentials never appear in config files, logs, or error messages

- [ ] **DB-007**: Role-based database access (per job type)
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

- [ ] **DB-008**: Network isolation
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

- [ ] **DB-009**: PostgreSQL hardening configuration
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

- [ ] **DB-010**: Encryption at rest
  - PostgreSQL data directory encryption
  - Option 1: LUKS/dm-crypt volume (Linux)
  - Option 2: PostgreSQL TDE extension (pgcrypto)
  - Encryption key managed by bibd, derived from node identity
  - Key escrow/backup mechanism for disaster recovery

### 2.4 Database Audit & Monitoring

- [ ] **DB-011**: Comprehensive audit logging
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

- [ ] **DB-012**: Audit log schema
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

- [ ] **DB-013**: Real-time audit streaming
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

- [ ] **DB-014**: Break glass configuration
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

- [ ] **DB-015**: Break glass procedure
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

- [ ] **DB-016**: Break glass audit trail
  - Separate audit category for break glass sessions
  - Full query logging (no redaction)
  - Terminal session recording (if applicable)
  - Post-session review required (configurable)
  - Compliance report generation

### 2.6 Schema & Migrations
- [ ] **DB-017**: Migration framework
  - golang-migrate or goose
  - Migrations executed by `bibd_admin_jobs` role only
  - Up/down migrations with checksums
  - Version tracking in `schema_migrations` table
  - Migration audit logging
- [ ] **DB-018**: Core schema design
  ```sql
  -- All tables include audit columns
  CREATE TABLE nodes (
    peer_id         TEXT PRIMARY KEY,
    address         TEXT[],
    mode            TEXT NOT NULL,
    storage_type    TEXT NOT NULL,  -- 'sqlite' | 'postgres'
    trusted_storage BOOLEAN NOT NULL DEFAULT false,
    last_seen       TIMESTAMPTZ,
    metadata        JSONB,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
  );
  
  CREATE TABLE topics (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT UNIQUE NOT NULL,
    description     TEXT,
    schema          JSONB,
    owner_node_id   TEXT REFERENCES nodes(peer_id),
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
  );
  
  CREATE TABLE datasets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    topic_id        UUID REFERENCES topics(id),
    name            TEXT NOT NULL,
    size_bytes      BIGINT,
    hash            TEXT NOT NULL,  -- Content hash
    location        TEXT,           -- Blob storage path
    metadata        JSONB,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
  );
  
  CREATE TABLE chunks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dataset_id      UUID REFERENCES datasets(id) ON DELETE CASCADE,
    chunk_index     INTEGER NOT NULL,
    hash            TEXT NOT NULL,
    size_bytes      INTEGER NOT NULL,
    storage_path    TEXT,
    UNIQUE (dataset_id, chunk_index)
  );
  
  CREATE TABLE jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type            TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',
    cel_expression  TEXT,
    priority        INTEGER DEFAULT 0,
    config          JSONB,
    created_by      TEXT,  -- Node ID or user
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW(),
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ
  );
  
  CREATE TABLE job_results (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id          UUID REFERENCES jobs(id) ON DELETE CASCADE,
    node_id         TEXT REFERENCES nodes(peer_id),
    status          TEXT NOT NULL,
    result          JSONB,
    error           TEXT,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ DEFAULT NOW()
  );
  ```
- [ ] **DB-019**: Initial migrations
  - Create all core tables with proper constraints
  - Create audit_log table and triggers
  - Create role-specific permissions
  - Indexes for common queries
  - Row-level security policies (optional, for extra isolation)

### 2.7 Blob Storage
- [ ] **DB-020**: Local blob storage
  - Content-addressed storage (CAS)
  - Directory structure: `<data_dir>/blobs/<hash[0:2]>/<hash[2:4]>/<hash>`
  - Integrity verification on read
  - Garbage collection for orphaned blobs
  - Blob access logged to audit trail
- [ ] **DB-021**: Optional S3-compatible storage
  - MinIO/S3 integration
  - Configurable backend
  - Tiered storage (hot/cold)
  - S3 credentials managed by bibd (same security model)

### 2.8 Database Lifecycle Management
- [ ] **DB-022**: Initialization workflow
  1. `bibd` starts, checks for existing PostgreSQL
  2. If none: provision container/pod with generated config
  3. Wait for PostgreSQL ready (health check)
  4. Connect as superuser, create roles and schema
  5. Run pending migrations
  6. Switch to least-privilege role for normal operations
  7. Begin accepting requests

- [ ] **DB-023**: Backup & recovery
  - Automatic daily backups (pg_dump)
  - Backup encryption with node key
  - Backup integrity verification
  - Point-in-time recovery (WAL archiving)
  - Backup to local storage or S3
  - `bib admin backup` and `bib admin restore` commands
  - Disaster recovery documentation

- [ ] **DB-024**: Graceful shutdown
  - Drain active connections
  - Complete in-flight transactions
  - Checkpoint and sync
  - Stop PostgreSQL container/pod
  - Verify clean shutdown in logs



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

### 4.1 Protocol Definitions
- [ ] **GRPC-001**: Proto file structure
  ```
  api/proto/
  ├── bib/v1/
  │   ├── common.proto       # Shared types
  │   ├── nodes.proto        # Node management
  │   ├── topics.proto       # Topic operations
  │   ├── datasets.proto     # Dataset operations  
  │   ├── jobs.proto         # Job scheduling
  │   ├── query.proto        # Data queries
  │   └── admin.proto        # Administrative ops
  ```
- [ ] **GRPC-002**: Common types
  - Pagination, filtering, sorting
  - Error types
  - Metadata structures
- [ ] **GRPC-003**: Service definitions
  - NodeService: Register, List, Get, Health
  - TopicService: Create, List, Subscribe, Unsubscribe
  - DatasetService: Upload, Download, Query, Delete
  - JobService: Create, List, Get, Cancel, Retry
  - QueryService: Execute, Stream
  - AdminService: Config, Metrics, Logs

### 4.2 Authentication & Security
- [ ] **GRPC-004**: mTLS implementation
  - Certificate generation tooling (`bib cert generate`)
  - CA management
  - Certificate rotation
- [ ] **GRPC-005**: Certificate management
  - Store certs in config directory
  - Auto-renewal with warning
  - Revocation list support
- [ ] **GRPC-006**: Authorization layer
  - Role-based access control (RBAC)
  - Resource-level permissions
  - Audit logging integration

### 4.3 Server Implementation
- [ ] **GRPC-007**: gRPC server setup
  - TLS configuration
  - Interceptors (logging, auth, recovery)
  - Reflection for debugging
- [ ] **GRPC-008**: Implement NodeService
- [ ] **GRPC-009**: Implement TopicService
- [ ] **GRPC-010**: Implement DatasetService
- [ ] **GRPC-011**: Implement JobService
- [ ] **GRPC-012**: Implement QueryService
- [ ] **GRPC-013**: Implement AdminService
- [ ] **GRPC-014**: Streaming endpoints
  - Job status streaming
  - Query result streaming
  - Real-time logs

### 4.4 Client SDK
- [ ] **GRPC-015**: Go client library
  - Connection management
  - Retry logic
  - Context propagation
- [ ] **GRPC-016**: Client in bib CLI
  - gRPC client initialization
  - Connection pooling
  - Error handling

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

