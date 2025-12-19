package admin

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"bib/internal/config"
	"bib/internal/storage"
	"bib/internal/storage/backup"

	"github.com/spf13/cobra"
)

var (
	backupNotes   string
	backupForce   bool
	backupVerify  bool
	backupID      string
	targetTime    string
	restoreForce  bool
	restoreVerify bool
)

// backupCmd represents the backup command
var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Database backup operations",
	Long: `Manage database backups for the bibd daemon.

Supports automated backups, manual backups, point-in-time recovery (PostgreSQL),
and backup verification.`,
}

// backupCreateCmd creates a new backup
var backupCreateCmd = &cobra.Command{
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

// backupListCmd lists available backups
var backupListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List available backups",
	Long: `List all available database backups with their metadata.

Shows backup ID, timestamp, size, backend, and format for each backup.`,
	RunE: runBackupList,
}

// backupDeleteCmd deletes a backup
var backupDeleteCmd = &cobra.Command{
	Use:   "delete <backup-id>",
	Short: "Delete a backup",
	Long: `Delete a specific backup by ID.

This permanently removes the backup file and its metadata.`,
	Example: `  # Delete a specific backup
  bib admin backup delete 1234567890`,
	Args: cobra.ExactArgs(1),
	RunE: runBackupDelete,
}

// restoreCmd restores a backup
var restoreCmd = &cobra.Command{
	Use:   "restore <backup-id>",
	Short: "Restore a database backup",
	Long: `Restore the database from a backup.

WARNING: This operation will replace the current database contents.
The bibd daemon should be stopped before running this command.

For PostgreSQL, you can optionally specify a target time for
point-in-time recovery (if WAL archiving is enabled).`,
	Example: `  # Restore from a specific backup
  bib admin restore 1234567890

  # Restore and verify first
  bib admin restore 1234567890 --verify

  # Force restore even if data exists
  bib admin restore 1234567890 --force

  # Point-in-time recovery (PostgreSQL only)
  bib admin restore 1234567890 --target-time "2025-12-18T14:30:00Z"`,
	Args: cobra.ExactArgs(1),
	RunE: runRestore,
}

func init() {
	Cmd.AddCommand(backupCmd)
	Cmd.AddCommand(restoreCmd)

	backupCmd.AddCommand(backupCreateCmd)
	backupCmd.AddCommand(backupListCmd)
	backupCmd.AddCommand(backupDeleteCmd)

	// Backup create flags
	backupCreateCmd.Flags().StringVar(&backupNotes, "notes", "", "Notes about this backup")
	backupCreateCmd.Flags().BoolVar(&backupVerify, "verify", true, "Verify backup integrity after creation")

	// Backup delete flags
	backupDeleteCmd.Flags().BoolVar(&backupForce, "force", false, "Force deletion without confirmation")

	// Restore flags
	restoreCmd.Flags().BoolVar(&restoreForce, "force", false, "Force restore even if data exists")
	restoreCmd.Flags().BoolVar(&restoreVerify, "verify", true, "Verify backup integrity before restore")
	restoreCmd.Flags().StringVar(&targetTime, "target-time", "", "Target time for point-in-time recovery (PostgreSQL only)")
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
		// TODO: Get connection string from lifecycle manager or config
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

func runBackupList(cmd *cobra.Command, args []string) error {
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

	// List backups
	backups, err := mgr.List()
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	if len(backups) == 0 {
		fmt.Println("No backups found")
		return nil
	}

	// Display backups in a table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tTIMESTAMP\tBACKEND\tFORMAT\tSIZE\tNOTES")
	fmt.Fprintln(w, "--\t---------\t-------\t------\t----\t-----")

	for _, b := range backups {
		sizeMB := float64(b.Size) / (1024 * 1024)
		timestamp := b.Timestamp.Format("2006-01-02 15:04")
		notes := b.Notes
		if len(notes) > 40 {
			notes = notes[:37] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%.1f MB\t%s\n",
			b.ID, timestamp, b.Backend, b.Format, sizeMB, notes)
	}

	w.Flush()

	fmt.Printf("\nTotal: %d backup(s)\n", len(backups))

	return nil
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

func runRestore(cmd *cobra.Command, args []string) error {
	_ = cmd // unused
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
		// TODO: Get connection string from lifecycle manager or config
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
	if err := mgr.Restore(ctx, opts); err != nil {
		return fmt.Errorf("restore failed: %w", err)
	}

	fmt.Println("\n✓ Database restored successfully")
	fmt.Println("\nPlease restart the bibd daemon for changes to take effect.")

	return nil
}
