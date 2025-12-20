package postgres

import (
	"context"
	"fmt"
	"time"

	"bib/internal/domain"
	"bib/internal/storage"

	"github.com/jackc/pgx/v5"
)

// DatasetRepository implements storage.DatasetRepository for PostgreSQL.
type DatasetRepository struct {
	store *Store
}

// Create creates a new dataset.
func (r *DatasetRepository) Create(ctx context.Context, dataset *domain.Dataset) error {
	if err := dataset.Validate(); err != nil {
		return err
	}

	owners := make([]string, len(dataset.Owners))
	for i, o := range dataset.Owners {
		owners[i] = string(o)
	}

	_, err := r.store.execWithAudit(ctx, "INSERT", "datasets", `
		INSERT INTO datasets (id, topic_id, name, description, status, latest_version_id, version_count, has_content, has_instructions, owners, created_by, created_at, updated_at, tags, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`,
		string(dataset.ID),
		string(dataset.TopicID),
		dataset.Name,
		dataset.Description,
		string(dataset.Status),
		nullableString(string(dataset.LatestVersionID)),
		dataset.VersionCount,
		dataset.HasContent,
		dataset.HasInstructions,
		owners,
		string(dataset.CreatedBy),
		dataset.CreatedAt,
		dataset.UpdatedAt,
		dataset.Tags,
		dataset.Metadata,
	)

	if err != nil {
		if isUniqueViolation(err) {
			return storage.ErrAlreadyExists
		}
		return fmt.Errorf("failed to create dataset: %w", err)
	}

	return nil
}

// Get retrieves a dataset by ID.
func (r *DatasetRepository) Get(ctx context.Context, id domain.DatasetID) (*domain.Dataset, error) {
	rows, err := r.store.queryWithAudit(ctx, "datasets", `
		SELECT id, topic_id, name, description, status, latest_version_id, version_count, has_content, has_instructions, owners, created_by, created_at, updated_at, tags, metadata
		FROM datasets WHERE id = $1
	`, string(id))
	if err != nil {
		return nil, fmt.Errorf("failed to query dataset: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, storage.ErrNotFound
	}

	return scanDataset(rows)
}

// List retrieves datasets matching the filter.
func (r *DatasetRepository) List(ctx context.Context, filter storage.DatasetFilter) ([]*domain.Dataset, error) {
	query := `
		SELECT id, topic_id, name, description, status, latest_version_id, version_count, has_content, has_instructions, owners, created_by, created_at, updated_at, tags, metadata
		FROM datasets WHERE 1=1
	`
	args := []any{}
	argNum := 1

	if filter.TopicID != nil {
		query += fmt.Sprintf(" AND topic_id = $%d", argNum)
		args = append(args, string(*filter.TopicID))
		argNum++
	}

	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, string(filter.Status))
		argNum++
	}

	if filter.OwnerID != nil {
		query += fmt.Sprintf(" AND $%d = ANY(owners)", argNum)
		args = append(args, string(*filter.OwnerID))
		argNum++
	}

	if filter.HasContent != nil {
		query += fmt.Sprintf(" AND has_content = $%d", argNum)
		args = append(args, *filter.HasContent)
		argNum++
	}

	if filter.HasInstructions != nil {
		query += fmt.Sprintf(" AND has_instructions = $%d", argNum)
		args = append(args, *filter.HasInstructions)
		argNum++
	}

	if filter.Search != "" {
		query += fmt.Sprintf(" AND (name ILIKE $%d OR description ILIKE $%d)", argNum, argNum+1)
		search := "%" + filter.Search + "%"
		args = append(args, search, search)
		argNum += 2
	}

	if len(filter.Tags) > 0 {
		query += fmt.Sprintf(" AND tags @> $%d", argNum)
		args = append(args, filter.Tags)
		argNum++
	}

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

	rows, err := r.store.queryWithAudit(ctx, "datasets", query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query datasets: %w", err)
	}
	defer rows.Close()

	var datasets []*domain.Dataset
	for rows.Next() {
		dataset, err := scanDataset(rows)
		if err != nil {
			return nil, err
		}
		datasets = append(datasets, dataset)
	}

	return datasets, rows.Err()
}

