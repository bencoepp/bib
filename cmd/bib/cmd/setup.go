package cmd

import (
	"bib/internal/ui/models"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
)

// setupCmd represents the setup command
var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Setup command for initial configuration",
	Long: `Use this command if you need to perform initial setup.

This is needed if you have not used bib before, or if you want to reset your configuration.
It is also a requirement to setup a bib daemon on your machine. You can also create the
configuration file manually if you prefer that.`,
	Run: func(cmd *cobra.Command, args []string) {
		tui, _ := cmd.Flags().GetBool("tui")

		if tui {
			p := tea.NewProgram(
				models.SetupModel{},
				tea.WithAltScreen(),
				tea.WithMouseCellMotion(),
			)
			if _, err := p.Run(); err != nil {
				log.Fatal(err)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)

	setupCmd.Flags().BoolP("daemon", "d", false, "Create configuration for bib daemon")
	setupCmd.Flags().BoolP("tui", "t", false, "Open interactive setup (TUI)")
}
