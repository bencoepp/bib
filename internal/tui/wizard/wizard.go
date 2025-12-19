// Package wizard provides a data-driven wizard system for the bib TUI.
//
// Wizards are defined in YAML files and support:
//   - Multiple step types (info, form, select, confirm)
//   - i18n for all text content
//   - Conditional step skipping
//   - Validation rules
//   - Dynamic field visibility
package wizard

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bib/internal/tui/i18n"
	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"
)

//go:embed definitions/*.yaml
var embeddedDefinitions embed.FS

// StepType identifies the type of wizard step.
type StepType string

const (
	StepTypeInfo    StepType = "info"
	StepTypeForm    StepType = "form"
	StepTypeSelect  StepType = "select"
	StepTypeConfirm StepType = "confirm"
)

// Definition defines a wizard loaded from YAML.
type Definition struct {
	ID    string         `yaml:"id"`
	Title LocalizedText  `yaml:"title"`
	Steps []StepDef      `yaml:"steps"`
	Data  map[string]any `yaml:"-"` // Runtime data
}

// LocalizedText supports i18n keys with fallback.
type LocalizedText struct {
	Key     string `yaml:"key"`
	Default string `yaml:"default"`
}

// Text returns the localized text.
func (l LocalizedText) Text(i *i18n.I18n) string {
	if i != nil && l.Key != "" && i.Has(l.Key) {
		return i.T(l.Key)
	}
	return l.Default
}

// StepDef defines a wizard step.
type StepDef struct {
	ID          string        `yaml:"id"`
	Type        StepType      `yaml:"type"`
	Title       LocalizedText `yaml:"title"`
	Description LocalizedText `yaml:"description"`
	Content     LocalizedText `yaml:"content"`
	Help        LocalizedText `yaml:"help"`
	SkipIf      string        `yaml:"skip_if"`
	Fields      []FieldDef    `yaml:"fields"`
	Options     []OptionDef   `yaml:"options"`
}

// FieldDef defines a form field.
type FieldDef struct {
	ID          string          `yaml:"id"`
	Type        string          `yaml:"type"` // text, textarea, number, select, confirm, password
	Label       LocalizedText   `yaml:"label"`
	Placeholder string          `yaml:"placeholder"`
	Help        LocalizedText   `yaml:"help"`
	Required    bool            `yaml:"required"`
	Default     any             `yaml:"default"`
	ShowIf      string          `yaml:"show_if"`
	Options     []OptionDef     `yaml:"options"`
	Validation  []ValidationDef `yaml:"validation"`
}

// OptionDef defines a select option.
type OptionDef struct {
	Value string        `yaml:"value"`
	Label LocalizedText `yaml:"label"`
}

// ValidationDef defines a validation rule.
type ValidationDef struct {
	Rule    string         `yaml:"rule"`
	Message LocalizedText  `yaml:"message"`
	Args    map[string]any `yaml:"args"`
}

// Wizard is the main wizard model.
type Wizard struct {
	definition *Definition
	theme      *themes.Theme
	i18n       *i18n.I18n

	// State
	currentStep int
	data        map[string]any
	stepForms   map[int]*huh.Form

	// Dimensions
	width  int
	height int

	// Completion
	done      bool
	cancelled bool
	err       error

	// Callbacks
	onComplete func(data map[string]any) error
}

// WizardOption configures the wizard.
type WizardOption func(*Wizard)

// WithTheme sets the wizard theme.
func WithTheme(theme *themes.Theme) WizardOption {
	return func(w *Wizard) {
		w.theme = theme
	}
}

// WithI18n sets the i18n instance.
func WithI18n(i *i18n.I18n) WizardOption {
	return func(w *Wizard) {
		w.i18n = i
	}
}

// WithData sets initial data.
func WithData(data map[string]any) WizardOption {
	return func(w *Wizard) {
		for k, v := range data {
			w.data[k] = v
		}
	}
}

// OnComplete sets the completion callback.
func OnComplete(fn func(data map[string]any) error) WizardOption {
	return func(w *Wizard) {
		w.onComplete = fn
	}
}

