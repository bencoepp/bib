package tui

import (
	"strings"

	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Tab represents a single tab
type Tab struct {
	ID      string
	Title   string
	Content func(width, height int) string
}

// TabBar is a tab-based navigation component
type TabBar struct {
	theme     *themes.Theme
	tabs      []Tab
	activeTab int
	width     int
	height    int
}

// NewTabBar creates a new tab bar
func NewTabBar(tabs []Tab) *TabBar {
	return &TabBar{
		theme:     themes.Global().Active(),
		tabs:      tabs,
		activeTab: 0,
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
		switch msg.String() {
		case "tab", "right", "l":
			t.activeTab = (t.activeTab + 1) % len(t.tabs)
		case "shift+tab", "left", "h":
			t.activeTab--
			if t.activeTab < 0 {
				t.activeTab = len(t.tabs) - 1
			}
		}

	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height
	}

	return t, nil
}

// View implements tea.Model
func (t *TabBar) View() string {
	var b strings.Builder

	// Render tab headers
	var tabs []string
	for i, tab := range t.tabs {
		var style lipgloss.Style
		if i == t.activeTab {
			style = t.theme.TabActive
		} else {
			style = t.theme.TabInactive
		}
		tabs = append(tabs, style.Render(tab.Title))
	}

	tabLine := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	b.WriteString(t.theme.TabBar.Width(t.width).Render(tabLine))
	b.WriteString("\n\n")

	// Render active tab content
	if t.activeTab >= 0 && t.activeTab < len(t.tabs) {
		tab := t.tabs[t.activeTab]
		if tab.Content != nil {
			b.WriteString(tab.Content(t.width, t.height-4))
		}
	}

	return b.String()
}

// ActiveTab returns the active tab index
func (t *TabBar) ActiveTab() int {
	return t.activeTab
}

// SetActiveTab sets the active tab
func (t *TabBar) SetActiveTab(index int) {
	if index >= 0 && index < len(t.tabs) {
		t.activeTab = index
	}
}

// ActiveTabID returns the ID of the active tab
func (t *TabBar) ActiveTabID() string {
	if t.activeTab >= 0 && t.activeTab < len(t.tabs) {
		return t.tabs[t.activeTab].ID
	}
	return ""
}
