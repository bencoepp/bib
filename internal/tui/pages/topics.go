package pages

import (
	"bib/internal/tui/app"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TopicsPage shows the topic management interface.
type TopicsPage struct {
	*BasePage
}

// NewTopicsPage creates a new topics page.
func NewTopicsPage(application *app.App) *TopicsPage {
	return &TopicsPage{
		BasePage: NewBasePage("topics", "Topics", application),
	}
}

// Init implements tea.Model.
func (p *TopicsPage) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (p *TopicsPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return p, nil
}

// View implements tea.Model.
func (p *TopicsPage) View() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Theme().Palette.Primary).
		Render("Topics")

	return lipgloss.Place(
		p.Width(),
		p.ContentHeight(),
		lipgloss.Center,
		lipgloss.Center,
		title,
	)
}

// ShortHelp returns brief keybinding help.
func (p *TopicsPage) ShortHelp() []app.KeyBinding {
	return []app.KeyBinding{
		{Key: "n", Help: "new"},
		{Key: "s", Help: "subscribe"},
		{Key: "u", Help: "unsubscribe"},
	}
}

// FullHelp returns complete keybinding help.
func (p *TopicsPage) FullHelp() [][]app.KeyBinding {
	return [][]app.KeyBinding{
		{
			{Key: "n", Help: "Create new topic"},
			{Key: "s", Help: "Subscribe to topic"},
			{Key: "u", Help: "Unsubscribe from topic"},
			{Key: "enter", Help: "View topic details"},
		},
	}
}
