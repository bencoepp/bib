package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"bib/internal/storage"

	"github.com/jackc/pgx/v5"
)

// AuditRepository implements storage.AuditRepository for PostgreSQL.
type AuditRepository struct {
	store     *Store
	lastHash  string
	hashChain bool
}

// Log records an audit entry.
func (r *AuditRepository) Log(ctx context.Context, entry *storage.AuditEntry) error {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	// Calculate hash chain
	if r.hashChain {
		entry.PrevHash = r.lastHash
	}
	entry.EntryHash = r.calculateHash(entry)

	// Use direct pool to avoid recursive audit logging
	row := r.store.pool.QueryRow(ctx, `
		INSERT INTO audit_log (timestamp, node_id, job_id, operation_id, role_used, action, table_name, query_hash, rows_affected, duration_ms, source_component, metadata, prev_hash, entry_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id
	`,
		entry.Timestamp,
		entry.NodeID,
		nullableString(entry.JobID),
		entry.OperationID,
		entry.RoleUsed,
		entry.Action,
		nullableString(entry.TableName),
		nullableString(entry.QueryHash),
		entry.RowsAffected,
		entry.DurationMS,
		entry.SourceComponent,
		entry.Metadata,
		nullableString(entry.PrevHash),
		entry.EntryHash,
	)

	var id int64
	if err := row.Scan(&id); err != nil {
		return fmt.Errorf("failed to log audit entry: %w", err)
	}

	entry.ID = id

	if r.hashChain {
		r.lastHash = entry.EntryHash
	}

	return nil
}

// Query retrieves audit entries matching the filter.
func (r *AuditRepository) Query(ctx context.Context, filter storage.AuditFilter) ([]*storage.AuditEntry, error) {
	query := `
		SELECT id, timestamp, node_id, job_id, operation_id, role_used, action, table_name, query_hash, rows_affected, duration_ms, source_component, metadata, prev_hash, entry_hash
		FROM audit_log WHERE 1=1
	`
	args := []any{}
	argNum := 1

	if filter.NodeID != "" {
		query += fmt.Sprintf(" AND node_id = $%d", argNum)
		args = append(args, filter.NodeID)
		argNum++
	}

	if filter.JobID != "" {
		query += fmt.Sprintf(" AND job_id = $%d", argNum)
		args = append(args, filter.JobID)
		argNum++
	}

	if filter.OperationID != "" {
		query += fmt.Sprintf(" AND operation_id = $%d", argNum)
		args = append(args, filter.OperationID)
		argNum++
	}

	if filter.Action != "" {
		query += fmt.Sprintf(" AND action = $%d", argNum)
		args = append(args, filter.Action)
		argNum++
	}

	if filter.TableName != "" {
		query += fmt.Sprintf(" AND table_name = $%d", argNum)
		args = append(args, filter.TableName)
		argNum++
	}

	if filter.RoleUsed != "" {
		query += fmt.Sprintf(" AND role_used = $%d", argNum)
		args = append(args, filter.RoleUsed)
		argNum++
	}

	if filter.After != nil {
		query += fmt.Sprintf(" AND timestamp >= $%d", argNum)
		args = append(args, *filter.After)
		argNum++
	}

	if filter.Before != nil {
		query += fmt.Sprintf(" AND timestamp <= $%d", argNum)
		args = append(args, *filter.Before)
		argNum++
	}

	query += " ORDER BY id DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argNum)
		args = append(args, filter.Limit)
		argNum++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argNum)
		args = append(args, filter.Offset)
		argNum++
	}

	rows, err := r.store.pool.Query(ctx, query, args...)
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
	argNum := 1

	if filter.NodeID != "" {
		query += fmt.Sprintf(" AND node_id = $%d", argNum)
		args = append(args, filter.NodeID)
		argNum++
	}

	if filter.JobID != "" {
		query += fmt.Sprintf(" AND job_id = $%d", argNum)
		args = append(args, filter.JobID)
		argNum++
	}

	if filter.Action != "" {
		query += fmt.Sprintf(" AND action = $%d", argNum)
		args = append(args, filter.Action)
		argNum++
	}

	if filter.After != nil {
		query += fmt.Sprintf(" AND timestamp >= $%d", argNum)
		args = append(args, *filter.After)
		argNum++
	}

	if filter.Before != nil {
		query += fmt.Sprintf(" AND timestamp <= $%d", argNum)
		args = append(args, *filter.Before)
		argNum++
	}

	rows, err := r.store.pool.Query(ctx, query, args...)
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
	// This will fail due to the append-only trigger
	// In production, implement archival to cold storage before purging
	result, err := r.store.pool.Exec(ctx, `
		DELETE FROM audit_log WHERE timestamp < $1
	`, before)

	if err != nil {
		return 0, fmt.Errorf("failed to purge audit log: %w", err)
	}

	return result.RowsAffected(), nil
}

// VerifyChain verifies the hash chain integrity between two entry IDs.
func (r *AuditRepository) VerifyChain(ctx context.Context, from, to int64) (bool, error) {
	rows, err := r.store.pool.Query(ctx, `
		SELECT id, timestamp, node_id, job_id, operation_id, role_used, action, table_name, query_hash, rows_affected, duration_ms, source_component, metadata, prev_hash, entry_hash
		FROM audit_log WHERE id >= $1 AND id <= $2 ORDER BY id ASC
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

func scanAuditEntry(rows pgx.Rows) (*storage.AuditEntry, error) {
	var (
		id              int64
		timestamp       interface{}
		nodeID          string
		jobID           *string
		operationID     string
		roleUsed        string
		action          string
		tableName       *string
		queryHash       *string
		rowsAffected    *int
		durationMS      *int
		sourceComponent string
		metadata        map[string]any
		prevHash        *string
		entryHash       string
	)

	err := rows.Scan(
		&id, &timestamp, &nodeID, &jobID, &operationID, &roleUsed,
		&action, &tableName, &queryHash, &rowsAffected, &durationMS,
		&sourceComponent, &metadata, &prevHash, &entryHash,
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
		Metadata:        metadata,
		EntryHash:       entryHash,
	}

	entry.Timestamp = parseTime(timestamp)

	if jobID != nil {
		entry.JobID = *jobID
	}

	if tableName != nil {
		entry.TableName = *tableName
	}

	if queryHash != nil {
		entry.QueryHash = *queryHash
	}

	if rowsAffected != nil {
		entry.RowsAffected = *rowsAffected
	}

	if durationMS != nil {
		entry.DurationMS = *durationMS
	}

	if prevHash != nil {
		entry.PrevHash = *prevHash
	}

	return entry, nil
}

// Ensure interface compliance
var _ storage.AuditRepository = (*AuditRepository)(nil)
