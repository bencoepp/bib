package blob

import (
	"context"
	"io"
	"time"
)

// Store is the main blob storage interface supporting multiple backends.
type Store interface {
	io.Closer

	// Put stores a blob with the given hash and data.
	// Returns error if blob already exists or storage fails.
	Put(ctx context.Context, hash string, data io.Reader, metadata *Metadata) error

	// Get retrieves a blob by hash.
	// Returns io.ReadCloser that must be closed by caller.
	Get(ctx context.Context, hash string) (io.ReadCloser, error)

	// Delete removes a blob by hash.
	// May move to trash instead of permanent deletion based on config.
	Delete(ctx context.Context, hash string) error

	// Exists checks if a blob exists.
	Exists(ctx context.Context, hash string) (bool, error)

	// Size returns the size of a blob in bytes.
	Size(ctx context.Context, hash string) (int64, error)

	// List lists blobs with the given prefix.
	// Used primarily for garbage collection scanning.
	List(ctx context.Context, prefix string) ([]BlobInfo, error)

	// Touch updates the last accessed time for LRU tracking.
	Touch(ctx context.Context, hash string) error

	// GetMetadata retrieves blob metadata without reading the data.
	GetMetadata(ctx context.Context, hash string) (*Metadata, error)

	// UpdateMetadata updates blob metadata.
	UpdateMetadata(ctx context.Context, hash string, meta *Metadata) error

	// Move moves a blob to another store (for tiering).
	Move(ctx context.Context, hash string, to Store) error

	// Copy copies a blob to another store (for replication).
	Copy(ctx context.Context, hash string, to Store) error

	// Backend returns the storage backend type.
	Backend() BackendType

	// Stats returns storage statistics.
	Stats(ctx context.Context) (*Stats, error)
}

// BackendType represents the storage backend.
type BackendType string

const (
	BackendLocal BackendType = "local"
	BackendS3    BackendType = "s3"
)

// String returns the backend name.
func (b BackendType) String() string {
	return string(b)
}

// Metadata holds blob metadata stored alongside the blob.
type Metadata struct {
	// Hash is the SHA-256 hash of the blob content.
	Hash string `json:"hash"`

	// Size is the size of the blob in bytes.
	Size int64 `json:"size"`

	// CreatedAt is when the blob was first stored.
	CreatedAt time.Time `json:"created_at"`

	// LastAccessed is when the blob was last accessed (for LRU).
	LastAccessed time.Time `json:"last_accessed"`

	// AccessCount tracks how many times the blob has been accessed.
	AccessCount int64 `json:"access_count"`

	// References tracks which datasets/chunks reference this blob.
	References []Reference `json:"references,omitempty"`

	// Compression indicates if and how the blob is compressed.
	Compression CompressionType `json:"compression"`

	// Encryption indicates if and how the blob is encrypted.
	Encryption EncryptionType `json:"encryption"`

	// Tags are user-defined tags for categorization (hot, cold, important, etc.).
	Tags []string `json:"tags,omitempty"`

	// OriginalHash is the hash before encryption (for verification).
	OriginalHash string `json:"original_hash,omitempty"`
}

// Reference tracks a database reference to a blob.
type Reference struct {
	DatasetID  string `json:"dataset_id"`
	VersionID  string `json:"version_id"`
	ChunkIndex int    `json:"chunk_index"`
}

// CompressionType represents compression algorithm.
type CompressionType string

const (
	CompressionNone CompressionType = "none"
	CompressionGzip CompressionType = "gzip"
	CompressionZstd CompressionType = "zstd"
)

// EncryptionType represents encryption algorithm.
type EncryptionType string

const (
	EncryptionNone      EncryptionType = "none"
	EncryptionAES256GCM EncryptionType = "aes256-gcm"
)

// BlobInfo contains basic information about a blob.
type BlobInfo struct {
	Hash         string
	Size         int64
	CreatedAt    time.Time
	LastAccessed time.Time
	Tags         []string
}

// Stats holds storage statistics.
type Stats struct {
	// TotalBlobs is the total number of blobs.
	TotalBlobs int64

	// TotalSize is the total size in bytes.
	TotalSize int64

	// TotalSizeCompressed is the size after compression.
	TotalSizeCompressed int64

	// OldestBlob is the creation time of the oldest blob.
	OldestBlob time.Time

	// NewestBlob is the creation time of the newest blob.
	NewestBlob time.Time

	// Backend is the storage backend type.
	Backend BackendType
}
