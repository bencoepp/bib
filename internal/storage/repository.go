package storage

import (
	"context"
	"io"
	"time"

	"bib/internal/domain"
)

// Store is the main storage interface that provides access to all repositories.
// It abstracts the underlying database implementation (SQLite or PostgreSQL).
type Store interface {
	io.Closer

	// Topics returns the topic repository.
	Topics() TopicRepository

	// Datasets returns the dataset repository.
	Datasets() DatasetRepository

	// Jobs returns the job repository.
	Jobs() JobRepository

	// Nodes returns the node repository.
	Nodes() NodeRepository

	// Audit returns the audit repository.
	Audit() AuditRepository

	// Ping checks database connectivity.
	Ping(ctx context.Context) error

	// IsAuthoritative returns true if this store can be an authoritative data source.
	// SQLite stores return false; PostgreSQL stores return true.
	IsAuthoritative() bool

	// Backend returns the storage backend type.
	Backend() BackendType

	// Migrate runs database migrations.
	Migrate(ctx context.Context) error
}

// BackendType represents the storage backend.
type BackendType string

const (
	BackendSQLite   BackendType = "sqlite"
	BackendPostgres BackendType = "postgres"
)

// String returns the backend name.
func (b BackendType) String() string {
	return string(b)
}

// TopicRepository handles topic persistence.
type TopicRepository interface {
	// Create creates a new topic.
	Create(ctx context.Context, topic *domain.Topic) error

	// Get retrieves a topic by ID.
	Get(ctx context.Context, id domain.TopicID) (*domain.Topic, error)

	// GetByName retrieves a topic by name.
	GetByName(ctx context.Context, name string) (*domain.Topic, error)

	// List retrieves topics matching the filter.
	List(ctx context.Context, filter TopicFilter) ([]*domain.Topic, error)

	// Update updates an existing topic.
	Update(ctx context.Context, topic *domain.Topic) error

	// Delete deletes a topic (soft delete - sets status to deleted).
	Delete(ctx context.Context, id domain.TopicID) error

	// Count returns the number of topics matching the filter.
	Count(ctx context.Context, filter TopicFilter) (int64, error)
}

// TopicFilter defines filtering options for topic queries.
type TopicFilter struct {
	// Status filters by topic status (empty = all)
	Status domain.TopicStatus

	// ParentID filters by parent topic
	ParentID *domain.TopicID

	// OwnerID filters by owner
	OwnerID *domain.UserID

	// Tags filters by tags (AND logic)
	Tags []string

	// Search performs text search on name/description
	Search string

	// Limit is the maximum number of results
	Limit int

	// Offset is the number of results to skip
	Offset int

	// OrderBy is the field to order by
	OrderBy string

	// OrderDesc reverses the order
	OrderDesc bool
}

// DatasetRepository handles dataset persistence.
type DatasetRepository interface {
	// Create creates a new dataset.
	Create(ctx context.Context, dataset *domain.Dataset) error

	// Get retrieves a dataset by ID.
	Get(ctx context.Context, id domain.DatasetID) (*domain.Dataset, error)

	// List retrieves datasets matching the filter.
	List(ctx context.Context, filter DatasetFilter) ([]*domain.Dataset, error)

	// Update updates an existing dataset.
	Update(ctx context.Context, dataset *domain.Dataset) error

	// Delete deletes a dataset (soft delete).
	Delete(ctx context.Context, id domain.DatasetID) error

	// Count returns the number of datasets matching the filter.
	Count(ctx context.Context, filter DatasetFilter) (int64, error)

	// Versions

	// CreateVersion creates a new dataset version.
	CreateVersion(ctx context.Context, version *domain.DatasetVersion) error

	// GetVersion retrieves a specific version.
	GetVersion(ctx context.Context, datasetID domain.DatasetID, versionID domain.DatasetVersionID) (*domain.DatasetVersion, error)

	// GetLatestVersion retrieves the latest version of a dataset.
	GetLatestVersion(ctx context.Context, datasetID domain.DatasetID) (*domain.DatasetVersion, error)

	// ListVersions lists all versions of a dataset.
	ListVersions(ctx context.Context, datasetID domain.DatasetID) ([]*domain.DatasetVersion, error)

	// Chunks

	// CreateChunk creates a new chunk record.
	CreateChunk(ctx context.Context, chunk *domain.Chunk) error

	// GetChunk retrieves a chunk by dataset version and index.
	GetChunk(ctx context.Context, versionID domain.DatasetVersionID, index int) (*domain.Chunk, error)

	// ListChunks lists all chunks for a version.
	ListChunks(ctx context.Context, versionID domain.DatasetVersionID) ([]*domain.Chunk, error)

	// UpdateChunkStatus updates the status of a chunk (e.g., downloaded, verified).
	UpdateChunkStatus(ctx context.Context, chunkID domain.ChunkID, status domain.ChunkStatus) error
}

