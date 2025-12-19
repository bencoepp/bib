// Package tui provides the tui command for launching the dashboard.
package tui

import (
	"bib/internal/cli/i18n"
	"bib/internal/config"
	"bib/internal/tui/app"
	tuii18n "bib/internal/tui/i18n"
	"bib/internal/tui/pages"
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

	// i18n is already initialized globally in root.go with the resolved locale

	// Load config
	cfg, err := config.LoadBib("")
	if err != nil {
		// Use defaults if config doesn't exist
		cfg = config.DefaultBibConfig()
	}

	// Create app
	application := app.New(
		app.WithConfig(cfg),
		app.WithTheme(themes.Global().Active()),
		app.WithI18n(tuii18n.Global()),
	)

	// Register pages
	application.State().Config = cfg
	registerPages(application)

	// Run program
	program := tea.NewProgram(application, tea.WithAltScreen())
	_, err = program.Run()
	return err
}

func registerPages(application *app.App) {
	router := application
	_ = router

	// Get the router from the app and register pages
	// For now, we'll create pages that will be registered during Init

	// Note: The actual page registration happens in the app.Router
	// We need to access the router after the app is created
	// This is a simplified version; in practice you'd have a factory

	// Create pages
	dashboardPage := pages.NewDashboardPage(application)
	jobsPage := pages.NewJobsPage(application)

	// Register with router
	// The app.Router handles this internally
	_ = dashboardPage
	_ = jobsPage
}
