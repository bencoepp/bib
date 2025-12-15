package domain

import (
	"time"
)

// JobID is a unique identifier for a job.
type JobID string

// String returns the string representation.
func (id JobID) String() string {
	return string(id)
}

// JobType represents the type of job.
type JobType string

const (
	JobTypeScrape    JobType = "scrape"
	JobTypeTransform JobType = "transform"
	JobTypeClean     JobType = "clean"
	JobTypeAnalyze   JobType = "analyze"
	JobTypeML        JobType = "ml"
	JobTypeETL       JobType = "etl"
	JobTypeIngest    JobType = "ingest"
	JobTypeExport    JobType = "export"
	JobTypeCustom    JobType = "custom"
)

// IsValid checks if the job type is valid.
func (t JobType) IsValid() bool {
	switch t {
	case JobTypeScrape, JobTypeTransform, JobTypeClean, JobTypeAnalyze,
		JobTypeML, JobTypeETL, JobTypeIngest, JobTypeExport, JobTypeCustom:
		return true
	default:
		return false
	}
}

// JobStatus represents the status of a job.
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusQueued    JobStatus = "queued"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
	JobStatusWaiting   JobStatus = "waiting" // waiting for dependencies
	JobStatusRetrying  JobStatus = "retrying"
)

// IsTerminal returns true if the status is a terminal state.
func (s JobStatus) IsTerminal() bool {
	return s == JobStatusCompleted || s == JobStatusFailed || s == JobStatusCancelled
}

// ExecutionMode represents how a job is executed.
type ExecutionMode string

const (
	// ExecutionModeGoroutine runs the job as a goroutine in the bibd process.
	ExecutionModeGoroutine ExecutionMode = "goroutine"

	// ExecutionModeContainer runs the job in a container (Docker/Podman).
	ExecutionModeContainer ExecutionMode = "container"

	// ExecutionModePod runs the job in a Kubernetes pod.
	ExecutionModePod ExecutionMode = "pod"
)

// IsValid checks if the execution mode is valid.
func (m ExecutionMode) IsValid() bool {
	switch m {
	case ExecutionModeGoroutine, ExecutionModeContainer, ExecutionModePod:
		return true
	default:
		return false
	}
}

// ResourceLimits defines resource constraints for job execution.
type ResourceLimits struct {
	// MaxMemoryMB is the maximum memory in megabytes.
	MaxMemoryMB int64 `json:"max_memory_mb,omitempty"`

	// MaxCPUCores is the maximum CPU cores (can be fractional).
	MaxCPUCores float64 `json:"max_cpu_cores,omitempty"`

	// TimeoutSeconds is the maximum execution time in seconds.
	TimeoutSeconds int64 `json:"timeout_seconds,omitempty"`

	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int `json:"max_retries,omitempty"`

	// MaxOutputSizeMB is the maximum output size in megabytes.
	MaxOutputSizeMB int64 `json:"max_output_size_mb,omitempty"`
}

// JobInput defines an input to a job.
type JobInput struct {
	// Name is the input variable name.
	Name string `json:"name"`

	// DatasetID is the source dataset (optional).
	DatasetID DatasetID `json:"dataset_id,omitempty"`

	// VersionID is the specific version to use (optional, defaults to latest).
	VersionID DatasetVersionID `json:"version_id,omitempty"`

	// Value is a literal value (optional, used instead of dataset).
	Value any `json:"value,omitempty"`
}

// JobOutput defines an output from a job.
type JobOutput struct {
	// Name is the output variable name.
	Name string `json:"name"`

	// DatasetID is the target dataset to store output (optional).
	DatasetID DatasetID `json:"dataset_id,omitempty"`

	// CreateNewVersion indicates whether to create a new version.
	CreateNewVersion bool `json:"create_new_version,omitempty"`

	// VersionMessage is the message for the new version.
	VersionMessage string `json:"version_message,omitempty"`
}

