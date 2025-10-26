package root

import (
	"bib/internal/ui/components/statusbar"
	"bib/internal/ui/keys"
	"bib/internal/ui/styles"
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	width     int
	height    int
	theme     styles.Theme
	themeMode string // "dark" or "light"
	keys      keys.KeyMap
	help      help.Model
	spin      spinner.Model
	status    statusbar.Model

	loading bool
	result  string
}

func New() Model {
	theme, mode := styles.DetectDefault()

	spin := spinner.New()
	spin.Spinner = spinner.Points

	m := Model{
		theme:     theme,
		themeMode: mode,
		keys:      keys.DefaultKeyMap(),
		help:      help.New(),
		spin:      spin,
		status:    statusbar.New(theme),
		loading:   true,
	}
	m.status.Left = "Starting..."
	m.status.Right = "Theme: " + m.themeMode
	return m
}

func NewWithTheme(mode string) Model {
	theme, m := styles.FromMode(mode)
	spin := spinner.New()
	spin.Spinner = spinner.Points

	model := Model{
		theme:     theme,
		themeMode: m,
		keys:      keys.DefaultKeyMap(),
		help:      help.New(),
		spin:      spin,
		status:    statusbar.New(theme),
		loading:   true,
	}
	model.status.Left = "Starting..."
	model.status.Right = "Theme: " + model.themeMode
	return model
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spin.Tick,
		func() tea.Msg {
			return workDoneMsg("Hello, World!")
		},
	)
}

type workDoneMsg string

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = v.Width, v.Height
		m.status, _ = m.status.Update(v)
		return m, nil

	case workDoneMsg:
		m.loading = false
		m.result = string(v)
		m.status.Left = "Ready"
		m.status.Right = "Theme: " + m.themeMode + "  •  Press ? for help"
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(v, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(v, m.keys.Help):
			m.status.Left = "Help toggled"
			return m, nil
		case key.Matches(v, m.keys.ToggleTheme):
			m.toggleTheme()
			return m, nil
		}
	}

	var cmd tea.Cmd
	if m.loading {
		m.spin, cmd = m.spin.Update(msg)
	}
	m.status, _ = m.status.Update(msg)
	return m, cmd
}

func (m *Model) toggleTheme() {
	if m.themeMode == "dark" {
		m.theme, m.themeMode = styles.FromMode("light")
	} else {
		m.theme, m.themeMode = styles.FromMode("dark")
	}
	m.status.SetTheme(m.theme)
	m.status.Right = "Theme: " + m.themeMode + "  •  Press ? for help"
}

func (m Model) View() string {
	header := m.theme.Title.Render("My App")
	sub := m.theme.Subtitle.Render("Shared Bubble Tea model used by TUI and SSH")

	body := ""
	if m.loading {
		body = lipgloss.JoinHorizontal(lipgloss.Left, m.spin.View(), " Loading...")
	} else {
		body = m.theme.Text.Render(fmt.Sprintf("Result: %s", m.result))
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		sub,
		"",
		m.theme.Panel.Render(body),
	)

	status := m.status.View()
	return lipgloss.JoinVertical(lipgloss.Left, content, "", status)
}
