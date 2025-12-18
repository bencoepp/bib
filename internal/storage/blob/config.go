package blob

import "bib/internal/storage"

// Type aliases to use storage package config types
type (
	Config            = storage.BlobConfig
	LocalConfig       = storage.BlobLocalConfig
	S3Config          = storage.BlobS3Config
	EncryptionConfig  = storage.BlobEncryptionConfig
	CompressionConfig = storage.BlobCompressionConfig
	TieringConfig     = storage.BlobTieringConfig
	GCConfig          = storage.BlobGCConfig
	AuditConfig       = storage.BlobAuditConfig
)
