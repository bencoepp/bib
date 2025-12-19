// Package errors provides rich error types and display for the bib CLI.
//
// Errors are designed to be user-friendly with:
//   - Clear error codes for documentation/support
//   - Actionable suggestions
//   - Links to documentation
//   - TUI-aware formatting
package errors

import (
	"errors"
	"fmt"
	"strings"

	"bib/internal/tui/themes"

	"github.com/charmbracelet/lipgloss"
)

// Code represents an error code for categorization.
type Code string

// Common error codes
const (
	CodeUnknown          Code = "UNKNOWN"
	CodeConfigNotFound   Code = "CONFIG_NOT_FOUND"
	CodeConfigInvalid    Code = "CONFIG_INVALID"
	CodeConnectionFailed Code = "CONNECTION_FAILED"
	CodeAuthFailed       Code = "AUTH_FAILED"
	CodeNotFound         Code = "NOT_FOUND"
	CodePermissionDenied Code = "PERMISSION_DENIED"
	CodeValidation       Code = "VALIDATION"
	CodeTimeout          Code = "TIMEOUT"
	CodeInternal         Code = "INTERNAL"
	CodeUserCancelled    Code = "USER_CANCELLED"
)

// Rich is an enhanced error with additional context for display.
type Rich struct {
	// Code is a unique error code for categorization
	Code Code
	// Message is the user-friendly error message
	Message string
	// Details provides additional technical information
	Details string
	// Suggestions are actionable items the user can try
	Suggestions []string
	// DocURL is a link to relevant documentation
	DocURL string
	// Cause is the underlying error
	Cause error
}

// Error implements the error interface.
func (e *Rich) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error.
func (e *Rich) Unwrap() error {
	return e.Cause
}

// New creates a new Rich error.
func New(code Code, message string) *Rich {
	return &Rich{
		Code:    code,
		Message: message,
	}
}

// Wrap wraps an existing error with additional context.
func Wrap(err error, code Code, message string) *Rich {
	return &Rich{
		Code:    code,
		Message: message,
		Cause:   err,
	}
}

// WithDetails adds technical details to the error.
func (e *Rich) WithDetails(details string) *Rich {
	e.Details = details
	return e
}

// WithSuggestions adds actionable suggestions.
func (e *Rich) WithSuggestions(suggestions ...string) *Rich {
	e.Suggestions = suggestions
	return e
}

// WithDocURL adds a documentation link.
func (e *Rich) WithDocURL(url string) *Rich {
	e.DocURL = url
	return e
}

// WithCause sets the underlying cause.
func (e *Rich) WithCause(cause error) *Rich {
	e.Cause = cause
	return e
}

// IsRich checks if an error is a Rich error.
func IsRich(err error) bool {
	var rich *Rich
	return errors.As(err, &rich)
}

// AsRich converts an error to a Rich error if possible.
func AsRich(err error) *Rich {
	var rich *Rich
	if errors.As(err, &rich) {
		return rich
	}
	return nil
}

// Display formats and prints the error with TUI styling.
func Display(err error, theme *themes.Theme) string {
	if theme == nil {
		theme = themes.Global().Active()
	}

	rich := AsRich(err)
	if rich == nil {
		// Wrap plain error
		rich = Wrap(err, CodeUnknown, err.Error())
	}

	var b strings.Builder

	// Error box style
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Palette.Error).
		Padding(0, 1).
		Width(60)

	// Header
	headerStyle := lipgloss.NewStyle().
		Foreground(theme.Palette.Error).
		Bold(true)

	codeStyle := lipgloss.NewStyle().
		Foreground(theme.Palette.TextMuted).
		Italic(true)

	b.WriteString(headerStyle.Render("✗ Error"))
	b.WriteString(" ")
	b.WriteString(codeStyle.Render(fmt.Sprintf("[%s]", rich.Code)))
	b.WriteString("\n\n")

	// Message
	messageStyle := lipgloss.NewStyle().
		Foreground(theme.Palette.Text)
	b.WriteString(messageStyle.Render(rich.Message))
	b.WriteString("\n")

	// Details
	if rich.Details != "" {
		b.WriteString("\n")
		detailsStyle := lipgloss.NewStyle().
			Foreground(theme.Palette.TextMuted)
		b.WriteString(detailsStyle.Render(rich.Details))
		b.WriteString("\n")
	}

	// Cause
	if rich.Cause != nil {
		b.WriteString("\n")
		causeStyle := lipgloss.NewStyle().
			Foreground(theme.Palette.TextMuted)
		b.WriteString(causeStyle.Render("Caused by: " + rich.Cause.Error()))
		b.WriteString("\n")
	}

	// Suggestions
	if len(rich.Suggestions) > 0 {
		b.WriteString("\n")
		suggestStyle := lipgloss.NewStyle().
			Foreground(theme.Palette.Info)
		b.WriteString(suggestStyle.Render("💡 Suggestions:"))
		b.WriteString("\n")

		for _, s := range rich.Suggestions {
			b.WriteString("   • ")
			b.WriteString(s)
			b.WriteString("\n")
		}
	}

	// Doc URL
	if rich.DocURL != "" {
		b.WriteString("\n")
		urlStyle := lipgloss.NewStyle().
			Foreground(theme.Palette.Info).
			Underline(true)
		b.WriteString("📖 ")
		b.WriteString(urlStyle.Render(rich.DocURL))
		b.WriteString("\n")
	}

	return boxStyle.Render(b.String())
}

