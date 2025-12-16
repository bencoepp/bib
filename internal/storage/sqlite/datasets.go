package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"bib/internal/domain"
	"bib/internal/storage"
)

// DatasetRepository implements storage.DatasetRepository for SQLite.
type DatasetRepository struct {
	store *Store
}

// Create creates a new dataset.
func (r *DatasetRepository) Create(ctx context.Context, dataset *domain.Dataset) error {
	if err := dataset.Validate(); err != nil {
		return err
	}

	ownersJSON, _ := json.Marshal(dataset.Owners)
	tagsJSON, _ := json.Marshal(dataset.Tags)
	metadataJSON, _ := json.Marshal(dataset.Metadata)
	now := time.Now().UTC().Format(time.RFC3339Nano)

	_, err := r.store.execWithAudit(ctx, "INSERT", "datasets", `
		INSERT INTO datasets (id, topic_id, name, description, status, latest_version_id, version_count, has_content, has_instructions, owners, created_by, created_at, updated_at, tags, metadata, cached_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		string(dataset.ID),
		string(dataset.TopicID),
		dataset.Name,
		dataset.Description,
		string(dataset.Status),
		nullString(string(dataset.LatestVersionID)),
		dataset.VersionCount,
		boolToInt(dataset.HasContent),
		boolToInt(dataset.HasInstructions),
		string(ownersJSON),
		string(dataset.CreatedBy),
		dataset.CreatedAt.UTC().Format(time.RFC3339Nano),
		dataset.UpdatedAt.UTC().Format(time.RFC3339Nano),
		string(tagsJSON),
		string(metadataJSON),
		now,
	)

	if err != nil {
		if isConstraintError(err) {
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
		FROM datasets WHERE id = ?
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

	if filter.TopicID != nil {
		query += " AND topic_id = ?"
		args = append(args, string(*filter.TopicID))
	}

	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, string(filter.Status))
	}

	if filter.OwnerID != nil {
		query += " AND owners LIKE ?"
		args = append(args, "%"+string(*filter.OwnerID)+"%")
	}

	if filter.HasContent != nil {
		query += " AND has_content = ?"
		args = append(args, boolToInt(*filter.HasContent))
	}

	if filter.HasInstructions != nil {
		query += " AND has_instructions = ?"
		args = append(args, boolToInt(*filter.HasInstructions))
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

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
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

	ownersJSON, _ := json.Marshal(dataset.Owners)
	tagsJSON, _ := json.Marshal(dataset.Tags)
	metadataJSON, _ := json.Marshal(dataset.Metadata)
	now := time.Now().UTC().Format(time.RFC3339Nano)

	result, err := r.store.execWithAudit(ctx, "UPDATE", "datasets", `
		UPDATE datasets SET
			topic_id = ?,
			name = ?,
			description = ?,
			status = ?,
			latest_version_id = ?,
			version_count = ?,
			has_content = ?,
			has_instructions = ?,
			owners = ?,
			updated_at = ?,
			tags = ?,
			metadata = ?,
			cached_at = ?
		WHERE id = ?
	`,
		string(dataset.TopicID),
		dataset.Name,
		dataset.Description,
		string(dataset.Status),
		nullString(string(dataset.LatestVersionID)),
		dataset.VersionCount,
		boolToInt(dataset.HasContent),
		boolToInt(dataset.HasInstructions),
		string(ownersJSON),
		now,
		string(tagsJSON),
		string(metadataJSON),
		now,
		string(dataset.ID),
	)
	if err != nil {
		return fmt.Errorf("failed to update dataset: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return storage.ErrNotFound
	}

	return nil
}

// Delete soft-deletes a dataset.
func (r *DatasetRepository) Delete(ctx context.Context, id domain.DatasetID) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)

	result, err := r.store.execWithAudit(ctx, "UPDATE", "datasets", `
		UPDATE datasets SET status = 'deleted', updated_at = ? WHERE id = ?
	`, now, string(id))
	if err != nil {
		return fmt.Errorf("failed to delete dataset: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return storage.ErrNotFound
	}

	return nil
}

// Count returns the number of datasets matching the filter.
func (r *DatasetRepository) Count(ctx context.Context, filter storage.DatasetFilter) (int64, error) {
	query := "SELECT COUNT(*) FROM datasets WHERE 1=1"
	args := []any{}

	if filter.TopicID != nil {
		query += " AND topic_id = ?"
		args = append(args, string(*filter.TopicID))
	}

	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, string(filter.Status))
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

	contentJSON, _ := json.Marshal(version.Content)
	instructionsJSON, _ := json.Marshal(version.Instructions)
	metadataJSON, _ := json.Marshal(version.Metadata)
	now := time.Now().UTC().Format(time.RFC3339Nano)

	_, err := r.store.execWithAudit(ctx, "INSERT", "dataset_versions", `
		INSERT INTO dataset_versions (id, dataset_id, version, previous_version_id, content, instructions, table_schema, created_by, created_at, message, metadata, cached_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		string(version.ID),
		string(version.DatasetID),
		version.Version,
		nullString(string(version.PreviousVersionID)),
		string(contentJSON),
		string(instructionsJSON),
		version.TableSchema,
		string(version.CreatedBy),
		version.CreatedAt.UTC().Format(time.RFC3339Nano),
		version.Message,
		string(metadataJSON),
		now,
	)

	if err != nil {
		if isConstraintError(err) {
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
		FROM dataset_versions WHERE dataset_id = ? AND id = ?
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
		FROM dataset_versions WHERE dataset_id = ? ORDER BY created_at DESC LIMIT 1
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
		FROM dataset_versions WHERE dataset_id = ? ORDER BY created_at DESC
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

	now := time.Now().UTC().Format(time.RFC3339Nano)

	_, err := r.store.execWithAudit(ctx, "INSERT", "chunks", `
		INSERT INTO chunks (id, dataset_id, version_id, chunk_index, hash, size, status, storage_path, cached_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		string(chunk.ID),
		string(chunk.DatasetID),
		string(chunk.VersionID),
		chunk.Index,
		chunk.Hash,
		chunk.Size,
		string(chunk.Status),
		chunk.StoragePath,
		now,
	)

	if err != nil {
		if isConstraintError(err) {
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
		FROM chunks WHERE version_id = ? AND chunk_index = ?
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
		FROM chunks WHERE version_id = ? ORDER BY chunk_index
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
	result, err := r.store.execWithAudit(ctx, "UPDATE", "chunks", `
		UPDATE chunks SET status = ? WHERE id = ?
	`, string(status), string(chunkID))
	if err != nil {
		return fmt.Errorf("failed to update chunk status: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return storage.ErrNotFound
	}

	return nil
}

// Helper functions

func scanDataset(rows *sql.Rows) (*domain.Dataset, error) {
	var (
		id              string
		topicID         string
		name            string
		description     string
		status          string
		latestVersionID sql.NullString
		versionCount    int
		hasContent      int
		hasInstructions int
		ownersJSON      string
		createdBy       string
		createdAt       string
		updatedAt       string
		tagsJSON        sql.NullString
		metadataJSON    sql.NullString
	)

	err := rows.Scan(
		&id, &topicID, &name, &description, &status, &latestVersionID,
		&versionCount, &hasContent, &hasInstructions, &ownersJSON,
		&createdBy, &createdAt, &updatedAt, &tagsJSON, &metadataJSON,
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
		HasContent:      hasContent == 1,
		HasInstructions: hasInstructions == 1,
		CreatedBy:       domain.UserID(createdBy),
	}

	if latestVersionID.Valid {
		dataset.LatestVersionID = domain.DatasetVersionID(latestVersionID.String)
	}

	if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
		dataset.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339Nano, updatedAt); err == nil {
		dataset.UpdatedAt = t
	}

	if ownersJSON != "" {
		var owners []domain.UserID
		if err := json.Unmarshal([]byte(ownersJSON), &owners); err == nil {
			dataset.Owners = owners
		}
	}

	if tagsJSON.Valid && tagsJSON.String != "" {
		var tags []string
		if err := json.Unmarshal([]byte(tagsJSON.String), &tags); err == nil {
			dataset.Tags = tags
		}
	}

	if metadataJSON.Valid && metadataJSON.String != "" {
		var metadata map[string]string
		if err := json.Unmarshal([]byte(metadataJSON.String), &metadata); err == nil {
			dataset.Metadata = metadata
		}
	}

	return dataset, nil
}

func scanVersion(rows *sql.Rows) (*domain.DatasetVersion, error) {
	var (
		id               string
		datasetID        string
		version          string
		prevVersionID    sql.NullString
		contentJSON      sql.NullString
		instructionsJSON sql.NullString
		tableSchema      string
		createdBy        string
		createdAt        string
		message          string
		metadataJSON     sql.NullString
	)

	err := rows.Scan(
		&id, &datasetID, &version, &prevVersionID, &contentJSON,
		&instructionsJSON, &tableSchema, &createdBy, &createdAt,
		&message, &metadataJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan version: %w", err)
	}

	v := &domain.DatasetVersion{
		ID:          domain.DatasetVersionID(id),
		DatasetID:   domain.DatasetID(datasetID),
		Version:     version,
		TableSchema: tableSchema,
		CreatedBy:   domain.UserID(createdBy),
		Message:     message,
	}

	if prevVersionID.Valid {
		v.PreviousVersionID = domain.DatasetVersionID(prevVersionID.String)
	}

	if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
		v.CreatedAt = t
	}

	if contentJSON.Valid && contentJSON.String != "" && contentJSON.String != "null" {
		var content domain.DatasetContent
		if err := json.Unmarshal([]byte(contentJSON.String), &content); err == nil {
			v.Content = &content
		}
	}

	if instructionsJSON.Valid && instructionsJSON.String != "" && instructionsJSON.String != "null" {
		var instructions domain.DatasetInstructions
		if err := json.Unmarshal([]byte(instructionsJSON.String), &instructions); err == nil {
			v.Instructions = &instructions
		}
	}

	if metadataJSON.Valid && metadataJSON.String != "" {
		var metadata map[string]string
		if err := json.Unmarshal([]byte(metadataJSON.String), &metadata); err == nil {
			v.Metadata = metadata
		}
	}

	return v, nil
}

func scanChunk(rows *sql.Rows) (*domain.Chunk, error) {
	var (
		id          string
		datasetID   string
		versionID   string
		index       int
		hash        string
		size        int64
		status      string
		storagePath sql.NullString
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

	if storagePath.Valid {
		chunk.StoragePath = storagePath.String
	}

	return chunk, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Ensure interface compliance
var _ storage.DatasetRepository = (*DatasetRepository)(nil)
