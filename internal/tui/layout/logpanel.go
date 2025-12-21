package layout

import (
	"fmt"
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

// String returns the string representation of a log level
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

// ShortString returns abbreviated log level
func (l LogLevel) ShortString() string {
	switch l {
	case LogLevelDebug:
		return "D"
	case LogLevelInfo:
		return "I"
	case LogLevelWarn:
		return "W"
	case LogLevelError:
		return "E"
	default:
		return "?"
	}
}

// LogEntry represents a single log message
type LogEntry struct {
	Time    time.Time
	Level   LogLevel
	Message string
	Source  string
}

// LogPanel displays real-time logs
type LogPanel struct {
	theme  *themes.Theme
	width  int
	height int
	icons  IconSet

	// Entries
	entries    []LogEntry
	maxEntries int
	mu         sync.RWMutex

	// Display options
	showTimestamp bool
	showLevel     bool
	showSource    bool
	autoScroll    bool
	minLevel      LogLevel

	// Scroll
	scrollOffset int

	// Focus
	focused  bool
	expanded bool
}

// NewLogPanel creates a new log panel
func NewLogPanel() *LogPanel {
	return &LogPanel{
		theme:         themes.Global().Active(),
		icons:         GetIcons(),
		entries:       make([]LogEntry, 0),
		maxEntries:    1000,
		showTimestamp: true,
		showLevel:     true,
		showSource:    false,
		autoScroll:    true,
		minLevel:      LogLevelDebug,
	}
}

// SetTheme sets the theme
func (l *LogPanel) SetTheme(theme *themes.Theme) {
	l.theme = theme
}

// SetSize sets the dimensions
func (l *LogPanel) SetSize(width, height int) {
	l.width = width
	l.height = height
}

// SetFocused sets the focused state
func (l *LogPanel) SetFocused(focused bool) {
	l.focused = focused
}

// SetExpanded sets whether the panel is expanded
func (l *LogPanel) SetExpanded(expanded bool) {
	l.expanded = expanded
}

// SetMinLevel sets the minimum log level to display
func (l *LogPanel) SetMinLevel(level LogLevel) {
	l.minLevel = level
}

// AddEntry adds a log entry
func (l *LogPanel) AddEntry(entry LogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if entry.Time.IsZero() {
		entry.Time = time.Now()
	}

	l.entries = append(l.entries, entry)

	// Trim old entries
	if len(l.entries) > l.maxEntries {
		l.entries = l.entries[len(l.entries)-l.maxEntries:]
	}

	// Auto-scroll to bottom
	if l.autoScroll {
		l.scrollToBottom()
	}
}

// AddMessage is a convenience method to add a simple log message
func (l *LogPanel) AddMessage(level LogLevel, message string) {
	l.AddEntry(LogEntry{
		Time:    time.Now(),
		Level:   level,
		Message: message,
	})
}

// Info adds an info message
func (l *LogPanel) Info(message string) {
	l.AddMessage(LogLevelInfo, message)
}

// Warn adds a warning message
func (l *LogPanel) Warn(message string) {
	l.AddMessage(LogLevelWarn, message)
}

// Error adds an error message
func (l *LogPanel) Error(message string) {
	l.AddMessage(LogLevelError, message)
}

// Debug adds a debug message
func (l *LogPanel) Debug(message string) {
	l.AddMessage(LogLevelDebug, message)
}

// Clear removes all entries
func (l *LogPanel) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = make([]LogEntry, 0)
	l.scrollOffset = 0
}

func (l *LogPanel) scrollToBottom() {
	visibleLines := l.height - 2 // Account for border/title
	if visibleLines < 1 {
		visibleLines = 1
	}

	filteredCount := l.filteredEntryCount()
	if filteredCount > visibleLines {
		l.scrollOffset = filteredCount - visibleLines
	} else {
		l.scrollOffset = 0
	}
}

func (l *LogPanel) filteredEntryCount() int {
	count := 0
	for _, e := range l.entries {
		if e.Level >= l.minLevel {
			count++
		}
	}
	return count
}

// Init implements tea.Model
func (l *LogPanel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (l *LogPanel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !l.focused {
			return l, nil
		}

		switch msg.String() {
		case "up", "k":
			l.scrollUp(1)
		case "down", "j":
			l.scrollDown(1)
		case "pgup":
			l.scrollUp(l.height - 2)
		case "pgdown":
			l.scrollDown(l.height - 2)
		case "home", "g":
			l.scrollOffset = 0
			l.autoScroll = false
		case "end", "G":
			l.scrollToBottom()
			l.autoScroll = true
		case "c":
			l.Clear()
		case "t":
			l.showTimestamp = !l.showTimestamp
		case "s":
			l.showSource = !l.showSource
		case "d":
			l.cycleMinLevel()
		}
	}

	return l, nil
}

func (l *LogPanel) scrollUp(lines int) {
	l.autoScroll = false
	l.scrollOffset -= lines
	if l.scrollOffset < 0 {
		l.scrollOffset = 0
	}
}

