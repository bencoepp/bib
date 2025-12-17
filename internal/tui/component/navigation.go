package component

import (
	"strings"

	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Breadcrumb displays a navigation path
type Breadcrumb struct {
	BaseComponent

	items     []string
	separator string
	maxWidth  int
}

// NewBreadcrumb creates a new breadcrumb
func NewBreadcrumb(items ...string) *Breadcrumb {
	return &Breadcrumb{
		BaseComponent: NewBaseComponent(),
		items:         items,
		separator:     themes.IconChevronRight,
	}
}

// WithItems sets the breadcrumb items
func (b *Breadcrumb) WithItems(items ...string) *Breadcrumb {
	b.items = items
	return b
}

// AddItem adds an item to the path
func (b *Breadcrumb) AddItem(item string) *Breadcrumb {
	b.items = append(b.items, item)
	return b
}

// WithSeparator sets the separator character
func (b *Breadcrumb) WithSeparator(sep string) *Breadcrumb {
	b.separator = sep
	return b
}

// WithMaxWidth sets max width (truncates if exceeded)
func (b *Breadcrumb) WithMaxWidth(width int) *Breadcrumb {
	b.maxWidth = width
	return b
}

// WithTheme sets the theme
func (b *Breadcrumb) WithTheme(theme *themes.Theme) *Breadcrumb {
	b.SetTheme(theme)
	return b
}

// View implements Component
func (b *Breadcrumb) View(width int) string {
	if len(b.items) == 0 {
		return ""
	}

	theme := b.Theme()
	var parts []string

	for i, item := range b.items {
		isLast := i == len(b.items)-1
		var style lipgloss.Style

		if isLast {
			style = theme.BreadcrumbActive
		} else {
			style = theme.BreadcrumbItem
		}

		parts = append(parts, style.Render(item))

		if !isLast {
			parts = append(parts, theme.BreadcrumbSeparator.Render(b.separator))
		}
	}

	result := strings.Join(parts, "")

	// Truncate if needed
	maxW := b.maxWidth
	if width > 0 && (maxW == 0 || width < maxW) {
		maxW = width
	}
	if maxW > 0 && lipgloss.Width(result) > maxW {
		// Show only last few items with ellipsis
		result = "…" + result[len(result)-maxW+1:]
	}

	return result
}

// TabItem represents a single tab
type TabItem struct {
	ID      string
	Title   string
	Badge   string
	Content func(width, height int) string
}

// Tabs is a tab navigation component
type Tabs struct {
	BaseComponent
	FocusState

	items  []TabItem
	active int
	width  int
	height int
}

// NewTabs creates a new tab component
func NewTabs(items ...TabItem) *Tabs {
	return &Tabs{
		BaseComponent: NewBaseComponent(),
		items:         items,
		active:        0,
	}
}

// WithItems sets the tab items
func (t *Tabs) WithItems(items ...TabItem) *Tabs {
	t.items = items
	return t
}

// AddTab adds a tab
func (t *Tabs) AddTab(id, title string, content func(width, height int) string) *Tabs {
	t.items = append(t.items, TabItem{ID: id, Title: title, Content: content})
	return t
}

// WithTheme sets the theme
func (t *Tabs) WithTheme(theme *themes.Theme) *Tabs {
	t.SetTheme(theme)
	return t
}

// Init implements tea.Model
func (t *Tabs) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (t *Tabs) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !t.Focused() {
		return t, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "right", "l":
			t.active = (t.active + 1) % len(t.items)
		case "shift+tab", "left", "h":
			t.active--
			if t.active < 0 {
				t.active = len(t.items) - 1
			}
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			idx := int(msg.String()[0] - '1')
			if idx < len(t.items) {
				t.active = idx
			}
		}

	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height
	}

	return t, nil
}

// View implements tea.Model
func (t *Tabs) View() string {
	return t.ViewWidth(t.width)
}

// ViewWidth renders the tabs at a specific width (implements Component)
func (t *Tabs) ViewWidth(width int) string {
	if len(t.items) == 0 {
		return ""
	}

	theme := t.Theme()
	var b strings.Builder

	// Render tab headers
	var tabs []string
	for i, item := range t.items {
		var style lipgloss.Style
		if i == t.active {
			style = theme.TabActive
		} else {
			style = theme.TabInactive
		}

		title := item.Title
		if item.Badge != "" {
			title += " " + theme.BadgeNeutral.Render(item.Badge)
		}
		tabs = append(tabs, style.Render(title))
	}

	tabLine := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	b.WriteString(theme.TabBar.Width(width).Render(tabLine))
	b.WriteString("\n\n")

	// Render active tab content
	if t.active >= 0 && t.active < len(t.items) {
		item := t.items[t.active]
		if item.Content != nil {
			contentHeight := t.height - 4
			if contentHeight < 0 {
				contentHeight = 10
			}
			b.WriteString(item.Content(width, contentHeight))
		}
	}

	return b.String()
}

// ActiveIndex returns the active tab index
func (t *Tabs) ActiveIndex() int {
	return t.active
}

