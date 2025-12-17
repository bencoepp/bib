package component

import (
	"fmt"
	"strings"
	"time"

	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SpinnerStyle defines the spinner animation style
type SpinnerStyle int

const (
	SpinnerDots SpinnerStyle = iota
	SpinnerLine
	SpinnerCircle
	SpinnerBounce
	SpinnerPulse
	SpinnerGrow
)

// Spinner is an animated loading indicator
type Spinner struct {
	BaseComponent
	FocusState

	frames   []string
	frame    int
	interval time.Duration
	label    string
	running  bool
	style    SpinnerStyle
}

// SpinnerTickMsg is sent on each animation frame
type SpinnerTickMsg struct {
	ID int
}

var spinnerID int

// NewSpinner creates a new spinner
func NewSpinner() *Spinner {
	s := &Spinner{
		BaseComponent: NewBaseComponent(),
		frames:        themes.SpinnerDots(),
		frame:         0,
		interval:      80 * time.Millisecond,
		style:         SpinnerDots,
	}
	return s
}

// WithStyle sets the spinner style
func (s *Spinner) WithStyle(style SpinnerStyle) *Spinner {
	s.style = style
	switch style {
	case SpinnerDots:
		s.frames = themes.SpinnerDots()
	case SpinnerLine:
		s.frames = themes.SpinnerLine()
	case SpinnerCircle:
		s.frames = themes.SpinnerCircle()
	case SpinnerBounce:
		s.frames = themes.SpinnerBounce()
	case SpinnerPulse:
		s.frames = themes.SpinnerPulse()
	case SpinnerGrow:
		s.frames = themes.SpinnerGrow()
	}
	return s
}

// WithLabel sets the spinner label
func (s *Spinner) WithLabel(label string) *Spinner {
	s.label = label
	return s
}

// WithInterval sets the animation interval
func (s *Spinner) WithInterval(d time.Duration) *Spinner {
	s.interval = d
	return s
}

// WithTheme sets the theme
func (s *Spinner) WithTheme(theme *themes.Theme) *Spinner {
	s.SetTheme(theme)
	return s
}

// Init implements tea.Model
func (s *Spinner) Init() tea.Cmd {
	return s.tick()
}

// Update implements tea.Model
func (s *Spinner) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case SpinnerTickMsg:
		if s.running {
			s.frame = (s.frame + 1) % len(s.frames)
			return s, s.tick()
		}
	}
	return s, nil
}

func (s *Spinner) tick() tea.Cmd {
	spinnerID++
	id := spinnerID
	return tea.Tick(s.interval, func(t time.Time) tea.Msg {
		return SpinnerTickMsg{ID: id}
	})
}

// View implements tea.Model
func (s *Spinner) View() string {
	return s.ViewWidth(0)
}

// ViewWidth renders the spinner (implements Component)
func (s *Spinner) ViewWidth(width int) string {
	frame := s.Theme().Spinner.Render(s.frames[s.frame])
	if s.label != "" {
		return frame + " " + s.label
	}
	return frame
}

// Start starts the spinner animation
func (s *Spinner) Start() tea.Cmd {
	s.running = true
	return s.tick()
}

// Stop stops the spinner animation
func (s *Spinner) Stop() {
	s.running = false
}

// IsRunning returns whether the spinner is animating
func (s *Spinner) IsRunning() bool {
	return s.running
}

// StartAnimation implements AnimatableComponent
func (s *Spinner) StartAnimation() tea.Cmd {
	return s.Start()
}

// StopAnimation implements AnimatableComponent
func (s *Spinner) StopAnimation() {
	s.Stop()
}

// IsAnimating implements AnimatableComponent
func (s *Spinner) IsAnimating() bool {
	return s.running
}

// ProgressBar is an animated progress bar component
type ProgressBar struct {
	BaseComponent

	current     float64
	width       int
	showLabel   bool
	showPercent bool
	label       string
	animated    bool
	filledChar  string
	emptyChar   string
}

// NewProgressBar creates a new progress bar
func NewProgressBar() *ProgressBar {
	return &ProgressBar{
		BaseComponent: NewBaseComponent(),
		current:       0,
		width:         40,
		showLabel:     false,
		showPercent:   true,
		filledChar:    "█",
		emptyChar:     "░",
	}
}

// WithProgress sets the progress (0.0 to 1.0)
func (p *ProgressBar) WithProgress(progress float64) *ProgressBar {
	p.current = clamp64(progress, 0, 1)
	return p
}

// WithWidth sets the bar width
func (p *ProgressBar) WithWidth(width int) *ProgressBar {
	p.width = width
	return p
}

// WithLabel sets and shows the label
func (p *ProgressBar) WithLabel(label string) *ProgressBar {
	p.label = label
	p.showLabel = true
	return p
}

// WithPercent toggles percent display
func (p *ProgressBar) WithPercent(show bool) *ProgressBar {
	p.showPercent = show
	return p
}

// WithChars sets the fill and empty characters
func (p *ProgressBar) WithChars(filled, empty string) *ProgressBar {
	p.filledChar = filled
	p.emptyChar = empty
	return p
}

// WithTheme sets the theme
func (p *ProgressBar) WithTheme(theme *themes.Theme) *ProgressBar {
	p.SetTheme(theme)
	return p
}

