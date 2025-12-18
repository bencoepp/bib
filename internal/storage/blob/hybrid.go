package blob

import (
	"context"
	"fmt"
	"io"
	"time"

	"bib/internal/logger"
)

// HybridStore implements tiered blob storage with hot (local) and cold (S3) backends.
type HybridStore struct {
	cfg    Config
	hot    Store // Local storage (hot tier)
	cold   Store // S3 storage (cold tier)
	logger *logger.Logger
}

// NewHybridStore creates a new hybrid blob store with tiering support.
func NewHybridStore(cfg Config, hot, cold Store, log *logger.Logger) (*HybridStore, error) {
	if hot == nil || cold == nil {
		return nil, fmt.Errorf("both hot and cold stores are required for hybrid mode")
	}

	return &HybridStore{
		cfg:    cfg,
		hot:    hot,
		cold:   cold,
		logger: log,
	}, nil
}

// Put stores a blob in the hot tier.
func (s *HybridStore) Put(ctx context.Context, hash string, data io.Reader, metadata *Metadata) error {
	// Always write to hot tier first
	return s.hot.Put(ctx, hash, data, metadata)
}

// Get retrieves a blob, checking hot tier first, then cold tier.
func (s *HybridStore) Get(ctx context.Context, hash string) (io.ReadCloser, error) {
	// Try hot tier first
	reader, err := s.hot.Get(ctx, hash)
	if err == nil {
		return reader, nil
	}

	// Try cold tier
	reader, err = s.cold.Get(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("blob not found in hot or cold tier: %s", hash)
	}

	// Optionally warm up: copy to hot tier for future access
	// This is done asynchronously to not block the read
	go func() {
		warmCtx := context.Background()
		if err := s.WarmUp(warmCtx, hash); err != nil {
			s.logger.Warn("Failed to warm up blob", "hash", hash, "error", err)
		}
	}()

	return reader, nil
}

// Delete removes a blob from both tiers.
func (s *HybridStore) Delete(ctx context.Context, hash string) error {
	var hotErr, coldErr error

	// Try to delete from hot tier
	hotErr = s.hot.Delete(ctx, hash)

	// Try to delete from cold tier
	coldErr = s.cold.Delete(ctx, hash)

	// Return error only if both failed
	if hotErr != nil && coldErr != nil {
		return fmt.Errorf("failed to delete from both tiers: hot=%v, cold=%v", hotErr, coldErr)
	}

	return nil
}

// Exists checks if a blob exists in either tier.
func (s *HybridStore) Exists(ctx context.Context, hash string) (bool, error) {
	// Check hot tier first
	exists, err := s.hot.Exists(ctx, hash)
	if err == nil && exists {
		return true, nil
	}

	// Check cold tier
	return s.cold.Exists(ctx, hash)
}

// Size returns the size of a blob from whichever tier has it.
func (s *HybridStore) Size(ctx context.Context, hash string) (int64, error) {
	// Try hot tier first
	size, err := s.hot.Size(ctx, hash)
	if err == nil {
		return size, nil
	}

	// Try cold tier
	return s.cold.Size(ctx, hash)
}

// List lists blobs from both tiers (deduplicated).
func (s *HybridStore) List(ctx context.Context, prefix string) ([]BlobInfo, error) {
	hotBlobs, err := s.hot.List(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list hot tier: %w", err)
	}

	coldBlobs, err := s.cold.List(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list cold tier: %w", err)
	}

	// Deduplicate by hash
	seen := make(map[string]bool)
	var allBlobs []BlobInfo

	for _, blob := range hotBlobs {
		if !seen[blob.Hash] {
			allBlobs = append(allBlobs, blob)
			seen[blob.Hash] = true
		}
	}

	for _, blob := range coldBlobs {
		if !seen[blob.Hash] {
			allBlobs = append(allBlobs, blob)
			seen[blob.Hash] = true
		}
	}

	return allBlobs, nil
}

