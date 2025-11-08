package cmd

import (
	"bib/internal/config"
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
configuration file manually if you prefer that.

By default, this command will launch an interactive TUI to guide you through the setup process.
The actual creation of a bibd configuration file is optional and can be triggered via the --daemon flag.`,
	Run: func(cmd *cobra.Command, args []string) {
		tui, _ := cmd.Flags().GetBool("no-tui")

		if tui {
			con, _ := cmd.Flags().GetBool("config")
			if con {
				path, err := config.SaveBibConfig()
				if err != nil {
					log.Fatal("Could not create default configuration file:", "error", err)
				}

				log.Info("As you have passed the --no-tui flag, we have created a default configuration file for you.")
				log.Info("You now need to edit the configuration file manually to your needs.")
				log.Info("The configuration file is located at:", "path", path)
				log.Info("If you want to run the interactive setup, please run 'bib setup' without the --no-tui flag.")

				if daemon, _ := cmd.Flags().GetBool("daemon"); daemon {
					daemonPath, err := config.SaveBibDaemonConfig()
					if err != nil {
						log.Fatal("Could not create default bib daemon configuration file:", "error", err)
					}
					log.Info("A default bib daemon configuration file has also been created for you.")
					log.Info("The bib daemon configuration file is located at:", "path", daemonPath)
				}
			}
		} else {
			p := tea.NewProgram(
				models.SetupModel{
					Cfg:     Config,
					Version: appVersion,
				},
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
	setupCmd.Flags().BoolP("config", "c", false, "Create config file only (default)")
}
