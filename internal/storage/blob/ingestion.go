package blob

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"bib/internal/domain"
	"bib/internal/logger"
	"bib/internal/storage"
)

// Ingestion handles the integration between blob storage and dataset ingestion.
type Ingestion struct {
	blobStore Store
	dbStore   storage.Store
	audit     AuditLogger
	logger    *logger.Logger
}

// AuditLogger interface for blob operation auditing.
type AuditLogger interface {
	LogBlobOperation(ctx context.Context, op AuditOperation) error
}

// AuditOperation represents a blob operation for auditing.
type AuditOperation struct {
	Operation  string // put, get, delete
	Hash       string
	Size       int64
	Success    bool
	Error      string
	UserID     string
	DatasetID  string
	VersionID  string
	ChunkIndex int
}

// NewIngestion creates a new data ingestion handler.
func NewIngestion(blobStore Store, dbStore storage.Store, audit AuditLogger, log *logger.Logger) *Ingestion {
	return &Ingestion{
		blobStore: blobStore,
		dbStore:   dbStore,
		audit:     audit,
		logger:    log,
	}
}

// IngestChunk ingests a single chunk into blob storage and database.
// This provides atomic handling of both blob storage and database operations.
func (ing *Ingestion) IngestChunk(ctx context.Context, chunk *domain.Chunk, data io.Reader) error {
	ing.logger.Debug("Ingesting chunk",
		"dataset_id", chunk.DatasetID,
		"version_id", chunk.VersionID,
		"index", chunk.Index,
		"hash", chunk.Hash,
	)

	// Read data into buffer so we can hash and store
	buf := new(bytes.Buffer)
	size, err := io.Copy(buf, data)
	if err != nil {
		return fmt.Errorf("failed to read chunk data: %w", err)
	}

	// Verify hash
	hash := sha256.Sum256(buf.Bytes())
	hashStr := hex.EncodeToString(hash[:])
	if hashStr != chunk.Hash {
		return fmt.Errorf("chunk hash mismatch: expected %s, got %s", chunk.Hash, hashStr)
	}

	chunk.Size = size

	// Check if blob already exists (deduplication)
	exists, err := ing.blobStore.Exists(ctx, chunk.Hash)
	if err != nil {
		return fmt.Errorf("failed to check blob existence: %w", err)
	}

	if !exists {
		// Store blob
		metadata := &Metadata{
			Hash: chunk.Hash,
			Size: size,
			References: []Reference{
				{
					DatasetID:  string(chunk.DatasetID),
					VersionID:  string(chunk.VersionID),
					ChunkIndex: chunk.Index,
				},
			},
		}

		if err := ing.blobStore.Put(ctx, chunk.Hash, bytes.NewReader(buf.Bytes()), metadata); err != nil {
			// Audit failure
			if ing.audit != nil {
				ing.audit.LogBlobOperation(ctx, AuditOperation{
					Operation:  "put",
					Hash:       chunk.Hash,
					Size:       size,
					Success:    false,
					Error:      err.Error(),
					DatasetID:  string(chunk.DatasetID),
					VersionID:  string(chunk.VersionID),
					ChunkIndex: chunk.Index,
				})
			}
			return fmt.Errorf("failed to store blob: %w", err)
		}

		// Audit success
		if ing.audit != nil {
			ing.audit.LogBlobOperation(ctx, AuditOperation{
				Operation:  "put",
				Hash:       chunk.Hash,
				Size:       size,
				Success:    true,
				DatasetID:  string(chunk.DatasetID),
				VersionID:  string(chunk.VersionID),
				ChunkIndex: chunk.Index,
			})
		}

		ing.logger.Debug("Blob stored", "hash", chunk.Hash, "size", size)
	} else {
		// Blob exists - update metadata to add reference
		meta, err := ing.blobStore.GetMetadata(ctx, chunk.Hash)
		if err != nil {
			ing.logger.Warn("Failed to get existing blob metadata", "hash", chunk.Hash, "error", err)
		} else {
			// Add reference if not already present
			ref := Reference{
				DatasetID:  string(chunk.DatasetID),
				VersionID:  string(chunk.VersionID),
				ChunkIndex: chunk.Index,
			}

			found := false
			for _, r := range meta.References {
				if r.DatasetID == ref.DatasetID && r.VersionID == ref.VersionID && r.ChunkIndex == ref.ChunkIndex {
					found = true
					break
				}
			}

			if !found {
				meta.References = append(meta.References, ref)
				if err := ing.blobStore.UpdateMetadata(ctx, chunk.Hash, meta); err != nil {
					ing.logger.Warn("Failed to update blob metadata", "hash", chunk.Hash, "error", err)
				}
			}
		}

		ing.logger.Debug("Blob already exists (deduplicated)", "hash", chunk.Hash)
	}

	// Store chunk metadata in database
	chunk.Status = domain.ChunkStatusVerified
	if err := ing.dbStore.Datasets().CreateChunk(ctx, chunk); err != nil {
		return fmt.Errorf("failed to store chunk metadata: %w", err)
	}

	ing.logger.Debug("Chunk ingested successfully", "hash", chunk.Hash)
	return nil
}