// Load loads a wizard definition from the embedded filesystem.
func Load(id string, opts ...WizardOption) (*Wizard, error) {
	path := fmt.Sprintf("definitions/%s.yaml", id)
	data, err := embeddedDefinitions.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("wizard definition not found: %s", id)
	}

	var def Definition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("failed to parse wizard definition: %w", err)
	}

	return New(&def, opts...), nil
}

// LoadFile loads a wizard definition from a file path.
func LoadFile(path string, opts ...WizardOption) (*Wizard, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read wizard file: %w", err)
	}

	var def Definition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("failed to parse wizard definition: %w", err)
	}

	return New(&def, opts...), nil
}

// LoadDir loads all wizard definitions from a directory.
func LoadDir(dir string) (map[string]*Definition, error) {
	defs := make(map[string]*Definition)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var def Definition
		if err := yaml.Unmarshal(data, &def); err != nil {
			continue
		}

		defs[def.ID] = &def
	}

	return defs, nil
}

// New creates a new wizard from a definition.
func New(def *Definition, opts ...WizardOption) *Wizard {
	w := &Wizard{
		definition:  def,
		theme:       themes.Global().Active(),
		i18n:        i18n.Global(),
		currentStep: 0,
		data:        make(map[string]any),
		stepForms:   make(map[int]*huh.Form),
	}

	for _, opt := range opts {
		opt(w)
	}

	// Initialize default data from fields
	for _, step := range def.Steps {
		for _, field := range step.Fields {
			if field.Default != nil {
				w.data[field.ID] = field.Default
			}
		}
	}

	// Skip to first visible step
	w.skipToNextVisible(1)

	return w
}

// Data returns the wizard data.
func (w *Wizard) Data() map[string]any {
	return w.data
}

// SetData sets a data value.
func (w *Wizard) SetData(key string, value any) {
	w.data[key] = value
}

// GetData gets a data value.
func (w *Wizard) GetData(key string) any {
	return w.data[key]
}

// Done returns true if the wizard is complete.
func (w *Wizard) Done() bool {
	return w.done
}

// Cancelled returns true if the wizard was cancelled.
func (w *Wizard) Cancelled() bool {
	return w.cancelled
}

// Error returns any error that occurred.
func (w *Wizard) Error() error {
	return w.err
}

// Init implements tea.Model.
func (w *Wizard) Init() tea.Cmd {
	// Initialize form for current step if needed
	step := w.currentStepDef()
	if step != nil && step.Type == StepTypeForm {
		return w.initStepForm(w.currentStep)
	}
	return nil
}

// Update implements tea.Model.
func (w *Wizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	step := w.currentStepDef()
	if step == nil {
		w.done = true
		return w, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			w.cancelled = true
			w.done = true
			return w, tea.Quit

		case "esc":
			// Go back
			if w.currentStep > 0 {
				w.skipToPrevious()
				return w, w.Init()
			}
			// On first step, cancel
			w.cancelled = true
			w.done = true
			return w, nil

		case "enter":
			// For info steps, proceed on enter
			if step.Type == StepTypeInfo {
				return w, w.nextStep()
			}
		}

	case tea.WindowSizeMsg:
		w.width = msg.Width
		w.height = msg.Height
	}

	// Forward to step form if exists
	if form, ok := w.stepForms[w.currentStep]; ok {
		newForm, cmd := form.Update(msg)
		w.stepForms[w.currentStep] = newForm.(*huh.Form)

		// Check if form is complete
		if newForm.(*huh.Form).State == huh.StateCompleted {
			return w, w.nextStep()
		}

		return w, cmd
	}

	return w, nil
}

