package backup

import (
	"fmt"
	"os"
	"text/tabwriter"

	"bib/internal/config"
	"bib/internal/storage"
	"bib/internal/storage/backup"

	"github.com/spf13/cobra"
)

// listCmd lists available backups
var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List available backups",
	Long: `List all available database backups with their metadata.

Shows backup ID, timestamp, size, backend, and format for each backup.`,
	RunE: runBackupList,
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
	backups, err := mgr.List(cmd.Context())
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
