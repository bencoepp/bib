package app

import (
	"strings"

	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Dialog is the interface for modal dialogs.
type Dialog interface {
	tea.Model

	// Done returns true if the dialog should be closed.
	Done() bool

	// SetSize updates the dialog dimensions.
	SetSize(width, height int)

	// WithTheme sets the dialog theme.
	WithTheme(theme *themes.Theme) Dialog
}

// ConfirmDialog is a yes/no confirmation dialog.
type ConfirmDialog struct {
	theme     *themes.Theme
	title     string
	message   string
	confirmed bool
	done      bool
	onConfirm func(bool) tea.Cmd
	width     int
	height    int
	focused   int // 0 = No, 1 = Yes
}

// NewConfirmDialog creates a new confirmation dialog.
func NewConfirmDialog(title, message string, onConfirm func(bool) tea.Cmd) *ConfirmDialog {
	return &ConfirmDialog{
		theme:     themes.Global().Active(),
		title:     title,
		message:   message,
		onConfirm: onConfirm,
		focused:   0,
	}
}

// WithTheme sets the dialog theme.
func (d *ConfirmDialog) WithTheme(theme *themes.Theme) Dialog {
	d.theme = theme
	return d
}

// Done returns true if the dialog should be closed.
func (d *ConfirmDialog) Done() bool {
	return d.done
}

// SetSize updates the dialog dimensions.
func (d *ConfirmDialog) SetSize(width, height int) {
	d.width = width
	d.height = height
}

// Init implements tea.Model.
func (d *ConfirmDialog) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (d *ConfirmDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			d.focused = 0
		case "right", "l":
			d.focused = 1
		case "tab":
			d.focused = (d.focused + 1) % 2
		case "y", "Y":
			d.confirmed = true
			d.done = true
			if d.onConfirm != nil {
				return d, d.onConfirm(true)
			}
		case "n", "N", "escape":
			d.confirmed = false
			d.done = true
			if d.onConfirm != nil {
				return d, d.onConfirm(false)
			}
		case "enter":
			d.confirmed = d.focused == 1
			d.done = true
			if d.onConfirm != nil {
				return d, d.onConfirm(d.confirmed)
			}
		}
	}
	return d, nil
}

// View implements tea.Model.
func (d *ConfirmDialog) View() string {
	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(d.theme.Palette.Primary)
	b.WriteString(titleStyle.Render(d.title))
	b.WriteString("\n\n")

	// Message
	b.WriteString(d.message)
	b.WriteString("\n\n")

	// Buttons
	noStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Border(lipgloss.RoundedBorder())
	yesStyle := noStyle.Copy()

	if d.focused == 0 {
		noStyle = noStyle.
			BorderForeground(d.theme.Palette.Primary).
			Foreground(d.theme.Palette.Primary)
	} else {
		yesStyle = yesStyle.
			BorderForeground(d.theme.Palette.Primary).
			Foreground(d.theme.Palette.Primary)
	}

	buttons := lipgloss.JoinHorizontal(
		lipgloss.Center,
		noStyle.Render("No"),
		"  ",
		yesStyle.Render("Yes"),
	)
	b.WriteString(buttons)

	// Dialog container
	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(d.theme.Palette.Border).
		Padding(1, 2).
		Width(50)

	return containerStyle.Render(b.String())
}

// HelpDialog shows keybinding help for a page.
type HelpDialog struct {
	theme  *themes.Theme
	page   Page
	done   bool
	width  int
	height int
}

// NewHelpDialog creates a new help dialog.
func NewHelpDialog(page Page) *HelpDialog {
	return &HelpDialog{
		theme: themes.Global().Active(),
		page:  page,
	}
}

// WithTheme sets the dialog theme.
func (d *HelpDialog) WithTheme(theme *themes.Theme) Dialog {
	d.theme = theme
	return d
}

// Done returns true if the dialog should be closed.
func (d *HelpDialog) Done() bool {
	return d.done
}

// SetSize updates the dialog dimensions.
func (d *HelpDialog) SetSize(width, height int) {
	d.width = width
	d.height = height
}

// Init implements tea.Model.
func (d *HelpDialog) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (d *HelpDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "escape", "q", "?", "enter":
			d.done = true
		}
	}
	return d, nil
}