// View implements tea.Model.
func (w *Wizard) View() string {
	step := w.currentStepDef()
	if step == nil {
		return ""
	}

	var b strings.Builder

	// Progress indicator
	b.WriteString(w.renderProgress())
	b.WriteString("\n\n")

	// Step title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(w.theme.Palette.Primary)
	b.WriteString(titleStyle.Render(step.Title.Text(w.i18n)))
	b.WriteString("\n")

	// Step description
	if step.Description.Default != "" || step.Description.Key != "" {
		descStyle := lipgloss.NewStyle().
			Foreground(w.theme.Palette.TextMuted)
		b.WriteString(descStyle.Render(step.Description.Text(w.i18n)))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Step content
	switch step.Type {
	case StepTypeInfo:
		b.WriteString(step.Content.Text(w.i18n))
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(w.theme.Palette.TextMuted).Render("Press Enter to continue"))

	case StepTypeForm:
		if form, ok := w.stepForms[w.currentStep]; ok {
			b.WriteString(form.View())
		}

	case StepTypeSelect:
		// TODO: Implement select rendering
		b.WriteString("Select step (TODO)")

	case StepTypeConfirm:
		// TODO: Implement confirm rendering
		b.WriteString("Confirm step (TODO)")
	}

	// Help text on the side (if width allows)
	if w.width > 80 && (step.Help.Default != "" || step.Help.Key != "") {
		// Split view with help on right
		helpStyle := lipgloss.NewStyle().
			Width(30).
			Padding(1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(w.theme.Palette.Border).
			Foreground(w.theme.Palette.TextMuted)

		mainWidth := w.width - 35
		main := lipgloss.NewStyle().Width(mainWidth).Render(b.String())
		help := helpStyle.Render(step.Help.Text(w.i18n))

		return lipgloss.JoinHorizontal(lipgloss.Top, main, help)
	}

	return b.String()
}

// renderProgress renders the progress indicator.
func (w *Wizard) renderProgress() string {
	visible := w.visibleSteps()
	current := w.currentVisibleIndex()

	var steps []string
	for i := range visible {
		style := w.theme.StepPending
		if i < current {
			style = w.theme.StepComplete
		} else if i == current {
			style = w.theme.StepCurrent
		}

		num := fmt.Sprintf("%d", i+1)
		steps = append(steps, style.Render(num))
	}

	return strings.Join(steps, " → ")
}

// currentStepDef returns the current step definition.
func (w *Wizard) currentStepDef() *StepDef {
	if w.currentStep >= 0 && w.currentStep < len(w.definition.Steps) {
		return &w.definition.Steps[w.currentStep]
	}
	return nil
}

// visibleSteps returns steps that aren't skipped.
func (w *Wizard) visibleSteps() []StepDef {
	var visible []StepDef
	for _, step := range w.definition.Steps {
		if !w.shouldSkip(&step) {
			visible = append(visible, step)
		}
	}
	return visible
}

// currentVisibleIndex returns the index of the current step among visible steps.
func (w *Wizard) currentVisibleIndex() int {
	idx := 0
	for i := 0; i < w.currentStep && i < len(w.definition.Steps); i++ {
		if !w.shouldSkip(&w.definition.Steps[i]) {
			idx++
		}
	}
	return idx
}

// shouldSkip evaluates if a step should be skipped.
func (w *Wizard) shouldSkip(step *StepDef) bool {
	if step.SkipIf == "" {
		return false
	}

	// Simple template evaluation
	// Supports: {{ .FieldName }} for boolean fields
	skipExpr := step.SkipIf
	skipExpr = strings.TrimPrefix(skipExpr, "{{")
	skipExpr = strings.TrimSuffix(skipExpr, "}}")
	skipExpr = strings.TrimSpace(skipExpr)

	// Handle negation
	negate := false
	if strings.HasPrefix(skipExpr, "!") || strings.HasPrefix(skipExpr, "not ") {
		negate = true
		skipExpr = strings.TrimPrefix(skipExpr, "!")
		skipExpr = strings.TrimPrefix(skipExpr, "not ")
		skipExpr = strings.TrimSpace(skipExpr)
	}

	// Get field value
	skipExpr = strings.TrimPrefix(skipExpr, ".")
	val, ok := w.data[skipExpr]
	if !ok {
		return negate
	}

	switch v := val.(type) {
	case bool:
		if negate {
			return !v
		}
		return v
	case string:
		if negate {
			return v == ""
		}
		return v != ""
	default:
		return negate
	}
}

// skipToNextVisible skips to the next visible step.
func (w *Wizard) skipToNextVisible(direction int) {
	for w.currentStep >= 0 && w.currentStep < len(w.definition.Steps) {
		step := &w.definition.Steps[w.currentStep]
		if !w.shouldSkip(step) {
			return
		}
		w.currentStep += direction
	}
}

// skipToPrevious moves to the previous visible step.
func (w *Wizard) skipToPrevious() {
	w.currentStep--
	for w.currentStep >= 0 {
		step := &w.definition.Steps[w.currentStep]
		if !w.shouldSkip(step) {
			return
		}
		w.currentStep--
	}
	if w.currentStep < 0 {
		w.currentStep = 0
	}
}

// nextStep advances to the next step.
func (w *Wizard) nextStep() tea.Cmd {
	// Save form data
	if step := w.currentStepDef(); step != nil && step.Type == StepTypeForm {
		// Data is already bound through huh form
	}

	w.currentStep++
	w.skipToNextVisible(1)

	if w.currentStep >= len(w.definition.Steps) {
		// Complete
		w.done = true
		if w.onComplete != nil {
			w.err = w.onComplete(w.data)
		}
		return nil
	}

	return w.Init()
}

// initStepForm creates a huh.Form for the current step.
func (w *Wizard) initStepForm(stepIdx int) tea.Cmd {
	if stepIdx >= len(w.definition.Steps) {
		return nil
	}

	step := &w.definition.Steps[stepIdx]
	if step.Type != StepTypeForm || len(step.Fields) == 0 {
		return nil
	}

	var fields []huh.Field
	for _, fieldDef := range step.Fields {
		field := w.createField(&fieldDef)
		if field != nil {
			fields = append(fields, field)
		}
	}

	group := huh.NewGroup(fields...)
	form := huh.NewForm(group)

	w.stepForms[stepIdx] = form

	return form.Init()
}

// createField creates a huh.Field from a field definition.
func (w *Wizard) createField(def *FieldDef) huh.Field {
	label := def.Label.Text(w.i18n)

	switch def.Type {
	case "text", "":
		// Get or create string pointer in data
		if _, ok := w.data[def.ID]; !ok {
			w.data[def.ID] = ""
		}
		val := w.data[def.ID].(string)

		input := huh.NewInput().
			Title(label).
			Placeholder(def.Placeholder).
			Value(&val)

		// Store updated value back
		// Note: huh handles this through the pointer

		return input

	case "password":
		if _, ok := w.data[def.ID]; !ok {
			w.data[def.ID] = ""
		}
		val := w.data[def.ID].(string)

		return huh.NewInput().
			Title(label).
			Placeholder(def.Placeholder).
			EchoMode(huh.EchoModePassword).
			Value(&val)

	case "confirm":
		if _, ok := w.data[def.ID]; !ok {
			w.data[def.ID] = false
		}
		val := w.data[def.ID].(bool)

		return huh.NewConfirm().
			Title(label).
			Value(&val)

	case "select":
		if _, ok := w.data[def.ID]; !ok {
			if len(def.Options) > 0 {
				w.data[def.ID] = def.Options[0].Value
			} else {
				w.data[def.ID] = ""
			}
		}
		val := w.data[def.ID].(string)

		options := make([]huh.Option[string], len(def.Options))
		for i, opt := range def.Options {
			options[i] = huh.NewOption(opt.Label.Text(w.i18n), opt.Value)
		}

		return huh.NewSelect[string]().
			Title(label).
			Options(options...).
			Value(&val)

	case "number":
		if _, ok := w.data[def.ID]; !ok {
			w.data[def.ID] = 0
		}
		// huh doesn't have a native number input, use text
		val := fmt.Sprintf("%v", w.data[def.ID])

		return huh.NewInput().
			Title(label).
			Placeholder(def.Placeholder).
			Value(&val)

	default:
		return nil
	}
}
