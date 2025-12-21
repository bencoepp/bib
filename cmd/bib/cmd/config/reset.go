package configcmd

import (
	"fmt"
	"os"
	"path/filepath"

	"bib/internal/config"
	"bib/internal/setup/partial"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

// ResetSection represents a configuration section that can be reset
type ResetSection string

const (
	ResetSectionAll        ResetSection = "all"
	ResetSectionConnection ResetSection = "connection"
	ResetSectionOutput     ResetSection = "output"
	ResetSectionNodes      ResetSection = "nodes"
	ResetSectionIdentity   ResetSection = "identity"
)

// NewResetCommand creates the config reset command
func NewResetCommand() *cobra.Command {
	var (
		force     bool
		allFlag   bool
		keepNodes bool
	)

	cmd := &cobra.Command{
		Use:   "reset [section]",
		Short: "Reset configuration to defaults",
		Long: `Reset configuration to default values.

Sections that can be reset:
  all          Reset entire configuration
  connection   Reset connection settings
  output       Reset output format settings
  nodes        Reset node list (clear favorites)
  identity     Reset identity key (regenerate)

Examples:
  # Reset all configuration (with confirmation)
  bib config reset --all

  # Reset output settings
  bib config reset output

  # Reset connection settings without confirmation
  bib config reset connection --force

  # Reset all but keep node list
  bib config reset --all --keep-nodes`,
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{"all", "connection", "output", "nodes", "identity"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var section ResetSection = ResetSectionAll

			if len(args) > 0 {
				section = ResetSection(args[0])
			} else if !allFlag {
				return fmt.Errorf("specify a section or use --all")
			}

			return runReset(section, force, keepNodes)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Reset without confirmation")
	cmd.Flags().BoolVar(&allFlag, "all", false, "Reset all configuration")
	cmd.Flags().BoolVar(&keepNodes, "keep-nodes", false, "Keep node list when resetting all")

	return cmd
}

func runReset(section ResetSection, force, keepNodes bool) error {
	// Confirm unless force
	if !force {
		var confirm bool
		confirmForm := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(fmt.Sprintf("Reset %s configuration?", section)).
					Description("This will restore default settings and cannot be undone.").
					Affirmative("Yes, reset").
					Negative("Cancel").
					Value(&confirm),
			),
		).WithTheme(huh.ThemeCatppuccin())

		if err := confirmForm.Run(); err != nil {
			return nil
		}

		if !confirm {
			fmt.Println("Reset cancelled.")
			return nil
		}
	}

	// Get config directory
	configDir, err := config.UserConfigDir(config.AppBib)
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	configPath := filepath.Join(configDir, "config.yaml")

	switch section {
	case ResetSectionAll:
		return resetAll(configPath, keepNodes)
	case ResetSectionConnection:
		return resetConnection(configPath)
	case ResetSectionOutput:
		return resetOutput(configPath)
	case ResetSectionNodes:
		return resetNodes(configPath)
	case ResetSectionIdentity:
		return resetIdentity(configDir)
	default:
		return fmt.Errorf("unknown section: %s", section)
	}
}

func resetAll(configPath string, keepNodes bool) error {
	var savedNodes []config.FavoriteNode

	// Optionally preserve nodes
	if keepNodes {
		cfg, err := config.LoadBib("")
		if err == nil && cfg != nil {
			savedNodes = cfg.Connection.FavoriteNodes
		}
	}

	// Delete existing config
	if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete config: %w", err)
	}

	// Delete partial configs
	configDir := filepath.Dir(configPath)
	partialMgr := partial.NewManager(configDir)
	partialMgr.Delete("cli")
	partialMgr.Delete("daemon")

	// Create new default config
	cfg := &config.BibConfig{}

	// Restore nodes if requested
	if keepNodes && len(savedNodes) > 0 {
		cfg.Connection.FavoriteNodes = savedNodes
	}

	if err := config.SaveBib(cfg, configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("✓ Configuration reset to defaults")
	if keepNodes && len(savedNodes) > 0 {
		fmt.Printf("  Preserved %d favorite node(s)\n", len(savedNodes))
	}

	return nil
}

func resetConnection(configPath string) error {
	cfg, err := config.LoadBib("")
	if err != nil {
		cfg = &config.BibConfig{}
	}

	// Reset connection settings
	cfg.Connection.DefaultNode = ""
	cfg.Connection.Timeout = ""
	cfg.Connection.RetryAttempts = 0
	cfg.Connection.Mode = ""
	cfg.Connection.AutoDetect = false
	cfg.Connection.PoolSize = 0

	if err := config.SaveBib(cfg, configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("✓ Connection settings reset to defaults")
	return nil
}

func resetOutput(configPath string) error {
	cfg, err := config.LoadBib("")
	if err != nil {
		cfg = &config.BibConfig{}
	}

	// Reset output settings
	cfg.Output.Format = ""
	cfg.Output.Color = true

	if err := config.SaveBib(cfg, configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("✓ Output settings reset to defaults")
	return nil
}

func resetNodes(configPath string) error {
	cfg, err := config.LoadBib("")
	if err != nil {
		cfg = &config.BibConfig{}
	}

	nodeCount := len(cfg.Connection.FavoriteNodes)

	// Clear nodes
	cfg.Connection.FavoriteNodes = nil
	cfg.Connection.DefaultNode = ""

	if err := config.SaveBib(cfg, configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✓ Cleared %d favorite node(s)\n", nodeCount)
	return nil
}

func resetIdentity(configDir string) error {
	keyPath := filepath.Join(configDir, "identity.key")

	// Check if key exists
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		fmt.Println("No identity key to reset.")
		return nil
	}

	// Delete identity key
	if err := os.Remove(keyPath); err != nil {
		return fmt.Errorf("failed to delete identity key: %w", err)
	}

	fmt.Println("✓ Identity key deleted")
	fmt.Println("  Run 'bib setup' to generate a new key")
	return nil
}
