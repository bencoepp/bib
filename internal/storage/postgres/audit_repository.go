package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"bib/internal/storage"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AuditRepository implements storage.AuditRepository for PostgreSQL.
type AuditRepository struct {
	pool      *pgxpool.Pool
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
		entry.EntryHash = calculateEntryHash(entry)
	}

	pool := r.pool
	if pool == nil && r.store != nil {
		pool = r.store.pool
	}

	row := pool.QueryRow(ctx, `
		INSERT INTO audit_log (
			timestamp, node_id, job_id, operation_id, role_used, action,
			table_name, query, query_hash, rows_affected, duration_ms,
			source_component, actor, metadata, prev_hash, entry_hash,
			flag_break_glass, flag_rate_limited, flag_suspicious, flag_alert_triggered
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
		RETURNING id
	`,
		entry.Timestamp,
		entry.NodeID,
		nullableString(entry.JobID),
		entry.OperationID,
		entry.RoleUsed,
		entry.Action,
		nullableString(entry.TableName),
		nullableString(entry.Query),
		nullableString(entry.QueryHash),
		entry.RowsAffected,
		entry.DurationMS,
		entry.SourceComponent,
		nullableString(entry.Actor),
		entry.Metadata,
		nullableString(entry.PrevHash),
		entry.EntryHash,
		entry.Flags.BreakGlass,
		entry.Flags.RateLimited,
		entry.Flags.Suspicious,
		entry.Flags.AlertTriggered,
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

	if filter.Actor != "" {
		query += fmt.Sprintf(" AND actor = $%d", argNum)
		args = append(args, filter.Actor)
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

	if filter.Suspicious != nil && *filter.Suspicious {
		query += " AND flag_suspicious = true"
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
	}

	pool := r.pool
	if pool == nil && r.store != nil {
		pool = r.store.pool
	}

	rows, err := pool.Query(ctx, query, args...)
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

// Count returns the number of matching entries.
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
	}

	pool := r.pool
	if pool == nil && r.store != nil {
		pool = r.store.pool
	}

	var count int64
	err := pool.QueryRow(ctx, query, args...).Scan(&count)
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
	pool := r.pool
	if pool == nil && r.store != nil {
		pool = r.store.pool
	}

	result, err := pool.Exec(ctx, `
		DELETE FROM audit_log WHERE timestamp < $1
	`, before)

	if err != nil {
		return 0, fmt.Errorf("failed to purge audit log: %w", err)
	}

	return result.RowsAffected(), nil
}

// VerifyChain verifies hash chain integrity.
func (r *AuditRepository) VerifyChain(ctx context.Context, from, to int64) (bool, error) {
	pool := r.pool
	if pool == nil && r.store != nil {
		pool = r.store.pool
	}

	rows, err := pool.Query(ctx, `
		SELECT id, timestamp, node_id, job_id, operation_id, role_used, action,
			table_name, query, query_hash, rows_affected, duration_ms,
			source_component, actor, metadata, prev_hash, entry_hash,
			flag_break_glass, flag_rate_limited, flag_suspicious, flag_alert_triggered
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

		if entry.PrevHash != prevHash {
			return false, nil
		}

		calculatedHash := calculateEntryHash(entry)
		if entry.EntryHash != calculatedHash {
			return false, nil
		}

		prevHash = entry.EntryHash
	}

	return true, rows.Err()
}

// GetLastHash returns the hash of the last entry.
func (r *AuditRepository) GetLastHash(ctx context.Context) (string, error) {
	pool := r.pool
	if pool == nil && r.store != nil {
		pool = r.store.pool
	}

	var entryHash *string
	err := pool.QueryRow(ctx, `
		SELECT entry_hash FROM audit_log ORDER BY id DESC LIMIT 1
	`).Scan(&entryHash)

	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("failed to get last hash: %w", err)
	}

	if entryHash == nil {
		return "", nil
	}
	return *entryHash, nil
}

// Helper functions

func calculateEntryHash(entry *storage.AuditEntry) string {
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

func scanAuditEntry(rows pgx.Rows) (*storage.AuditEntry, error) {
	var (
		id                 int64
		timestamp          interface{}
		nodeID             string
		jobID              *string
		operationID        string
		roleUsed           string
		action             string
		tableName          *string
		query              *string
		queryHash          *string
		rowsAffected       *int
		durationMS         *int
		sourceComponent    string
		actor              *string
		metadata           map[string]any
		prevHash           *string
		entryHash          string
		flagBreakGlass     bool
		flagRateLimited    bool
		flagSuspicious     bool
		flagAlertTriggered bool
	)

	err := rows.Scan(
		&id, &timestamp, &nodeID, &jobID, &operationID, &roleUsed,
		&action, &tableName, &query, &queryHash, &rowsAffected, &durationMS,
		&sourceComponent, &actor, &metadata, &prevHash, &entryHash,
		&flagBreakGlass, &flagRateLimited, &flagSuspicious, &flagAlertTriggered,
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
		Flags: storage.AuditEntryFlags{
			BreakGlass:     flagBreakGlass,
			RateLimited:    flagRateLimited,
			Suspicious:     flagSuspicious,
			AlertTriggered: flagAlertTriggered,
		},
	}

	entry.Timestamp = parseTime(timestamp)

	if jobID != nil {
		entry.JobID = *jobID
	}
	if tableName != nil {
		entry.TableName = *tableName
	}
	if query != nil {
		entry.Query = *query
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
	if actor != nil {
		entry.Actor = *actor
	}
	if prevHash != nil {
		entry.PrevHash = *prevHash
	}

	return entry, nil
}

// Ensure interface compliance
var _ storage.AuditRepository = (*AuditRepository)(nil)
