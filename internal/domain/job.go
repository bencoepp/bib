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
	JobTypeCustom    JobType = "custom"
)

// JobStatus represents the status of a job.
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusQueued    JobStatus = "queued"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// Job represents a scheduled job.
// TODO: This is a placeholder for Phase 3 (Scheduler).
type Job struct {
	// ID is the unique identifier.
	ID JobID `json:"id"`

	// Type is the job type.
	Type JobType `json:"type"`

	// Status is the current status.
	Status JobStatus `json:"status"`

	// Expression is the CEL expression to execute.
	Expression string `json:"expression"`

	// TopicID is the target topic (optional).
	TopicID TopicID `json:"topic_id,omitempty"`

	// DatasetID is the target dataset (optional).
	DatasetID DatasetID `json:"dataset_id,omitempty"`

	// Priority is the job priority (higher = more urgent).
	Priority int `json:"priority"`

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

	// NodeID is the node executing this job.
	NodeID string `json:"node_id,omitempty"`

	// Metadata holds additional job-specific data.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Validate validates the job.
func (j *Job) Validate() error {
	if j.ID == "" {
		return ErrInvalidJobID
	}
	if j.Type == "" {
		return ErrInvalidJobType
	}
	return nil
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
