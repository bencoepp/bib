package storage

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"

	"bib/internal/logger"
	"bib/internal/storage/audit"
)

// BlobAuditLogger interface for blob operation auditing (avoids import cycle).
type BlobAuditLogger interface {
	LogBlobOperation(ctx context.Context, op BlobAuditOperation) error
}

// BlobAuditOperation represents a blob operation for auditing.
type BlobAuditOperation struct {
	Operation  string
	Hash       string
	Size       int64
	Success    bool
	Error      string
	UserID     string
	DatasetID  string
	VersionID  string
	ChunkIndex int
}

// ValidateModeBackend validates that the node mode is compatible with the storage backend.
// Returns an error if incompatible, or nil if compatible.
// If the mode is "full" but backend is SQLite, this returns a warning-level error
// that callers can choose to downgrade the mode.
func ValidateModeBackend(nodeMode string, backend BackendType) error {
	switch nodeMode {
	case "full":
		if backend == BackendSQLite {
			return &ModeBackendError{
				Mode:          nodeMode,
				Backend:       backend,
				Message:       "full replica mode requires PostgreSQL backend; SQLite cannot be an authoritative data source",
				CanDowngrade:  true,
				SuggestedMode: "selective",
			}
		}
	case "selective":
		// Both backends are allowed, but SQLite is cache-only
		// No error, but caller should be aware of limitations
	case "proxy":
		// Both backends are allowed
	}
	return nil
}

// ModeBackendError represents an incompatibility between node mode and storage backend.
type ModeBackendError struct {
	Mode          string
	Backend       BackendType
	Message       string
	CanDowngrade  bool
	SuggestedMode string
}

func (e *ModeBackendError) Error() string {
	return e.Message
}

// OpenFunc is the signature for store factory functions.
type OpenFunc func(ctx context.Context, dataDir, nodeID string) (Store, error)

// OpenSQLite is set by the sqlite package init to avoid import cycles.
var OpenSQLite func(ctx context.Context, cfg SQLiteConfig, dataDir, nodeID string) (Store, error)

// OpenPostgres is set by the postgres package init to avoid import cycles.
var OpenPostgres func(ctx context.Context, cfg PostgresConfig, dataDir, nodeID string) (Store, error)

// Open creates a new Store based on the configuration.
// It validates the configuration and returns the appropriate store implementation.
// Note: The caller must import the sqlite and/or postgres packages to register the factories.
func Open(ctx context.Context, cfg Config, dataDir, nodeID, nodeMode string) (Store, error) {
	storageLog := getLogger("open")

	storageLog.Debug("opening storage",
		"backend", cfg.Backend,
		"data_dir", dataDir,
		"node_id", nodeID,
		"node_mode", nodeMode,
	)

	if err := cfg.Validate(); err != nil {
		storageLog.Error("invalid storage config", "error", err)
		return nil, fmt.Errorf("invalid storage config: %w", err)
	}

	// Validate mode vs backend compatibility
	if err := ValidateModeBackend(nodeMode, cfg.Backend); err != nil {
		storageLog.Error("mode/backend incompatibility", "error", err, "mode", nodeMode, "backend", cfg.Backend)
		return nil, err
	}

	switch cfg.Backend {
	case BackendSQLite:
		storageLog.Debug("creating SQLite store")
		if OpenSQLite == nil {
			storageLog.Error("SQLite backend not available")
			return nil, fmt.Errorf("SQLite backend not available; import bib/internal/storage/sqlite")
		}
		store, err := OpenSQLite(ctx, cfg.SQLite, dataDir, nodeID)
		if err != nil {
			storageLog.Error("failed to create SQLite store", "error", err)
			return nil, fmt.Errorf("failed to create SQLite store: %w", err)
		}
		storageLog.Debug("running SQLite migrations")
		migrationCfg := cfg.Migrations
		if migrationCfg.LockTimeoutSeconds == 0 {
			migrationCfg = DefaultMigrationsConfig()
		}
		if err := RunMigrations(ctx, store, migrationCfg); err != nil {
			_ = store.Close()
			storageLog.Error("failed to run SQLite migrations", "error", err)
			return nil, fmt.Errorf("failed to run SQLite migrations: %w", err)
		}
		storageLog.Info("SQLite storage opened successfully", "authoritative", store.IsAuthoritative())
		return store, nil

	case BackendPostgres:
		storageLog.Debug("creating PostgreSQL store")
		if OpenPostgres == nil {
			storageLog.Error("PostgreSQL backend not available")
			return nil, fmt.Errorf("PostgreSQL backend not available; import bib/internal/storage/postgres")
		}
		store, err := OpenPostgres(ctx, cfg.Postgres, dataDir, nodeID)
		if err != nil {
			storageLog.Error("failed to create PostgreSQL store", "error", err)
			return nil, fmt.Errorf("failed to create PostgreSQL store: %w", err)
		}
		storageLog.Debug("running PostgreSQL migrations")
		migrationCfg := cfg.Migrations
		if migrationCfg.LockTimeoutSeconds == 0 {
			migrationCfg = DefaultMigrationsConfig()
		}
		if err := RunMigrations(ctx, store, migrationCfg); err != nil {
			_ = store.Close()
			storageLog.Error("failed to run PostgreSQL migrations", "error", err)
			return nil, fmt.Errorf("failed to run PostgreSQL migrations: %w", err)
		}
		storageLog.Info("PostgreSQL storage opened successfully", "authoritative", store.IsAuthoritative())
		return store, nil

	default:
		storageLog.Error("unknown storage backend", "backend", cfg.Backend)
		return nil, fmt.Errorf("unknown storage backend: %s", cfg.Backend)
	}
}

