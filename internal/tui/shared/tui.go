// Package shared provides the shared TUI application that runs in both
// bib CLI and bibd SSH server.
package shared

import (
	"context"
	"time"

	"bib/internal/config"
	"bib/internal/grpc/client"
	"bib/internal/tui/app"
	"bib/internal/tui/i18n"
	"bib/internal/tui/layout"
	"bib/internal/tui/pages"
	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Mode represents the TUI mode.
type Mode int

const (
	// ModeCLI runs in the CLI context (bib tui).
	ModeCLI Mode = iota
	// ModeSSH runs in the SSH server context (bibd).
	ModeSSH
)

// Connection messages
type (
	// ConnectingMsg indicates connection is in progress
	ConnectingMsg struct{}

	// ConnectedMsg indicates successful connection
	ConnectedMsg struct {
		Client *client.Client
	}

	// ConnectionErrorMsg indicates connection failed
	ConnectionErrorMsg struct {
		Err error
	}

	// RetryConnectionMsg triggers a connection retry
	RetryConnectionMsg struct{}
)

// TUI is the shared TUI application model that uses the Shell layout.
type TUI struct {
	// Mode of operation
	mode Mode

	// Configuration
	config        *config.BibConfig
	theme         *themes.Theme
	i18n          *i18n.I18n
	clientOptions *client.Options

	// gRPC client
	client *client.Client

	// App state
	appState *app.State

	// Shell layout
	shell *layout.Shell

	// Page views (wrapped as ContentViews)
	pageViews []*PageView

	// Dialog overlay
	dialog app.Dialog

	// Connection state
	connecting      bool
	connected       bool
	connectionError string

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

// WithClient sets an existing gRPC client.
func WithClient(c *client.Client) Option {
	return func(t *TUI) {
		t.client = c
		if c != nil {
			t.connected = c.IsConnected()
		}
	}
}

// WithClientOptions sets the client connection options.
func WithClientOptions(opts *client.Options) Option {
	return func(t *TUI) {
		t.clientOptions = opts
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
	cmds := []tea.Cmd{t.shell.Init()}

	// If we don't have a client, start connecting
	if t.client == nil {
		t.connecting = true
		cmds = append(cmds, t.connectCmd())
	} else {
		t.connected = t.client.IsConnected()
		t.updatePagesWithClient()
	}

	return tea.Batch(cmds...)
}

func (t *TUI) connectCmd() tea.Cmd {
	return func() tea.Msg {
		// Build options if not provided
		opts := t.clientOptions
		if opts == nil {
			defaultOpts := client.DefaultOptions()
			opts = &defaultOpts

			// Apply config settings if available
			if t.config != nil {
				connCfg := t.config.Connection
				if connCfg.Timeout != "" {
					if timeout, err := time.ParseDuration(connCfg.Timeout); err == nil {
						opts.Timeout = timeout
					}
				}
				if connCfg.RetryAttempts > 0 {
					opts.RetryAttempts = connCfg.RetryAttempts
				}
				// Apply TLS settings
				opts.TLS.InsecureSkipVerify = connCfg.TLS.SkipVerify
				opts.TLS.CAFile = connCfg.TLS.CAFile
			}
		}

		// Create client
		c, err := client.New(*opts)
		if err != nil {
			return ConnectionErrorMsg{Err: err}
		}

		// Connect with timeout
		ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
		defer cancel()

		if err := c.Connect(ctx); err != nil {
			return ConnectionErrorMsg{Err: err}
		}

		return ConnectedMsg{Client: c}
	}
}

func (t *TUI) updatePagesWithClient() {
	// Update dashboard page with client
	for _, pv := range t.pageViews {
		if pv.ID() == pages.PageDashboard {
			if dp, ok := pv.Page().(*pages.DashboardPage); ok {
				dp.SetClient(t.client)
			}
		}
	}
}

// Update implements tea.Model.
func (t *TUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case ConnectedMsg:
		t.connecting = false
		t.connected = true
		t.connectionError = ""
		t.client = msg.Client
		t.appState.Connected = true
		t.updatePagesWithClient()
		t.updateInfoBar()
		return t, nil

	case ConnectionErrorMsg:
		t.connecting = false
		t.connected = false
		t.connectionError = msg.Err.Error()
		t.appState.Connected = false
		t.updateInfoBar()
		return t, nil

	case RetryConnectionMsg:
		t.connecting = true
		t.connectionError = ""
		return t, t.connectCmd()

	case tea.KeyMsg:
		// Handle quit
		if msg.String() == "ctrl+c" {
			t.quitting = true
			if t.client != nil {
				t.client.Close()
			}
			return t, tea.Quit
		}

		// If showing connection dialog, handle its keys
		if !t.connected && !t.connecting {
			switch msg.String() {
			case "r":
				// Retry connection
				return t, func() tea.Msg { return RetryConnectionMsg{} }
			case "q":
				t.quitting = true
				return t, tea.Quit
			}
			return t, nil
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
			if t.client != nil {
				t.client.Close()
			}
			return t, tea.Quit

		case "?":
			return t, nil

		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
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

	// Show connection dialog if not connected
	if !t.connected {
		return t.renderConnectionDialog()
	}

	content := t.shell.View()

	// Overlay dialog if present
	if t.dialog != nil {
		content = t.overlayDialog(content, t.dialog.View())
	}

	return content
}

// renderConnectionDialog renders the connection status/error dialog
func (t *TUI) renderConnectionDialog() string {
	boxWidth := 50
	boxHeight := 10

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.theme.Palette.Primary)

	mutedStyle := lipgloss.NewStyle().
		Foreground(t.theme.Palette.TextMuted)

	errorStyle := lipgloss.NewStyle().
		Foreground(t.theme.Palette.Error)

	var content string
	if t.connecting {
		content = titleStyle.Render("Connecting to bibd...") + "\n\n" +
			mutedStyle.Render("Please wait...")
	} else {
		// Connection error
		content = titleStyle.Render("Connection Failed") + "\n\n" +
			errorStyle.Render(t.truncateError(t.connectionError, boxWidth-4)) + "\n\n" +
			mutedStyle.Render("[r] retry  [q] quit")
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.theme.Palette.Primary).
		Width(boxWidth).
		Height(boxHeight).
		Padding(1, 2).
		Align(lipgloss.Center, lipgloss.Center).
		Render(content)

	// Place box in center
	return lipgloss.Place(t.width, t.height, lipgloss.Center, lipgloss.Center, box)
}

func (t *TUI) truncateError(err string, maxLen int) string {
	if len(err) <= maxLen {
		return err
	}
	return err[:maxLen-3] + "..."
}

// overlayDialog renders a dialog centered over the content.
func (t *TUI) overlayDialog(background, dialog string) string {
	return dialog
}

// updateInfoBar updates the info bar with current state.
func (t *TUI) updateInfoBar() {
	data := layout.InfoBarData{
		Connected: t.connected,
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

// Client returns the gRPC client.
func (t *TUI) Client() *client.Client {
	return t.client
}
