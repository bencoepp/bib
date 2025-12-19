package configcmd

import (
	"github.com/spf13/cobra"
)

// configEditCmd launches interactive TUI editor
var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Interactively edit configuration",
	Long: `Launch an interactive TUI to edit configuration.

The TUI provides a guided interface to modify all configuration options
with validation and help text.`,
	RunE: runConfigEdit,
}

func runConfigEdit(cmd *cobra.Command, args []string) error {
	// Launch TUI editor
	return runConfigTUI(configDaemon)
}
