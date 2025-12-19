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
	Use:         "config",
	Short:       "config.short",
	Long:        "config.long",
	Annotations: map[string]string{"i18n": "true"},
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
