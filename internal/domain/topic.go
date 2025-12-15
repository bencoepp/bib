// Package domain contains core domain entities for bib.
// These types are used across all layers (P2P, storage, gRPC, etc.).
package domain

import (
	"strings"
	"time"
)

// TopicID is a unique identifier for a topic.
type TopicID string

// String returns the string representation.
func (id TopicID) String() string {
	return string(id)
}

// TopicStatus represents the lifecycle status of a topic.
type TopicStatus string

const (
	TopicStatusActive   TopicStatus = "active"
	TopicStatusArchived TopicStatus = "archived"
	TopicStatusDeleted  TopicStatus = "deleted"
)

// IsValid checks if the status is valid.
func (s TopicStatus) IsValid() bool {
	switch s {
	case TopicStatusActive, TopicStatusArchived, TopicStatusDeleted:
		return true
	default:
		return false
	}
}

// Topic represents a category of datasets (e.g., "weather", "finance/stocks").
type Topic struct {
	// ID is the unique identifier for the topic.
	ID TopicID `json:"id"`

	// ParentID is the parent topic ID for hierarchical topics (optional).
	// e.g., "weather/precipitation" has parent "weather"
	ParentID TopicID `json:"parent_id,omitempty"`

	// Name is the human-readable name.
	Name string `json:"name"`

	// Description provides details about the topic.
	Description string `json:"description"`

	// TableSchema defines the SQL DDL schema for datasets in this topic.
	// This describes the expected table structure for queryable data.
	TableSchema string `json:"table_schema,omitempty"`

	// Status is the lifecycle status of the topic.
	Status TopicStatus `json:"status"`

	// Owners are the user IDs who own this topic.
	Owners []UserID `json:"owners"`

	// CreatedBy is the user who created this topic.
	CreatedBy UserID `json:"created_by"`

	// CreatedAt is when the topic was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the topic was last modified.
	UpdatedAt time.Time `json:"updated_at"`

	// DatasetCount is the number of datasets in this topic.
	DatasetCount int `json:"dataset_count"`

	// Tags are optional labels for categorization and discovery.
	Tags []string `json:"tags,omitempty"`

	// Metadata holds additional key-value pairs.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Validate validates the topic.
func (t *Topic) Validate() error {
	if t.ID == "" {
		return ErrInvalidTopicID
	}
	if t.Name == "" {
		return ErrInvalidTopicName
	}
	if t.Status != "" && !t.Status.IsValid() {
		return ErrInvalidTopicStatus
	}
	if len(t.Owners) == 0 {
		return ErrNoOwners
	}
	return nil
}

// Path returns the full hierarchical path of the topic.
// For a topic with ID "precipitation" and parent "weather",
// this would need to be resolved through a topic lookup.
// This method returns the topic's own ID segment.
func (t *Topic) Path() string {
	return string(t.ID)
}

// IsHierarchical returns true if this topic has a parent.
func (t *Topic) IsHierarchical() bool {
	return t.ParentID != ""
}

// IsActive returns true if the topic is active.
func (t *Topic) IsActive() bool {
	return t.Status == TopicStatusActive
}

// IsOwner checks if the given user is an owner of this topic.
func (t *Topic) IsOwner(userID UserID) bool {
	for _, owner := range t.Owners {
		if owner == userID {
			return true
		}
	}
	return false
}

// AddOwner adds a new owner to the topic.
func (t *Topic) AddOwner(userID UserID) {
	if !t.IsOwner(userID) {
		t.Owners = append(t.Owners, userID)
		t.UpdatedAt = time.Now()
	}
}

// RemoveOwner removes an owner from the topic.
// Returns error if trying to remove the last owner.
func (t *Topic) RemoveOwner(userID UserID) error {
	if len(t.Owners) <= 1 {
		return ErrCannotRemoveLastOwner
	}
	for i, owner := range t.Owners {
		if owner == userID {
			t.Owners = append(t.Owners[:i], t.Owners[i+1:]...)
			t.UpdatedAt = time.Now()
			return nil
		}
	}
	return ErrOwnerNotFound
}

// TopicTree represents a hierarchical view of topics.
type TopicTree struct {
	// Topic is the topic at this node.
	Topic *Topic `json:"topic"`

	// Children are the child topics.
	Children []*TopicTree `json:"children,omitempty"`
}

// ParseTopicPath splits a topic path into segments.
// e.g., "weather/precipitation/rainfall" -> ["weather", "precipitation", "rainfall"]
func ParseTopicPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}
