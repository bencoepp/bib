package cmd

import (
	"encoding/json"
	"fmt"

	"bib/internal/config"

	"github.com/spf13/cobra"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  `View and manage bib configuration.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// configShowCmd shows current configuration
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  `Display the current configuration values that are in effect.`,
	RunE:  runConfigShow,
}

// configPathCmd shows config file path
var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show config file path",
	Long:  `Display the path to the configuration file being used.`,
	RunE:  runConfigPath,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configPathCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	currentCfg := Config()
	if currentCfg == nil {
		var err error
		currentCfg, err = config.LoadBib(ConfigFile())
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
	}

	// Pretty print the config as JSON
	output, err := json.MarshalIndent(currentCfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	fmt.Println(string(output))
	return nil
}

func runConfigPath(cmd *cobra.Command, args []string) error {
	cfgFile := ConfigFile()
	if cfgFile != "" {
		fmt.Println(cfgFile)
		return nil
	}

	if path := config.ConfigFileUsed(config.AppBib); path != "" {
		fmt.Println(path)
		return nil
	}

	fmt.Println("No config file found, using defaults")
	return nil
}
