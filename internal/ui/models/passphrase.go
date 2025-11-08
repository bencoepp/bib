package models

import (
	"bib/internal/ui"
	"errors"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

type PassphraseModel struct {
	form         *huh.Form
	width        int
	height       int
	ready        bool
	cancelled    bool
	value        string
	passPtr      *string // points to bound huh input value
	boxWidth     int
	useAltScreen bool
}

func NewPassphraseModel(prompt string) PassphraseModel {
	if prompt == "" {
		prompt = "Enter passphrase"
	}

	var pass string

	input := huh.NewInput().
		Title(prompt).
		Key("passphrase").
		EchoMode(huh.EchoModePassword). // masks with • by default
		Value(&pass)

	form := huh.NewForm(
		huh.NewGroup(input),
	).WithWidth(50)

	return PassphraseModel{
		form:         form,
		passPtr:      &pass,
		boxWidth:     50,
		useAltScreen: true,
	}
}

func (m PassphraseModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m PassphraseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		}
	}

	// Update form
	f, cmd := m.form.Update(msg)
	if nf, ok := f.(*huh.Form); ok {
		m.form = nf
	}

	// Check state (FIELD, not method)
	switch m.form.State {
	case huh.StateCompleted:
		// capture the final value
		if m.passPtr != nil {
			m.value = *m.passPtr
		}
		return m, tea.Quit
	case huh.StateAborted:
		m.cancelled = true
		return m, tea.Quit
	}

	return m, cmd
}

func (m PassphraseModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	// If form still active, center it in a bordered box.
	if m.form.State != huh.StateCompleted && m.form.State != huh.StateAborted {
		boxStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 3).
			Width(ui.Min(m.boxWidth, ui.Max(20, m.width-4)))

		helpStyle := lipgloss.NewStyle().Faint(true)

		content := lipgloss.JoinVertical(lipgloss.Left,
			m.form.View(),
			"",
			helpStyle.Render("Enter to submit • Esc/Ctrl+C to cancel"),
		)

		box := boxStyle.Render(content)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
	}

	// After completion we can show nothing (program will quit anyway) or a simple message.
	return ""
}

func PromptPassphrase(prompt string) (string, error) {
	m := NewPassphraseModel(prompt)

	var opts []tea.ProgramOption
	if m.useAltScreen {
		opts = append(opts, tea.WithAltScreen())
	}

	p := tea.NewProgram(m, opts...)
	final, err := p.Run()
	if err != nil {
		return "", err
	}

	pm := final.(PassphraseModel)
	if pm.cancelled {
		return "", errors.New("passphrase input cancelled")
	}
	return pm.value, nil
}
