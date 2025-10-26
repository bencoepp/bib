package cmd

import (
	"fmt"

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
		fmt.Println("setup called")
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)

	setupCmd.Flags().BoolP("daemon", "d", true, "Create configuration for bib daemon")
}
