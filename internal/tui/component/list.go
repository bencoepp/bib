package component

import (
	"strings"

	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ListItem represents an item in the list
type ListItem struct {
	ID          string
	Title       string
	Description string
	Icon        string
	Badge       string
	Data        interface{}
}

// List is an interactive list component
type List struct {
	BaseComponent
	FocusState
	ScrollState

	items         []ListItem
	selectedIndex int
	width         int
	height        int
	showDesc      bool
	showIcons     bool
	showBadges    bool
	filterText    string
	filteredItems []int // Indices into items
}

// NewList creates a new list
func NewList() *List {
	return &List{
		BaseComponent: NewBaseComponent(),
		items:         make([]ListItem, 0),
		filteredItems: make([]int, 0),
		showDesc:      true,
		showIcons:     true,
		showBadges:    true,
	}
}

// WithItems sets the list items
func (l *List) WithItems(items ...ListItem) *List {
	l.items = items
	l.rebuildFilter()
	return l
}

// AddItem adds an item to the list
func (l *List) AddItem(item ListItem) *List {
	l.items = append(l.items, item)
	l.rebuildFilter()
	return l
}

// WithSize sets the list dimensions
func (l *List) WithSize(width, height int) *List {
	l.width = width
	l.height = height
	l.ScrollState.SetMaxOffset(max(0, len(l.filteredItems)-l.visibleItems()))
	return l
}

// WithDescription enables/disables descriptions
func (l *List) WithDescription(show bool) *List {
	l.showDesc = show
	return l
}

// WithIcons enables/disables icons
func (l *List) WithIcons(show bool) *List {
	l.showIcons = show
	return l
}

// WithBadges enables/disables badges
func (l *List) WithBadges(show bool) *List {
	l.showBadges = show
	return l
}

// WithTheme sets the theme
func (l *List) WithTheme(theme *themes.Theme) *List {
	l.SetTheme(theme)
	return l
}

func (l *List) rebuildFilter() {
	l.filteredItems = make([]int, 0)
	filter := strings.ToLower(l.filterText)

	for i, item := range l.items {
		if filter == "" {
			l.filteredItems = append(l.filteredItems, i)
			continue
		}
		// Match against title and description
		if strings.Contains(strings.ToLower(item.Title), filter) ||
			strings.Contains(strings.ToLower(item.Description), filter) {
			l.filteredItems = append(l.filteredItems, i)
		}
	}

	// Adjust selection
	if l.selectedIndex >= len(l.filteredItems) {
		l.selectedIndex = max(0, len(l.filteredItems)-1)
	}

	l.ScrollState.SetMaxOffset(max(0, len(l.filteredItems)-l.visibleItems()))
}

func (l *List) visibleItems() int {
	if l.height <= 0 {
		return len(l.filteredItems)
	}
	itemHeight := 1
	if l.showDesc {
		itemHeight = 2
	}
	return max(1, l.height/itemHeight)
}

// Init implements tea.Model
func (l *List) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (l *List) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !l.Focused() {
		return l, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if l.selectedIndex > 0 {
				l.selectedIndex--
				if l.selectedIndex < l.Offset() {
					l.SetOffset(l.selectedIndex)
				}
			}

		case "down", "j":
			if l.selectedIndex < len(l.filteredItems)-1 {
				l.selectedIndex++
				visibleEnd := l.Offset() + l.visibleItems()
				if l.selectedIndex >= visibleEnd {
					l.SetOffset(l.selectedIndex - l.visibleItems() + 1)
				}
			}

		case "home", "g":
			l.selectedIndex = 0
			l.SetOffset(0)

		case "end", "G":
			l.selectedIndex = len(l.filteredItems) - 1
			l.SetOffset(l.MaxOffset())

		case "pgup":
			l.selectedIndex = max(0, l.selectedIndex-l.visibleItems())
			l.SetOffset(max(0, l.Offset()-l.visibleItems()))

		case "pgdown":
			l.selectedIndex = min(len(l.filteredItems)-1, l.selectedIndex+l.visibleItems())
			l.SetOffset(min(l.MaxOffset(), l.Offset()+l.visibleItems()))

		case "/":
			// TODO: Enter filter mode
		}

	case tea.WindowSizeMsg:
		l.width = msg.Width
		l.height = msg.Height
		l.ScrollState.SetMaxOffset(max(0, len(l.filteredItems)-l.visibleItems()))
	}

	return l, nil
}

// View implements tea.Model
func (l *List) View() string {
	return l.ViewWidth(l.width)
}

// ViewWidth renders the list at a specific width (implements Component)
func (l *List) ViewWidth(width int) string {
	if len(l.filteredItems) == 0 {
		theme := l.Theme()
		return theme.Blurred.Render("No items")
	}

	if width == 0 {
		width = l.width
	}
	if width == 0 {
		width = 60
	}

	theme := l.Theme()
	var lines []string

	startIdx := l.Offset()
	endIdx := min(startIdx+l.visibleItems(), len(l.filteredItems))

	for i := startIdx; i < endIdx; i++ {
		itemIndex := l.filteredItems[i]
		item := l.items[itemIndex]

		// Build item line
		var parts []string

		// Selection indicator
		if i == l.selectedIndex {
			parts = append(parts, theme.Selected.Render(themes.IconTriangleRight))
		} else {
			parts = append(parts, " ")
		}

		// Icon
		if l.showIcons && item.Icon != "" {
			parts = append(parts, item.Icon)
		}

		// Title
		titleStyle := theme.Base
		if i == l.selectedIndex {
			titleStyle = theme.Selected
		}
		parts = append(parts, titleStyle.Render(item.Title))

		// Badge
		if l.showBadges && item.Badge != "" {
			parts = append(parts, theme.BadgeNeutral.Render(item.Badge))
		}

		titleLine := strings.Join(parts, " ")

		// Truncate if needed
		if lipgloss.Width(titleLine) > width {
			titleLine = Truncate(titleLine, width)
		}

		lines = append(lines, titleLine)

		// Description
		if l.showDesc && item.Description != "" {
			descStyle := theme.Description
			desc := "  " + descStyle.Render(item.Description)
			if lipgloss.Width(desc) > width {
				desc = Truncate(desc, width)
			}
			lines = append(lines, desc)
		}
	}

	return strings.Join(lines, "\n")
}

