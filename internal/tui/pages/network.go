package pages

import (
	"bib/internal/tui/app"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// NetworkPage shows the P2P network status and peers.
type NetworkPage struct {
	*BasePage
}

// NewNetworkPage creates a new network page.
func NewNetworkPage(application *app.App) *NetworkPage {
	return &NetworkPage{
		BasePage: NewBasePage("network", "Network", application),
	}
}

// Init implements tea.Model.
func (p *NetworkPage) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (p *NetworkPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return p, nil
}

// View implements tea.Model.
func (p *NetworkPage) View() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Theme().Palette.Primary).
		Render("Network")

	return lipgloss.Place(
		p.Width(),
		p.ContentHeight(),
		lipgloss.Center,
		lipgloss.Center,
		title,
	)
}

// ShortHelp returns brief keybinding help.
func (p *NetworkPage) ShortHelp() []app.KeyBinding {
	return []app.KeyBinding{
		{Key: "c", Help: "connect"},
		{Key: "d", Help: "disconnect"},
		{Key: "r", Help: "refresh"},
	}
}

// FullHelp returns complete keybinding help.
func (p *NetworkPage) FullHelp() [][]app.KeyBinding {
	return [][]app.KeyBinding{
		{
			{Key: "c", Help: "Connect to peer"},
			{Key: "d", Help: "Disconnect peer"},
			{Key: "r", Help: "Refresh peer list"},
			{Key: "enter", Help: "View peer details"},
		},
	}
}
