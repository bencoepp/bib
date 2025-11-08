package models

import (
	"bib/internal/config"
	"bib/internal/contexts"
	"errors"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

var (
	firstName   string
	lastName    string
	email       string
	confirmUser bool

	theme             string
	checkCapabilities bool
	checkLocation     bool
	passphrase        string
	useSecondFactor   bool
	confirmConfig     bool
)

type SetupModel struct {
	ready      bool
	width      int
	height     int
	userForm   *huh.Form
	configForm *huh.Form
	daemonForm *huh.Form

	Cfg     *config.BibConfig
	Version string
}

func (m SetupModel) Init() tea.Cmd {
	return nil
}

func (m SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		if m.userForm == nil {
			m.userForm = buildUserForm()
			cmds = append(cmds, m.userForm.Init())
		}

		formWidth := min(60, max(20, m.width-6))
		m.userForm = m.userForm.WithWidth(formWidth)
		if m.configForm != nil {
			m.configForm = m.configForm.WithWidth(formWidth)
		}
	}

	activeConfig := m.configForm != nil

	if activeConfig {
		_, cmd := m.configForm.Update(msg)
		cmds = append(cmds, cmd)

		switch m.configForm.State {
		case huh.StateCompleted:
			// Only persist if user confirmed config
			if confirmConfig {
				// Apply updated values to the model's config
				m.Cfg = config.DefaultBibConfig()
				m.Cfg.General.Theme = theme
				m.Cfg.General.CheckCapabilities = checkCapabilities
				m.Cfg.General.CheckLocation = checkLocation
				m.Cfg.General.UseSecondFactor = useSecondFactor

				// Save updated config
				if _, err := config.SaveUpdatedBibConfig(m.Cfg); err != nil {
					log.Fatal("Failed to save updated config:", "error", err)
					return m, tea.Quit
				}

				// Register user identity (passphrase collected in this form)
				userIdentity, err := contexts.RegisterUserIdentity(
					m.Cfg,
					m.Version,
					firstName,
					lastName,
					email,
					passphrase,
				)
				if err != nil {
					log.Fatal("Failed to register user identity:", "error", err)
					return m, tea.Quit
				}

				log.Info("User identity registered", "id", userIdentity.ID)
			} else {
				log.Info("Config not confirmed; exiting without saving changes.")
			}
			return m, tea.Quit
		case huh.StateAborted:
			return m, tea.Quit
		}
	} else if m.userForm != nil {
		_, cmd := m.userForm.Update(msg)
		cmds = append(cmds, cmd)

		switch m.userForm.State {
		case huh.StateCompleted:
			if confirmUser {
				// Build config form exactly once
				if m.configForm == nil {
					formWidth := min(60, max(20, m.width-6))
					m.configForm = buildConfigForm().WithWidth(formWidth)
					cmds = append(cmds, m.configForm.Init())
				}
				// Do NOT quit yet; allow user to interact with config form
			} else {
				// User declined identity; exit early
				return m, tea.Quit
			}
		case huh.StateAborted:
			return m, tea.Quit
		}
	}

	return m, tea.Batch(cmds...)
}

func (m SetupModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}
	if m.userForm == nil {
		return "\n  Loading form..."
	}

	var content string
	if m.configForm != nil {
		content = m.configForm.View()
	} else {
		content = m.userForm.View()
	}

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

func buildUserForm() *huh.Form {
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
				Value(&confirmUser),
		),
	).WithShowHelp(true)
}

func buildConfigForm() *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Theme:").
				Options(
					huh.NewOption("Auto", "auto"),
					huh.NewOption("Dark", "dark"),
					huh.NewOption("Light", "light"),
				).
				Value(&theme),
			huh.NewConfirm().
				Title("Check capabilities?").
				Affirmative("Yes!").
				Negative("No.").
				Value(&checkCapabilities),
			huh.NewConfirm().
				Title("Check location?").
				Affirmative("Yes!").
				Negative("No.").
				Value(&checkLocation),
			huh.NewInput().
				Title("Passphrase").
				EchoMode(huh.EchoModePassword).
				Value(&passphrase),
			huh.NewConfirm().
				Title("Use second factor auth?").
				Affirmative("Yes!").
				Negative("No.").
				Value(&useSecondFactor),
			huh.NewConfirm().
				Title("Confirm config").
				Affirmative("Yes").
				Negative("No!").
				Value(&confirmConfig),
		),
	).WithShowHelp(true)
}
