package component

import (
	"strings"

	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SplitDirection defines the split orientation
type SplitDirection int

const (
	SplitHorizontal SplitDirection = iota // Left | Right
	SplitVertical                         // Top / Bottom
)

// SplitPane represents a pane in the split view
type SplitPane struct {
	Content func(width, height int) string
	Model   tea.Model
	MinSize int
	MaxSize int
}

// SplitView provides a resizable split pane layout
type SplitView struct {
	BaseComponent
	FocusState

	direction   SplitDirection
	panes       [2]*SplitPane
	ratio       float64 // 0.0 to 1.0, portion of first pane
	activePane  int
	width       int
	height      int
	resizing    bool
	showDivider bool
	dividerChar string
}

// NewSplitView creates a new split view
func NewSplitView(direction SplitDirection) *SplitView {
	return &SplitView{
		BaseComponent: NewBaseComponent(),
		direction:     direction,
		ratio:         0.5,
		activePane:    0,
		showDivider:   true,
		dividerChar:   "│",
	}
}

// WithPanes sets both panes
func (s *SplitView) WithPanes(first, second *SplitPane) *SplitView {
	s.panes[0] = first
	s.panes[1] = second
	return s
}

// WithFirstPane sets the first pane
func (s *SplitView) WithFirstPane(pane *SplitPane) *SplitView {
	s.panes[0] = pane
	return s
}

// WithSecondPane sets the second pane
func (s *SplitView) WithSecondPane(pane *SplitPane) *SplitView {
	s.panes[1] = pane
	return s
}

// WithRatio sets the split ratio (0.0 to 1.0)
func (s *SplitView) WithRatio(ratio float64) *SplitView {
	s.ratio = clamp64(ratio, 0.1, 0.9)
	return s
}

// WithSize sets the container dimensions
func (s *SplitView) WithSize(width, height int) *SplitView {
	s.width = width
	s.height = height
	return s
}

// WithDivider enables/disables the divider
func (s *SplitView) WithDivider(show bool) *SplitView {
	s.showDivider = show
	return s
}

// WithDividerChar sets the divider character
func (s *SplitView) WithDividerChar(char string) *SplitView {
	s.dividerChar = char
	return s
}

// WithTheme sets the theme
func (s *SplitView) WithTheme(theme *themes.Theme) *SplitView {
	s.SetTheme(theme)
	return s
}

// Init implements tea.Model
func (s *SplitView) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, pane := range s.panes {
		if pane != nil && pane.Model != nil {
			cmds = append(cmds, pane.Model.Init())
		}
	}
	return tea.Batch(cmds...)
}

