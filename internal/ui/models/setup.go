package models

import (
	"errors"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

var (
	firstName string
	lastName  string
	email     string

	confirm bool
)

type SetupModel struct {
	ready  bool
	width  int
	height int
	form   *huh.Form
}

func (m SetupModel) Init() tea.Cmd {
	// We lazily build the form on the first WindowSizeMsg, so nothing to init yet.
	// If you prefer to build it eagerly, create it here and return m.form.Init().
	return nil
}

func (m SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			// esc will also Abort the form; keeping it to quit is fine.
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		// Lazily build the form and INIT it exactly once so inputs are focused.
		if m.form == nil {
			m.form = buildForm()
			cmds = append(cmds, m.form.Init())
		}

		// Constrain form width
		formWidth := min(60, max(20, m.width-6))
		m.form = m.form.WithWidth(formWidth)
	}

	// Forward messages to the form
	if m.form != nil {
		_, cmd := m.form.Update(msg)
		cmds = append(cmds, cmd)

		// State is a field in recent huh versions
		switch m.form.State {
		case huh.StateCompleted:

			log.Printf("Saved user:\n- First: %s\n- Last:  %s\n- Email: %s\n", firstName, lastName, email)
			return m, tea.Quit
		case huh.StateAborted:
			return m, tea.Quit
		default:

		}
	}

	return m, tea.Batch(cmds...)
}

func (m SetupModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}
	if m.form == nil {
		return "\n  Loading form..."
	}

	content := m.form.View()

	box := lipgloss.NewStyle().
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		Width(min(60, max(20, m.width-6))).
		Render(content)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		box,
	)
}

func buildForm() *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("First Name:").
				Value(&firstName).
				Validate(func(str string) error {
					if str == "" {
						return errors.New("first name cannot be empty")
					}
					return nil
				}),
			huh.NewInput().
				Title("Last Name:").
				Value(&lastName).
				Validate(func(str string) error {
					if str == "" {
						return errors.New("last name cannot be empty")
					}
					return nil
				}),
			huh.NewInput().
				Title("Email:").
				Value(&email).
				Validate(func(str string) error {
					if str == "" {
						return errors.New("email cannot be empty")
					}
					return nil
				}),
			huh.NewConfirm().
				Title("Confirm user identity").
				Affirmative("Yes").
				Negative("No!").
				Value(&confirm),
		),
	).WithShowHelp(true)
}
