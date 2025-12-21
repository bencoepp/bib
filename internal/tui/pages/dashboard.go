package pages

import (
	"context"
	"fmt"
	"strings"
	"time"

	services "bib/api/gen/go/bib/v1/services"
	"bib/internal/grpc/client"
	"bib/internal/tui/app"
	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Dashboard configuration
const (
	dashboardRefreshInterval = 5 * time.Second
	maxActivityEntries       = 6
	maxJobsDisplay           = 3
	maxTransfersDisplay      = 3
)

// TickMsg is sent periodically to refresh the dashboard
type TickMsg time.Time

// DataRefreshedMsg is sent when data has been fetched from the server
type DataRefreshedMsg struct {
	Data DashboardData
	Err  error
}

// DashboardPage is the main overview page.
type DashboardPage struct {
	*BasePage

	// gRPC client
	client *client.Client

	// Data
	data DashboardData

	// UI State
	showConnectionDialog bool
	connectionError      string
}

// DashboardData holds all dashboard display data
type DashboardData struct {
	// Node Status
	NodeID      string
	NodeMode    string // "full-replica", "selective", "proxy"
	ClusterRole string // "leader", "follower", "standalone"
	IsConnected bool
	PeerCount   int
	Version     string

	// Stats
	ActiveJobs       int
	QueuedJobs       int
	TotalDatasets    int64
	SubscribedTopics int64
	StorageUsed      int64
	StorageTotal     int64

	// Network
	DownloadSpeed int64 // bytes/sec (calculated from delta)
	UploadSpeed   int64 // bytes/sec (calculated from delta)

	// Active Jobs (limited) - empty until JobService is implemented
	RunningJobs []JobSummary

	// Active Transfers - empty until transfer tracking is implemented
	Downloads []TransferSummary
	Uploads   []TransferSummary

	// Recent Activity - empty until event system is implemented
	ActivityLog []ActivityEntry

	// Last refresh
	LastUpdated time.Time
}

// JobSummary represents a job for display
type JobSummary struct {
	ID       string
	Name     string
	Type     string
	Status   string
	Progress float64
	ETA      time.Duration
}

// TransferSummary represents a transfer for display
type TransferSummary struct {
	DatasetName string
	Progress    float64
	Speed       int64
	PeerID      string
	Direction   string
}

// ActivityEntry represents an activity log entry
type ActivityEntry struct {
	Time    time.Time
	Type    string
	Icon    string
	Message string
}

// NewDashboardPage creates a new dashboard page.
func NewDashboardPage(application *app.App) *DashboardPage {
	p := &DashboardPage{
		BasePage: NewBasePage("dashboard", "Dashboard", application),
		data: DashboardData{
			NodeMode:    "unknown",
			ClusterRole: "standalone",
			LastUpdated: time.Now(),
		},
	}
	return p
}

// SetClient sets the gRPC client for data fetching
func (p *DashboardPage) SetClient(c *client.Client) {
	p.client = c
	if c != nil {
		p.data.IsConnected = c.IsConnected()
	}
}

// Init implements tea.Model.
func (p *DashboardPage) Init() tea.Cmd {
	return tea.Batch(
		p.tickCmd(),
		p.fetchDataCmd(),
	)
}

func (p *DashboardPage) tickCmd() tea.Cmd {
	return tea.Tick(dashboardRefreshInterval, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func (p *DashboardPage) fetchDataCmd() tea.Cmd {
	return func() tea.Msg {
		data := p.fetchData()
		return DataRefreshedMsg{Data: data, Err: nil}
	}
}

func (p *DashboardPage) fetchData() DashboardData {
	data := DashboardData{
		LastUpdated: time.Now(),
		NodeMode:    "unknown",
		ClusterRole: "standalone",
	}

	if p.client == nil || !p.client.IsConnected() {
		data.IsConnected = false
		return data
	}

	data.IsConnected = true

	// Fetch node info from HealthService
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	healthSvc, err := p.client.Health()
	if err != nil {
		return data
	}

	// Get node info with network and storage details
	nodeInfo, err := healthSvc.GetNodeInfo(ctx, &services.GetNodeInfoRequest{
		IncludeNetwork: true,
		IncludeStorage: true,
	})
	if err != nil {
		return data
	}

	// Populate data from response
	data.NodeID = nodeInfo.GetNodeId()
	data.NodeMode = nodeInfo.GetMode()
	data.Version = nodeInfo.GetVersion()

	if network := nodeInfo.GetNetwork(); network != nil {
		data.PeerCount = int(network.GetConnectedPeers())
		// Note: Speed would need delta calculation over time
	}

	if storage := nodeInfo.GetStorage(); storage != nil {
		data.TotalDatasets = storage.GetDatasetCount()
		data.SubscribedTopics = storage.GetTopicCount()
		data.StorageUsed = storage.GetBytesUsed()
		data.StorageTotal = storage.GetBytesUsed() + storage.GetBytesAvailable()
	}

	if cluster := nodeInfo.GetCluster(); cluster != nil {
		data.ClusterRole = cluster.GetRole()
	}

	return data
}

// Update implements tea.Model.
func (p *DashboardPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TickMsg:
		return p, tea.Batch(p.tickCmd(), p.fetchDataCmd())

	case DataRefreshedMsg:
		p.data = msg.Data
		return p, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			return p, p.fetchDataCmd()
		case "n":
			// TODO: Navigate to new job wizard
			return p, nil
		case "s":
			// TODO: Open search
			return p, nil
		case "j":
			return p, func() tea.Msg { return app.NavigateMsg{PageID: "jobs"} }
		case "d":
			return p, func() tea.Msg { return app.NavigateMsg{PageID: "datasets"} }
		case "t":
			return p, func() tea.Msg { return app.NavigateMsg{PageID: "topics"} }
		}

	case app.StateMsg:
		if msg.Type == app.StateMsgConnected {
			p.data.IsConnected = msg.Connected
		}
	}

	return p, nil
}

// View implements tea.Model.
func (p *DashboardPage) View() string {
	theme := p.Theme()
	width := p.Width()
	height := p.ContentHeight()

	if width < 40 {
		width = 80
	}
	if height < 10 {
		height = 24
	}

	// Content width accounting for padding
	contentWidth := width - 4

	var sections []string

	// 1. Status Header
	sections = append(sections, p.renderStatusHeader(contentWidth))

	// 2. Stats Cards Row
	sections = append(sections, p.renderStatsCards(contentWidth))

	// 3. Main panels (Jobs + Transfers)
	sections = append(sections, p.renderMainPanels(contentWidth))

	// 4. Activity Log
	sections = append(sections, p.renderActivityLog(contentWidth))

	// 5. Quick Actions
	sections = append(sections, p.renderQuickActions(contentWidth))

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Padding(1, 2).
		Foreground(theme.Palette.Text).
		Render(content)
}

func (p *DashboardPage) renderStatusHeader(width int) string {
	theme := p.Theme()

	var parts []string

	// Connection status
	if p.data.IsConnected {
		parts = append(parts, lipgloss.NewStyle().
			Foreground(theme.Palette.Success).
			Render("● Connected"))
	} else {
		parts = append(parts, lipgloss.NewStyle().
			Foreground(theme.Palette.Error).
			Render("○ Disconnected"))
	}

	sep := lipgloss.NewStyle().Foreground(theme.Palette.TextMuted).Render(" │ ")

	if p.data.NodeMode != "" && p.data.NodeMode != "unknown" {
		parts = append(parts, sep+p.data.NodeMode)
	}

	if p.data.ClusterRole != "" {
		parts = append(parts, sep+p.data.ClusterRole)
	}

	parts = append(parts, sep+fmt.Sprintf("%d peers", p.data.PeerCount))

	statusLine := strings.Join(parts, "")

	// Truncate if too long
	if lipgloss.Width(statusLine) > width {
		statusLine = truncateWithEllipsis(statusLine, width)
	}

	return lipgloss.NewStyle().
		Width(width).
		MarginBottom(1).
		Render(statusLine)
}

func (p *DashboardPage) renderStatsCards(width int) string {
	theme := p.Theme()

	numCards := 5
	gap := 1
	totalGaps := (numCards - 1) * gap
	cardWidth := (width - totalGaps) / numCards
	if cardWidth < 10 {
		cardWidth = 10
	}

	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Palette.Border).
		Width(cardWidth-2).
		Height(3).
		Align(lipgloss.Center, lipgloss.Center)

	valueStyle := lipgloss.NewStyle().Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(theme.Palette.TextMuted)

	type cardData struct {
		icon  string
		value string
		label string
	}

	cards := []cardData{
		{"⚡", fmt.Sprintf("%d", p.data.ActiveJobs), "jobs"},
		{"📦", fmt.Sprintf("%d", p.data.TotalDatasets), "datasets"},
		{"📁", fmt.Sprintf("%d", p.data.SubscribedTopics), "topics"},
		{"🌐", fmt.Sprintf("%d", p.data.PeerCount), "peers"},
		{"💾", formatBytes(p.data.StorageUsed), "storage"},
	}

	var renderedCards []string
	for _, c := range cards {
		content := fmt.Sprintf("%s %s\n%s",
			c.icon,
			valueStyle.Render(c.value),
			labelStyle.Render(c.label))
		renderedCards = append(renderedCards, cardStyle.Render(content))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, renderedCards...)
}

