// Package tui provides reusable terminal UI components for bib and bibd.
package tui

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Color palette for bib
var (
	// Primary colors
	ColorPrimary   = lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"} // Purple
	ColorSecondary = lipgloss.AdaptiveColor{Light: "#2563EB", Dark: "#60A5FA"} // Blue
	ColorAccent    = lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34D399"} // Green
	ColorWarning   = lipgloss.AdaptiveColor{Light: "#D97706", Dark: "#FBBF24"} // Amber
	ColorError     = lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#F87171"} // Red
	ColorSuccess   = lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34D399"} // Green
	ColorInfo      = lipgloss.AdaptiveColor{Light: "#0891B2", Dark: "#22D3EE"} // Cyan

	// Neutral colors
	ColorText          = lipgloss.AdaptiveColor{Light: "#1F2937", Dark: "#F9FAFB"}
	ColorTextMuted     = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	ColorTextSubtle    = lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#6B7280"}
	ColorBackground    = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#111827"}
	ColorBackgroundAlt = lipgloss.AdaptiveColor{Light: "#F3F4F6", Dark: "#1F2937"}
	ColorBorder        = lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#374151"}
	ColorBorderFocus   = lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"}
)

// Theme contains all the styles for the TUI
type Theme struct {
	// Base styles
	Base        lipgloss.Style
	Title       lipgloss.Style
	Subtitle    lipgloss.Style
	Description lipgloss.Style

	// Status styles
	Success lipgloss.Style
	Error   lipgloss.Style
	Warning lipgloss.Style
	Info    lipgloss.Style

	// Interactive styles
	Focused  lipgloss.Style
	Blurred  lipgloss.Style
	Selected lipgloss.Style
	Cursor   lipgloss.Style

	// Button styles
	ButtonActive   lipgloss.Style
	ButtonInactive lipgloss.Style
	ButtonDanger   lipgloss.Style

	// Section styles
	Section       lipgloss.Style
	SectionTitle  lipgloss.Style
	SectionBorder lipgloss.Style

	// Help styles
	Help     lipgloss.Style
	HelpKey  lipgloss.Style
	HelpDesc lipgloss.Style

	// Progress styles
	ProgressBar      lipgloss.Style
	ProgressComplete lipgloss.Style
	ProgressPending  lipgloss.Style

	// Box/Panel styles
	Box       lipgloss.Style
	BoxTitle  lipgloss.Style
	BoxBorder lipgloss.Style

	// Tab styles
	TabActive   lipgloss.Style
	TabInactive lipgloss.Style
	TabBar      lipgloss.Style

	// Wizard step styles
	StepComplete lipgloss.Style
	StepCurrent  lipgloss.Style
	StepPending  lipgloss.Style
	StepNumber   lipgloss.Style
}

// DefaultTheme returns the default bib theme
func DefaultTheme() *Theme {
	return &Theme{
		Base: lipgloss.NewStyle().
			Foreground(ColorText),

		Title: lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			MarginBottom(1),

		Subtitle: lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true),

		Description: lipgloss.NewStyle().
			Foreground(ColorTextMuted).
			Italic(true),

		Success: lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true),

		Error: lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true),

		Warning: lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true),

		Info: lipgloss.NewStyle().
			Foreground(ColorInfo),

		Focused: lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true),

		Blurred: lipgloss.NewStyle().
			Foreground(ColorTextMuted),

		Selected: lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Background(ColorBackgroundAlt),

		Cursor: lipgloss.NewStyle().
			Foreground(ColorPrimary),

		ButtonActive: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(ColorPrimary).
			Padding(0, 3).
			Bold(true),

		ButtonInactive: lipgloss.NewStyle().
			Foreground(ColorTextMuted).
			Background(ColorBackgroundAlt).
			Padding(0, 3),

		ButtonDanger: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(ColorError).
			Padding(0, 3).
			Bold(true),

		Section: lipgloss.NewStyle().
			MarginTop(1).
			MarginBottom(1),

		SectionTitle: lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true).
			MarginBottom(1),

		SectionBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2),

		Help: lipgloss.NewStyle().
			Foreground(ColorTextSubtle).
			MarginTop(1),

		HelpKey: lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true),

		HelpDesc: lipgloss.NewStyle().
			Foreground(ColorTextMuted),

		ProgressBar: lipgloss.NewStyle().
			Foreground(ColorPrimary),

		ProgressComplete: lipgloss.NewStyle().
			Foreground(ColorSuccess),

		ProgressPending: lipgloss.NewStyle().
			Foreground(ColorTextSubtle),

		Box: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2),

		BoxTitle: lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true),

		BoxBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorderFocus),

		TabActive: lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Background(ColorBackgroundAlt).
			Bold(true).
			Padding(0, 2),

		TabInactive: lipgloss.NewStyle().
			Foreground(ColorTextMuted).
			Padding(0, 2),

		TabBar: lipgloss.NewStyle().
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(ColorBorder),

		StepComplete: lipgloss.NewStyle().
			Foreground(ColorSuccess),

		StepCurrent: lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true),

		StepPending: lipgloss.NewStyle().
			Foreground(ColorTextSubtle),

		StepNumber: lipgloss.NewStyle().
			Width(3).
			Align(lipgloss.Center),
	}
}

// HuhTheme returns a huh.Theme based on the bib theme
func HuhTheme() *huh.Theme {
	t := huh.ThemeBase()

	// Customize the theme to match bib colors
	t.Focused.Title = t.Focused.Title.Foreground(ColorPrimary).Bold(true)
	t.Focused.Description = t.Focused.Description.Foreground(ColorTextMuted)
	t.Focused.Base = t.Focused.Base.BorderForeground(ColorPrimary)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(ColorPrimary)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(ColorPrimary)
	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(ColorPrimary)
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(ColorTextSubtle)

	t.Blurred.Title = t.Blurred.Title.Foreground(ColorTextMuted)
	t.Blurred.Description = t.Blurred.Description.Foreground(ColorTextSubtle)

	return t
}

// Icons for consistent UI elements
var (
	IconCheck    = "✓"
	IconCross    = "✗"
	IconArrow    = "→"
	IconBullet   = "•"
	IconCircle   = "○"
	IconDot      = "●"
	IconStar     = "★"
	IconWarning  = "⚠"
	IconInfo     = "ℹ"
	IconQuestion = "?"
	IconSpinner  = "◐"
)

// Banner returns the ASCII art banner for bib
func Banner() string {
	banner := `
 ██████╗ ██╗██████╗ 
 ██╔══██╗██║██╔══██╗
 ██████╔╝██║██████╔╝
 ██╔══██╗██║██╔══██╗
 ██████╔╝██║██████╔╝
 ╚═════╝ ╚═╝╚═════╝ `

	return lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true).
		Render(banner)
}

// SmallBanner returns a smaller banner for tighter spaces
func SmallBanner() string {
	return lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true).
		Render("◆ bib")
}