// DatasetFilter defines filtering options for dataset queries.
type DatasetFilter struct {
	// TopicID filters by topic
	TopicID *domain.TopicID

	// Status filters by dataset status
	Status domain.DatasetStatus

	// OwnerID filters by owner
	OwnerID *domain.UserID

	// HasContent filters by content presence
	HasContent *bool

	// HasInstructions filters by instruction presence
	HasInstructions *bool

	// Tags filters by tags
	Tags []string

	// Search performs text search
	Search string

	// Limit is the maximum number of results
	Limit int

	// Offset is the number of results to skip
	Offset int

	// OrderBy is the field to order by
	OrderBy string

	// OrderDesc reverses the order
	OrderDesc bool
}

// JobRepository handles job persistence.
type JobRepository interface {
	// Create creates a new job.
	Create(ctx context.Context, job *domain.Job) error

	// Get retrieves a job by ID.
	Get(ctx context.Context, id domain.JobID) (*domain.Job, error)

	// List retrieves jobs matching the filter.
	List(ctx context.Context, filter JobFilter) ([]*domain.Job, error)

	// Update updates an existing job.
	Update(ctx context.Context, job *domain.Job) error

	// UpdateStatus updates just the job status and related timestamps.
	UpdateStatus(ctx context.Context, id domain.JobID, status domain.JobStatus) error

	// Delete deletes a job.
	Delete(ctx context.Context, id domain.JobID) error

	// Count returns the number of jobs matching the filter.
	Count(ctx context.Context, filter JobFilter) (int64, error)

	// GetPending retrieves pending jobs ordered by priority.
	GetPending(ctx context.Context, limit int) ([]*domain.Job, error)

	// Results

	// CreateResult creates a job result.
	CreateResult(ctx context.Context, result *domain.JobResult) error

	// GetResult retrieves a job result by ID.
	GetResult(ctx context.Context, id string) (*domain.JobResult, error)

	// ListResults lists results for a job.
	ListResults(ctx context.Context, jobID domain.JobID) ([]*domain.JobResult, error)
}

// JobFilter defines filtering options for job queries.
type JobFilter struct {
	// Type filters by job type
	Type domain.JobType

	// Status filters by job status
	Status domain.JobStatus

	// CreatedBy filters by creator
	CreatedBy string

	// TopicID filters by target topic
	TopicID *domain.TopicID

	// DatasetID filters by target dataset
	DatasetID *domain.DatasetID

	// MinPriority filters by minimum priority
	MinPriority *int

	// CreatedAfter filters by creation time
	CreatedAfter *time.Time

	// CreatedBefore filters by creation time
	CreatedBefore *time.Time

	// Limit is the maximum number of results
	Limit int

	// Offset is the number of results to skip
	Offset int

	// OrderBy is the field to order by
	OrderBy string

	// OrderDesc reverses the order
	OrderDesc bool
}

// NodeRepository handles peer node persistence.
type NodeRepository interface {
	// Upsert creates or updates a node.
	Upsert(ctx context.Context, node *NodeInfo) error

	// Get retrieves a node by peer ID.
	Get(ctx context.Context, peerID string) (*NodeInfo, error)

	// List retrieves nodes matching the filter.
	List(ctx context.Context, filter NodeFilter) ([]*NodeInfo, error)

	// Delete removes a node.
	Delete(ctx context.Context, peerID string) error

	// UpdateLastSeen updates the last seen timestamp.
	UpdateLastSeen(ctx context.Context, peerID string) error

	// Count returns the number of nodes matching the filter.
	Count(ctx context.Context, filter NodeFilter) (int64, error)
}

