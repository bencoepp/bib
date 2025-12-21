// Package demo provides the demo TUI command showcasing the layout system.
package demo

import (
	"fmt"
	"strings"
	"time"

	"bib/internal/tui/layout"
	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// NewShellDemoCommand returns the shell demo command
func NewShellDemoCommand() *cobra.Command {
	return &cobra.Command{
		Use:         "shell",
		Short:       "demo.shell.short",
		Long:        "demo.shell.long",
		Annotations: map[string]string{"i18n": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			themeName, _ := cmd.Flags().GetString("theme")

			// Set theme based on flag
			switch themeName {
			case "light":
				themes.Global().SetActive(themes.PresetLight)
			case "dracula":
				themes.Global().SetActive(themes.PresetDracula)
			case "nord":
				themes.Global().SetActive(themes.PresetNord)
			case "gruvbox":
				themes.Global().SetActive(themes.PresetGruvbox)
			default:
				themes.Global().SetActive(themes.PresetDark)
			}

			p := tea.NewProgram(newShellDemoModel(), tea.WithAltScreen())
			_, err := p.Run()
			return err
		},
	}
}

// shellDemoModel demonstrates the Shell layout system
type shellDemoModel struct {
	shell    *layout.Shell
	theme    *themes.Theme
	quitting bool

	// Demo state
	logTicker     int
	activityIndex int
}

func newShellDemoModel() *shellDemoModel {
	theme := themes.Global().Active()

	// Create shell with options - enable all panels for demo
	shell := layout.NewShell(
		layout.WithTheme(theme),
		layout.WithInfoBar(true),
		layout.WithLogPanel(true),
	)

	// Add demo views
	shell.AddView(NewDashboardView(theme))
	shell.AddView(NewTopicsView(theme))
	shell.AddView(NewDatasetsView(theme))
	shell.AddView(NewJobsView(theme))
	shell.AddView(NewLogsView(theme))

	// Set sidebar items with badges
	shell.SetSidebarItems([]layout.SidebarItem{
		{ID: "dashboard", Title: "Dashboard", Icon: "◉", Selected: true},
		{ID: "topics", Title: "Topics", Icon: "◎", Badge: "5"},
		{ID: "datasets", Title: "Datasets", Icon: "◎", Badge: "156"},
		{ID: "jobs", Title: "Jobs", Icon: "◎", Badge: "3"},
		{ID: "logs", Title: "Logs", Icon: "◎"},
	})

	shell.SetQuickAccessItems([]layout.SidebarItem{
		{ID: "qa1", Title: "Climate DB", Icon: "★"},
		{ID: "qa2", Title: "Genome v2", Icon: "★"},
	})

	shell.SetRecentItems([]layout.SidebarItem{
		{ID: "r1", Title: "Survey.csv", Icon: "📄"},
		{ID: "r2", Title: "Analysis.bib", Icon: "📄"},
		{ID: "r3", Title: "Results.json", Icon: "📄"},
	})

	// Set initial info bar data
	shell.SetInfoBarData(layout.InfoBarData{
		Username:      "researcher",
		NodeName:      "node-alpha",
		Connected:     true,
		PeerCount:     24,
		UploadSpeed:   "2.4MB/s",
		DownloadSpeed: "890KB/s",
		SyncingItems:  3,
		ShowTime:      true,
		ShowDate:      true,
		CPUPercent:    12,
		MemoryUsage:   "2.4GB",
	})

	// Set status hints
	shell.SetStatusHints(layout.DefaultContentHints())

	// Add initial log entries so log panel has content
	initialLogs := []struct {
		level   layout.LogLevel
		message string
	}{
		{layout.LogLevelInfo, "Bib TUI started"},
		{layout.LogLevelInfo, "Connected to bibd at localhost:9090"},
		{layout.LogLevelDebug, "Loading user preferences..."},
		{layout.LogLevelInfo, "Loaded 5 datasets from cache"},
		{layout.LogLevelInfo, "Peer discovery started on port 9091"},
		{layout.LogLevelDebug, "DHT bootstrap complete"},
		{layout.LogLevelInfo, "Found 24 peers in network"},
		{layout.LogLevelWarn, "Peer node-gamma has high latency (320ms)"},
		{layout.LogLevelInfo, "Sync queue initialized with 3 pending items"},
		{layout.LogLevelDebug, "WebSocket connection established"},
	}

	for _, log := range initialLogs {
		shell.AddLogEntry(layout.LogEntry{
			Time:    time.Now().Add(-time.Duration(len(initialLogs)-1) * time.Second),
			Level:   log.level,
			Message: log.message,
		})
	}

	// Set breadcrumb path
	shell.SetBreadcrumb([]string{"Home", "Projects", "Climate Research"})

	m := &shellDemoModel{
		shell: shell,
		theme: theme,
	}

	return m
}

func (m *shellDemoModel) Init() tea.Cmd {
	return tea.Batch(
		m.shell.Init(),
		m.tickCmd(),
		m.logTickCmd(),
	)
}

func (m *shellDemoModel) tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return shellTickMsg{}
	})
}

