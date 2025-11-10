package models

import (
	"bib/internal/contexts"
	"bib/internal/ui"
	"bib/internal/ui/styles"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type MeModel struct {
	Theme        styles.Theme
	Identity     contexts.IdentityContext
	width        int
	height       int
	ready        bool
	cancelled    bool
	boxWidth     int
	useAltScreen bool
}

func (m MeModel) Init() tea.Cmd { return nil }

func (m MeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	return m, cmd
}

func (m MeModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	boxStyle := m.Theme.Box.
		Width(ui.Min(m.boxWidth, ui.Max(20, m.width-4)))

	content := lipgloss.JoinVertical(lipgloss.Left,
		m.Theme.Muted.Render(m.Identity.ID),
		m.Theme.Title.Render(m.Identity.User.FirstName+", "+m.Identity.User.LastName),
		m.Identity.User.Email,
		"",
		m.Theme.Help.Render("Enter to submit • Esc/Ctrl+C to cancel"),
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

	boxUser := boxStyle.Render(content)
	boxLocation := boxStyle.Render(location)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, lipgloss.JoinHorizontal(lipgloss.Left, boxUser, boxLocation))
}
