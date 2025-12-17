package component

import (
	"fmt"
	"strings"
	"time"

	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ToastType defines the toast notification type
type ToastType int

const (
	ToastInfo ToastType = iota
	ToastSuccess
	ToastWarning
	ToastError
)

// Toast represents a temporary notification
type Toast struct {
	BaseComponent

	message   string
	toastType ToastType
	duration  time.Duration
	visible   bool
	id        int
}

// ToastDismissMsg is sent when a toast should be dismissed
type ToastDismissMsg struct {
	ID int
}

var toastIDCounter int

// NewToast creates a new toast notification
func NewToast(message string, toastType ToastType) *Toast {
	toastIDCounter++
	return &Toast{
		BaseComponent: NewBaseComponent(),
		message:       message,
		toastType:     toastType,
		duration:      3 * time.Second,
		visible:       true,
		id:            toastIDCounter,
	}
}

// ToastInfo creates an info toast
func ToastInfoMsg(message string) *Toast {
	return NewToast(message, ToastInfo)
}

// ToastSuccess creates a success toast
func ToastSuccessMsg(message string) *Toast {
	return NewToast(message, ToastSuccess)
}

// ToastWarning creates a warning toast
func ToastWarningMsg(message string) *Toast {
	return NewToast(message, ToastWarning)
}

// ToastError creates an error toast
func ToastErrorMsg(message string) *Toast {
	return NewToast(message, ToastError)
}

// WithDuration sets the display duration
func (t *Toast) WithDuration(d time.Duration) *Toast {
	t.duration = d
	return t
}

// WithTheme sets the theme
func (t *Toast) WithTheme(theme *themes.Theme) *Toast {
	t.SetTheme(theme)
	return t
}

// Show starts the toast timer
func (t *Toast) Show() tea.Cmd {
	t.visible = true
	id := t.id
	return tea.Tick(t.duration, func(time.Time) tea.Msg {
		return ToastDismissMsg{ID: id}
	})
}

// Dismiss hides the toast
func (t *Toast) Dismiss() {
	t.visible = false
}

// IsVisible returns whether the toast is visible
func (t *Toast) IsVisible() bool {
	return t.visible
}

// ID returns the toast ID
func (t *Toast) ID() int {
	return t.id
}

// View implements Component
func (t *Toast) View(width int) string {
	if !t.visible {
		return ""
	}

	theme := t.Theme()
	var style lipgloss.Style
	var icon string

	switch t.toastType {
	case ToastSuccess:
		style = theme.ToastSuccess
		icon = themes.IconCheck
	case ToastError:
		style = theme.ToastError
		icon = themes.IconCross
	case ToastWarning:
		style = theme.ToastWarning
		icon = themes.IconWarning
	default:
		style = theme.ToastInfo
		icon = themes.IconInfo
	}

	content := fmt.Sprintf(" %s %s ", icon, t.message)
	return style.Render(content)
}

// ToastManager manages multiple toast notifications
type ToastManager struct {
	BaseComponent

	toasts    []*Toast
	maxToasts int
	position  ToastPosition
	width     int
}

// ToastPosition defines where toasts appear
type ToastPosition int

const (
	ToastTopRight ToastPosition = iota
	ToastTopLeft
	ToastTopCenter
	ToastBottomRight
	ToastBottomLeft
	ToastBottomCenter
)

// NewToastManager creates a new toast manager
func NewToastManager() *ToastManager {
	return &ToastManager{
		BaseComponent: NewBaseComponent(),
		toasts:        make([]*Toast, 0),
		maxToasts:     5,
		position:      ToastTopRight,
	}
}

// WithMaxToasts sets the maximum number of visible toasts
func (tm *ToastManager) WithMaxToasts(max int) *ToastManager {
	tm.maxToasts = max
	return tm
}

// WithPosition sets the toast position
func (tm *ToastManager) WithPosition(pos ToastPosition) *ToastManager {
	tm.position = pos
	return tm
}

// WithWidth sets the container width
func (tm *ToastManager) WithWidth(width int) *ToastManager {
	tm.width = width
	return tm
}

// Add adds a toast and returns the command to show it
func (tm *ToastManager) Add(toast *Toast) tea.Cmd {
	toast.SetTheme(tm.Theme())
	tm.toasts = append(tm.toasts, toast)

	// Remove oldest if over limit
	if len(tm.toasts) > tm.maxToasts {
		tm.toasts = tm.toasts[1:]
	}

	return toast.Show()
}

// AddInfo adds an info toast
func (tm *ToastManager) AddInfo(message string) tea.Cmd {
	return tm.Add(ToastInfoMsg(message))
}

// AddSuccess adds a success toast
func (tm *ToastManager) AddSuccess(message string) tea.Cmd {
	return tm.Add(ToastSuccessMsg(message))
}

// AddWarning adds a warning toast
func (tm *ToastManager) AddWarning(message string) tea.Cmd {
	return tm.Add(ToastWarningMsg(message))
}

// AddError adds an error toast
func (tm *ToastManager) AddError(message string) tea.Cmd {
	return tm.Add(ToastErrorMsg(message))
}

// Update handles toast dismiss messages
func (tm *ToastManager) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case ToastDismissMsg:
		// Find and dismiss the toast
		for _, toast := range tm.toasts {
			if toast.ID() == msg.ID {
				toast.Dismiss()
				break
			}
		}
		// Clean up dismissed toasts
		tm.cleanup()
	}
	return nil
}

func (tm *ToastManager) cleanup() {
	visible := make([]*Toast, 0)
	for _, toast := range tm.toasts {
		if toast.IsVisible() {
			visible = append(visible, toast)
		}
	}
	tm.toasts = visible
}

// View implements Component
func (tm *ToastManager) View(width int) string {
	if len(tm.toasts) == 0 {
		return ""
	}

	w := tm.width
	if w == 0 {
		w = width
	}

	var toastViews []string
	for _, toast := range tm.toasts {
		if toast.IsVisible() {
			toastViews = append(toastViews, toast.View(w))
		}
	}

	if len(toastViews) == 0 {
		return ""
	}

	content := strings.Join(toastViews, "\n")

	// Position the toasts
	switch tm.position {
	case ToastTopRight, ToastBottomRight:
		return lipgloss.PlaceHorizontal(w, lipgloss.Right, content)
	case ToastTopCenter, ToastBottomCenter:
		return lipgloss.PlaceHorizontal(w, lipgloss.Center, content)
	default:
		return content
	}
}

// HasToasts returns whether there are visible toasts
func (tm *ToastManager) HasToasts() bool {
	for _, toast := range tm.toasts {
		if toast.IsVisible() {
			return true
		}
	}
	return false
}
