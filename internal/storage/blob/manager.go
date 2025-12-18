package blob

import (
	"context"
	"fmt"

	"bib/internal/logger"
	"bib/internal/storage"
	"bib/internal/storage/audit"
)

// Manager manages blob storage lifecycle and operations.
type Manager struct {
	cfg       Config
	store     Store
	gc        *GarbageCollector
	ingestion *Ingestion
	logger    *logger.Logger
}

// Open initializes blob storage based on configuration.
func Open(cfg Config, dataDir string, encKey []byte, dbStore storage.Store, s3Client audit.S3Client, auditLog AuditLogger, log *logger.Logger) (*Manager, error) {
	log.Info("Initializing blob storage", "mode", cfg.Mode)

	var blobStore Store
	var err error

	switch cfg.Mode {
	case "local":
		if !cfg.Local.Enabled {
			return nil, fmt.Errorf("local storage is disabled but mode is 'local'")
		}
		blobStore, err = NewLocalStore(cfg.Local, dataDir, encKey, log)
		if err != nil {
			return nil, fmt.Errorf("failed to create local blob store: %w", err)
		}

	case "s3":
		if !cfg.S3.Enabled {
			return nil, fmt.Errorf("S3 storage is disabled but mode is 's3'")
		}
		if s3Client == nil {
			return nil, fmt.Errorf("S3 client is required for S3 mode")
		}
		blobStore, err = NewS3Store(cfg.S3, s3Client, encKey, log)
		if err != nil {
			return nil, fmt.Errorf("failed to create S3 blob store: %w", err)
		}

	case "hybrid":
		if !cfg.Local.Enabled || !cfg.S3.Enabled {
			return nil, fmt.Errorf("both local and S3 storage must be enabled for hybrid mode")
		}
		if s3Client == nil {
			return nil, fmt.Errorf("S3 client is required for hybrid mode")
		}

		// Create hot (local) store
		hotStore, err := NewLocalStore(cfg.Local, dataDir, encKey, log)
		if err != nil {
			return nil, fmt.Errorf("failed to create local blob store: %w", err)
		}

		// Create cold (S3) store
		coldStore, err := NewS3Store(cfg.S3, s3Client, encKey, log)
		if err != nil {
			return nil, fmt.Errorf("failed to create S3 blob store: %w", err)
		}

		// Create hybrid store
		blobStore, err = NewHybridStore(cfg, hotStore, coldStore, log)
		if err != nil {
			return nil, fmt.Errorf("failed to create hybrid blob store: %w", err)
		}

	default:
		return nil, fmt.Errorf("unknown blob storage mode: %s", cfg.Mode)
	}

	// Create garbage collector
	gc := NewGarbageCollector(cfg.GC, blobStore, dbStore, log)

	// Create ingestion handler
	ingestion := NewIngestion(blobStore, dbStore, auditLog, log)

	manager := &Manager{
		cfg:       cfg,
		store:     blobStore,
		gc:        gc,
		ingestion: ingestion,
		logger:    log,
	}

	log.Info("Blob storage initialized",
		"mode", cfg.Mode,
		"gc_enabled", cfg.GC.Enabled,
		"gc_method", cfg.GC.Method,
	)

	return manager, nil
}

// Start starts background processes (GC, tiering, etc.).
func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("Starting blob storage background processes")

	// Start garbage collector
	if m.cfg.GC.Enabled {
		if err := m.gc.Start(ctx); err != nil {
			return fmt.Errorf("failed to start garbage collector: %w", err)
		}
	}

	// Start tiering for hybrid mode
	if m.cfg.Mode == "hybrid" && m.cfg.Tiering.Enabled {
		// TODO: Implement tiering scheduler
		m.logger.Info("Tiering enabled", "strategy", m.cfg.Tiering.Strategy)
	}

	return nil
}

// Stop stops background processes gracefully.
func (m *Manager) Stop() error {
	m.logger.Info("Stopping blob storage background processes")

	if m.gc != nil {
		if err := m.gc.Stop(); err != nil {
			m.logger.Warn("Failed to stop garbage collector", "error", err)
		}
	}

	return nil
}

// Close closes the blob store.
func (m *Manager) Close() error {
	m.logger.Info("Closing blob storage")

	if err := m.Stop(); err != nil {
		m.logger.Warn("Failed to stop background processes", "error", err)
	}

	if m.store != nil {
		if err := m.store.Close(); err != nil {
			return fmt.Errorf("failed to close blob store: %w", err)
		}
	}

	return nil
}

// Store returns the blob store instance.
func (m *Manager) Store() Store {
	return m.store
}

// GC returns the garbage collector instance.
func (m *Manager) GC() *GarbageCollector {
	return m.gc
}

// Ingestion returns the ingestion handler instance.
func (m *Manager) Ingestion() *Ingestion {
	return m.ingestion
}

// Stats returns blob storage statistics.
func (m *Manager) Stats(ctx context.Context) (*Stats, error) {
	return m.store.Stats(ctx)
}

// RunGC manually triggers a garbage collection cycle.
func (m *Manager) RunGC(ctx context.Context) error {
	return m.gc.Run(ctx)
}

// ApplyTieringPolicy manually applies tiering policy (for hybrid mode).
func (m *Manager) ApplyTieringPolicy(ctx context.Context) error {
	if m.cfg.Mode != "hybrid" {
		return fmt.Errorf("tiering is only available in hybrid mode")
	}

	hybridStore, ok := m.store.(*HybridStore)
	if !ok {
		return fmt.Errorf("store is not a hybrid store")
	}

	return hybridStore.ApplyTieringPolicy(ctx)
}
