// Package i18n provides internationalization helpers for the CLI.
//
// # Overview
//
// This package provides utilities for translating CLI command descriptions,
// help text, and other user-facing strings. It works in conjunction with
// the TUI i18n package which manages the actual translations.
//
// # How It Works
//
// Commands use annotation markers instead of hardcoded strings:
//
//	cmd := &cobra.Command{
//	    Use:   "mycommand",
//	    Short: "cmd.mycommand.short",  // Translation key
//	    Long:  "cmd.mycommand.long",   // Translation key
//	    Annotations: map[string]string{
//	        "i18n": "true",            // Mark for translation
//	    },
//	}
//
// After the locale is resolved (in PersistentPreRunE), call TranslateCommands
// to replace all keys with actual translations:
//
//	i18n.TranslateCommands(rootCmd)
//
// # Translation Keys
//
// CLI translation keys follow this pattern:
//   - cmd.<command-path>.short - Short description
//   - cmd.<command-path>.long - Long description
//   - cmd.<command-path>.example - Example usage
//   - cmd.<command-path>.flags.<flag-name> - Flag description
//
// For nested commands, use dots: cmd.admin.backup.short
//
// # Adding New Commands
//
// 1. Define the command with translation keys instead of strings
// 2. Add i18n annotation: Annotations: map[string]string{"i18n": "true"}
// 3. Add translation keys to all locale files under the "cmd" section
//
// See docs/development/cli-i18n.md for full documentation.
package i18n

import (
	"strings"

	tuii18n "bib/internal/tui/i18n"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Annotation key used to mark commands for translation
const AnnotationKey = "i18n"

// TranslateCommands recursively translates all commands marked with i18n annotation.
// This should be called after the locale is resolved, typically in PersistentPreRunE.
func TranslateCommands(cmd *cobra.Command) {
	translateCommand(cmd)
	for _, child := range cmd.Commands() {
		TranslateCommands(child)
	}
}

// translateCommand translates a single command if it has the i18n annotation.
func translateCommand(cmd *cobra.Command) {
	// Check if command is marked for translation
	if cmd.Annotations == nil || cmd.Annotations[AnnotationKey] != "true" {
		return
	}

	i := tuii18n.Global()

	// Translate Short description
	if cmd.Short != "" {
		key := "cmd." + cmd.Short
		if i.Has(key) {
			cmd.Short = i.T(key)
		}
	}

	// Translate Long description
	if cmd.Long != "" {
		key := "cmd." + cmd.Long
		if i.Has(key) {
			cmd.Long = i.T(key)
		}
	}

	// Translate Example
	if cmd.Example != "" {
		key := "cmd." + cmd.Example
		if i.Has(key) {
			cmd.Example = i.T(key)
		}
	}

	// Translate flags
	translateFlags(cmd, i)
}

// translateFlags translates flag usage strings.
func translateFlags(cmd *cobra.Command, i *tuii18n.I18n) {
	cmdPath := strings.ReplaceAll(cmd.CommandPath(), " ", ".")

	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// Check for flag-specific translation key
		key := "cmd." + cmdPath + ".flags." + f.Name
		if i.Has(key) {
			f.Usage = i.T(key)
		}
	})

	cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		key := "cmd." + cmdPath + ".flags." + f.Name
		if i.Has(key) {
			f.Usage = i.T(key)
		}
	})
}

// T is a convenience function for translating a key using the global i18n instance.
// Useful for translating runtime messages in command implementations.
func T(key string, args ...any) string {
	return tuii18n.Global().T(key, args...)
}

// TPlural translates a key with pluralization support.
func TPlural(key string, count int, args ...any) string {
	return tuii18n.Global().TPlural(key, count, args...)
}

// MarkForTranslation returns a map with the i18n annotation set.
// Use this when creating commands to mark them for translation.
//
//	cmd := &cobra.Command{
//	    Annotations: i18n.MarkForTranslation(),
//	}
func MarkForTranslation() map[string]string {
	return map[string]string{AnnotationKey: "true"}
}

// MergeAnnotations merges i18n annotation with existing annotations.
func MergeAnnotations(existing map[string]string) map[string]string {
	if existing == nil {
		return MarkForTranslation()
	}
	existing[AnnotationKey] = "true"
	return existing
}
