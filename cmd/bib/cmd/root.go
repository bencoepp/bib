package cmd

import (
	"bib/internal/config"
	"bib/internal/ui/tea"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	AppVersion = "dev"
	cfgFile    string
	v          *viper.Viper
	bibCfg     *config.BibConfig
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:          "bib",
	Short:        "bib is the terminal client",
	SilenceUsage: true,
	// Initialize Viper, bind flags, and load config before any command runs.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialize Viper with conventional locations and env support.
		var used string
		var err error
		v, used, err = config.Init("bib", cfgFile)
		if err != nil {
			return err
		}

		// Defaults for bib.
		config.DefaultsBib(v)

		// Bind flags -> viper keys so flags override file/env.
		if err := bindBibFlags(cmd, v); err != nil {
			return err
		}

		// Load into struct.
		bibCfg, err = config.LoadBib(v)
		if err != nil {
			return err
		}
		if used != "" {
			// Optional: show which config file was used when verbose/logging.
			// fmt.Fprintf(os.Stderr, "Using config file: %s\n", used)
		}
		return nil
	},
	// If no subcommand is provided, run the TUI.
	RunE: func(cmd *cobra.Command, args []string) error {
		return tea.Run(bibCfg)
	},
}

func init() {
	// Persistent flags available to all subcommands
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default searches XDG/HOME/.)")

	// Attach the built-in version subcommand
	rootCmd.AddCommand(versionCmd)
}

func bindBibFlags(cmd *cobra.Command, v *viper.Viper) error {
	// Map flags to viper keys (env and file already supported by config.Init)

	return nil
}
