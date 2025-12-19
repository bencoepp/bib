package component

import (
	"strings"
	"sync"
	"time"

	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LogLevel represents the severity of a log entry
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// LogEntry represents a single log message
type LogEntry struct {
	Time    time.Time
	Level   LogLevel
	Message string
	Details string
}

// ErrorLogMsg is sent when a new log entry should be added
type ErrorLogMsg struct {
	Entry LogEntry
}

// ClearErrorLogMsg clears all log entries
type ClearErrorLogMsg struct{}

// ErrorLog is a collapsible error/warning log panel
type ErrorLog struct {
	BaseComponent
	FocusState

	entries    []LogEntry
	maxEntries int
	expanded   bool
	height     int // collapsed height (expanded uses full available)
	width      int
	selected   int
	mu         sync.RWMutex
}

// NewErrorLog creates a new error log panel
func NewErrorLog() *ErrorLog {
	return &ErrorLog{
		BaseComponent: NewBaseComponent(),
		entries:       make([]LogEntry, 0),
		maxEntries:    100,
		expanded:      false,
		height:        3, // collapsed shows latest entry only
	}
}

// WithMaxEntries sets the maximum number of entries to keep
func (e *ErrorLog) WithMaxEntries(max int) *ErrorLog {
	e.maxEntries = max
	return e
}

// WithTheme sets the theme
func (e *ErrorLog) WithTheme(theme *themes.Theme) *ErrorLog {
	e.SetTheme(theme)
	return e
}

// AddEntry adds a new log entry
func (e *ErrorLog) AddEntry(entry LogEntry) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.entries = append(e.entries, entry)
	if len(e.entries) > e.maxEntries {
		e.entries = e.entries[1:]
	}
}

// AddError adds an error entry
func (e *ErrorLog) AddError(msg string, details string) {
	e.AddEntry(LogEntry{
		Time:    time.Now(),
		Level:   LogLevelError,
		Message: msg,
		Details: details,
	})
}

// AddWarning adds a warning entry
func (e *ErrorLog) AddWarning(msg string, details string) {
	e.AddEntry(LogEntry{
		Time:    time.Now(),
		Level:   LogLevelWarn,
		Message: msg,
		Details: details,
	})
}

// AddInfo adds an info entry
func (e *ErrorLog) AddInfo(msg string, details string) {
	e.AddEntry(LogEntry{
		Time:    time.Now(),
		Level:   LogLevelInfo,
		Message: msg,
		Details: details,
	})
}

// Clear removes all entries
func (e *ErrorLog) Clear() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.entries = make([]LogEntry, 0)
}

// HasErrors returns true if there are error entries
func (e *ErrorLog) HasErrors() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, entry := range e.entries {
		if entry.Level == LogLevelError {
			return true
		}
	}
	return false
}

// HasWarnings returns true if there are warning entries
func (e *ErrorLog) HasWarnings() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, entry := range e.entries {
		if entry.Level == LogLevelWarn {
			return true
		}
	}
	return false
}

// IsEmpty returns true if there are no entries
func (e *ErrorLog) IsEmpty() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.entries) == 0
}

// Toggle expands/collapses the log panel
func (e *ErrorLog) Toggle() {
	e.expanded = !e.expanded
}

// Expand expands the log panel
func (e *ErrorLog) Expand() {
	e.expanded = true
}

// Collapse collapses the log panel
func (e *ErrorLog) Collapse() {
	e.expanded = false
}

// IsExpanded returns true if the panel is expanded
func (e *ErrorLog) IsExpanded() bool {
	return e.expanded
}

// SetSize sets the panel dimensions
func (e *ErrorLog) SetSize(width, height int) {
	e.width = width
	e.height = height
}

// Init implements tea.Model
func (e *ErrorLog) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (e *ErrorLog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ErrorLogMsg:
		e.AddEntry(msg.Entry)
		return e, nil

	case ClearErrorLogMsg:
		e.Clear()
		return e, nil

	case tea.KeyMsg:
		if !e.Focused() {
			return e, nil
		}

		switch msg.String() {
		case "enter", "space":
			e.Toggle()
		case "j", "down":
			e.mu.RLock()
			if e.selected < len(e.entries)-1 {
				e.selected++
			}
			e.mu.RUnlock()
		case "k", "up":
			if e.selected > 0 {
				e.selected--
			}
		case "c":
			e.Clear()
		case "escape":
			if e.expanded {
				e.Collapse()
			}
		}
	}

	return e, nil
}