func (p *DashboardPage) renderMainPanels(width int) string {
	// Two panels side by side
	panelWidth := (width - 2) / 2

	jobsPanel := p.renderJobsPanel(panelWidth)
	transfersPanel := p.renderTransfersPanel(panelWidth)

	return lipgloss.JoinHorizontal(lipgloss.Top, jobsPanel, " ", transfersPanel)
}

func (p *DashboardPage) renderJobsPanel(width int) string {
	theme := p.Theme()
	innerWidth := width - 4

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Palette.Primary)

	header := headerStyle.Render(fmt.Sprintf("ACTIVE JOBS (%d)", p.data.ActiveJobs))

	var content string
	if len(p.data.RunningJobs) == 0 {
		// Empty state
		content = p.renderEmptyState(innerWidth, 4, "No active jobs", "[n] create job")
	} else {
		var lines []string
		for i, job := range p.data.RunningJobs {
			if i >= maxJobsDisplay {
				lines = append(lines, lipgloss.NewStyle().
					Foreground(theme.Palette.TextMuted).
					Render(fmt.Sprintf("... +%d more", len(p.data.RunningJobs)-maxJobsDisplay)))
				break
			}
			lines = append(lines, p.renderJobLine(job, innerWidth))
		}
		content = strings.Join(lines, "\n")
	}

	panelContent := header + "\n" + content

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Palette.Border).
		Width(width-2).
		Padding(0, 1).
		MarginTop(1).
		Render(panelContent)
}

