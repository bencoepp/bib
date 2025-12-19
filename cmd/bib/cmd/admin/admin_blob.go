package admin

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"bib/internal/config"
	"bib/internal/domain"
	"bib/internal/logger"
	"bib/internal/storage"
	"bib/internal/storage/blob"

	"github.com/spf13/cobra"
)

var (
	blobGCForce       bool
	blobGCPermanent   bool
	blobGCEmptyTrash  bool
	blobStatsFormat   string
	blobVerifyDataset string
	blobTierCool      string
	blobTierWarm      string
	blobTierApply     bool
)

// adminBlobCmd represents the blob admin command group
var adminBlobCmd = &cobra.Command{
	Use:   "blob",
	Short: "Manage blob storage",
	Long: `Manage blob storage including garbage collection, statistics, and tiering.

Blob storage is used for storing dataset chunks with features like
content-addressed storage, deduplication, compression, and encryption.`,
}

// adminBlobStatsCmd shows blob storage statistics
var adminBlobStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show blob storage statistics",
	Long: `Display statistics about blob storage including total blobs,
total size, compression ratio, and backend information.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBlobStats(cmd.Context())
	},
}

// adminBlobGCCmd runs garbage collection
var adminBlobGCCmd = &cobra.Command{
	Use:   "gc",
	Short: "Run garbage collection",
	Long: `Run garbage collection on blob storage to remove orphaned blobs.

Orphaned blobs are those that:
- Have no database references
- Are older than the configured minimum age
- Belong to deleted datasets

By default, orphaned blobs are moved to trash. Use --permanent to
permanently delete trash contents.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBlobGC(cmd.Context())
	},
}

// adminBlobVerifyCmd verifies blob integrity
var adminBlobVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify blob integrity",
	Long: `Verify the integrity of all blobs for a dataset version.

This checks that:
- All required blobs exist
- Blob hashes match expectations
- Blobs can be read successfully`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if blobVerifyDataset == "" {
			return fmt.Errorf("--dataset flag is required")
		}
		return runBlobVerify(cmd.Context())
	},
}

