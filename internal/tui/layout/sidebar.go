package layout

import (
	"strings"

	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SidebarItem represents an item in the sidebar
type SidebarItem struct {
	ID       string
	Title    string
	Icon     string
	Badge    string
	Selected bool
	Children []SidebarItem
	Expanded bool
}

// SidebarSection represents a section in the sidebar
type SidebarSection struct {
	Title string
	Icon  string
	Items []SidebarItem
}

// Sidebar provides navigation with collapsible sections
type Sidebar struct {
	theme     *themes.Theme
	width     int
	height    int
	collapsed bool
	icons     IconSet

	// App title/logo
	appTitle string
	appIcon  string

	// Navigation items
	mainItems   []SidebarItem
	quickAccess []SidebarItem
	recentItems []SidebarItem

	// Selection
	selectedIdx     int
	selectedSection int // 0=main, 1=quick, 2=recent

	// Scroll
	scrollOffset int

	// Focus
	focused bool
}

// NewSidebar creates a new sidebar
func NewSidebar() *Sidebar {
	return &Sidebar{
		theme:     themes.Global().Active(),
		icons:     GetIcons(),
		appTitle:  "Bib",
		appIcon:   "◈",
		mainItems: make([]SidebarItem, 0),
	}
}

// SetTheme sets the theme
func (s *Sidebar) SetTheme(theme *themes.Theme) {
	s.theme = theme
}

// SetSize sets the dimensions
func (s *Sidebar) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// SetCollapsed sets the collapsed state
func (s *Sidebar) SetCollapsed(collapsed bool) {
	s.collapsed = collapsed
}

// SetItems sets the main navigation items
func (s *Sidebar) SetItems(items []SidebarItem) {
	s.mainItems = items
}

// SetQuickAccess sets the quick access items
func (s *Sidebar) SetQuickAccess(items []SidebarItem) {
	s.quickAccess = items
}

// SetRecentItems sets the recent items
func (s *Sidebar) SetRecentItems(items []SidebarItem) {
	s.recentItems = items
}

// SetFocused sets the focused state
func (s *Sidebar) SetFocused(focused bool) {
	s.focused = focused
}

// Init implements tea.Model
func (s *Sidebar) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (s *Sidebar) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !s.focused {
			return s, nil
		}

		switch msg.String() {
		case "up", "k":
			s.moveSelection(-1)
		case "down", "j":
			s.moveSelection(1)
		case "enter", " ":
			return s, s.selectCurrent()
		case "left", "h":
			s.collapseSelected()
		case "right", "l":
			s.expandSelected()
		}
	}

	return s, nil
}

func (s *Sidebar) moveSelection(delta int) {
	totalItems := len(s.mainItems) + len(s.quickAccess) + len(s.recentItems)
	if totalItems == 0 {
		return
	}

	s.selectedIdx = (s.selectedIdx + delta + totalItems) % totalItems
	s.updateSelectedSection()
}

func (s *Sidebar) updateSelectedSection() {
	idx := s.selectedIdx
	if idx < len(s.mainItems) {
		s.selectedSection = 0
		return
	}
	idx -= len(s.mainItems)
	if idx < len(s.quickAccess) {
		s.selectedSection = 1
		return
	}
	s.selectedSection = 2
}

func (s *Sidebar) selectCurrent() tea.Cmd {
	item := s.getSelectedItem()
	if item == nil {
		return nil
	}

	if len(item.Children) > 0 {
		item.Expanded = !item.Expanded
		return nil
	}

	return func() tea.Msg {
		return ShellMsg{
			Type:   ShellMsgViewChanged,
			ViewID: item.ID,
		}
	}
}

func (s *Sidebar) getSelectedItem() *SidebarItem {
	idx := s.selectedIdx
	if idx < len(s.mainItems) {
		return &s.mainItems[idx]
	}
	idx -= len(s.mainItems)
	if idx < len(s.quickAccess) {
		return &s.quickAccess[idx]
	}
	idx -= len(s.quickAccess)
	if idx < len(s.recentItems) {
		return &s.recentItems[idx]
	}
	return nil
}

func (s *Sidebar) collapseSelected() {
	if item := s.getSelectedItem(); item != nil {
		item.Expanded = false
	}
}

func (s *Sidebar) expandSelected() {
	if item := s.getSelectedItem(); item != nil && len(item.Children) > 0 {
		item.Expanded = true
	}
}

// SelectedID returns the ID of the selected item
func (s *Sidebar) SelectedID() string {
	if item := s.getSelectedItem(); item != nil {
		return item.ID
	}
	return ""
}

// View implements tea.Model - returns content with border
func (s *Sidebar) View() string {
	content := s.ViewContent()
	return s.applyContainerStyle(content)
}

// ViewContent returns just the sidebar content without border
func (s *Sidebar) ViewContent() string {
	if s.collapsed {
		return s.renderCollapsedContent()
	}
	return s.renderExpandedContent()
}

func (s *Sidebar) renderCollapsedContent() string {
	var lines []string

	// App icon
	appStyle := lipgloss.NewStyle().
		Foreground(s.theme.Palette.Primary).
		Bold(true).
		Width(s.width).
		Align(lipgloss.Center)
	lines = append(lines, appStyle.Render(s.appIcon))

	// Separator
	lines = append(lines, s.renderSeparator())

	// Main items (icons only)
	for i, item := range s.mainItems {
		isSelected := s.selectedSection == 0 && i == s.selectedIdx
		lines = append(lines, s.renderCollapsedItem(item, isSelected))
	}

	// Quick access
	if len(s.quickAccess) > 0 {
		lines = append(lines, s.renderSeparator())
		lines = append(lines, s.renderCollapsedSectionIcon(s.icons.Star))
	}

	// Recent
	if len(s.recentItems) > 0 {
		lines = append(lines, s.renderSeparator())
		lines = append(lines, s.renderCollapsedSectionIcon("◷"))
	}

	// Pad to height
	content := strings.Join(lines, "\n")
	return s.padToHeight(content)
}

