package backup

import (
	"context"
	"fmt"
	"time"

	"bib/internal/config"
	"bib/internal/storage"
	"bib/internal/storage/backup"

	"github.com/spf13/cobra"
)

var (
	restoreForce  bool
	restoreVerify bool
	targetTime    string
)

func runRestore(cmd *cobra.Command, args []string) error {
	backupID := args[0]
	ctx := context.Background()

	// Load configuration
	cfg, err := config.LoadBibd("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize backup manager
	backupCfg := backup.DefaultBackupConfig()
	backupCfg.LocalPath = cfg.Server.DataDir + "/backups"

	mgr, err := backup.NewManager(backupCfg, storage.BackendType(cfg.Database.Backend), cfg.Server.DataDir, cfg.Cluster.NodeID)
	if err != nil {
		return fmt.Errorf("failed to create backup manager: %w", err)
	}

	// Set connection information based on backend
	if cfg.Database.Backend == "postgres" {
		fmt.Println("PostgreSQL restore requires the daemon to be running or connection details to be provided")
		return fmt.Errorf("PostgreSQL restore not yet fully implemented for offline mode")
	} else {
		// SQLite
		dbPath := cfg.Database.SQLite.Path
		if dbPath == "" {
			dbPath = cfg.Server.DataDir + "/cache.db"
		}
		mgr.SetDatabasePath(dbPath)
	}

	// Parse target time if provided
	var targetTimePtr *time.Time
	if targetTime != "" {
		t, err := time.Parse(time.RFC3339, targetTime)
		if err != nil {
			return fmt.Errorf("invalid target time format (use RFC3339): %w", err)
		}
		targetTimePtr = &t
	}

	// Build restore options
	opts := backup.RestoreOptions{
		BackupID:     backupID,
		TargetTime:   targetTimePtr,
		Force:        restoreForce,
		VerifyBefore: restoreVerify,
	}

	// Confirm restore unless --force is used
	if !restoreForce {
		fmt.Println("WARNING: This will replace the current database contents.")
		fmt.Printf("Are you sure you want to restore backup %s? (y/N): ", backupID)
		var response string
		_, _ = fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	fmt.Println("Restoring backup...")
	if err := mgr.RestoreWithOptions(ctx, opts); err != nil {
		return fmt.Errorf("restore failed: %w", err)
	}

	fmt.Println("\n✓ Database restored successfully")
	fmt.Println("\nPlease restart the bibd daemon for changes to take effect.")

	return nil
}