func (m *shellDemoModel) logTickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return shellLogTickMsg{}
	})
}

type shellTickMsg struct{}
type shellLogTickMsg struct{}

func (m *shellDemoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		}

	case shellTickMsg:
		// Update info bar with current time
		data := layout.InfoBarData{
			Username:      "researcher",
			NodeName:      "node-alpha",
			Connected:     true,
			PeerCount:     24 + (m.logTicker % 5),
			UploadSpeed:   fmt.Sprintf("%.1fMB/s", 2.0+float64(m.logTicker%10)/10),
			DownloadSpeed: fmt.Sprintf("%dKB/s", 800+m.logTicker%200),
			SyncingItems:  m.logTicker % 5,
			ShowTime:      true,
			ShowDate:      true,
			CPUPercent:    10 + m.logTicker%20,
			MemoryUsage:   "2.4GB",
		}
		m.shell.SetInfoBarData(data)
		cmds = append(cmds, m.tickCmd())

	case shellLogTickMsg:
		// Add demo log entries
		m.logTicker++
		logMessages := []struct {
			level   layout.LogLevel
			message string
		}{
			{layout.LogLevelInfo, "Sync completed for \"Climate Research\" (156 items)"},
			{layout.LogLevelDebug, "Peer node-beta connected"},
			{layout.LogLevelInfo, "Dataset \"temp_2024.csv\" uploaded successfully"},
			{layout.LogLevelWarn, "Peer node-gamma high latency (450ms)"},
			{layout.LogLevelInfo, "Backup task completed"},
			{layout.LogLevelError, "Connection to node-delta timed out"},
			{layout.LogLevelInfo, "Schema validation passed"},
			{layout.LogLevelDebug, "Cache invalidated for dataset alpha"},
		}

		idx := m.activityIndex % len(logMessages)
		m.shell.AddLogEntry(layout.LogEntry{
			Time:    time.Now(),
			Level:   logMessages[idx].level,
			Message: logMessages[idx].message,
		})
		m.activityIndex++
		cmds = append(cmds, m.logTickCmd())
	}

	// Update shell
	newShell, cmd := m.shell.Update(msg)
	m.shell = newShell.(*layout.Shell)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *shellDemoModel) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}
	return m.shell.View()
}

// --- Demo Content Views ---

// DashboardView is the main dashboard view
type DashboardView struct {
	theme  *themes.Theme
	width  int
	height int
}

func NewDashboardView(theme *themes.Theme) *DashboardView {
	return &DashboardView{theme: theme}
}

func (v *DashboardView) ID() string         { return "dashboard" }
func (v *DashboardView) Title() string      { return "Dashboard" }
func (v *DashboardView) ShortTitle() string { return "Dash" }
func (v *DashboardView) Icon() string       { return "◉" }

