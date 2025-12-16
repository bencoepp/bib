// Package postgres provides a PostgreSQL implementation of the storage interfaces.
// PostgreSQL stores are authoritative and can serve as trusted data sources.
package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"bib/internal/storage"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store implements the storage.Store interface using PostgreSQL.
// PostgreSQL stores are authoritative and can serve as trusted data sources.
type Store struct {
	pool   *pgxpool.Pool
	cfg    storage.PostgresConfig
	nodeID string

	// Role-specific pools for permission isolation
	pools map[storage.DBRole]*pgxpool.Pool

	topics   *TopicRepository
	datasets *DatasetRepository
	jobs     *JobRepository
	nodes    *NodeRepository
	audit    *AuditRepository

	mu     sync.RWMutex
	closed bool
}

// New creates a new PostgreSQL store.
func New(ctx context.Context, cfg storage.PostgresConfig, dataDir, nodeID string) (*Store, error) {
	var pool *pgxpool.Pool

	if cfg.Advanced != nil {
		// Use advanced/manual configuration (for testing only)
		connString := fmt.Sprintf(
			"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
			cfg.Advanced.Host,
			cfg.Advanced.Port,
			cfg.Advanced.Database,
			cfg.Advanced.User,
			cfg.Advanced.Password,
			cfg.Advanced.SSLMode,
		)

		poolConfig, err := pgxpool.ParseConfig(connString)
		if err != nil {
			return nil, fmt.Errorf("failed to parse connection string: %w", err)
		}

		poolConfig.MaxConns = int32(cfg.MaxConnections)
		if poolConfig.MaxConns == 0 {
			poolConfig.MaxConns = 20
		}

		var poolErr error
		pool, poolErr = pgxpool.NewWithConfig(ctx, poolConfig)
		if poolErr != nil {
			return nil, fmt.Errorf("failed to create connection pool: %w", poolErr)
		}
	} else if cfg.Managed {
		// For managed mode, we need to start the container first
		// This will be handled by the lifecycle manager
		return nil, fmt.Errorf("managed PostgreSQL requires lifecycle manager initialization")
	} else {
		return nil, fmt.Errorf("either managed=true or advanced config is required")
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	s := &Store{
		pool:   pool,
		cfg:    cfg,
		nodeID: nodeID,
		pools:  make(map[storage.DBRole]*pgxpool.Pool),
	}

	// Initialize repositories
	s.topics = &TopicRepository{store: s}
	s.datasets = &DatasetRepository{store: s}
	s.jobs = &JobRepository{store: s}
	s.nodes = &NodeRepository{store: s}
	s.audit = &AuditRepository{store: s}

	return s, nil
}

// NewWithPool creates a store with an existing connection pool (for testing).
func NewWithPool(pool *pgxpool.Pool, nodeID string) *Store {
	s := &Store{
		pool:   pool,
		nodeID: nodeID,
		pools:  make(map[storage.DBRole]*pgxpool.Pool),
	}

	s.topics = &TopicRepository{store: s}
	s.datasets = &DatasetRepository{store: s}
	s.jobs = &JobRepository{store: s}
	s.nodes = &NodeRepository{store: s}
	s.audit = &AuditRepository{store: s}

	return s
}

// Close closes all database connections.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true

	// Close role-specific pools
	for _, pool := range s.pools {
		pool.Close()
	}

	// Close main pool
	s.pool.Close()

	return nil
}

// Topics returns the topic repository.
func (s *Store) Topics() storage.TopicRepository {
	return s.topics
}

// Datasets returns the dataset repository.
func (s *Store) Datasets() storage.DatasetRepository {
	return s.datasets
}

// Jobs returns the job repository.
func (s *Store) Jobs() storage.JobRepository {
	return s.jobs
}

// Nodes returns the node repository.
func (s *Store) Nodes() storage.NodeRepository {
	return s.nodes
}

// Audit returns the audit repository.
func (s *Store) Audit() storage.AuditRepository {
	return s.audit
}

