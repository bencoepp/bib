package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"bib/internal/domain"
	"bib/internal/storage"
)

// TopicRepository implements storage.TopicRepository for SQLite.
type TopicRepository struct {
	store *Store
}

// Create creates a new topic.
func (r *TopicRepository) Create(ctx context.Context, topic *domain.Topic) error {
	if err := topic.Validate(); err != nil {
		return err
	}

	ownersJSON, err := json.Marshal(topic.Owners)
	if err != nil {
		return fmt.Errorf("failed to marshal owners: %w", err)
	}

	tagsJSON, err := json.Marshal(topic.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	metadataJSON, err := json.Marshal(topic.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)

	_, err = r.store.execWithAudit(ctx, "INSERT", "topics", `
		INSERT INTO topics (id, parent_id, name, description, table_schema, status, owners, created_by, created_at, updated_at, dataset_count, tags, metadata, cached_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		string(topic.ID),
		nullString(string(topic.ParentID)),
		topic.Name,
		topic.Description,
		topic.TableSchema,
		string(topic.Status),
		string(ownersJSON),
		string(topic.CreatedBy),
		topic.CreatedAt.UTC().Format(time.RFC3339Nano),
		topic.UpdatedAt.UTC().Format(time.RFC3339Nano),
		topic.DatasetCount,
		string(tagsJSON),
		string(metadataJSON),
		now,
	)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return storage.ErrAlreadyExists
		}
		return fmt.Errorf("failed to create topic: %w", err)
	}

	return nil
}

// Get retrieves a topic by ID.
func (r *TopicRepository) Get(ctx context.Context, id domain.TopicID) (*domain.Topic, error) {
	rows, err := r.store.queryWithAudit(ctx, "topics", `
		SELECT id, parent_id, name, description, table_schema, status, owners, created_by, created_at, updated_at, dataset_count, tags, metadata
		FROM topics WHERE id = ?
	`, string(id))
	if err != nil {
		return nil, fmt.Errorf("failed to query topic: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, storage.ErrNotFound
	}

	return scanTopic(rows)
}

// GetByName retrieves a topic by name.
func (r *TopicRepository) GetByName(ctx context.Context, name string) (*domain.Topic, error) {
	rows, err := r.store.queryWithAudit(ctx, "topics", `
		SELECT id, parent_id, name, description, table_schema, status, owners, created_by, created_at, updated_at, dataset_count, tags, metadata
		FROM topics WHERE name = ?
	`, name)
	if err != nil {
		return nil, fmt.Errorf("failed to query topic: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, storage.ErrNotFound
	}

	return scanTopic(rows)
}

// List retrieves topics matching the filter.
func (r *TopicRepository) List(ctx context.Context, filter storage.TopicFilter) ([]*domain.Topic, error) {
	query := `
		SELECT id, parent_id, name, description, table_schema, status, owners, created_by, created_at, updated_at, dataset_count, tags, metadata
		FROM topics WHERE 1=1
	`
	args := []any{}

	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, string(filter.Status))
	}

	if filter.ParentID != nil {
		if *filter.ParentID == "" {
			query += " AND parent_id IS NULL"
		} else {
			query += " AND parent_id = ?"
			args = append(args, string(*filter.ParentID))
		}
	}

	if filter.OwnerID != nil {
		query += " AND owners LIKE ?"
		args = append(args, "%"+string(*filter.OwnerID)+"%")
	}

	if filter.Search != "" {
		query += " AND (name LIKE ? OR description LIKE ?)"
		search := "%" + filter.Search + "%"
		args = append(args, search, search)
	}

	// Order by
	orderBy := "name"
	if filter.OrderBy != "" {
		orderBy = filter.OrderBy
	}
	order := "ASC"
	if filter.OrderDesc {
		order = "DESC"
	}
	query += fmt.Sprintf(" ORDER BY %s %s", orderBy, order)

	// Pagination
	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := r.store.queryWithAudit(ctx, "topics", query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query topics: %w", err)
	}
	defer rows.Close()

	var topics []*domain.Topic
	for rows.Next() {
		topic, err := scanTopic(rows)
		if err != nil {
			return nil, err
		}
		topics = append(topics, topic)
	}

	return topics, rows.Err()
}

// Update updates an existing topic.
func (r *TopicRepository) Update(ctx context.Context, topic *domain.Topic) error {
	if err := topic.Validate(); err != nil {
		return err
	}

	ownersJSON, _ := json.Marshal(topic.Owners)
	tagsJSON, _ := json.Marshal(topic.Tags)
	metadataJSON, _ := json.Marshal(topic.Metadata)
	now := time.Now().UTC().Format(time.RFC3339Nano)

	result, err := r.store.execWithAudit(ctx, "UPDATE", "topics", `
		UPDATE topics SET
			parent_id = ?,
			name = ?,
			description = ?,
			table_schema = ?,
			status = ?,
			owners = ?,
			updated_at = ?,
			dataset_count = ?,
			tags = ?,
			metadata = ?,
			cached_at = ?
		WHERE id = ?
	`,
		nullString(string(topic.ParentID)),
		topic.Name,
		topic.Description,
		topic.TableSchema,
		string(topic.Status),
		string(ownersJSON),
		now,
		topic.DatasetCount,
		string(tagsJSON),
		string(metadataJSON),
		now,
		string(topic.ID),
	)
	if err != nil {
		return fmt.Errorf("failed to update topic: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return storage.ErrNotFound
	}

	return nil
}

// Delete soft-deletes a topic.
func (r *TopicRepository) Delete(ctx context.Context, id domain.TopicID) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)

	result, err := r.store.execWithAudit(ctx, "UPDATE", "topics", `
		UPDATE topics SET status = 'deleted', updated_at = ? WHERE id = ?
	`, now, string(id))
	if err != nil {
		return fmt.Errorf("failed to delete topic: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return storage.ErrNotFound
	}

	return nil
}

// Count returns the number of topics matching the filter.
func (r *TopicRepository) Count(ctx context.Context, filter storage.TopicFilter) (int64, error) {
	query := "SELECT COUNT(*) FROM topics WHERE 1=1"
	args := []any{}

	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, string(filter.Status))
	}

	if filter.ParentID != nil {
		if *filter.ParentID == "" {
			query += " AND parent_id IS NULL"
		} else {
			query += " AND parent_id = ?"
			args = append(args, string(*filter.ParentID))
		}
	}

	rows, err := r.store.queryWithAudit(ctx, "topics", query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to count topics: %w", err)
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

func scanTopic(rows *sql.Rows) (*domain.Topic, error) {
	var (
		id           string
		parentID     sql.NullString
		name         string
		description  string
		tableSchema  string
		status       string
		ownersJSON   string
		createdBy    string
		createdAt    string
		updatedAt    string
		datasetCount int
		tagsJSON     sql.NullString
		metadataJSON sql.NullString
	)

	err := rows.Scan(
		&id, &parentID, &name, &description, &tableSchema, &status,
		&ownersJSON, &createdBy, &createdAt, &updatedAt, &datasetCount,
		&tagsJSON, &metadataJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan topic: %w", err)
	}

	topic := &domain.Topic{
		ID:           domain.TopicID(id),
		Name:         name,
		Description:  description,
		TableSchema:  tableSchema,
		Status:       domain.TopicStatus(status),
		CreatedBy:    domain.UserID(createdBy),
		DatasetCount: datasetCount,
	}

	if parentID.Valid {
		topic.ParentID = domain.TopicID(parentID.String)
	}

	// Parse timestamps
	if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
		topic.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339Nano, updatedAt); err == nil {
		topic.UpdatedAt = t
	}

	// Parse JSON fields
	if ownersJSON != "" {
		var owners []domain.UserID
		if err := json.Unmarshal([]byte(ownersJSON), &owners); err == nil {
			topic.Owners = owners
		}
	}

	if tagsJSON.Valid && tagsJSON.String != "" {
		var tags []string
		if err := json.Unmarshal([]byte(tagsJSON.String), &tags); err == nil {
			topic.Tags = tags
		}
	}

	if metadataJSON.Valid && metadataJSON.String != "" {
		var metadata map[string]string
		if err := json.Unmarshal([]byte(metadataJSON.String), &metadata); err == nil {
			topic.Metadata = metadata
		}
	}

	return topic, nil
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// Ensure interface compliance
var _ storage.TopicRepository = (*TopicRepository)(nil)

// errNotAuthoritative is a helper for write operations in cache mode
func errNotAuthoritative(op string) error {
	return fmt.Errorf("%s: %w", op, storage.ErrNotAuthoritative)
}

// isConstraintError checks if the error is a constraint violation
func isConstraintError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint") ||
		strings.Contains(err.Error(), "FOREIGN KEY constraint")
}

// wrapNotFound wraps sql.ErrNoRows as storage.ErrNotFound
func wrapNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return storage.ErrNotFound
	}
	return err
}
