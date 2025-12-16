package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"bib/internal/storage"
)

// AuditRepository implements storage.AuditRepository for SQLite.
type AuditRepository struct {
	store     *Store
	lastHash  string
	hashChain bool
}

// Log records an audit entry.
func (r *AuditRepository) Log(ctx context.Context, entry *storage.AuditEntry) error {
	// Set timestamp if not set
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	metadataJSON, _ := json.Marshal(entry.Metadata)

	// Calculate hash chain
	if r.hashChain {
		entry.PrevHash = r.lastHash
		entry.EntryHash = r.calculateHash(entry)
	} else {
		entry.EntryHash = r.calculateHash(entry)
	}

	// Use direct exec to avoid recursive audit logging
	result, err := r.store.db.ExecContext(ctx, `
		INSERT INTO audit_log (timestamp, node_id, job_id, operation_id, role_used, action, table_name, query_hash, rows_affected, duration_ms, source_component, metadata, prev_hash, entry_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		entry.Timestamp.UTC().Format(time.RFC3339Nano),
		entry.NodeID,
		nullString(entry.JobID),
		entry.OperationID,
		entry.RoleUsed,
		entry.Action,
		nullString(entry.TableName),
		nullString(entry.QueryHash),
		entry.RowsAffected,
		entry.DurationMS,
		entry.SourceComponent,
		string(metadataJSON),
		nullString(entry.PrevHash),
		entry.EntryHash,
	)

	if err != nil {
		return fmt.Errorf("failed to log audit entry: %w", err)
	}

	// Update last hash and get the ID
	if r.hashChain {
		r.lastHash = entry.EntryHash
	}

	id, _ := result.LastInsertId()
	entry.ID = id

	return nil
}

// Query retrieves audit entries matching the filter.
func (r *AuditRepository) Query(ctx context.Context, filter storage.AuditFilter) ([]*storage.AuditEntry, error) {
	query := `
		SELECT id, timestamp, node_id, job_id, operation_id, role_used, action, table_name, query_hash, rows_affected, duration_ms, source_component, metadata, prev_hash, entry_hash
		FROM audit_log WHERE 1=1
	`
	args := []any{}

	if filter.NodeID != "" {
		query += " AND node_id = ?"
		args = append(args, filter.NodeID)
	}

	if filter.JobID != "" {
		query += " AND job_id = ?"
		args = append(args, filter.JobID)
	}

	if filter.OperationID != "" {
		query += " AND operation_id = ?"
		args = append(args, filter.OperationID)
	}

	if filter.Action != "" {
		query += " AND action = ?"
		args = append(args, filter.Action)
	}

	if filter.TableName != "" {
		query += " AND table_name = ?"
		args = append(args, filter.TableName)
	}

	if filter.RoleUsed != "" {
		query += " AND role_used = ?"
		args = append(args, filter.RoleUsed)
	}

	if filter.After != nil {
		query += " AND timestamp >= ?"
		args = append(args, filter.After.UTC().Format(time.RFC3339Nano))
	}

	if filter.Before != nil {
		query += " AND timestamp <= ?"
		args = append(args, filter.Before.UTC().Format(time.RFC3339Nano))
	}

	query += " ORDER BY id DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := r.store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit log: %w", err)
	}
	defer rows.Close()

	var entries []*storage.AuditEntry
	for rows.Next() {
		entry, err := scanAuditEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// Count returns the number of entries matching the filter.
func (r *AuditRepository) Count(ctx context.Context, filter storage.AuditFilter) (int64, error) {
	query := "SELECT COUNT(*) FROM audit_log WHERE 1=1"
	args := []any{}

	if filter.NodeID != "" {
		query += " AND node_id = ?"
		args = append(args, filter.NodeID)
	}

	if filter.JobID != "" {
		query += " AND job_id = ?"
		args = append(args, filter.JobID)
	}

	if filter.Action != "" {
		query += " AND action = ?"
		args = append(args, filter.Action)
	}

	if filter.After != nil {
		query += " AND timestamp >= ?"
		args = append(args, filter.After.UTC().Format(time.RFC3339Nano))
	}

	if filter.Before != nil {
		query += " AND timestamp <= ?"
		args = append(args, filter.Before.UTC().Format(time.RFC3339Nano))
	}

	rows, err := r.store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to count audit entries: %w", err)
	}
	defer rows.Close()

	var count int64
	if rows.Next() {
		if err := rows.Scan(&count); err != nil {
			return 0, err
		}
	}

	return count, nil
}

// GetByOperationID retrieves all entries for an operation.
func (r *AuditRepository) GetByOperationID(ctx context.Context, operationID string) ([]*storage.AuditEntry, error) {
	return r.Query(ctx, storage.AuditFilter{OperationID: operationID})
}

// GetByJobID retrieves all entries for a job.
func (r *AuditRepository) GetByJobID(ctx context.Context, jobID string) ([]*storage.AuditEntry, error) {
	return r.Query(ctx, storage.AuditFilter{JobID: jobID})
}

// Purge removes entries older than the specified time.
func (r *AuditRepository) Purge(ctx context.Context, before time.Time) (int64, error) {
	// Note: This bypasses the append-only trigger by dropping and recreating the trigger
	// In production, you might want to handle this differently (e.g., archive to cold storage)

	result, err := r.store.db.ExecContext(ctx, `
		DELETE FROM audit_log WHERE timestamp < ?
	`, before.UTC().Format(time.RFC3339Nano))

	if err != nil {
		// If delete fails due to trigger, we need a different approach
		// For now, just return the error
		return 0, fmt.Errorf("failed to purge audit log: %w", err)
	}

	return result.RowsAffected()
}

// VerifyChain verifies the hash chain integrity between two entry IDs.
func (r *AuditRepository) VerifyChain(ctx context.Context, from, to int64) (bool, error) {
	rows, err := r.store.db.QueryContext(ctx, `
		SELECT id, timestamp, node_id, job_id, operation_id, role_used, action, table_name, query_hash, rows_affected, duration_ms, source_component, metadata, prev_hash, entry_hash
		FROM audit_log WHERE id >= ? AND id <= ? ORDER BY id ASC
	`, from, to)
	if err != nil {
		return false, fmt.Errorf("failed to query audit log for verification: %w", err)
	}
	defer rows.Close()

	var prevHash string
	for rows.Next() {
		entry, err := scanAuditEntry(rows)
		if err != nil {
			return false, err
		}

		// Verify previous hash matches
		if entry.PrevHash != prevHash {
			return false, nil
		}

		// Verify entry hash
		calculatedHash := r.calculateHash(entry)
		if entry.EntryHash != calculatedHash {
			return false, nil
		}

		prevHash = entry.EntryHash
	}

	return true, rows.Err()
}

// calculateHash calculates the hash for an audit entry.
func (r *AuditRepository) calculateHash(entry *storage.AuditEntry) string {
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%d|%d|%s|%s",
		entry.Timestamp.UTC().Format(time.RFC3339Nano),
		entry.NodeID,
		entry.OperationID,
		entry.RoleUsed,
		entry.Action,
		entry.TableName,
		entry.SourceComponent,
		entry.RowsAffected,
		entry.DurationMS,
		entry.PrevHash,
		entry.JobID,
	)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// Helper functions

func scanAuditEntry(rows *sql.Rows) (*storage.AuditEntry, error) {
	var (
		id              int64
		timestamp       string
		nodeID          string
		jobID           sql.NullString
		operationID     string
		roleUsed        string
		action          string
		tableName       sql.NullString
		queryHash       sql.NullString
		rowsAffected    sql.NullInt64
		durationMS      sql.NullInt64
		sourceComponent string
		metadataJSON    sql.NullString
		prevHash        sql.NullString
		entryHash       string
	)

	err := rows.Scan(
		&id, &timestamp, &nodeID, &jobID, &operationID, &roleUsed,
		&action, &tableName, &queryHash, &rowsAffected, &durationMS,
		&sourceComponent, &metadataJSON, &prevHash, &entryHash,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan audit entry: %w", err)
	}

	entry := &storage.AuditEntry{
		ID:              id,
		NodeID:          nodeID,
		OperationID:     operationID,
		RoleUsed:        roleUsed,
		Action:          action,
		SourceComponent: sourceComponent,
		EntryHash:       entryHash,
	}

	if t, err := time.Parse(time.RFC3339Nano, timestamp); err == nil {
		entry.Timestamp = t
	}

	if jobID.Valid {
		entry.JobID = jobID.String
	}

	if tableName.Valid {
		entry.TableName = tableName.String
	}

	if queryHash.Valid {
		entry.QueryHash = queryHash.String
	}

	if rowsAffected.Valid {
		entry.RowsAffected = int(rowsAffected.Int64)
	}

	if durationMS.Valid {
		entry.DurationMS = int(durationMS.Int64)
	}

	if prevHash.Valid {
		entry.PrevHash = prevHash.String
	}

	if metadataJSON.Valid && metadataJSON.String != "" {
		var metadata map[string]any
		if err := json.Unmarshal([]byte(metadataJSON.String), &metadata); err == nil {
			entry.Metadata = metadata
		}
	}

	return entry, nil
}

// Ensure interface compliance
var _ storage.AuditRepository = (*AuditRepository)(nil)