// adminBlobTierCmd manages blob tiering
var adminBlobTierCmd = &cobra.Command{
	Use:   "tier",
	Short: "Manage blob tiering (hybrid mode only)",
	Long: `Manage blob tiering between hot (local) and cold (S3) storage.

Operations:
- Move specific blobs between tiers (--cool, --warm)
- Apply tiering policy to all blobs (--apply)

Only available when blob storage is configured in hybrid mode.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBlobTier(cmd.Context())
	},
}

func init() {
	Cmd.AddCommand(adminBlobCmd)
	adminBlobCmd.AddCommand(adminBlobStatsCmd)
	adminBlobCmd.AddCommand(adminBlobGCCmd)
	adminBlobCmd.AddCommand(adminBlobVerifyCmd)
	adminBlobCmd.AddCommand(adminBlobTierCmd)

	// GC flags
	adminBlobGCCmd.Flags().BoolVar(&blobGCForce, "force", false, "Force GC even if conditions not met")
	adminBlobGCCmd.Flags().BoolVar(&blobGCPermanent, "permanent", false, "Permanently delete trash contents")
	adminBlobGCCmd.Flags().BoolVar(&blobGCEmptyTrash, "empty-trash", false, "Empty trash without running GC")

	// Stats flags
	adminBlobStatsCmd.Flags().StringVar(&blobStatsFormat, "format", "table", "Output format: table, json")

	// Verify flags
	adminBlobVerifyCmd.Flags().StringVar(&blobVerifyDataset, "dataset", "", "Dataset version ID to verify")

	// Tier flags
	adminBlobTierCmd.Flags().StringVar(&blobTierCool, "cool", "", "Move blob to cold tier (hash)")
	adminBlobTierCmd.Flags().StringVar(&blobTierWarm, "warm", "", "Move blob to hot tier (hash)")
	adminBlobTierCmd.Flags().BoolVar(&blobTierApply, "apply", false, "Apply tiering policy to all blobs")
}

func runBlobStats(ctx context.Context) error {
	// Load configuration
	cfg, err := config.LoadBibd("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create logger
	log, err := logger.New(cfg.Log)
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	defer log.Close()

	// Convert config
	storageConfig := convertDatabaseConfig(&cfg.Database)

	// Open storage with blob support
	_, blobMgrIface, err := storage.OpenWithBlob(ctx, storageConfig, cfg.Server.DataDir, "admin-cli", "proxy", nil)
	if err != nil {
		return fmt.Errorf("failed to open storage: %w", err)
	}

	// Type assert to blob.Manager
	blobManager, ok := blobMgrIface.(*blob.Manager)
	if !ok {
		return fmt.Errorf("blob manager type assertion failed")
	}
	defer blobManager.Close()

	// Get statistics
	stats, err := blobManager.Stats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get stats: %w", err)
	}

	// Display based on format
	switch blobStatsFormat {
	case "json":
		fmt.Printf(`{
  "total_blobs": %d,
  "total_size": %d,
  "total_size_compressed": %d,
  "oldest_blob": "%s",
  "newest_blob": "%s",
  "backend": "%s"
}
`, stats.TotalBlobs, stats.TotalSize, stats.TotalSizeCompressed,
			stats.OldestBlob.Format(time.RFC3339),
			stats.NewestBlob.Format(time.RFC3339),
			stats.Backend)

	default: // table
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "METRIC\tVALUE")
		fmt.Fprintf(w, "Total Blobs\t%d\n", stats.TotalBlobs)
		fmt.Fprintf(w, "Total Size\t%s\n", formatBytes(stats.TotalSize))
		if stats.TotalSizeCompressed > 0 {
			ratio := float64(stats.TotalSize) / float64(stats.TotalSizeCompressed)
			fmt.Fprintf(w, "Compressed Size\t%s (%.2fx)\n", formatBytes(stats.TotalSizeCompressed), ratio)
		}
		if !stats.OldestBlob.IsZero() {
			fmt.Fprintf(w, "Oldest Blob\t%s\n", stats.OldestBlob.Format(time.RFC3339))
		}
		if !stats.NewestBlob.IsZero() {
			fmt.Fprintf(w, "Newest Blob\t%s\n", stats.NewestBlob.Format(time.RFC3339))
		}
		fmt.Fprintf(w, "Backend\t%s\n", stats.Backend)
		w.Flush()
	}

	return nil
}

func runBlobGC(ctx context.Context) error {
	// Load configuration
	cfg, err := config.LoadBibd("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create logger
	log, err := logger.New(cfg.Log)
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	defer log.Close()

	// Convert config
	storageConfig := convertDatabaseConfig(&cfg.Database)

	// Open storage with blob support
	_, blobMgrIface, err := storage.OpenWithBlob(ctx, storageConfig, cfg.Server.DataDir, "admin-cli", "proxy", nil)
	if err != nil {
		return fmt.Errorf("failed to open storage: %w", err)
	}

	// Type assert to blob.Manager
	blobManager, ok := blobMgrIface.(*blob.Manager)
	if !ok {
		return fmt.Errorf("blob manager type assertion failed")
	}
	defer blobManager.Close()

	gc := blobManager.GC()

	// Handle empty-trash operation
	if blobGCEmptyTrash {
		if !blobGCPermanent {
			return fmt.Errorf("--empty-trash requires --permanent flag")
		}
		fmt.Println("Emptying trash (permanent deletion)...")
		if err := gc.EmptyTrash(ctx, true); err != nil {
			return fmt.Errorf("failed to empty trash: %w", err)
		}
		fmt.Println("Trash emptied successfully")
		return nil
	}

	// Run garbage collection
	fmt.Println("Running garbage collection...")
	startTime := time.Now()

	var gcErr error
	if blobGCForce {
		gcErr = gc.Run(ctx)
	} else {
		gcErr = gc.RunWithPressure(ctx)
	}

	if gcErr != nil {
		return fmt.Errorf("garbage collection failed: %w", gcErr)
	}

	duration := time.Since(startTime)
	fmt.Printf("Garbage collection completed in %s\n", duration)

	// Show updated stats
	stats, err := blobManager.Stats(ctx)
	if err == nil {
		fmt.Printf("Total blobs: %d (%s)\n", stats.TotalBlobs, formatBytes(stats.TotalSize))
	}

	return nil
}

func runBlobVerify(ctx context.Context) error {
	// Load configuration
	cfg, err := config.LoadBibd("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create logger
	log, err := logger.New(cfg.Log)
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	defer log.Close()

	// Convert config
	storageConfig := convertDatabaseConfig(&cfg.Database)

	// Open storage with blob support
	store, blobMgrIface, err := storage.OpenWithBlob(ctx, storageConfig, cfg.Server.DataDir, "admin-cli", "proxy", nil)
	if err != nil {
		return fmt.Errorf("failed to open storage: %w", err)
	}
	defer store.Close()

	// Type assert to blob.Manager
	blobManager, ok := blobMgrIface.(*blob.Manager)
	if !ok {
		return fmt.Errorf("blob manager type assertion failed")
	}
	defer blobManager.Close()

	ingestion := blobManager.Ingestion()

	// Parse dataset version ID
	versionID := domain.DatasetVersionID(blobVerifyDataset)

	fmt.Printf("Verifying integrity for dataset version: %s\n", versionID)

	// Run verification
	if err := ingestion.VerifyDatasetIntegrity(ctx, versionID); err != nil {
		return fmt.Errorf("integrity verification failed: %w", err)
	}

	fmt.Println("✓ Integrity verification passed")
	return nil
}

func runBlobTier(ctx context.Context) error {
	// Load configuration
	cfg, err := config.LoadBibd("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Convert config
	storageConfig := convertDatabaseConfig(&cfg.Database)

	// Check that we're in hybrid mode
	if storageConfig.Blob.Mode != "hybrid" {
		return fmt.Errorf("tiering is only available in hybrid mode (current mode: %s)", storageConfig.Blob.Mode)
	}

	// Create logger
	log, err := logger.New(cfg.Log)
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	defer log.Close()

	// Open storage with blob support
	_, blobMgrIface, err := storage.OpenWithBlob(ctx, storageConfig, cfg.Server.DataDir, "admin-cli", "proxy", nil)
	if err != nil {
		return fmt.Errorf("failed to open storage: %w", err)
	}

	// Type assert to blob.Manager
	blobManager, ok := blobMgrIface.(*blob.Manager)
	if !ok {
		return fmt.Errorf("blob manager type assertion failed")
	}
	defer blobManager.Close()

	hybridStore, ok := blobManager.Store().(*blob.HybridStore)
	if !ok {
		return fmt.Errorf("blob store is not in hybrid mode")
	}

	// Handle cool operation
	if blobTierCool != "" {
		fmt.Printf("Moving blob to cold tier: %s\n", blobTierCool)
		if err := hybridStore.CoolDown(ctx, blobTierCool); err != nil {
			return fmt.Errorf("failed to cool down blob: %w", err)
		}
		fmt.Println("✓ Blob moved to cold tier")
		return nil
	}

	// Handle warm operation
	if blobTierWarm != "" {
		fmt.Printf("Moving blob to hot tier: %s\n", blobTierWarm)
		if err := hybridStore.WarmUp(ctx, blobTierWarm); err != nil {
			return fmt.Errorf("failed to warm up blob: %w", err)
		}
		fmt.Println("✓ Blob moved to hot tier")
		return nil
	}

	// Handle apply policy operation
	if blobTierApply {
		fmt.Println("Applying tiering policy...")
		if err := blobManager.ApplyTieringPolicy(ctx); err != nil {
			return fmt.Errorf("failed to apply tiering policy: %w", err)
		}
		fmt.Println("✓ Tiering policy applied")
		return nil
	}

	return fmt.Errorf("no operation specified (use --cool, --warm, or --apply)")
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// convertDatabaseConfig converts config.DatabaseConfig to storage.Config
// This is a simplified conversion - in production, you'd want full field mapping
func convertDatabaseConfig(cfg *config.DatabaseConfig) storage.Config {
	storageConfig := storage.DefaultConfig()

	// Map backend type
	if cfg.Backend == "sqlite" {
		storageConfig.Backend = storage.BackendSQLite
	} else if cfg.Backend == "postgres" {
		storageConfig.Backend = storage.BackendPostgres
	}

	// Map SQLite config
	storageConfig.SQLite.Path = cfg.SQLite.Path
	storageConfig.SQLite.MaxOpenConns = cfg.SQLite.MaxOpenConns

	// Map Postgres config
	storageConfig.Postgres.Managed = cfg.Postgres.Managed
	storageConfig.Postgres.DataDir = cfg.Postgres.DataDir

	// Blob config is part of storage.Config by default

	return storageConfig
}
