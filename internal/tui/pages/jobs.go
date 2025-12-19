package pages

import (
	"strings"

	"bib/internal/tui/app"
	"bib/internal/tui/component"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// JobsPage shows the jobs list and details.
type JobsPage struct {
	*BasePage

	// Components
	table    *component.Table
	selected int
}

// NewJobsPage creates a new jobs page.
func NewJobsPage(application *app.App) *JobsPage {
	p := &JobsPage{
		BasePage: NewBasePage("jobs", "Jobs", application),
	}

	// Initialize table
	p.table = component.NewTable().
		WithColumns(
			component.TableColumn{Title: "ID", Width: 12},
			component.TableColumn{Title: "Name", Width: 20},
			component.TableColumn{Title: "Status", Width: 12},
			component.TableColumn{Title: "Progress", Width: 10},
			component.TableColumn{Title: "Created", Width: 20},
		)

	return p
}

// Init implements tea.Model.
func (p *JobsPage) Init() tea.Cmd {
	return p.loadJobs()
}

func (p *JobsPage) loadJobs() tea.Cmd {
	return func() tea.Msg {
		// This would actually call the bibd API
		return app.StateMsg{
			Type: app.StateMsgJobsLoaded,
		}
	}
}

// Update implements tea.Model.
func (p *JobsPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			p.selected++
			if state := p.App().State(); p.selected >= len(state.Jobs) {
				p.selected = len(state.Jobs) - 1
			}
			if p.selected < 0 {
				p.selected = 0
			}

		case "k", "up":
			p.selected--
			if p.selected < 0 {
				p.selected = 0
			}

		case "enter":
			// View job details
			if state := p.App().State(); p.selected < len(state.Jobs) {
				// TODO: Show job detail dialog
			}

		case "c":
			// Create new job
			// TODO: Show create job wizard

		case "x":
			// Cancel selected job
			if state := p.App().State(); p.selected < len(state.Jobs) {
				// TODO: Show cancel confirmation
			}

		case "r":
			// Refresh jobs
			return p, p.loadJobs()

		case "l":
			// View logs for selected job
			if state := p.App().State(); p.selected < len(state.Jobs) {
				// TODO: Open logs view
			}
		}

	case app.StateMsg:
		if msg.Type == app.StateMsgJobsLoaded {
			p.updateTable()
		}
	}

	return p, cmd
}

func (p *JobsPage) updateTable() {
	if p.App() == nil || p.table == nil {
		return
	}

	state := p.App().State()

	// Clear and repopulate table
	rows := make([]component.TableRow, len(state.Jobs))
	for i, job := range state.Jobs {
		rows[i] = component.TableRow{
			ID: job.ID.String(),
			Cells: []string{
				job.ID.String()[:8],
				string(job.Type),
				string(job.Status),
				"0%", // TODO: Calculate progress
				job.CreatedAt.Format("2006-01-02 15:04:05"),
			},
		}
	}
	p.table.WithRows(rows...)
}

// View implements tea.Model.
func (p *JobsPage) View() string {
	theme := p.Theme()
	var b strings.Builder

	i18n := p.App().I18n()

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Palette.Primary).
		MarginBottom(1)

	b.WriteString(titleStyle.Render(i18n.T("dashboard.jobs.title")))
	b.WriteString("\n\n")

	state := p.App().State()

	if len(state.Jobs) == 0 {
		// Empty state
		emptyStyle := lipgloss.NewStyle().
			Foreground(theme.Palette.TextMuted).
			Italic(true)
		b.WriteString(emptyStyle.Render(i18n.T("dashboard.jobs.no_jobs")))
	} else {
		// Table - selection is handled internally by the table component
		b.WriteString(p.table.ViewWidth(p.ContentWidth()))
	}

	return lipgloss.NewStyle().
		Padding(1, 2).
		Render(b.String())
}

// ShortHelp returns brief keybinding help.
func (p *JobsPage) ShortHelp() []app.KeyBinding {
	return []app.KeyBinding{
		{Key: "c", Help: "create"},
		{Key: "x", Help: "cancel"},
		{Key: "l", Help: "logs"},
		{Key: "r", Help: "refresh"},
	}
}

// FullHelp returns complete keybinding help.
func (p *JobsPage) FullHelp() [][]app.KeyBinding {
	return [][]app.KeyBinding{
		{
			{Key: "j/k or ↑/↓", Help: "Navigate jobs"},
			{Key: "enter", Help: "View job details"},
			{Key: "c", Help: "Create new job"},
			{Key: "x", Help: "Cancel selected job"},
			{Key: "l", Help: "View job logs"},
			{Key: "r", Help: "Refresh jobs list"},
		},
	}
}