// SelectedIndex returns the currently selected index
func (l *List) SelectedIndex() int {
	return l.selectedIndex
}

// SetSelectedIndex sets the selected index
func (l *List) SetSelectedIndex(index int) {
	l.selectedIndex = clamp(index, 0, len(l.filteredItems)-1)
}

// SelectedItem returns the currently selected item
func (l *List) SelectedItem() *ListItem {
	if l.selectedIndex >= 0 && l.selectedIndex < len(l.filteredItems) {
		itemIndex := l.filteredItems[l.selectedIndex]
		return &l.items[itemIndex]
	}
	return nil
}

// SelectedValue returns the data of the selected item
func (l *List) SelectedValue() interface{} {
	if item := l.SelectedItem(); item != nil {
		return item.Data
	}
	return nil
}

// SetFilter sets the filter text
func (l *List) SetFilter(filter string) {
	l.filterText = filter
	l.rebuildFilter()
}

// ClearFilter clears the filter
func (l *List) ClearFilter() {
	l.filterText = ""
	l.rebuildFilter()
}

// ItemCount returns the total number of items
func (l *List) ItemCount() int {
	return len(l.items)
}

// FilteredCount returns the number of filtered items
func (l *List) FilteredCount() int {
	return len(l.filteredItems)
}

// Focus implements FocusableComponent
func (l *List) Focus() tea.Cmd {
	l.FocusState.Focus()
	return nil
}

// SelectList is a simpler list for selection (like a dropdown)
type SelectList struct {
	BaseComponent
	FocusState

	options       []SelectOption
	selectedIndex int
	width         int
	maxVisible    int
}

// SelectOption represents an option in a select list
type SelectOption struct {
	Label string
	Value interface{}
}

// NewSelectList creates a new select list
func NewSelectList(options ...SelectOption) *SelectList {
	return &SelectList{
		BaseComponent: NewBaseComponent(),
		options:       options,
		maxVisible:    5,
	}
}

// WithOptions sets the options
func (s *SelectList) WithOptions(options ...SelectOption) *SelectList {
	s.options = options
	return s
}

// WithMaxVisible sets the maximum visible options
func (s *SelectList) WithMaxVisible(max int) *SelectList {
	s.maxVisible = max
	return s
}

// WithWidth sets the width
func (s *SelectList) WithWidth(width int) *SelectList {
	s.width = width
	return s
}

// WithTheme sets the theme
func (s *SelectList) WithTheme(theme *themes.Theme) *SelectList {
	s.SetTheme(theme)
	return s
}

// Init implements tea.Model
func (s *SelectList) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (s *SelectList) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !s.Focused() {
		return s, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if s.selectedIndex > 0 {
				s.selectedIndex--
			}
		case "down", "j":
			if s.selectedIndex < len(s.options)-1 {
				s.selectedIndex++
			}
		case "home":
			s.selectedIndex = 0
		case "end":
			s.selectedIndex = len(s.options) - 1
		}
	}

	return s, nil
}

// View implements tea.Model
func (s *SelectList) View() string {
	return s.ViewWidth(s.width)
}

// ViewWidth renders the select list at a specific width (implements Component)
func (s *SelectList) ViewWidth(width int) string {
	if len(s.options) == 0 {
		return ""
	}

	if width == 0 {
		width = s.width
	}
	if width == 0 {
		width = 30
	}

	theme := s.Theme()
	var lines []string

	// Calculate visible range
	startIdx := 0
	endIdx := len(s.options)
	if len(s.options) > s.maxVisible {
		half := s.maxVisible / 2
		startIdx = max(0, s.selectedIndex-half)
		endIdx = min(len(s.options), startIdx+s.maxVisible)
		if endIdx == len(s.options) {
			startIdx = endIdx - s.maxVisible
		}
	}

	for i := startIdx; i < endIdx; i++ {
		opt := s.options[i]
		var style lipgloss.Style
		prefix := "  "

		if i == s.selectedIndex {
			style = theme.Selected
			prefix = theme.Cursor.Render(themes.IconTriangleRight) + " "
		} else {
			style = theme.Base
		}

		line := prefix + style.Render(opt.Label)
		if lipgloss.Width(line) > width {
			line = Truncate(line, width)
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// SelectedIndex returns the selected index
func (s *SelectList) SelectedIndex() int {
	return s.selectedIndex
}

// SetSelectedIndex sets the selected index
func (s *SelectList) SetSelectedIndex(index int) {
	s.selectedIndex = clamp(index, 0, len(s.options)-1)
}

// SelectedValue returns the value of the selected option
func (s *SelectList) SelectedValue() interface{} {
	if s.selectedIndex >= 0 && s.selectedIndex < len(s.options) {
		return s.options[s.selectedIndex].Value
	}
	return nil
}

// SelectedLabel returns the label of the selected option
func (s *SelectList) SelectedLabel() string {
	if s.selectedIndex >= 0 && s.selectedIndex < len(s.options) {
		return s.options[s.selectedIndex].Label
	}
	return ""
}

// Focus implements FocusableComponent
func (s *SelectList) Focus() tea.Cmd {
	s.FocusState.Focus()
	return nil
}
