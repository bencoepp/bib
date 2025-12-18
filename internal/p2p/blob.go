package p2p

import (
	"bytes"
	"context"

	"bib/internal/domain"
	"bib/internal/logger"
	"bib/internal/storage/blob"
)

// BlobIntegration provides integration between P2P transfers and blob storage.
type BlobIntegration struct {
	ingestion *blob.Ingestion
	logger    *logger.Logger
}

// NewBlobIntegration creates a new P2P-to-blob integration helper.
func NewBlobIntegration(blobManager *blob.Manager, log *logger.Logger) *BlobIntegration {
	return &BlobIntegration{
		ingestion: blobManager.Ingestion(),
		logger:    log,
	}
}

// SetupCallbacks configures the transfer manager to use blob storage.
func (bi *BlobIntegration) SetupCallbacks(tm *TransferManager) {
	// Set chunk callback to store in blob storage
	tm.SetChunkCallback(func(download *domain.Download, chunk *domain.Chunk) {
		if err := bi.handleChunk(download, chunk); err != nil {
			bi.logger.Error("failed to store chunk in blob storage",
				"download_id", download.ID,
				"chunk_index", chunk.Index,
				"hash", chunk.Hash,
				"error", err,
			)
		}
	})

	// Set completion callback
	tm.SetCompleteCallback(func(download *domain.Download) {
		bi.logger.Info("download completed",
			"download_id", download.ID,
			"dataset_id", download.DatasetID,
			"total_chunks", download.TotalChunks,
		)
	})

	// Set error callback
	tm.SetErrorCallback(func(download *domain.Download, err error) {
		bi.logger.Error("download failed",
			"download_id", download.ID,
			"dataset_id", download.DatasetID,
			"error", err,
		)
	})
}

// handleChunk stores a received chunk in blob storage.
func (bi *BlobIntegration) handleChunk(download *domain.Download, chunk *domain.Chunk) error {
	ctx := context.Background()

	bi.logger.Debug("storing chunk in blob storage",
		"download_id", download.ID,
		"chunk_index", chunk.Index,
		"hash", chunk.Hash,
		"size", len(chunk.Data),
	)

	// Ingest chunk into blob storage
	// This handles both blob storage and database metadata
	data := bytes.NewReader(chunk.Data)
	if err := bi.ingestion.IngestChunk(ctx, chunk, data); err != nil {
		return err
	}

	bi.logger.Debug("chunk stored successfully",
		"download_id", download.ID,
		"chunk_index", chunk.Index,
		"hash", chunk.Hash,
	)

	return nil
}

// VerifyDownload verifies the integrity of all chunks for a completed download.
func (bi *BlobIntegration) VerifyDownload(ctx context.Context, versionID domain.DatasetVersionID) error {
	bi.logger.Info("verifying download integrity", "version_id", versionID)

	if err := bi.ingestion.VerifyDatasetIntegrity(ctx, versionID); err != nil {
		bi.logger.Error("download integrity verification failed",
			"version_id", versionID,
			"error", err,
		)
		return err
	}

	bi.logger.Info("download integrity verified", "version_id", versionID)
	return nil
}
