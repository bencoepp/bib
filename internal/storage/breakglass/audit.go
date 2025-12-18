package breakglass

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AuditCallbackImpl implements AuditCallback for break glass session tracking.
type AuditCallbackImpl struct {
	pool   *pgxpool.Pool
	nodeID string
}

// NewAuditCallback creates a new audit callback.
func NewAuditCallback(pool *pgxpool.Pool, nodeID string) *AuditCallbackImpl {
	return &AuditCallbackImpl{
		pool:   pool,
		nodeID: nodeID,
	}
}

// LogBreakGlassEvent logs a break glass event to the audit trail.
func (a *AuditCallbackImpl) LogBreakGlassEvent(ctx context.Context, eventType string, session *Session, metadata map[string]any) error {
	if a.pool == nil {
		return nil // No pool configured, skip silently
	}

	// Add break glass specific fields to metadata
	fullMetadata := map[string]any{
		"event_type":      eventType,
		"break_glass":     true,
		"session_id":      session.ID,
		"username":        session.User.Name,
		"access_level":    session.AccessLevel.String(),
		"reason":          session.Reason,
		"custom_metadata": metadata,
	}

	fullMetadataJSON, _ := json.Marshal(fullMetadata)

	// Insert into audit_log with break_glass flag set
	insertSQL := `
		INSERT INTO audit_log (
			timestamp, node_id, operation_id, role_used, action,
			source_component, metadata, flag_break_glass, entry_hash
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, true, $8
		)
	`

	_, err := a.pool.Exec(ctx, insertSQL,
		time.Now().UTC(),
		a.nodeID,
		session.ID,
		"break_glass",
		eventType,
		"breakglass",
		fullMetadataJSON,
		"", // Hash will be computed by trigger if enabled
	)

	return err
}

// GetSessionQueryStats returns query statistics for a break glass session.
func (a *AuditCallbackImpl) GetSessionQueryStats(ctx context.Context, sessionID string) (
	queryCount int64,
	tablesAccessed []string,
	operationCounts map[string]int64,
	err error,
) {
	if a.pool == nil {
		return 0, nil, nil, nil
	}

	// Get total query count for this session
	countSQL := `
		SELECT COUNT(*) 
		FROM audit_log 
		WHERE flag_break_glass = true 
		AND metadata->>'session_id' = $1
	`
	err = a.pool.QueryRow(ctx, countSQL, sessionID).Scan(&queryCount)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("failed to get query count: %w", err)
	}

	// Get distinct tables accessed
	tablesSQL := `
		SELECT DISTINCT table_name 
		FROM audit_log 
		WHERE flag_break_glass = true 
		AND metadata->>'session_id' = $1
		AND table_name IS NOT NULL
		AND table_name != ''
	`
	rows, err := a.pool.Query(ctx, tablesSQL, sessionID)
	if err != nil {
		return queryCount, nil, nil, fmt.Errorf("failed to get tables accessed: %w", err)
	}
	defer rows.Close()

	tablesAccessed = make([]string, 0)
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			continue
		}
		tablesAccessed = append(tablesAccessed, table)
	}

	// Get operation counts by action type
	opsSQL := `
		SELECT action, COUNT(*) 
		FROM audit_log 
		WHERE flag_break_glass = true 
		AND metadata->>'session_id' = $1
		GROUP BY action
	`
	rows, err = a.pool.Query(ctx, opsSQL, sessionID)
	if err != nil {
		return queryCount, tablesAccessed, nil, fmt.Errorf("failed to get operation counts: %w", err)
	}
	defer rows.Close()

	operationCounts = make(map[string]int64)
	for rows.Next() {
		var action string
		var count int64
		if err := rows.Scan(&action, &count); err != nil {
			continue
		}
		operationCounts[action] = count
	}

	return queryCount, tablesAccessed, operationCounts, nil
}

// LogQuery logs an individual query executed during a break glass session.
// This is called for each query to provide full audit trail with no redaction.
func (a *AuditCallbackImpl) LogQuery(ctx context.Context, sessionID string, query string, params []any, tableName string, action string, rowsAffected int, durationMS int) error {
	if a.pool == nil {
		return nil
	}

	// For paranoid audit level, we include full query parameters (no redaction)
	metadata := map[string]any{
		"session_id":    sessionID,
		"query":         query,
		"parameters":    params,
		"rows_affected": rowsAffected,
		"duration_ms":   durationMS,
	}

	metadataJSON, _ := json.Marshal(metadata)

	insertSQL := `
		INSERT INTO audit_log (
			timestamp, node_id, operation_id, role_used, action,
			table_name, query_hash, rows_affected, duration_ms,
			source_component, metadata, flag_break_glass, entry_hash
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, true, $12
		)
	`

	_, err := a.pool.Exec(ctx, insertSQL,
		time.Now().UTC(),
		a.nodeID,
		sessionID,
		"break_glass",
		action,
		tableName,
		hashQuery(query),
		rowsAffected,
		durationMS,
		"breakglass",
		metadataJSON,
		"", // Hash computed by trigger
	)

	return err
}

// hashQuery creates a hash of a query for grouping similar queries.
func hashQuery(query string) string {
	// Simple hash for grouping - in production, use a proper normalization
	// that removes literals and focuses on query structure
	if len(query) > 64 {
		return query[:64]
	}
	return query
}
