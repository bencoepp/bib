package blob

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"bib/internal/logger"
	"bib/internal/storage"
)

// GarbageCollector manages blob garbage collection.
type GarbageCollector struct {
	cfg       GCConfig
	store     Store
	dbStore   storage.Store
	logger    *logger.Logger
	scheduler *Scheduler
}

// NewGarbageCollector creates a new garbage collector.
func NewGarbageCollector(cfg GCConfig, blobStore Store, dbStore storage.Store, log *logger.Logger) *GarbageCollector {
	gc := &GarbageCollector{
		cfg:     cfg,
		store:   blobStore,
		dbStore: dbStore,
		logger:  log,
	}

	// Setup scheduler if enabled
	if cfg.Enabled && cfg.Schedule != "" {
		gc.scheduler = NewScheduler(cfg.Schedule, gc.Run, log)
	}

	return gc
}

// Start starts the garbage collector scheduler.
func (gc *GarbageCollector) Start(ctx context.Context) error {
	if gc.scheduler == nil {
		return nil
	}

	gc.logger.Info("Starting blob garbage collector", "schedule", gc.cfg.Schedule)
	return gc.scheduler.Start(ctx)
}

// Stop stops the garbage collector scheduler.
func (gc *GarbageCollector) Stop() error {
	if gc.scheduler == nil {
		return nil
	}

	gc.logger.Info("Stopping blob garbage collector")
	return gc.scheduler.Stop()
}

// Run executes a garbage collection cycle.
func (gc *GarbageCollector) Run(ctx context.Context) error {
	gc.logger.Info("Starting garbage collection cycle")
	startTime := time.Now()

	var stats GCStats

	switch gc.cfg.Method {
	case "mark-and-sweep":
		var err error
		stats, err = gc.runMarkAndSweep(ctx)
		if err != nil {
			return fmt.Errorf("mark-and-sweep failed: %w", err)
		}
	case "reference-counting":
		var err error
		stats, err = gc.runReferenceCounting(ctx)
		if err != nil {
			return fmt.Errorf("reference-counting failed: %w", err)
		}
	default:
		return fmt.Errorf("unknown GC method: %s", gc.cfg.Method)
	}

	duration := time.Since(startTime)
	gc.logger.Info("Garbage collection completed",
		"duration", duration,
		"scanned", stats.BlobsScanned,
		"marked", stats.BlobsMarked,
		"collected", stats.BlobsCollected,
		"freed_bytes", stats.BytesFreed,
	)

	return nil
}

// RunWithPressure runs GC if storage pressure threshold is exceeded.
func (gc *GarbageCollector) RunWithPressure(ctx context.Context) error {
	if gc.cfg.StoragePressureThreshold <= 0 {
		return nil
	}

	// Check storage pressure
	pressure, err := gc.getStoragePressure()
	if err != nil {
		return fmt.Errorf("failed to get storage pressure: %w", err)
	}

	if pressure < gc.cfg.StoragePressureThreshold {
		gc.logger.Debug("Storage pressure below threshold", "pressure", pressure, "threshold", gc.cfg.StoragePressureThreshold)
		return nil
	}

	gc.logger.Warn("Storage pressure threshold exceeded, running GC", "pressure", pressure)
	return gc.Run(ctx)
}

// GCStats holds garbage collection statistics.
type GCStats struct {
	BlobsScanned   int64
	BlobsMarked    int64
	BlobsCollected int64
	BytesFreed     int64
}

// runMarkAndSweep implements mark-and-sweep garbage collection.
func (gc *GarbageCollector) runMarkAndSweep(ctx context.Context) (GCStats, error) {
	var stats GCStats

	// Phase 1: Mark - scan database for all referenced blobs
	gc.logger.Info("GC Phase 1: Marking referenced blobs")
	referenced, err := gc.markReferencedBlobs(ctx)
	if err != nil {
		return stats, fmt.Errorf("mark phase failed: %w", err)
	}
	stats.BlobsMarked = int64(len(referenced))

	// Phase 2: Sweep - find and collect unreferenced blobs
	gc.logger.Info("GC Phase 2: Sweeping unreferenced blobs", "marked", stats.BlobsMarked)

	// List all blobs
	allBlobs, err := gc.store.List(ctx, "")
	if err != nil {
		return stats, fmt.Errorf("failed to list blobs: %w", err)
	}
	stats.BlobsScanned = int64(len(allBlobs))

	minAge := time.Now().UTC().AddDate(0, 0, -gc.cfg.MinAgeDays)

	for _, blob := range allBlobs {
		// Check if referenced
		if referenced[blob.Hash] {
			continue
		}

		// Check minimum age
		if blob.CreatedAt.After(minAge) {
			gc.logger.Debug("Skipping blob - too new", "hash", blob.Hash, "age_days", time.Since(blob.CreatedAt).Hours()/24)
			continue
		}

		// Collect this blob
		if err := gc.collectBlob(ctx, blob.Hash); err != nil {
			gc.logger.Warn("Failed to collect blob", "hash", blob.Hash, "error", err)
			continue
		}

		stats.BlobsCollected++
		stats.BytesFreed += blob.Size
	}

	// Phase 3: Clean up old trash
	if err := gc.cleanupTrash(ctx); err != nil {
		gc.logger.Warn("Failed to cleanup trash", "error", err)
	}

	return stats, nil
}

