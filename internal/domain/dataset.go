package domain

import (
	"time"
)

// DatasetID is a unique identifier for a dataset.
type DatasetID string

// String returns the string representation.
func (id DatasetID) String() string {
	return string(id)
}

// DatasetVersionID is a unique identifier for a dataset version.
type DatasetVersionID string

// String returns the string representation.
func (id DatasetVersionID) String() string {
	return string(id)
}

// DatasetStatus represents the status of a dataset.
type DatasetStatus string

const (
	DatasetStatusDraft     DatasetStatus = "draft"
	DatasetStatusActive    DatasetStatus = "active"
	DatasetStatusArchived  DatasetStatus = "archived"
	DatasetStatusDeleted   DatasetStatus = "deleted"
	DatasetStatusIngesting DatasetStatus = "ingesting"
	DatasetStatusFailed    DatasetStatus = "failed"
)

// IsValid checks if the status is valid.
func (s DatasetStatus) IsValid() bool {
	switch s {
	case DatasetStatusDraft, DatasetStatusActive, DatasetStatusArchived,
		DatasetStatusDeleted, DatasetStatusIngesting, DatasetStatusFailed:
		return true
	default:
		return false
	}
}

// Dataset represents a unit of data within a topic.
// A dataset can contain both data content and instructions for obtaining data.
type Dataset struct {
	// ID is the unique identifier for the dataset.
	ID DatasetID `json:"id"`

	// TopicID is the topic this dataset belongs to.
	TopicID TopicID `json:"topic_id"`

	// Name is the human-readable name.
	Name string `json:"name"`

	// Description provides details about the dataset.
	Description string `json:"description,omitempty"`

	// Status is the current status of the dataset.
	Status DatasetStatus `json:"status"`

	// LatestVersionID is the most recent version ID.
	LatestVersionID DatasetVersionID `json:"latest_version_id,omitempty"`

	// VersionCount is the total number of versions.
	VersionCount int `json:"version_count"`

	// HasContent indicates if the dataset has actual data content.
	HasContent bool `json:"has_content"`

	// HasInstructions indicates if the dataset has acquisition instructions.
	HasInstructions bool `json:"has_instructions"`

	// Owners are the user IDs who own this dataset.
	Owners []UserID `json:"owners"`

	// CreatedBy is the user who created this dataset.
	CreatedBy UserID `json:"created_by"`

	// CreatedAt is when the dataset was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the dataset was last modified.
	UpdatedAt time.Time `json:"updated_at"`

	// Tags are optional labels for categorization.
	Tags []string `json:"tags,omitempty"`

	// Metadata holds additional key-value pairs.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Validate validates the dataset.
func (d *Dataset) Validate() error {
	if d.ID == "" {
		return ErrInvalidDatasetID
	}
	if d.TopicID == "" {
		return ErrInvalidTopicID
	}
	if d.Name == "" {
		return ErrInvalidDatasetName
	}
	if d.Status != "" && !d.Status.IsValid() {
		return ErrInvalidDatasetStatus
	}
	if len(d.Owners) == 0 {
		return ErrNoOwners
	}
	return nil
}

// IsOwner checks if the given user is an owner of this dataset.
func (d *Dataset) IsOwner(userID UserID) bool {
	for _, owner := range d.Owners {
		if owner == userID {
			return true
		}
	}
	return false
}

// DatasetVersion represents a specific version of a dataset.
// Each version is immutable once created.
type DatasetVersion struct {
	// ID is the unique version identifier.
	ID DatasetVersionID `json:"id"`

	// DatasetID is the parent dataset.
	DatasetID DatasetID `json:"dataset_id"`

	// Version is the semantic version string (e.g., "1.0.0").
	Version string `json:"version"`

	// PreviousVersionID links to the previous version (for history chain).
	PreviousVersionID DatasetVersionID `json:"previous_version_id,omitempty"`

	// Content contains the actual data information (optional).
	Content *DatasetContent `json:"content,omitempty"`

	// Instructions contains data acquisition instructions (optional).
	Instructions *DatasetInstructions `json:"instructions,omitempty"`

	// TableSchema is the SQL DDL for this version's data structure.
	// May differ from topic schema for migrations.
	TableSchema string `json:"table_schema,omitempty"`

	// CreatedBy is the user who created this version.
	CreatedBy UserID `json:"created_by"`

	// CreatedAt is when this version was created.
	CreatedAt time.Time `json:"created_at"`

	// Message is an optional version message (like a commit message).
	Message string `json:"message,omitempty"`

	// Metadata holds version-specific metadata.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Validate validates the dataset version.
func (v *DatasetVersion) Validate() error {
	if v.ID == "" {
		return ErrInvalidVersionID
	}
	if v.DatasetID == "" {
		return ErrInvalidDatasetID
	}
	if v.Version == "" {
		return ErrInvalidVersionString
	}
	if v.Content == nil && v.Instructions == nil {
		return ErrEmptyVersion
	}
	return nil
}

// HasContent returns true if this version has data content.
func (v *DatasetVersion) HasContent() bool {
	return v.Content != nil
}

// HasInstructions returns true if this version has instructions.
func (v *DatasetVersion) HasInstructions() bool {
	return v.Instructions != nil
}

// DatasetContent represents the actual data content of a dataset version.
type DatasetContent struct {
	// Hash is the content hash (SHA-256) for integrity verification.
	Hash string `json:"hash"`

	// Size is the total size in bytes.
	Size int64 `json:"size"`

	// RowCount is the number of rows/records in the data.
	RowCount int64 `json:"row_count,omitempty"`

	// ChunkCount is the number of chunks this content is split into.
	ChunkCount int `json:"chunk_count"`

	// ChunkSize is the size of each chunk in bytes.
	ChunkSize int64 `json:"chunk_size"`

	// Format describes the internal storage format.
	Format string `json:"format,omitempty"`

	// Checksum is an additional checksum for validation.
	Checksum string `json:"checksum,omitempty"`

	// StoragePath is the internal storage location (not exposed externally).
	StoragePath string `json:"-"`
}

// Validate validates the dataset content.
func (c *DatasetContent) Validate() error {
	if c.Hash == "" {
		return ErrInvalidHash
	}
	if c.Size < 0 {
		return ErrInvalidSize
	}
	if c.ChunkCount < 0 {
		return ErrInvalidChunkCount
	}
	return nil
}

// DatasetInstructions contains CEL-based instructions for data acquisition.
type DatasetInstructions struct {
	// TaskID references a reusable Task definition (optional).
	// If set, uses the Task's instructions instead of inline Instructions.
	TaskID TaskID `json:"task_id,omitempty"`

	// Instructions are inline CEL instructions (used if TaskID is empty).
	Instructions []Instruction `json:"instructions,omitempty"`

	// InputVariables are the required input variables for the instructions.
	InputVariables map[string]string `json:"input_variables,omitempty"`

	// SourceMetadata describes the data source.
	SourceMetadata *SourceMetadata `json:"source_metadata,omitempty"`

	// LastExecutedAt is when the instructions were last run.
	LastExecutedAt *time.Time `json:"last_executed_at,omitempty"`

	// LastExecutionStatus is the status of the last execution.
	LastExecutionStatus string `json:"last_execution_status,omitempty"`

	// Schedule defines automatic re-execution schedule (optional).
	Schedule *Schedule `json:"schedule,omitempty"`
}

// Validate validates the dataset instructions.
func (i *DatasetInstructions) Validate() error {
	if i.TaskID == "" && len(i.Instructions) == 0 {
		return ErrNoInstructions
	}
	for _, inst := range i.Instructions {
		if err := inst.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// SourceMetadata describes the original data source.
type SourceMetadata struct {
	// Type is the source type (e.g., "http", "ftp", "s3", "file").
	Type string `json:"type"`

	// URL is the source URL or path.
	URL string `json:"url,omitempty"`

	// Description describes the source.
	Description string `json:"description,omitempty"`

	// License is the data license.
	License string `json:"license,omitempty"`

	// Attribution is the required attribution.
	Attribution string `json:"attribution,omitempty"`

	// LastChecked is when the source was last checked for updates.
	LastChecked *time.Time `json:"last_checked,omitempty"`

	// OriginalFormat is the format of the source data.
	OriginalFormat string `json:"original_format,omitempty"`

	// Metadata holds additional source-specific metadata.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Chunk represents a piece of a dataset for chunked transfer.
type Chunk struct {
	// DatasetID is the dataset this chunk belongs to.
	DatasetID DatasetID `json:"dataset_id"`

	// VersionID is the version this chunk belongs to.
	VersionID DatasetVersionID `json:"version_id"`

	// Index is the chunk index (0-based).
	Index int `json:"index"`

	// Hash is the chunk's content hash.
	Hash string `json:"hash"`

	// Size is the chunk size in bytes.
	Size int64 `json:"size"`

	// Data is the chunk content (only populated during transfer).
	Data []byte `json:"-"`
}

// Validate validates the chunk.
func (c *Chunk) Validate() error {
	if c.DatasetID == "" {
		return ErrInvalidDatasetID
	}
	if c.Index < 0 {
		return ErrInvalidChunkIndex
	}
	if c.Hash == "" {
		return ErrInvalidHash
	}
	return nil
}
