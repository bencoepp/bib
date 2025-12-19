package configcmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"bib/internal/config"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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

// configEditCmd launches interactive TUI editor
var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Interactively edit configuration",
	Long: `Launch an interactive TUI to edit configuration.

The TUI provides a guided interface to modify all configuration options
with validation and help text.`,
	RunE: runConfigEdit,
}

var configInitForce bool

func init() {
	// Add --force flag to init
	configInitCmd.Flags().BoolVarP(&configInitForce, "force", "f", false, "overwrite existing configuration")
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

func runConfigPath(cmd *cobra.Command, args []string) error {
	out := NewOutputWriter()

	var appName string
	if configDaemon {
		appName = config.AppBibd
	} else {
		appName = config.AppBib
	}

	cfgFile := ConfigFile()
	if cfgFile != "" && !configDaemon {
		out.Write(cfgFile)
		return nil
	}

	if path := config.ConfigFileUsed(appName); path != "" {
		out.Write(path)
		return nil
	}

	out.Write("No config file found, using defaults")
	return nil
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

func runConfigEdit(cmd *cobra.Command, args []string) error {
	// Launch TUI editor
	return runConfigTUI(configDaemon)
}

// Helper functions

func getNestedValue(m map[string]any, key string) (any, error) {
	parts := strings.Split(key, ".")
	current := any(m)

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			val, ok := v[part]
			if !ok {
				return nil, fmt.Errorf("key not found: %s", key)
			}
			current = val
		default:
			return nil, fmt.Errorf("cannot access key %s in non-object value", part)
		}
	}

	return current, nil
}

func setNestedValue(m map[string]any, key string, value string) error {
	parts := strings.Split(key, ".")
	current := m

	// Navigate to parent
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		next, ok := current[part]
		if !ok {
			// Create nested map
			current[part] = make(map[string]any)
			next = current[part]
		}
		nextMap, ok := next.(map[string]any)
		if !ok {
			return fmt.Errorf("cannot set nested key %s: parent is not an object", key)
		}
		current = nextMap
	}

	// Set the value (try to parse as appropriate type)
	lastKey := parts[len(parts)-1]
	current[lastKey] = parseValue(value)
	return nil
}

func parseValue(s string) any {
	// Try bool
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}

	// Try int
	var i int
	if _, err := fmt.Sscanf(s, "%d", &i); err == nil {
		return i
	}

	// Default to string
	return s
}

func validateBibConfig(cfg *config.BibConfig) []string {
	var errors []string

	// Validate log level
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[cfg.Log.Level] {
		errors = append(errors, fmt.Sprintf("invalid log.level: %s (must be debug, info, warn, or error)", cfg.Log.Level))
	}

	// Validate output format
	validFormats := map[string]bool{"text": true, "json": true, "yaml": true, "table": true}
	if cfg.Output.Format != "" && !validFormats[cfg.Output.Format] {
		errors = append(errors, fmt.Sprintf("invalid output.format: %s", cfg.Output.Format))
	}

	return errors
}

func validateBibdConfig(cfg *config.BibdConfig) []string {
	var errors []string

	// Validate log level
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[cfg.Log.Level] {
		errors = append(errors, fmt.Sprintf("invalid log.level: %s", cfg.Log.Level))
	}

	// Validate database backend
	validBackends := map[string]bool{"sqlite": true, "postgres": true}
	if !validBackends[cfg.Database.Backend] {
		errors = append(errors, fmt.Sprintf("invalid database.backend: %s", cfg.Database.Backend))
	}

	// Validate P2P mode
	validP2PModes := map[string]bool{"proxy": true, "selective": true, "full": true}
	if cfg.P2P.Enabled && !validP2PModes[cfg.P2P.Mode] {
		errors = append(errors, fmt.Sprintf("invalid p2p.mode: %s", cfg.P2P.Mode))
	}

	// Validate server port
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		errors = append(errors, fmt.Sprintf("invalid server.port: %d", cfg.Server.Port))
	}

	return errors
}
