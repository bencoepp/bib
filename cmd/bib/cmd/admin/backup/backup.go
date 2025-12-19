package backup

import (
	"github.com/spf13/cobra"
)

// Cmd represents the backup command group
var Cmd = &cobra.Command{
	Use:   "backup",
	Short: "Database backup operations",
	Long: `Manage database backups for the bibd daemon.

Supports automated backups, manual backups, point-in-time recovery (PostgreSQL),
and backup verification.`,
}

// restoreCmd is the restore command (at admin level, not under backup)
var RestoreCmd = &cobra.Command{
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

// NewCommand returns the backup command with all subcommands registered
func NewCommand() *cobra.Command {
	Cmd.AddCommand(createCmd)
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(deleteCmd)

	return Cmd
}

// NewRestoreCommand returns the restore command
func NewRestoreCommand() *cobra.Command {
	return RestoreCmd
}

func init() {
	// Backup create flags
	createCmd.Flags().StringVar(&backupNotes, "notes", "", "Notes about this backup")
	createCmd.Flags().BoolVar(&backupVerify, "verify", true, "Verify backup integrity after creation")

	// Backup delete flags
	deleteCmd.Flags().BoolVar(&backupForce, "force", false, "Force deletion without confirmation")

	// Restore flags
	RestoreCmd.Flags().BoolVar(&restoreForce, "force", false, "Force restore even if data exists")
	RestoreCmd.Flags().BoolVar(&restoreVerify, "verify", true, "Verify backup integrity before restore")
	RestoreCmd.Flags().StringVar(&targetTime, "target-time", "", "Target time for point-in-time recovery (PostgreSQL only)")
}
