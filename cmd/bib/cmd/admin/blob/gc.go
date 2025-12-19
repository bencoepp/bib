package blob

import (
	"context"
	"fmt"
	"time"

	"bib/internal/config"
	"bib/internal/logger"
	"bib/internal/storage"
	"bib/internal/storage/blob"

	"github.com/spf13/cobra"
)

var (
	gcForce      bool
	gcPermanent  bool
	gcEmptyTrash bool
)

// gcCmd runs garbage collection
var gcCmd = &cobra.Command{
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
	if gcEmptyTrash {
		if !gcPermanent {
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
	if gcForce {
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