// Update updates an existing dataset.
func (r *DatasetRepository) Update(ctx context.Context, dataset *domain.Dataset) error {
	if err := dataset.Validate(); err != nil {
		return err
	}

	owners := make([]string, len(dataset.Owners))
	for i, o := range dataset.Owners {
		owners[i] = string(o)
	}

	rowsAffected, err := r.store.execWithAudit(ctx, "UPDATE", "datasets", `
		UPDATE datasets SET
			topic_id = $1,
			name = $2,
			description = $3,
			status = $4,
			latest_version_id = $5,
			version_count = $6,
			has_content = $7,
			has_instructions = $8,
			owners = $9,
			tags = $10,
			metadata = $11
		WHERE id = $12
	`,
		string(dataset.TopicID),
		dataset.Name,
		dataset.Description,
		string(dataset.Status),
		nullableString(string(dataset.LatestVersionID)),
		dataset.VersionCount,
		dataset.HasContent,
		dataset.HasInstructions,
		owners,
		dataset.Tags,
		dataset.Metadata,
		string(dataset.ID),
	)
	if err != nil {
		return fmt.Errorf("failed to update dataset: %w", err)
	}

	if rowsAffected == 0 {
		return storage.ErrNotFound
	}

	return nil
}

// Delete soft-deletes a dataset.
func (r *DatasetRepository) Delete(ctx context.Context, id domain.DatasetID) error {
	rowsAffected, err := r.store.execWithAudit(ctx, "UPDATE", "datasets", `
		UPDATE datasets SET status = 'deleted' WHERE id = $1
	`, string(id))
	if err != nil {
		return fmt.Errorf("failed to delete dataset: %w", err)
	}

	if rowsAffected == 0 {
		return storage.ErrNotFound
	}

	return nil
}