func (p *DashboardPage) renderJobLine(job JobSummary, width int) string {
	theme := p.Theme()

	var iconStyle lipgloss.Style
	var icon string
	switch job.Status {
	case "running":
		icon = "⚡"
		iconStyle = lipgloss.NewStyle().Foreground(theme.Palette.Success)
	case "queued":
		icon = "⏸"
		iconStyle = lipgloss.NewStyle().Foreground(theme.Palette.Warning)
	default:
		icon = "○"
		iconStyle = lipgloss.NewStyle().Foreground(theme.Palette.TextMuted)
	}

	// Calculate available space for name
	progressWidth := 8
	percentWidth := 5
	etaWidth := 4
	fixedWidth := 2 + progressWidth + percentWidth + etaWidth + 4 // icon + spaces
	nameWidth := width - fixedWidth
	if nameWidth < 8 {
		nameWidth = 8
	}

	name := truncateWithEllipsis(job.Name, nameWidth)
	progress := renderProgressBar(job.Progress, progressWidth, theme)

	var eta string
	if job.ETA > 0 {
		if job.ETA >= time.Minute {
			eta = fmt.Sprintf("%dm", int(job.ETA.Minutes()))
		} else {
			eta = fmt.Sprintf("%ds", int(job.ETA.Seconds()))
		}
	}

	return fmt.Sprintf("%s %-*s %s %3.0f%% %s",
		iconStyle.Render(icon),
		nameWidth, name,
		progress,
		job.Progress*100,
		eta)
}

