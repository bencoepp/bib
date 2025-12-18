package cmd

import (
	"github.com/spf13/cobra"
)

// adminCmd represents the admin command group
var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Administrative commands for bibd management",
	Long: `Administrative commands for managing the bibd daemon.

These commands provide access to low-level administrative operations
including break-glass emergency access, backup management, and system
diagnostics.

Most admin commands require a connection to a running bibd daemon
and appropriate permissions.`,
}

func init() {
	rootCmd.AddCommand(adminCmd)
}
