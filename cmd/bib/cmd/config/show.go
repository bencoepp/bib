package configcmd

import (
	"fmt"

	"bib/internal/config"

	"github.com/spf13/cobra"
)

// configShowCmd shows current configuration
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  `Display the current configuration values that are in effect.`,
	RunE:  runConfigShow,
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	out := NewOutputWriter()

	if configDaemon {
		// Load bibd config
		cfg, err := config.LoadBibd("")
		if err != nil {
			return fmt.Errorf("failed to load bibd config: %w", err)
		}
		return out.Write(cfg)
	}

	// Load bib config
	currentCfg := Config()
	if currentCfg == nil {
		var err error
		currentCfg, err = config.LoadBib(ConfigFile())
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
	}

	return out.Write(currentCfg)
}
