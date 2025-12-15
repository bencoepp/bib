package domain

import (
	"time"
)

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

// DownloadStatus represents the status of a download.
type DownloadStatus string

const (
	DownloadStatusActive    DownloadStatus = "active"
	DownloadStatusPaused    DownloadStatus = "paused"
	DownloadStatusCompleted DownloadStatus = "completed"
	DownloadStatusFailed    DownloadStatus = "failed"
)

// Download represents a dataset download in progress.
type Download struct {
	// ID is a unique identifier for this download.
	ID string `json:"id"`

	// DatasetID is the dataset being downloaded.
	DatasetID DatasetID `json:"dataset_id"`

	// DatasetHash is the expected content hash.
	DatasetHash string `json:"dataset_hash"`

	// PeerID is the peer we're downloading from (may change for multi-peer).
	PeerID string `json:"peer_id"`

	// TotalChunks is the total number of chunks.
	TotalChunks int `json:"total_chunks"`

	// CompletedChunks is the number of completed chunks.
	CompletedChunks int `json:"completed_chunks"`

	// ChunkBitmap is a bitmap of which chunks are completed.
	ChunkBitmap []byte `json:"chunk_bitmap"`

	// Status is the download status.
	Status DownloadStatus `json:"status"`

	// StartedAt is when the download started.
	StartedAt time.Time `json:"started_at"`

	// UpdatedAt is when the download was last updated.
	UpdatedAt time.Time `json:"updated_at"`

	// Error is the error message if failed.
	Error string `json:"error,omitempty"`
}

// IsChunkCompleted returns true if the chunk at index is completed.
func (d *Download) IsChunkCompleted(index int) bool {
	if index < 0 || d.ChunkBitmap == nil {
		return false
	}
	byteIndex := index / 8
	bitIndex := uint(index % 8)
	if byteIndex >= len(d.ChunkBitmap) {
		return false
	}
	return (d.ChunkBitmap[byteIndex] & (1 << bitIndex)) != 0
}

// SetChunkCompleted marks a chunk as completed.
func (d *Download) SetChunkCompleted(index int) {
	if index < 0 {
		return
	}
	byteIndex := index / 8
	bitIndex := uint(index % 8)

	// Expand bitmap if needed
	for byteIndex >= len(d.ChunkBitmap) {
		d.ChunkBitmap = append(d.ChunkBitmap, 0)
	}

	d.ChunkBitmap[byteIndex] |= (1 << bitIndex)
	d.CompletedChunks = d.countCompletedChunks()
}

// countCompletedChunks counts the number of completed chunks.
func (d *Download) countCompletedChunks() int {
	count := 0
	for _, b := range d.ChunkBitmap {
		for i := 0; i < 8; i++ {
			if (b & (1 << uint(i))) != 0 {
				count++
			}
		}
	}
	// Cap at total chunks
	if count > d.TotalChunks {
		count = d.TotalChunks
	}
	return count
}

// Progress returns the download progress as a percentage (0-100).
func (d *Download) Progress() float64 {
	if d.TotalChunks == 0 {
		return 0
	}
	return float64(d.CompletedChunks) / float64(d.TotalChunks) * 100
}

// IsComplete returns true if all chunks are downloaded.
func (d *Download) IsComplete() bool {
	return d.CompletedChunks >= d.TotalChunks && d.TotalChunks > 0
}

// MissingChunks returns the indices of chunks that haven't been downloaded.
func (d *Download) MissingChunks() []int {
	var missing []int
	for i := 0; i < d.TotalChunks; i++ {
		if !d.IsChunkCompleted(i) {
			missing = append(missing, i)
		}
	}
	return missing
}
