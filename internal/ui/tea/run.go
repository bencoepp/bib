package tea

import (
	"bib/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

// Run boots the Bubble Tea program using settings from config.
func Run(cfg *config.BibConfig) error {
	m := NewModel(cfg)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
