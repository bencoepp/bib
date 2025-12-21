package pages

import (
	"bib/internal/tui/app"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SettingsPage shows the settings and preferences.
type SettingsPage struct {
	*BasePage
}

// NewSettingsPage creates a new settings page.
func NewSettingsPage(application *app.App) *SettingsPage {
	return &SettingsPage{
		BasePage: NewBasePage("settings", "Settings", application),
	}
}

// Init implements tea.Model.
func (p *SettingsPage) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (p *SettingsPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return p, nil
}

// View implements tea.Model.
func (p *SettingsPage) View() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Theme().Palette.Primary).
		Render("Settings")

	return lipgloss.Place(
		p.Width(),
		p.ContentHeight(),
		lipgloss.Center,
		lipgloss.Center,
		title,
	)
}

// ShortHelp returns brief keybinding help.
func (p *SettingsPage) ShortHelp() []app.KeyBinding {
	return []app.KeyBinding{
		{Key: "enter", Help: "edit"},
		{Key: "r", Help: "reset"},
		{Key: "s", Help: "save"},
	}
}

// FullHelp returns complete keybinding help.
func (p *SettingsPage) FullHelp() [][]app.KeyBinding {
	return [][]app.KeyBinding{
		{
			{Key: "enter", Help: "Edit setting"},
			{Key: "r", Help: "Reset to default"},
			{Key: "s", Help: "Save changes"},
			{Key: "t", Help: "Toggle theme"},
		},
	}
}
