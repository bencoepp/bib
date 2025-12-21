package layout

import (
	"strings"

	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// StatusHint represents a keyboard hint in the status bar
type StatusHint struct {
	Key  string
	Help string
}

// StatusBar displays keyboard hints and status messages
type StatusBar struct {
	theme  *themes.Theme
	width  int
	height int
	icons  IconSet

	// Hints to display
	hints []StatusHint

	// Status message (temporary, overrides hints)
	message     string
	messageType StatusMessageType

	// Mode indicator
	mode string
}

// StatusMessageType indicates the type of status message
type StatusMessageType int

const (
	StatusMessageNormal StatusMessageType = iota
	StatusMessageSuccess
	StatusMessageWarning
	StatusMessageError
)

// NewStatusBar creates a new status bar
func NewStatusBar() *StatusBar {
	return &StatusBar{
		theme:  themes.Global().Active(),
		height: 1,
		icons:  GetIcons(),
		hints:  defaultHints(),
	}
}

func defaultHints() []StatusHint {
	return []StatusHint{
		{Key: "↑↓", Help: "Navigate"},
		{Key: "Enter", Help: "Select"},
		{Key: "/", Help: "Filter"},
		{Key: "?", Help: "Help"},
		{Key: "q", Help: "Quit"},
	}
}

// SetTheme sets the theme
func (s *StatusBar) SetTheme(theme *themes.Theme) {
	s.theme = theme
}

// SetSize sets the dimensions
func (s *StatusBar) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// SetHints sets the keyboard hints
func (s *StatusBar) SetHints(hints []StatusHint) {
	s.hints = hints
}

// SetMessage sets a temporary status message
func (s *StatusBar) SetMessage(message string) {
	s.message = message
	s.messageType = StatusMessageNormal
}

// SetSuccessMessage sets a success status message
func (s *StatusBar) SetSuccessMessage(message string) {
	s.message = message
	s.messageType = StatusMessageSuccess
}

// SetWarningMessage sets a warning status message
func (s *StatusBar) SetWarningMessage(message string) {
	s.message = message
	s.messageType = StatusMessageWarning
}

// SetErrorMessage sets an error status message
func (s *StatusBar) SetErrorMessage(message string) {
	s.message = message
	s.messageType = StatusMessageError
}

// ClearMessage clears any status message
func (s *StatusBar) ClearMessage() {
	s.message = ""
}

// SetMode sets the mode indicator (e.g., "NORMAL", "INSERT", "SEARCH")
func (s *StatusBar) SetMode(mode string) {
	s.mode = mode
}

// Init implements tea.Model
func (s *StatusBar) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (s *StatusBar) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return s, nil
}

// View implements tea.Model
func (s *StatusBar) View() string {
	bp := GetBreakpoint(s.width)

	// If there's a message, show it instead of hints
	if s.message != "" {
		return s.renderMessage()
	}

	return s.renderHints(bp)
}

func (s *StatusBar) renderMessage() string {
	var msgStyle lipgloss.Style

	switch s.messageType {
	case StatusMessageSuccess:
		msgStyle = lipgloss.NewStyle().
			Foreground(s.theme.Palette.Success).
			Bold(true)
	case StatusMessageWarning:
		msgStyle = lipgloss.NewStyle().
			Foreground(s.theme.Palette.Warning).
			Bold(true)
	case StatusMessageError:
		msgStyle = lipgloss.NewStyle().
			Foreground(s.theme.Palette.Error).
			Bold(true)
	default:
		msgStyle = lipgloss.NewStyle().
			Foreground(s.theme.Palette.Text)
	}

	// Icon based on type
	var icon string
	switch s.messageType {
	case StatusMessageSuccess:
		icon = s.icons.Check + " "
	case StatusMessageWarning:
		icon = s.icons.Warning + " "
	case StatusMessageError:
		icon = s.icons.Cross + " "
	}

	content := icon + s.message

	return lipgloss.NewStyle().
		Width(s.width).
		Render(msgStyle.Render(content))
}