// Ping checks database connectivity.
func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// IsAuthoritative returns true for PostgreSQL stores.
func (s *Store) IsAuthoritative() bool {
	return true
}

// Backend returns the storage backend type.
func (s *Store) Backend() storage.BackendType {
	return storage.BackendPostgres
}

// Migrate runs database migrations.
func (s *Store) Migrate(ctx context.Context) error {
	migrations := []struct {
		version int
		sql     string
	}{
		{1, migrationV1Schema},
		{2, migrationV1Indexes},
		{3, migrationV1Audit},
		{4, migrationV1Functions},
	}

	// Create migrations table
	if _, err := s.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Check current version
	var currentVersion int
	row := s.pool.QueryRow(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations")
	if err := row.Scan(&currentVersion); err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	// Apply migrations
	for _, m := range migrations {
		if m.version <= currentVersion {
			continue
		}

		if _, err := s.pool.Exec(ctx, m.sql); err != nil {
			return fmt.Errorf("failed to apply migration %d: %w", m.version, err)
		}

		if _, err := s.pool.Exec(ctx,
			"INSERT INTO schema_migrations (version) VALUES ($1)",
			m.version,
		); err != nil {
			return fmt.Errorf("failed to record migration %d: %w", m.version, err)
		}
	}

	return nil
}

// Pool returns the main connection pool.
func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}

// PoolForRole returns a connection pool for a specific role.
// If no role-specific pool exists, returns the main pool.
func (s *Store) PoolForRole(role storage.DBRole) *pgxpool.Pool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if pool, ok := s.pools[role]; ok {
		return pool
	}
	return s.pool
}

// execWithAudit executes a query and logs to audit.
func (s *Store) execWithAudit(ctx context.Context, action, table, query string, args ...any) (int64, error) {
	oc := storage.MustGetOperationContext(ctx)
	start := time.Now()

	// Add query comment for tagging
	taggedQuery := oc.QueryComment() + " " + query

	// Get appropriate pool for the role
	pool := s.PoolForRole(oc.Role)

	result, err := pool.Exec(ctx, taggedQuery, args...)

	duration := time.Since(start)

	// Log to audit
	if s.audit != nil {
		auditEntry := &storage.AuditEntry{
			Timestamp:       start,
			NodeID:          s.nodeID,
			JobID:           oc.JobID,
			OperationID:     oc.OperationID,
			RoleUsed:        string(oc.Role),
			Action:          action,
			TableName:       table,
			RowsAffected:    int(result.RowsAffected()),
			DurationMS:      int(duration.Milliseconds()),
			SourceComponent: oc.Source,
			Metadata:        oc.Metadata,
		}
		_ = s.audit.Log(ctx, auditEntry)
	}

	return result.RowsAffected(), err
}

// queryWithAudit executes a query and logs to audit.
func (s *Store) queryWithAudit(ctx context.Context, table, query string, args ...any) (pgx.Rows, error) {
	oc := storage.MustGetOperationContext(ctx)
	start := time.Now()

	// Add query comment for tagging
	taggedQuery := oc.QueryComment() + " " + query

	// Get appropriate pool for the role
	pool := s.PoolForRole(oc.Role)

	rows, err := pool.Query(ctx, taggedQuery, args...)

	duration := time.Since(start)

	// Log to audit
	if s.audit != nil {
		auditEntry := &storage.AuditEntry{
			Timestamp:       start,
			NodeID:          s.nodeID,
			JobID:           oc.JobID,
			OperationID:     oc.OperationID,
			RoleUsed:        string(oc.Role),
			Action:          "SELECT",
			TableName:       table,
			DurationMS:      int(duration.Milliseconds()),
			SourceComponent: oc.Source,
			Metadata:        oc.Metadata,
		}
		_ = s.audit.Log(ctx, auditEntry)
	}

	return rows, err
}

// queryRowWithAudit executes a single-row query and logs to audit.
func (s *Store) queryRowWithAudit(ctx context.Context, table, query string, args ...any) pgx.Row {
	oc := storage.MustGetOperationContext(ctx)

	// Add query comment for tagging
	taggedQuery := oc.QueryComment() + " " + query

	// Get appropriate pool for the role
	pool := s.PoolForRole(oc.Role)

	// Note: We can't easily audit single row queries without executing twice
	// The audit happens after the scan in the caller
	return pool.QueryRow(ctx, taggedQuery, args...)
}

