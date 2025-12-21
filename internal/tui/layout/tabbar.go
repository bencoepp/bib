package layout

import (
	"strings"

	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TabItem represents a tab in the tab bar
type TabItem struct {
	ID    string
	Title string
	Icon  string
	Badge string
	// Modified indicates unsaved changes
	Modified bool
}

// TabBar provides tab navigation for multiple views
type TabBar struct {
	theme  *themes.Theme
	width  int
	height int
	icons  IconSet

	tabs         []TabItem
	activeTab    int
	scrollOffset int

	// Breadcrumb (optional)
	breadcrumb []string

	focused bool
}

// NewTabBar creates a new tab bar
func NewTabBar() *TabBar {
	return &TabBar{
		theme:  themes.Global().Active(),
		height: 1,
		icons:  GetIcons(),
		tabs:   make([]TabItem, 0),
	}
}

// SetTheme sets the theme
func (t *TabBar) SetTheme(theme *themes.Theme) {
	t.theme = theme
}

// SetSize sets the dimensions
func (t *TabBar) SetSize(width, height int) {
	t.width = width
	t.height = height
}

// SetTabs sets the tab items
func (t *TabBar) SetTabs(tabs []TabItem) {
	t.tabs = tabs
	t.ensureValidActive()
}

// SetActive sets the active tab index
func (t *TabBar) SetActive(index int) {
	if index >= 0 && index < len(t.tabs) {
		t.activeTab = index
		t.ensureActiveVisible()
	}
}

// SetBreadcrumb sets the breadcrumb path
func (t *TabBar) SetBreadcrumb(items []string) {
	t.breadcrumb = items
}

// SetFocused sets the focused state
func (t *TabBar) SetFocused(focused bool) {
	t.focused = focused
}

func (t *TabBar) ensureValidActive() {
	if t.activeTab >= len(t.tabs) {
		t.activeTab = len(t.tabs) - 1
	}
	if t.activeTab < 0 {
		t.activeTab = 0
	}
}

func (t *TabBar) ensureActiveVisible() {
	// Adjust scroll to keep active tab visible
	// This is a simplified version
	if t.activeTab < t.scrollOffset {
		t.scrollOffset = t.activeTab
	}
}

// Init implements tea.Model
func (t *TabBar) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (t *TabBar) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !t.focused {
			return t, nil
		}

		switch msg.String() {
		case "left", "h":
			t.prevTab()
		case "right", "l":
			t.nextTab()
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			idx := int(msg.String()[0] - '1')
			if idx < len(t.tabs) {
				t.activeTab = idx
				return t, t.emitTabChange()
			}
		case "enter":
			return t, t.emitTabChange()
		}
	}

	return t, nil
}

func (t *TabBar) prevTab() {
	if t.activeTab > 0 {
		t.activeTab--
		t.ensureActiveVisible()
	}
}

func (t *TabBar) nextTab() {
	if t.activeTab < len(t.tabs)-1 {
		t.activeTab++
		t.ensureActiveVisible()
	}
}

func (t *TabBar) emitTabChange() tea.Cmd {
	if t.activeTab < len(t.tabs) {
		return func() tea.Msg {
			return ShellMsg{
				Type:   ShellMsgViewChanged,
				ViewID: t.tabs[t.activeTab].ID,
			}
		}
	}
	return nil
}

// ActiveTab returns the active tab index
func (t *TabBar) ActiveTab() int {
	return t.activeTab
}

// ActiveID returns the ID of the active tab
func (t *TabBar) ActiveID() string {
	if t.activeTab < len(t.tabs) {
		return t.tabs[t.activeTab].ID
	}
	return ""
}

// View implements tea.Model
func (t *TabBar) View() string {
	if len(t.tabs) == 0 {
		return ""
	}

	bp := GetBreakpoint(t.width)
	var parts []string

	// Render breadcrumb if present and space allows
	if len(t.breadcrumb) > 0 && bp >= BreakpointMD {
		parts = append(parts, t.renderBreadcrumb())
	}

	// Render tabs
	parts = append(parts, t.renderTabs(bp))

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (t *TabBar) renderBreadcrumb() string {
	sep := t.icons.ChevronRight
	parts := make([]string, 0)

	for i, item := range t.breadcrumb {
		style := lipgloss.NewStyle()
		if i == len(t.breadcrumb)-1 {
			style = style.Foreground(t.theme.Palette.Primary).Bold(true)
		} else {
			style = style.Foreground(t.theme.Palette.TextMuted)
		}
		parts = append(parts, style.Render(item))

		if i < len(t.breadcrumb)-1 {
			parts = append(parts, lipgloss.NewStyle().
				Foreground(t.theme.Palette.TextSubtle).
				Render(" "+sep+" "))
		}
	}

	return strings.Join(parts, "")
}

func (t *TabBar) renderTabs(bp Breakpoint) string {
	var tabs []string
	box := GetBoxChars()

	for i, tab := range t.tabs {
		isActive := i == t.activeTab
		tabs = append(tabs, t.renderTab(tab, isActive, bp))
	}

	// Join tabs
	content := strings.Join(tabs, "")

	// Fill remaining width
	contentWidth := lipgloss.Width(content)
	if contentWidth < t.width {
		padding := strings.Repeat(box.Horizontal, t.width-contentWidth)
		paddingStyle := lipgloss.NewStyle().Foreground(t.theme.Palette.Border)
		content += paddingStyle.Render(padding)
	}

	return content
}

func (t *TabBar) renderTab(tab TabItem, active bool, bp Breakpoint) string {
	box := GetBoxChars()

	// Determine what to show based on breakpoint
	var content string

	if bp >= BreakpointMD {
		// Icon + Title
		if tab.Icon != "" {
			content = tab.Icon + " " + tab.Title
		} else {
			content = tab.Title
		}
	} else {
		// Just title (abbreviated if needed)
		content = tab.Title
		if len(content) > 10 {
			content = content[:8] + "…"
		}
	}

	// Add badge
	if tab.Badge != "" && bp >= BreakpointLG {
		badgeStyle := lipgloss.NewStyle().
			Foreground(t.theme.Palette.Warning).
			Bold(true)
		content += " " + badgeStyle.Render(tab.Badge)
	}

	// Add modified indicator
	if tab.Modified {
		content += " " + t.icons.Bullet
	}

	// Build tab with borders
	var style lipgloss.Style

	if active {
		style = lipgloss.NewStyle().
			Foreground(t.theme.Palette.Primary).
			Bold(true).
			Padding(0, 1)
	} else {
		style = lipgloss.NewStyle().
			Foreground(t.theme.Palette.TextMuted).
			Padding(0, 1)
	}

	tabContent := style.Render(content)

	// Add tab decorations
	if active {
		// Active tab: ─┤ content ├─
		leftDec := lipgloss.NewStyle().Foreground(t.theme.Palette.Border).Render(box.Horizontal + box.RightT)
		rightDec := lipgloss.NewStyle().Foreground(t.theme.Palette.Border).Render(box.LeftT + box.Horizontal)
		return leftDec + tabContent + rightDec
	} else {
		// Inactive tab: ─ content ─
		dec := lipgloss.NewStyle().Foreground(t.theme.Palette.Border).Render(box.Horizontal)
		return dec + tabContent + dec
	}
}