// Touch updates the last accessed time.
func (s *HybridStore) Touch(ctx context.Context, hash string) error {
	// Update in whichever tier has the blob
	if exists, _ := s.hot.Exists(ctx, hash); exists {
		return s.hot.Touch(ctx, hash)
	}

	if exists, _ := s.cold.Exists(ctx, hash); exists {
		return s.cold.Touch(ctx, hash)
	}

	return fmt.Errorf("blob not found: %s", hash)
}

// GetMetadata retrieves metadata from whichever tier has it.
func (s *HybridStore) GetMetadata(ctx context.Context, hash string) (*Metadata, error) {
	// Try hot tier first
	meta, err := s.hot.GetMetadata(ctx, hash)
	if err == nil {
		return meta, nil
	}

	// Try cold tier
	return s.cold.GetMetadata(ctx, hash)
}

// UpdateMetadata updates metadata in whichever tier has the blob.
func (s *HybridStore) UpdateMetadata(ctx context.Context, hash string, meta *Metadata) error {
	var hotErr, coldErr error

	if exists, _ := s.hot.Exists(ctx, hash); exists {
		hotErr = s.hot.UpdateMetadata(ctx, hash, meta)
	}

	if exists, _ := s.cold.Exists(ctx, hash); exists {
		coldErr = s.cold.UpdateMetadata(ctx, hash, meta)
	}

	if hotErr != nil && coldErr != nil {
		return fmt.Errorf("failed to update metadata in both tiers")
	}

	return nil
}

// Move moves a blob to another store.
func (s *HybridStore) Move(ctx context.Context, hash string, to Store) error {
	// Determine which tier has the blob
	if exists, _ := s.hot.Exists(ctx, hash); exists {
		return s.hot.Move(ctx, hash, to)
	}

	if exists, _ := s.cold.Exists(ctx, hash); exists {
		return s.cold.Move(ctx, hash, to)
	}

	return fmt.Errorf("blob not found: %s", hash)
}

// Copy copies a blob to another store.
func (s *HybridStore) Copy(ctx context.Context, hash string, to Store) error {
	// Determine which tier has the blob
	if exists, _ := s.hot.Exists(ctx, hash); exists {
		return s.hot.Copy(ctx, hash, to)
	}

	if exists, _ := s.cold.Exists(ctx, hash); exists {
		return s.cold.Copy(ctx, hash, to)
	}

	return fmt.Errorf("blob not found: %s", hash)
}

// Backend returns the hybrid backend type.
func (s *HybridStore) Backend() BackendType {
	return "hybrid"
}

// Stats returns combined storage statistics.
func (s *HybridStore) Stats(ctx context.Context) (*Stats, error) {
	hotStats, err := s.hot.Stats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get hot tier stats: %w", err)
	}

	coldStats, err := s.cold.Stats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get cold tier stats: %w", err)
	}

	// Combine stats (note: this may double-count blobs that exist in both tiers)
	combined := &Stats{
		TotalBlobs:          hotStats.TotalBlobs + coldStats.TotalBlobs,
		TotalSize:           hotStats.TotalSize + coldStats.TotalSize,
		TotalSizeCompressed: hotStats.TotalSizeCompressed + coldStats.TotalSizeCompressed,
		Backend:             "hybrid",
	}

	if !hotStats.OldestBlob.IsZero() && (coldStats.OldestBlob.IsZero() || hotStats.OldestBlob.Before(coldStats.OldestBlob)) {
		combined.OldestBlob = hotStats.OldestBlob
	} else {
		combined.OldestBlob = coldStats.OldestBlob
	}

	if hotStats.NewestBlob.After(coldStats.NewestBlob) {
		combined.NewestBlob = hotStats.NewestBlob
	} else {
		combined.NewestBlob = coldStats.NewestBlob
	}

	return combined, nil
}

// Close closes both stores.
func (s *HybridStore) Close() error {
	var hotErr, coldErr error

	if s.hot != nil {
		hotErr = s.hot.Close()
	}

	if s.cold != nil {
		coldErr = s.cold.Close()
	}

	if hotErr != nil {
		return hotErr
	}

	return coldErr
}

// Tier management methods

