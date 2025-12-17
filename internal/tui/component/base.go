// Package component provides base interfaces and helpers for TUI components.
package component

import (
	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
)

// Renderer is the base interface for components that can render with width
type Renderer interface {
	// ViewWidth renders the component at the given width
	ViewWidth(width int) string
}

// ThemedRenderer can accept a theme and render
type ThemedRenderer interface {
	Renderer
	// WithTheme sets the theme for the component
	WithTheme(theme *themes.Theme) ThemedRenderer
}

// StatefulComponent is a component that manages its own state
// It implements tea.Model and can also render at specific widths
type StatefulComponent interface {
	tea.Model
	Renderer
}

// FocusableComponent can receive and lose focus
type FocusableComponent interface {
	StatefulComponent
	// Focus gives focus to the component
	Focus() tea.Cmd
	// Blur removes focus from the component
	Blur()
	// Focused returns whether the component has focus
	Focused() bool
}

// ValidatableComponent can validate its content
type ValidatableComponent interface {
	// Validate returns an error if validation fails
	Validate() error
	// SetError sets the validation error to display
	SetError(err error)
	// ClearError clears any validation error
	ClearError()
}

// ResizableComponent responds to size changes
type ResizableComponent interface {
	// SetSize updates the component dimensions
	SetSize(width, height int)
	// Width returns the current width
	Width() int
	// Height returns the current height
	Height() int
}

// SelectableComponent has selectable items
type SelectableComponent interface {
	// SelectedIndex returns the index of the selected item
	SelectedIndex() int
	// SetSelectedIndex sets the selected item by index
	SetSelectedIndex(index int)
	// SelectedValue returns the value of the selected item
	SelectedValue() interface{}
}

// ScrollableComponent supports scrolling
type ScrollableComponent interface {
	// ScrollUp scrolls up
	ScrollUp(lines int)
	// ScrollDown scrolls down
	ScrollDown(lines int)
	// ScrollTop scrolls to the top
	ScrollTop()
	// ScrollBottom scrolls to the bottom
	ScrollBottom()
	// ScrollOffset returns the current scroll offset
	ScrollOffset() int
	// SetScrollOffset sets the scroll offset
	SetScrollOffset(offset int)
}

// AnimatableComponent supports animations
type AnimatableComponent interface {
	// StartAnimation starts the component's animation
	StartAnimation() tea.Cmd
	// StopAnimation stops the animation
	StopAnimation()
	// IsAnimating returns whether animation is active
	IsAnimating() bool
}

// Props is a generic interface for component properties
type Props interface{}

// Option is a functional option for configuring components
type Option[T any] func(*T)

// Apply applies options to a component
func Apply[T any](c *T, opts ...Option[T]) {
	for _, opt := range opts {
		opt(c)
	}
}

// BaseComponent provides common functionality for components
type BaseComponent struct {
	theme  *themes.Theme
	width  int
	height int
}

// NewBaseComponent creates a new base component
func NewBaseComponent() BaseComponent {
	return BaseComponent{
		theme: themes.Global().Active(),
	}
}

// Theme returns the component's theme
func (b *BaseComponent) Theme() *themes.Theme {
	if b.theme == nil {
		b.theme = themes.Global().Active()
	}
	return b.theme
}

// SetTheme sets the component's theme
func (b *BaseComponent) SetTheme(theme *themes.Theme) {
	b.theme = theme
}

// SetSize sets the component dimensions
func (b *BaseComponent) SetSize(width, height int) {
	b.width = width
	b.height = height
}

// GetWidth returns the component width
func (b *BaseComponent) GetWidth() int {
	return b.width
}

// GetHeight returns the component height
func (b *BaseComponent) GetHeight() int {
	return b.height
}

// FocusState tracks focus for focusable components
type FocusState struct {
	focused bool
}

// Focus sets the focus state to true
func (f *FocusState) Focus() {
	f.focused = true
}

// Blur sets the focus state to false
func (f *FocusState) Blur() {
	f.focused = false
}

// Focused returns whether the component has focus
func (f *FocusState) Focused() bool {
	return f.focused
}

// ValidationState tracks validation for validatable components
type ValidationState struct {
	err error
}

// SetError sets the validation error
func (v *ValidationState) SetError(err error) {
	v.err = err
}

// ClearError clears the validation error
func (v *ValidationState) ClearError() {
	v.err = nil
}

// Error returns the current validation error
func (v *ValidationState) Error() error {
	return v.err
}

// HasError returns whether there is a validation error
func (v *ValidationState) HasError() bool {
	return v.err != nil
}

// ScrollState tracks scroll position
type ScrollState struct {
	offset    int
	maxOffset int
}

// SetOffset sets the scroll offset
func (s *ScrollState) SetOffset(offset int) {
	s.offset = clamp(offset, 0, s.maxOffset)
}

// SetMaxOffset sets the maximum scroll offset
func (s *ScrollState) SetMaxOffset(max int) {
	s.maxOffset = max
	if s.offset > max {
		s.offset = max
	}
}

// Offset returns the current scroll offset
func (s *ScrollState) Offset() int {
	return s.offset
}

// MaxOffset returns the maximum scroll offset
func (s *ScrollState) MaxOffset() int {
	return s.maxOffset
}

// ScrollUp scrolls up by the given number of lines
func (s *ScrollState) ScrollUp(lines int) {
	s.SetOffset(s.offset - lines)
}

// ScrollDown scrolls down by the given number of lines
func (s *ScrollState) ScrollDown(lines int) {
	s.SetOffset(s.offset + lines)
}

// ScrollTop scrolls to the top
func (s *ScrollState) ScrollTop() {
	s.offset = 0
}

// ScrollBottom scrolls to the bottom
func (s *ScrollState) ScrollBottom() {
	s.offset = s.maxOffset
}