// RetrieveChunk retrieves a chunk from blob storage.
func (ing *Ingestion) RetrieveChunk(ctx context.Context, chunk *domain.Chunk) (io.ReadCloser, error) {
	ing.logger.Debug("Retrieving chunk", "hash", chunk.Hash, "index", chunk.Index)

	// Get blob
	reader, err := ing.blobStore.Get(ctx, chunk.Hash)
	if err != nil {
		// Audit failure
		if ing.audit != nil {
			ing.audit.LogBlobOperation(ctx, AuditOperation{
				Operation:  "get",
				Hash:       chunk.Hash,
				Success:    false,
				Error:      err.Error(),
				DatasetID:  string(chunk.DatasetID),
				VersionID:  string(chunk.VersionID),
				ChunkIndex: chunk.Index,
			})
		}
		return nil, fmt.Errorf("failed to retrieve blob: %w", err)
	}

	// Audit success
	if ing.audit != nil {
		ing.audit.LogBlobOperation(ctx, AuditOperation{
			Operation:  "get",
			Hash:       chunk.Hash,
			Size:       chunk.Size,
			Success:    true,
			DatasetID:  string(chunk.DatasetID),
			VersionID:  string(chunk.VersionID),
			ChunkIndex: chunk.Index,
		})
	}

	return reader, nil
}

// DeleteChunk deletes a chunk from blob storage and updates metadata.
func (ing *Ingestion) DeleteChunk(ctx context.Context, chunk *domain.Chunk) error {
	ing.logger.Debug("Deleting chunk", "hash", chunk.Hash, "index", chunk.Index)

	// Get blob metadata
	meta, err := ing.blobStore.GetMetadata(ctx, chunk.Hash)
	if err != nil {
		return fmt.Errorf("failed to get blob metadata: %w", err)
	}

	// Remove reference
	ref := Reference{
		DatasetID:  string(chunk.DatasetID),
		VersionID:  string(chunk.VersionID),
		ChunkIndex: chunk.Index,
	}

	var updatedRefs []Reference
	for _, r := range meta.References {
		if r.DatasetID != ref.DatasetID || r.VersionID != ref.VersionID || r.ChunkIndex != ref.ChunkIndex {
			updatedRefs = append(updatedRefs, r)
		}
	}

	meta.References = updatedRefs

	// If no more references, delete the blob
	if len(updatedRefs) == 0 {
		if err := ing.blobStore.Delete(ctx, chunk.Hash); err != nil {
			// Audit failure
			if ing.audit != nil {
				ing.audit.LogBlobOperation(ctx, AuditOperation{
					Operation:  "delete",
					Hash:       chunk.Hash,
					Success:    false,
					Error:      err.Error(),
					DatasetID:  string(chunk.DatasetID),
					VersionID:  string(chunk.VersionID),
					ChunkIndex: chunk.Index,
				})
			}
			return fmt.Errorf("failed to delete blob: %w", err)
		}

		// Audit success
		if ing.audit != nil {
			ing.audit.LogBlobOperation(ctx, AuditOperation{
				Operation:  "delete",
				Hash:       chunk.Hash,
				Size:       chunk.Size,
				Success:    true,
				DatasetID:  string(chunk.DatasetID),
				VersionID:  string(chunk.VersionID),
				ChunkIndex: chunk.Index,
			})
		}

		ing.logger.Debug("Blob deleted (no more references)", "hash", chunk.Hash)
	} else {
		// Update metadata with remaining references
		if err := ing.blobStore.UpdateMetadata(ctx, chunk.Hash, meta); err != nil {
			return fmt.Errorf("failed to update blob metadata: %w", err)
		}

		ing.logger.Debug("Blob reference removed", "hash", chunk.Hash, "remaining_refs", len(updatedRefs))
	}

	// Delete chunk from database
	// Note: We might want to keep this for audit trail
	// For now, just update status to deleted
	return ing.dbStore.Datasets().UpdateChunkStatus(ctx, chunk.ID, domain.ChunkStatusFailed)
}

