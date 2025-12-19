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

	topics           *TopicRepository
	datasets         *DatasetRepository
	jobs             *JobRepository
	nodes            *NodeRepository
	users            *UserRepository
	sessions         *SessionRepository
	audit            *AuditRepository
	userPreferences  *UserPreferencesRepository
	topicMembers     *TopicMemberRepository
	topicInvitations *TopicInvitationRepository
	bannedPeers      *BannedPeerRepository
	queryHistory     *QueryHistoryRepository
	savedQueries     *SavedQueryRepository
	allowedPeers     *AllowedPeerRepository

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
	s.users = &UserRepository{store: s}
	s.sessions = &SessionRepository{store: s}
	s.audit = &AuditRepository{store: s, nodeID: nodeID, hashChain: true}
	s.userPreferences = &UserPreferencesRepository{store: s}
	s.topicMembers = &TopicMemberRepository{store: s}
	s.topicInvitations = &TopicInvitationRepository{store: s}
	s.bannedPeers = &BannedPeerRepository{store: s}
	s.queryHistory = &QueryHistoryRepository{store: s}
	s.savedQueries = &SavedQueryRepository{store: s}
	s.allowedPeers = &AllowedPeerRepository{store: s}

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

// UserPreferences returns the user preferences repository.
func (s *Store) UserPreferences() storage.UserPreferencesRepository {
	return s.userPreferences
}

// TopicMembers returns the topic membership repository.
func (s *Store) TopicMembers() storage.TopicMemberRepository {
	return s.topicMembers
}

// TopicInvitations returns the topic invitations repository.
func (s *Store) TopicInvitations() storage.TopicInvitationRepository {
	return s.topicInvitations
}

// BannedPeers returns the banned peers repository.
func (s *Store) BannedPeers() storage.BannedPeerRepository {
	return s.bannedPeers
}

// QueryHistory returns the query history repository.
func (s *Store) QueryHistory() storage.QueryHistoryRepository {
	return s.queryHistory
}

// SavedQueries returns the saved queries repository.
func (s *Store) SavedQueries() storage.SavedQueryRepository {
	return s.savedQueries
}

// AllowedPeers returns the allowed peers repository.
func (s *Store) AllowedPeers() storage.AllowedPeerRepository {
	return s.allowedPeers
}

// Vacuum performs database maintenance.
func (s *Store) Vacuum(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "VACUUM")
	return err
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

// Migrate runs database migrations using golang-migrate.
func (s *Store) Migrate(ctx context.Context) error {
	// Migration system updated - use storage.RunMigrations() instead
	return fmt.Errorf("migration system updated - please use storage.RunMigrations() instead")
}

// Stats returns storage statistics for the SQLite database.
func (s *Store) Stats(ctx context.Context) (storage.StorageStats, error) {
	stats := storage.StorageStats{
		Healthy: true,
		Message: "SQLite storage operational",
	}

	// Check database health first
	if err := s.Ping(ctx); err != nil {
		stats.Healthy = false
		stats.Message = fmt.Sprintf("database ping failed: %v", err)
		return stats, nil
	}

	// Get dataset count
	var datasetCount int64
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM datasets WHERE status != 'deleted'").Scan(&datasetCount)
	if err != nil && err != sql.ErrNoRows {
		// Table might not exist yet
		datasetCount = 0
	}
	stats.DatasetCount = datasetCount

	// Get topic count
	var topicCount int64
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM topics WHERE status != 'deleted'").Scan(&topicCount)
	if err != nil && err != sql.ErrNoRows {
		topicCount = 0
	}
	stats.TopicCount = topicCount

	// Get database file size
	dbPath := s.cfg.Path
	if dbPath == "" {
		// Default path wasn't stored, try to get from pragma
		var dbFile string
		err = s.db.QueryRowContext(ctx, "PRAGMA database_list").Scan(nil, nil, &dbFile)
		if err == nil && dbFile != "" {
			dbPath = dbFile
		}
	}

	if dbPath != "" && dbPath != ":memory:" {
		// Get file size
		if fi, err := os.Stat(dbPath); err == nil {
			stats.BytesUsed = fi.Size()
		}

		// Also count WAL and SHM files
		if walFi, err := os.Stat(dbPath + "-wal"); err == nil {
			stats.BytesUsed += walFi.Size()
		}
		if shmFi, err := os.Stat(dbPath + "-shm"); err == nil {
			stats.BytesUsed += shmFi.Size()
		}

		// Get available disk space (platform-dependent)
		stats.BytesAvailable = getAvailableSpace(filepath.Dir(dbPath))
	}

	return stats, nil
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

// init registers the SQLite store factory with the storage package.
func init() {
	storage.OpenSQLite = func(ctx context.Context, cfg storage.SQLiteConfig, dataDir, nodeID string) (storage.Store, error) {
		return New(cfg, dataDir, nodeID)
	}
}