// NodeInfo represents a peer node in the network.
type NodeInfo struct {
	// PeerID is the libp2p peer ID.
	PeerID string `json:"peer_id"`

	// Addresses are the multiaddrs for this peer.
	Addresses []string `json:"addresses"`

	// Mode is the node's operation mode.
	Mode string `json:"mode"`

	// StorageType is the storage backend (sqlite/postgres).
	StorageType string `json:"storage_type"`

	// TrustedStorage indicates if the node can be an authoritative source.
	TrustedStorage bool `json:"trusted_storage"`

	// LastSeen is when the node was last seen.
	LastSeen time.Time `json:"last_seen"`

	// Metadata holds additional node information.
	Metadata map[string]any `json:"metadata,omitempty"`

	// CreatedAt is when the node was first seen.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the node info was last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// NodeFilter defines filtering options for node queries.
type NodeFilter struct {
	// Mode filters by node mode
	Mode string

	// TrustedOnly filters to only trusted storage nodes
	TrustedOnly bool

	// SeenAfter filters by last seen time
	SeenAfter *time.Time

	// Limit is the maximum number of results
	Limit int

	// Offset is the number of results to skip
	Offset int
}

// AuditRepository handles audit log persistence.
type AuditRepository interface {
	// Log records an audit entry.
	Log(ctx context.Context, entry *AuditEntry) error

	// Query retrieves audit entries matching the filter.
	Query(ctx context.Context, filter AuditFilter) ([]*AuditEntry, error)

	// Count returns the number of entries matching the filter.
	Count(ctx context.Context, filter AuditFilter) (int64, error)

	// GetByOperationID retrieves all entries for an operation.
	GetByOperationID(ctx context.Context, operationID string) ([]*AuditEntry, error)

	// GetByJobID retrieves all entries for a job.
	GetByJobID(ctx context.Context, jobID string) ([]*AuditEntry, error)

	// Purge removes entries older than the retention period.
	Purge(ctx context.Context, before time.Time) (int64, error)

	// VerifyChain verifies the hash chain integrity.
	VerifyChain(ctx context.Context, from, to int64) (bool, error)

	// GetLastHash returns the hash of the last entry.
	GetLastHash(ctx context.Context) (string, error)
}

// AuditEntry represents a single audit log entry.
type AuditEntry struct {
	// ID is the unique entry ID.
	ID int64 `json:"id"`

	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`

	// NodeID is the node that generated the entry.
	NodeID string `json:"node_id"`

	// JobID is the associated job (optional).
	JobID string `json:"job_id,omitempty"`

	// OperationID is the operation identifier.
	OperationID string `json:"operation_id"`

	// RoleUsed is the database role used.
	RoleUsed string `json:"role_used"`

	// Action is the type of action (SELECT, INSERT, UPDATE, DELETE, DDL).
	Action string `json:"action"`

	// TableName is the affected table.
	TableName string `json:"table_name,omitempty"`

	// Query is the SQL query (with sensitive values redacted).
	Query string `json:"query,omitempty"`

	// QueryHash is a hash of the query for grouping.
	QueryHash string `json:"query_hash,omitempty"`

	// RowsAffected is the number of rows affected.
	RowsAffected int `json:"rows_affected"`

	// DurationMS is the execution time in milliseconds.
	DurationMS int `json:"duration_ms"`

	// SourceComponent is the component that initiated the operation.
	SourceComponent string `json:"source_component"`

	// Actor is the user/node that initiated the operation.
	Actor string `json:"actor,omitempty"`

	// Metadata holds additional context.
	Metadata map[string]any `json:"metadata,omitempty"`

	// PrevHash is the hash of the previous entry (for tamper detection).
	PrevHash string `json:"prev_hash,omitempty"`

	// EntryHash is the hash of this entry.
	EntryHash string `json:"entry_hash"`

	// Flags contains additional flags for this entry.
	Flags AuditEntryFlags `json:"flags,omitempty"`
}

// AuditEntryFlags contains additional flags for audit entries.
type AuditEntryFlags struct {
	// BreakGlass indicates this was a break-glass session operation.
	BreakGlass bool `json:"break_glass,omitempty"`

	// RateLimited indicates this operation triggered rate limiting.
	RateLimited bool `json:"rate_limited,omitempty"`

	// Suspicious indicates this operation matched a suspicious pattern.
	Suspicious bool `json:"suspicious,omitempty"`

	// AlertTriggered indicates an alert was triggered for this operation.
	AlertTriggered bool `json:"alert_triggered,omitempty"`
}

// AuditFilter defines filtering options for audit queries.
type AuditFilter struct {
	// NodeID filters by node
	NodeID string

	// JobID filters by job
	JobID string

	// OperationID filters by operation
	OperationID string

	// Action filters by action type
	Action string

	// TableName filters by table
	TableName string

	// RoleUsed filters by role
	RoleUsed string

	// Actor filters by actor
	Actor string

	// After filters by timestamp
	After *time.Time

	// Before filters by timestamp
	Before *time.Time

	// Suspicious filters for suspicious entries only
	Suspicious *bool

	// Limit is the maximum number of results
	Limit int

	// Offset is the number of results to skip
	Offset int
}
