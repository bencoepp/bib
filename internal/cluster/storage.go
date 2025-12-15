package cluster

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"bib/internal/config"

	_ "modernc.org/sqlite"
)

// Storage provides persistent storage for Raft logs, metadata, and snapshots using SQLite
type Storage struct {
	db      *sql.DB
	dataDir string
	cfg     config.ClusterConfig
	mu      sync.RWMutex
}

// LogEntry represents a Raft log entry
type LogEntry struct {
	Index uint64 `json:"index"`
	Term  uint64 `json:"term"`
	Type  uint8  `json:"type"`
	Data  []byte `json:"data"`
}

// SnapshotMeta represents snapshot metadata
type SnapshotMeta struct {
	ID            string    `json:"id"`
	Index         uint64    `json:"index"`
	Term          uint64    `json:"term"`
	Configuration []byte    `json:"configuration"`
	Size          int64     `json:"size"`
	CreatedAt     time.Time `json:"created_at"`
}

// HardState represents persistent Raft state
type HardState struct {
	Term   uint64 `json:"term"`
	Vote   string `json:"vote"`
	Commit uint64 `json:"commit"`
}

// NewStorage creates a new SQLite-backed storage for Raft
func NewStorage(cfg config.ClusterConfig, configDir string) (*Storage, error) {
	dataDir := cfg.DataDir
	if dataDir == "" {
		dataDir = filepath.Join(configDir, "raft")
	}

	// Create directories
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create raft data directory: %w", err)
	}

	snapshotDir := filepath.Join(dataDir, "snapshots")
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "raft.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open raft storage: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	s := &Storage{
		db:      db,
		dataDir: dataDir,
		cfg:     cfg,
	}

	if err := s.init(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

// init creates the necessary tables
func (s *Storage) init() error {
	schema := `
	-- Raft log entries
	CREATE TABLE IF NOT EXISTS raft_log (
		log_index INTEGER PRIMARY KEY,
		term INTEGER NOT NULL,
		entry_type INTEGER NOT NULL,
		data BLOB,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Raft hard state (persistent state)
	CREATE TABLE IF NOT EXISTS raft_state (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		term INTEGER NOT NULL DEFAULT 0,
		vote TEXT DEFAULT '',
		commit_index INTEGER NOT NULL DEFAULT 0,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Snapshot metadata
	CREATE TABLE IF NOT EXISTS snapshots (
		id TEXT PRIMARY KEY,
		log_index INTEGER NOT NULL,
		term INTEGER NOT NULL,
		configuration BLOB,
		size INTEGER NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Cluster membership
	CREATE TABLE IF NOT EXISTS cluster_members (
		node_id TEXT PRIMARY KEY,
		address TEXT NOT NULL,
		role TEXT NOT NULL,
		peer_id TEXT,
		joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Join tokens
	CREATE TABLE IF NOT EXISTS join_tokens (
		token TEXT PRIMARY KEY,
		cluster_name TEXT NOT NULL,
		leader_addr TEXT NOT NULL,
		expires_at TIMESTAMP NOT NULL,
		used INTEGER DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Replicated metadata: catalog entries
	CREATE TABLE IF NOT EXISTS replicated_catalog (
		topic_id TEXT NOT NULL,
		dataset_id TEXT NOT NULL,
		hash TEXT NOT NULL,
		size INTEGER NOT NULL,
		chunk_count INTEGER NOT NULL,
		owner_peer_id TEXT NOT NULL,
		updated_at TIMESTAMP NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (topic_id, dataset_id)
	);

	-- Replicated metadata: job assignments
	CREATE TABLE IF NOT EXISTS replicated_jobs (
		job_id TEXT PRIMARY KEY,
		job_type TEXT NOT NULL,
		assigned_node TEXT,
		status TEXT NOT NULL,
		priority INTEGER DEFAULT 0,
		cel_expression TEXT,
		metadata BLOB,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Replicated metadata: global configuration
	CREATE TABLE IF NOT EXISTS replicated_config (
		key TEXT PRIMARY KEY,
		value BLOB NOT NULL,
		version INTEGER NOT NULL DEFAULT 1,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Initialize hard state if not exists
	INSERT OR IGNORE INTO raft_state (id, term, vote, commit_index) VALUES (1, 0, '', 0);

	-- Indexes
	CREATE INDEX IF NOT EXISTS idx_raft_log_term ON raft_log(term);
	CREATE INDEX IF NOT EXISTS idx_snapshots_index ON snapshots(log_index);
	CREATE INDEX IF NOT EXISTS idx_join_tokens_expires ON join_tokens(expires_at);
	CREATE INDEX IF NOT EXISTS idx_replicated_jobs_status ON replicated_jobs(status);
	CREATE INDEX IF NOT EXISTS idx_replicated_jobs_assigned ON replicated_jobs(assigned_node);
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create raft storage schema: %w", err)
	}

	return nil
}

// Close closes the storage
func (s *Storage) Close() error {
	return s.db.Close()
}

// --- Log Operations ---

// FirstIndex returns the first log index
func (s *Storage) FirstIndex() (uint64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var index sql.NullInt64
	err := s.db.QueryRow("SELECT MIN(log_index) FROM raft_log").Scan(&index)
	if err != nil {
		return 0, err
	}
	if !index.Valid {
		return 0, nil
	}
	return uint64(index.Int64), nil
}

// LastIndex returns the last log index
func (s *Storage) LastIndex() (uint64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var index sql.NullInt64
	err := s.db.QueryRow("SELECT MAX(log_index) FROM raft_log").Scan(&index)
	if err != nil {
		return 0, err
	}
	if !index.Valid {
		return 0, nil
	}
	return uint64(index.Int64), nil
}

// GetLog retrieves a log entry by index
func (s *Storage) GetLog(index uint64) (*LogEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var entry LogEntry
	err := s.db.QueryRow(
		"SELECT log_index, term, entry_type, data FROM raft_log WHERE log_index = ?",
		index,
	).Scan(&entry.Index, &entry.Term, &entry.Type, &entry.Data)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

// StoreLog stores a log entry
func (s *Storage) StoreLog(entry *LogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO raft_log (log_index, term, entry_type, data) VALUES (?, ?, ?, ?)",
		entry.Index, entry.Term, entry.Type, entry.Data,
	)
	return err
}

// StoreLogs stores multiple log entries
func (s *Storage) StoreLogs(entries []*LogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("INSERT OR REPLACE INTO raft_log (log_index, term, entry_type, data) VALUES (?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, entry := range entries {
		if _, err := stmt.Exec(entry.Index, entry.Term, entry.Type, entry.Data); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// DeleteRange deletes log entries in a range (inclusive)
func (s *Storage) DeleteRange(min, max uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("DELETE FROM raft_log WHERE log_index >= ? AND log_index <= ?", min, max)
	return err
}

// --- Hard State Operations ---

// GetHardState retrieves the hard state
func (s *Storage) GetHardState() (*HardState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var state HardState
	err := s.db.QueryRow(
		"SELECT term, vote, commit_index FROM raft_state WHERE id = 1",
	).Scan(&state.Term, &state.Vote, &state.Commit)

	if err == sql.ErrNoRows {
		return &HardState{}, nil
	}
	if err != nil {
		return nil, err
	}
	return &state, nil
}

// SetHardState saves the hard state
func (s *Storage) SetHardState(state *HardState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		"UPDATE raft_state SET term = ?, vote = ?, commit_index = ?, updated_at = CURRENT_TIMESTAMP WHERE id = 1",
		state.Term, state.Vote, state.Commit,
	)
	return err
}

// --- Snapshot Operations ---

// CreateSnapshot creates a new snapshot
func (s *Storage) CreateSnapshot(index, term uint64, configuration []byte, data []byte) (*SnapshotMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("%d-%d-%d", term, index, time.Now().UnixNano())

	// Write snapshot data to file
	snapshotPath := filepath.Join(s.dataDir, "snapshots", id+".snap")
	if err := os.WriteFile(snapshotPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write snapshot file: %w", err)
	}

	// Store metadata
	_, err := s.db.Exec(
		"INSERT INTO snapshots (id, log_index, term, configuration, size) VALUES (?, ?, ?, ?, ?)",
		id, index, term, configuration, len(data),
	)
	if err != nil {
		os.Remove(snapshotPath)
		return nil, err
	}

	// Cleanup old snapshots
	go s.cleanupSnapshots()

	return &SnapshotMeta{
		ID:            id,
		Index:         index,
		Term:          term,
		Configuration: configuration,
		Size:          int64(len(data)),
		CreatedAt:     time.Now(),
	}, nil
}

// GetLatestSnapshot returns the latest snapshot metadata
func (s *Storage) GetLatestSnapshot() (*SnapshotMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var meta SnapshotMeta
	err := s.db.QueryRow(
		"SELECT id, log_index, term, configuration, size, created_at FROM snapshots ORDER BY log_index DESC LIMIT 1",
	).Scan(&meta.ID, &meta.Index, &meta.Term, &meta.Configuration, &meta.Size, &meta.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &meta, nil
}

// ReadSnapshot reads snapshot data
func (s *Storage) ReadSnapshot(id string) ([]byte, error) {
	snapshotPath := filepath.Join(s.dataDir, "snapshots", id+".snap")
	return os.ReadFile(snapshotPath)
}

// ListSnapshots returns all snapshot metadata
func (s *Storage) ListSnapshots() ([]SnapshotMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		"SELECT id, log_index, term, configuration, size, created_at FROM snapshots ORDER BY log_index DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []SnapshotMeta
	for rows.Next() {
		var meta SnapshotMeta
		if err := rows.Scan(&meta.ID, &meta.Index, &meta.Term, &meta.Configuration, &meta.Size, &meta.CreatedAt); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, meta)
	}
	return snapshots, rows.Err()
}

// cleanupSnapshots removes old snapshots beyond retention count
func (s *Storage) cleanupSnapshots() {
	s.mu.Lock()
	defer s.mu.Unlock()

	retainCount := s.cfg.Snapshot.RetainCount
	if retainCount <= 0 {
		retainCount = 3
	}

	rows, err := s.db.Query(
		"SELECT id FROM snapshots ORDER BY log_index DESC LIMIT -1 OFFSET ?",
		retainCount,
	)
	if err != nil {
		return
	}
	defer rows.Close()

	var idsToDelete []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		idsToDelete = append(idsToDelete, id)
	}

	for _, id := range idsToDelete {
		snapshotPath := filepath.Join(s.dataDir, "snapshots", id+".snap")
		os.Remove(snapshotPath)
		s.db.Exec("DELETE FROM snapshots WHERE id = ?", id)
	}
}

// --- Cluster Membership Operations ---

// AddMember adds or updates a cluster member
func (s *Storage) AddMember(member *ClusterMember) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		INSERT INTO cluster_members (node_id, address, role, peer_id, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(node_id) DO UPDATE SET
			address = excluded.address,
			role = excluded.role,
			peer_id = excluded.peer_id,
			updated_at = CURRENT_TIMESTAMP
	`, member.NodeID, member.Address, string(member.Role), member.PeerID)
	return err
}

