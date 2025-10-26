package statusbar

import (
	"bib/internal/ui/styles"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	Left  string
	Right string
	width int
	style lipgloss.Style
	theme styles.Theme
}

func New(theme styles.Theme) Model {
	m := Model{theme: theme}
	m.rebuild()
	return m
}

func (m *Model) rebuild() {
	m.style = m.theme.Muted.Copy().Padding(0, 1)
}

func (m *Model) SetTheme(theme styles.Theme) {
	m.theme = theme
	m.rebuild()
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = v.Width
	}
	return m, nil
}

func (m Model) View() string {
	left := m.Left
	right := m.Right

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 0 {
		gap = 0
	}
	space := strings.Repeat(" ", gap)
	return m.style.Render(left + space + right)
}
