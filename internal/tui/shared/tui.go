// Package shared provides the shared TUI application that runs in both
// bib CLI and bibd SSH server.
package shared

import (
	"bib/internal/config"
	"bib/internal/tui/app"
	"bib/internal/tui/i18n"
	"bib/internal/tui/layout"
	"bib/internal/tui/pages"
	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
)

// Mode represents the TUI mode.
type Mode int

const (
	// ModeCLI runs in the CLI context (bib tui).
	ModeCLI Mode = iota
	// ModeSSH runs in the SSH server context (bibd).
	ModeSSH
)

// TUI is the shared TUI application model that uses the Shell layout.
type TUI struct {
	// Mode of operation
	mode Mode

	// Configuration
	config *config.BibConfig
	theme  *themes.Theme
	i18n   *i18n.I18n

	// App state
	appState *app.State

	// Shell layout
	shell *layout.Shell

	// Page views (wrapped as ContentViews)
	pageViews []*PageView

	// Dialog overlay
	dialog app.Dialog

	// State
	ready    bool
	quitting bool
	width    int
	height   int

	// SSH context (only used in ModeSSH)
	sshUser   string
	sshPeerID string
}

// Option configures the TUI.
type Option func(*TUI)

// WithMode sets the TUI mode.
func WithMode(mode Mode) Option {
	return func(t *TUI) {
		t.mode = mode
	}
}

// WithConfig sets the configuration.
func WithConfig(cfg *config.BibConfig) Option {
	return func(t *TUI) {
		t.config = cfg
	}
}

// WithTheme sets the theme.
func WithTheme(theme *themes.Theme) Option {
	return func(t *TUI) {
		t.theme = theme
	}
}

// WithI18n sets the i18n instance.
func WithI18n(i *i18n.I18n) Option {
	return func(t *TUI) {
		t.i18n = i
	}
}

// WithSSHContext sets SSH session context.
func WithSSHContext(user, peerID string) Option {
	return func(t *TUI) {
		t.sshUser = user
		t.sshPeerID = peerID
	}
}

// New creates a new shared TUI application.
func New(opts ...Option) *TUI {
	t := &TUI{
		mode:     ModeCLI,
		theme:    themes.Global().Active(),
		i18n:     i18n.Global(),
		appState: app.NewState(),
	}

	for _, opt := range opts {
		opt(t)
	}

	// Create shell with default options
	t.shell = layout.NewShell(
		layout.WithTheme(t.theme),
		layout.WithSidebar(true),
		layout.WithInfoBar(true),
		layout.WithLogPanel(false), // Log panel hidden by default
	)

	// Initialize pages
	t.initializePages()

	// Setup sidebar navigation
	t.setupSidebar()

	return t
}

// initializePages creates and registers all pages as ContentViews.
func (t *TUI) initializePages() {
	// Create a minimal app reference for pages
	// Pages need access to state and theme
	appRef := app.New(
		app.WithConfig(t.config),
		app.WithTheme(t.theme),
		app.WithI18n(t.i18n),
	)

	// Get all pages
	allPages := pages.AllPages(appRef)

	// Wrap pages as ContentViews and add to shell
	for _, page := range allPages {
		pv := NewPageView(page)
		t.pageViews = append(t.pageViews, pv)
		t.shell.AddView(pv)
	}
}

// setupSidebar configures the sidebar navigation items.
func (t *TUI) setupSidebar() {
	items := []layout.SidebarItem{
		{ID: pages.PageDashboard, Title: "Dashboard", Icon: "◈"},
		{ID: pages.PageJobs, Title: "Jobs", Icon: "⚡"},
		{ID: pages.PageDatasets, Title: "Datasets", Icon: "📦"},
		{ID: pages.PageTopics, Title: "Topics", Icon: "📢"},
		{ID: pages.PageCluster, Title: "Cluster", Icon: "🖥️"},
		{ID: pages.PageNetwork, Title: "Network", Icon: "🌐"},
		{ID: pages.PageLogs, Title: "Logs", Icon: "📜"},
		{ID: pages.PageSettings, Title: "Settings", Icon: "⚙️"},
	}

	t.shell.SetSidebarItems(items)
}

// Init implements tea.Model.
func (t *TUI) Init() tea.Cmd {
	return tea.Batch(
		t.shell.Init(),
		t.appState.Connect(),
	)
}

// Update implements tea.Model.
func (t *TUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle quit
		if msg.String() == "ctrl+c" {
			t.quitting = true
			return t, tea.Quit
		}

		// If dialog is open, send keys to dialog first
		if t.dialog != nil {
			dialog, cmd := t.dialog.Update(msg)
			t.dialog = dialog.(app.Dialog)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			if t.dialog.Done() {
				t.dialog = nil
			}
			return t, tea.Batch(cmds...)
		}

		// Global keybindings
		switch msg.String() {
		case "q":
			t.quitting = true
			return t, tea.Quit

		case "?":
			// Show help - could add help dialog here
			return t, nil

		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			// Number keys switch views
			idx := int(msg.String()[0] - '1')
			if idx < len(t.pageViews) {
				t.shell.SetActiveView(t.pageViews[idx].ID())
			}
			return t, nil
		}

	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height
		t.ready = true

		// Update info bar with connection info
		t.updateInfoBar()
	}

	// Forward to shell
	shell, cmd := t.shell.Update(msg)
	t.shell = shell.(*layout.Shell)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return t, tea.Batch(cmds...)
}

// View implements tea.Model.
func (t *TUI) View() string {
	if t.quitting {
		return ""
	}

	if !t.ready {
		return "Loading..."
	}

	content := t.shell.View()

	// Overlay dialog if present
	if t.dialog != nil {
		content = t.overlayDialog(content, t.dialog.View())
	}

	return content
}

// overlayDialog renders a dialog centered over the content.
func (t *TUI) overlayDialog(background, dialog string) string {
	// For now, just center the dialog (Shell already handles full layout)
	return dialog
}

// updateInfoBar updates the info bar with current state.
func (t *TUI) updateInfoBar() {
	data := layout.InfoBarData{
		Connected: t.appState.Connected,
		ShowTime:  true,
	}

	if t.mode == ModeSSH {
		data.Username = t.sshUser
		data.NodeName = t.sshPeerID
	}

	t.shell.SetInfoBarData(data)
}

// Shell returns the shell layout (for advanced customization).
func (t *TUI) Shell() *layout.Shell {
	return t.shell
}

// State returns the application state.
func (t *TUI) State() *app.State {
	return t.appState
}
