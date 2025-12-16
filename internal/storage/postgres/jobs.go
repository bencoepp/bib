package postgres

import (
	"context"
	"fmt"
	"time"

	"bib/internal/domain"
	"bib/internal/storage"

	"github.com/jackc/pgx/v5"
)

// JobRepository implements storage.JobRepository for PostgreSQL.
type JobRepository struct {
	store *Store
}

// Create creates a new job.
func (r *JobRepository) Create(ctx context.Context, job *domain.Job) error {
	if err := job.Validate(); err != nil {
		return err
	}

	// Convert dependencies to string array for PostgreSQL UUID[]
	var dependencies []string
	for _, d := range job.Dependencies {
		dependencies = append(dependencies, string(d))
	}

	_, err := r.store.execWithAudit(ctx, "INSERT", "jobs", `
		INSERT INTO jobs (id, type, status, task_id, inline_instructions, execution_mode, schedule, inputs, outputs, dependencies, topic_id, dataset_id, priority, resource_limits, created_by, created_at, started_at, completed_at, error, result, progress, current_instruction, node_id, retry_count, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25)
	`,
		string(job.ID),
		string(job.Type),
		string(job.Status),
		nullableString(string(job.TaskID)),
		job.InlineInstructions,
		string(job.ExecutionMode),
		job.Schedule,
		job.Inputs,
		job.Outputs,
		dependencies,
		nullableString(string(job.TopicID)),
		nullableString(string(job.DatasetID)),
		job.Priority,
		job.ResourceLimits,
		string(job.CreatedBy),
		job.CreatedAt,
		job.StartedAt,
		job.CompletedAt,
		job.Error,
		job.Result,
		job.Progress,
		job.CurrentInstruction,
		job.NodeID,
		job.RetryCount,
		job.Metadata,
	)

	if err != nil {
		if isUniqueViolation(err) {
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
		FROM jobs WHERE id = $1
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
	argNum := 1

	if filter.Type != "" {
		query += fmt.Sprintf(" AND type = $%d", argNum)
		args = append(args, string(filter.Type))
		argNum++
	}

	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, string(filter.Status))
		argNum++
	}

	if filter.CreatedBy != "" {
		query += fmt.Sprintf(" AND created_by = $%d", argNum)
		args = append(args, filter.CreatedBy)
		argNum++
	}

	if filter.TopicID != nil {
		query += fmt.Sprintf(" AND topic_id = $%d", argNum)
		args = append(args, string(*filter.TopicID))
		argNum++
	}

	if filter.DatasetID != nil {
		query += fmt.Sprintf(" AND dataset_id = $%d", argNum)
		args = append(args, string(*filter.DatasetID))
		argNum++
	}

	if filter.MinPriority != nil {
		query += fmt.Sprintf(" AND priority >= $%d", argNum)
		args = append(args, *filter.MinPriority)
		argNum++
	}

	if filter.CreatedAfter != nil {
		query += fmt.Sprintf(" AND created_at >= $%d", argNum)
		args = append(args, *filter.CreatedAfter)
		argNum++
	}

	if filter.CreatedBefore != nil {
		query += fmt.Sprintf(" AND created_at <= $%d", argNum)
		args = append(args, *filter.CreatedBefore)
		argNum++
	}

	orderBy := "created_at"
	if filter.OrderBy != "" {
		orderBy = filter.OrderBy
	}
	order := "DESC"
	if !filter.OrderDesc {
		order = "ASC"
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

	var dependencies []string
	for _, d := range job.Dependencies {
		dependencies = append(dependencies, string(d))
	}

	rowsAffected, err := r.store.execWithAudit(ctx, "UPDATE", "jobs", `
		UPDATE jobs SET
			type = $1,
			status = $2,
			task_id = $3,
			inline_instructions = $4,
			execution_mode = $5,
			schedule = $6,
			inputs = $7,
			outputs = $8,
			dependencies = $9,
			topic_id = $10,
			dataset_id = $11,
			priority = $12,
			resource_limits = $13,
			started_at = $14,
			completed_at = $15,
			error = $16,
			result = $17,
			progress = $18,
			current_instruction = $19,
			node_id = $20,
			retry_count = $21,
			metadata = $22
		WHERE id = $23
	`,
		string(job.Type),
		string(job.Status),
		nullableString(string(job.TaskID)),
		job.InlineInstructions,
		string(job.ExecutionMode),
		job.Schedule,
		job.Inputs,
		job.Outputs,
		dependencies,
		nullableString(string(job.TopicID)),
		nullableString(string(job.DatasetID)),
		job.Priority,
		job.ResourceLimits,
		job.StartedAt,
		job.CompletedAt,
		job.Error,
		job.Result,
		job.Progress,
		job.CurrentInstruction,
		job.NodeID,
		job.RetryCount,
		job.Metadata,
		string(job.ID),
	)
	if err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	if rowsAffected == 0 {
		return storage.ErrNotFound
	}

	return nil
}

// UpdateStatus updates just the job status.
func (r *JobRepository) UpdateStatus(ctx context.Context, id domain.JobID, status domain.JobStatus) error {
	now := time.Now().UTC()

	var query string
	var args []any

	switch status {
	case domain.JobStatusRunning:
		query = "UPDATE jobs SET status = $1, started_at = $2 WHERE id = $3"
		args = []any{string(status), now, string(id)}
	case domain.JobStatusCompleted, domain.JobStatusFailed, domain.JobStatusCancelled:
		query = "UPDATE jobs SET status = $1, completed_at = $2 WHERE id = $3"
		args = []any{string(status), now, string(id)}
	default:
		query = "UPDATE jobs SET status = $1 WHERE id = $2"
		args = []any{string(status), string(id)}
	}

	rowsAffected, err := r.store.execWithAudit(ctx, "UPDATE", "jobs", query, args...)
	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	if rowsAffected == 0 {
		return storage.ErrNotFound
	}

	return nil
}

// Delete deletes a job.
func (r *JobRepository) Delete(ctx context.Context, id domain.JobID) error {
	rowsAffected, err := r.store.execWithAudit(ctx, "DELETE", "jobs", `
		DELETE FROM jobs WHERE id = $1
	`, string(id))
	if err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}

	if rowsAffected == 0 {
		return storage.ErrNotFound
	}

	return nil
}

