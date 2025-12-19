// Package pages provides full-screen pages for the bib TUI dashboard.
//
// Each page implements the app.Page interface and represents a distinct
// view in the application (dashboard, jobs, datasets, etc.).
package pages

import (
	"bib/internal/tui/app"
	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
)

// BasePage provides common functionality for pages.
type BasePage struct {
	id     string
	title  string
	theme  *themes.Theme
	app    *app.App
	width  int
	height int
}

// NewBasePage creates a new base page.
func NewBasePage(id, title string, application *app.App) *BasePage {
	return &BasePage{
		id:    id,
		title: title,
		theme: themes.Global().Active(),
		app:   application,
	}
}

// ID returns the page identifier.
func (p *BasePage) ID() string {
	return p.id
}

// Title returns the page title.
func (p *BasePage) Title() string {
	return p.title
}

// Theme returns the page theme.
func (p *BasePage) Theme() *themes.Theme {
	return p.theme
}

// App returns the application instance.
func (p *BasePage) App() *app.App {
	return p.app
}

// Width returns the page width.
func (p *BasePage) Width() int {
	return p.width
}

// Height returns the page height.
func (p *BasePage) Height() int {
	return p.height
}

// SetSize updates the page dimensions.
func (p *BasePage) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// Init implements tea.Model (to be overridden).
func (p *BasePage) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model (to be overridden).
func (p *BasePage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return p, nil
}

// View implements tea.Model (to be overridden).
func (p *BasePage) View() string {
	return ""
}

// ShortHelp returns brief keybinding help (to be overridden).
func (p *BasePage) ShortHelp() []app.KeyBinding {
	return nil
}

// FullHelp returns complete keybinding help (to be overridden).
func (p *BasePage) FullHelp() [][]app.KeyBinding {
	return nil
}

// ContentHeight returns the height available for page content.
// Accounts for tab bar and status bar.
func (p *BasePage) ContentHeight() int {
	// Tab bar = 1 line, status bar = 1 line, padding = 2 lines
	return p.height - 4
}

// ContentWidth returns the width available for page content.
func (p *BasePage) ContentWidth() int {
	// Some padding on the sides
	return p.width - 4
}
