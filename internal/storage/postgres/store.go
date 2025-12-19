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
	users    *UserRepository
	sessions *SessionRepository
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
	s.users = &UserRepository{store: s}
	s.sessions = &SessionRepository{store: s}
	s.audit = &AuditRepository{store: s, nodeID: nodeID, hashChain: true}

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
	s.users = &UserRepository{store: s}
	s.sessions = &SessionRepository{store: s}
	s.audit = &AuditRepository{store: s, nodeID: nodeID, hashChain: true}

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

// Users returns the user repository.
func (s *Store) Users() storage.UserRepository {
	return s.users
}

// Sessions returns the session repository.
func (s *Store) Sessions() storage.SessionRepository {
	return s.sessions
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

// Migrate runs database migrations using golang-migrate.
// Use storage.RunMigrations() instead for new migration system.
func (s *Store) Migrate(ctx context.Context) error {
	return fmt.Errorf("legacy Migrate() deprecated - use storage.RunMigrations() with the new migration framework")
}

// Stats returns storage statistics for the PostgreSQL database.
func (s *Store) Stats(ctx context.Context) (storage.StorageStats, error) {
	stats := storage.StorageStats{
		Healthy: true,
		Message: "PostgreSQL storage operational",
	}

	// Check database health first
	if err := s.Ping(ctx); err != nil {
		stats.Healthy = false
		stats.Message = fmt.Sprintf("database ping failed: %v", err)
		return stats, nil
	}

	// Get dataset count
	var datasetCount int64
	err := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM datasets WHERE status != 'deleted'").Scan(&datasetCount)
	if err != nil {
		// Table might not exist yet
		datasetCount = 0
	}
	stats.DatasetCount = datasetCount

	// Get topic count
	var topicCount int64
	err = s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM topics WHERE status != 'deleted'").Scan(&topicCount)
	if err != nil {
		topicCount = 0
	}
	stats.TopicCount = topicCount

	// Get database size using pg_database_size
	var dbSize int64
	err = s.pool.QueryRow(ctx, "SELECT pg_database_size(current_database())").Scan(&dbSize)
	if err != nil {
		dbSize = 0
	}
	stats.BytesUsed = dbSize

	// Get available tablespace size (if possible)
	// This queries the data directory size which requires superuser in some configs
	// For now, we use a reasonable approach - check disk space of the data directory
	dataDir := s.DataDir()
	if dataDir != "" {
		stats.BytesAvailable = getAvailableSpace(dataDir)
	}

	return stats, nil
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

// ConnString returns a stdlib-compatible connection string for migrations.
// This is used by golang-migrate which requires database/sql.
func (s *Store) ConnString() string {
	cfg := s.pool.Config().ConnConfig
	sslMode := "disable"
	if cfg.TLSConfig != nil {
		sslMode = "require"
	}

	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
		sslMode,
	)
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

// init registers the PostgreSQL store factory with the storage package.
func init() {
	storage.OpenPostgres = func(ctx context.Context, cfg storage.PostgresConfig, dataDir, nodeID string) (storage.Store, error) {
		return New(ctx, cfg, dataDir, nodeID)
	}
}
