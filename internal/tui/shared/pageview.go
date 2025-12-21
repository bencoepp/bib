package shared

import (
	"bib/internal/tui/app"

	tea "github.com/charmbracelet/bubbletea"
)

// PageView adapts an app.Page to the layout.ContentView interface.
type PageView struct {
	page   app.Page
	width  int
	height int
}

// NewPageView creates a new PageView wrapper.
func NewPageView(page app.Page) *PageView {
	return &PageView{
		page: page,
	}
}

// ID returns the unique identifier for this view.
func (p *PageView) ID() string {
	return p.page.ID()
}

// Title returns the display title.
func (p *PageView) Title() string {
	return p.page.Title()
}

// ShortTitle returns abbreviated title for small spaces.
func (p *PageView) ShortTitle() string {
	title := p.page.Title()
	if len(title) > 8 {
		return title[:8]
	}
	return title
}

// Icon returns the icon for this view.
func (p *PageView) Icon() string {
	// Return icon based on page ID
	switch p.page.ID() {
	case "dashboard":
		return "◈"
	case "jobs":
		return "⚡"
	case "datasets":
		return "📦"
	case "topics":
		return "📢"
	case "cluster":
		return "🖥️"
	case "network":
		return "🌐"
	case "logs":
		return "📜"
	case "settings":
		return "⚙️"
	default:
		return "•"
	}
}

// SetSize updates the view dimensions.
func (p *PageView) SetSize(width, height int) {
	p.width = width
	p.height = height
	p.page.SetSize(width, height)
}

// Init implements tea.Model.
func (p *PageView) Init() tea.Cmd {
	return p.page.Init()
}

// Update implements tea.Model.
func (p *PageView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	updated, cmd := p.page.Update(msg)
	p.page = updated.(app.Page)
	return p, cmd
}

// View implements tea.Model.
func (p *PageView) View() string {
	return p.page.View()
}

// Page returns the underlying app.Page.
func (p *PageView) Page() app.Page {
	return p.page
}
