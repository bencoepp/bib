package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"bib/internal/domain"
	"bib/internal/storage"
)

// SessionRepository implements storage.SessionRepository for SQLite.
type SessionRepository struct {
	store *Store
}

// Create creates a new session.
func (r *SessionRepository) Create(ctx context.Context, session *storage.Session) error {
	metadataJSON, err := json.Marshal(session.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	_, err = r.store.execWithAudit(ctx, "INSERT", "sessions", `
		INSERT INTO sessions (id, user_id, type, client_ip, client_agent, public_key_fingerprint, node_id, started_at, ended_at, last_activity_at, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		session.ID,
		string(session.UserID),
		string(session.Type),
		session.ClientIP,
		nullString(session.ClientAgent),
		session.PublicKeyFingerprint,
		session.NodeID,
		session.StartedAt.UTC().Format(time.RFC3339Nano),
		nullTimeString(session.EndedAt),
		session.LastActivityAt.UTC().Format(time.RFC3339Nano),
		string(metadataJSON),
	)

	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

// Get retrieves a session by ID.
func (r *SessionRepository) Get(ctx context.Context, id string) (*storage.Session, error) {
	rows, err := r.store.queryWithAudit(ctx, "sessions", `
		SELECT id, user_id, type, client_ip, client_agent, public_key_fingerprint, node_id, started_at, ended_at, last_activity_at, metadata
		FROM sessions WHERE id = ?
	`, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query session: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, domain.ErrSessionNotFound
	}

	return scanSession(rows)
}

// GetByUser retrieves all active sessions for a user.
func (r *SessionRepository) GetByUser(ctx context.Context, userID domain.UserID) ([]*storage.Session, error) {
	rows, err := r.store.queryWithAudit(ctx, "sessions", `
		SELECT id, user_id, type, client_ip, client_agent, public_key_fingerprint, node_id, started_at, ended_at, last_activity_at, metadata
		FROM sessions WHERE user_id = ? AND ended_at IS NULL
		ORDER BY started_at DESC
	`, string(userID))
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*storage.Session
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// Update updates an existing session.
func (r *SessionRepository) Update(ctx context.Context, session *storage.Session) error {
	metadataJSON, err := json.Marshal(session.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	result, err := r.store.execWithAudit(ctx, "UPDATE", "sessions", `
		UPDATE sessions SET
			ended_at = ?,
			last_activity_at = ?,
			metadata = ?
		WHERE id = ?
	`,
		nullTimeString(session.EndedAt),
		session.LastActivityAt.UTC().Format(time.RFC3339Nano),
		string(metadataJSON),
		session.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return domain.ErrSessionNotFound
	}

	return nil
}

// End marks a session as ended.
func (r *SessionRepository) End(ctx context.Context, id string) error {
	now := time.Now().UTC()
	result, err := r.store.execWithAudit(ctx, "UPDATE", "sessions", `
		UPDATE sessions SET ended_at = ?, last_activity_at = ? WHERE id = ? AND ended_at IS NULL
	`, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), id)
	if err != nil {
		return fmt.Errorf("failed to end session: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return domain.ErrSessionNotFound
	}

	return nil
}

// EndAllForUser ends all sessions for a user.
func (r *SessionRepository) EndAllForUser(ctx context.Context, userID domain.UserID) error {
	now := time.Now().UTC()
	_, err := r.store.execWithAudit(ctx, "UPDATE", "sessions", `
		UPDATE sessions SET ended_at = ?, last_activity_at = ? WHERE user_id = ? AND ended_at IS NULL
	`, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), string(userID))
	if err != nil {
		return fmt.Errorf("failed to end sessions: %w", err)
	}

	return nil
}

// List retrieves sessions matching the filter.
func (r *SessionRepository) List(ctx context.Context, filter storage.SessionFilter) ([]*storage.Session, error) {
	query := `
		SELECT id, user_id, type, client_ip, client_agent, public_key_fingerprint, node_id, started_at, ended_at, last_activity_at, metadata
		FROM sessions WHERE 1=1
	`
	args := []any{}

	if filter.UserID != nil {
		query += " AND user_id = ?"
		args = append(args, string(*filter.UserID))
	}

	if filter.Type != "" {
		query += " AND type = ?"
		args = append(args, string(filter.Type))
	}

	if filter.Active != nil {
		if *filter.Active {
			query += " AND ended_at IS NULL"
		} else {
			query += " AND ended_at IS NOT NULL"
		}
	}

	if filter.ClientIP != "" {
		query += " AND client_ip = ?"
		args = append(args, filter.ClientIP)
	}

	if filter.NodeID != "" {
		query += " AND node_id = ?"
		args = append(args, filter.NodeID)
	}

	if filter.After != nil {
		query += " AND started_at >= ?"
		args = append(args, filter.After.UTC().Format(time.RFC3339Nano))
	}

	if filter.Before != nil {
		query += " AND started_at <= ?"
		args = append(args, filter.Before.UTC().Format(time.RFC3339Nano))
	}

	query += " ORDER BY started_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := r.store.queryWithAudit(ctx, "sessions", query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*storage.Session
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// Cleanup removes expired sessions older than the given time.
func (r *SessionRepository) Cleanup(ctx context.Context, before time.Time) (int64, error) {
	result, err := r.store.execWithAudit(ctx, "DELETE", "sessions", `
		DELETE FROM sessions WHERE ended_at IS NOT NULL AND ended_at < ?
	`, before.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup sessions: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	return rowsAffected, nil
}

// scanSession scans a session row into a Session struct.
func scanSession(rows *sql.Rows) (*storage.Session, error) {
	var (
		session        storage.Session
		userID         string
		sessionType    string
		clientAgent    sql.NullString
		startedAt      string
		endedAt        sql.NullString
		lastActivityAt string
		metadataJSON   sql.NullString
	)

	err := rows.Scan(
		&session.ID,
		&userID,
		&sessionType,
		&session.ClientIP,
		&clientAgent,
		&session.PublicKeyFingerprint,
		&session.NodeID,
		&startedAt,
		&endedAt,
		&lastActivityAt,
		&metadataJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan session: %w", err)
	}

	session.UserID = domain.UserID(userID)
	session.Type = storage.SessionType(sessionType)

	if clientAgent.Valid {
		session.ClientAgent = clientAgent.String
	}

	if t, err := time.Parse(time.RFC3339Nano, startedAt); err == nil {
		session.StartedAt = t
	}
	if endedAt.Valid {
		if t, err := time.Parse(time.RFC3339Nano, endedAt.String); err == nil {
			session.EndedAt = &t
		}
	}
	if t, err := time.Parse(time.RFC3339Nano, lastActivityAt); err == nil {
		session.LastActivityAt = t
	}

	if metadataJSON.Valid && metadataJSON.String != "" {
		if err := json.Unmarshal([]byte(metadataJSON.String), &session.Metadata); err != nil {
			// Ignore metadata parse errors
		}
	}

	return &session, nil
}

// nullTimeString converts a *time.Time to a nullable string for SQLite.
func nullTimeString(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339Nano)
	return &s
}
