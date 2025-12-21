// Package tui provides the tui command for launching the dashboard.
package tui

import (
	"bib/internal/cli/i18n"
	"bib/internal/config"
	tuii18n "bib/internal/tui/i18n"
	"bib/internal/tui/shared"
	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	tuiTheme string
)

// NewCommand creates the tui command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "tui",
		Short:       "tui.short",
		Long:        "tui.long",
		Example:     "tui.example",
		Annotations: i18n.MarkForTranslation(),
		RunE:        runTUI,
	}

	cmd.Flags().StringVarP(&tuiTheme, "theme", "t", "dark", "color theme (dark, light, nord, dracula, gruvbox)")

	return cmd
}

func runTUI(cmd *cobra.Command, args []string) error {
	// Set theme
	switch tuiTheme {
	case "light":
		themes.Global().SetActive(themes.PresetLight)
	case "nord":
		themes.Global().SetActive(themes.PresetNord)
	case "dracula":
		themes.Global().SetActive(themes.PresetDracula)
	case "gruvbox":
		themes.Global().SetActive(themes.PresetGruvbox)
	default:
		themes.Global().SetActive(themes.PresetDark)
	}

	// Load config
	cfg, err := config.LoadBib("")
	if err != nil {
		// Use defaults if config doesn't exist
		cfg = config.DefaultBibConfig()
	}

	// Create shared TUI
	tui := shared.New(
		shared.WithMode(shared.ModeCLI),
		shared.WithConfig(cfg),
		shared.WithTheme(themes.Global().Active()),
		shared.WithI18n(tuii18n.Global()),
	)

	// Run program
	program := tea.NewProgram(tui, tea.WithAltScreen())
	_, err = program.Run()
	return err
}
