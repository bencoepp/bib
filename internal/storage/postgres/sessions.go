package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"bib/internal/domain"
	"bib/internal/storage"

	"github.com/jackc/pgx/v5"
)

// SessionRepository implements storage.SessionRepository for PostgreSQL.
type SessionRepository struct {
	store *Store
}

// Create creates a new session.
func (r *SessionRepository) Create(ctx context.Context, session *storage.Session) error {
	metadataJSON, err := json.Marshal(session.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	_, err = r.store.pool.Exec(ctx, `
		INSERT INTO sessions (id, user_id, type, client_ip, client_agent, public_key_fingerprint, node_id, started_at, ended_at, last_activity_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`,
		session.ID,
		string(session.UserID),
		string(session.Type),
		session.ClientIP,
		nullString(session.ClientAgent),
		session.PublicKeyFingerprint,
		session.NodeID,
		session.StartedAt,
		session.EndedAt,
		session.LastActivityAt,
		metadataJSON,
	)

	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

// Get retrieves a session by ID.
func (r *SessionRepository) Get(ctx context.Context, id string) (*storage.Session, error) {
	row := r.store.pool.QueryRow(ctx, `
		SELECT id, user_id, type, client_ip, client_agent, public_key_fingerprint, node_id, started_at, ended_at, last_activity_at, metadata
		FROM sessions WHERE id = $1
	`, id)

	return scanSessionRow(row)
}

// GetByUser retrieves all active sessions for a user.
func (r *SessionRepository) GetByUser(ctx context.Context, userID domain.UserID) ([]*storage.Session, error) {
	rows, err := r.store.pool.Query(ctx, `
		SELECT id, user_id, type, client_ip, client_agent, public_key_fingerprint, node_id, started_at, ended_at, last_activity_at, metadata
		FROM sessions WHERE user_id = $1 AND ended_at IS NULL
		ORDER BY started_at DESC
	`, string(userID))
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*storage.Session
	for rows.Next() {
		session, err := scanSessionRows(rows)
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

	result, err := r.store.pool.Exec(ctx, `
		UPDATE sessions SET
			ended_at = $1,
			last_activity_at = $2,
			metadata = $3
		WHERE id = $4
	`,
		session.EndedAt,
		session.LastActivityAt,
		metadataJSON,
		session.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrSessionNotFound
	}

	return nil
}

// End marks a session as ended.
func (r *SessionRepository) End(ctx context.Context, id string) error {
	now := time.Now().UTC()
	result, err := r.store.pool.Exec(ctx, `
		UPDATE sessions SET ended_at = $1, last_activity_at = $1 WHERE id = $2 AND ended_at IS NULL
	`, now, id)
	if err != nil {
		return fmt.Errorf("failed to end session: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrSessionNotFound
	}

	return nil
}

// EndAllForUser ends all sessions for a user.
func (r *SessionRepository) EndAllForUser(ctx context.Context, userID domain.UserID) error {
	now := time.Now().UTC()
	_, err := r.store.pool.Exec(ctx, `
		UPDATE sessions SET ended_at = $1, last_activity_at = $1 WHERE user_id = $2 AND ended_at IS NULL
	`, now, string(userID))
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
	argIdx := 1

	if filter.UserID != nil {
		query += fmt.Sprintf(" AND user_id = $%d", argIdx)
		args = append(args, string(*filter.UserID))
		argIdx++
	}

	if filter.Type != "" {
		query += fmt.Sprintf(" AND type = $%d", argIdx)
		args = append(args, string(filter.Type))
		argIdx++
	}

	if filter.Active != nil {
		if *filter.Active {
			query += " AND ended_at IS NULL"
		} else {
			query += " AND ended_at IS NOT NULL"
		}
	}

	if filter.ClientIP != "" {
		query += fmt.Sprintf(" AND client_ip = $%d", argIdx)
		args = append(args, filter.ClientIP)
		argIdx++
	}

	if filter.NodeID != "" {
		query += fmt.Sprintf(" AND node_id = $%d", argIdx)
		args = append(args, filter.NodeID)
		argIdx++
	}

	if filter.After != nil {
		query += fmt.Sprintf(" AND started_at >= $%d", argIdx)
		args = append(args, *filter.After)
		argIdx++
	}

	if filter.Before != nil {
		query += fmt.Sprintf(" AND started_at <= $%d", argIdx)
		args = append(args, *filter.Before)
		argIdx++
	}

	query += " ORDER BY started_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, filter.Limit)
		argIdx++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, filter.Offset)
	}

	rows, err := r.store.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*storage.Session
	for rows.Next() {
		session, err := scanSessionRows(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// Cleanup removes expired sessions older than the given time.
func (r *SessionRepository) Cleanup(ctx context.Context, before time.Time) (int64, error) {
	result, err := r.store.pool.Exec(ctx, `
		DELETE FROM sessions WHERE ended_at IS NOT NULL AND ended_at < $1
	`, before)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup sessions: %w", err)
	}

	return result.RowsAffected(), nil
}

// scanSessionRow scans a single session row.
func scanSessionRow(row pgx.Row) (*storage.Session, error) {
	var (
		session      storage.Session
		userID       string
		sessionType  string
		clientAgent  *string
		metadataJSON []byte
	)

	err := row.Scan(
		&session.ID,
		&userID,
		&sessionType,
		&session.ClientIP,
		&clientAgent,
		&session.PublicKeyFingerprint,
		&session.NodeID,
		&session.StartedAt,
		&session.EndedAt,
		&session.LastActivityAt,
		&metadataJSON,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrSessionNotFound
		}
		return nil, fmt.Errorf("failed to scan session: %w", err)
	}

	session.UserID = domain.UserID(userID)
	session.Type = storage.SessionType(sessionType)

	if clientAgent != nil {
		session.ClientAgent = *clientAgent
	}

	if len(metadataJSON) > 0 {
		_ = json.Unmarshal(metadataJSON, &session.Metadata)
	}

	return &session, nil
}

// scanSessionRows scans session rows.
func scanSessionRows(rows pgx.Rows) (*storage.Session, error) {
	var (
		session      storage.Session
		userID       string
		sessionType  string
		clientAgent  *string
		metadataJSON []byte
	)

	err := rows.Scan(
		&session.ID,
		&userID,
		&sessionType,
		&session.ClientIP,
		&clientAgent,
		&session.PublicKeyFingerprint,
		&session.NodeID,
		&session.StartedAt,
		&session.EndedAt,
		&session.LastActivityAt,
		&metadataJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan session: %w", err)
	}

	session.UserID = domain.UserID(userID)
	session.Type = storage.SessionType(sessionType)

	if clientAgent != nil {
		session.ClientAgent = *clientAgent
	}

	if len(metadataJSON) > 0 {
		_ = json.Unmarshal(metadataJSON, &session.Metadata)
	}

	return &session, nil
}