func (s *StatusBar) renderHints(bp Breakpoint) string {
	var parts []string

	// Mode indicator if set
	if s.mode != "" {
		modeStyle := lipgloss.NewStyle().
			Background(s.theme.Palette.Primary).
			Foreground(s.theme.Palette.Background).
			Bold(true).
			Padding(0, 1)
		parts = append(parts, modeStyle.Render(s.mode))
	}

	// Render hints based on available width
	hintsStr := s.renderHintsList(bp)
	parts = append(parts, hintsStr)

	content := strings.Join(parts, " ")

	return lipgloss.NewStyle().
		Width(s.width).
		Foreground(s.theme.Palette.TextMuted).
		Render(content)
}

func (s *StatusBar) renderHintsList(bp Breakpoint) string {
	var hintParts []string

	keyStyle := lipgloss.NewStyle().
		Foreground(s.theme.Palette.Primary).
		Bold(true)

	helpStyle := lipgloss.NewStyle().
		Foreground(s.theme.Palette.TextMuted)

	// Calculate max hints based on breakpoint
	maxHints := len(s.hints)
	switch bp {
	case BreakpointXS:
		maxHints = 3
	case BreakpointSM:
		maxHints = 5
	case BreakpointMD:
		maxHints = 7
	}

	for i, hint := range s.hints {
		if i >= maxHints {
			break
		}

		var hintStr string
		if bp >= BreakpointSM {
			hintStr = keyStyle.Render(hint.Key) + " " + helpStyle.Render(hint.Help)
		} else {
			// Just keys on very small screens
			hintStr = keyStyle.Render(hint.Key)
		}
		hintParts = append(hintParts, hintStr)
	}

	return strings.Join(hintParts, "  ")
}

// DefaultContentHints returns standard hints for content navigation
func DefaultContentHints() []StatusHint {
	return []StatusHint{
		{Key: "↑↓/jk", Help: "Navigate"},
		{Key: "Enter", Help: "Select"},
		{Key: "Tab", Help: "Switch Panel"},
		{Key: "/", Help: "Filter"},
		{Key: ":", Help: "Command"},
		{Key: "?", Help: "Help"},
		{Key: "q", Help: "Quit"},
	}
}

// DefaultSidebarHints returns hints for sidebar navigation
func DefaultSidebarHints() []StatusHint {
	return []StatusHint{
		{Key: "↑↓", Help: "Navigate"},
		{Key: "Enter", Help: "Select"},
		{Key: "←→", Help: "Collapse/Expand"},
		{Key: "Tab", Help: "Switch Panel"},
		{Key: "Ctrl+B", Help: "Toggle Sidebar"},
	}
}

// DefaultLogPanelHints returns hints for log panel
func DefaultLogPanelHints() []StatusHint {
	return []StatusHint{
		{Key: "↑↓", Help: "Scroll"},
		{Key: "g/G", Help: "Top/Bottom"},
		{Key: "c", Help: "Clear"},
		{Key: "d", Help: "Filter Level"},
		{Key: "Tab", Help: "Switch Panel"},
		{Key: "L", Help: "Toggle Logs"},
	}
}

// WideLayoutHints returns additional hints for wide layouts
func WideLayoutHints() []StatusHint {
	return []StatusHint{
		{Key: "↑↓/jk", Help: "Navigate"},
		{Key: "Enter", Help: "Select"},
		{Key: "Tab", Help: "Switch Panel"},
		{Key: "Ctrl+1/2/3", Help: "Focus Panel"},
		{Key: "/", Help: "Filter"},
		{Key: ":", Help: "Command"},
		{Key: "L", Help: "Toggle Logs"},
		{Key: "I", Help: "Toggle Info"},
		{Key: "?", Help: "Help"},
		{Key: "q", Help: "Quit"},
	}
}
