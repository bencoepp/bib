package storage

import (
	"context"
	"fmt"
)

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
		if err := store.Migrate(ctx); err != nil {
			store.Close()
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
		if err := store.Migrate(ctx); err != nil {
			store.Close()
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
