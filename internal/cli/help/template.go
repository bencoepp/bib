package help

import (
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

// KeyBinding represents a keyboard shortcut for help display
type KeyBinding struct {
	Key         string
	Description string
}

// UsageTemplate returns a custom usage template for cobra commands
func UsageTemplate() string {
	return `{{if .Long}}{{.Long}}{{else}}{{.Short}}{{end}}

{{if .Runnable}}Usage:
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
}

// HelpTemplate returns a custom help template for cobra commands
func HelpTemplate() string {
	return `{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`
}

// SetupCobraHelp configures cobra for custom help output
func SetupCobraHelp(cmd *cobra.Command) {
	cmd.SetUsageTemplate(UsageTemplate())
	cmd.SetHelpTemplate(HelpTemplate())

	// Add custom help command
	cmd.SetHelpCommand(&cobra.Command{
		Use:   "help [command]",
		Short: "Help about any command",
		Long: `Help provides help for any command in the application.
Simply type ` + cmd.Name() + ` help [path to command] for full details.`,
		Run: func(c *cobra.Command, args []string) {
			cmd, _, e := c.Root().Find(args)
			if cmd == nil || e != nil {
				c.Printf("Unknown help topic %#q\n", args)
				c.Root().Usage()
			} else {
				cmd.InitDefaultHelpFlag()
				cmd.Help()
			}
		},
	})
}

// FormatKeyBindings formats keybindings for CLI help output
func FormatKeyBindings(bindings []KeyBinding) string {
	if len(bindings) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("Keyboard Shortcuts:\n")

	maxKeyLen := 0
	for _, binding := range bindings {
		if len(binding.Key) > maxKeyLen {
			maxKeyLen = len(binding.Key)
		}
	}

	for _, binding := range bindings {
		b.WriteString("  ")
		b.WriteString(binding.Key)
		b.WriteString(strings.Repeat(" ", maxKeyLen-len(binding.Key)+2))
		b.WriteString(binding.Description)
		b.WriteString("\n")
	}

	return b.String()
}

// AddHelpSection adds a help section to a cobra command's long description
func AddHelpSection(cmd *cobra.Command, title, content string) {
	if cmd.Long == "" {
		cmd.Long = cmd.Short
	}
	cmd.Long = cmd.Long + "\n\n" + title + ":\n" + content
}

// TemplateFunc returns template functions for help rendering
func TemplateFunc() template.FuncMap {
	return template.FuncMap{
		"trimTrailingWhitespaces": strings.TrimSpace,
		"rpad": func(s string, padding int) string {
			return s + strings.Repeat(" ", padding-len(s))
		},
	}
}