// DataDir returns the configured data directory.
func (s *Store) DataDir() string {
	if s.cfg.DataDir != "" {
		return s.cfg.DataDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "bibd", "postgres")
}

// Migrations
const migrationV1Schema = `
-- Nodes table
CREATE TABLE IF NOT EXISTS nodes (
	peer_id TEXT PRIMARY KEY,
	addresses TEXT[] NOT NULL,
	mode TEXT NOT NULL,
	storage_type TEXT NOT NULL,
	trusted_storage BOOLEAN NOT NULL DEFAULT false,
	last_seen TIMESTAMPTZ NOT NULL,
	metadata JSONB,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Topics table
CREATE TABLE IF NOT EXISTS topics (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	parent_id UUID REFERENCES topics(id),
	name TEXT UNIQUE NOT NULL,
	description TEXT,
	table_schema TEXT,
	status TEXT NOT NULL DEFAULT 'active',
	owners TEXT[] NOT NULL,
	created_by TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	dataset_count INTEGER NOT NULL DEFAULT 0,
	tags TEXT[],
	metadata JSONB
);

-- Datasets table
CREATE TABLE IF NOT EXISTS datasets (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	topic_id UUID NOT NULL REFERENCES topics(id),
	name TEXT NOT NULL,
	description TEXT,
	status TEXT NOT NULL DEFAULT 'draft',
	latest_version_id UUID,
	version_count INTEGER NOT NULL DEFAULT 0,
	has_content BOOLEAN NOT NULL DEFAULT false,
	has_instructions BOOLEAN NOT NULL DEFAULT false,
	owners TEXT[] NOT NULL,
	created_by TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	tags TEXT[],
	metadata JSONB
);

-- Dataset versions table
CREATE TABLE IF NOT EXISTS dataset_versions (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	dataset_id UUID NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
	version TEXT NOT NULL,
	previous_version_id UUID REFERENCES dataset_versions(id),
	content JSONB,
	instructions JSONB,
	table_schema TEXT,
	created_by TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	message TEXT,
	metadata JSONB,
	UNIQUE (dataset_id, version)
);

-- Update datasets foreign key for latest_version_id
ALTER TABLE datasets 
	ADD CONSTRAINT fk_latest_version 
	FOREIGN KEY (latest_version_id) REFERENCES dataset_versions(id);

-- Chunks table
CREATE TABLE IF NOT EXISTS chunks (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	dataset_id UUID NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
	version_id UUID NOT NULL REFERENCES dataset_versions(id) ON DELETE CASCADE,
	chunk_index INTEGER NOT NULL,
	hash TEXT NOT NULL,
	size BIGINT NOT NULL,
	status TEXT NOT NULL DEFAULT 'pending',
	storage_path TEXT,
	UNIQUE (version_id, chunk_index)
);

-- Jobs table
CREATE TABLE IF NOT EXISTS jobs (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	type TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'pending',
	task_id TEXT,
	inline_instructions JSONB,
	execution_mode TEXT NOT NULL DEFAULT 'goroutine',
	schedule JSONB,
	inputs JSONB,
	outputs JSONB,
	dependencies UUID[],
	topic_id UUID REFERENCES topics(id),
	dataset_id UUID REFERENCES datasets(id),
	priority INTEGER NOT NULL DEFAULT 0,
	resource_limits JSONB,
	created_by TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	started_at TIMESTAMPTZ,
	completed_at TIMESTAMPTZ,
	error TEXT,
	result TEXT,
	progress INTEGER NOT NULL DEFAULT 0,
	current_instruction INTEGER NOT NULL DEFAULT 0,
	node_id TEXT,
	retry_count INTEGER NOT NULL DEFAULT 0,
	metadata JSONB
);

-- Job results table
CREATE TABLE IF NOT EXISTS job_results (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
	node_id TEXT NOT NULL,
	status TEXT NOT NULL,
	result TEXT,
	error TEXT,
	started_at TIMESTAMPTZ,
	completed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	duration_ms BIGINT NOT NULL DEFAULT 0,
	metadata JSONB
);
`

