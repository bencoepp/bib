package pages

import (
	"fmt"
	"strings"

	"bib/internal/tui/app"
	"bib/internal/tui/component"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DashboardPage is the main overview page.
type DashboardPage struct {
	*BasePage

	// Components
	statsCards []statsCard
	activity   *component.List
}

type statsCard struct {
	title string
	value string
	icon  string
}

// NewDashboardPage creates a new dashboard page.
func NewDashboardPage(application *app.App) *DashboardPage {
	p := &DashboardPage{
		BasePage: NewBasePage("dashboard", "Dashboard", application),
	}

	// Initialize stats cards
	p.statsCards = []statsCard{
		{title: "Active Jobs", value: "0", icon: "⚡"},
		{title: "Datasets", value: "0", icon: "📦"},
		{title: "Cluster Nodes", value: "1", icon: "🖥️"},
		{title: "Storage Used", value: "0 B", icon: "💾"},
	}

	// Initialize activity list
	p.activity = component.NewList()

	return p
}

// Init implements tea.Model.
func (p *DashboardPage) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (p *DashboardPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			// Refresh dashboard
			return p, p.refresh()
		}

	case app.StateMsg:
		// Update stats from state
		p.updateStats()
	}

	return p, nil
}

func (p *DashboardPage) refresh() tea.Cmd {
	return func() tea.Msg {
		return app.StateMsg{
			Type:       app.StateMsgLoading,
			LoadingMsg: "Refreshing...",
		}
	}
}

func (p *DashboardPage) updateStats() {
	if p.App() == nil {
		return
	}
	state := p.App().State()

	p.statsCards[0].value = fmt.Sprintf("%d", len(state.Jobs))
	p.statsCards[1].value = fmt.Sprintf("%d", len(state.Datasets))
	p.statsCards[2].value = fmt.Sprintf("%d", len(state.ClusterNodes)+1)
}

// View implements tea.Model.
func (p *DashboardPage) View() string {
	theme := p.Theme()
	var b strings.Builder

	// Welcome message
	welcomeStyle := lipgloss.NewStyle().
		Foreground(theme.Palette.Primary).
		Bold(true).
		MarginBottom(1)

	i18n := p.App().I18n()
	b.WriteString(welcomeStyle.Render(i18n.T("dashboard.overview.welcome")))
	b.WriteString("\n\n")

	// Stats cards row
	b.WriteString(p.renderStatsCards())
	b.WriteString("\n\n")

	// Quick stats label
	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Palette.Secondary).
		MarginBottom(1)

	b.WriteString(sectionStyle.Render(i18n.T("dashboard.overview.quick_stats")))
	b.WriteString("\n\n")

	// Connection status
	statusStyle := lipgloss.NewStyle().
		Padding(0, 1)

	state := p.App().State()
	if state.Connected {
		b.WriteString(statusStyle.Copy().
			Foreground(theme.Palette.Success).
			Render("● Connected to bibd"))
	} else {
		b.WriteString(statusStyle.Copy().
			Foreground(theme.Palette.Error).
			Render("○ Not connected"))
	}
	b.WriteString("\n\n")

	// Recent activity section
	b.WriteString(sectionStyle.Render(i18n.T("dashboard.overview.recent_activity")))
	b.WriteString("\n")

	if p.activity != nil {
		b.WriteString(p.activity.ViewWidth(p.ContentWidth()))
	}

	// Pad to fill available height
	content := b.String()
	lines := strings.Count(content, "\n")
	for i := lines; i < p.ContentHeight(); i++ {
		content += "\n"
	}

	return lipgloss.NewStyle().
		Padding(1, 2).
		Render(content)
}

func (p *DashboardPage) renderStatsCards() string {
	theme := p.Theme()

	// Card style
	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Palette.Border).
		Padding(1, 2).
		Width(20)

	iconStyle := lipgloss.NewStyle().
		Foreground(theme.Palette.Primary)

	valueStyle := lipgloss.NewStyle().
		Foreground(theme.Palette.Text).
		Bold(true)

	titleStyle := lipgloss.NewStyle().
		Foreground(theme.Palette.TextMuted)

	var cards []string
	for _, stat := range p.statsCards {
		cardContent := iconStyle.Render(stat.icon) + "\n" +
			valueStyle.Render(stat.value) + "\n" +
			titleStyle.Render(stat.title)

		cards = append(cards, cardStyle.Render(cardContent))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, cards...)
}

// ShortHelp returns brief keybinding help.
func (p *DashboardPage) ShortHelp() []app.KeyBinding {
	return []app.KeyBinding{
		{Key: "r", Help: "refresh"},
		{Key: "j/k", Help: "navigate"},
	}
}

// FullHelp returns complete keybinding help.
func (p *DashboardPage) FullHelp() [][]app.KeyBinding {
	return [][]app.KeyBinding{
		{
			{Key: "r", Help: "Refresh dashboard data"},
			{Key: "j/k", Help: "Navigate activity list"},
			{Key: "enter", Help: "View details"},
		},
	}
}
