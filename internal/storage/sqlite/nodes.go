package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"bib/internal/storage"
)

// NodeRepository implements storage.NodeRepository for SQLite.
type NodeRepository struct {
	store *Store
}

// Upsert creates or updates a node.
func (r *NodeRepository) Upsert(ctx context.Context, node *storage.NodeInfo) error {
	addressesJSON, _ := json.Marshal(node.Addresses)
	metadataJSON, _ := json.Marshal(node.Metadata)
	now := time.Now().UTC().Format(time.RFC3339Nano)

	if node.CreatedAt.IsZero() {
		node.CreatedAt = time.Now().UTC()
	}
	node.UpdatedAt = time.Now().UTC()

	_, err := r.store.execWithAudit(ctx, "INSERT", "nodes", `
		INSERT INTO nodes (peer_id, addresses, mode, storage_type, trusted_storage, last_seen, metadata, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(peer_id) DO UPDATE SET
			addresses = excluded.addresses,
			mode = excluded.mode,
			storage_type = excluded.storage_type,
			trusted_storage = excluded.trusted_storage,
			last_seen = excluded.last_seen,
			metadata = excluded.metadata,
			updated_at = excluded.updated_at
	`,
		node.PeerID,
		string(addressesJSON),
		node.Mode,
		node.StorageType,
		boolToInt(node.TrustedStorage),
		node.LastSeen.UTC().Format(time.RFC3339Nano),
		string(metadataJSON),
		node.CreatedAt.UTC().Format(time.RFC3339Nano),
		now,
	)

	if err != nil {
		return fmt.Errorf("failed to upsert node: %w", err)
	}

	return nil
}

// Get retrieves a node by peer ID.
func (r *NodeRepository) Get(ctx context.Context, peerID string) (*storage.NodeInfo, error) {
	rows, err := r.store.queryWithAudit(ctx, "nodes", `
		SELECT peer_id, addresses, mode, storage_type, trusted_storage, last_seen, metadata, created_at, updated_at
		FROM nodes WHERE peer_id = ?
	`, peerID)
	if err != nil {
		return nil, fmt.Errorf("failed to query node: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, storage.ErrNotFound
	}

	return scanNode(rows)
}

// List retrieves nodes matching the filter.
func (r *NodeRepository) List(ctx context.Context, filter storage.NodeFilter) ([]*storage.NodeInfo, error) {
	query := `
		SELECT peer_id, addresses, mode, storage_type, trusted_storage, last_seen, metadata, created_at, updated_at
		FROM nodes WHERE 1=1
	`
	args := []any{}

	if filter.Mode != "" {
		query += " AND mode = ?"
		args = append(args, filter.Mode)
	}

	if filter.TrustedOnly {
		query += " AND trusted_storage = 1"
	}

	if filter.SeenAfter != nil {
		query += " AND last_seen >= ?"
		args = append(args, filter.SeenAfter.UTC().Format(time.RFC3339Nano))
	}

	query += " ORDER BY last_seen DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := r.store.queryWithAudit(ctx, "nodes", query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query nodes: %w", err)
	}
	defer rows.Close()

	var nodes []*storage.NodeInfo
	for rows.Next() {
		node, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}

	return nodes, rows.Err()
}

// Delete removes a node.
func (r *NodeRepository) Delete(ctx context.Context, peerID string) error {
	result, err := r.store.execWithAudit(ctx, "DELETE", "nodes", `
		DELETE FROM nodes WHERE peer_id = ?
	`, peerID)
	if err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return storage.ErrNotFound
	}

	return nil
}

// UpdateLastSeen updates the last seen timestamp.
func (r *NodeRepository) UpdateLastSeen(ctx context.Context, peerID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)

	result, err := r.store.execWithAudit(ctx, "UPDATE", "nodes", `
		UPDATE nodes SET last_seen = ?, updated_at = ? WHERE peer_id = ?
	`, now, now, peerID)
	if err != nil {
		return fmt.Errorf("failed to update last seen: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return storage.ErrNotFound
	}

	return nil
}

// Count returns the number of nodes matching the filter.
func (r *NodeRepository) Count(ctx context.Context, filter storage.NodeFilter) (int64, error) {
	query := "SELECT COUNT(*) FROM nodes WHERE 1=1"
	args := []any{}

	if filter.Mode != "" {
		query += " AND mode = ?"
		args = append(args, filter.Mode)
	}

	if filter.TrustedOnly {
		query += " AND trusted_storage = 1"
	}

	rows, err := r.store.queryWithAudit(ctx, "nodes", query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to count nodes: %w", err)
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

// Helper functions

func scanNode(rows *sql.Rows) (*storage.NodeInfo, error) {
	var (
		peerID         string
		addressesJSON  string
		mode           string
		storageType    string
		trustedStorage int
		lastSeen       string
		metadataJSON   sql.NullString
		createdAt      string
		updatedAt      string
	)

	err := rows.Scan(
		&peerID, &addressesJSON, &mode, &storageType,
		&trustedStorage, &lastSeen, &metadataJSON, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan node: %w", err)
	}

	node := &storage.NodeInfo{
		PeerID:         peerID,
		Mode:           mode,
		StorageType:    storageType,
		TrustedStorage: trustedStorage == 1,
	}

	if addressesJSON != "" {
		var addresses []string
		if err := json.Unmarshal([]byte(addressesJSON), &addresses); err == nil {
			node.Addresses = addresses
		}
	}

	if t, err := time.Parse(time.RFC3339Nano, lastSeen); err == nil {
		node.LastSeen = t
	}

	if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
		node.CreatedAt = t
	}

	if t, err := time.Parse(time.RFC3339Nano, updatedAt); err == nil {
		node.UpdatedAt = t
	}

	if metadataJSON.Valid && metadataJSON.String != "" {
		var metadata map[string]any
		if err := json.Unmarshal([]byte(metadataJSON.String), &metadata); err == nil {
			node.Metadata = metadata
		}
	}

	return node, nil
}

// Ensure interface compliance
var _ storage.NodeRepository = (*NodeRepository)(nil)