// CoolDown moves a blob from hot to cold tier.
func (s *HybridStore) CoolDown(ctx context.Context, hash string) error {
	// Check if blob is in hot tier
	exists, err := s.hot.Exists(ctx, hash)
	if err != nil || !exists {
		return fmt.Errorf("blob not in hot tier: %s", hash)
	}

	// Check if already in cold tier
	existsInCold, _ := s.cold.Exists(ctx, hash)
	if !existsInCold {
		// Copy to cold tier
		if err := s.hot.Copy(ctx, hash, s.cold); err != nil {
			return fmt.Errorf("failed to copy to cold tier: %w", err)
		}
	}

	// Delete from hot tier
	if err := s.hot.Delete(ctx, hash); err != nil {
		return fmt.Errorf("failed to delete from hot tier: %w", err)
	}

	s.logger.Debug("Blob cooled down", "hash", hash)
	return nil
}

// WarmUp moves a blob from cold to hot tier.
func (s *HybridStore) WarmUp(ctx context.Context, hash string) error {
	// Check if blob is in cold tier
	exists, err := s.cold.Exists(ctx, hash)
	if err != nil || !exists {
		return fmt.Errorf("blob not in cold tier: %s", hash)
	}

	// Check if already in hot tier
	existsInHot, _ := s.hot.Exists(ctx, hash)
	if existsInHot {
		return nil // Already warm
	}

	// Copy to hot tier
	if err := s.cold.Copy(ctx, hash, s.hot); err != nil {
		return fmt.Errorf("failed to copy to hot tier: %w", err)
	}

	s.logger.Debug("Blob warmed up", "hash", hash)
	return nil
}

// ApplyTieringPolicy applies the tiering policy to move blobs between tiers.
func (s *HybridStore) ApplyTieringPolicy(ctx context.Context) error {
	if !s.cfg.Tiering.Enabled {
		return nil
	}

	s.logger.Info("Applying tiering policy", "strategy", s.cfg.Tiering.Strategy)

	switch s.cfg.Tiering.Strategy {
	case "lru":
		return s.applyLRUPolicy(ctx)
	case "age":
		return s.applyAgePolicy(ctx)
	case "manual":
		// Manual tiering - no automatic moves
		return nil
	default:
		return fmt.Errorf("unknown tiering strategy: %s", s.cfg.Tiering.Strategy)
	}
}

func (s *HybridStore) applyLRUPolicy(ctx context.Context) error {
	// Get hot tier stats
	hotStats, err := s.hot.Stats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get hot tier stats: %w", err)
	}

	// Check if hot tier is over limit
	maxSizeBytes := s.cfg.Tiering.HotMaxSizeGB * 1024 * 1024 * 1024
	if hotStats.TotalSize < maxSizeBytes {
		return nil // Under limit
	}

	// List all hot blobs
	blobs, err := s.hot.List(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to list hot blobs: %w", err)
	}

	// Sort by last accessed time (oldest first)
	// Note: This is a simplified implementation
	// For production, you'd want a more sophisticated LRU algorithm

	bytesToFree := hotStats.TotalSize - maxSizeBytes
	var freedBytes int64

	for _, blob := range blobs {
		if freedBytes >= bytesToFree {
			break
		}

		// Cool down this blob
		if err := s.CoolDown(ctx, blob.Hash); err != nil {
			s.logger.Warn("Failed to cool down blob", "hash", blob.Hash, "error", err)
			continue
		}

		freedBytes += blob.Size
	}

	s.logger.Info("LRU policy applied", "freed_bytes", freedBytes, "blobs_moved", freedBytes/1024/1024)
	return nil
}

func (s *HybridStore) applyAgePolicy(ctx context.Context) error {
	// Get all hot blobs
	blobs, err := s.hot.List(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to list hot blobs: %w", err)
	}

	cutoffTime := time.Now().UTC().AddDate(0, 0, -s.cfg.Tiering.HotMaxAgeDays)
	var movedCount int

	for _, blob := range blobs {
		if blob.CreatedAt.Before(cutoffTime) {
			if err := s.CoolDown(ctx, blob.Hash); err != nil {
				s.logger.Warn("Failed to cool down blob", "hash", blob.Hash, "error", err)
				continue
			}
			movedCount++
		}
	}

	s.logger.Info("Age policy applied", "blobs_moved", movedCount)
	return nil
}