func (v *DashboardView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

func (v *DashboardView) Init() tea.Cmd { return nil }

func (v *DashboardView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return v, nil
}

func (v *DashboardView) View() string {
	bp := layout.GetBreakpoint(v.width)

	var b strings.Builder

	// Welcome header
	headerStyle := lipgloss.NewStyle().
		Foreground(v.theme.Palette.Primary).
		Bold(true).
		MarginBottom(1)
	b.WriteString(headerStyle.Render("Welcome to Bib Dashboard"))
	b.WriteString("\n\n")

	// Stats cards
	b.WriteString(v.renderStatsCards(bp))
	b.WriteString("\n\n")

	// Section header
	sectionStyle := lipgloss.NewStyle().
		Foreground(v.theme.Palette.Secondary).
		Bold(true)
	b.WriteString(sectionStyle.Render("Quick Stats"))
	b.WriteString("\n\n")

	// Connection status
	connStyle := lipgloss.NewStyle().Foreground(v.theme.Palette.Success)
	b.WriteString(connStyle.Render("● Connected to bibd"))
	b.WriteString("\n\n")

	// Recent activity
	b.WriteString(sectionStyle.Render("Recent Activity"))
	b.WriteString("\n")

	activities := []string{
		"14:32 Sync completed: temp_2024.csv",
		"14:31 Peer node-beta joined",
		"14:30 Dataset schema updated",
		"14:28 New comment on Analysis.bib",
		"14:25 Backup completed",
	}

	activityStyle := lipgloss.NewStyle().Foreground(v.theme.Palette.TextMuted)
	for _, a := range activities {
		b.WriteString(activityStyle.Render("  " + a))
		b.WriteString("\n")
	}

	return b.String()
}

func (v *DashboardView) renderStatsCards(bp layout.Breakpoint) string {
	cards := []struct {
		title string
		value string
		icon  string
	}{
		{"Active Jobs", "3", "⚡"},
		{"Datasets", "156", "📦"},
		{"Cluster Nodes", "4", "🖥️"},
		{"Storage Used", "2.4 TB", "💾"},
	}

	cardWidth := 18
	if bp >= layout.BreakpointLG {
		cardWidth = 22
	}

	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(v.theme.Palette.Border).
		Padding(0, 1).
		Width(cardWidth)

	titleStyle := lipgloss.NewStyle().Foreground(v.theme.Palette.TextMuted)
	valueStyle := lipgloss.NewStyle().Foreground(v.theme.Palette.Primary).Bold(true)
	iconStyle := lipgloss.NewStyle().Foreground(v.theme.Palette.Secondary)

	var renderedCards []string
	for _, card := range cards {
		content := iconStyle.Render(card.icon) + " " + titleStyle.Render(card.title) + "\n" +
			valueStyle.Render(card.value)
		renderedCards = append(renderedCards, cardStyle.Render(content))
	}

	// Layout based on breakpoint
	if bp >= layout.BreakpointLG {
		return lipgloss.JoinHorizontal(lipgloss.Top, renderedCards...)
	} else if bp >= layout.BreakpointMD {
		row1 := lipgloss.JoinHorizontal(lipgloss.Top, renderedCards[0], renderedCards[1])
		row2 := lipgloss.JoinHorizontal(lipgloss.Top, renderedCards[2], renderedCards[3])
		return lipgloss.JoinVertical(lipgloss.Left, row1, row2)
	} else {
		return lipgloss.JoinVertical(lipgloss.Left, renderedCards...)
	}
}

// TopicsView displays topics
type TopicsView struct {
	theme  *themes.Theme
	width  int
	height int
}

func NewTopicsView(theme *themes.Theme) *TopicsView {
	return &TopicsView{theme: theme}
}

func (v *TopicsView) ID() string         { return "topics" }
func (v *TopicsView) Title() string      { return "Topics" }
func (v *TopicsView) ShortTitle() string { return "Topics" }
func (v *TopicsView) Icon() string       { return "◎" }

func (v *TopicsView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

func (v *TopicsView) Init() tea.Cmd                           { return nil }
func (v *TopicsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return v, nil }

func (v *TopicsView) View() string {
	bp := layout.GetBreakpoint(v.width)

	header := lipgloss.NewStyle().
		Foreground(v.theme.Palette.Primary).
		Bold(true).
		Render("Topics")

	// Table data
	topics := []struct {
		name     string
		items    int
		status   string
		lastSync string
		owner    string
	}{
		{"Climate Research", 156, "Published", "2 min ago", "researcher@alpha"},
		{"Genome Project", 42, "Draft", "1 hour ago", "scientist@beta"},
		{"Ocean Survey 2025", 891, "Syncing", "syncing...", "team@gamma"},
		{"Particle Physics", 2401, "Published", "5 min ago", "physics@cern"},
		{"Neural Networks Study", 67, "Private", "3 days ago", "ml@research"},
	}

	// Build table based on breakpoint
	var rows []string

	// Header row
	if bp >= layout.BreakpointLG {
		headerRow := fmt.Sprintf("%-30s %8s %-12s %-12s %-20s",
			"Name", "Items", "Status", "Last Sync", "Owner")
		rows = append(rows, lipgloss.NewStyle().
			Foreground(v.theme.Palette.TextMuted).
			Bold(true).
			Render(headerRow))
	} else if bp >= layout.BreakpointMD {
		headerRow := fmt.Sprintf("%-28s %8s %-10s",
			"Name", "Items", "Status")
		rows = append(rows, lipgloss.NewStyle().
			Foreground(v.theme.Palette.TextMuted).
			Bold(true).
			Render(headerRow))
	}

	// Data rows
	for i, t := range topics {
		var row string
		statusStyle := v.getStatusStyle(t.status)

		namePrefix := "  "
		if i == 0 {
			namePrefix = "▸ "
		}

		if bp >= layout.BreakpointLG {
			row = fmt.Sprintf("%s%-28s %8d %s %-12s %-20s",
				namePrefix, t.name, t.items,
				statusStyle.Render(fmt.Sprintf("%-12s", t.status)),
				t.lastSync, t.owner)
		} else if bp >= layout.BreakpointMD {
			row = fmt.Sprintf("%s%-26s %8d %s",
				namePrefix, t.name, t.items,
				statusStyle.Render(fmt.Sprintf("%-10s", t.status)))
		} else {
			row = fmt.Sprintf("%s%s (%d)",
				namePrefix, t.name, t.items)
		}

		rowStyle := lipgloss.NewStyle().Foreground(v.theme.Palette.Text)
		if i == 0 {
			rowStyle = rowStyle.Foreground(v.theme.Palette.Primary)
		}
		rows = append(rows, rowStyle.Render(row))
	}

	table := strings.Join(rows, "\n")
	return header + "\n\n" + table
}

func (v *TopicsView) getStatusStyle(status string) lipgloss.Style {
	switch status {
	case "Published":
		return lipgloss.NewStyle().Foreground(v.theme.Palette.Success)
	case "Syncing":
		return lipgloss.NewStyle().Foreground(v.theme.Palette.Warning)
	case "Draft":
		return lipgloss.NewStyle().Foreground(v.theme.Palette.Secondary)
	case "Private":
		return lipgloss.NewStyle().Foreground(v.theme.Palette.TextMuted)
	default:
		return lipgloss.NewStyle().Foreground(v.theme.Palette.Text)
	}
}

// DatasetsView displays datasets
type DatasetsView struct {
	theme  *themes.Theme
	width  int
	height int
}

func NewDatasetsView(theme *themes.Theme) *DatasetsView {
	return &DatasetsView{theme: theme}
}

func (v *DatasetsView) ID() string         { return "datasets" }
func (v *DatasetsView) Title() string      { return "Datasets" }
func (v *DatasetsView) ShortTitle() string { return "Data" }
func (v *DatasetsView) Icon() string       { return "📦" }

func (v *DatasetsView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

func (v *DatasetsView) Init() tea.Cmd                           { return nil }
func (v *DatasetsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return v, nil }

func (v *DatasetsView) View() string {
	header := lipgloss.NewStyle().
		Foreground(v.theme.Palette.Primary).
		Bold(true).
		Render("Datasets")

	datasets := []string{
		"📄 temp_2024.csv          124 MB    2 min ago",
		"📄 pressure_readings.db    56 MB    1 hour ago",
		"📄 satellite_img.tar      1.2 GB    3 hours ago",
		"📄 sensor_data.json        12 MB    5 hours ago",
		"📄 climate_model.pkl      890 MB    1 day ago",
	}

	itemStyle := lipgloss.NewStyle().Foreground(v.theme.Palette.Text)
	var items []string
	for _, d := range datasets {
		items = append(items, itemStyle.Render("  "+d))
	}

	return header + "\n\n" + strings.Join(items, "\n")
}

// JobsView displays jobs
type JobsView struct {
	theme  *themes.Theme
	width  int
	height int
}

func NewJobsView(theme *themes.Theme) *JobsView {
	return &JobsView{theme: theme}
}

func (v *JobsView) ID() string         { return "jobs" }
func (v *JobsView) Title() string      { return "Jobs" }
func (v *JobsView) ShortTitle() string { return "Jobs" }
func (v *JobsView) Icon() string       { return "⚡" }

func (v *JobsView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

func (v *JobsView) Init() tea.Cmd                           { return nil }
func (v *JobsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return v, nil }

func (v *JobsView) View() string {
	header := lipgloss.NewStyle().
		Foreground(v.theme.Palette.Primary).
		Bold(true).
		Render("Active Jobs")

	jobs := []struct {
		name     string
		progress int
		status   string
	}{
		{"Ocean Survey Sync", 67, "Running"},
		{"Genome Data Update", 89, "Running"},
		{"Physics Data Import", 100, "Complete"},
		{"Backup Task", 45, "Running"},
	}

	var items []string
	for _, j := range jobs {
		bar := v.renderProgressBar(j.progress, 20)

		var statusStyle lipgloss.Style
		switch j.status {
		case "Complete":
			statusStyle = lipgloss.NewStyle().Foreground(v.theme.Palette.Success)
		case "Running":
			statusStyle = lipgloss.NewStyle().Foreground(v.theme.Palette.Warning)
		default:
			statusStyle = lipgloss.NewStyle().Foreground(v.theme.Palette.TextMuted)
		}

		line := fmt.Sprintf("  %-25s %s %3d%% %s",
			j.name, bar, j.progress, statusStyle.Render(j.status))
		items = append(items, line)
	}

	return header + "\n\n" + strings.Join(items, "\n")
}

func (v *JobsView) renderProgressBar(percent, width int) string {
	filled := (percent * width) / 100
	empty := width - filled

	filledStyle := lipgloss.NewStyle().Foreground(v.theme.Palette.Success)
	emptyStyle := lipgloss.NewStyle().Foreground(v.theme.Palette.Border)

	return filledStyle.Render(strings.Repeat("█", filled)) +
		emptyStyle.Render(strings.Repeat("░", empty))
}

// LogsView displays logs
type LogsView struct {
	theme  *themes.Theme
	width  int
	height int
}

func NewLogsView(theme *themes.Theme) *LogsView {
	return &LogsView{theme: theme}
}

func (v *LogsView) ID() string         { return "logs" }
func (v *LogsView) Title() string      { return "Logs" }
func (v *LogsView) ShortTitle() string { return "Logs" }
func (v *LogsView) Icon() string       { return "📋" }

func (v *LogsView) SetSize(width, height int) {
	v.width = width
	v.height = height
}

func (v *LogsView) Init() tea.Cmd                           { return nil }
func (v *LogsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return v, nil }

func (v *LogsView) View() string {
	header := lipgloss.NewStyle().
		Foreground(v.theme.Palette.Primary).
		Bold(true).
		Render("System Logs")

	hint := lipgloss.NewStyle().
		Foreground(v.theme.Palette.TextMuted).
		Render("Press 'L' to toggle the log panel at the bottom")

	return header + "\n\n" + hint + "\n\n" +
		lipgloss.NewStyle().
			Foreground(v.theme.Palette.TextMuted).
			Render("Real-time logs appear in the log panel.\nUse Ctrl+L or L to toggle visibility.")
}

func init() {
	// Add shell subcommand to demo command
	Cmd.AddCommand(NewShellDemoCommand())
}
