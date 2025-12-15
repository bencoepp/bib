package domain

import (
	"time"
)

// CatalogEntry represents a lightweight entry in a peer's catalog.
// Used for discovery without transferring full dataset content.
type CatalogEntry struct {
	// TopicID is the topic ID.
	TopicID TopicID `json:"topic_id"`

	// TopicName is the topic name.
	TopicName string `json:"topic_name"`

	// DatasetID is the dataset ID.
	DatasetID DatasetID `json:"dataset_id"`

	// DatasetName is the dataset name.
	DatasetName string `json:"dataset_name"`

	// VersionID is the version ID.
	VersionID DatasetVersionID `json:"version_id"`

	// Version is the semantic version string.
	Version string `json:"version"`

	// Hash is the content hash.
	Hash string `json:"hash"`

	// Size is the size in bytes.
	Size int64 `json:"size"`

	// ChunkCount is the number of chunks.
	ChunkCount int `json:"chunk_count"`

	// HasContent indicates if the version has data content.
	HasContent bool `json:"has_content"`

	// HasInstructions indicates if the version has instructions.
	HasInstructions bool `json:"has_instructions"`

	// Owners are the user IDs who own this dataset.
	Owners []UserID `json:"owners,omitempty"`

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

	// Version is the catalog version for change detection.
	Version uint64 `json:"version"`
}

// HasEntry checks if the catalog has an entry with the given hash.
func (c *Catalog) HasEntry(hash string) bool {
	for _, e := range c.Entries {
		if e.Hash == hash {
			return true
		}
	}
	return false
}

// GetEntry returns the entry with the given dataset ID, or nil if not found.
func (c *Catalog) GetEntry(datasetID DatasetID) *CatalogEntry {
	for i, e := range c.Entries {
		if e.DatasetID == datasetID {
			return &c.Entries[i]
		}
	}
	return nil
}
