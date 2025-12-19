// Package postgres provides a PostgreSQL implementation of the storage interfaces.
package postgres

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"bib/internal/domain"
	"bib/internal/storage"

	"github.com/jackc/pgx/v5"
)

// BannedPeerRepository implements storage.BannedPeerRepository for PostgreSQL.
type BannedPeerRepository struct {
	store *Store
}

// Create creates a new ban.
func (r *BannedPeerRepository) Create(ctx context.Context, ban *storage.BannedPeer) error {
	var metadataJSON []byte
	if ban.Metadata != nil {
		metadataJSON, _ = json.Marshal(ban.Metadata)
	}

	query := `
		INSERT INTO banned_peers (
			peer_id, reason, banned_by, banned_at, expires_at, metadata
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT(peer_id) DO UPDATE SET
			reason = EXCLUDED.reason,
			banned_by = EXCLUDED.banned_by,
			banned_at = EXCLUDED.banned_at,
			expires_at = EXCLUDED.expires_at,
			metadata = EXCLUDED.metadata
	`

	var bannedBy *string
	if ban.BannedBy != "" {
		s := string(ban.BannedBy)
		bannedBy = &s
	}

	_, err := r.store.pool.Exec(ctx, query,
		ban.PeerID,
		ban.Reason,
		bannedBy,
		ban.BannedAt,
		ban.ExpiresAt,
		metadataJSON,
	)

	return err
}

// Get retrieves a ban by peer ID.
func (r *BannedPeerRepository) Get(ctx context.Context, peerID string) (*storage.BannedPeer, error) {
	query := `
		SELECT peer_id, reason, banned_by, banned_at, expires_at, metadata
		FROM banned_peers
		WHERE peer_id = $1
	`

	return r.scanBan(r.store.pool.QueryRow(ctx, query, peerID))
}

// List lists all bans.
func (r *BannedPeerRepository) List(ctx context.Context, filter storage.BannedPeerFilter) ([]*storage.BannedPeer, error) {
	query := `
		SELECT peer_id, reason, banned_by, banned_at, expires_at, metadata
		FROM banned_peers
		WHERE 1=1
	`
	args := []interface{}{}
	argNum := 1

	if !filter.IncludeExpired {
		query += " AND (expires_at IS NULL OR expires_at > $" + strconv.Itoa(argNum) + ")"
		args = append(args, time.Now().UTC())
		argNum++
	}

	query += " ORDER BY banned_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT $" + strconv.Itoa(argNum)
		args = append(args, filter.Limit)
		argNum++
	}
	if filter.Offset > 0 {
		query += " OFFSET $" + strconv.Itoa(argNum)
		args = append(args, filter.Offset)
	}

	rows, err := r.store.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanBans(rows)
}

// Delete removes a ban.
func (r *BannedPeerRepository) Delete(ctx context.Context, peerID string) error {
	_, err := r.store.pool.Exec(ctx,
		"DELETE FROM banned_peers WHERE peer_id = $1",
		peerID,
	)
	return err
}

// IsBanned checks if a peer is currently banned.
func (r *BannedPeerRepository) IsBanned(ctx context.Context, peerID string) (bool, error) {
	query := `
		SELECT COUNT(*) FROM banned_peers 
		WHERE peer_id = $1 AND (expires_at IS NULL OR expires_at > $2)
	`

	var count int
	err := r.store.pool.QueryRow(ctx, query, peerID, time.Now().UTC()).Scan(&count)
	return count > 0, err
}

// CleanupExpired removes expired bans.
func (r *BannedPeerRepository) CleanupExpired(ctx context.Context) (int64, error) {
	result, err := r.store.pool.Exec(ctx,
		"DELETE FROM banned_peers WHERE expires_at IS NOT NULL AND expires_at < $1",
		time.Now().UTC(),
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// scanBan scans a single ban from a row.
func (r *BannedPeerRepository) scanBan(row pgx.Row) (*storage.BannedPeer, error) {
	var ban storage.BannedPeer
	var bannedBy *string
	var metadataJSON []byte

	err := row.Scan(
		&ban.PeerID,
		&ban.Reason,
		&bannedBy,
		&ban.BannedAt,
		&ban.ExpiresAt,
		&metadataJSON,
	)

	if err == pgx.ErrNoRows {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	if bannedBy != nil {
		ban.BannedBy = domain.UserID(*bannedBy)
	}

	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &ban.Metadata)
	}

	return &ban, nil
}

// scanBans scans multiple bans from rows.
func (r *BannedPeerRepository) scanBans(rows pgx.Rows) ([]*storage.BannedPeer, error) {
	var bans []*storage.BannedPeer

	for rows.Next() {
		var ban storage.BannedPeer
		var bannedBy *string
		var metadataJSON []byte

		err := rows.Scan(
			&ban.PeerID,
			&ban.Reason,
			&bannedBy,
			&ban.BannedAt,
			&ban.ExpiresAt,
			&metadataJSON,
		)
		if err != nil {
			return nil, err
		}

		if bannedBy != nil {
			ban.BannedBy = domain.UserID(*bannedBy)
		}

		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &ban.Metadata)
		}

		bans = append(bans, &ban)
	}

	return bans, rows.Err()
}
