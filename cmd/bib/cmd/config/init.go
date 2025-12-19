package configcmd

import (
	"fmt"
	"os"

	"bib/internal/config"

	"github.com/spf13/cobra"
)

var configInitForce bool

// configInitCmd generates default configuration
var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate default configuration",
	Long: `Generate a default configuration file.

If a configuration file already exists, this will not overwrite it
unless --force is specified.

Examples:
  bib config init
  bib config init --daemon
  bib config init --force`,
	RunE: runConfigInit,
}

func init() {
	// Add --force flag to init
	configInitCmd.Flags().BoolVarP(&configInitForce, "force", "f", false, "overwrite existing configuration")
}

func runConfigInit(cmd *cobra.Command, args []string) error {
	out := NewOutputWriter()

	var appName string
	if configDaemon {
		appName = config.AppBibd
	} else {
		appName = config.AppBib
	}

	// Check if config already exists
	existingPath := config.ConfigFileUsed(appName)
	if existingPath != "" && !configInitForce {
		return fmt.Errorf("config file already exists at %s; use --force to overwrite", existingPath)
	}

	// Generate config
	path, _, err := config.GenerateConfigIfNotExists(appName, "yaml")
	if err != nil {
		// If it already exists and we're forcing, we need to regenerate
		if configInitForce && existingPath != "" {
			if err := os.Remove(existingPath); err != nil {
				return fmt.Errorf("failed to remove existing config: %w", err)
			}
			path, _, err = config.GenerateConfigIfNotExists(appName, "yaml")
			if err != nil {
				return fmt.Errorf("failed to generate config: %w", err)
			}
		} else {
			return fmt.Errorf("failed to generate config: %w", err)
		}
	}

	out.WriteSuccess(fmt.Sprintf("Configuration initialized at: %s", path))
	return nil
}
