package configcmd

import (
	"fmt"

	"bib/internal/config"

	"github.com/spf13/cobra"
)

// configValidateCmd validates configuration
var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration",
	Long: `Validate the current configuration for errors.

Checks for:
  - Valid YAML/JSON syntax
  - Required fields
  - Valid enum values
  - Path existence (for file paths)
  - Port conflicts`,
	RunE: runConfigValidate,
}

func runConfigValidate(cmd *cobra.Command, args []string) error {
	out := NewOutputWriter()

	var validationErrors []string

	if configDaemon {
		cfg, err := config.LoadBibd("")
		if err != nil {
			return fmt.Errorf("failed to load bibd config: %w", err)
		}
		validationErrors = validateBibdConfig(cfg)
	} else {
		cfg, err := config.LoadBib(ConfigFile())
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		validationErrors = validateBibConfig(cfg)
	}

	if len(validationErrors) > 0 {
		fmt.Println("Configuration validation failed:")
		for _, e := range validationErrors {
			fmt.Printf("  - %s\n", e)
		}
		return fmt.Errorf("found %d validation error(s)", len(validationErrors))
	}

	out.WriteSuccess("Configuration is valid")
	return nil
}
