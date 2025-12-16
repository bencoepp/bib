// Package sqlite provides a SQLite implementation of the storage interfaces.
// SQLite stores are non-authoritative and can only be used for caching and proxy modes.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"bib/internal/storage"

	_ "modernc.org/sqlite"
)

// Store implements the storage.Store interface using SQLite.
// SQLite stores are non-authoritative and cannot serve as trusted data sources.
type Store struct {
	db     *sql.DB
	cfg    storage.SQLiteConfig
	nodeID string

	topics   *TopicRepository
	datasets *DatasetRepository
	jobs     *JobRepository
	nodes    *NodeRepository
	audit    *AuditRepository

	mu     sync.RWMutex
	closed bool
}

// New creates a new SQLite store.
func New(cfg storage.SQLiteConfig, dataDir, nodeID string) (*Store, error) {
	dbPath := cfg.Path
	if dbPath == "" {
		dbPath = filepath.Join(dataDir, "cache.db")
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	if cfg.MaxOpenConns == 0 {
		db.SetMaxOpenConns(10)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Set busy timeout
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
	}

	s := &Store{
		db:     db,
		cfg:    cfg,
		nodeID: nodeID,
	}

	// Initialize repositories
	s.topics = &TopicRepository{store: s}
	s.datasets = &DatasetRepository{store: s}
	s.jobs = &JobRepository{store: s}
	s.nodes = &NodeRepository{store: s}
	s.audit = &AuditRepository{store: s}

	return s, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true

	return s.db.Close()
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
	return s.db.PingContext(ctx)
}

// IsAuthoritative returns false for SQLite stores.
// SQLite cannot be an authoritative data source.
func (s *Store) IsAuthoritative() bool {
	return false
}

// Backend returns the storage backend type.
func (s *Store) Backend() storage.BackendType {
	return storage.BackendSQLite
}

// Migrate runs database migrations.
func (s *Store) Migrate(ctx context.Context) error {
	migrations := []string{
		migrationV1Topics,
		migrationV1Datasets,
		migrationV1Jobs,
		migrationV1Nodes,
		migrationV1Audit,
		migrationV1Cache,
		migrationV1Indexes,
	}

	// Create migrations table
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Check current version
	var currentVersion int
	row := s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations")
	if err := row.Scan(&currentVersion); err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	// Apply migrations
	for i, migration := range migrations {
		version := i + 1
		if version <= currentVersion {
			continue
		}

		if _, err := s.db.ExecContext(ctx, migration); err != nil {
			return fmt.Errorf("failed to apply migration %d: %w", version, err)
		}

		if _, err := s.db.ExecContext(ctx,
			"INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)",
			version, time.Now().UTC().Format(time.RFC3339),
		); err != nil {
			return fmt.Errorf("failed to record migration %d: %w", version, err)
		}
	}

	return nil
}

// DB returns the underlying database connection.
// Use with caution - prefer repository methods.
func (s *Store) DB() *sql.DB {
	return s.db
}

// execWithAudit executes a query and logs to audit.
func (s *Store) execWithAudit(ctx context.Context, action, table, query string, args ...any) (sql.Result, error) {
	oc := storage.MustGetOperationContext(ctx)
	start := time.Now()

	// Add query comment for tagging
	taggedQuery := oc.QueryComment() + " " + query

	result, err := s.db.ExecContext(ctx, taggedQuery, args...)

	duration := time.Since(start)

	// Log to audit
	if s.audit != nil && s.cfg.CacheTTL > 0 { // Only audit if not in test mode
		var rowsAffected int64
		if result != nil {
			rowsAffected, _ = result.RowsAffected()
		}

		auditEntry := &storage.AuditEntry{
			Timestamp:       start,
			NodeID:          s.nodeID,
			JobID:           oc.JobID,
			OperationID:     oc.OperationID,
			RoleUsed:        string(oc.Role),
			Action:          action,
			TableName:       table,
			RowsAffected:    int(rowsAffected),
			DurationMS:      int(duration.Milliseconds()),
			SourceComponent: oc.Source,
			Metadata:        oc.Metadata,
		}
		// Best effort audit logging - don't fail the operation
		_ = s.audit.Log(ctx, auditEntry)
	}

	return result, err
}

// queryWithAudit executes a query and logs to audit.
func (s *Store) queryWithAudit(ctx context.Context, table, query string, args ...any) (*sql.Rows, error) {
	oc := storage.MustGetOperationContext(ctx)
	start := time.Now()

	// Add query comment for tagging
	taggedQuery := oc.QueryComment() + " " + query

	rows, err := s.db.QueryContext(ctx, taggedQuery, args...)

	duration := time.Since(start)

	// Log to audit
	if s.audit != nil && s.cfg.CacheTTL > 0 {
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

// Migrations
const migrationV1Topics = `
CREATE TABLE IF NOT EXISTS topics (
	id TEXT PRIMARY KEY,
	parent_id TEXT,
	name TEXT NOT NULL UNIQUE,
	description TEXT,
	table_schema TEXT,
	status TEXT NOT NULL DEFAULT 'active',
	owners TEXT NOT NULL, -- JSON array of user IDs
	created_by TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	dataset_count INTEGER NOT NULL DEFAULT 0,
	tags TEXT, -- JSON array
	metadata TEXT, -- JSON object
	cached_at TEXT, -- When this was cached (for TTL)
	FOREIGN KEY (parent_id) REFERENCES topics(id)
);
`

const migrationV1Datasets = `
CREATE TABLE IF NOT EXISTS datasets (
	id TEXT PRIMARY KEY,
	topic_id TEXT NOT NULL,
	name TEXT NOT NULL,
	description TEXT,
	status TEXT NOT NULL DEFAULT 'draft',
	latest_version_id TEXT,
	version_count INTEGER NOT NULL DEFAULT 0,
	has_content INTEGER NOT NULL DEFAULT 0,
	has_instructions INTEGER NOT NULL DEFAULT 0,
	owners TEXT NOT NULL, -- JSON array
	created_by TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	tags TEXT, -- JSON array
	metadata TEXT, -- JSON object
	cached_at TEXT,
	FOREIGN KEY (topic_id) REFERENCES topics(id)
);

CREATE TABLE IF NOT EXISTS dataset_versions (
	id TEXT PRIMARY KEY,
	dataset_id TEXT NOT NULL,
	version TEXT NOT NULL,
	previous_version_id TEXT,
	content TEXT, -- JSON object
	instructions TEXT, -- JSON object
	table_schema TEXT,
	created_by TEXT NOT NULL,
	created_at TEXT NOT NULL,
	message TEXT,
	metadata TEXT, -- JSON object
	cached_at TEXT,
	FOREIGN KEY (dataset_id) REFERENCES datasets(id) ON DELETE CASCADE,
	FOREIGN KEY (previous_version_id) REFERENCES dataset_versions(id),
	UNIQUE (dataset_id, version)
);

CREATE TABLE IF NOT EXISTS chunks (
	id TEXT PRIMARY KEY,
	dataset_id TEXT NOT NULL,
	version_id TEXT NOT NULL,
	chunk_index INTEGER NOT NULL,
	hash TEXT NOT NULL,
	size INTEGER NOT NULL,
	status TEXT NOT NULL DEFAULT 'pending',
	storage_path TEXT,
	cached_at TEXT,
	FOREIGN KEY (dataset_id) REFERENCES datasets(id) ON DELETE CASCADE,
	FOREIGN KEY (version_id) REFERENCES dataset_versions(id) ON DELETE CASCADE,
	UNIQUE (version_id, chunk_index)
);
`

const migrationV1Jobs = `
CREATE TABLE IF NOT EXISTS jobs (
	id TEXT PRIMARY KEY,
	type TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'pending',
	task_id TEXT,
	inline_instructions TEXT, -- JSON array
	execution_mode TEXT NOT NULL DEFAULT 'goroutine',
	schedule TEXT, -- JSON object
	inputs TEXT, -- JSON array
	outputs TEXT, -- JSON array
	dependencies TEXT, -- JSON array of job IDs
	topic_id TEXT,
	dataset_id TEXT,
	priority INTEGER NOT NULL DEFAULT 0,
	resource_limits TEXT, -- JSON object
	created_by TEXT NOT NULL,
	created_at TEXT NOT NULL,
	started_at TEXT,
	completed_at TEXT,
	error TEXT,
	result TEXT,
	progress INTEGER NOT NULL DEFAULT 0,
	current_instruction INTEGER NOT NULL DEFAULT 0,
	node_id TEXT,
	retry_count INTEGER NOT NULL DEFAULT 0,
	metadata TEXT -- JSON object
);

CREATE TABLE IF NOT EXISTS job_results (
	id TEXT PRIMARY KEY,
	job_id TEXT NOT NULL,
	node_id TEXT NOT NULL,
	status TEXT NOT NULL,
	result TEXT,
	error TEXT,
	started_at TEXT,
	completed_at TEXT NOT NULL,
	duration_ms INTEGER NOT NULL DEFAULT 0,
	metadata TEXT, -- JSON object
	FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE
);
`

const migrationV1Nodes = `
CREATE TABLE IF NOT EXISTS nodes (
	peer_id TEXT PRIMARY KEY,
	addresses TEXT NOT NULL, -- JSON array
	mode TEXT NOT NULL,
	storage_type TEXT NOT NULL,
	trusted_storage INTEGER NOT NULL DEFAULT 0,
	last_seen TEXT NOT NULL,
	metadata TEXT, -- JSON object
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
`

const migrationV1Audit = `
CREATE TABLE IF NOT EXISTS audit_log (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	timestamp TEXT NOT NULL,
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
	metadata TEXT, -- JSON object
	prev_hash TEXT,
	entry_hash TEXT NOT NULL
);

-- Trigger to prevent modifications (append-only)
CREATE TRIGGER IF NOT EXISTS audit_no_update
	BEFORE UPDATE ON audit_log
BEGIN
	SELECT RAISE(ABORT, 'Audit log is append-only');
END;

CREATE TRIGGER IF NOT EXISTS audit_no_delete
	BEFORE DELETE ON audit_log
BEGIN
	SELECT RAISE(ABORT, 'Audit log is append-only');
END;
`

const migrationV1Cache = `
-- Cache metadata table for TTL management
CREATE TABLE IF NOT EXISTS cache_metadata (
	key TEXT PRIMARY KEY,
	table_name TEXT NOT NULL,
	entity_id TEXT NOT NULL,
	cached_at TEXT NOT NULL,
	expires_at TEXT NOT NULL,
	source_node TEXT -- Node the data was cached from
);
`

const migrationV1Indexes = `
CREATE INDEX IF NOT EXISTS idx_topics_status ON topics(status);
CREATE INDEX IF NOT EXISTS idx_topics_parent ON topics(parent_id);
CREATE INDEX IF NOT EXISTS idx_topics_name ON topics(name);

CREATE INDEX IF NOT EXISTS idx_datasets_topic ON datasets(topic_id);
CREATE INDEX IF NOT EXISTS idx_datasets_status ON datasets(status);
CREATE INDEX IF NOT EXISTS idx_datasets_name ON datasets(name);

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

CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_log(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_job ON audit_log(job_id);
CREATE INDEX IF NOT EXISTS idx_audit_operation ON audit_log(operation_id);

CREATE INDEX IF NOT EXISTS idx_cache_expires ON cache_metadata(expires_at);
`

// init registers the SQLite store factory with the storage package.
func init() {
	storage.OpenSQLite = func(ctx context.Context, cfg storage.SQLiteConfig, dataDir, nodeID string) (storage.Store, error) {
		return New(cfg, dataDir, nodeID)
	}
}