// SetProgress updates the progress value
func (p *ProgressBar) SetProgress(progress float64) {
	p.current = clamp64(progress, 0, 1)
}

// View implements Component
func (p *ProgressBar) View(width int) string {
	barWidth := p.width
	if width > 0 && width < barWidth+15 {
		barWidth = width - 15
	}
	if barWidth < 10 {
		barWidth = 10
	}

	filled := int(float64(barWidth) * p.current)
	if filled > barWidth {
		filled = barWidth
	}

	theme := p.Theme()
	bar := theme.ProgressFilled.Render(strings.Repeat(p.filledChar, filled)) +
		theme.ProgressEmpty.Render(strings.Repeat(p.emptyChar, barWidth-filled))

	var parts []string

	if p.showLabel && p.label != "" {
		parts = append(parts, theme.Base.Render(p.label))
	}

	parts = append(parts, bar)

	if p.showPercent {
		percent := fmt.Sprintf("%3.0f%%", p.current*100)
		parts = append(parts, theme.Info.Render(percent))
	}

	return strings.Join(parts, " ")
}

// clamp64 restricts a float64 value to a range
func clamp64(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// Badge is a small status indicator
type Badge struct {
	BaseComponent

	text    string
	variant BadgeVariant
}

// BadgeVariant defines the badge style variant
type BadgeVariant int

const (
	BadgePrimary BadgeVariant = iota
	BadgeSuccess
	BadgeWarning
	BadgeError
	BadgeInfo
	BadgeNeutral
)

// NewBadge creates a new badge
func NewBadge(text string) *Badge {
	return &Badge{
		BaseComponent: NewBaseComponent(),
		text:          text,
		variant:       BadgeNeutral,
	}
}

// WithVariant sets the badge variant
func (b *Badge) WithVariant(v BadgeVariant) *Badge {
	b.variant = v
	return b
}

// Primary sets the badge to primary variant
func (b *Badge) Primary() *Badge {
	b.variant = BadgePrimary
	return b
}

// Success sets the badge to success variant
func (b *Badge) Success() *Badge {
	b.variant = BadgeSuccess
	return b
}

// Warning sets the badge to warning variant
func (b *Badge) Warning() *Badge {
	b.variant = BadgeWarning
	return b
}

// Error sets the badge to error variant
func (b *Badge) Error() *Badge {
	b.variant = BadgeError
	return b
}

// Info sets the badge to info variant
func (b *Badge) Info() *Badge {
	b.variant = BadgeInfo
	return b
}

// WithTheme sets the theme
func (b *Badge) WithTheme(theme *themes.Theme) *Badge {
	b.SetTheme(theme)
	return b
}

// View implements Component
func (b *Badge) View(width int) string {
	theme := b.Theme()
	var style lipgloss.Style

	switch b.variant {
	case BadgePrimary:
		style = theme.BadgePrimary
	case BadgeSuccess:
		style = theme.BadgeSuccess
	case BadgeWarning:
		style = theme.BadgeWarning
	case BadgeError:
		style = theme.BadgeError
	case BadgeInfo:
		style = theme.BadgeInfo
	default:
		style = theme.BadgeNeutral
	}

	return style.Render(b.text)
}

// StatusMessage displays a status with icon
type StatusMessage struct {
	BaseComponent

	message string
	status  StatusType
}

// StatusType defines the status type
type StatusType int

const (
	StatusSuccess StatusType = iota
	StatusError
	StatusWarning
	StatusInfo
	StatusPending
)

// NewStatusMessage creates a new status message
func NewStatusMessage(message string, status StatusType) *StatusMessage {
	return &StatusMessage{
		BaseComponent: NewBaseComponent(),
		message:       message,
		status:        status,
	}
}

// Success creates a success status message
func Success(message string) *StatusMessage {
	return NewStatusMessage(message, StatusSuccess)
}

// Error creates an error status message
func Error(message string) *StatusMessage {
	return NewStatusMessage(message, StatusError)
}

// Warning creates a warning status message
func Warning(message string) *StatusMessage {
	return NewStatusMessage(message, StatusWarning)
}

// Info creates an info status message
func Info(message string) *StatusMessage {
	return NewStatusMessage(message, StatusInfo)
}

// Pending creates a pending status message
func Pending(message string) *StatusMessage {
	return NewStatusMessage(message, StatusPending)
}

// WithTheme sets the theme
func (s *StatusMessage) WithTheme(theme *themes.Theme) *StatusMessage {
	s.SetTheme(theme)
	return s
}

// View implements Component
func (s *StatusMessage) View(width int) string {
	theme := s.Theme()
	var style lipgloss.Style
	var icon string

	switch s.status {
	case StatusSuccess:
		style = theme.Success
		icon = themes.IconCheck
	case StatusError:
		style = theme.Error
		icon = themes.IconCross
	case StatusWarning:
		style = theme.Warning
		icon = themes.IconWarning
	case StatusInfo:
		style = theme.Info
		icon = themes.IconInfo
	case StatusPending:
		style = theme.Blurred
		icon = themes.IconCircle
	}

	return style.Render(fmt.Sprintf("%s %s", icon, s.message))
}
