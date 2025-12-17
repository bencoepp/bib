package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ProgressBar is a progress bar component
type ProgressBar struct {
	theme      *Theme
	current    int
	total      int
	width      int
	showLabel  bool
	labelStyle lipgloss.Style
}

// NewProgressBar creates a new progress bar
func NewProgressBar(current, total, width int) *ProgressBar {
	theme := DefaultTheme()
	return &ProgressBar{
		theme:      theme,
		current:    current,
		total:      total,
		width:      width,
		showLabel:  true,
		labelStyle: theme.Info,
	}
}

// SetProgress updates the progress
func (p *ProgressBar) SetProgress(current int) {
	p.current = current
}

// SetWidth sets the width of the progress bar
func (p *ProgressBar) SetWidth(width int) {
	p.width = width
}

// ShowLabel toggles label visibility
func (p *ProgressBar) ShowLabel(show bool) {
	p.showLabel = show
}

// View renders the progress bar
func (p *ProgressBar) View() string {
	if p.total == 0 {
		return ""
	}

	// Calculate filled portion
	percent := float64(p.current) / float64(p.total)
	filled := int(float64(p.width) * percent)
	if filled > p.width {
		filled = p.width
	}

	// Build the bar
	bar := strings.Repeat("█", filled) + strings.Repeat("░", p.width-filled)

	// Add label
	if p.showLabel {
		label := fmt.Sprintf(" %d/%d", p.current, p.total)
		return p.theme.ProgressBar.Render(bar) + p.labelStyle.Render(label)
	}

	return p.theme.ProgressBar.Render(bar)
}

// StatusIndicator shows a status with an icon
type StatusIndicator struct {
	theme *Theme
}

// NewStatusIndicator creates a new status indicator
func NewStatusIndicator() *StatusIndicator {
	return &StatusIndicator{
		theme: DefaultTheme(),
	}
}

// Success renders a success message
func (s *StatusIndicator) Success(message string) string {
	return s.theme.Success.Render(fmt.Sprintf("%s %s", IconCheck, message))
}

// Error renders an error message
func (s *StatusIndicator) Error(message string) string {
	return s.theme.Error.Render(fmt.Sprintf("%s %s", IconCross, message))
}

// Warning renders a warning message
func (s *StatusIndicator) Warning(message string) string {
	return s.theme.Warning.Render(fmt.Sprintf("%s %s", IconWarning, message))
}

// Info renders an info message
func (s *StatusIndicator) Info(message string) string {
	return s.theme.Info.Render(fmt.Sprintf("%s %s", IconInfo, message))
}

// Pending renders a pending message
func (s *StatusIndicator) Pending(message string) string {
	return s.theme.Blurred.Render(fmt.Sprintf("%s %s", IconCircle, message))
}

// Box renders content in a styled box
type Box struct {
	theme   *Theme
	title   string
	content string
	width   int
}

// NewBox creates a new box
func NewBox(title, content string, width int) *Box {
	return &Box{
		theme:   DefaultTheme(),
		title:   title,
		content: content,
		width:   width,
	}
}

// View renders the box
func (b *Box) View() string {
	boxStyle := b.theme.Box.Width(b.width)

	var content strings.Builder
	if b.title != "" {
		content.WriteString(b.theme.BoxTitle.Render(b.title))
		content.WriteString("\n\n")
	}
	content.WriteString(b.content)

	return boxStyle.Render(content.String())
}

// KeyValue renders a key-value pair
type KeyValue struct {
	theme *Theme
}

// NewKeyValue creates a new key-value renderer
func NewKeyValue() *KeyValue {
	return &KeyValue{
		theme: DefaultTheme(),
	}
}

// Render renders a key-value pair
func (kv *KeyValue) Render(key, value string) string {
	keyStyle := kv.theme.Focused.Width(20)
	valueStyle := kv.theme.Base
	return keyStyle.Render(key+":") + " " + valueStyle.Render(value)
}

// RenderList renders multiple key-value pairs
func (kv *KeyValue) RenderList(items map[string]string, order []string) string {
	var lines []string
	for _, key := range order {
		if value, ok := items[key]; ok {
			lines = append(lines, kv.Render(key, value))
		}
	}
	return strings.Join(lines, "\n")
}

// Divider renders a horizontal divider
func Divider(width int) string {
	theme := DefaultTheme()
	return theme.Blurred.Render(strings.Repeat("─", width))
}

// Header renders a section header
func Header(title string) string {
	theme := DefaultTheme()
	return theme.SectionTitle.Render(title)
}

// Bullet renders a bullet point
func Bullet(text string) string {
	theme := DefaultTheme()
	return theme.Base.Render(fmt.Sprintf("  %s %s", IconBullet, text))
}

// BulletList renders a list of bullet points
func BulletList(items []string) string {
	var lines []string
	for _, item := range items {
		lines = append(lines, Bullet(item))
	}
	return strings.Join(lines, "\n")
}
