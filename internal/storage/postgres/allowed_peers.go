// Package postgres provides a PostgreSQL implementation of the storage interfaces.
package postgres

import (
	"context"
	"encoding/json"
	"time"

	"bib/internal/storage"

	"github.com/jackc/pgx/v5"
)

// AllowedPeerRepository implements storage.AllowedPeerRepository for PostgreSQL.
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
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT(peer_id) DO UPDATE SET
			name = EXCLUDED.name,
			added_at = EXCLUDED.added_at,
			added_by = EXCLUDED.added_by,
			expires_at = EXCLUDED.expires_at,
			metadata = EXCLUDED.metadata
	`

	addedAt := peer.AddedAt
	if addedAt.IsZero() {
		addedAt = time.Now().UTC()
	}

	_, err := r.store.pool.Exec(ctx, query,
		peer.PeerID,
		nullString(peer.Name),
		addedAt,
		peer.AddedBy,
		peer.ExpiresAt,
		metadataJSON,
	)

	return err
}

// Remove removes a peer from the allowed list.
func (r *AllowedPeerRepository) Remove(ctx context.Context, peerID string) error {
	_, err := r.store.pool.Exec(ctx,
		"DELETE FROM allowed_peers WHERE peer_id = $1",
		peerID,
	)
	return err
}

// Get retrieves an allowed peer by ID.
func (r *AllowedPeerRepository) Get(ctx context.Context, peerID string) (*storage.AllowedPeer, error) {
	query := `
		SELECT peer_id, name, added_at, added_by, expires_at, metadata
		FROM allowed_peers
		WHERE peer_id = $1
	`

	return r.scanPeer(r.store.pool.QueryRow(ctx, query, peerID))
}

// List lists all allowed peers.
func (r *AllowedPeerRepository) List(ctx context.Context) ([]*storage.AllowedPeer, error) {
	query := `
		SELECT peer_id, name, added_at, added_by, expires_at, metadata
		FROM allowed_peers
		ORDER BY added_at DESC
	`

	rows, err := r.store.pool.Query(ctx, query)
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
		WHERE peer_id = $1 AND (expires_at IS NULL OR expires_at > $2)
	`

	var count int
	err := r.store.pool.QueryRow(ctx, query, peerID, time.Now().UTC()).Scan(&count)
	return count > 0, err
}

// Cleanup removes expired entries.
func (r *AllowedPeerRepository) Cleanup(ctx context.Context) error {
	_, err := r.store.pool.Exec(ctx,
		"DELETE FROM allowed_peers WHERE expires_at IS NOT NULL AND expires_at < $1",
		time.Now().UTC(),
	)
	return err
}

// Count returns the number of allowed peers.
func (r *AllowedPeerRepository) Count(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(*) FROM allowed_peers WHERE expires_at IS NULL OR expires_at > $1`

	var count int64
	err := r.store.pool.QueryRow(ctx, query, time.Now().UTC()).Scan(&count)
	return count, err
}

// scanPeer scans a single peer from a row.
func (r *AllowedPeerRepository) scanPeer(row pgx.Row) (*storage.AllowedPeer, error) {
	var peer storage.AllowedPeer
	var name *string
	var metadataJSON []byte

	err := row.Scan(
		&peer.PeerID,
		&name,
		&peer.AddedAt,
		&peer.AddedBy,
		&peer.ExpiresAt,
		&metadataJSON,
	)

	if err == pgx.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if name != nil {
		peer.Name = *name
	}

	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &peer.Metadata)
	}

	return &peer, nil
}

// scanPeers scans multiple peers from rows.
func (r *AllowedPeerRepository) scanPeers(rows pgx.Rows) ([]*storage.AllowedPeer, error) {
	var peers []*storage.AllowedPeer

	for rows.Next() {
		var peer storage.AllowedPeer
		var name *string
		var metadataJSON []byte

		err := rows.Scan(
			&peer.PeerID,
			&name,
			&peer.AddedAt,
			&peer.AddedBy,
			&peer.ExpiresAt,
			&metadataJSON,
		)
		if err != nil {
			return nil, err
		}

		if name != nil {
			peer.Name = *name
		}

		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &peer.Metadata)
		}

		peers = append(peers, &peer)
	}

	return peers, rows.Err()
}
