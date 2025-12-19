// Package help provides a universal help system for the bib CLI.
//
// The help system provides:
//   - Enhanced command help with examples
//   - Interactive help browser (TUI)
//   - Links to documentation
//   - Context-aware suggestions
package help

import (
	"fmt"
	"strings"

	"bib/internal/tui/themes"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// Example represents a usage example.
type Example struct {
	Description string
	Command     string
	Output      string // Optional expected output
}

// CommandHelp provides enhanced help for a command.
type CommandHelp struct {
	Command     *cobra.Command
	Examples    []Example
	SeeAlso     []string
	DocURL      string
	Notes       []string
	Deprecation string
}

// Registry stores help information for commands.
type Registry struct {
	commands map[string]*CommandHelp
	theme    *themes.Theme
}

// NewRegistry creates a new help registry.
func NewRegistry() *Registry {
	return &Registry{
		commands: make(map[string]*CommandHelp),
		theme:    themes.Global().Active(),
	}
}

// Register adds help for a command.
func (r *Registry) Register(path string, help *CommandHelp) {
	r.commands[path] = help
}

// Get retrieves help for a command path.
func (r *Registry) Get(path string) *CommandHelp {
	return r.commands[path]
}

// SetTheme sets the theme for rendering.
func (r *Registry) SetTheme(theme *themes.Theme) {
	r.theme = theme
}

// RenderHelp renders enhanced help for a command.
func (r *Registry) RenderHelp(cmd *cobra.Command) string {
	theme := r.theme
	if theme == nil {
		theme = themes.Global().Active()
	}

	var b strings.Builder

	// Command path and short description
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Palette.Primary)

	b.WriteString(titleStyle.Render(cmd.CommandPath()))
	b.WriteString("\n")

	// Short description
	if cmd.Short != "" {
		descStyle := lipgloss.NewStyle().
			Foreground(theme.Palette.Text)
		b.WriteString(descStyle.Render(cmd.Short))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Long description
	if cmd.Long != "" {
		b.WriteString(cmd.Long)
		b.WriteString("\n\n")
	}

	// Usage
	usageStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Palette.Secondary)

	b.WriteString(usageStyle.Render("Usage:"))
	b.WriteString("\n")
	b.WriteString("  " + cmd.UseLine())
	b.WriteString("\n\n")

	// Subcommands
	if cmd.HasAvailableSubCommands() {
		b.WriteString(usageStyle.Render("Commands:"))
		b.WriteString("\n")

		maxLen := 0
		for _, c := range cmd.Commands() {
			if c.IsAvailableCommand() && len(c.Name()) > maxLen {
				maxLen = len(c.Name())
			}
		}

		for _, c := range cmd.Commands() {
			if !c.IsAvailableCommand() {
				continue
			}
			padding := strings.Repeat(" ", maxLen-len(c.Name()))
			b.WriteString(fmt.Sprintf("  %s%s  %s\n", c.Name(), padding, c.Short))
		}
		b.WriteString("\n")
	}

	// Flags
	if cmd.HasAvailableLocalFlags() {
		b.WriteString(usageStyle.Render("Flags:"))
		b.WriteString("\n")
		b.WriteString(cmd.LocalFlags().FlagUsages())
		b.WriteString("\n")
	}

	if cmd.HasAvailablePersistentFlags() {
		b.WriteString(usageStyle.Render("Global Flags:"))
		b.WriteString("\n")
		b.WriteString(cmd.InheritedFlags().FlagUsages())
		b.WriteString("\n")
	}

	// Enhanced help from registry
	if help := r.commands[cmd.CommandPath()]; help != nil {
		// Examples
		if len(help.Examples) > 0 {
			b.WriteString(usageStyle.Render("Examples:"))
			b.WriteString("\n")

			for _, ex := range help.Examples {
				exStyle := lipgloss.NewStyle().
					Foreground(theme.Palette.TextMuted).
					Italic(true)
				b.WriteString("  " + exStyle.Render("# "+ex.Description))
				b.WriteString("\n")

				cmdStyle := lipgloss.NewStyle().
					Foreground(theme.Palette.Info)
				b.WriteString("  " + cmdStyle.Render("$ "+ex.Command))
				b.WriteString("\n")

				if ex.Output != "" {
					b.WriteString("  " + ex.Output)
					b.WriteString("\n")
				}
				b.WriteString("\n")
			}
		}

		// Notes
		if len(help.Notes) > 0 {
			b.WriteString(usageStyle.Render("Notes:"))
			b.WriteString("\n")
			for _, note := range help.Notes {
				b.WriteString("  • " + note)
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}

		// See also
		if len(help.SeeAlso) > 0 {
			b.WriteString(usageStyle.Render("See Also:"))
			b.WriteString("\n")
			for _, ref := range help.SeeAlso {
				b.WriteString("  • " + ref)
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}

		// Documentation URL
		if help.DocURL != "" {
			urlStyle := lipgloss.NewStyle().
				Foreground(theme.Palette.Info).
				Underline(true)
			b.WriteString(usageStyle.Render("Documentation:"))
			b.WriteString("\n")
			b.WriteString("  " + urlStyle.Render(help.DocURL))
			b.WriteString("\n\n")
		}

		// Deprecation warning
		if help.Deprecation != "" {
			warnStyle := lipgloss.NewStyle().
				Foreground(theme.Palette.Warning).
				Bold(true)
			b.WriteString(warnStyle.Render("⚠ DEPRECATED: " + help.Deprecation))
			b.WriteString("\n\n")
		}
	}

	// Standard examples from cobra
	if cmd.Example != "" {
		b.WriteString(usageStyle.Render("Examples:"))
		b.WriteString("\n")
		b.WriteString(cmd.Example)
		b.WriteString("\n\n")
	}

	return b.String()
}

// ApplyToCommand sets up the custom help function on a command tree.
func (r *Registry) ApplyToCommand(cmd *cobra.Command) {
	// Set custom help template/function
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Println(r.RenderHelp(cmd))
	})

	// Recurse into subcommands
	for _, sub := range cmd.Commands() {
		r.ApplyToCommand(sub)
	}
}

// Global registry
var globalRegistry *Registry

// Global returns the global help registry.
func Global() *Registry {
	if globalRegistry == nil {
		globalRegistry = NewRegistry()
	}
	return globalRegistry
}

// RegisterExamples is a helper to quickly register examples for a command.
func RegisterExamples(cmdPath string, examples ...Example) {
	help := Global().Get(cmdPath)
	if help == nil {
		help = &CommandHelp{}
	}
	help.Examples = append(help.Examples, examples...)
	Global().Register(cmdPath, help)
}

// RegisterDocURL registers a documentation URL for a command.
func RegisterDocURL(cmdPath, url string) {
	help := Global().Get(cmdPath)
	if help == nil {
		help = &CommandHelp{}
	}
	help.DocURL = url
	Global().Register(cmdPath, help)
}
