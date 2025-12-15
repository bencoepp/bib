package p2p

import (
	"time"
)

// TODO: These types are placeholders that will be fully implemented in Phase 2 (Storage Layer).
// They provide the minimal interface needed for P2P sync operations.

// Topic represents a category of datasets.
type Topic struct {
	// ID is the unique identifier for the topic.
	ID string `json:"id"`

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

// Dataset represents a unit of data within a topic.
type Dataset struct {
	// ID is the unique identifier for the dataset.
	ID string `json:"id"`

	// TopicID is the topic this dataset belongs to.
	TopicID string `json:"topic_id"`

	// Name is the human-readable name.
	Name string `json:"name"`

	// Size is the size in bytes.
	Size int64 `json:"size"`

	// Hash is the content hash for integrity verification.
	Hash string `json:"hash"`

	// CreatedAt is when the dataset was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the dataset was last modified.
	UpdatedAt time.Time `json:"updated_at"`

	// Metadata holds additional key-value pairs.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// CatalogEntry represents a lightweight entry in a peer's catalog.
// Used for discovery without transferring full dataset content.
type CatalogEntry struct {
	// TopicID is the topic ID.
	TopicID string `json:"topic_id"`

	// TopicName is the topic name.
	TopicName string `json:"topic_name"`

	// DatasetID is the dataset ID.
	DatasetID string `json:"dataset_id"`

	// DatasetName is the dataset name.
	DatasetName string `json:"dataset_name"`

	// Hash is the content hash.
	Hash string `json:"hash"`

	// Size is the size in bytes.
	Size int64 `json:"size"`

	// UpdatedAt is when this entry was last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// Catalog represents a peer's available data.
type Catalog struct {
	// PeerID is the peer that owns this catalog.
	PeerID string `json:"peer_id"`

	// Entries is the list of available data.
	Entries []CatalogEntry `json:"entries"`

	// LastUpdated is when this catalog was last refreshed.
	LastUpdated time.Time `json:"last_updated"`
}

// QueryRequest represents a data query.
type QueryRequest struct {
	// ID is a unique identifier for this query.
	ID string `json:"id"`

	// TopicID filters by topic (optional).
	TopicID string `json:"topic_id,omitempty"`

	// DatasetID filters by dataset (optional).
	DatasetID string `json:"dataset_id,omitempty"`

	// Expression is a CEL expression for filtering (optional).
	// TODO: Will be implemented in Phase 3 (Scheduler).
	Expression string `json:"expression,omitempty"`

	// Limit is the maximum number of results.
	Limit int `json:"limit,omitempty"`

	// Offset is the starting position for pagination.
	Offset int `json:"offset,omitempty"`
}

// QueryResult represents the result of a query.
type QueryResult struct {
	// QueryID is the original query ID.
	QueryID string `json:"query_id"`

	// Entries are the matching catalog entries.
	Entries []CatalogEntry `json:"entries"`

	// TotalCount is the total number of matches (before pagination).
	TotalCount int `json:"total_count"`

	// FromCache indicates if this result was served from cache.
	FromCache bool `json:"from_cache"`

	// SourcePeer is the peer that provided this result.
	SourcePeer string `json:"source_peer,omitempty"`
}

// SyncStatus represents the synchronization status.
type SyncStatus struct {
	// InProgress indicates if a sync is currently running.
	InProgress bool `json:"in_progress"`

	// LastSyncTime is when the last sync completed.
	LastSyncTime time.Time `json:"last_sync_time"`

	// LastSyncError is the error from the last sync, if any.
	LastSyncError string `json:"last_sync_error,omitempty"`

	// PendingEntries is the number of entries waiting to be synced.
	PendingEntries int `json:"pending_entries"`

	// SyncedEntries is the total number of synced entries.
	SyncedEntries int `json:"synced_entries"`
}

// Subscription represents a topic subscription for selective mode.
type Subscription struct {
	// TopicPattern is a pattern to match topic names or IDs.
	// Supports wildcards: "*" matches any sequence, "?" matches single char.
	TopicPattern string `json:"topic_pattern"`

	// CreatedAt is when the subscription was created.
	CreatedAt time.Time `json:"created_at"`

	// LastSync is when this subscription was last synced.
	LastSync time.Time `json:"last_sync"`
}
