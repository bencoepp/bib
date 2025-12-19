package backup

import (
	"fmt"

	"bib/internal/config"
	"bib/internal/storage"
	"bib/internal/storage/backup"

	"github.com/spf13/cobra"
)

var backupForce bool

// deleteCmd deletes a backup
var deleteCmd = &cobra.Command{
	Use:   "delete <backup-id>",
	Short: "Delete a backup",
	Long: `Delete a specific backup by ID.

This permanently removes the backup file and its metadata.`,
	Example: `  # Delete a specific backup
  bib admin backup delete 1234567890`,
	Args: cobra.ExactArgs(1),
	RunE: runBackupDelete,
}

func runBackupDelete(cmd *cobra.Command, args []string) error {
	backupID := args[0]

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

	// Confirm deletion unless --force is used
	if !backupForce {
		fmt.Printf("Are you sure you want to delete backup %s? (y/N): ", backupID)
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	// Delete backup
	if err := mgr.Delete(backupID); err != nil {
		return fmt.Errorf("failed to delete backup: %w", err)
	}

	fmt.Printf("✓ Backup %s deleted successfully\n", backupID)

	return nil
}
