package admin

import (
	"bib/cmd/bib/cmd/admin/backup"
	"bib/cmd/bib/cmd/admin/blob"
	"bib/cmd/bib/cmd/admin/breakglass"

	"github.com/spf13/cobra"
)

// Cmd represents the admin command group
var Cmd = &cobra.Command{
	Use:         "admin",
	Short:       "admin.short",
	Long:        "admin.long",
	Annotations: map[string]string{"i18n": "true"},
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