const migrationV1Indexes = `
CREATE INDEX IF NOT EXISTS idx_topics_status ON topics(status);
CREATE INDEX IF NOT EXISTS idx_topics_parent ON topics(parent_id);
CREATE INDEX IF NOT EXISTS idx_topics_name ON topics(name);
CREATE INDEX IF NOT EXISTS idx_topics_owners ON topics USING GIN(owners);

CREATE INDEX IF NOT EXISTS idx_datasets_topic ON datasets(topic_id);
CREATE INDEX IF NOT EXISTS idx_datasets_status ON datasets(status);
CREATE INDEX IF NOT EXISTS idx_datasets_name ON datasets(name);
CREATE INDEX IF NOT EXISTS idx_datasets_owners ON datasets USING GIN(owners);

CREATE INDEX IF NOT EXISTS idx_versions_dataset ON dataset_versions(dataset_id);

CREATE INDEX IF NOT EXISTS idx_chunks_version ON chunks(version_id);
CREATE INDEX IF NOT EXISTS idx_chunks_status ON chunks(status);

CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_type ON jobs(type);
CREATE INDEX IF NOT EXISTS idx_jobs_priority ON jobs(priority DESC);
CREATE INDEX IF NOT EXISTS idx_jobs_created ON jobs(created_at);

CREATE INDEX IF NOT EXISTS idx_results_job ON job_results(job_id);

CREATE INDEX IF NOT EXISTS idx_nodes_mode ON nodes(mode);
CREATE INDEX IF NOT EXISTS idx_nodes_seen ON nodes(last_seen);
`

const migrationV1Audit = `
-- Audit log table
CREATE TABLE IF NOT EXISTS audit_log (
	id BIGSERIAL PRIMARY KEY,
	timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	node_id TEXT NOT NULL,
	job_id TEXT,
	operation_id TEXT NOT NULL,
	role_used TEXT NOT NULL,
	action TEXT NOT NULL,
	table_name TEXT,
	query_hash TEXT,
	rows_affected INTEGER,
	duration_ms INTEGER,
	source_component TEXT,
	metadata JSONB,
	prev_hash TEXT,
	entry_hash TEXT NOT NULL
);

-- Audit log indexes
CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_log(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_job ON audit_log(job_id);
CREATE INDEX IF NOT EXISTS idx_audit_operation ON audit_log(operation_id);
CREATE INDEX IF NOT EXISTS idx_audit_node ON audit_log(node_id);

-- Append-only enforcement
CREATE OR REPLACE FUNCTION audit_no_modify() RETURNS TRIGGER AS $$
BEGIN
	RAISE EXCEPTION 'Audit log is append-only';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS audit_immutable ON audit_log;
CREATE TRIGGER audit_immutable
	BEFORE UPDATE OR DELETE ON audit_log
	FOR EACH ROW EXECUTE FUNCTION audit_no_modify();
`

const migrationV1Functions = `
-- Helper function to update updated_at timestamps
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
	NEW.updated_at = NOW();
	RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply to tables
DROP TRIGGER IF EXISTS update_topics_updated_at ON topics;
CREATE TRIGGER update_topics_updated_at
	BEFORE UPDATE ON topics
	FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_datasets_updated_at ON datasets;
CREATE TRIGGER update_datasets_updated_at
	BEFORE UPDATE ON datasets
	FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_nodes_updated_at ON nodes;
CREATE TRIGGER update_nodes_updated_at
	BEFORE UPDATE ON nodes
	FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
`

// init registers the PostgreSQL store factory with the storage package.
func init() {
	storage.OpenPostgres = func(ctx context.Context, cfg storage.PostgresConfig, dataDir, nodeID string) (storage.Store, error) {
		return New(ctx, cfg, dataDir, nodeID)
	}
}
