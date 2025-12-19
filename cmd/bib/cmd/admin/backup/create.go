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
	backupNotes  string
	backupVerify bool
)

// createCmd creates a new backup
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new database backup",
	Long: `Create a new backup of the bibd database.

The backup is automatically compressed, encrypted (if configured), and
verified for integrity. Metadata about the backup is stored for later
restore operations.`,
	Example: `  # Create a backup with notes
  bib admin backup create --notes "Before upgrade to v0.2.0"

  # Create and verify
  bib admin backup create --verify`,
	RunE: runBackupCreate,
}

func runBackupCreate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load configuration
	cfg, err := config.LoadBibd("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize backup manager
	backupCfg := backup.DefaultBackupConfig()
	backupCfg.LocalPath = cfg.Server.DataDir + "/backups"
	backupCfg.VerifyAfterBackup = backupVerify

	mgr, err := backup.NewManager(backupCfg, storage.BackendType(cfg.Database.Backend), cfg.Server.DataDir, cfg.Cluster.NodeID)
	if err != nil {
		return fmt.Errorf("failed to create backup manager: %w", err)
	}

	// Set connection information based on backend
	if cfg.Database.Backend == "postgres" {
		fmt.Println("PostgreSQL backup requires the daemon to be running or connection details to be provided")
		return fmt.Errorf("PostgreSQL backup not yet fully implemented for offline mode")
	} else {
		// SQLite
		dbPath := cfg.Database.SQLite.Path
		if dbPath == "" {
			dbPath = cfg.Server.DataDir + "/cache.db"
		}
		mgr.SetDatabasePath(dbPath)
	}

	fmt.Println("Creating backup...")
	metadata, err := mgr.Backup(ctx, backupNotes)
	if err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	fmt.Println("\n✓ Backup created successfully")
	fmt.Printf("\nBackup ID:    %s\n", metadata.ID)
	fmt.Printf("Timestamp:    %s\n", metadata.Timestamp.Format(time.RFC3339))
	fmt.Printf("Backend:      %s\n", metadata.Backend)
	fmt.Printf("Format:       %s\n", metadata.Format)
	fmt.Printf("Size:         %.2f MB\n", float64(metadata.Size)/(1024*1024))
	fmt.Printf("Compressed:   %v\n", metadata.Compressed)
	fmt.Printf("Encrypted:    %v\n", metadata.Encrypted)
	fmt.Printf("Location:     %s\n", metadata.Path)
	fmt.Printf("Hash:         %s\n", metadata.IntegrityHash)
	if metadata.Notes != "" {
		fmt.Printf("Notes:        %s\n", metadata.Notes)
	}

	return nil
}