func (l *LogPanel) scrollDown(lines int) {
	maxScroll := l.filteredEntryCount() - (l.height - 2)
	if maxScroll < 0 {
		maxScroll = 0
	}

	l.scrollOffset += lines
	if l.scrollOffset >= maxScroll {
		l.scrollOffset = maxScroll
		l.autoScroll = true
	}
}

func (l *LogPanel) cycleMinLevel() {
	l.minLevel = (l.minLevel + 1) % 4
}

// View implements tea.Model - returns content with border
func (l *LogPanel) View() string {
	content := l.ViewContent()

	// Apply container style - always use border to prevent layout shift
	style := lipgloss.NewStyle().
		Width(l.width).
		Height(l.height).
		BorderStyle(lipgloss.RoundedBorder())

	if l.focused {
		style = style.BorderForeground(l.theme.Palette.Primary)
	} else {
		style = style.BorderForeground(l.theme.Palette.Border)
	}

	return style.Render(content)
}

// ViewContent returns just the log panel content without border
func (l *LogPanel) ViewContent() string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	bp := GetBreakpoint(l.width)
	box := GetBoxChars()

	// Build header
	headerStyle := lipgloss.NewStyle().
		Foreground(l.theme.Palette.TextMuted).
		Bold(true)

	title := box.Horizontal + " Logs "
	if l.minLevel > LogLevelDebug {
		title += fmt.Sprintf("[%s+] ", l.minLevel.String())
	}
	titleWidth := lipgloss.Width(title)

	// Fill rest of header with horizontal line
	fillWidth := l.width - titleWidth
	if fillWidth < 0 {
		fillWidth = 0
	}
	headerLine := headerStyle.Render(title) +
		lipgloss.NewStyle().Foreground(l.theme.Palette.Border).
			Render(strings.Repeat(box.Horizontal, fillWidth))

	// Collect visible entries
	var lines []string
	lines = append(lines, headerLine)

	// Filter and render entries
	filteredEntries := l.getFilteredEntries()
	visibleHeight := l.height - 1 // Account for header

	start := l.scrollOffset
	if start < 0 {
		start = 0
	}
	end := start + visibleHeight
	if end > len(filteredEntries) {
		end = len(filteredEntries)
	}

	for i := start; i < end; i++ {
		entry := filteredEntries[i]
		lines = append(lines, l.renderEntry(entry, bp))
	}

	// Pad with empty lines if needed
	for len(lines) < l.height {
		lines = append(lines, strings.Repeat(" ", l.width))
	}

	// Truncate if too many lines
	if len(lines) > l.height {
		lines = lines[:l.height]
	}

	return strings.Join(lines, "\n")
}

func (l *LogPanel) getFilteredEntries() []LogEntry {
	filtered := make([]LogEntry, 0)
	for _, e := range l.entries {
		if e.Level >= l.minLevel {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func (l *LogPanel) renderEntry(entry LogEntry, bp Breakpoint) string {
	var parts []string

	// Timestamp
	if l.showTimestamp {
		timeStyle := lipgloss.NewStyle().Foreground(l.theme.Palette.TextSubtle)
		if bp >= BreakpointLG {
			parts = append(parts, timeStyle.Render(entry.Time.Format("15:04:05")))
		} else {
			parts = append(parts, timeStyle.Render(entry.Time.Format("15:04")))
		}
	}

	// Level
	if l.showLevel {
		levelStyle := l.getLevelStyle(entry.Level)
		if bp >= BreakpointMD {
			parts = append(parts, levelStyle.Render(fmt.Sprintf("%-5s", entry.Level.String())))
		} else {
			parts = append(parts, levelStyle.Render(entry.Level.ShortString()))
		}
	}

	// Source
	if l.showSource && entry.Source != "" && bp >= BreakpointLG {
		sourceStyle := lipgloss.NewStyle().Foreground(l.theme.Palette.Secondary)
		parts = append(parts, sourceStyle.Render(fmt.Sprintf("[%s]", entry.Source)))
	}

	// Message
	msgStyle := lipgloss.NewStyle().Foreground(l.theme.Palette.Text)
	parts = append(parts, msgStyle.Render(entry.Message))

	line := strings.Join(parts, " ")

	// Truncate if too long
	if lipgloss.Width(line) > l.width {
		line = line[:l.width-1] + "…"
	}

	return line
}

func (l *LogPanel) getLevelStyle(level LogLevel) lipgloss.Style {
	switch level {
	case LogLevelDebug:
		return lipgloss.NewStyle().Foreground(l.theme.Palette.TextMuted)
	case LogLevelInfo:
		return lipgloss.NewStyle().Foreground(l.theme.Palette.Info)
	case LogLevelWarn:
		return lipgloss.NewStyle().Foreground(l.theme.Palette.Warning)
	case LogLevelError:
		return lipgloss.NewStyle().Foreground(l.theme.Palette.Error).Bold(true)
	default:
		return lipgloss.NewStyle().Foreground(l.theme.Palette.Text)
	}
}

// EntryCount returns the total number of entries
func (l *LogPanel) EntryCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.entries)
}

// FilteredCount returns the number of visible entries (after filtering)
func (l *LogPanel) FilteredCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.filteredEntryCount()
}
