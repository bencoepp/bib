package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version information (set via ldflags at build time)
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  `Print the version, commit hash, and build date of bib.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("bib version %s\n", Version)
		fmt.Printf("  commit:  %s\n", Commit)
		fmt.Printf("  built:   %s\n", BuildDate)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
