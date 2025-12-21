package pages

import (
	"bib/internal/tui/app"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LogsPage shows the log viewer.
type LogsPage struct {
	*BasePage
}

// NewLogsPage creates a new logs page.
func NewLogsPage(application *app.App) *LogsPage {
	return &LogsPage{
		BasePage: NewBasePage("logs", "Logs", application),
	}
}

// Init implements tea.Model.
func (p *LogsPage) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (p *LogsPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return p, nil
}

// View implements tea.Model.
func (p *LogsPage) View() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Theme().Palette.Primary).
		Render("Logs")

	return lipgloss.Place(
		p.Width(),
		p.ContentHeight(),
		lipgloss.Center,
		lipgloss.Center,
		title,
	)
}

// ShortHelp returns brief keybinding help.
func (p *LogsPage) ShortHelp() []app.KeyBinding {
	return []app.KeyBinding{
		{Key: "/", Help: "filter"},
		{Key: "f", Help: "follow"},
		{Key: "c", Help: "clear"},
	}
}

// FullHelp returns complete keybinding help.
func (p *LogsPage) FullHelp() [][]app.KeyBinding {
	return [][]app.KeyBinding{
		{
			{Key: "/", Help: "Filter logs"},
			{Key: "f", Help: "Follow/tail logs"},
			{Key: "c", Help: "Clear log view"},
			{Key: "s", Help: "Save logs to file"},
		},
	}
}
