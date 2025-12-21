// Package layout provides the Shell layout system for the TUI.
package layout

import (
	"strings"

	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Region identifies a focusable region in the shell
type Region int

const (
	RegionSidebar Region = iota
	RegionContent
	RegionLogPanel
	RegionTabBar
)

// RegionName returns the name of a region
func RegionName(r Region) string {
	switch r {
	case RegionSidebar:
		return "sidebar"
	case RegionContent:
		return "content"
	case RegionLogPanel:
		return "logs"
	case RegionTabBar:
		return "tabs"
	default:
		return "unknown"
	}
}

// ShellMsg is a message type for shell events
type ShellMsg struct {
	Type   ShellMsgType
	Region Region
	ViewID string
	Data   interface{}
}

// ShellMsgType identifies the type of shell message
type ShellMsgType int

const (
	ShellMsgFocusChanged ShellMsgType = iota
	ShellMsgSidebarToggle
	ShellMsgLogPanelToggle
	ShellMsgInfoBarToggle
	ShellMsgViewChanged
	ShellMsgSidebarResize
	ShellMsgLogPanelResize
)

// ContentView represents a view that can be displayed in the content area
type ContentView interface {
	tea.Model
	// ID returns the unique identifier for this view
	ID() string
	// Title returns the display title
	Title() string
	// ShortTitle returns abbreviated title for small spaces
	ShortTitle() string
	// Icon returns the icon for this view
	Icon() string
	// SetSize updates the view dimensions
	SetSize(width, height int)
}

// Shell is the root layout model implementing the master layout structure
type Shell struct {
	// Theme
	theme *themes.Theme

	// Dimensions
	width  int
	height int

	// Responsive state
	breakpoint  Breakpoint
	layoutMode  LayoutMode
	constraints LayoutConstraints

	// Resize debouncing
	debouncer *ResizeDebouncer

	// Regions
	infoBar   *InfoBar
	sidebar   *Sidebar
	tabBar    *TabBar
	logPanel  *LogPanel
	statusBar *StatusBar

	// Content management
	views      []ContentView
	activeView int

	// Focus management
	focus        Region
	focusHistory []Region

	// Toggles
	showInfoBar  bool
	showLogPanel bool
	showSidebar  bool

	// Sidebar state
	sidebarWidth     int
	sidebarCollapsed bool

	// Log panel state
	logPanelHeight int

	// Ready state
	ready bool
}

// ShellOption configures the Shell
type ShellOption func(*Shell)

// NewShell creates a new Shell layout
func NewShell(opts ...ShellOption) *Shell {
	s := &Shell{
		theme:          themes.Global().Active(),
		breakpoint:     BreakpointMD,
		layoutMode:     LayoutStandard,
		constraints:    DefaultConstraints(),
		debouncer:      DefaultResizeDebouncer(),
		views:          make([]ContentView, 0),
		activeView:     0,
		focus:          RegionContent,
		focusHistory:   make([]Region, 0),
		showInfoBar:    true,
		showLogPanel:   false,
		showSidebar:    true,
		sidebarWidth:   20,
		logPanelHeight: 6,
	}

	// Initialize components
	s.infoBar = NewInfoBar()
	s.sidebar = NewSidebar()
	s.tabBar = NewTabBar()
	s.logPanel = NewLogPanel()
	s.statusBar = NewStatusBar()

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// WithTheme sets the theme
func WithTheme(theme *themes.Theme) ShellOption {
	return func(s *Shell) {
		s.theme = theme
		s.infoBar.SetTheme(theme)
		s.sidebar.SetTheme(theme)
		s.tabBar.SetTheme(theme)
		s.logPanel.SetTheme(theme)
		s.statusBar.SetTheme(theme)
	}
}

// WithInfoBar enables/disables the info bar
func WithInfoBar(show bool) ShellOption {
	return func(s *Shell) {
		s.showInfoBar = show
	}
}

// WithLogPanel enables/disables the log panel
func WithLogPanel(show bool) ShellOption {
	return func(s *Shell) {
		s.showLogPanel = show
	}
}

// WithSidebar enables/disables the sidebar
func WithSidebar(show bool) ShellOption {
	return func(s *Shell) {
		s.showSidebar = show
	}
}

// Init implements tea.Model
func (s *Shell) Init() tea.Cmd {
	var cmds []tea.Cmd

	// Initialize all components
	if s.infoBar != nil {
		cmds = append(cmds, s.infoBar.Init())
	}
	if s.sidebar != nil {
		cmds = append(cmds, s.sidebar.Init())
	}
	if s.tabBar != nil {
		cmds = append(cmds, s.tabBar.Init())
	}
	if s.logPanel != nil {
		cmds = append(cmds, s.logPanel.Init())
	}
	if s.statusBar != nil {
		cmds = append(cmds, s.statusBar.Init())
	}

	// Initialize active view
	if len(s.views) > 0 && s.activeView < len(s.views) {
		cmds = append(cmds, s.views[s.activeView].Init())
	}

	return tea.Batch(cmds...)
}

// Update implements tea.Model
func (s *Shell) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Handle first resize immediately, debounce subsequent ones
		if !s.ready {
			s.handleResize(msg.Width, msg.Height)
			return s, nil
		}
		// Debounce subsequent resizes
		return s, s.debouncer.Debounce(msg.Width, msg.Height)

	case DebouncedResizeMsg:
		s.handleResize(msg.Width, msg.Height)

	case tea.KeyMsg:
		cmd := s.handleKeyMsg(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case ShellMsg:
		cmd := s.handleShellMsg(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// Route messages to focused region
	cmd := s.routeMessage(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return s, tea.Batch(cmds...)
}

// handleResize processes a resize event
func (s *Shell) handleResize(width, height int) {
	s.width = width
	s.height = height
	s.ready = true

	// Update breakpoint and layout mode
	s.breakpoint = GetBreakpoint(width)
	s.layoutMode = GetLayoutMode(s.breakpoint)
	s.constraints = ConstraintsForBreakpoint(s.breakpoint)

	// Adjust sidebar based on breakpoint
	s.adjustLayoutForBreakpoint()

	// Recalculate component sizes
	s.recalculateSizes()
}

// adjustLayoutForBreakpoint modifies layout based on current breakpoint
func (s *Shell) adjustLayoutForBreakpoint() {
	switch s.layoutMode {
	case LayoutMinimal:
		// Hide everything except content
		s.showSidebar = false
		s.showLogPanel = false
		s.showInfoBar = false
		s.sidebarCollapsed = true

	case LayoutCompact:
		// Collapsed sidebar, no log panel, show info bar
		s.showSidebar = true
		s.showLogPanel = false
		s.showInfoBar = true
		s.sidebarCollapsed = true
		s.sidebarWidth = s.constraints.SidebarCollapsed

	case LayoutStandard, LayoutExtended, LayoutWide, LayoutUltrawide:
		// Full sidebar, info bar, log panel based on user preference
		s.showSidebar = true
		s.sidebarCollapsed = false
		s.sidebarWidth = s.constraints.SidebarDefaultWidth
		s.showInfoBar = true
		// Keep showLogPanel as user set it (don't override)
	}
}

// recalculateSizes updates all component dimensions
func (s *Shell) recalculateSizes() {
	// Calculate available height
	contentHeight := s.height

	if s.showInfoBar {
		contentHeight -= s.constraints.InfoBarHeight
	}
	contentHeight -= s.constraints.StatusBarHeight

	if len(s.views) > 1 {
		contentHeight -= s.constraints.TabBarHeight
	}

	if s.showLogPanel {
		contentHeight -= s.logPanelHeight
	}

	// Calculate content width
	contentWidth := s.width
	if s.showSidebar {
		contentWidth -= s.sidebarWidth
	}

	// Update components
	if s.infoBar != nil {
		s.infoBar.SetSize(s.width, s.constraints.InfoBarHeight)
	}

	if s.sidebar != nil {
		sidebarHeight := s.height
		if s.showInfoBar {
			sidebarHeight -= s.constraints.InfoBarHeight
		}
		sidebarHeight -= s.constraints.StatusBarHeight
		s.sidebar.SetSize(s.sidebarWidth, sidebarHeight)
		s.sidebar.SetCollapsed(s.sidebarCollapsed)
	}

	if s.tabBar != nil {
		s.tabBar.SetSize(contentWidth, s.constraints.TabBarHeight)
	}

	if s.statusBar != nil {
		s.statusBar.SetSize(s.width, s.constraints.StatusBarHeight)
	}

	if s.logPanel != nil {
		logWidth := contentWidth
		s.logPanel.SetSize(logWidth, s.logPanelHeight)
	}

	// Update active view
	if len(s.views) > 0 && s.activeView < len(s.views) {
		s.views[s.activeView].SetSize(contentWidth, contentHeight)
	}
}

// handleKeyMsg processes keyboard input
func (s *Shell) handleKeyMsg(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "tab":
		s.cycleFocus(1)
		return nil

	case "shift+tab":
		s.cycleFocus(-1)
		return nil

	case "ctrl+1":
		if s.showSidebar {
			s.setFocus(RegionSidebar)
		}
		return nil

	case "ctrl+2":
		s.setFocus(RegionContent)
		return nil

	case "ctrl+3":
		if s.showLogPanel {
			s.setFocus(RegionLogPanel)
		}
		return nil

	case "ctrl+b":
		// Toggle sidebar
		s.toggleSidebar()
		return nil

	case "ctrl+l", "L":
		// Toggle log panel
		s.toggleLogPanel()
		return nil

	case "ctrl+i", "I":
		// Toggle info bar
		s.toggleInfoBar()
		return nil
	}

	return nil
}

// handleShellMsg processes shell-specific messages
func (s *Shell) handleShellMsg(msg ShellMsg) tea.Cmd {
	switch msg.Type {
	case ShellMsgFocusChanged:
		s.setFocus(msg.Region)

	case ShellMsgSidebarToggle:
		s.toggleSidebar()

	case ShellMsgLogPanelToggle:
		s.toggleLogPanel()

	case ShellMsgInfoBarToggle:
		s.toggleInfoBar()

	case ShellMsgViewChanged:
		if msg.ViewID != "" {
			for i, v := range s.views {
				if v.ID() == msg.ViewID {
					s.activeView = i
					s.recalculateSizes()
					return s.views[i].Init()
				}
			}
		}

	case ShellMsgSidebarResize:
		if delta, ok := msg.Data.(int); ok {
			s.resizeSidebar(delta)
		}

	case ShellMsgLogPanelResize:
		if delta, ok := msg.Data.(int); ok {
			s.resizeLogPanel(delta)
		}
	}

	return nil
}

// routeMessage routes a message to the appropriate component
func (s *Shell) routeMessage(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	// Always update info bar and status bar (they show global state)
	if s.infoBar != nil {
		newInfoBar, cmd := s.infoBar.Update(msg)
		s.infoBar = newInfoBar.(*InfoBar)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	if s.statusBar != nil {
		newStatusBar, cmd := s.statusBar.Update(msg)
		s.statusBar = newStatusBar.(*StatusBar)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// Route to focused region
	switch s.focus {
	case RegionSidebar:
		if s.sidebar != nil {
			newSidebar, cmd := s.sidebar.Update(msg)
			s.sidebar = newSidebar.(*Sidebar)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case RegionContent:
		if len(s.views) > 0 && s.activeView < len(s.views) {
			newView, cmd := s.views[s.activeView].Update(msg)
			s.views[s.activeView] = newView.(ContentView)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case RegionLogPanel:
		if s.logPanel != nil {
			newLogPanel, cmd := s.logPanel.Update(msg)
			s.logPanel = newLogPanel.(*LogPanel)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case RegionTabBar:
		if s.tabBar != nil {
			newTabBar, cmd := s.tabBar.Update(msg)
			s.tabBar = newTabBar.(*TabBar)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	return tea.Batch(cmds...)
}

// setFocus changes focus to a region
func (s *Shell) setFocus(region Region) {
	if s.focus != region {
		s.focusHistory = append(s.focusHistory, s.focus)
		s.focus = region
	}
}

// cycleFocus moves focus to the next/previous region
func (s *Shell) cycleFocus(direction int) {
	regions := s.availableRegions()
	if len(regions) == 0 {
		return
	}

	currentIdx := 0
	for i, r := range regions {
		if r == s.focus {
			currentIdx = i
			break
		}
	}

	newIdx := (currentIdx + direction + len(regions)) % len(regions)
	s.setFocus(regions[newIdx])
}

// availableRegions returns regions that can receive focus
func (s *Shell) availableRegions() []Region {
	regions := make([]Region, 0)

	if s.showSidebar && !s.sidebarCollapsed {
		regions = append(regions, RegionSidebar)
	}

	regions = append(regions, RegionContent)

	if s.showLogPanel {
		regions = append(regions, RegionLogPanel)
	}

	return regions
}

// toggleSidebar toggles sidebar visibility/collapse state
func (s *Shell) toggleSidebar() {
	if s.layoutMode == LayoutMinimal {
		return // Can't show sidebar in minimal mode
	}

	if s.sidebarCollapsed {
		s.sidebarCollapsed = false
		s.sidebarWidth = s.constraints.SidebarDefaultWidth
	} else {
		s.sidebarCollapsed = true
		s.sidebarWidth = s.constraints.SidebarCollapsed
	}

	s.recalculateSizes()
}

// toggleLogPanel toggles log panel visibility
func (s *Shell) toggleLogPanel() {
	s.showLogPanel = !s.showLogPanel
	s.recalculateSizes()
}

// toggleInfoBar toggles info bar visibility
func (s *Shell) toggleInfoBar() {
	s.showInfoBar = !s.showInfoBar
	s.recalculateSizes()
}

// resizeSidebar adjusts sidebar width
func (s *Shell) resizeSidebar(delta int) {
	newWidth := s.sidebarWidth + delta
	if newWidth >= s.constraints.SidebarMinWidth && newWidth <= s.constraints.SidebarMaxWidth {
		s.sidebarWidth = newWidth
		s.recalculateSizes()
	}
}

// resizeLogPanel adjusts log panel height
func (s *Shell) resizeLogPanel(delta int) {
	newHeight := s.logPanelHeight + delta
	maxHeight := (s.height * s.constraints.LogPanelMaxHeight) / 100

	if newHeight >= s.constraints.LogPanelMinHeight && newHeight <= maxHeight {
		s.logPanelHeight = newHeight
		s.recalculateSizes()
	}
}

// View implements tea.Model
func (s *Shell) View() string {
	if !s.ready {
		return "Initializing..."
	}

	// Use height - 1 to account for potential terminal scrolling
	viewHeight := s.height
	if viewHeight < 3 {
		viewHeight = 3
	}

	// Build output lines array with exact height
	output := make([]string, viewHeight)
	box := GetBoxChars()

	// Calculate line positions
	currentLine := 0

	// Row 1: Info bar (1 line)
	if s.showInfoBar && s.infoBar != nil {
		s.infoBar.SetSize(s.width, s.constraints.InfoBarHeight)
		output[currentLine] = s.renderFixedWidthLine(s.infoBar.View(), s.width)
		currentLine++

		// Divider after info bar
		output[currentLine] = lipgloss.NewStyle().
			Foreground(s.theme.Palette.Border).
			Render(strings.Repeat(box.Horizontal, s.width))
		currentLine++
	}

	// Reserve last 2 rows: Divider + Status bar
	statusDividerLine := viewHeight - 2
	statusBarLine := viewHeight - 1

	// Safety check
	if statusDividerLine < currentLine {
		statusDividerLine = currentLine
	}
	if statusBarLine <= statusDividerLine {
		statusBarLine = statusDividerLine + 1
	}
	if statusBarLine >= viewHeight {
		statusBarLine = viewHeight - 1
	}
	if statusDividerLine >= viewHeight {
		statusDividerLine = viewHeight - 2
	}

	// Status bar divider
	if statusDividerLine >= 0 && statusDividerLine < viewHeight {
		output[statusDividerLine] = lipgloss.NewStyle().
			Foreground(s.theme.Palette.Border).
			Render(strings.Repeat(box.Horizontal, s.width))
	}

	// Status bar
	if statusBarLine >= 0 && statusBarLine < viewHeight {
		if s.statusBar != nil {
			s.statusBar.SetSize(s.width, s.constraints.StatusBarHeight)
			output[statusBarLine] = s.renderFixedWidthLine(s.statusBar.View(), s.width)
		} else {
			output[statusBarLine] = strings.Repeat(" ", s.width)
		}
	}

	// Middle: Main area (fills remaining space)
	mainHeight := statusDividerLine - currentLine
	if mainHeight < 1 {
		mainHeight = 1
	}

	mainArea := s.renderMainArea(mainHeight)
	mainLines := strings.Split(mainArea, "\n")

	// Truncate if we got more lines than expected
	if len(mainLines) > mainHeight {
		mainLines = mainLines[:mainHeight]
	}

	// Fill main area lines
	for i := 0; i < mainHeight && currentLine+i < statusDividerLine; i++ {
		if i < len(mainLines) {
			output[currentLine+i] = s.renderFixedWidthLine(mainLines[i], s.width)
		} else {
			output[currentLine+i] = strings.Repeat(" ", s.width)
		}
	}

	return strings.Join(output, "\n")
}

// renderFixedWidthLine ensures a line is exactly the given width
func (s *Shell) renderFixedWidthLine(line string, width int) string {
	// Remove any newlines
	line = strings.Split(line, "\n")[0]

	lineWidth := lipgloss.Width(line)
	if lineWidth < width {
		return line + strings.Repeat(" ", width-lineWidth)
	} else if lineWidth > width {
		return truncateString(line, width)
	}
	return line
}

// renderMainArea renders the sidebar and content area side by side
func (s *Shell) renderMainArea(height int) string {
	// Sidebar
	var sidebarView string
	sidebarWidth := 0
	if s.showSidebar && s.sidebar != nil {
		sidebarWidth = s.sidebarWidth
		s.sidebar.SetFocused(s.focus == RegionSidebar)
		sidebarView = s.renderSidebarPanel(sidebarWidth, height)
	}

	// Content area width
	contentWidth := s.width - sidebarWidth

	// Content area (tab bar + content + log panel)
	contentArea := s.renderContentArea(contentWidth, height)

	var result string
	if sidebarView == "" {
		result = contentArea
	} else {
		result = lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, contentArea)
	}

	// Ensure exact dimensions
	return constrainToSize(result, s.width, height)
}

// renderSidebarPanel renders the sidebar with proper sizing
func (s *Shell) renderSidebarPanel(width, height int) string {
	if s.sidebar == nil {
		return ""
	}

	// Account for border (2 chars for left+right, 2 for top+bottom)
	innerWidth := width - 2
	innerHeight := height - 2

	if innerWidth < 1 {
		innerWidth = 1
	}
	if innerHeight < 1 {
		innerHeight = 1
	}

	s.sidebar.SetSize(innerWidth, innerHeight)
	content := s.sidebar.ViewContent()

	// Apply border - lipgloss adds border outside the content
	style := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder())

	if s.focus == RegionSidebar {
		style = style.BorderForeground(s.theme.Palette.Primary)
	} else {
		style = style.BorderForeground(s.theme.Palette.Border)
	}

	rendered := style.Render(content)

	// Ensure exact dimensions (border adds 2 to each dimension)
	return constrainToSize(rendered, width, height)
}

// renderContentArea renders tab bar, content, and log panel stacked vertically
func (s *Shell) renderContentArea(width, height int) string {
	var sections []string

	// Account for content border
	innerWidth := width - 2
	remainingHeight := height

	// Tab bar (if multiple views)
	if len(s.views) > 1 && s.tabBar != nil {
		s.updateTabBar()
		s.tabBar.SetSize(width, s.constraints.TabBarHeight)
		sections = append(sections, s.tabBar.View())
		remainingHeight -= s.constraints.TabBarHeight
	}

	// Log panel height (with border)
	logPanelTotalHeight := 0
	if s.showLogPanel && s.logPanel != nil {
		logPanelTotalHeight = s.logPanelHeight
		remainingHeight -= logPanelTotalHeight
	}

	// Content area (with border: -2 for top+bottom)
	contentInnerHeight := remainingHeight - 2
	if contentInnerHeight < 1 {
		contentInnerHeight = 1
	}

	contentView := s.renderContentPanel(innerWidth, contentInnerHeight)
	sections = append(sections, contentView)

	// Log panel
	if s.showLogPanel && s.logPanel != nil {
		logInnerHeight := logPanelTotalHeight - 2
		if logInnerHeight < 1 {
			logInnerHeight = 1
		}
		logView := s.renderLogPanelView(innerWidth, logInnerHeight)
		sections = append(sections, logView)
	}

	result := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Ensure exact height
	return constrainToSize(result, width, height)
}

// renderContentPanel renders the active content view with border
func (s *Shell) renderContentPanel(width, height int) string {
	var content string

	if len(s.views) == 0 || s.activeView >= len(s.views) {
		content = lipgloss.NewStyle().
			Width(width).
			Height(height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(s.theme.Palette.TextMuted).
			Render("No content")
	} else {
		s.views[s.activeView].SetSize(width, height)
		viewContent := s.views[s.activeView].View()

		// Constrain the content to the allocated size
		content = constrainToSize(viewContent, width, height)
	}

	// Apply border - don't set Width/Height as border adds to size
	style := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder())

	if s.focus == RegionContent {
		style = style.BorderForeground(s.theme.Palette.Primary)
	} else {
		style = style.BorderForeground(s.theme.Palette.Border)
	}

	return style.Render(content)
}

// constrainToSize ensures content fits within the given dimensions
func constrainToSize(content string, width, height int) string {
	lines := strings.Split(content, "\n")

	// Ensure each line fits the width
	for i, line := range lines {
		lineWidth := lipgloss.Width(line)
		if lineWidth < width {
			lines[i] = line + strings.Repeat(" ", width-lineWidth)
		} else if lineWidth > width {
			// Truncate with ellipsis
			if width > 1 {
				lines[i] = truncateString(line, width)
			} else {
				lines[i] = ""
			}
		}
	}

	// Ensure correct number of lines
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	if len(lines) > height {
		lines = lines[:height]
	}

	return strings.Join(lines, "\n")
}

// truncateString truncates a string to fit within maxWidth
func truncateString(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}

	// Simple truncation - could be improved for multi-byte chars
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes)) > maxWidth-1 {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

// renderLogPanelView renders the log panel with border
func (s *Shell) renderLogPanelView(width, height int) string {
	if s.logPanel == nil {
		return ""
	}

	s.logPanel.SetSize(width, height)
	s.logPanel.SetFocused(s.focus == RegionLogPanel)
	content := s.logPanel.ViewContent()

	// Apply border - don't set Width/Height as border adds to size
	style := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder())

	if s.focus == RegionLogPanel {
		style = style.BorderForeground(s.theme.Palette.Primary)
	} else {
		style = style.BorderForeground(s.theme.Palette.Border)
	}

	return style.Render(content)
}

// updateTabBar syncs tab bar with current views
func (s *Shell) updateTabBar() {
	if s.tabBar == nil {
		return
	}

	tabs := make([]TabItem, len(s.views))
	for i, v := range s.views {
		tabs[i] = TabItem{
			ID:    v.ID(),
			Title: v.Title(),
			Icon:  v.Icon(),
		}
	}

	s.tabBar.SetTabs(tabs)
	s.tabBar.SetActive(s.activeView)
}

// --- Public API ---

// AddView adds a content view to the shell
func (s *Shell) AddView(view ContentView) {
	s.views = append(s.views, view)
	s.recalculateSizes()
}

// RemoveView removes a view by ID
func (s *Shell) RemoveView(id string) {
	for i, v := range s.views {
		if v.ID() == id {
			s.views = append(s.views[:i], s.views[i+1:]...)
			if s.activeView >= len(s.views) {
				s.activeView = len(s.views) - 1
			}
			if s.activeView < 0 {
				s.activeView = 0
			}
			s.recalculateSizes()
			return
		}
	}
}

// SetActiveView sets the active view by ID
func (s *Shell) SetActiveView(id string) bool {
	for i, v := range s.views {
		if v.ID() == id {
			s.activeView = i
			s.recalculateSizes()
			return true
		}
	}
	return false
}

// ActiveView returns the currently active view
func (s *Shell) ActiveView() ContentView {
	if len(s.views) > 0 && s.activeView < len(s.views) {
		return s.views[s.activeView]
	}
	return nil
}

// SetSidebarItems sets the sidebar navigation items
func (s *Shell) SetSidebarItems(items []SidebarItem) {
	if s.sidebar != nil {
		s.sidebar.SetItems(items)
	}
}

// SetQuickAccessItems sets the quick access section
func (s *Shell) SetQuickAccessItems(items []SidebarItem) {
	if s.sidebar != nil {
		s.sidebar.SetQuickAccess(items)
	}
}

// SetRecentItems sets the recent items section
func (s *Shell) SetRecentItems(items []SidebarItem) {
	if s.sidebar != nil {
		s.sidebar.SetRecentItems(items)
	}
}

// AddLogEntry adds an entry to the log panel
func (s *Shell) AddLogEntry(entry LogEntry) {
	if s.logPanel != nil {
		s.logPanel.AddEntry(entry)
	}
}

// SetInfoBarData updates info bar data
func (s *Shell) SetInfoBarData(data InfoBarData) {
	if s.infoBar != nil {
		s.infoBar.SetData(data)
	}
}

// SetStatusHints sets the status bar hints
func (s *Shell) SetStatusHints(hints []StatusHint) {
	if s.statusBar != nil {
		s.statusBar.SetHints(hints)
	}
}

// SetStatusMessage sets a status message
func (s *Shell) SetStatusMessage(msg string) {
	if s.statusBar != nil {
		s.statusBar.SetMessage(msg)
	}
}

// SetBreadcrumb sets the tab bar breadcrumb path
func (s *Shell) SetBreadcrumb(items []string) {
	if s.tabBar != nil {
		s.tabBar.SetBreadcrumb(items)
	}
}

// Focus returns the currently focused region
func (s *Shell) Focus() Region {
	return s.focus
}

// Breakpoint returns the current breakpoint
func (s *Shell) Breakpoint() Breakpoint {
	return s.breakpoint
}

// LayoutMode returns the current layout mode
func (s *Shell) LayoutMode() LayoutMode {
	return s.layoutMode
}

// Width returns the shell width
func (s *Shell) Width() int {
	return s.width
}

// Height returns the shell height
func (s *Shell) Height() int {
	return s.height
}

// ContentWidth returns the width available for content
func (s *Shell) ContentWidth() int {
	w := s.width
	if s.showSidebar {
		w -= s.sidebarWidth
	}
	return w
}

// ContentHeight returns the height available for content
func (s *Shell) ContentHeight() int {
	h := s.height
	if s.showInfoBar {
		h -= s.constraints.InfoBarHeight
	}
	h -= s.constraints.StatusBarHeight
	if len(s.views) > 1 {
		h -= s.constraints.TabBarHeight
	}
	if s.showLogPanel {
		h -= s.logPanelHeight
	}
	return h
}

// --- Helper for padding views ---

// PadToSize pads a view to fill the given dimensions
func PadToSize(content string, width, height int) string {
	lines := strings.Split(content, "\n")

	// Pad width
	for i, line := range lines {
		lineWidth := lipgloss.Width(line)
		if lineWidth < width {
			lines[i] = line + strings.Repeat(" ", width-lineWidth)
		}
	}

	// Pad height
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}

	return strings.Join(lines, "\n")
}
