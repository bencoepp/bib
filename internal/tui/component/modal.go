package component

import (
	"strings"

	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ModalAction represents a button action in the modal
type ModalAction struct {
	Label   string
	Key     string
	Primary bool
	Danger  bool
	Handler func() tea.Cmd
}

// Modal is an overlay dialog component
type Modal struct {
	BaseComponent
	FocusState

	title        string
	content      string
	contentView  func(width int) string
	actions      []ModalAction
	activeAction int
	visible      bool
	width        int
	height       int
	containerW   int
	containerH   int
}

// NewModal creates a new modal
func NewModal() *Modal {
	return &Modal{
		BaseComponent: NewBaseComponent(),
		actions:       make([]ModalAction, 0),
		visible:       false,
		width:         60,
		height:        0, // Auto
	}
}

// WithTitle sets the modal title
func (m *Modal) WithTitle(title string) *Modal {
	m.title = title
	return m
}

// WithContent sets the modal content text
func (m *Modal) WithContent(content string) *Modal {
	m.content = content
	return m
}

// WithContentView sets a dynamic content renderer
func (m *Modal) WithContentView(view func(width int) string) *Modal {
	m.contentView = view
	return m
}

// WithSize sets the modal dimensions
func (m *Modal) WithSize(width, height int) *Modal {
	m.width = width
	m.height = height
	return m
}

// WithActions sets the modal actions
func (m *Modal) WithActions(actions ...ModalAction) *Modal {
	m.actions = actions
	if len(actions) > 0 {
		m.activeAction = len(actions) - 1 // Default to last (usually primary)
	}
	return m
}

// AddAction adds an action button
func (m *Modal) AddAction(label, key string, handler func() tea.Cmd) *Modal {
	m.actions = append(m.actions, ModalAction{
		Label:   label,
		Key:     key,
		Handler: handler,
	})
	return m
}

// AddPrimaryAction adds a primary action button
func (m *Modal) AddPrimaryAction(label, key string, handler func() tea.Cmd) *Modal {
	m.actions = append(m.actions, ModalAction{
		Label:   label,
		Key:     key,
		Primary: true,
		Handler: handler,
	})
	m.activeAction = len(m.actions) - 1
	return m
}

// AddDangerAction adds a danger action button
func (m *Modal) AddDangerAction(label, key string, handler func() tea.Cmd) *Modal {
	m.actions = append(m.actions, ModalAction{
		Label:   label,
		Key:     key,
		Danger:  true,
		Handler: handler,
	})
	return m
}

// WithTheme sets the theme
func (m *Modal) WithTheme(theme *themes.Theme) *Modal {
	m.SetTheme(theme)
	return m
}

// Show makes the modal visible
func (m *Modal) Show() {
	m.visible = true
	m.FocusState.Focus()
}

// Hide hides the modal
func (m *Modal) Hide() {
	m.visible = false
	m.FocusState.Blur()
}

// IsVisible returns whether the modal is visible
func (m *Modal) IsVisible() bool {
	return m.visible
}

// SetContainerSize sets the container dimensions for centering
func (m *Modal) SetContainerSize(width, height int) {
	m.containerW = width
	m.containerH = height
}

// Init implements tea.Model
func (m *Modal) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m *Modal) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.visible || !m.Focused() {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.Hide()
			return m, nil

		case "tab", "right", "l":
			if len(m.actions) > 0 {
				m.activeAction = (m.activeAction + 1) % len(m.actions)
			}

		case "shift+tab", "left", "h":
			if len(m.actions) > 0 {
				m.activeAction--
				if m.activeAction < 0 {
					m.activeAction = len(m.actions) - 1
				}
			}

		case "enter":
			if m.activeAction >= 0 && m.activeAction < len(m.actions) {
				action := m.actions[m.activeAction]
				if action.Handler != nil {
					return m, action.Handler()
				}
			}

		default:
			// Check for action keys
			for i, action := range m.actions {
				if action.Key != "" && msg.String() == action.Key {
					m.activeAction = i
					if action.Handler != nil {
						return m, action.Handler()
					}
				}
			}
		}

	case tea.WindowSizeMsg:
		m.containerW = msg.Width
		m.containerH = msg.Height
	}

	return m, nil
}

// View implements tea.Model
func (m *Modal) View() string {
	return m.ViewWidth(m.width)
}

// ViewWidth renders the modal at a specific width (implements Component)
func (m *Modal) ViewWidth(width int) string {
	if !m.visible {
		return ""
	}

	theme := m.Theme()

	// Calculate dimensions
	modalWidth := m.width
	if width > 0 && modalWidth > width-4 {
		modalWidth = width - 4
	}
	if modalWidth < 30 {
		modalWidth = 30
	}

	contentWidth := modalWidth - 4 // Account for padding

	// Build content
	var contentBuilder strings.Builder

	// Title
	if m.title != "" {
		contentBuilder.WriteString(theme.ModalTitle.Render(m.title))
		contentBuilder.WriteString("\n\n")
	}

	// Content
	if m.contentView != nil {
		contentBuilder.WriteString(m.contentView(contentWidth))
	} else if m.content != "" {
		// Wrap content
		lines := WrapText(m.content, contentWidth)
		contentBuilder.WriteString(theme.ModalContent.Render(strings.Join(lines, "\n")))
	}

	// Actions
	if len(m.actions) > 0 {
		contentBuilder.WriteString("\n\n")
		var buttons []string
		for i, action := range m.actions {
			var style lipgloss.Style
			if action.Danger {
				style = theme.ButtonDanger
			} else if action.Primary || i == m.activeAction {
				style = theme.ButtonPrimary
			} else {
				style = theme.ButtonSecondary
			}

			if i == m.activeAction {
				style = style.Underline(true)
			}

			label := action.Label
			if action.Key != "" {
				label += " (" + action.Key + ")"
			}
			buttons = append(buttons, style.Render(label))
		}
		contentBuilder.WriteString(theme.ModalFooter.Render(strings.Join(buttons, "  ")))
	}

	// Create modal container
	modalStyle := theme.ModalContainer.
		Width(modalWidth)

	if m.height > 0 {
		modalStyle = modalStyle.Height(m.height)
	}

	modal := modalStyle.Render(contentBuilder.String())

	// Center in container if dimensions are set
	if m.containerW > 0 && m.containerH > 0 {
		return lipgloss.Place(
			m.containerW,
			m.containerH,
			lipgloss.Center,
			lipgloss.Center,
			modal,
		)
	}

	return modal
}

// Focus implements FocusableComponent
func (m *Modal) Focus() tea.Cmd {
	m.FocusState.Focus()
	return nil
}

// ConfirmModal creates a simple confirmation modal
func ConfirmModal(title, message string, onConfirm, onCancel func() tea.Cmd) *Modal {
	return NewModal().
		WithTitle(title).
		WithContent(message).
		AddAction("Cancel", "esc", onCancel).
		AddPrimaryAction("Confirm", "enter", onConfirm)
}

// AlertModal creates a simple alert modal
func AlertModal(title, message string, onClose func() tea.Cmd) *Modal {
	return NewModal().
		WithTitle(title).
		WithContent(message).
		AddPrimaryAction("OK", "enter", onClose)
}

// DangerModal creates a danger confirmation modal
func DangerModal(title, message string, onConfirm, onCancel func() tea.Cmd) *Modal {
	return NewModal().
		WithTitle(title).
		WithContent(message).
		AddAction("Cancel", "esc", onCancel).
		AddDangerAction("Delete", "enter", onConfirm)
}