func (s *Sidebar) renderExpandedContent() string {
	var lines []string

	// App title
	titleStyle := lipgloss.NewStyle().
		Foreground(s.theme.Palette.Primary).
		Bold(true).
		Width(s.width).
		Padding(0, 1)
	lines = append(lines, titleStyle.Render(s.appIcon+" "+s.appTitle))

	// Separator
	lines = append(lines, s.renderSeparator())

	// Main items
	for i, item := range s.mainItems {
		isSelected := s.selectedSection == 0 && i == s.selectedIdx
		lines = append(lines, s.renderExpandedItem(item, isSelected, 0))

		// Render children if expanded
		if item.Expanded {
			for _, child := range item.Children {
				lines = append(lines, s.renderExpandedItem(child, false, 1))
			}
		}
	}

	// Quick access section
	if len(s.quickAccess) > 0 {
		lines = append(lines, s.renderSeparator())
		lines = append(lines, s.renderSectionHeader("Quick Access", s.icons.Star))

		baseIdx := len(s.mainItems)
		for i, item := range s.quickAccess {
			isSelected := s.selectedSection == 1 && (baseIdx+i) == s.selectedIdx
			lines = append(lines, s.renderExpandedItem(item, isSelected, 0))
		}
	}

	// Recent section
	if len(s.recentItems) > 0 {
		lines = append(lines, s.renderSeparator())
		lines = append(lines, s.renderSectionHeader("Recent", "◷"))

		baseIdx := len(s.mainItems) + len(s.quickAccess)
		for i, item := range s.recentItems {
			isSelected := s.selectedSection == 2 && (baseIdx+i) == s.selectedIdx
			lines = append(lines, s.renderExpandedItem(item, isSelected, 0))
		}
	}

	content := strings.Join(lines, "\n")
	return s.padToHeight(content)
}

// padToHeight pads content to fill the sidebar dimensions
func (s *Sidebar) padToHeight(content string) string {
	lines := strings.Split(content, "\n")

	// Ensure each line is the correct width
	for i, line := range lines {
		lineWidth := lipgloss.Width(line)
		if lineWidth < s.width {
			lines[i] = line + strings.Repeat(" ", s.width-lineWidth)
		} else if lineWidth > s.width {
			// Truncate
			lines[i] = line[:s.width]
		}
	}

	// Pad height
	for len(lines) < s.height {
		lines = append(lines, strings.Repeat(" ", s.width))
	}
	// Truncate if too many lines
	if len(lines) > s.height {
		lines = lines[:s.height]
	}
	return strings.Join(lines, "\n")
}

func (s *Sidebar) renderCollapsedItem(item SidebarItem, selected bool) string {
	icon := item.Icon
	if icon == "" {
		icon = s.icons.Circle
	}

	style := lipgloss.NewStyle().
		Width(s.width).
		Align(lipgloss.Center)

	if selected {
		style = style.Foreground(s.theme.Palette.Primary).Bold(true)
	} else {
		style = style.Foreground(s.theme.Palette.Text)
	}

	return style.Render(icon)
}

func (s *Sidebar) renderCollapsedSectionIcon(icon string) string {
	return lipgloss.NewStyle().
		Width(s.width).
		Align(lipgloss.Center).
		Foreground(s.theme.Palette.TextMuted).
		Render(icon)
}

func (s *Sidebar) renderExpandedItem(item SidebarItem, selected bool, indent int) string {
	icon := item.Icon
	if icon == "" {
		if len(item.Children) > 0 {
			if item.Expanded {
				icon = s.icons.TreeExpanded
			} else {
				icon = s.icons.TreeCollapsed
			}
		} else {
			icon = s.icons.Circle
		}
	}

	// Selection indicator
	prefix := "  "
	if selected {
		prefix = s.icons.ChevronRight + " "
	}

	// Indent
	indentStr := strings.Repeat("  ", indent)

	// Build line
	text := prefix + indentStr + icon + " " + item.Title

	// Badge
	if item.Badge != "" {
		text += " " + item.Badge
	}

	// Style
	style := lipgloss.NewStyle().Width(s.width - 2)

	if selected {
		style = style.Foreground(s.theme.Palette.Primary).Bold(true)
	} else if item.Selected {
		style = style.Foreground(s.theme.Palette.Secondary)
	} else {
		style = style.Foreground(s.theme.Palette.Text)
	}

	return style.Render(text)
}

func (s *Sidebar) renderSectionHeader(title, icon string) string {
	style := lipgloss.NewStyle().
		Foreground(s.theme.Palette.TextMuted).
		Bold(true).
		Width(s.width-2).
		Padding(0, 1)

	return style.Render(icon + " " + title)
}

func (s *Sidebar) renderSeparator() string {
	box := GetBoxChars()
	return lipgloss.NewStyle().
		Foreground(s.theme.Palette.Border).
		Render(strings.Repeat(box.Horizontal, s.width))
}

func (s *Sidebar) applyContainerStyle(content string) string {
	// Always use border to prevent layout shift when focus changes
	style := lipgloss.NewStyle().
		Width(s.width).
		Height(s.height).
		BorderStyle(lipgloss.RoundedBorder())

	if s.focused {
		style = style.BorderForeground(s.theme.Palette.Primary)
	} else {
		style = style.BorderForeground(s.theme.Palette.Border)
	}

	return style.Render(content)
}
