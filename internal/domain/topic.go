// Package domain contains core domain entities for bib.
// These types are used across all layers (P2P, storage, gRPC, etc.).
package domain

import (
	"time"
)

// TopicID is a unique identifier for a topic.
type TopicID string

// String returns the string representation.
func (id TopicID) String() string {
	return string(id)
}

// Topic represents a category of datasets.
type Topic struct {
	// ID is the unique identifier for the topic.
	ID TopicID `json:"id"`

	// Name is the human-readable name.
	Name string `json:"name"`

	// Description provides details about the topic.
	Description string `json:"description"`

	// Schema defines the expected data structure (optional).
	Schema string `json:"schema,omitempty"`

	// CreatedAt is when the topic was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the topic was last modified.
	UpdatedAt time.Time `json:"updated_at"`

	// DatasetCount is the number of datasets in this topic.
	DatasetCount int `json:"dataset_count"`
}

// Validate validates the topic.
func (t *Topic) Validate() error {
	if t.ID == "" {
		return ErrInvalidTopicID
	}
	if t.Name == "" {
		return ErrInvalidTopicName
	}
	return nil
}
