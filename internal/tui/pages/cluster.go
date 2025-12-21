package pages

import (
	"bib/internal/tui/app"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ClusterPage shows the cluster status and node management.
type ClusterPage struct {
	*BasePage
}

// NewClusterPage creates a new cluster page.
func NewClusterPage(application *app.App) *ClusterPage {
	return &ClusterPage{
		BasePage: NewBasePage("cluster", "Cluster", application),
	}
}

// Init implements tea.Model.
func (p *ClusterPage) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (p *ClusterPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return p, nil
}

// View implements tea.Model.
func (p *ClusterPage) View() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Theme().Palette.Primary).
		Render("Cluster")

	return lipgloss.Place(
		p.Width(),
		p.ContentHeight(),
		lipgloss.Center,
		lipgloss.Center,
		title,
	)
}

// ShortHelp returns brief keybinding help.
func (p *ClusterPage) ShortHelp() []app.KeyBinding {
	return []app.KeyBinding{
		{Key: "j", Help: "join"},
		{Key: "l", Help: "leave"},
		{Key: "r", Help: "refresh"},
	}
}

// FullHelp returns complete keybinding help.
func (p *ClusterPage) FullHelp() [][]app.KeyBinding {
	return [][]app.KeyBinding{
		{
			{Key: "j", Help: "Join cluster"},
			{Key: "l", Help: "Leave cluster"},
			{Key: "r", Help: "Refresh status"},
			{Key: "enter", Help: "View node details"},
		},
	}
}
