package pages

import (
	"bib/internal/tui/app"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DatasetsPage shows the datasets browser.
type DatasetsPage struct {
	*BasePage
}

// NewDatasetsPage creates a new datasets page.
func NewDatasetsPage(application *app.App) *DatasetsPage {
	return &DatasetsPage{
		BasePage: NewBasePage("datasets", "Datasets", application),
	}
}

// Init implements tea.Model.
func (p *DatasetsPage) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (p *DatasetsPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return p, nil
}

// View implements tea.Model.
func (p *DatasetsPage) View() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Theme().Palette.Primary).
		Render("Datasets")

	return lipgloss.Place(
		p.Width(),
		p.ContentHeight(),
		lipgloss.Center,
		lipgloss.Center,
		title,
	)
}

// ShortHelp returns brief keybinding help.
func (p *DatasetsPage) ShortHelp() []app.KeyBinding {
	return []app.KeyBinding{
		{Key: "n", Help: "new"},
		{Key: "d", Help: "delete"},
		{Key: "r", Help: "refresh"},
	}
}

// FullHelp returns complete keybinding help.
func (p *DatasetsPage) FullHelp() [][]app.KeyBinding {
	return [][]app.KeyBinding{
		{
			{Key: "n", Help: "Create new dataset"},
			{Key: "d", Help: "Delete dataset"},
			{Key: "r", Help: "Refresh list"},
			{Key: "enter", Help: "View details"},
		},
	}
}
