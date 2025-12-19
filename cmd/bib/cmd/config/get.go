package configcmd

import (
	"encoding/json"
	"fmt"

	"bib/internal/config"

	"github.com/spf13/cobra"
)

// configGetCmd gets a specific configuration value
var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Long: `Get a specific configuration value by its key.

Keys use dot notation to access nested values.

Examples:
  bib config get log.level
  bib config get output.format
  bib config get --daemon database.backend
  bib config get --daemon p2p.mode`,
	Args: cobra.ExactArgs(1),
	RunE: runConfigGet,
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	key := args[0]
	out := NewOutputWriter()

	var configMap map[string]any

	if configDaemon {
		cfg, err := config.LoadBibd("")
		if err != nil {
			return fmt.Errorf("failed to load bibd config: %w", err)
		}
		// Convert to map for key access
		data, _ := json.Marshal(cfg)
		json.Unmarshal(data, &configMap)
	} else {
		cfg, err := config.LoadBib(ConfigFile())
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		data, _ := json.Marshal(cfg)
		json.Unmarshal(data, &configMap)
	}

	// Navigate nested keys
	value, err := getNestedValue(configMap, key)
	if err != nil {
		return err
	}

	return out.Write(value)
}
