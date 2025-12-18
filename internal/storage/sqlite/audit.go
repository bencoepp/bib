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
	nodeID    string
	hashChain bool
	lastHash  string
}

// Log persists an audit entry.
func (r *AuditRepository) Log(ctx context.Context, entry *storage.AuditEntry) error {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	if entry.NodeID == "" {
		entry.NodeID = r.nodeID
	}

	// Calculate hash chain if enabled
	if r.hashChain && entry.EntryHash == "" {
		entry.PrevHash = r.lastHash
		entry.EntryHash = r.calculateEntryHash(entry)
	}

	metadataJSON, _ := json.Marshal(entry.Metadata)

	result, err := r.store.db.ExecContext(ctx, `
		INSERT INTO audit_log (
			timestamp, node_id, job_id, operation_id, role_used, action,
			table_name, query, query_hash, rows_affected, duration_ms,
			source_component, actor, metadata, prev_hash, entry_hash,
			flag_break_glass, flag_rate_limited, flag_suspicious, flag_alert_triggered
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		entry.Timestamp.UTC().Format(time.RFC3339Nano),
		entry.NodeID,
		nullStr(entry.JobID),
		entry.OperationID,
		entry.RoleUsed,
		entry.Action,
		nullStr(entry.TableName),
		nullStr(entry.Query),
		nullStr(entry.QueryHash),
		entry.RowsAffected,
		entry.DurationMS,
		entry.SourceComponent,
		nullStr(entry.Actor),
		string(metadataJSON),
		nullStr(entry.PrevHash),
		entry.EntryHash,
		entry.Flags.BreakGlass,
		entry.Flags.RateLimited,
		entry.Flags.Suspicious,
		entry.Flags.AlertTriggered,
	)

	if err != nil {
		return fmt.Errorf("failed to log audit entry: %w", err)
	}

	id, _ := result.LastInsertId()
	entry.ID = id

	if r.hashChain {
		r.lastHash = entry.EntryHash
	}

	return nil
}

// Query retrieves entries matching a filter.
func (r *AuditRepository) Query(ctx context.Context, filter storage.AuditFilter) ([]*storage.AuditEntry, error) {
	query := `
		SELECT id, timestamp, node_id, job_id, operation_id, role_used, action,
			table_name, query, query_hash, rows_affected, duration_ms,
			source_component, actor, metadata, prev_hash, entry_hash,
			flag_break_glass, flag_rate_limited, flag_suspicious, flag_alert_triggered
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

	if filter.Actor != "" {
		query += " AND actor = ?"
		args = append(args, filter.Actor)
	}

	if filter.After != nil {
		query += " AND timestamp >= ?"
		args = append(args, filter.After.UTC().Format(time.RFC3339Nano))
	}

	if filter.Before != nil {
		query += " AND timestamp <= ?"
		args = append(args, filter.Before.UTC().Format(time.RFC3339Nano))
	}

	if filter.Suspicious != nil && *filter.Suspicious {
		query += " AND flag_suspicious = 1"
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
		entry, err := r.scanAuditEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// Count returns the number of matching entries.
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

	var count int64
	err := r.store.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count audit entries: %w", err)
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

// Purge removes entries older than the given time.
func (r *AuditRepository) Purge(ctx context.Context, before time.Time) (int64, error) {
	result, err := r.store.db.ExecContext(ctx, `
		DELETE FROM audit_log WHERE timestamp < ?
	`, before.UTC().Format(time.RFC3339Nano))

	if err != nil {
		return 0, fmt.Errorf("failed to purge audit log: %w", err)
	}

	count, _ := result.RowsAffected()
	return count, nil
}

// VerifyChain verifies hash chain integrity.
func (r *AuditRepository) VerifyChain(ctx context.Context, from, to int64) (bool, error) {
	rows, err := r.store.db.QueryContext(ctx, `
		SELECT id, timestamp, node_id, job_id, operation_id, role_used, action,
			table_name, query, query_hash, rows_affected, duration_ms,
			source_component, actor, metadata, prev_hash, entry_hash,
			flag_break_glass, flag_rate_limited, flag_suspicious, flag_alert_triggered
		FROM audit_log WHERE id >= ? AND id <= ? ORDER BY id ASC
	`, from, to)
	if err != nil {
		return false, fmt.Errorf("failed to query audit log for verification: %w", err)
	}
	defer rows.Close()

	var prevHash string
	for rows.Next() {
		entry, err := r.scanAuditEntry(rows)
		if err != nil {
			return false, err
		}

		if entry.PrevHash != prevHash {
			return false, nil
		}

		calculatedHash := r.calculateEntryHash(entry)
		if entry.EntryHash != calculatedHash {
			return false, nil
		}

		prevHash = entry.EntryHash
	}

	return true, rows.Err()
}

// GetLastHash returns the hash of the last entry.
func (r *AuditRepository) GetLastHash(ctx context.Context) (string, error) {
	var entryHash sql.NullString
	err := r.store.db.QueryRowContext(ctx, `
		SELECT entry_hash FROM audit_log ORDER BY id DESC LIMIT 1
	`).Scan(&entryHash)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("failed to get last hash: %w", err)
	}

	return entryHash.String, nil
}

// Helper functions

func (r *AuditRepository) calculateEntryHash(entry *storage.AuditEntry) string {
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%d|%d|%s|%s|%s",
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
		entry.QueryHash,
	)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func (r *AuditRepository) scanAuditEntry(rows *sql.Rows) (*storage.AuditEntry, error) {
	var (
		id                 int64
		timestamp          string
		nodeID             string
		jobID              sql.NullString
		operationID        string
		roleUsed           string
		action             string
		tableName          sql.NullString
		query              sql.NullString
		queryHash          sql.NullString
		rowsAffected       sql.NullInt64
		durationMS         sql.NullInt64
		sourceComponent    string
		actor              sql.NullString
		metadataJSON       string
		prevHash           sql.NullString
		entryHash          string
		flagBreakGlass     bool
		flagRateLimited    bool
		flagSuspicious     bool
		flagAlertTriggered bool
	)

	err := rows.Scan(
		&id, &timestamp, &nodeID, &jobID, &operationID, &roleUsed,
		&action, &tableName, &query, &queryHash, &rowsAffected, &durationMS,
		&sourceComponent, &actor, &metadataJSON, &prevHash, &entryHash,
		&flagBreakGlass, &flagRateLimited, &flagSuspicious, &flagAlertTriggered,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan audit entry: %w", err)
	}

	var metadata map[string]any
	if metadataJSON != "" {
		_ = json.Unmarshal([]byte(metadataJSON), &metadata)
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
		Flags: storage.AuditEntryFlags{
			BreakGlass:     flagBreakGlass,
			RateLimited:    flagRateLimited,
			Suspicious:     flagSuspicious,
			AlertTriggered: flagAlertTriggered,
		},
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
	if query.Valid {
		entry.Query = query.String
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
	if actor.Valid {
		entry.Actor = actor.String
	}
	if prevHash.Valid {
		entry.PrevHash = prevHash.String
	}

	return entry, nil
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// Ensure interface compliance
var _ storage.AuditRepository = (*AuditRepository)(nil)
