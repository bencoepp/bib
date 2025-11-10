package keys

import "github.com/charmbracelet/bubbles/key"

type MeKeyMap struct {
	Bootstrap key.Binding
	Setup     key.Binding
	Status    key.Binding
	Help      key.Binding
	Quit      key.Binding
}

func (k MeKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Help, k.Quit},
		{k.Bootstrap, k.Setup, k.Status},
	}
}

func (k MeKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

var keys = MeKeyMap{
	Bootstrap: key.NewBinding(
		key.WithKeys("b"),
		key.WithHelp("b", "bootstrap node"),
	),
	Setup: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "setup configuration"),
	),
	Status: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "view daemon status"),
	),
	Help: key.NewBinding(
		key.WithKeys("?", "h"),
		key.WithHelp("?", "toggle help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "esc", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}

var MeKeys = keys
