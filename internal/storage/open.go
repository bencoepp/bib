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
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid storage config: %w", err)
	}

	// Validate mode vs backend compatibility
	if err := ValidateModeBackend(nodeMode, cfg.Backend); err != nil {
		return nil, err
	}

	switch cfg.Backend {
	case BackendSQLite:
		if OpenSQLite == nil {
			return nil, fmt.Errorf("SQLite backend not available; import bib/internal/storage/sqlite")
		}
		store, err := OpenSQLite(ctx, cfg.SQLite, dataDir, nodeID)
		if err != nil {
			return nil, fmt.Errorf("failed to create SQLite store: %w", err)
		}
		if err := store.Migrate(ctx); err != nil {
			store.Close()
			return nil, fmt.Errorf("failed to run SQLite migrations: %w", err)
		}
		return store, nil

	case BackendPostgres:
		if OpenPostgres == nil {
			return nil, fmt.Errorf("PostgreSQL backend not available; import bib/internal/storage/postgres")
		}
		store, err := OpenPostgres(ctx, cfg.Postgres, dataDir, nodeID)
		if err != nil {
			return nil, fmt.Errorf("failed to create PostgreSQL store: %w", err)
		}
		if err := store.Migrate(ctx); err != nil {
			store.Close()
			return nil, fmt.Errorf("failed to run PostgreSQL migrations: %w", err)
		}
		return store, nil

	default:
		return nil, fmt.Errorf("unknown storage backend: %s", cfg.Backend)
	}
}
