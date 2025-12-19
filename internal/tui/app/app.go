// Package app provides the main TUI application for bib.
//
// The app is a full-screen interactive application similar to k9s or lazygit,
// providing a dashboard interface for managing bib resources.
//
// # Architecture
//
// The app is organized around:
//   - State: Global application state
//   - Router: Page navigation
//   - Pages: Full-screen views (dashboard, jobs, datasets, etc.)
//   - Dialogs: Modal overlays (confirm, input, error)
//
// # Usage
//
//	app := app.New(config)
//	program := tea.NewProgram(app, tea.WithAltScreen())
//	program.Run()
package app

import (
	"bib/internal/config"
	"bib/internal/tui/component"
	"bib/internal/tui/i18n"
	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// App is the main TUI application model.
type App struct {
	// Configuration
	config *config.BibConfig
	theme  *themes.Theme
	i18n   *i18n.I18n

	// State
	state *State

	// Navigation
	router *Router

	// Dimensions
	width  int
	height int

	// Modal overlay
	dialog Dialog

	// Error log panel (bottom of screen)
	errorLog *component.ErrorLog

	// Flags
	ready    bool
	quitting bool
}

// Option configures the App.
type Option func(*App)

// WithConfig sets the configuration.
func WithConfig(cfg *config.BibConfig) Option {
	return func(a *App) {
		a.config = cfg
	}
}

// WithTheme sets the theme.
func WithTheme(theme *themes.Theme) Option {
	return func(a *App) {
		a.theme = theme
	}
}

// WithI18n sets the i18n instance.
func WithI18n(i *i18n.I18n) Option {
	return func(a *App) {
		a.i18n = i
	}
}

// New creates a new App instance.
func New(opts ...Option) *App {
	a := &App{
		theme:    themes.Global().Active(),
		i18n:     i18n.Global(),
		state:    NewState(),
		errorLog: component.NewErrorLog().WithMaxEntries(100),
	}

	for _, opt := range opts {
		opt(a)
	}

	// Apply theme to error log
	a.errorLog.WithTheme(a.theme)

	// Initialize router with pages
	a.router = NewRouter(a)

	return a
}

// Init implements tea.Model.
func (a *App) Init() tea.Cmd {
	return tea.Batch(
		a.router.Init(),
		// Connect to bibd
		a.state.Connect(),
		// Load initial data
		a.state.LoadInitialData(),
	)
}

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle quit
		if msg.String() == "ctrl+c" {
			a.quitting = true
			return a, tea.Quit
		}

		// If dialog is open, send keys to dialog first
		if a.dialog != nil {
			dialog, cmd := a.dialog.Update(msg)
			a.dialog = dialog.(Dialog)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			// Check if dialog is done
			if a.dialog.Done() {
				a.dialog = nil
			}
			return a, tea.Batch(cmds...)
		}

		// Global keybindings
		switch msg.String() {
		case "q":
			// Show quit confirmation
			a.dialog = NewConfirmDialog(
				a.i18n.T("commands.quit.title"),
				a.i18n.T("commands.quit.confirm"),
				func(confirmed bool) tea.Cmd {
					if confirmed {
						a.quitting = true
						return tea.Quit
					}
					return nil
				},
			).WithTheme(a.theme)
			return a, nil

		case "?":
			// Show help
			a.dialog = NewHelpDialog(a.router.CurrentPage()).
				WithTheme(a.theme)
			return a, nil

		case "/":
			// Open search/command palette
			a.dialog = NewCommandPalette(a.router.Pages()).
				OnSelect(func(pageID string) tea.Cmd {
					return a.router.NavigateTo(pageID)
				}).
				WithTheme(a.theme)
			return a, nil

		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			// Number keys switch tabs
			idx := int(msg.String()[0] - '1')
			if idx < len(a.router.Pages()) {
				return a, a.router.NavigateToIndex(idx)
			}
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true

		// Propagate to router
		a.router.SetSize(msg.Width, msg.Height)

		// Propagate to dialog
		if a.dialog != nil {
			a.dialog.SetSize(msg.Width, msg.Height)
		}

	case StateMsg:
		// Handle state updates
		a.state.HandleMsg(msg)

	case NavigateMsg:
		return a, a.router.NavigateTo(msg.PageID)

	case ShowDialogMsg:
		a.dialog = msg.Dialog
		a.dialog.SetSize(a.width, a.height)
		return a, a.dialog.Init()

	case CloseDialogMsg:
		a.dialog = nil
	}

	// Update router
	router, cmd := a.router.Update(msg)
	a.router = router.(*Router)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return a, tea.Batch(cmds...)
}

// View implements tea.Model.
func (a *App) View() string {
	if a.quitting {
		return ""
	}

	if !a.ready {
		return a.i18n.T("common.loading")
	}

	// Calculate layout heights
	mainHeight := a.height
	var errorLogView string
	if !a.errorLog.IsEmpty() {
		a.errorLog.SetSize(a.width, 5) // Fixed height for collapsed log
		errorLogView = a.errorLog.ViewWidth(a.width)
		errorLogHeight := lipgloss.Height(errorLogView)
		mainHeight = a.height - errorLogHeight
	}

	// Main content (adjusted height)
	a.router.SetSize(a.width, mainHeight)
	content := a.router.View()

	// Overlay dialog if present
	if a.dialog != nil {
		content = a.overlayDialog(content, a.dialog.View())
	}

	// Combine main content with error log
	if !a.errorLog.IsEmpty() {
		content = lipgloss.JoinVertical(lipgloss.Left, content, errorLogView)
	}

	return content
}

// overlayDialog renders a dialog on top of the main content.
func (a *App) overlayDialog(background, dialog string) string {
	// Create semi-transparent overlay effect
	// In terminal we just center the dialog over the background
	bgLines := lipgloss.Height(background)
	dialogLines := lipgloss.Height(dialog)
	dialogWidth := lipgloss.Width(dialog)

	// Calculate position
	topPadding := (bgLines - dialogLines) / 2
	leftPadding := (a.width - dialogWidth) / 2

	if topPadding < 0 {
		topPadding = 0
	}
	if leftPadding < 0 {
		leftPadding = 0
	}

	// For now, just show the dialog (full overlay would require complex rendering)
	// Position dialog centered in the view
	_ = topPadding  // reserved for future overlay rendering
	_ = leftPadding // reserved for future overlay rendering

	return lipgloss.Place(
		a.width,
		a.height,
		lipgloss.Center,
		lipgloss.Center,
		dialog,
	)
}

// State returns the application state.
func (a *App) State() *State {
	return a.state
}

// Theme returns the current theme.
func (a *App) Theme() *themes.Theme {
	return a.theme
}

// I18n returns the i18n instance.
func (a *App) I18n() *i18n.I18n {
	return a.i18n
}

// Width returns the terminal width.
func (a *App) Width() int {
	return a.width
}

// Height returns the terminal height.
func (a *App) Height() int {
	return a.height
}

// ErrorLog returns the error log component.
func (a *App) ErrorLog() *component.ErrorLog {
	return a.errorLog
}

// LogError adds an error to the error log panel.
func (a *App) LogError(msg string, details string) {
	a.errorLog.AddError(msg, details)
}

// LogWarning adds a warning to the error log panel.
func (a *App) LogWarning(msg string, details string) {
	a.errorLog.AddWarning(msg, details)
}

// LogInfo adds an info message to the error log panel.
func (a *App) LogInfo(msg string, details string) {
	a.errorLog.AddInfo(msg, details)
}