// Count returns the number of datasets matching the filter.
func (r *DatasetRepository) Count(ctx context.Context, filter storage.DatasetFilter) (int64, error) {
	query := "SELECT COUNT(*) FROM datasets WHERE 1=1"
	args := []any{}
	argNum := 1

	if filter.TopicID != nil {
		query += fmt.Sprintf(" AND topic_id = $%d", argNum)
		args = append(args, string(*filter.TopicID))
		argNum++
	}

	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, string(filter.Status))
		argNum++
	}

	rows, err := r.store.queryWithAudit(ctx, "datasets", query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to count datasets: %w", err)
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

// CreateVersion creates a new dataset version.
func (r *DatasetRepository) CreateVersion(ctx context.Context, version *domain.DatasetVersion) error {
	if err := version.Validate(); err != nil {
		return err
	}

	_, err := r.store.execWithAudit(ctx, "INSERT", "dataset_versions", `
		INSERT INTO dataset_versions (id, dataset_id, version, previous_version_id, content, instructions, table_schema, created_by, created_at, message, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`,
		string(version.ID),
		string(version.DatasetID),
		version.Version,
		nullableString(string(version.PreviousVersionID)),
		version.Content,
		version.Instructions,
		version.TableSchema,
		string(version.CreatedBy),
		version.CreatedAt,
		version.Message,
		version.Metadata,
	)

	if err != nil {
		if isUniqueViolation(err) {
			return storage.ErrAlreadyExists
		}
		return fmt.Errorf("failed to create version: %w", err)
	}

	return nil
}

// GetVersion retrieves a specific version.
func (r *DatasetRepository) GetVersion(ctx context.Context, datasetID domain.DatasetID, versionID domain.DatasetVersionID) (*domain.DatasetVersion, error) {
	rows, err := r.store.queryWithAudit(ctx, "dataset_versions", `
		SELECT id, dataset_id, version, previous_version_id, content, instructions, table_schema, created_by, created_at, message, metadata
		FROM dataset_versions WHERE dataset_id = $1 AND id = $2
	`, string(datasetID), string(versionID))
	if err != nil {
		return nil, fmt.Errorf("failed to query version: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, storage.ErrNotFound
	}

	return scanVersion(rows)
}

// GetLatestVersion retrieves the latest version of a dataset.
func (r *DatasetRepository) GetLatestVersion(ctx context.Context, datasetID domain.DatasetID) (*domain.DatasetVersion, error) {
	rows, err := r.store.queryWithAudit(ctx, "dataset_versions", `
		SELECT id, dataset_id, version, previous_version_id, content, instructions, table_schema, created_by, created_at, message, metadata
		FROM dataset_versions WHERE dataset_id = $1 ORDER BY created_at DESC LIMIT 1
	`, string(datasetID))
	if err != nil {
		return nil, fmt.Errorf("failed to query latest version: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, storage.ErrNotFound
	}

	return scanVersion(rows)
}

// ListVersions lists all versions of a dataset.
func (r *DatasetRepository) ListVersions(ctx context.Context, datasetID domain.DatasetID) ([]*domain.DatasetVersion, error) {
	rows, err := r.store.queryWithAudit(ctx, "dataset_versions", `
		SELECT id, dataset_id, version, previous_version_id, content, instructions, table_schema, created_by, created_at, message, metadata
		FROM dataset_versions WHERE dataset_id = $1 ORDER BY created_at DESC
	`, string(datasetID))
	if err != nil {
		return nil, fmt.Errorf("failed to query versions: %w", err)
	}
	defer rows.Close()

	var versions []*domain.DatasetVersion
	for rows.Next() {
		version, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}

	return versions, rows.Err()
}

// CreateChunk creates a new chunk record.
func (r *DatasetRepository) CreateChunk(ctx context.Context, chunk *domain.Chunk) error {
	if err := chunk.Validate(); err != nil {
		return err
	}

	_, err := r.store.execWithAudit(ctx, "INSERT", "chunks", `
		INSERT INTO chunks (id, dataset_id, version_id, chunk_index, hash, size, status, storage_path)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`,
		string(chunk.ID),
		string(chunk.DatasetID),
		string(chunk.VersionID),
		chunk.Index,
		chunk.Hash,
		chunk.Size,
		string(chunk.Status),
		chunk.StoragePath,
	)

	if err != nil {
		if isUniqueViolation(err) {
			return storage.ErrAlreadyExists
		}
		return fmt.Errorf("failed to create chunk: %w", err)
	}

	return nil
}

// GetChunk retrieves a chunk by version and index.
func (r *DatasetRepository) GetChunk(ctx context.Context, versionID domain.DatasetVersionID, index int) (*domain.Chunk, error) {
	rows, err := r.store.queryWithAudit(ctx, "chunks", `
		SELECT id, dataset_id, version_id, chunk_index, hash, size, status, storage_path
		FROM chunks WHERE version_id = $1 AND chunk_index = $2
	`, string(versionID), index)
	if err != nil {
		return nil, fmt.Errorf("failed to query chunk: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, storage.ErrNotFound
	}

	return scanChunk(rows)
}

// ListChunks lists all chunks for a version.
func (r *DatasetRepository) ListChunks(ctx context.Context, versionID domain.DatasetVersionID) ([]*domain.Chunk, error) {
	rows, err := r.store.queryWithAudit(ctx, "chunks", `
		SELECT id, dataset_id, version_id, chunk_index, hash, size, status, storage_path
		FROM chunks WHERE version_id = $1 ORDER BY chunk_index
	`, string(versionID))
	if err != nil {
		return nil, fmt.Errorf("failed to query chunks: %w", err)
	}
	defer rows.Close()

	var chunks []*domain.Chunk
	for rows.Next() {
		chunk, err := scanChunk(rows)
		if err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}

	return chunks, rows.Err()
}

// UpdateChunkStatus updates the status of a chunk.
func (r *DatasetRepository) UpdateChunkStatus(ctx context.Context, chunkID domain.ChunkID, status domain.ChunkStatus) error {
	rowsAffected, err := r.store.execWithAudit(ctx, "UPDATE", "chunks", `
		UPDATE chunks SET status = $1 WHERE id = $2
	`, string(status), string(chunkID))
	if err != nil {
		return fmt.Errorf("failed to update chunk status: %w", err)
	}

	if rowsAffected == 0 {
		return storage.ErrNotFound
	}

	return nil
}

// Helper functions

func scanDataset(rows pgx.Rows) (*domain.Dataset, error) {
	var (
		id              string
		topicID         string
		name            string
		description     string
		status          string
		latestVersionID *string
		versionCount    int
		hasContent      bool
		hasInstructions bool
		owners          []string
		createdBy       string
		createdAt       interface{}
		updatedAt       interface{}
		tags            []string
		metadata        map[string]string
	)

	err := rows.Scan(
		&id, &topicID, &name, &description, &status, &latestVersionID,
		&versionCount, &hasContent, &hasInstructions, &owners,
		&createdBy, &createdAt, &updatedAt, &tags, &metadata,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan dataset: %w", err)
	}

	dataset := &domain.Dataset{
		ID:              domain.DatasetID(id),
		TopicID:         domain.TopicID(topicID),
		Name:            name,
		Description:     description,
		Status:          domain.DatasetStatus(status),
		VersionCount:    versionCount,
		HasContent:      hasContent,
		HasInstructions: hasInstructions,
		CreatedBy:       domain.UserID(createdBy),
		Tags:            tags,
		Metadata:        metadata,
	}

	if latestVersionID != nil {
		dataset.LatestVersionID = domain.DatasetVersionID(*latestVersionID)
	}

	dataset.CreatedAt = parseTime(createdAt)
	dataset.UpdatedAt = parseTime(updatedAt)

	for _, o := range owners {
		dataset.Owners = append(dataset.Owners, domain.UserID(o))
	}

	return dataset, nil
}

func scanVersion(rows pgx.Rows) (*domain.DatasetVersion, error) {
	var (
		id            string
		datasetID     string
		version       string
		prevVersionID *string
		content       *domain.DatasetContent
		instructions  *domain.DatasetInstructions
		tableSchema   string
		createdBy     string
		createdAt     interface{}
		message       string
		metadata      map[string]string
	)

	err := rows.Scan(
		&id, &datasetID, &version, &prevVersionID, &content,
		&instructions, &tableSchema, &createdBy, &createdAt,
		&message, &metadata,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan version: %w", err)
	}

	v := &domain.DatasetVersion{
		ID:           domain.DatasetVersionID(id),
		DatasetID:    domain.DatasetID(datasetID),
		Version:      version,
		Content:      content,
		Instructions: instructions,
		TableSchema:  tableSchema,
		CreatedBy:    domain.UserID(createdBy),
		Message:      message,
		Metadata:     metadata,
	}

	if prevVersionID != nil {
		v.PreviousVersionID = domain.DatasetVersionID(*prevVersionID)
	}

	v.CreatedAt = parseTime(createdAt)

	return v, nil
}

func scanChunk(rows pgx.Rows) (*domain.Chunk, error) {
	var (
		id          string
		datasetID   string
		versionID   string
		index       int
		hash        string
		size        int64
		status      string
		storagePath *string
	)

	err := rows.Scan(&id, &datasetID, &versionID, &index, &hash, &size, &status, &storagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to scan chunk: %w", err)
	}

	chunk := &domain.Chunk{
		ID:        domain.ChunkID(id),
		DatasetID: domain.DatasetID(datasetID),
		VersionID: domain.DatasetVersionID(versionID),
		Index:     index,
		Hash:      hash,
		Size:      size,
		Status:    domain.ChunkStatus(status),
	}

	if storagePath != nil {
		chunk.StoragePath = *storagePath
	}

	return chunk, nil
}

func parseTime(v interface{}) time.Time {
	if v == nil {
		return time.Time{}
	}
	switch t := v.(type) {
	case time.Time:
		return t
	default:
		return time.Time{}
	}
}

// Ensure interface compliance
var _ storage.DatasetRepository = (*DatasetRepository)(nil)
