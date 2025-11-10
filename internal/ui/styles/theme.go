package styles

import (
	"github.com/charmbracelet/lipgloss"
)

type Theme struct {
	// Colors
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Accent    lipgloss.Color
	Faint     lipgloss.Color
	Error     lipgloss.Color
	Warning   lipgloss.Color
	Success   lipgloss.Color
	Bg        lipgloss.Color
	Fg        lipgloss.Color

	// Styles
	Title    lipgloss.Style
	Subtitle lipgloss.Style
	Text     lipgloss.Style
	Muted    lipgloss.Style
	Border   lipgloss.Style
	Panel    lipgloss.Style
	Table    lipgloss.Style
	Help     lipgloss.Style
	Box      lipgloss.Style
}

func buildStyles(t Theme) Theme {
	t.Title = lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	t.Subtitle = lipgloss.NewStyle().Foreground(t.Accent)
	t.Text = lipgloss.NewStyle().Foreground(t.Fg)
	t.Muted = lipgloss.NewStyle().Foreground(t.Faint)
	t.Border = lipgloss.NewStyle().Foreground(t.Faint)
	t.Panel = lipgloss.NewStyle().
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Faint)
	t.Table = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240"))
	t.Help = lipgloss.NewStyle().Faint(true)
	t.Box = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 3)
	return t
}

func NewDark() Theme {
	t := Theme{
		Primary:   lipgloss.Color("#7D56F4"),
		Secondary: lipgloss.Color("#2B2B2B"),
		Accent:    lipgloss.Color("#FFB86C"),
		Faint:     lipgloss.Color("#5A5E6A"),
		Error:     lipgloss.Color("#FF5C57"),
		Warning:   lipgloss.Color("#F3F99D"),
		Success:   lipgloss.Color("#5AF78E"),
		Bg:        lipgloss.Color("#1E1E1E"),
		Fg:        lipgloss.Color("#E6E6E6"),
	}
	return buildStyles(t)
}

func NewLight() Theme {
	t := Theme{
		Primary:   lipgloss.Color("#5B3DF5"),
		Secondary: lipgloss.Color("#EAEAEA"),
		Accent:    lipgloss.Color("#B46900"),
		Faint:     lipgloss.Color("#9AA0A6"),
		Error:     lipgloss.Color("#D93025"),
		Warning:   lipgloss.Color("#A07B00"),
		Success:   lipgloss.Color("#188038"),
		Bg:        lipgloss.Color("#FFFFFF"),
		Fg:        lipgloss.Color("#202124"),
	}
	return buildStyles(t)
}

var (
	DefaultDark  = NewDark()
	DefaultLight = NewLight()
)