// markReferencedBlobs scans the database and marks all referenced blobs.
func (gc *GarbageCollector) markReferencedBlobs(ctx context.Context) (map[string]bool, error) {
	referenced := make(map[string]bool)

	// Query all chunks from database
	// Note: This is a simplified approach. In production, you'd want to stream this.
	datasets, err := gc.dbStore.Datasets().List(ctx, storage.DatasetFilter{})
	if err != nil {
		return nil, fmt.Errorf("failed to list datasets: %w", err)
	}

	for _, dataset := range datasets {
		// Get all versions
		versions, err := gc.dbStore.Datasets().ListVersions(ctx, dataset.ID)
		if err != nil {
			gc.logger.Warn("Failed to list versions", "dataset_id", dataset.ID, "error", err)
			continue
		}

		for _, version := range versions {
			// Get all chunks for this version
			chunks, err := gc.dbStore.Datasets().ListChunks(ctx, version.ID)
			if err != nil {
				gc.logger.Warn("Failed to list chunks", "version_id", version.ID, "error", err)
				continue
			}

			for _, chunk := range chunks {
				// Mark this chunk's hash as referenced
				referenced[chunk.Hash] = true
			}
		}
	}

	return referenced, nil
}

// runReferenceCounting implements reference-counting garbage collection.
func (gc *GarbageCollector) runReferenceCounting(ctx context.Context) (GCStats, error) {
	var stats GCStats

	// List all blobs
	allBlobs, err := gc.store.List(ctx, "")
	if err != nil {
		return stats, fmt.Errorf("failed to list blobs: %w", err)
	}
	stats.BlobsScanned = int64(len(allBlobs))

	minAge := time.Now().UTC().AddDate(0, 0, -gc.cfg.MinAgeDays)

	for _, blob := range allBlobs {
		// Get metadata
		meta, err := gc.store.GetMetadata(ctx, blob.Hash)
		if err != nil {
			gc.logger.Warn("Failed to get metadata", "hash", blob.Hash, "error", err)
			continue
		}

		// Check reference count
		refCount := len(meta.References)
		if refCount > 0 {
			stats.BlobsMarked++
			continue
		}

		// Check minimum age
		if meta.CreatedAt.After(minAge) {
			continue
		}

		// Collect this blob
		if err := gc.collectBlob(ctx, blob.Hash); err != nil {
			gc.logger.Warn("Failed to collect blob", "hash", blob.Hash, "error", err)
			continue
		}

		stats.BlobsCollected++
		stats.BytesFreed += blob.Size
	}

	// Clean up old trash
	if err := gc.cleanupTrash(ctx); err != nil {
		gc.logger.Warn("Failed to cleanup trash", "error", err)
	}

	return stats, nil
}

// collectBlob moves a blob to trash.
func (gc *GarbageCollector) collectBlob(ctx context.Context, hash string) error {
	gc.logger.Debug("Collecting blob", "hash", hash)

	// Delete blob (moves to trash)
	return gc.store.Delete(ctx, hash)
}

// cleanupTrash permanently deletes blobs from trash that are older than retention period.
func (gc *GarbageCollector) cleanupTrash(ctx context.Context) error {
	// This implementation is backend-specific
	// For local store, scan trash directory
	if localStore, ok := gc.store.(*LocalStore); ok {
		return gc.cleanupLocalTrash(localStore)
	}

	// For hybrid store, cleanup both tiers
	if hybridStore, ok := gc.store.(*HybridStore); ok {
		if hot, ok := hybridStore.hot.(*LocalStore); ok {
			if err := gc.cleanupLocalTrash(hot); err != nil {
				gc.logger.Warn("Failed to cleanup hot tier trash", "error", err)
			}
		}
		// S3 trash cleanup would be similar
	}

	return nil
}

func (gc *GarbageCollector) cleanupLocalTrash(store *LocalStore) error {
	trashPath := filepath.Join(store.basePath, ".trash")
	cutoffTime := time.Now().UTC().AddDate(0, 0, -gc.cfg.TrashRetentionDays)

	var deletedCount int

	err := filepath.Walk(trashPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Check if old enough to delete
		if info.ModTime().Before(cutoffTime) {
			if err := os.Remove(path); err != nil {
				gc.logger.Warn("Failed to delete trash file", "path", path, "error", err)
				return nil
			}
			deletedCount++
		}

		return nil
	})

	if err != nil {
		return err
	}

	if deletedCount > 0 {
		gc.logger.Info("Cleaned up trash", "files_deleted", deletedCount)
	}

	return nil
}