// Count returns the number of jobs matching the filter.
func (r *JobRepository) Count(ctx context.Context, filter storage.JobFilter) (int64, error) {
	query := "SELECT COUNT(*) FROM jobs WHERE 1=1"
	args := []any{}
	argNum := 1

	if filter.Type != "" {
		query += fmt.Sprintf(" AND type = $%d", argNum)
		args = append(args, string(filter.Type))
		argNum++
	}

	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argNum)
		args = append(args, string(filter.Status))
		argNum++
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
		FROM jobs WHERE status = $1 ORDER BY priority DESC, created_at ASC
	`
	args := []any{string(domain.JobStatusPending)}

	if limit > 0 {
		query += " LIMIT $2"
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
	_, err := r.store.execWithAudit(ctx, "INSERT", "job_results", `
		INSERT INTO job_results (id, job_id, node_id, status, result, error, started_at, completed_at, duration_ms, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`,
		result.ID,
		string(result.JobID),
		result.NodeID,
		string(result.Status),
		result.Result,
		result.Error,
		result.StartedAt,
		result.CompletedAt,
		result.DurationMS,
		result.Metadata,
	)

	if err != nil {
		if isUniqueViolation(err) {
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
		FROM job_results WHERE id = $1
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
		FROM job_results WHERE job_id = $1 ORDER BY completed_at DESC
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

func scanJob(rows pgx.Rows) (*domain.Job, error) {
	var (
		id                 string
		jobType            string
		status             string
		taskID             *string
		inlineInstructions []domain.Instruction
		executionMode      string
		schedule           *domain.Schedule
		inputs             []domain.JobInput
		outputs            []domain.JobOutput
		dependencies       []string
		topicID            *string
		datasetID          *string
		priority           int
		resourceLimits     *domain.ResourceLimits
		createdBy          string
		createdAt          interface{}
		startedAt          interface{}
		completedAt        interface{}
		jobError           string
		result             string
		progress           int
		currentInstruction int
		nodeID             string
		retryCount         int
		metadata           map[string]string
	)

	err := rows.Scan(
		&id, &jobType, &status, &taskID, &inlineInstructions, &executionMode,
		&schedule, &inputs, &outputs, &dependencies,
		&topicID, &datasetID, &priority, &resourceLimits, &createdBy,
		&createdAt, &startedAt, &completedAt, &jobError, &result,
		&progress, &currentInstruction, &nodeID, &retryCount, &metadata,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan job: %w", err)
	}

	job := &domain.Job{
		ID:                 domain.JobID(id),
		Type:               domain.JobType(jobType),
		Status:             domain.JobStatus(status),
		InlineInstructions: inlineInstructions,
		ExecutionMode:      domain.ExecutionMode(executionMode),
		Schedule:           schedule,
		Inputs:             inputs,
		Outputs:            outputs,
		Priority:           priority,
		ResourceLimits:     resourceLimits,
		CreatedBy:          domain.UserID(createdBy),
		Error:              jobError,
		Result:             result,
		Progress:           progress,
		CurrentInstruction: currentInstruction,
		NodeID:             nodeID,
		RetryCount:         retryCount,
		Metadata:           metadata,
	}

	if taskID != nil {
		job.TaskID = domain.TaskID(*taskID)
	}

	if topicID != nil {
		job.TopicID = domain.TopicID(*topicID)
	}

	if datasetID != nil {
		job.DatasetID = domain.DatasetID(*datasetID)
	}

	// Convert dependencies
	for _, d := range dependencies {
		job.Dependencies = append(job.Dependencies, domain.JobID(d))
	}

	// Parse timestamps
	job.CreatedAt = parseTime(createdAt)

	if startedAt != nil {
		t := parseTime(startedAt)
		if !t.IsZero() {
			job.StartedAt = &t
		}
	}

	if completedAt != nil {
		t := parseTime(completedAt)
		if !t.IsZero() {
			job.CompletedAt = &t
		}
	}

	return job, nil
}

func scanJobResult(rows pgx.Rows) (*domain.JobResult, error) {
	var (
		id          string
		jobID       string
		nodeID      string
		status      string
		result      *string
		resultError *string
		startedAt   interface{}
		completedAt interface{}
		durationMS  int64
		metadata    map[string]any
	)

	err := rows.Scan(&id, &jobID, &nodeID, &status, &result, &resultError, &startedAt, &completedAt, &durationMS, &metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to scan job result: %w", err)
	}

	jr := &domain.JobResult{
		ID:         id,
		JobID:      domain.JobID(jobID),
		NodeID:     nodeID,
		Status:     domain.JobStatus(status),
		DurationMS: durationMS,
		Metadata:   metadata,
	}

	if result != nil {
		jr.Result = *result
	}

	if resultError != nil {
		jr.Error = *resultError
	}

	jr.CompletedAt = parseTime(completedAt)

	if startedAt != nil {
		t := parseTime(startedAt)
		if !t.IsZero() {
			jr.StartedAt = &t
		}
	}

	return jr, nil
}

// Ensure interface compliance
var _ storage.JobRepository = (*JobRepository)(nil)
