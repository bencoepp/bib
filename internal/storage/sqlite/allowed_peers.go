// Package sqlite provides a SQLite implementation of the storage interfaces.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"bib/internal/storage"
)

// AllowedPeerRepository implements storage.AllowedPeerRepository for SQLite.
type AllowedPeerRepository struct {
	store *Store
}

// Add adds a peer to the allowed list.
func (r *AllowedPeerRepository) Add(ctx context.Context, peer *storage.AllowedPeer) error {
	var metadataJSON []byte
	if peer.Metadata != nil {
		metadataJSON, _ = json.Marshal(peer.Metadata)
	}

	query := `
		INSERT INTO allowed_peers (
			peer_id, name, added_at, added_by, expires_at, metadata
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(peer_id) DO UPDATE SET
			name = excluded.name,
			added_at = excluded.added_at,
			added_by = excluded.added_by,
			expires_at = excluded.expires_at,
			metadata = excluded.metadata
	`

	addedAt := peer.AddedAt
	if addedAt.IsZero() {
		addedAt = time.Now().UTC()
	}

	var expiresAt *string
	if peer.ExpiresAt != nil {
		t := peer.ExpiresAt.UTC().Format(time.RFC3339)
		expiresAt = &t
	}

	_, err := r.store.db.ExecContext(ctx, query,
		peer.PeerID,
		nullString(peer.Name),
		addedAt.UTC().Format(time.RFC3339),
		peer.AddedBy,
		expiresAt,
		string(metadataJSON),
	)

	return err
}

// Remove removes a peer from the allowed list.
func (r *AllowedPeerRepository) Remove(ctx context.Context, peerID string) error {
	_, err := r.store.db.ExecContext(ctx,
		"DELETE FROM allowed_peers WHERE peer_id = ?",
		peerID,
	)
	return err
}

// Get retrieves an allowed peer by ID.
func (r *AllowedPeerRepository) Get(ctx context.Context, peerID string) (*storage.AllowedPeer, error) {
	query := `
		SELECT peer_id, name, added_at, added_by, expires_at, metadata
		FROM allowed_peers
		WHERE peer_id = ?
	`

	return r.scanPeer(r.store.db.QueryRowContext(ctx, query, peerID))
}

// List lists all allowed peers.
func (r *AllowedPeerRepository) List(ctx context.Context) ([]*storage.AllowedPeer, error) {
	query := `
		SELECT peer_id, name, added_at, added_by, expires_at, metadata
		FROM allowed_peers
		ORDER BY added_at DESC
	`

	rows, err := r.store.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanPeers(rows)
}

// IsAllowed checks if a peer is in the allowed list and not expired.
func (r *AllowedPeerRepository) IsAllowed(ctx context.Context, peerID string) (bool, error) {
	query := `
		SELECT COUNT(*) FROM allowed_peers 
		WHERE peer_id = ? AND (expires_at IS NULL OR expires_at > ?)
	`

	var count int
	err := r.store.db.QueryRowContext(ctx, query, peerID, time.Now().UTC().Format(time.RFC3339)).Scan(&count)
	return count > 0, err
}

// Cleanup removes expired entries.
func (r *AllowedPeerRepository) Cleanup(ctx context.Context) error {
	_, err := r.store.db.ExecContext(ctx,
		"DELETE FROM allowed_peers WHERE expires_at IS NOT NULL AND expires_at < ?",
		time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// Count returns the number of allowed peers.
func (r *AllowedPeerRepository) Count(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(*) FROM allowed_peers WHERE expires_at IS NULL OR expires_at > ?`

	var count int64
	err := r.store.db.QueryRowContext(ctx, query, time.Now().UTC().Format(time.RFC3339)).Scan(&count)
	return count, err
}

// scanPeer scans a single peer from a row.
func (r *AllowedPeerRepository) scanPeer(row *sql.Row) (*storage.AllowedPeer, error) {
	var peer storage.AllowedPeer
	var name sql.NullString
	var addedAtStr string
	var expiresAtStr sql.NullString
	var metadataJSON sql.NullString

	err := row.Scan(
		&peer.PeerID,
		&name,
		&addedAtStr,
		&peer.AddedBy,
		&expiresAtStr,
		&metadataJSON,
	)

	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if name.Valid {
		peer.Name = name.String
	}

	peer.AddedAt, _ = time.Parse(time.RFC3339, addedAtStr)

	if expiresAtStr.Valid {
		t, _ := time.Parse(time.RFC3339, expiresAtStr.String)
		peer.ExpiresAt = &t
	}

	if metadataJSON.Valid && metadataJSON.String != "" {
		json.Unmarshal([]byte(metadataJSON.String), &peer.Metadata)
	}

	return &peer, nil
}

// scanPeers scans multiple peers from rows.
func (r *AllowedPeerRepository) scanPeers(rows *sql.Rows) ([]*storage.AllowedPeer, error) {
	var peers []*storage.AllowedPeer

	for rows.Next() {
		var peer storage.AllowedPeer
		var name sql.NullString
		var addedAtStr string
		var expiresAtStr sql.NullString
		var metadataJSON sql.NullString

		err := rows.Scan(
			&peer.PeerID,
			&name,
			&addedAtStr,
			&peer.AddedBy,
			&expiresAtStr,
			&metadataJSON,
		)
		if err != nil {
			return nil, err
		}

		if name.Valid {
			peer.Name = name.String
		}

		peer.AddedAt, _ = time.Parse(time.RFC3339, addedAtStr)

		if expiresAtStr.Valid {
			t, _ := time.Parse(time.RFC3339, expiresAtStr.String)
			peer.ExpiresAt = &t
		}

		if metadataJSON.Valid && metadataJSON.String != "" {
			json.Unmarshal([]byte(metadataJSON.String), &peer.Metadata)
		}

		peers = append(peers, &peer)
	}

	return peers, rows.Err()
}
