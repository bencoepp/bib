package configcmd

import (
	"fmt"
	"os"

	"bib/internal/config"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// configSetCmd sets a configuration value
var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a specific configuration value.

Keys use dot notation to access nested values.
Changes are written to the configuration file.

Examples:
  bib config set log.level debug
  bib config set output.format json
  bib config set --daemon database.backend postgres
  bib config set --daemon p2p.mode full`,
	Args: cobra.ExactArgs(2),
	RunE: runConfigSet,
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]
	out := NewOutputWriter()

	var appName string
	if configDaemon {
		appName = config.AppBibd
	} else {
		appName = config.AppBib
	}

	// Get config file path
	cfgPath := config.ConfigFileUsed(appName)
	if cfgPath == "" {
		return fmt.Errorf("no config file found; run 'bib config init' first")
	}

	// Read existing config
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse as YAML (works for both YAML and JSON)
	var configMap map[string]any
	if err := yaml.Unmarshal(data, &configMap); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set the nested value
	if err := setNestedValue(configMap, key, value); err != nil {
		return err
	}

	// Write back
	output, err := yaml.Marshal(configMap)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(cfgPath, output, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	out.WriteSuccess(fmt.Sprintf("Set %s = %s", key, value))
	return nil
}