// View implements tea.Model.
func (d *HelpDialog) View() string {
	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(d.theme.Palette.Primary)

	if d.page != nil {
		b.WriteString(titleStyle.Render(d.page.Title() + " Help"))
	} else {
		b.WriteString(titleStyle.Render("Help"))
	}
	b.WriteString("\n\n")

	// Keybindings
	if d.page != nil {
		fullHelp := d.page.FullHelp()
		for _, group := range fullHelp {
			for _, binding := range group {
				keyStyle := lipgloss.NewStyle().
					Width(15).
					Foreground(d.theme.Palette.Primary)
				b.WriteString(keyStyle.Render(binding.Key))
				b.WriteString(binding.Help)
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
	}

	// Global keybindings
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Global"))
	b.WriteString("\n")
	globalBindings := []KeyBinding{
		{Key: "tab / shift+tab", Help: "Switch pages"},
		{Key: "1-9", Help: "Jump to page"},
		{Key: "/", Help: "Command palette"},
		{Key: "?", Help: "Help"},
		{Key: "q", Help: "Quit"},
	}
	for _, binding := range globalBindings {
		keyStyle := lipgloss.NewStyle().
			Width(15).
			Foreground(d.theme.Palette.Primary)
		b.WriteString(keyStyle.Render(binding.Key))
		b.WriteString(binding.Help)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(d.theme.Palette.TextMuted).Render("Press Esc to close"))

	// Dialog container
	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(d.theme.Palette.Border).
		Padding(1, 2).
		Width(60)

	return containerStyle.Render(b.String())
}

// CommandPalette is a k9s-style command palette for quick navigation.
type CommandPalette struct {
	theme    *themes.Theme
	pages    []Page
	filter   string
	selected int
	done     bool
	width    int
	height   int
	onSelect func(pageID string) tea.Cmd
}

// NewCommandPalette creates a new command palette.
func NewCommandPalette(pages []Page) *CommandPalette {
	return &CommandPalette{
		theme:    themes.Global().Active(),
		pages:    pages,
		selected: 0,
	}
}

// WithTheme sets the dialog theme.
func (d *CommandPalette) WithTheme(theme *themes.Theme) Dialog {
	d.theme = theme
	return d
}

// OnSelect sets the callback when a page is selected.
func (d *CommandPalette) OnSelect(fn func(pageID string) tea.Cmd) *CommandPalette {
	d.onSelect = fn
	return d
}

// Done returns true if the dialog should be closed.
func (d *CommandPalette) Done() bool {
	return d.done
}

// SetSize updates the dialog dimensions.
func (d *CommandPalette) SetSize(width, height int) {
	d.width = width
	d.height = height
}

// Init implements tea.Model.
func (d *CommandPalette) Init() tea.Cmd {
	return nil
}

// filteredPages returns pages matching the filter.
func (d *CommandPalette) filteredPages() []Page {
	if d.filter == "" {
		return d.pages
	}

	filter := strings.ToLower(d.filter)
	var result []Page
	for _, page := range d.pages {
		if strings.Contains(strings.ToLower(page.Title()), filter) ||
			strings.Contains(strings.ToLower(page.ID()), filter) {
			result = append(result, page)
		}
	}
	return result
}

// Update implements tea.Model.
func (d *CommandPalette) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "escape":
			d.done = true
			return d, nil

		case "up", "ctrl+p":
			if d.selected > 0 {
				d.selected--
			}

		case "down", "ctrl+n":
			pages := d.filteredPages()
			if d.selected < len(pages)-1 {
				d.selected++
			}

		case "enter":
			pages := d.filteredPages()
			if d.selected >= 0 && d.selected < len(pages) {
				d.done = true
				if d.onSelect != nil {
					return d, d.onSelect(pages[d.selected].ID())
				}
			}

		case "backspace":
			if len(d.filter) > 0 {
				d.filter = d.filter[:len(d.filter)-1]
				d.selected = 0
			}

		default:
			// Add character to filter
			if len(msg.String()) == 1 {
				d.filter += msg.String()
				d.selected = 0
			}
		}
	}
	return d, nil
}

// View implements tea.Model.
func (d *CommandPalette) View() string {
	var b strings.Builder

	// Search input
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(d.theme.Palette.Primary).
		Padding(0, 1).
		Width(40)

	prompt := "> "
	cursor := "█"
	if d.filter == "" {
		b.WriteString(inputStyle.Render(prompt + lipgloss.NewStyle().Foreground(d.theme.Palette.TextMuted).Render("Type to search...") + cursor))
	} else {
		b.WriteString(inputStyle.Render(prompt + d.filter + cursor))
	}
	b.WriteString("\n\n")

	// Pages list
	pages := d.filteredPages()
	for i, page := range pages {
		style := lipgloss.NewStyle().Width(40)
		if i == d.selected {
			style = style.
				Background(d.theme.Palette.Primary).
				Foreground(d.theme.Palette.Background)
		}
		b.WriteString(style.Render("  " + page.Title()))
		b.WriteString("\n")
	}

	if len(pages) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(d.theme.Palette.TextMuted).Render("  No matching pages"))
	}

	// Container
	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(d.theme.Palette.Border).
		Padding(1, 2).
		Width(50)

	return containerStyle.Render(b.String())
}