// View implements tea.Model
func (e *ErrorLog) View() string {
	return e.ViewWidth(e.width)
}

// ViewWidth renders the error log at a specific width
func (e *ErrorLog) ViewWidth(width int) string {
	if width == 0 {
		width = 80
	}

	theme := e.Theme()

	e.mu.RLock()
	defer e.mu.RUnlock()

	if len(e.entries) == 0 {
		return ""
	}

	var b strings.Builder

	// Header bar
	headerStyle := lipgloss.NewStyle().
		Background(theme.Palette.BackgroundAlt).
		Foreground(theme.Palette.Text).
		Bold(true).
		Width(width).
		Padding(0, 1)

	expandIcon := "▶"
	if e.expanded {
		expandIcon = "▼"
	}

	// Count errors and warnings
	errorCount := 0
	warnCount := 0
	for _, entry := range e.entries {
		switch entry.Level {
		case LogLevelError:
			errorCount++
		case LogLevelWarn:
			warnCount++
		}
	}

	headerText := expandIcon + " Log"
	if errorCount > 0 {
		headerText += lipgloss.NewStyle().Foreground(theme.Palette.Error).Render(" ● " + string(rune('0'+errorCount)) + " errors")
	}
	if warnCount > 0 {
		headerText += lipgloss.NewStyle().Foreground(theme.Palette.Warning).Render(" ● " + string(rune('0'+warnCount)) + " warnings")
	}

	b.WriteString(headerStyle.Render(headerText))
	b.WriteString("\n")

	// Content
	if e.expanded {
		// Show all entries
		contentStyle := lipgloss.NewStyle().
			Width(width).
			MaxHeight(e.height - 1)

		var content strings.Builder
		for i, entry := range e.entries {
			content.WriteString(e.renderEntry(entry, i == e.selected, width-2))
			content.WriteString("\n")
		}

		b.WriteString(contentStyle.Render(content.String()))
	} else {
		// Show only latest entry
		if len(e.entries) > 0 {
			latest := e.entries[len(e.entries)-1]
			b.WriteString(e.renderEntry(latest, false, width-2))
		}
	}

	// Border
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Palette.Border)

	if e.HasErrors() {
		borderStyle = borderStyle.BorderForeground(theme.Palette.Error)
	} else if e.HasWarnings() {
		borderStyle = borderStyle.BorderForeground(theme.Palette.Warning)
	}

	return borderStyle.Render(b.String())
}

func (e *ErrorLog) renderEntry(entry LogEntry, selected bool, width int) string {
	theme := e.Theme()

	// Level indicator
	var levelStyle lipgloss.Style
	switch entry.Level {
	case LogLevelError:
		levelStyle = lipgloss.NewStyle().Foreground(theme.Palette.Error)
	case LogLevelWarn:
		levelStyle = lipgloss.NewStyle().Foreground(theme.Palette.Warning)
	case LogLevelInfo:
		levelStyle = lipgloss.NewStyle().Foreground(theme.Palette.Info)
	default:
		levelStyle = lipgloss.NewStyle().Foreground(theme.Palette.TextMuted)
	}

	timeStr := entry.Time.Format("15:04:05")
	timeStyle := lipgloss.NewStyle().Foreground(theme.Palette.TextMuted)

	msgStyle := lipgloss.NewStyle().Foreground(theme.Palette.Text)
	if selected {
		msgStyle = msgStyle.Background(theme.Palette.Selection)
	}

	line := timeStyle.Render(timeStr) + " " +
		levelStyle.Render("["+entry.Level.String()+"]") + " " +
		msgStyle.Render(entry.Message)

	// Truncate if too long
	if lipgloss.Width(line) > width {
		line = line[:width-3] + "..."
	}

	return line
}

// EntryCount returns the number of entries
func (e *ErrorLog) EntryCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.entries)
}

// Entries returns a copy of all entries
func (e *ErrorLog) Entries() []LogEntry {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]LogEntry, len(e.entries))
	copy(result, e.entries)
	return result
}