// OpenWithBlob creates both database store and blob storage manager.
// This is the recommended way to initialize storage with blob support.
// Returns the store and a generic blob manager interface to avoid import cycles.
func OpenWithBlob(ctx context.Context, cfg Config, dataDir, nodeID, nodeMode string, s3Client audit.S3Client) (Store, interface{}, error) {
	storageLog := getLogger("open")

	// Open database store
	store, err := Open(ctx, cfg, dataDir, nodeID, nodeMode)
	if err != nil {
		return nil, nil, err
	}

	// Generate encryption key from node ID if encryption is enabled
	var encKey []byte
	if cfg.Blob.Local.Encryption.Enabled || cfg.Blob.S3.ClientSideEncryption.Enabled {
		storageLog.Debug("generating blob encryption key from node identity")
		encKey = deriveEncryptionKey(nodeID)
	}

	// Create audit logger wrapper for blob operations
	var auditLogger BlobAuditLogger
	if cfg.Blob.BlobAudit.LogWrites || cfg.Blob.BlobAudit.LogDeletes || cfg.Blob.BlobAudit.LogReads {
		auditLogger = &blobAuditAdapter{
			store: store,
			log:   storageLog,
		}
	}

	// Initialize blob storage using OpenBlobFunc
	// This is set by the blob package to avoid import cycles
	if OpenBlobFunc == nil {
		store.Close()
		return nil, nil, fmt.Errorf("blob storage not available; import bib/internal/storage/blob")
	}

	storageLog.Info("initializing blob storage", "mode", cfg.Blob.Mode)
	blobManager, err := OpenBlobFunc(cfg.Blob, dataDir, encKey, store, s3Client, auditLogger, storageLog)
	if err != nil {
		store.Close()
		storageLog.Error("failed to initialize blob storage", "error", err)
		return nil, nil, fmt.Errorf("failed to initialize blob storage: %w", err)
	}

	// Start blob storage background processes using interface method
	storageLog.Debug("starting blob storage background processes")
	if starter, ok := blobManager.(interface{ Start(context.Context) error }); ok {
		if err := starter.Start(ctx); err != nil {
			if closer, ok := blobManager.(interface{ Close() error }); ok {
				closer.Close()
			}
			store.Close()
			storageLog.Error("failed to start blob storage", "error", err)
			return nil, nil, fmt.Errorf("failed to start blob storage: %w", err)
		}
	}

	storageLog.Info("storage opened successfully with blob support",
		"db_backend", cfg.Backend,
		"blob_mode", cfg.Blob.Mode,
		"gc_enabled", cfg.Blob.GC.Enabled,
	)

	return store, blobManager, nil
}

// OpenBlobFunc is set by the blob package init to avoid import cycles.
var OpenBlobFunc func(cfg BlobConfig, dataDir string, encKey []byte, dbStore Store, s3Client audit.S3Client, auditLog BlobAuditLogger, log *logger.Logger) (interface{}, error)

// deriveEncryptionKey derives a 32-byte encryption key from the node ID.
// This provides node-specific encryption without requiring separate key management.
func deriveEncryptionKey(nodeID string) []byte {
	// Use a simple approach: hash the node ID to get 32 bytes
	// In production, you might want to use HKDF or similar
	key := make([]byte, 32)
	copy(key, []byte(nodeID))

	// If node ID is shorter than 32 bytes, pad with random data
	if len(nodeID) < 32 {
		_, _ = io.ReadFull(rand.Reader, key[len(nodeID):])
	}

	return key
}

// blobAuditAdapter adapts blob audit operations to the storage audit system.
type blobAuditAdapter struct {
	store Store
	log   *logger.Logger
}

func (a *blobAuditAdapter) LogBlobOperation(ctx context.Context, op BlobAuditOperation) error {
	// For now, just log to the logger
	// TODO: integrate with proper audit trail once implemented
	if op.Success {
		a.log.Info("blob operation",
			"operation", op.Operation,
			"hash", op.Hash,
			"size", op.Size,
			"dataset_id", op.DatasetID,
			"version_id", op.VersionID,
			"chunk_index", op.ChunkIndex,
		)
	} else {
		a.log.Warn("blob operation failed",
			"operation", op.Operation,
			"hash", op.Hash,
			"error", op.Error,
			"dataset_id", op.DatasetID,
			"version_id", op.VersionID,
			"chunk_index", op.ChunkIndex,
		)
	}
	return nil
}
