package blob

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"bib/internal/config"
	"bib/internal/logger"
	"bib/internal/storage"
	"bib/internal/storage/blob"

	"github.com/spf13/cobra"
)

var statsFormat string

// statsCmd shows blob storage statistics
var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show blob storage statistics",
	Long: `Display statistics about blob storage including total blobs,
total size, compression ratio, and backend information.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBlobStats(cmd.Context())
	},
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
	switch statsFormat {
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