func (p *DashboardPage) renderTransfersPanel(width int) string {
	theme := p.Theme()
	innerWidth := width - 4

	totalTransfers := len(p.data.Downloads) + len(p.data.Uploads)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Palette.Primary)

	header := headerStyle.Render(fmt.Sprintf("TRANSFERS (%d)", totalTransfers))

	var content string
	if totalTransfers == 0 {
		content = p.renderEmptyState(innerWidth, 4, "No active transfers", "[t] browse topics")
	} else {
		var lines []string
		count := 0
		for _, dl := range p.data.Downloads {
			if count >= maxTransfersDisplay {
				break
			}
			lines = append(lines, p.renderTransferLine(dl, innerWidth, "⬇"))
			count++
		}
		for _, ul := range p.data.Uploads {
			if count >= maxTransfersDisplay*2 {
				break
			}
			lines = append(lines, p.renderTransferLine(ul, innerWidth, "⬆"))
			count++
		}
		content = strings.Join(lines, "\n")
	}

	panelContent := header + "\n" + content

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Palette.Border).
		Width(width-2).
		Padding(0, 1).
		MarginTop(1).
		Render(panelContent)
}

func (p *DashboardPage) renderTransferLine(t TransferSummary, width int, icon string) string {
	theme := p.Theme()

	var iconStyle lipgloss.Style
	if t.Progress >= 1.0 {
		iconStyle = lipgloss.NewStyle().Foreground(theme.Palette.Success)
	} else {
		iconStyle = lipgloss.NewStyle().Foreground(theme.Palette.Primary)
	}

	progressWidth := 8
	percentWidth := 5
	speedWidth := 10
	fixedWidth := 2 + progressWidth + percentWidth + speedWidth + 4
	nameWidth := width - fixedWidth
	if nameWidth < 8 {
		nameWidth = 8
	}

	name := truncateWithEllipsis(t.DatasetName, nameWidth)
	progress := renderProgressBar(t.Progress, progressWidth, theme)

	var speedStr string
	if t.Progress >= 1.0 {
		speedStr = lipgloss.NewStyle().Foreground(theme.Palette.Success).Render("Done!")
	} else if t.Speed > 0 {
		speedStr = formatSpeed(t.Speed)
	}

	return fmt.Sprintf("%s %-*s %s %3.0f%% %s",
		iconStyle.Render(icon),
		nameWidth, name,
		progress,
		t.Progress*100,
		speedStr)
}

func (p *DashboardPage) renderActivityLog(width int) string {
	theme := p.Theme()
	innerWidth := width - 4

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Palette.Primary)

	header := headerStyle.Render("RECENT ACTIVITY")

	var content string
	if len(p.data.ActivityLog) == 0 {
		content = p.renderEmptyState(innerWidth, 3, "No recent activity", "")
	} else {
		var lines []string
		for i, entry := range p.data.ActivityLog {
			if i >= maxActivityEntries {
				break
			}
			lines = append(lines, p.renderActivityLine(entry, innerWidth))
		}
		content = strings.Join(lines, "\n")
	}

	panelContent := header + "\n" + content

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Palette.Border).
		Width(width-2).
		Padding(0, 1).
		MarginTop(1).
		Render(panelContent)
}

func (p *DashboardPage) renderActivityLine(entry ActivityEntry, width int) string {
	theme := p.Theme()

	timeStr := entry.Time.Format("15:04")
	timeStyle := lipgloss.NewStyle().Foreground(theme.Palette.TextMuted)

	var iconStyle lipgloss.Style
	switch entry.Type {
	case "job_complete", "sync_complete", "download_complete":
		iconStyle = lipgloss.NewStyle().Foreground(theme.Palette.Success)
	case "job_failed":
		iconStyle = lipgloss.NewStyle().Foreground(theme.Palette.Error)
	default:
		iconStyle = lipgloss.NewStyle().Foreground(theme.Palette.Primary)
	}

	// Calculate message width
	fixedWidth := 5 + 2 + 2 // time + icon + spaces
	msgWidth := width - fixedWidth
	if msgWidth < 10 {
		msgWidth = 10
	}

	msg := truncateWithEllipsis(entry.Message, msgWidth)

	return fmt.Sprintf("%s %s %s",
		timeStyle.Render(timeStr),
		iconStyle.Render(entry.Icon),
		msg)
}

