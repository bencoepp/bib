package postgres

import (
	"context"
	"fmt"

	"bib/internal/domain"
	"bib/internal/storage"

	"github.com/jackc/pgx/v5"
)

// TopicRepository implements storage.TopicRepository for PostgreSQL.
type TopicRepository struct {
	store *Store
}

// Create creates a new topic.
func (r *TopicRepository) Create(ctx context.Context, topic *domain.Topic) error {
	if err := topic.Validate(); err != nil {
		return err
	}

	owners := make([]string, len(topic.Owners))
	for i, o := range topic.Owners {
		owners[i] = string(o)
	}

	_, err := r.store.execWithAudit(ctx, "INSERT", "topics", `
		INSERT INTO topics (id, parent_id, name, description, table_schema, status, owners, created_by, created_at, updated_at, dataset_count, tags, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`,
		string(topic.ID),
		nullableString(string(topic.ParentID)),
		topic.Name,
		topic.Description,
		topic.TableSchema,
		string(topic.Status),
		owners,
		string(topic.CreatedBy),
		topic.CreatedAt,
		topic.UpdatedAt,
		topic.DatasetCount,
		topic.Tags,
		topic.Metadata,
	)

	if err != nil {
		if isUniqueViolation(err) {
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
		FROM topics WHERE id = $1
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
		FROM topics WHERE name = $1
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
	argNum := 1

	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, string(filter.Status))
		argNum++
	}

	if filter.ParentID != nil {
		if *filter.ParentID == "" {
			query += " AND parent_id IS NULL"
		} else {
			query += fmt.Sprintf(" AND parent_id = $%d", argNum)
			args = append(args, string(*filter.ParentID))
			argNum++
		}
	}

	if filter.OwnerID != nil {
		query += fmt.Sprintf(" AND $%d = ANY(owners)", argNum)
		args = append(args, string(*filter.OwnerID))
		argNum++
	}

	if filter.Search != "" {
		query += fmt.Sprintf(" AND (name ILIKE $%d OR description ILIKE $%d)", argNum, argNum+1)
		search := "%" + filter.Search + "%"
		args = append(args, search, search)
		argNum += 2
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

	owners := make([]string, len(topic.Owners))
	for i, o := range topic.Owners {
		owners[i] = string(o)
	}

	rowsAffected, err := r.store.execWithAudit(ctx, "UPDATE", "topics", `
		UPDATE topics SET
			parent_id = $1,
			name = $2,
			description = $3,
			table_schema = $4,
			status = $5,
			owners = $6,
			dataset_count = $7,
			tags = $8,
			metadata = $9
		WHERE id = $10
	`,
		nullableString(string(topic.ParentID)),
		topic.Name,
		topic.Description,
		topic.TableSchema,
		string(topic.Status),
		owners,
		topic.DatasetCount,
		topic.Tags,
		topic.Metadata,
		string(topic.ID),
	)
	if err != nil {
		return fmt.Errorf("failed to update topic: %w", err)
	}

	if rowsAffected == 0 {
		return storage.ErrNotFound
	}

	return nil
}

// Delete soft-deletes a topic.
func (r *TopicRepository) Delete(ctx context.Context, id domain.TopicID) error {
	rowsAffected, err := r.store.execWithAudit(ctx, "UPDATE", "topics", `
		UPDATE topics SET status = 'deleted' WHERE id = $1
	`, string(id))
	if err != nil {
		return fmt.Errorf("failed to delete topic: %w", err)
	}

	if rowsAffected == 0 {
		return storage.ErrNotFound
	}

	return nil
}

// Count returns the number of topics matching the filter.
func (r *TopicRepository) Count(ctx context.Context, filter storage.TopicFilter) (int64, error) {
	query := "SELECT COUNT(*) FROM topics WHERE 1=1"
	args := []any{}
	argNum := 1

	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, string(filter.Status))
		argNum++
	}

	if filter.ParentID != nil {
		if *filter.ParentID == "" {
			query += " AND parent_id IS NULL"
		} else {
			query += fmt.Sprintf(" AND parent_id = $%d", argNum)
			args = append(args, string(*filter.ParentID))
			argNum++
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

func scanTopic(rows pgx.Rows) (*domain.Topic, error) {
	var (
		id           string
		parentID     *string
		name         string
		description  string
		tableSchema  string
		status       string
		owners       []string
		createdBy    string
		createdAt    interface{}
		updatedAt    interface{}
		datasetCount int
		tags         []string
		metadata     map[string]string
	)

	err := rows.Scan(
		&id, &parentID, &name, &description, &tableSchema, &status,
		&owners, &createdBy, &createdAt, &updatedAt, &datasetCount,
		&tags, &metadata,
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
		Tags:         tags,
		Metadata:     metadata,
	}

	if parentID != nil {
		topic.ParentID = domain.TopicID(*parentID)
	}

	// Parse timestamps
	topic.CreatedAt = parseTime(createdAt)
	topic.UpdatedAt = parseTime(updatedAt)

	// Parse owners
	for _, o := range owners {
		topic.Owners = append(topic.Owners, domain.UserID(o))
	}

	return topic, nil
}

// Ensure interface compliance
var _ storage.TopicRepository = (*TopicRepository)(nil)