// RemoveMember removes a cluster member
func (s *Storage) RemoveMember(nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("DELETE FROM cluster_members WHERE node_id = ?", nodeID)
	return err
}

// GetMembers returns all cluster members
func (s *Storage) GetMembers() ([]ClusterMember, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query("SELECT node_id, address, role, peer_id FROM cluster_members")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []ClusterMember
	for rows.Next() {
		var m ClusterMember
		var role string
		var peerID sql.NullString
		if err := rows.Scan(&m.NodeID, &m.Address, &role, &peerID); err != nil {
			return nil, err
		}
		m.Role = NodeRole(role)
		if peerID.Valid {
			m.PeerID = peerID.String
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// --- Join Token Operations ---

// StoreJoinToken stores a join token
func (s *Storage) StoreJoinToken(token *JoinToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		INSERT INTO join_tokens (token, cluster_name, leader_addr, expires_at)
		VALUES (?, ?, ?, ?)
	`, token.Token, token.ClusterName, token.LeaderAddr, token.ExpiresAt)
	return err
}

// ValidateJoinToken validates a join token
func (s *Storage) ValidateJoinToken(token string) (*JoinToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var jt JoinToken
	var used int
	err := s.db.QueryRow(`
		SELECT token, cluster_name, leader_addr, expires_at, used
		FROM join_tokens WHERE token = ?
	`, token).Scan(&jt.Token, &jt.ClusterName, &jt.LeaderAddr, &jt.ExpiresAt, &used)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("invalid join token")
	}
	if err != nil {
		return nil, err
	}

	if used != 0 {
		return nil, fmt.Errorf("join token already used")
	}

	if time.Now().After(jt.ExpiresAt) {
		return nil, fmt.Errorf("join token expired")
	}

	// Mark token as used
	_, err = s.db.Exec("UPDATE join_tokens SET used = 1 WHERE token = ?", token)
	if err != nil {
		return nil, err
	}

	return &jt, nil
}

// CleanupExpiredTokens removes expired join tokens
func (s *Storage) CleanupExpiredTokens() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("DELETE FROM join_tokens WHERE expires_at < ?", time.Now())
	return err
}

// --- Replicated Catalog Operations ---

// CatalogEntry represents a replicated catalog entry
type ReplicatedCatalogEntry struct {
	TopicID     string    `json:"topic_id"`
	DatasetID   string    `json:"dataset_id"`
	Hash        string    `json:"hash"`
	Size        int64     `json:"size"`
	ChunkCount  int       `json:"chunk_count"`
	OwnerPeerID string    `json:"owner_peer_id"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// StoreCatalogEntry stores a catalog entry
func (s *Storage) StoreCatalogEntry(entry *ReplicatedCatalogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		INSERT INTO replicated_catalog (topic_id, dataset_id, hash, size, chunk_count, owner_peer_id, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(topic_id, dataset_id) DO UPDATE SET
			hash = excluded.hash,
			size = excluded.size,
			chunk_count = excluded.chunk_count,
			owner_peer_id = excluded.owner_peer_id,
			updated_at = excluded.updated_at
	`, entry.TopicID, entry.DatasetID, entry.Hash, entry.Size, entry.ChunkCount, entry.OwnerPeerID, entry.UpdatedAt)
	return err
}

// GetCatalogEntries returns all catalog entries
func (s *Storage) GetCatalogEntries() ([]ReplicatedCatalogEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query("SELECT topic_id, dataset_id, hash, size, chunk_count, owner_peer_id, updated_at FROM replicated_catalog")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []ReplicatedCatalogEntry
	for rows.Next() {
		var e ReplicatedCatalogEntry
		if err := rows.Scan(&e.TopicID, &e.DatasetID, &e.Hash, &e.Size, &e.ChunkCount, &e.OwnerPeerID, &e.UpdatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// --- Replicated Job Operations ---

// ReplicatedJob represents a job replicated across the cluster
type ReplicatedJob struct {
	JobID         string          `json:"job_id"`
	JobType       string          `json:"job_type"`
	AssignedNode  string          `json:"assigned_node"`
	Status        string          `json:"status"`
	Priority      int             `json:"priority"`
	CELExpression string          `json:"cel_expression"`
	Metadata      json.RawMessage `json:"metadata"`
}

// StoreJob stores a job
func (s *Storage) StoreJob(job *ReplicatedJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		INSERT INTO replicated_jobs (job_id, job_type, assigned_node, status, priority, cel_expression, metadata, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(job_id) DO UPDATE SET
			assigned_node = excluded.assigned_node,
			status = excluded.status,
			priority = excluded.priority,
			metadata = excluded.metadata,
			updated_at = CURRENT_TIMESTAMP
	`, job.JobID, job.JobType, job.AssignedNode, job.Status, job.Priority, job.CELExpression, job.Metadata)
	return err
}

// GetUnassignedJobs returns jobs that need assignment
func (s *Storage) GetUnassignedJobs() ([]ReplicatedJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT job_id, job_type, assigned_node, status, priority, cel_expression, metadata
		FROM replicated_jobs
		WHERE status = 'pending' AND (assigned_node IS NULL OR assigned_node = '')
		ORDER BY priority DESC, created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []ReplicatedJob
	for rows.Next() {
		var j ReplicatedJob
		var assignedNode sql.NullString
		var celExpr sql.NullString
		if err := rows.Scan(&j.JobID, &j.JobType, &assignedNode, &j.Status, &j.Priority, &celExpr, &j.Metadata); err != nil {
			return nil, err
		}
		if assignedNode.Valid {
			j.AssignedNode = assignedNode.String
		}
		if celExpr.Valid {
			j.CELExpression = celExpr.String
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// --- Replicated Config Operations ---

// StoreConfig stores a configuration value
func (s *Storage) StoreConfig(key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		INSERT INTO replicated_config (key, value, version, updated_at)
		VALUES (?, ?, 1, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			version = version + 1,
			updated_at = CURRENT_TIMESTAMP
	`, key, value)
	return err
}

// GetConfig retrieves a configuration value
func (s *Storage) GetConfig(key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var value []byte
	err := s.db.QueryRow("SELECT value FROM replicated_config WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return value, err
}
