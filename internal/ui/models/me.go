package models

import (
	"bib/internal/contexts"
	"bib/internal/ui"
	"bib/internal/ui/keys"
	"bib/internal/ui/styles"
	"strconv"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type MeModel struct {
	Theme        styles.Theme
	Identity     contexts.IdentityContext
	keys         keys.MeKeyMap
	help         help.Model
	width        int
	height       int
	ready        bool
	boxWidth     int
	useAltScreen bool
}

func (m MeModel) Init() tea.Cmd { return nil }

func (m MeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	m.keys = keys.MeKeys
	m.help = help.New()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

	case tea.KeyMsg:
		switch {

		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		}
	}
	return m, cmd
}

func (m MeModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	leftWidth := m.boxWidth
	if leftWidth <= 0 {
		leftWidth = ui.Max(20, (m.width-6)/3*2) // reserve a couple of columns for spacing
	}
	leftWidth = ui.Min(leftWidth, ui.Max(20, m.width-4))

	leftBoxStyle := m.Theme.Box.Width(leftWidth)

	rightWidth := m.width - 4 - leftWidth
	if rightWidth < 20 {
		rightWidth = 20
	}
	rightBoxStyle := m.Theme.Box.Width(rightWidth)

	content := lipgloss.JoinVertical(lipgloss.Left,
		m.Theme.Muted.Render(m.Identity.ID),
		m.Theme.Title.Render(m.Identity.User.FirstName+", "+m.Identity.User.LastName),
		m.Identity.User.Email,
	)

	lat := strconv.FormatFloat(m.Identity.User.Location.Latitude, 'E', -1, 64)
	long := strconv.FormatFloat(m.Identity.User.Location.Longitude, 'E', -1, 64)

	location := lipgloss.JoinVertical(lipgloss.Left,
		m.Identity.User.Location.CountryCode+" - "+m.Identity.User.Location.Country,
		m.Theme.Muted.Render("Timezone: "+m.Identity.User.Location.Timezone),
		m.Identity.User.Location.Region+" - "+m.Identity.User.Location.RegionName,
		m.Identity.User.Location.City+", "+m.Identity.User.Location.Zip,
		lat+", "+long,
		m.Identity.User.Location.Isp,
		m.Identity.User.Location.Org,
		m.Identity.User.Location.As,
		m.Theme.Muted.Render("IP: "+m.Identity.User.Location.Ip.String()),
	)

	local := lipgloss.JoinVertical(lipgloss.Left,
		"We can not find a local bibd instance.",
		"To work with bib, please set up a bibd instance first.",
	)

	boxUser := leftBoxStyle.Render(content)
	boxLocal := leftBoxStyle.Render(local)
	boxLocation := rightBoxStyle.Render(location)
	helpView := m.help.View(m.keys)

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		lipgloss.JoinVertical(
			lipgloss.Left,
			lipgloss.JoinHorizontal(lipgloss.Left, lipgloss.JoinVertical(lipgloss.Left, boxUser, boxLocal), boxLocation),
			helpView,
		),
	)
}
