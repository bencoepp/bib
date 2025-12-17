package tui

import (
	"fmt"
	"strings"

	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// WizardStep represents a single step in the wizard
type WizardStep struct {
	ID          string
	Title       string
	Description string
	// HelpText is displayed on the right side of the card (optional)
	HelpText string
	// View returns the content for this step
	View func(width int) string
	// Validate validates the step before proceeding (optional)
	Validate func() error
	// OnEnter is called when entering this step (optional)
	OnEnter func() tea.Cmd
	// OnExit is called when leaving this step (optional)
	OnExit func() error
	// ShouldSkip returns true if this step should be skipped based on current state
	ShouldSkip func() bool
}

// Wizard is a multi-step wizard with progress indicator
type Wizard struct {
	theme       *themes.Theme
	title       string
	description string
	steps       []WizardStep
	currentStep int
	width       int
	height      int
	err         error
	done        bool
	cancelled   bool

	// Card layout settings
	cardWidth     int
	helpWidth     int
	showHelp      bool
	centerContent bool

	// Child model for the current step's form
	stepModel tea.Model

	// Callbacks
	onComplete func() error
}

// WizardOption configures the wizard
type WizardOption func(*Wizard)

// WithCardWidth sets the width of the centered card
func WithCardWidth(width int) WizardOption {
	return func(w *Wizard) {
		w.cardWidth = width
	}
}

// WithHelpPanel enables the right-side help panel
func WithHelpPanel(width int) WizardOption {
	return func(w *Wizard) {
		w.showHelp = true
		w.helpWidth = width
	}
}

// WithCentering enables horizontal centering
func WithCentering(center bool) WizardOption {
	return func(w *Wizard) {
		w.centerContent = center
	}
}

// WithStepModel sets a child model for the current step
func WithStepModel(m tea.Model) WizardOption {
	return func(w *Wizard) {
		w.stepModel = m
	}
}

// NewWizard creates a new wizard
func NewWizard(title, description string, steps []WizardStep, onComplete func() error, opts ...WizardOption) *Wizard {
	w := &Wizard{
		theme:         themes.Global().Active(),
		title:         title,
		description:   description,
		steps:         steps,
		currentStep:   0,
		onComplete:    onComplete,
		cardWidth:     60,
		helpWidth:     30,
		showHelp:      true,
		centerContent: true,
	}

	for _, opt := range opts {
		opt(w)
	}

	// Skip to first non-skippable step
	w.skipToNextValidStep(1)

	return w
}

// SetStepModel sets the model for the current step
func (w *Wizard) SetStepModel(m tea.Model) {
	w.stepModel = m
}

// CurrentStepIndex returns the current step index
func (w *Wizard) CurrentStepIndex() int {
	return w.currentStep
}

// CurrentStep returns the current step
func (w *Wizard) CurrentStep() *WizardStep {
	if w.currentStep >= 0 && w.currentStep < len(w.steps) {
		return &w.steps[w.currentStep]
	}
	return nil
}

// StepCount returns the total number of steps
func (w *Wizard) StepCount() int {
	return len(w.steps)
}

// VisibleStepCount returns the number of non-skipped steps
func (w *Wizard) VisibleStepCount() int {
	count := 0
	for _, step := range w.steps {
		if step.ShouldSkip == nil || !step.ShouldSkip() {
			count++
		}
	}
	return count
}

// Init implements tea.Model
func (w *Wizard) Init() tea.Cmd {
	var cmds []tea.Cmd

	// Initialize step model if present
	if w.stepModel != nil {
		cmds = append(cmds, w.stepModel.Init())
	}

	// Call OnEnter for the first step
	if step := w.CurrentStep(); step != nil && step.OnEnter != nil {
		if cmd := step.OnEnter(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return tea.Batch(cmds...)
}

// Update implements tea.Model
func (w *Wizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			w.cancelled = true
			w.done = true
			return w, tea.Quit

		case "esc":
			// Go back or cancel
			if w.currentStep > 0 {
				return w, w.prevStep()
			}
			w.cancelled = true
			w.done = true
			return w, tea.Quit
		}

	case tea.WindowSizeMsg:
		w.width = msg.Width
		w.height = msg.Height
	}

	// Update step model if present
	if w.stepModel != nil {
		var cmd tea.Cmd
		w.stepModel, cmd = w.stepModel.Update(msg)
		return w, cmd
	}

	return w, nil
}

// NextStep advances to the next step
func (w *Wizard) NextStep() tea.Cmd {
	return w.nextStep()
}

// PrevStep goes back to the previous step
func (w *Wizard) PrevStep() tea.Cmd {
	return w.prevStep()
}

// skipToNextValidStep moves to the next step that shouldn't be skipped
func (w *Wizard) skipToNextValidStep(direction int) bool {
	for {
		step := w.CurrentStep()
		if step == nil {
			return false
		}
		if step.ShouldSkip == nil || !step.ShouldSkip() {
			return true
		}
		// Skip this step
		if direction > 0 {
			if w.currentStep >= len(w.steps)-1 {
				return false
			}
			w.currentStep++
		} else {
			if w.currentStep <= 0 {
				return false
			}
			w.currentStep--
		}
	}
}

func (w *Wizard) nextStep() tea.Cmd {
	// Validate current step
	if step := w.CurrentStep(); step != nil && step.Validate != nil {
		if err := step.Validate(); err != nil {
			w.err = err
			return nil
		}
	}
	w.err = nil

	// Call OnExit for current step
	if step := w.CurrentStep(); step != nil && step.OnExit != nil {
		if err := step.OnExit(); err != nil {
			w.err = err
			return nil
		}
	}

	// Move to next step
	if w.currentStep < len(w.steps)-1 {
		w.currentStep++
		w.stepModel = nil // Clear step model for new step

		// Skip steps that should be skipped
		if !w.skipToNextValidStep(1) {
			// No more valid steps, complete
			return w.complete()
		}

		// Call OnEnter for new step
		if step := w.CurrentStep(); step != nil && step.OnEnter != nil {
			return step.OnEnter()
		}
	} else {
		return w.complete()
	}

	return nil
}

func (w *Wizard) complete() tea.Cmd {
	if w.onComplete != nil {
		if err := w.onComplete(); err != nil {
			w.err = err
			return nil
		}
	}
	w.done = true
	return tea.Quit
}

func (w *Wizard) prevStep() tea.Cmd {
	if w.currentStep > 0 {
		w.currentStep--
		w.stepModel = nil
		w.err = nil

		// Skip steps that should be skipped (going backwards)
		w.skipToNextValidStep(-1)

		// Call OnEnter for the step
		if step := w.CurrentStep(); step != nil && step.OnEnter != nil {
			return step.OnEnter()
		}
	}
	return nil
}

// View implements tea.Model
func (w *Wizard) View() string {
	var b strings.Builder

	// Calculate available width - use terminal width
	availableWidth := w.width
	if availableWidth == 0 {
		availableWidth = 80 // Sensible default for terminals
	}
	availableHeight := w.height
	if availableHeight == 0 {
		availableHeight = 24
	}

	// Render header
	header := w.renderHeader()
	b.WriteString(header)
	b.WriteString("\n\n")

	// Render progress indicator
	progress := w.renderProgress()
	b.WriteString(progress)
	b.WriteString("\n\n")

	// Render main content area with card and optional help panel
	content := w.renderCardWithHelp(availableWidth)
	b.WriteString(content)

	// Error message if any
	if w.err != nil {
		errMsg := w.theme.Error.Render(fmt.Sprintf("%s %s", themes.IconCross, w.err.Error()))
		b.WriteString("\n")
		b.WriteString(errMsg)
		b.WriteString("\n")
	}

	// Navigation help
	b.WriteString("\n")
	help := w.renderHelp()
	b.WriteString(help)

	// Get the full content
	fullContent := b.String()

	// Center everything if centering is enabled
	if w.centerContent && availableWidth > 0 && availableHeight > 0 {
		// Use lipgloss.Place for 2D centering
		fullContent = lipgloss.Place(
			availableWidth,
			availableHeight,
			lipgloss.Center,
			lipgloss.Center,
			fullContent,
		)
	}

	return fullContent
}

// SetSize updates the wizard dimensions
func (w *Wizard) SetSize(width, height int) {
	w.width = width
	w.height = height
}

func (w *Wizard) renderHeader() string {
	// Title
	title := w.theme.Title.Render(w.title)

	// Description
	desc := ""
	if w.description != "" {
		desc = "\n" + w.theme.Description.Render(w.description)
	}

	return title + desc
}

func (w *Wizard) renderProgress() string {
	var parts []string

	visibleIndex := 0
	for i, step := range w.steps {
		// Skip steps that should be skipped
		if step.ShouldSkip != nil && step.ShouldSkip() {
			continue
		}

		var stepStyle lipgloss.Style
		var icon string

		if i < w.currentStep {
			// Completed step
			stepStyle = w.theme.StepComplete
			icon = themes.IconCheck
		} else if i == w.currentStep {
			// Current step
			stepStyle = w.theme.StepCurrent
			icon = themes.IconDot
		} else {
			// Pending step
			stepStyle = w.theme.StepPending
			icon = themes.IconCircle
		}

		stepNum := w.theme.StepNumber.Render(icon)
		stepTitle := stepStyle.Render(step.Title)
		parts = append(parts, fmt.Sprintf("%s %s", stepNum, stepTitle))
		visibleIndex++
	}

	// Join with separator
	separator := w.theme.Blurred.Render(" ─── ")
	return strings.Join(parts, separator)
}

func (w *Wizard) renderCardWithHelp(availableWidth int) string {
	step := w.CurrentStep()
	if step == nil {
		return ""
	}

	// Calculate card width
	cardWidth := w.cardWidth
	if cardWidth > availableWidth-4 {
		cardWidth = availableWidth - 4
	}
	if cardWidth < 40 {
		cardWidth = 40
	}

	// Build card content
	var cardContent strings.Builder

	// Step title
	cardContent.WriteString(w.theme.SectionTitle.Render(step.Title))
	cardContent.WriteString("\n")

	// Step description (inside card)
	if step.Description != "" {
		cardContent.WriteString(w.theme.Description.Render(step.Description))
		cardContent.WriteString("\n\n")
	}

	// Step model view if present
	if w.stepModel != nil {
		cardContent.WriteString(w.stepModel.View())
	} else if step.View != nil {
		cardContent.WriteString(step.View(cardWidth - 4))
	}

	// Create the card with border
	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(w.theme.Palette.BorderFocus).
		Padding(1, 2).
		Width(cardWidth)

	card := cardStyle.Render(cardContent.String())

	// If we have help text and showHelp is enabled, render side panel
	if w.showHelp && step.HelpText != "" {
		helpWidth := w.helpWidth
		if cardWidth+helpWidth+4 > availableWidth {
			helpWidth = availableWidth - cardWidth - 4
		}

		if helpWidth > 10 {
			// Build help panel
			helpStyle := lipgloss.NewStyle().
				Foreground(w.theme.Palette.TextMuted).
				Width(helpWidth).
				PaddingLeft(2)

			helpContent := w.renderHelpPanel(step.HelpText, helpWidth-4)
			helpPanel := helpStyle.Render(helpContent)

			// Join card and help horizontally
			return lipgloss.JoinHorizontal(lipgloss.Top, card, helpPanel)
		}
	}

	// Just the card
	return card
}

func (w *Wizard) renderHelpPanel(text string, width int) string {
	var b strings.Builder

	// Help icon and title
	b.WriteString(w.theme.Info.Render(fmt.Sprintf("%s Help", themes.IconInfo)))
	b.WriteString("\n\n")

	// Wrap and render help text
	lines := wrapText(text, width)
	for _, line := range lines {
		b.WriteString(w.theme.Description.Render(line))
		b.WriteString("\n")
	}

	return b.String()
}

func (w *Wizard) renderHelp() string {
	var keys []string

	if w.currentStep > 0 {
		keys = append(keys, fmt.Sprintf("%s back", w.theme.HelpKey.Render("esc")))
	}

	visibleRemaining := 0
	for i := w.currentStep + 1; i < len(w.steps); i++ {
		if w.steps[i].ShouldSkip == nil || !w.steps[i].ShouldSkip() {
			visibleRemaining++
		}
	}

	if visibleRemaining > 0 {
		keys = append(keys, fmt.Sprintf("%s next", w.theme.HelpKey.Render("enter")))
	} else {
		keys = append(keys, fmt.Sprintf("%s finish", w.theme.HelpKey.Render("enter")))
	}

	keys = append(keys, fmt.Sprintf("%s quit", w.theme.HelpKey.Render("ctrl+c")))

	return w.theme.Help.Render(strings.Join(keys, w.theme.HelpDesc.Render(" • ")))
}

// IsDone returns true if the wizard is complete
func (w *Wizard) IsDone() bool {
	return w.done
}

// IsCancelled returns true if the wizard was cancelled
func (w *Wizard) IsCancelled() bool {
	return w.cancelled
}

// Error returns any error from the wizard
func (w *Wizard) Error() error {
	return w.err
}

// wrapText wraps text to fit within the given width
func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	var lines []string
	words := strings.Fields(text)
	if len(words) == 0 {
		return lines
	}

	currentLine := words[0]
	for _, word := range words[1:] {
		if len(currentLine)+1+len(word) <= width {
			currentLine += " " + word
		} else {
			lines = append(lines, currentLine)
			currentLine = word
		}
	}
	lines = append(lines, currentLine)

	return lines
}
