package admin

import (
	"bib/cmd/bib/cmd/admin/backup"
	"bib/cmd/bib/cmd/admin/blob"
	"bib/cmd/bib/cmd/admin/breakglass"

	"github.com/spf13/cobra"
)

// Cmd represents the admin command group
var Cmd = &cobra.Command{
	Use:   "admin",
	Short: "Administrative commands for bibd management",
	Long: `Administrative commands for managing the bibd daemon.

These commands provide access to low-level administrative operations
including break-glass emergency access, backup management, and system
diagnostics.

Most admin commands require a connection to a running bibd daemon
and appropriate permissions.`,
}

// NewCommand returns the admin command with all subcommands registered
func NewCommand() *cobra.Command {
	// Add subcommand groups from subpackages
	Cmd.AddCommand(backup.NewCommand())
	Cmd.AddCommand(backup.NewRestoreCommand())
	Cmd.AddCommand(blob.NewCommand())
	Cmd.AddCommand(breakglass.NewCommand())

	// Add standalone commands
	Cmd.AddCommand(cleanupCmd)
	Cmd.AddCommand(resetCmd)

	return Cmd
}