// Job represents a scheduled execution of a Task.
// Jobs are runtime instances that execute Task templates.
type Job struct {
	// ID is the unique identifier.
	ID JobID `json:"id"`

	// Type is the job type.
	Type JobType `json:"type"`

	// Status is the current status.
	Status JobStatus `json:"status"`

	// TaskID references the Task to execute.
	TaskID TaskID `json:"task_id,omitempty"`

	// InlineInstructions are used if TaskID is empty.
	InlineInstructions []Instruction `json:"inline_instructions,omitempty"`

	// ExecutionMode determines how the job runs.
	ExecutionMode ExecutionMode `json:"execution_mode"`

	// Schedule defines when/how often to run.
	Schedule *Schedule `json:"schedule,omitempty"`

	// Inputs are the job inputs.
	Inputs []JobInput `json:"inputs,omitempty"`

	// Outputs are the job outputs.
	Outputs []JobOutput `json:"outputs,omitempty"`

	// Dependencies are job IDs that must complete before this job runs.
	// Enables DAG/pipeline execution.
	Dependencies []JobID `json:"dependencies,omitempty"`

	// TopicID is the target topic (optional).
	TopicID TopicID `json:"topic_id,omitempty"`

	// DatasetID is the target dataset (optional).
	DatasetID DatasetID `json:"dataset_id,omitempty"`

	// Priority is the job priority (higher = more urgent).
	Priority int `json:"priority"`

	// ResourceLimits define execution constraints.
	ResourceLimits *ResourceLimits `json:"resource_limits,omitempty"`

	// CreatedBy is the user who created this job.
	CreatedBy UserID `json:"created_by"`

	// CreatedAt is when the job was created.
	CreatedAt time.Time `json:"created_at"`

	// StartedAt is when the job started running.
	StartedAt *time.Time `json:"started_at,omitempty"`

	// CompletedAt is when the job completed.
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Error is the error message if failed.
	Error string `json:"error,omitempty"`

	// Result is the job result (JSON encoded).
	Result string `json:"result,omitempty"`

	// Progress is the execution progress (0-100).
	Progress int `json:"progress"`

	// CurrentInstruction is the index of the currently executing instruction.
	CurrentInstruction int `json:"current_instruction,omitempty"`

	// NodeID is the node executing this job.
	NodeID string `json:"node_id,omitempty"`

	// RetryCount is the number of times this job has been retried.
	RetryCount int `json:"retry_count"`

	// Metadata holds additional job-specific data.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Validate validates the job.
func (j *Job) Validate() error {
	if j.ID == "" {
		return ErrInvalidJobID
	}
	if !j.Type.IsValid() {
		return ErrInvalidJobType
	}
	if j.TaskID == "" && len(j.InlineInstructions) == 0 {
		return ErrNoTaskOrInstructions
	}
	if j.ExecutionMode != "" && !j.ExecutionMode.IsValid() {
		return ErrInvalidExecutionMode
	}
	return nil
}

// IsComplete returns true if the job has finished (success or failure).
func (j *Job) IsComplete() bool {
	return j.Status.IsTerminal()
}

// CanRun checks if the job can run (all dependencies completed).
func (j *Job) CanRun(completedJobs map[JobID]bool) bool {
	if j.Status != JobStatusPending && j.Status != JobStatusQueued && j.Status != JobStatusWaiting {
		return false
	}
	for _, depID := range j.Dependencies {
		if !completedJobs[depID] {
			return false
		}
	}
	return true
}

// Duration returns the execution duration.
func (j *Job) Duration() time.Duration {
	if j.StartedAt == nil {
		return 0
	}
	if j.CompletedAt != nil {
		return j.CompletedAt.Sub(*j.StartedAt)
	}
	return time.Since(*j.StartedAt)
}

// JobRequest represents a request to create a job.
// Used in the /bib/jobs protocol.
type JobRequest struct {
	// Job is the job to create.
	Job *Job `json:"job"`

	// RequestID is a unique request identifier.
	RequestID string `json:"request_id"`

	// RequesterPeerID is the peer requesting the job.
	RequesterPeerID string `json:"requester_peer_id"`

	// RequesterUserID is the user requesting the job.
	RequesterUserID UserID `json:"requester_user_id"`
}

// JobResponse represents a response to a job request.
type JobResponse struct {
	// RequestID is the original request ID.
	RequestID string `json:"request_id"`

	// Accepted indicates if the job was accepted.
	Accepted bool `json:"accepted"`

	// JobID is the assigned job ID.
	JobID JobID `json:"job_id,omitempty"`

	// Error is the error message if not accepted.
	Error string `json:"error,omitempty"`
}

// Pipeline represents a collection of jobs with dependencies.
// Convenience type for creating complex workflows.
type Pipeline struct {
	// ID is the unique pipeline identifier.
	ID string `json:"id"`

	// Name is the pipeline name.
	Name string `json:"name"`

	// Description describes the pipeline.
	Description string `json:"description,omitempty"`

	// Jobs are the jobs in this pipeline.
	Jobs []*Job `json:"jobs"`

	// CreatedBy is the user who created this pipeline.
	CreatedBy UserID `json:"created_by"`

	// CreatedAt is when the pipeline was created.
	CreatedAt time.Time `json:"created_at"`

	// Status is the overall pipeline status.
	Status JobStatus `json:"status"`
}

// Validate validates the pipeline.
func (p *Pipeline) Validate() error {
	if p.ID == "" {
		return ErrInvalidPipelineID
	}
	if p.Name == "" {
		return ErrInvalidPipelineName
	}
	if len(p.Jobs) == 0 {
		return ErrEmptyPipeline
	}
	// Validate all jobs
	for _, job := range p.Jobs {
		if err := job.Validate(); err != nil {
			return err
		}
	}
	// Validate DAG (no cycles)
	if err := p.validateDAG(); err != nil {
		return err
	}
	return nil
}

// validateDAG checks for cycles in job dependencies.
func (p *Pipeline) validateDAG() error {
	// Build job map
	jobMap := make(map[JobID]*Job)
	for _, job := range p.Jobs {
		jobMap[job.ID] = job
	}

	// Check for cycles using DFS
	visited := make(map[JobID]bool)
	recStack := make(map[JobID]bool)

	var hasCycle func(jobID JobID) bool
	hasCycle = func(jobID JobID) bool {
		visited[jobID] = true
		recStack[jobID] = true

		job := jobMap[jobID]
		if job == nil {
			return false
		}

		for _, depID := range job.Dependencies {
			if !visited[depID] {
				if hasCycle(depID) {
					return true
				}
			} else if recStack[depID] {
				return true
			}
		}

		recStack[jobID] = false
		return false
	}

	for _, job := range p.Jobs {
		if !visited[job.ID] {
			if hasCycle(job.ID) {
				return ErrCyclicDependency
			}
		}
	}

	return nil
}