// SetActiveIndex sets the active tab
func (t *Tabs) SetActiveIndex(index int) {
	if index >= 0 && index < len(t.items) {
		t.active = index
	}
}

// ActiveID returns the ID of the active tab
func (t *Tabs) ActiveID() string {
	if t.active >= 0 && t.active < len(t.items) {
		return t.items[t.active].ID
	}
	return ""
}

// SetActiveByID sets the active tab by ID
func (t *Tabs) SetActiveByID(id string) {
	for i, item := range t.items {
		if item.ID == id {
			t.active = i
			return
		}
	}
}

// Focus implements FocusableComponent
func (t *Tabs) Focus() tea.Cmd {
	t.FocusState.Focus()
	return nil
}

// StepIndicator shows progress through steps
type StepIndicator struct {
	BaseComponent

	steps   []string
	current int
}

// NewStepIndicator creates a new step indicator
func NewStepIndicator(steps ...string) *StepIndicator {
	return &StepIndicator{
		BaseComponent: NewBaseComponent(),
		steps:         steps,
		current:       0,
	}
}

// WithSteps sets the step names
func (s *StepIndicator) WithSteps(steps ...string) *StepIndicator {
	s.steps = steps
	return s
}

// WithCurrent sets the current step (0-indexed)
func (s *StepIndicator) WithCurrent(current int) *StepIndicator {
	s.current = clamp(current, 0, len(s.steps)-1)
	return s
}

// Next advances to the next step
func (s *StepIndicator) Next() {
	if s.current < len(s.steps)-1 {
		s.current++
	}
}

// Prev goes back to the previous step
func (s *StepIndicator) Prev() {
	if s.current > 0 {
		s.current--
	}
}

// WithTheme sets the theme
func (s *StepIndicator) WithTheme(theme *themes.Theme) *StepIndicator {
	s.SetTheme(theme)
	return s
}

// View implements Component
func (s *StepIndicator) View(width int) string {
	if len(s.steps) == 0 {
		return ""
	}

	theme := s.Theme()
	var parts []string

	for i, step := range s.steps {
		var stepStyle lipgloss.Style
		var icon string

		if i < s.current {
			stepStyle = theme.StepComplete
			icon = themes.IconCheck
		} else if i == s.current {
			stepStyle = theme.StepCurrent
			icon = themes.IconDot
		} else {
			stepStyle = theme.StepPending
			icon = themes.IconCircle
		}

		stepNum := theme.StepNumber.Render(icon)
		stepTitle := stepStyle.Render(step)
		parts = append(parts, stepNum+" "+stepTitle)
	}

	separator := theme.Blurred.Render(" ─── ")
	return strings.Join(parts, separator)
}

// VerticalStepIndicator shows steps vertically
type VerticalStepIndicator struct {
	BaseComponent

	steps       []StepInfo
	current     int
	showNumbers bool
}

// StepInfo contains information about a step
type StepInfo struct {
	Title       string
	Description string
}

// NewVerticalStepIndicator creates a new vertical step indicator
func NewVerticalStepIndicator(steps ...StepInfo) *VerticalStepIndicator {
	return &VerticalStepIndicator{
		BaseComponent: NewBaseComponent(),
		steps:         steps,
		current:       0,
		showNumbers:   true,
	}
}

// WithSteps sets the steps
func (v *VerticalStepIndicator) WithSteps(steps ...StepInfo) *VerticalStepIndicator {
	v.steps = steps
	return v
}

// WithCurrent sets the current step
func (v *VerticalStepIndicator) WithCurrent(current int) *VerticalStepIndicator {
	v.current = clamp(current, 0, len(v.steps)-1)
	return v
}

// WithNumbers enables/disables step numbers
func (v *VerticalStepIndicator) WithNumbers(show bool) *VerticalStepIndicator {
	v.showNumbers = show
	return v
}

// WithTheme sets the theme
func (v *VerticalStepIndicator) WithTheme(theme *themes.Theme) *VerticalStepIndicator {
	v.SetTheme(theme)
	return v
}

// View implements Component
func (v *VerticalStepIndicator) View(width int) string {
	if len(v.steps) == 0 {
		return ""
	}

	theme := v.Theme()
	var lines []string

	for i, step := range v.steps {
		var stepStyle lipgloss.Style
		var icon string
		var connector string

		if i < v.current {
			stepStyle = theme.StepComplete
			icon = themes.IconCheck
		} else if i == v.current {
			stepStyle = theme.StepCurrent
			icon = themes.IconDot
		} else {
			stepStyle = theme.StepPending
			icon = themes.IconCircle
		}

		// Icon/number + title
		prefix := theme.StepNumber.Render(icon)
		title := stepStyle.Render(step.Title)
		lines = append(lines, prefix+" "+title)

		// Description (indented)
		if step.Description != "" {
			desc := theme.Description.Render(step.Description)
			lines = append(lines, "    "+desc)
		}

		// Connector to next step
		if i < len(v.steps)-1 {
			if i < v.current {
				connector = theme.StepComplete.Render("│")
			} else {
				connector = theme.StepPending.Render("│")
			}
			lines = append(lines, "  "+connector)
		}
	}

	return strings.Join(lines, "\n")
}