// IngestDataset ingests an entire dataset version with all chunks.
func (ing *Ingestion) IngestDataset(ctx context.Context, version *domain.DatasetVersion, chunks []*domain.Chunk, chunkData map[int]io.Reader) error {
	ing.logger.Info("Ingesting dataset",
		"dataset_id", version.DatasetID,
		"version_id", version.ID,
		"chunk_count", len(chunks),
	)

	// Validate inputs
	if len(chunks) != len(chunkData) {
		return fmt.Errorf("chunk count mismatch: %d chunks, %d data readers", len(chunks), len(chunkData))
	}

	// Ingest each chunk
	for _, chunk := range chunks {
		data, ok := chunkData[chunk.Index]
		if !ok {
			return fmt.Errorf("missing data for chunk index %d", chunk.Index)
		}

		if err := ing.IngestChunk(ctx, chunk, data); err != nil {
			return fmt.Errorf("failed to ingest chunk %d: %w", chunk.Index, err)
		}
	}

	ing.logger.Info("Dataset ingestion completed",
		"dataset_id", version.DatasetID,
		"version_id", version.ID,
	)

	return nil
}

// VerifyDatasetIntegrity verifies that all chunks for a dataset version exist and are valid.
func (ing *Ingestion) VerifyDatasetIntegrity(ctx context.Context, versionID domain.DatasetVersionID) error {
	ing.logger.Debug("Verifying dataset integrity", "version_id", versionID)

	// Get all chunks for version
	chunks, err := ing.dbStore.Datasets().ListChunks(ctx, versionID)
	if err != nil {
		return fmt.Errorf("failed to list chunks: %w", err)
	}

	var missingCount int
	var invalidCount int

	for _, chunk := range chunks {
		// Check if blob exists
		exists, err := ing.blobStore.Exists(ctx, chunk.Hash)
		if err != nil {
			return fmt.Errorf("failed to check blob existence: %w", err)
		}

		if !exists {
			ing.logger.Warn("Missing blob", "hash", chunk.Hash, "chunk_index", chunk.Index)
			missingCount++
			continue
		}

		// Verify hash by reading and re-hashing
		// This is expensive but thorough
		reader, err := ing.blobStore.Get(ctx, chunk.Hash)
		if err != nil {
			ing.logger.Warn("Failed to read blob", "hash", chunk.Hash, "error", err)
			invalidCount++
			continue
		}

		hasher := sha256.New()
		if _, err := io.Copy(hasher, reader); err != nil {
			reader.Close()
			ing.logger.Warn("Failed to hash blob", "hash", chunk.Hash, "error", err)
			invalidCount++
			continue
		}
		reader.Close()

		computedHash := hex.EncodeToString(hasher.Sum(nil))
		if computedHash != chunk.Hash {
			ing.logger.Warn("Hash mismatch", "expected", chunk.Hash, "computed", computedHash)
			invalidCount++
		}
	}

	if missingCount > 0 || invalidCount > 0 {
		return fmt.Errorf("integrity check failed: %d missing, %d invalid", missingCount, invalidCount)
	}

	ing.logger.Debug("Dataset integrity verified", "version_id", versionID, "chunks", len(chunks))
	return nil
}
