package blob

import (
	"context"
	"fmt"

	"bib/internal/config"
	"bib/internal/domain"
	"bib/internal/logger"
	"bib/internal/storage"
	"bib/internal/storage/blob"

	"github.com/spf13/cobra"
)

var verifyDataset string

// verifyCmd verifies blob integrity
var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify blob integrity",
	Long: `Verify the integrity of all blobs for a dataset version.

This checks that:
- All required blobs exist
- Blob hashes match expectations
- Blobs can be read successfully`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if verifyDataset == "" {
			return fmt.Errorf("--dataset flag is required")
		}
		return runBlobVerify(cmd.Context())
	},
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
	versionID := domain.DatasetVersionID(verifyDataset)

	fmt.Printf("Verifying integrity for dataset version: %s\n", versionID)

	// Run verification
	if err := ingestion.VerifyDatasetIntegrity(ctx, versionID); err != nil {
		return fmt.Errorf("integrity verification failed: %w", err)
	}

	fmt.Println("✓ Integrity verification passed")
	return nil
}
