package blob

import (
	"context"
	"fmt"

	"bib/internal/config"
	"bib/internal/logger"
	"bib/internal/storage"
	"bib/internal/storage/blob"

	"github.com/spf13/cobra"
)

var (
	tierCool  string
	tierWarm  string
	tierApply bool
)

// tierCmd manages blob tiering
var tierCmd = &cobra.Command{
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
	if tierCool != "" {
		fmt.Printf("Moving blob to cold tier: %s\n", tierCool)
		if err := hybridStore.CoolDown(ctx, tierCool); err != nil {
			return fmt.Errorf("failed to cool down blob: %w", err)
		}
		fmt.Println("✓ Blob moved to cold tier")
		return nil
	}

	// Handle warm operation
	if tierWarm != "" {
		fmt.Printf("Moving blob to hot tier: %s\n", tierWarm)
		if err := hybridStore.WarmUp(ctx, tierWarm); err != nil {
			return fmt.Errorf("failed to warm up blob: %w", err)
		}
		fmt.Println("✓ Blob moved to hot tier")
		return nil
	}

	// Handle apply policy operation
	if tierApply {
		fmt.Println("Applying tiering policy...")
		if err := blobManager.ApplyTieringPolicy(ctx); err != nil {
			return fmt.Errorf("failed to apply tiering policy: %w", err)
		}
		fmt.Println("✓ Tiering policy applied")
		return nil
	}

	return fmt.Errorf("no operation specified (use --cool, --warm, or --apply)")
}
