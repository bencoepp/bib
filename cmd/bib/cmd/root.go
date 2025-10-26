package cmd

import (
	"bib/internal/capcheck"
	"bib/internal/capcheck/checks"
	"bib/internal/config"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "bib",
	Short: "Scientific and historical bibliography manager",
	Long: `bib is a command‑line toolkit designed for scientists, historians, and academic researchers 
who work with large, complex, and distributed datasets—often across teams, institutions, and borders. 

Its central ambition is to help you curate, link, verify, and analyze information at scale, 
even when the network or the counterparties cannot be fully trusted. Think of bib as a portable 
backbone for your research data: versioned, reproducible, provenance‑aware, and collaboration‑friendly.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if shouldSkipConfig(cmd) {

		}

		configPath, err := config.FindConfigPath(config.Options{AppName: "bib", FileNames: []string{"config.yaml", "config.yml"}})
		cfg, err := config.LoadBibConfig(configPath)
		if err != nil {
			log.Error(err)
			log.Info("You need to setup bib before using it. Please run 'bib setup' to create a configuration file.")
			return
		}

		if cfg.General.CheckCapabilities {
			checkers := []capcheck.Checker{
				checks.ContainerRuntimeChecker{},
				checks.KubernetesConfigChecker{},
				checks.InternetAccessChecker{
					// Use a target that works in your environment. Alternatives:
					// "https://www.google.com/generate_204" or a company endpoint.
					HTTPURL: "https://www.google.com/generate_204",
				},
				checks.ResourcesChecker{},
			}

			runner := capcheck.NewRunner(
				checkers,
				capcheck.WithGlobalTimeout(6*time.Second),
				capcheck.WithPerCheckTimeout(1*time.Second),
			)
			_ = runner
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.bib.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func shouldSkipConfig(cmd *cobra.Command) bool {
	skipCommands := []string{"help", "version", "setup"}
	for _, c := range skipCommands {
		if cmd.CalledAs() == c {
			return true
		}
	}
	return false
}
