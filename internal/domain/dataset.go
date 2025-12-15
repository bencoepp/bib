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

// Dataset represents a unit of data within a topic.
type Dataset struct {
	// ID is the unique identifier for the dataset.
	ID DatasetID `json:"id"`

	// TopicID is the topic this dataset belongs to.
	TopicID TopicID `json:"topic_id"`

	// Name is the human-readable name.
	Name string `json:"name"`

	// Size is the size in bytes.
	Size int64 `json:"size"`

	// Hash is the content hash (SHA-256) for integrity verification.
	Hash string `json:"hash"`

	// ChunkCount is the number of chunks this dataset is split into.
	ChunkCount int `json:"chunk_count"`

	// ChunkSize is the size of each chunk in bytes.
	ChunkSize int64 `json:"chunk_size"`

	// CreatedAt is when the dataset was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the dataset was last modified.
	UpdatedAt time.Time `json:"updated_at"`

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
	if d.Hash == "" {
		return ErrInvalidHash
	}
	return nil
}

// Chunk represents a piece of a dataset for chunked transfer.
type Chunk struct {
	// DatasetID is the dataset this chunk belongs to.
	DatasetID DatasetID `json:"dataset_id"`

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
