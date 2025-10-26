package tea

import (
	"bib/internal/config"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type keymap struct {
	Enter key.Binding
	Quit  key.Binding
	Help  key.Binding
}

type Model struct {
	cfg  *config.BibConfig
	keys keymap
	help help.Model

	width  int
	height int
	ready  bool
}

func NewModel(cfg *config.BibConfig) Model {
	m := Model{
		cfg: cfg,
		keys: keymap{
			Enter: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "start")),
			Quit:  key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"), key.WithHelp("q", "quit")),
			Help:  key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "toggle help")),
		},
		help: help.New(),
	}
	m.help.ShowAll = false
	return m
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ready = true

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		case key.Matches(msg, m.keys.Enter):

		}
	}
	return m, nil
}

func (m Model) View() string {
	if !m.ready || m.width == 0 || m.height == 0 {
		return "\n  loading..."
	}

	return m.viewWelcome()
}

func (m Model) viewWelcome() string {

	return "Welcome to Bib!\n\nPress " + m.keys.Help.Help().Key + " to toggle help.\n"
}