func (p *DashboardPage) renderEmptyState(width, height int, message, hint string) string {
	theme := p.Theme()

	msgStyle := lipgloss.NewStyle().
		Foreground(theme.Palette.TextMuted)

	hintStyle := lipgloss.NewStyle().
		Foreground(theme.Palette.Primary)

	content := msgStyle.Render(message)
	if hint != "" {
		content += "\n" + hintStyle.Render(hint)
	}

	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(content)
}

func (p *DashboardPage) renderQuickActions(width int) string {
	theme := p.Theme()

	keyStyle := lipgloss.NewStyle().
		Foreground(theme.Palette.Primary).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(theme.Palette.TextMuted)

	actions := []struct{ key, desc string }{
		{"n", "new"},
		{"s", "search"},
		{"r", "refresh"},
		{"j", "jobs"},
		{"d", "datasets"},
		{"t", "topics"},
		{"?", "help"},
	}

	var parts []string
	for _, a := range actions {
		parts = append(parts, keyStyle.Render("["+a.key+"]")+descStyle.Render(a.desc))
	}

	line := strings.Join(parts, "  ")

	// Truncate if too wide
	if lipgloss.Width(line) > width {
		line = truncateWithEllipsis(line, width)
	}

	return lipgloss.NewStyle().
		MarginTop(1).
		Width(width).
		Render(line)
}

// Helper functions

func truncateWithEllipsis(s string, maxLen int) string {
	if maxLen <= 3 {
		return s[:maxLen]
	}
	if lipgloss.Width(s) <= maxLen {
		return s
	}
	// Simple rune-based truncation
	runes := []rune(s)
	if len(runes) > maxLen-1 {
		return string(runes[:maxLen-1]) + "…"
	}
	return s
}

func renderProgressBar(progress float64, width int, theme *themes.Theme) string {
	if width < 2 {
		width = 2
	}

	filled := int(progress * float64(width))
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)

	var style lipgloss.Style
	if progress >= 1.0 {
		style = lipgloss.NewStyle().Foreground(theme.Palette.Success)
	} else if progress >= 0.5 {
		style = lipgloss.NewStyle().Foreground(theme.Palette.Primary)
	} else {
		style = lipgloss.NewStyle().Foreground(theme.Palette.Warning)
	}

	return style.Render(bar)
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatSpeed(bytesPerSec int64) string {
	if bytesPerSec < 1024 {
		return fmt.Sprintf("%dB/s", bytesPerSec)
	} else if bytesPerSec < 1024*1024 {
		return fmt.Sprintf("%.0fKB/s", float64(bytesPerSec)/1024)
	}
	return fmt.Sprintf("%.1fMB/s", float64(bytesPerSec)/(1024*1024))
}

// ShortHelp returns brief keybinding help.
func (p *DashboardPage) ShortHelp() []app.KeyBinding {
	return []app.KeyBinding{
		{Key: "n", Help: "new job"},
		{Key: "s", Help: "search"},
		{Key: "r", Help: "refresh"},
	}
}

// FullHelp returns complete keybinding help.
func (p *DashboardPage) FullHelp() [][]app.KeyBinding {
	return [][]app.KeyBinding{
		{
			{Key: "n", Help: "Create new job"},
			{Key: "s", Help: "Search datasets"},
			{Key: "r", Help: "Refresh dashboard"},
		},
		{
			{Key: "j", Help: "Go to Jobs"},
			{Key: "d", Help: "Go to Datasets"},
			{Key: "t", Help: "Go to Topics"},
		},
	}
}
