// Package tui provides the tui command for launching the dashboard.
package tui

import (
	"fmt"

	"bib/internal/config"
	"bib/internal/tui/app"
	"bib/internal/tui/i18n"
	"bib/internal/tui/pages"
	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	tuiTheme  string
	tuiLocale string
)

// NewCommand creates the tui command.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Launch interactive dashboard",
		Long: `Launch the full-screen interactive TUI dashboard.

The dashboard provides a visual interface for managing bib resources,
including jobs, datasets, cluster status, and more.

Navigation:
  Tab/Shift+Tab  Switch between pages
  1-9            Jump to page by number
  /              Open command palette
  ?              Show help
  q              Quit

Available pages:
  Dashboard      Overview with stats and recent activity
  Jobs           View and manage jobs
  Datasets       Browse datasets
  Cluster        Cluster status and nodes
  Logs           View logs
  Settings       Configuration`,
		Example: `  # Launch dashboard with default theme
  bib tui

  # Launch with specific theme
  bib tui --theme nord

  # Launch with specific locale
  bib tui --locale de`,
		RunE: runTUI,
	}

	cmd.Flags().StringVarP(&tuiTheme, "theme", "t", "dark", "color theme (dark, light, nord, dracula, gruvbox)")
	cmd.Flags().StringVarP(&tuiLocale, "locale", "l", "en", "locale for UI text")

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

	// Set locale
	if tuiLocale != "" && tuiLocale != "en" {
		if err := i18n.Global().SetLocale(tuiLocale); err != nil {
			// Warn but continue with default
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: locale %q not available, using English\n", tuiLocale)
		}
	}

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
		app.WithI18n(i18n.Global()),
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
