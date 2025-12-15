package domain

import "errors"

// Domain errors
var (
	// Topic errors
	ErrInvalidTopicID   = errors.New("invalid topic ID")
	ErrInvalidTopicName = errors.New("invalid topic name")
	ErrTopicNotFound    = errors.New("topic not found")

	// Dataset errors
	ErrInvalidDatasetID = errors.New("invalid dataset ID")
	ErrDatasetNotFound  = errors.New("dataset not found")
	ErrInvalidHash      = errors.New("invalid hash")
	ErrHashMismatch     = errors.New("hash mismatch")

	// Chunk errors
	ErrInvalidChunkIndex = errors.New("invalid chunk index")
	ErrChunkNotFound     = errors.New("chunk not found")

	// Job errors
	ErrInvalidJobID   = errors.New("invalid job ID")
	ErrInvalidJobType = errors.New("invalid job type")
	ErrJobNotFound    = errors.New("job not found")

	// Download errors
	ErrDownloadNotFound = errors.New("download not found")
	ErrDownloadFailed   = errors.New("download failed")

	// Catalog errors
	ErrCatalogNotFound = errors.New("catalog not found")
	ErrEntryNotFound   = errors.New("catalog entry not found")

	// Protocol errors
	ErrUnsupportedProtocol = errors.New("unsupported protocol version")
	ErrInvalidMessage      = errors.New("invalid protocol message")
	ErrTimeout             = errors.New("operation timed out")
)
