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

// JobRepository implements storage.JobRepository for SQLite.
type JobRepository struct {
	store *Store
}

// Create creates a new job.
func (r *JobRepository) Create(ctx context.Context, job *domain.Job) error {
	if err := job.Validate(); err != nil {
		return err
	}

	inlineInstructionsJSON, _ := json.Marshal(job.InlineInstructions)
	scheduleJSON, _ := json.Marshal(job.Schedule)
	inputsJSON, _ := json.Marshal(job.Inputs)
	outputsJSON, _ := json.Marshal(job.Outputs)
	dependenciesJSON, _ := json.Marshal(job.Dependencies)
	resourceLimitsJSON, _ := json.Marshal(job.ResourceLimits)
	metadataJSON, _ := json.Marshal(job.Metadata)

	_, err := r.store.execWithAudit(ctx, "INSERT", "jobs", `
		INSERT INTO jobs (id, type, status, task_id, inline_instructions, execution_mode, schedule, inputs, outputs, dependencies, topic_id, dataset_id, priority, resource_limits, created_by, created_at, started_at, completed_at, error, result, progress, current_instruction, node_id, retry_count, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		string(job.ID),
		string(job.Type),
		string(job.Status),
		nullString(string(job.TaskID)),
		string(inlineInstructionsJSON),
		string(job.ExecutionMode),
		string(scheduleJSON),
		string(inputsJSON),
		string(outputsJSON),
		string(dependenciesJSON),
		nullString(string(job.TopicID)),
		nullString(string(job.DatasetID)),
		job.Priority,
		string(resourceLimitsJSON),
		string(job.CreatedBy),
		job.CreatedAt.UTC().Format(time.RFC3339Nano),
		nullTime(job.StartedAt),
		nullTime(job.CompletedAt),
		job.Error,
		job.Result,
		job.Progress,
		job.CurrentInstruction,
		job.NodeID,
		job.RetryCount,
		string(metadataJSON),
	)

	if err != nil {
		if isConstraintError(err) {
			return storage.ErrAlreadyExists
		}
		return fmt.Errorf("failed to create job: %w", err)
	}

	return nil
}

// Get retrieves a job by ID.
func (r *JobRepository) Get(ctx context.Context, id domain.JobID) (*domain.Job, error) {
	rows, err := r.store.queryWithAudit(ctx, "jobs", `
		SELECT id, type, status, task_id, inline_instructions, execution_mode, schedule, inputs, outputs, dependencies, topic_id, dataset_id, priority, resource_limits, created_by, created_at, started_at, completed_at, error, result, progress, current_instruction, node_id, retry_count, metadata
		FROM jobs WHERE id = ?
	`, string(id))
	if err != nil {
		return nil, fmt.Errorf("failed to query job: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, storage.ErrNotFound
	}

	return scanJob(rows)
}

// List retrieves jobs matching the filter.
func (r *JobRepository) List(ctx context.Context, filter storage.JobFilter) ([]*domain.Job, error) {
	query := `
		SELECT id, type, status, task_id, inline_instructions, execution_mode, schedule, inputs, outputs, dependencies, topic_id, dataset_id, priority, resource_limits, created_by, created_at, started_at, completed_at, error, result, progress, current_instruction, node_id, retry_count, metadata
		FROM jobs WHERE 1=1
	`
	args := []any{}

	if filter.Type != "" {
		query += " AND type = ?"
		args = append(args, string(filter.Type))
	}

	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, string(filter.Status))
	}

	if filter.CreatedBy != "" {
		query += " AND created_by = ?"
		args = append(args, filter.CreatedBy)
	}

	if filter.TopicID != nil {
		query += " AND topic_id = ?"
		args = append(args, string(*filter.TopicID))
	}

	if filter.DatasetID != nil {
		query += " AND dataset_id = ?"
		args = append(args, string(*filter.DatasetID))
	}

	if filter.MinPriority != nil {
		query += " AND priority >= ?"
		args = append(args, *filter.MinPriority)
	}

	if filter.CreatedAfter != nil {
		query += " AND created_at >= ?"
		args = append(args, filter.CreatedAfter.UTC().Format(time.RFC3339Nano))
	}

	if filter.CreatedBefore != nil {
		query += " AND created_at <= ?"
		args = append(args, filter.CreatedBefore.UTC().Format(time.RFC3339Nano))
	}

	// Order by
	orderBy := "created_at"
	if filter.OrderBy != "" {
		orderBy = filter.OrderBy
	}
	order := "DESC"
	if filter.OrderDesc {
		order = "DESC"
	} else {
		order = "ASC"
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

	rows, err := r.store.queryWithAudit(ctx, "jobs", query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*domain.Job
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

// Update updates an existing job.
func (r *JobRepository) Update(ctx context.Context, job *domain.Job) error {
	if err := job.Validate(); err != nil {
		return err
	}

	inlineInstructionsJSON, _ := json.Marshal(job.InlineInstructions)
	scheduleJSON, _ := json.Marshal(job.Schedule)
	inputsJSON, _ := json.Marshal(job.Inputs)
	outputsJSON, _ := json.Marshal(job.Outputs)
	dependenciesJSON, _ := json.Marshal(job.Dependencies)
	resourceLimitsJSON, _ := json.Marshal(job.ResourceLimits)
	metadataJSON, _ := json.Marshal(job.Metadata)

	result, err := r.store.execWithAudit(ctx, "UPDATE", "jobs", `
		UPDATE jobs SET
			type = ?,
			status = ?,
			task_id = ?,
			inline_instructions = ?,
			execution_mode = ?,
			schedule = ?,
			inputs = ?,
			outputs = ?,
			dependencies = ?,
			topic_id = ?,
			dataset_id = ?,
			priority = ?,
			resource_limits = ?,
			started_at = ?,
			completed_at = ?,
			error = ?,
			result = ?,
			progress = ?,
			current_instruction = ?,
			node_id = ?,
			retry_count = ?,
			metadata = ?
		WHERE id = ?
	`,
		string(job.Type),
		string(job.Status),
		nullString(string(job.TaskID)),
		string(inlineInstructionsJSON),
		string(job.ExecutionMode),
		string(scheduleJSON),
		string(inputsJSON),
		string(outputsJSON),
		string(dependenciesJSON),
		nullString(string(job.TopicID)),
		nullString(string(job.DatasetID)),
		job.Priority,
		string(resourceLimitsJSON),
		nullTime(job.StartedAt),
		nullTime(job.CompletedAt),
		job.Error,
		job.Result,
		job.Progress,
		job.CurrentInstruction,
		job.NodeID,
		job.RetryCount,
		string(metadataJSON),
		string(job.ID),
	)
	if err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return storage.ErrNotFound
	}

	return nil
}

// UpdateStatus updates just the job status.
func (r *JobRepository) UpdateStatus(ctx context.Context, id domain.JobID, status domain.JobStatus) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)

	query := "UPDATE jobs SET status = ?"
	args := []any{string(status)}

	// Update timestamps based on status
	switch status {
	case domain.JobStatusRunning:
		query += ", started_at = ?"
		args = append(args, now)
	case domain.JobStatusCompleted, domain.JobStatusFailed, domain.JobStatusCancelled:
		query += ", completed_at = ?"
		args = append(args, now)
	}

	query += " WHERE id = ?"
	args = append(args, string(id))

	result, err := r.store.execWithAudit(ctx, "UPDATE", "jobs", query, args...)
	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return storage.ErrNotFound
	}

	return nil
}

// Delete deletes a job.
func (r *JobRepository) Delete(ctx context.Context, id domain.JobID) error {
	result, err := r.store.execWithAudit(ctx, "DELETE", "jobs", `
		DELETE FROM jobs WHERE id = ?
	`, string(id))
	if err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return storage.ErrNotFound
	}

	return nil
}

// Count returns the number of jobs matching the filter.
func (r *JobRepository) Count(ctx context.Context, filter storage.JobFilter) (int64, error) {
	query := "SELECT COUNT(*) FROM jobs WHERE 1=1"
	args := []any{}

	if filter.Type != "" {
		query += " AND type = ?"
		args = append(args, string(filter.Type))
	}

	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, string(filter.Status))
	}

	rows, err := r.store.queryWithAudit(ctx, "jobs", query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to count jobs: %w", err)
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

// GetPending retrieves pending jobs ordered by priority.
func (r *JobRepository) GetPending(ctx context.Context, limit int) ([]*domain.Job, error) {
	query := `
		SELECT id, type, status, task_id, inline_instructions, execution_mode, schedule, inputs, outputs, dependencies, topic_id, dataset_id, priority, resource_limits, created_by, created_at, started_at, completed_at, error, result, progress, current_instruction, node_id, retry_count, metadata
		FROM jobs WHERE status = ? ORDER BY priority DESC, created_at ASC
	`
	args := []any{string(domain.JobStatusPending)}

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := r.store.queryWithAudit(ctx, "jobs", query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*domain.Job
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

// CreateResult creates a job result.
func (r *JobRepository) CreateResult(ctx context.Context, result *domain.JobResult) error {
	metadataJSON, _ := json.Marshal(result.Metadata)

	_, err := r.store.execWithAudit(ctx, "INSERT", "job_results", `
		INSERT INTO job_results (id, job_id, node_id, status, result, error, started_at, completed_at, duration_ms, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		result.ID,
		string(result.JobID),
		result.NodeID,
		string(result.Status),
		result.Result,
		result.Error,
		nullTime(result.StartedAt),
		result.CompletedAt.UTC().Format(time.RFC3339Nano),
		result.DurationMS,
		string(metadataJSON),
	)

	if err != nil {
		if isConstraintError(err) {
			return storage.ErrAlreadyExists
		}
		return fmt.Errorf("failed to create job result: %w", err)
	}

	return nil
}

// GetResult retrieves a job result by ID.
func (r *JobRepository) GetResult(ctx context.Context, id string) (*domain.JobResult, error) {
	rows, err := r.store.queryWithAudit(ctx, "job_results", `
		SELECT id, job_id, node_id, status, result, error, started_at, completed_at, duration_ms, metadata
		FROM job_results WHERE id = ?
	`, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query job result: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, storage.ErrNotFound
	}

	return scanJobResult(rows)
}

// ListResults lists results for a job.
func (r *JobRepository) ListResults(ctx context.Context, jobID domain.JobID) ([]*domain.JobResult, error) {
	rows, err := r.store.queryWithAudit(ctx, "job_results", `
		SELECT id, job_id, node_id, status, result, error, started_at, completed_at, duration_ms, metadata
		FROM job_results WHERE job_id = ? ORDER BY completed_at DESC
	`, string(jobID))
	if err != nil {
		return nil, fmt.Errorf("failed to query job results: %w", err)
	}
	defer rows.Close()

	var results []*domain.JobResult
	for rows.Next() {
		result, err := scanJobResult(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	return results, rows.Err()
}

// Helper functions

func scanJob(rows *sql.Rows) (*domain.Job, error) {
	var (
		id                     string
		jobType                string
		status                 string
		taskID                 sql.NullString
		inlineInstructionsJSON string
		executionMode          string
		scheduleJSON           string
		inputsJSON             string
		outputsJSON            string
		dependenciesJSON       string
		topicID                sql.NullString
		datasetID              sql.NullString
		priority               int
		resourceLimitsJSON     string
		createdBy              string
		createdAt              string
		startedAt              sql.NullString
		completedAt            sql.NullString
		jobError               string
		result                 string
		progress               int
		currentInstruction     int
		nodeID                 string
		retryCount             int
		metadataJSON           sql.NullString
	)

	err := rows.Scan(
		&id, &jobType, &status, &taskID, &inlineInstructionsJSON, &executionMode,
		&scheduleJSON, &inputsJSON, &outputsJSON, &dependenciesJSON,
		&topicID, &datasetID, &priority, &resourceLimitsJSON, &createdBy,
		&createdAt, &startedAt, &completedAt, &jobError, &result,
		&progress, &currentInstruction, &nodeID, &retryCount, &metadataJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan job: %w", err)
	}

	job := &domain.Job{
		ID:                 domain.JobID(id),
		Type:               domain.JobType(jobType),
		Status:             domain.JobStatus(status),
		ExecutionMode:      domain.ExecutionMode(executionMode),
		Priority:           priority,
		CreatedBy:          domain.UserID(createdBy),
		Error:              jobError,
		Result:             result,
		Progress:           progress,
		CurrentInstruction: currentInstruction,
		NodeID:             nodeID,
		RetryCount:         retryCount,
	}

	if taskID.Valid {
		job.TaskID = domain.TaskID(taskID.String)
	}

	if topicID.Valid {
		job.TopicID = domain.TopicID(topicID.String)
	}

	if datasetID.Valid {
		job.DatasetID = domain.DatasetID(datasetID.String)
	}

	// Parse timestamps
	if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
		job.CreatedAt = t
	}

	if startedAt.Valid {
		if t, err := time.Parse(time.RFC3339Nano, startedAt.String); err == nil {
			job.StartedAt = &t
		}
	}

	if completedAt.Valid {
		if t, err := time.Parse(time.RFC3339Nano, completedAt.String); err == nil {
			job.CompletedAt = &t
		}
	}

	// Parse JSON fields
	if inlineInstructionsJSON != "" && inlineInstructionsJSON != "null" {
		var instructions []domain.Instruction
		if err := json.Unmarshal([]byte(inlineInstructionsJSON), &instructions); err == nil {
			job.InlineInstructions = instructions
		}
	}

	if scheduleJSON != "" && scheduleJSON != "null" {
		var schedule domain.Schedule
		if err := json.Unmarshal([]byte(scheduleJSON), &schedule); err == nil {
			job.Schedule = &schedule
		}
	}

	if inputsJSON != "" && inputsJSON != "null" {
		var inputs []domain.JobInput
		if err := json.Unmarshal([]byte(inputsJSON), &inputs); err == nil {
			job.Inputs = inputs
		}
	}

	if outputsJSON != "" && outputsJSON != "null" {
		var outputs []domain.JobOutput
		if err := json.Unmarshal([]byte(outputsJSON), &outputs); err == nil {
			job.Outputs = outputs
		}
	}

	if dependenciesJSON != "" && dependenciesJSON != "null" {
		var dependencies []domain.JobID
		if err := json.Unmarshal([]byte(dependenciesJSON), &dependencies); err == nil {
			job.Dependencies = dependencies
		}
	}

	if resourceLimitsJSON != "" && resourceLimitsJSON != "null" {
		var limits domain.ResourceLimits
		if err := json.Unmarshal([]byte(resourceLimitsJSON), &limits); err == nil {
			job.ResourceLimits = &limits
		}
	}

	if metadataJSON.Valid && metadataJSON.String != "" {
		var metadata map[string]string
		if err := json.Unmarshal([]byte(metadataJSON.String), &metadata); err == nil {
			job.Metadata = metadata
		}
	}

	return job, nil
}

func scanJobResult(rows *sql.Rows) (*domain.JobResult, error) {
	var (
		id           string
		jobID        string
		nodeID       string
		status       string
		result       sql.NullString
		resultError  sql.NullString
		startedAt    sql.NullString
		completedAt  string
		durationMS   int64
		metadataJSON sql.NullString
	)

	err := rows.Scan(&id, &jobID, &nodeID, &status, &result, &resultError, &startedAt, &completedAt, &durationMS, &metadataJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to scan job result: %w", err)
	}

	jr := &domain.JobResult{
		ID:         id,
		JobID:      domain.JobID(jobID),
		NodeID:     nodeID,
		Status:     domain.JobStatus(status),
		DurationMS: durationMS,
	}

	if result.Valid {
		jr.Result = result.String
	}

	if resultError.Valid {
		jr.Error = resultError.String
	}

	if t, err := time.Parse(time.RFC3339Nano, completedAt); err == nil {
		jr.CompletedAt = t
	}

	if startedAt.Valid {
		if t, err := time.Parse(time.RFC3339Nano, startedAt.String); err == nil {
			jr.StartedAt = &t
		}
	}

	if metadataJSON.Valid && metadataJSON.String != "" {
		var metadata map[string]any
		if err := json.Unmarshal([]byte(metadataJSON.String), &metadata); err == nil {
			jr.Metadata = metadata
		}
	}

	return jr, nil
}

func nullTime(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: t.UTC().Format(time.RFC3339Nano), Valid: true}
}

// Ensure interface compliance
var _ storage.JobRepository = (*JobRepository)(nil)