// DisplaySimple formats an error for non-TUI output.
func DisplaySimple(err error) string {
	rich := AsRich(err)
	if rich == nil {
		return fmt.Sprintf("Error: %v", err)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Error [%s]: %s\n", rich.Code, rich.Message))

	if rich.Details != "" {
		b.WriteString(fmt.Sprintf("  Details: %s\n", rich.Details))
	}

	if rich.Cause != nil {
		b.WriteString(fmt.Sprintf("  Caused by: %v\n", rich.Cause))
	}

	if len(rich.Suggestions) > 0 {
		b.WriteString("  Suggestions:\n")
		for _, s := range rich.Suggestions {
			b.WriteString(fmt.Sprintf("    - %s\n", s))
		}
	}

	if rich.DocURL != "" {
		b.WriteString(fmt.Sprintf("  Documentation: %s\n", rich.DocURL))
	}

	return b.String()
}

// Common errors with helpful messages

// ConfigNotFound returns a config not found error.
func ConfigNotFound(path string) *Rich {
	return New(CodeConfigNotFound, "Configuration file not found").
		WithDetails(fmt.Sprintf("Expected config at: %s", path)).
		WithSuggestions(
			"Run 'bib setup' to create a new configuration",
			"Use '--config' flag to specify a custom config path",
			"Check file permissions",
		).
		WithDocURL("https://docs.bib.dev/getting-started/configuration")
}

// ConfigInvalid returns a config validation error.
func ConfigInvalid(path string, validationErr error) *Rich {
	return New(CodeConfigInvalid, "Configuration file is invalid").
		WithDetails(fmt.Sprintf("File: %s", path)).
		WithCause(validationErr).
		WithSuggestions(
			"Run 'bib config validate' to see detailed errors",
			"Check the configuration file syntax",
			"Run 'bib setup' to regenerate configuration",
		).
		WithDocURL("https://docs.bib.dev/getting-started/configuration")
}

// ConnectionFailed returns a connection error.
func ConnectionFailed(addr string, cause error) *Rich {
	return New(CodeConnectionFailed, "Failed to connect to bibd").
		WithDetails(fmt.Sprintf("Address: %s", addr)).
		WithCause(cause).
		WithSuggestions(
			"Verify bibd is running with 'bib admin status'",
			"Check the server address in your config",
			"Verify network connectivity",
			"Check firewall settings",
		).
		WithDocURL("https://docs.bib.dev/getting-started/quickstart")
}

// NotFound returns a resource not found error.
func NotFound(resource, id string) *Rich {
	return New(CodeNotFound, fmt.Sprintf("%s not found: %s", resource, id)).
		WithSuggestions(
			fmt.Sprintf("Use 'bib %s list' to see available items", strings.ToLower(resource)),
			"Verify the ID is correct",
		)
}

// UserCancelled returns an error indicating the user cancelled the operation.
func UserCancelled() *Rich {
	return New(CodeUserCancelled, "Operation cancelled by user")
}

// Timeout returns a timeout error.
func Timeout(operation string, duration string) *Rich {
	return New(CodeTimeout, fmt.Sprintf("Operation timed out: %s", operation)).
		WithDetails(fmt.Sprintf("Timeout after: %s", duration)).
		WithSuggestions(
			"Try the operation again",
			"Check if the server is under heavy load",
			"Increase timeout with '--timeout' flag",
		)
}
