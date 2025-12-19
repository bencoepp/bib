// Package sqlite provides a SQLite implementation of the storage interfaces.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"bib/internal/domain"
	"bib/internal/storage"
)

// BannedPeerRepository implements storage.BannedPeerRepository for SQLite.
type BannedPeerRepository struct {
	store *Store
}

// Create creates a new ban.
func (r *BannedPeerRepository) Create(ctx context.Context, ban *storage.BannedPeer) error {
	query := `
		INSERT INTO banned_peers (
			peer_id, reason, banned_by, banned_at, expires_at, metadata
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(peer_id) DO UPDATE SET
			reason = excluded.reason,
			banned_by = excluded.banned_by,
			banned_at = excluded.banned_at,
			expires_at = excluded.expires_at,
			metadata = excluded.metadata
	`

	var expiresAt *string
	if ban.ExpiresAt != nil {
		s := ban.ExpiresAt.Format(time.RFC3339)
		expiresAt = &s
	}

	var metadataJSON []byte
	if ban.Metadata != nil {
		metadataJSON, _ = json.Marshal(ban.Metadata)
	}

	_, err := r.store.db.ExecContext(ctx, query,
		ban.PeerID,
		ban.Reason,
		string(ban.BannedBy),
		ban.BannedAt.Format(time.RFC3339),
		expiresAt,
		string(metadataJSON),
	)

	return err
}

// Get retrieves a ban by peer ID.
func (r *BannedPeerRepository) Get(ctx context.Context, peerID string) (*storage.BannedPeer, error) {
	query := `
		SELECT peer_id, reason, banned_by, banned_at, expires_at, metadata
		FROM banned_peers
		WHERE peer_id = ?
	`

	return r.scanBan(r.store.db.QueryRowContext(ctx, query, peerID))
}

// List lists all bans.
func (r *BannedPeerRepository) List(ctx context.Context, filter storage.BannedPeerFilter) ([]*storage.BannedPeer, error) {
	query := `
		SELECT peer_id, reason, banned_by, banned_at, expires_at, metadata
		FROM banned_peers
		WHERE 1=1
	`
	args := []interface{}{}

	if !filter.IncludeExpired {
		query += " AND (expires_at IS NULL OR expires_at > ?)"
		args = append(args, time.Now().UTC().Format(time.RFC3339))
	}

	query += " ORDER BY banned_at DESC"

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
		return nil, err
	}
	defer rows.Close()

	return r.scanBans(rows)
}

// Delete removes a ban.
func (r *BannedPeerRepository) Delete(ctx context.Context, peerID string) error {
	_, err := r.store.db.ExecContext(ctx,
		"DELETE FROM banned_peers WHERE peer_id = ?",
		peerID,
	)
	return err
}

// IsBanned checks if a peer is currently banned.
func (r *BannedPeerRepository) IsBanned(ctx context.Context, peerID string) (bool, error) {
	query := `
		SELECT COUNT(*) FROM banned_peers 
		WHERE peer_id = ? AND (expires_at IS NULL OR expires_at > ?)
	`

	var count int
	err := r.store.db.QueryRowContext(ctx, query, peerID, time.Now().UTC().Format(time.RFC3339)).Scan(&count)
	return count > 0, err
}

// CleanupExpired removes expired bans.
func (r *BannedPeerRepository) CleanupExpired(ctx context.Context) (int64, error) {
	result, err := r.store.db.ExecContext(ctx,
		"DELETE FROM banned_peers WHERE expires_at IS NOT NULL AND expires_at < ?",
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// scanBan scans a single ban from a row.
func (r *BannedPeerRepository) scanBan(row *sql.Row) (*storage.BannedPeer, error) {
	var ban storage.BannedPeer
	var bannedBy sql.NullString
	var bannedAt string
	var expiresAt sql.NullString
	var metadataJSON sql.NullString

	err := row.Scan(
		&ban.PeerID,
		&ban.Reason,
		&bannedBy,
		&bannedAt,
		&expiresAt,
		&metadataJSON,
	)

	if err == sql.ErrNoRows {
		return nil, domain.ErrUserNotFound // Reuse generic not found
	}
	if err != nil {
		return nil, err
	}

	if bannedBy.Valid {
		ban.BannedBy = domain.UserID(bannedBy.String)
	}

	ban.BannedAt, _ = time.Parse(time.RFC3339, bannedAt)

	if expiresAt.Valid {
		t, _ := time.Parse(time.RFC3339, expiresAt.String)
		ban.ExpiresAt = &t
	}

	if metadataJSON.Valid && metadataJSON.String != "" {
		json.Unmarshal([]byte(metadataJSON.String), &ban.Metadata)
	}

	return &ban, nil
}

// scanBans scans multiple bans from rows.
func (r *BannedPeerRepository) scanBans(rows *sql.Rows) ([]*storage.BannedPeer, error) {
	var bans []*storage.BannedPeer

	for rows.Next() {
		var ban storage.BannedPeer
		var bannedBy sql.NullString
		var bannedAt string
		var expiresAt sql.NullString
		var metadataJSON sql.NullString

		err := rows.Scan(
			&ban.PeerID,
			&ban.Reason,
			&bannedBy,
			&bannedAt,
			&expiresAt,
			&metadataJSON,
		)
		if err != nil {
			return nil, err
		}

		if bannedBy.Valid {
			ban.BannedBy = domain.UserID(bannedBy.String)
		}

		ban.BannedAt, _ = time.Parse(time.RFC3339, bannedAt)

		if expiresAt.Valid {
			t, _ := time.Parse(time.RFC3339, expiresAt.String)
			ban.ExpiresAt = &t
		}

		if metadataJSON.Valid && metadataJSON.String != "" {
			json.Unmarshal([]byte(metadataJSON.String), &ban.Metadata)
		}

		bans = append(bans, &ban)
	}

	return bans, rows.Err()
}