// Update implements tea.Model
func (s *SplitView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if s.Focused() {
			switch msg.String() {
			case "tab":
				// Switch active pane
				s.activePane = (s.activePane + 1) % 2

			case "shift+tab":
				s.activePane--
				if s.activePane < 0 {
					s.activePane = 1
				}

			case "ctrl+left", "ctrl+h":
				if s.direction == SplitHorizontal && !s.resizing {
					s.ratio = clamp64(s.ratio-0.05, 0.1, 0.9)
				}

			case "ctrl+right", "ctrl+l":
				if s.direction == SplitHorizontal && !s.resizing {
					s.ratio = clamp64(s.ratio+0.05, 0.1, 0.9)
				}

			case "ctrl+up", "ctrl+k":
				if s.direction == SplitVertical && !s.resizing {
					s.ratio = clamp64(s.ratio-0.05, 0.1, 0.9)
				}

			case "ctrl+down", "ctrl+j":
				if s.direction == SplitVertical && !s.resizing {
					s.ratio = clamp64(s.ratio+0.05, 0.1, 0.9)
				}

			case "=":
				// Reset to 50/50
				s.ratio = 0.5

			default:
				// Pass through to active pane
				if pane := s.panes[s.activePane]; pane != nil && pane.Model != nil {
					var cmd tea.Cmd
					pane.Model, cmd = pane.Model.Update(msg)
					cmds = append(cmds, cmd)
				}
			}
		}

	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height

		// Propagate to pane models
		for i, pane := range s.panes {
			if pane != nil && pane.Model != nil {
				w, h := s.paneSize(i)
				pane.Model, _ = pane.Model.Update(tea.WindowSizeMsg{Width: w, Height: h})
			}
		}

	default:
		// Pass through to active pane model
		if pane := s.panes[s.activePane]; pane != nil && pane.Model != nil {
			var cmd tea.Cmd
			pane.Model, cmd = pane.Model.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return s, tea.Batch(cmds...)
}

func (s *SplitView) paneSize(index int) (width, height int) {
	dividerSize := 0
	if s.showDivider {
		dividerSize = 1
	}

	if s.direction == SplitHorizontal {
		firstWidth := int(float64(s.width-dividerSize) * s.ratio)
		secondWidth := s.width - dividerSize - firstWidth

		// Apply constraints
		if pane := s.panes[0]; pane != nil {
			if pane.MinSize > 0 && firstWidth < pane.MinSize {
				firstWidth = pane.MinSize
				secondWidth = s.width - dividerSize - firstWidth
			}
			if pane.MaxSize > 0 && firstWidth > pane.MaxSize {
				firstWidth = pane.MaxSize
				secondWidth = s.width - dividerSize - firstWidth
			}
		}
		if pane := s.panes[1]; pane != nil {
			if pane.MinSize > 0 && secondWidth < pane.MinSize {
				secondWidth = pane.MinSize
				firstWidth = s.width - dividerSize - secondWidth
			}
		}

		if index == 0 {
			return firstWidth, s.height
		}
		return secondWidth, s.height
	}

	// Vertical split
	firstHeight := int(float64(s.height-dividerSize) * s.ratio)
	secondHeight := s.height - dividerSize - firstHeight

	// Apply constraints
	if pane := s.panes[0]; pane != nil {
		if pane.MinSize > 0 && firstHeight < pane.MinSize {
			firstHeight = pane.MinSize
			secondHeight = s.height - dividerSize - firstHeight
		}
		if pane.MaxSize > 0 && firstHeight > pane.MaxSize {
			firstHeight = pane.MaxSize
			secondHeight = s.height - dividerSize - firstHeight
		}
	}
	if pane := s.panes[1]; pane != nil {
		if pane.MinSize > 0 && secondHeight < pane.MinSize {
			secondHeight = pane.MinSize
			firstHeight = s.height - dividerSize - secondHeight
		}
	}

	if index == 0 {
		return s.width, firstHeight
	}
	return s.width, secondHeight
}

// View implements tea.Model
func (s *SplitView) View() string {
	return s.ViewWidth(s.width)
}

// ViewWidth renders the split view at a specific width (implements Component)
func (s *SplitView) ViewWidth(width int) string {
	if width == 0 {
		width = s.width
	}
	if width == 0 {
		width = 80
	}

	theme := s.Theme()

	// Render both panes
	paneViews := make([]string, 2)
	for i, pane := range s.panes {
		w, h := s.paneSize(i)
		if pane == nil {
			paneViews[i] = ""
			continue
		}

		var content string
		if pane.Model != nil {
			content = pane.Model.View()
		} else if pane.Content != nil {
			content = pane.Content(w, h)
		}

		// Apply pane styling
		style := lipgloss.NewStyle().Width(w).Height(h)
		if s.Focused() && i == s.activePane {
			style = style.BorderForeground(theme.Palette.BorderFocus)
		}

		paneViews[i] = style.Render(content)
	}

	// Create divider
	var divider string
	if s.showDivider {
		if s.direction == SplitHorizontal {
			divider = theme.Divider.Render(strings.Repeat(s.dividerChar+"\n", s.height))
			divider = strings.TrimSuffix(divider, "\n")
		} else {
			if s.dividerChar == "│" {
				divider = theme.Divider.Render(strings.Repeat("─", width))
			} else {
				divider = theme.Divider.Render(strings.Repeat(s.dividerChar, width))
			}
		}
	}

	// Join panes
	if s.direction == SplitHorizontal {
		return lipgloss.JoinHorizontal(lipgloss.Top, paneViews[0], divider, paneViews[1])
	}
	return lipgloss.JoinVertical(lipgloss.Left, paneViews[0], divider, paneViews[1])
}

// ActivePane returns the index of the active pane
func (s *SplitView) ActivePane() int {
	return s.activePane
}

// SetActivePane sets the active pane
func (s *SplitView) SetActivePane(index int) {
	s.activePane = clamp(index, 0, 1)
}

// Ratio returns the current split ratio
func (s *SplitView) Ratio() float64 {
	return s.ratio
}

// SetRatio sets the split ratio
func (s *SplitView) SetRatio(ratio float64) {
	s.ratio = clamp64(ratio, 0.1, 0.9)
}

// Focus implements FocusableComponent
func (s *SplitView) Focus() tea.Cmd {
	s.FocusState.Focus()
	return nil
}

// PaneModel returns the model for a pane
func (s *SplitView) PaneModel(index int) tea.Model {
	if index >= 0 && index < 2 && s.panes[index] != nil {
		return s.panes[index].Model
	}
	return nil
}

// SetPaneModel sets the model for a pane
func (s *SplitView) SetPaneModel(index int, model tea.Model) {
	if index >= 0 && index < 2 {
		if s.panes[index] == nil {
			s.panes[index] = &SplitPane{}
		}
		s.panes[index].Model = model
	}
}