// getStoragePressure returns the current storage pressure as a percentage (0-100).
func (gc *GarbageCollector) getStoragePressure() (int, error) {
	// For local storage, check disk usage
	if localStore, ok := gc.store.(*LocalStore); ok {
		return gc.getLocalStoragePressure(localStore)
	}

	// For S3, check against configured limits
	if s3Store, ok := gc.store.(*S3Store); ok {
		return gc.getS3StoragePressure(s3Store)
	}

	// For hybrid, check hot tier
	if hybridStore, ok := gc.store.(*HybridStore); ok {
		if hot, ok := hybridStore.hot.(*LocalStore); ok {
			return gc.getLocalStoragePressure(hot)
		}
	}

	return 0, fmt.Errorf("unable to determine storage pressure for backend type")
}

func (gc *GarbageCollector) getLocalStoragePressure(store *LocalStore) (int, error) {
	stats, err := store.Stats(context.Background())
	if err != nil {
		return 0, err
	}

	// If no max size configured, always return low pressure
	if store.cfg.MaxSizeGB == 0 {
		return 0, nil
	}

	maxBytes := store.cfg.MaxSizeGB * 1024 * 1024 * 1024
	used := stats.TotalSize

	pressure := int((used * 100) / maxBytes)
	if pressure > 100 {
		pressure = 100
	}

	return pressure, nil
}

func (gc *GarbageCollector) getS3StoragePressure(store *S3Store) (int, error) {
	// S3 typically doesn't have storage limits, so return 0
	// In production, you might check against billing limits or quotas
	return 0, nil
}

// Scheduler manages scheduled garbage collection.
type Scheduler struct {
	schedule string
	task     func(context.Context) error
	logger   *logger.Logger
	stop     chan struct{}
	done     chan struct{}
}

// NewScheduler creates a new GC scheduler.
func NewScheduler(schedule string, task func(context.Context) error, log *logger.Logger) *Scheduler {
	return &Scheduler{
		schedule: schedule,
		task:     task,
		logger:   log,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start starts the scheduler.
func (s *Scheduler) Start(ctx context.Context) error {
	// Parse cron schedule
	next, err := parseCronSchedule(s.schedule)
	if err != nil {
		return fmt.Errorf("invalid cron schedule: %w", err)
	}

	go s.run(ctx, next)
	return nil
}

// Stop stops the scheduler.
func (s *Scheduler) Stop() error {
	close(s.stop)
	<-s.done
	return nil
}

func (s *Scheduler) run(ctx context.Context, nextRun time.Time) {
	defer close(s.done)

	timer := time.NewTimer(time.Until(nextRun))
	defer timer.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-timer.C:
			// Run the task
			if err := s.task(ctx); err != nil {
				s.logger.Error("Scheduled GC task failed", "error", err)
			}

			// Schedule next run
			next, err := parseCronSchedule(s.schedule)
			if err != nil {
				s.logger.Error("Failed to parse cron schedule", "error", err)
				return
			}
			timer.Reset(time.Until(next))
		}
	}
}

// parseCronSchedule is a simplified cron parser.
// For production, use a library like github.com/robfig/cron/v3
func parseCronSchedule(schedule string) (time.Time, error) {
	// This is a placeholder implementation
	// For now, just schedule for tomorrow at 2 AM
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day()+1, 2, 0, 0, 0, now.Location())
	return next, nil
}

// ForceCollect forces immediate garbage collection (for testing/debugging).
func (gc *GarbageCollector) ForceCollect(ctx context.Context, hash string) error {
	return gc.collectBlob(ctx, hash)
}

// EmptyTrash permanently deletes all blobs in trash.
func (gc *GarbageCollector) EmptyTrash(ctx context.Context, permanent bool) error {
	if !permanent {
		return fmt.Errorf("must set permanent=true to empty trash")
	}

	gc.logger.Warn("Permanently deleting all blobs in trash")

	// For local store
	if localStore, ok := gc.store.(*LocalStore); ok {
		trashPath := filepath.Join(localStore.basePath, ".trash")
		return os.RemoveAll(trashPath)
	}

	// For hybrid store
	if hybridStore, ok := gc.store.(*HybridStore); ok {
		if hot, ok := hybridStore.hot.(*LocalStore); ok {
			trashPath := filepath.Join(hot.basePath, ".trash")
			if err := os.RemoveAll(trashPath); err != nil {
				return err
			}
		}
	}

	return nil
}
