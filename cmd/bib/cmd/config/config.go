package configcmd

import (
	"bib/internal/config"

	"github.com/spf13/cobra"
)

var (
	configDaemon bool // --daemon flag for bibd config
)

// Cmd represents the config command
var Cmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long: `View and manage bib configuration.

Use --daemon to manage bibd daemon configuration instead of bib CLI.

Subcommands:
  show      Display current configuration
  get       Get a specific configuration value
  set       Set a configuration value
  path      Show config file path
  init      Generate default configuration
  validate  Validate configuration
  edit      Interactively edit configuration (TUI)`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// NewCommand returns the config command with all subcommands registered
func NewCommand() *cobra.Command {
	// Register subcommands
	Cmd.AddCommand(configShowCmd)
	Cmd.AddCommand(configPathCmd)
	Cmd.AddCommand(configGetCmd)
	Cmd.AddCommand(configSetCmd)
	Cmd.AddCommand(configInitCmd)
	Cmd.AddCommand(configValidateCmd)
	Cmd.AddCommand(configEditCmd)

	// Add flags
	Cmd.PersistentFlags().BoolVar(&configDaemon, "daemon", false, "Manage bibd daemon configuration")

	return Cmd
}

// Package-level config state (loaded by commands that need it)
var (
	loadedConfig  *config.BibConfig
	loadedCfgFile string
)

// Config returns the loaded bib config (loads if needed)
func Config() *config.BibConfig {
	if loadedConfig == nil {
		loadedConfig, _ = config.LoadBib("")
	}
	return loadedConfig
}

// ConfigFile returns the config file path
func ConfigFile() string {
	if loadedCfgFile == "" {
		loadedCfgFile = config.ConfigFileUsed(config.AppBib)
	}
	return loadedCfgFile
}
