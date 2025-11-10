package cmd

import (
	"bib/internal/ui/models"
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// meCmd represents the me command
var meCmd = &cobra.Command{
	Use:   "me",
	Short: "Display information about yourself",
	Long: `This command displays detailed information about your
local identity. As well as the list of local bib deamon instances.

You can use this command to verify your identity and check the status,
you can also use it to troubleshoot any issues related to your identity.

For more detailed status information run 'bib status'.`,
	Run: func(cmd *cobra.Command, args []string) {
		if _, err := tea.NewProgram(models.MeModel{
			Theme:    Theme,
			Identity: *Identity,
		}).Run(); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(meCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// meCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// meCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
