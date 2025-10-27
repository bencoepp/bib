package cmd

import (
	"bib/internal/selfupdate"
	"fmt"
	"runtime"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Check the version of bib",
	Long: `Use this command to check the current version of bib
and update bib if needed. To update bib, run:

	bib version --update

We recommend keeping bib up-to-date to ensure you have a version
that is compatible with the latest features and improvements.
Note that updating bib may require internet access. And 
breaking changes may be introduced in new versions.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("You are running bib version:")
		fmt.Printf("bib v%s\n", appVersion)
		fmt.Printf("OS: %s\n", runtime.GOOS)
		fmt.Printf("Architecture: %s\n", runtime.GOARCH)
		fmt.Printf("CPU cores: %d\n", runtime.NumCPU())
		fmt.Printf("Go Version: %s\n", runtime.Version())

		update, _ := cmd.Flags().GetBool("update")
		if update {
			if !Config.Update.Enabled {
				log.Info("⛔  Auto-updates are disabled in the configuration. Enable them to use this feature.")
			}
			err := selfupdate.UpdateBib(appVersion, &selfupdate.Option{
				Owner:           Config.Update.GitHubOwner,
				Repo:            Config.Update.GitHubRepo,
				BinaryName:      "bib",
				AllowPrerelease: Config.Update.AllowPrerelease,
				HTTPTimeout:     time.Duration(Config.Update.HTTPTimeoutInSec),
			})
			if err != nil {
				log.Error(err)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)

	versionCmd.Flags().BoolP("update", "u", false, "Update bib to the latest version")

}
