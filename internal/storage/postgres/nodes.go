package postgres

import (
	"context"
	"fmt"
	"time"

	"bib/internal/storage"

	"github.com/jackc/pgx/v5"
)

// NodeRepository implements storage.NodeRepository for PostgreSQL.
type NodeRepository struct {
	store *Store
}

// Upsert creates or updates a node.
func (r *NodeRepository) Upsert(ctx context.Context, node *storage.NodeInfo) error {
	if node.CreatedAt.IsZero() {
		node.CreatedAt = time.Now().UTC()
	}
	node.UpdatedAt = time.Now().UTC()

	_, err := r.store.execWithAudit(ctx, "INSERT", "nodes", `
		INSERT INTO nodes (peer_id, addresses, mode, storage_type, trusted_storage, last_seen, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (peer_id) DO UPDATE SET
			addresses = EXCLUDED.addresses,
			mode = EXCLUDED.mode,
			storage_type = EXCLUDED.storage_type,
			trusted_storage = EXCLUDED.trusted_storage,
			last_seen = EXCLUDED.last_seen,
			metadata = EXCLUDED.metadata,
			updated_at = EXCLUDED.updated_at
	`,
		node.PeerID,
		node.Addresses,
		node.Mode,
		node.StorageType,
		node.TrustedStorage,
		node.LastSeen,
		node.Metadata,
		node.CreatedAt,
		node.UpdatedAt,
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
		FROM nodes WHERE peer_id = $1
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
	argNum := 1

	if filter.Mode != "" {
		query += fmt.Sprintf(" AND mode = $%d", argNum)
		args = append(args, filter.Mode)
		argNum++
	}

	if filter.TrustedOnly {
		query += " AND trusted_storage = true"
	}

	if filter.SeenAfter != nil {
		query += fmt.Sprintf(" AND last_seen >= $%d", argNum)
		args = append(args, *filter.SeenAfter)
		argNum++
	}

	query += " ORDER BY last_seen DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argNum)
		args = append(args, filter.Limit)
		argNum++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argNum)
		args = append(args, filter.Offset)
		argNum++
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
	rowsAffected, err := r.store.execWithAudit(ctx, "DELETE", "nodes", `
		DELETE FROM nodes WHERE peer_id = $1
	`, peerID)
	if err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}

	if rowsAffected == 0 {
		return storage.ErrNotFound
	}

	return nil
}

// UpdateLastSeen updates the last seen timestamp.
func (r *NodeRepository) UpdateLastSeen(ctx context.Context, peerID string) error {
	now := time.Now().UTC()

	rowsAffected, err := r.store.execWithAudit(ctx, "UPDATE", "nodes", `
		UPDATE nodes SET last_seen = $1 WHERE peer_id = $2
	`, now, peerID)
	if err != nil {
		return fmt.Errorf("failed to update last seen: %w", err)
	}

	if rowsAffected == 0 {
		return storage.ErrNotFound
	}

	return nil
}

// Count returns the number of nodes matching the filter.
func (r *NodeRepository) Count(ctx context.Context, filter storage.NodeFilter) (int64, error) {
	query := "SELECT COUNT(*) FROM nodes WHERE 1=1"
	args := []any{}
	argNum := 1

	if filter.Mode != "" {
		query += fmt.Sprintf(" AND mode = $%d", argNum)
		args = append(args, filter.Mode)
		argNum++
	}

	if filter.TrustedOnly {
		query += " AND trusted_storage = true"
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

func scanNode(rows pgx.Rows) (*storage.NodeInfo, error) {
	var (
		peerID         string
		addresses      []string
		mode           string
		storageType    string
		trustedStorage bool
		lastSeen       interface{}
		metadata       map[string]any
		createdAt      interface{}
		updatedAt      interface{}
	)

	err := rows.Scan(
		&peerID, &addresses, &mode, &storageType,
		&trustedStorage, &lastSeen, &metadata, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan node: %w", err)
	}

	node := &storage.NodeInfo{
		PeerID:         peerID,
		Addresses:      addresses,
		Mode:           mode,
		StorageType:    storageType,
		TrustedStorage: trustedStorage,
		Metadata:       metadata,
	}

	node.LastSeen = parseTime(lastSeen)
	node.CreatedAt = parseTime(createdAt)
	node.UpdatedAt = parseTime(updatedAt)

	return node, nil
}

// Ensure interface compliance
var _ storage.NodeRepository = (*NodeRepository)(nil)
