package cmd

import (
	"fmt"
	"os"

	"bib/internal/config"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// cfgFile is the path to the config file (set via --config flag)
	cfgFile string

	// cfg holds the loaded configuration
	cfg *config.BibConfig
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "bib",
	Short: "bib is a CLI client",
	Long:  `bib is a command-line interface client that communicates with the bibd daemon.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip config loading for setup command when no config exists
		if cmd.Name() == "setup" {
			return nil
		}

		return loadConfig(cmd)
	},
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(onInitialize)

	// Global persistent flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/bib/config.yaml)")
	rootCmd.PersistentFlags().StringP("output", "o", "", "output format (text, json, yaml, table)")

	// Bind flags to viper
	viper.BindPFlag("output.format", rootCmd.PersistentFlags().Lookup("output"))
}

// onInitialize is called before any command runs
func onInitialize() {
	// Auto-generate config on first run if setup command is not being used
	if cfgFile == "" {
		path, created, err := config.GenerateConfigIfNotExists(config.AppBib, "yaml")
		if err == nil && created {
			fmt.Fprintf(os.Stderr, "Created default config at: %s\n", path)
			fmt.Fprintf(os.Stderr, "Run 'bib setup' to customize your configuration.\n")
		}
	}
}

// loadConfig loads the configuration
func loadConfig(cmd *cobra.Command) error {
	var err error
	cfg, err = config.LoadBib(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Apply flag overrides
	if cmd.Flags().Changed("output") {
		cfg.Output.Format = viper.GetString("output.format")
	}

	return nil
}

// Config returns the current configuration (for use by subcommands)
func Config() *config.BibConfig {
	return cfg
}

// ConfigFile returns the config file path (for use by subcommands)
func ConfigFile() string {
	return cfgFile
}
