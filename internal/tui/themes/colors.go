package themes

import "github.com/charmbracelet/lipgloss"

// ColorPalette defines a complete color palette for a theme
type ColorPalette struct {
	// Primary brand colors
	Primary   lipgloss.AdaptiveColor
	Secondary lipgloss.AdaptiveColor
	Accent    lipgloss.AdaptiveColor

	// Semantic colors
	Success lipgloss.AdaptiveColor
	Warning lipgloss.AdaptiveColor
	Error   lipgloss.AdaptiveColor
	Info    lipgloss.AdaptiveColor

	// Text colors
	Text       lipgloss.AdaptiveColor
	TextMuted  lipgloss.AdaptiveColor
	TextSubtle lipgloss.AdaptiveColor

	// Background colors
	Background    lipgloss.AdaptiveColor
	BackgroundAlt lipgloss.AdaptiveColor
	Surface       lipgloss.AdaptiveColor
	Overlay       lipgloss.AdaptiveColor

	// Border colors
	Border      lipgloss.AdaptiveColor
	BorderFocus lipgloss.AdaptiveColor

	// Special colors
	Selection lipgloss.AdaptiveColor
	Cursor    lipgloss.AdaptiveColor
	Link      lipgloss.AdaptiveColor
}

// DefaultDarkPalette returns the default dark color palette
func DefaultDarkPalette() ColorPalette {
	return ColorPalette{
		Primary:       lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"},
		Secondary:     lipgloss.AdaptiveColor{Light: "#2563EB", Dark: "#60A5FA"},
		Accent:        lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34D399"},
		Success:       lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34D399"},
		Warning:       lipgloss.AdaptiveColor{Light: "#D97706", Dark: "#FBBF24"},
		Error:         lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#F87171"},
		Info:          lipgloss.AdaptiveColor{Light: "#0891B2", Dark: "#22D3EE"},
		Text:          lipgloss.AdaptiveColor{Light: "#1F2937", Dark: "#F9FAFB"},
		TextMuted:     lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"},
		TextSubtle:    lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#6B7280"},
		Background:    lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#111827"},
		BackgroundAlt: lipgloss.AdaptiveColor{Light: "#F3F4F6", Dark: "#1F2937"},
		Surface:       lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#1F2937"},
		Overlay:       lipgloss.AdaptiveColor{Light: "#000000", Dark: "#000000"},
		Border:        lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#374151"},
		BorderFocus:   lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"},
		Selection:     lipgloss.AdaptiveColor{Light: "#EDE9FE", Dark: "#312E81"},
		Cursor:        lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"},
		Link:          lipgloss.AdaptiveColor{Light: "#2563EB", Dark: "#60A5FA"},
	}
}

// DefaultLightPalette returns the default light color palette
func DefaultLightPalette() ColorPalette {
	return ColorPalette{
		Primary:       lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#7C3AED"},
		Secondary:     lipgloss.AdaptiveColor{Light: "#2563EB", Dark: "#2563EB"},
		Accent:        lipgloss.AdaptiveColor{Light: "#059669", Dark: "#059669"},
		Success:       lipgloss.AdaptiveColor{Light: "#059669", Dark: "#059669"},
		Warning:       lipgloss.AdaptiveColor{Light: "#D97706", Dark: "#D97706"},
		Error:         lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#DC2626"},
		Info:          lipgloss.AdaptiveColor{Light: "#0891B2", Dark: "#0891B2"},
		Text:          lipgloss.AdaptiveColor{Light: "#1F2937", Dark: "#1F2937"},
		TextMuted:     lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#6B7280"},
		TextSubtle:    lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#9CA3AF"},
		Background:    lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#FFFFFF"},
		BackgroundAlt: lipgloss.AdaptiveColor{Light: "#F3F4F6", Dark: "#F3F4F6"},
		Surface:       lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#FFFFFF"},
		Overlay:       lipgloss.AdaptiveColor{Light: "#000000", Dark: "#000000"},
		Border:        lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#E5E7EB"},
		BorderFocus:   lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#7C3AED"},
		Selection:     lipgloss.AdaptiveColor{Light: "#EDE9FE", Dark: "#EDE9FE"},
		Cursor:        lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#7C3AED"},
		Link:          lipgloss.AdaptiveColor{Light: "#2563EB", Dark: "#2563EB"},
	}
}

// DraculaPalette returns the Dracula theme color palette
func DraculaPalette() ColorPalette {
	return ColorPalette{
		Primary:       lipgloss.AdaptiveColor{Light: "#BD93F9", Dark: "#BD93F9"},
		Secondary:     lipgloss.AdaptiveColor{Light: "#8BE9FD", Dark: "#8BE9FD"},
		Accent:        lipgloss.AdaptiveColor{Light: "#FF79C6", Dark: "#FF79C6"},
		Success:       lipgloss.AdaptiveColor{Light: "#50FA7B", Dark: "#50FA7B"},
		Warning:       lipgloss.AdaptiveColor{Light: "#FFB86C", Dark: "#FFB86C"},
		Error:         lipgloss.AdaptiveColor{Light: "#FF5555", Dark: "#FF5555"},
		Info:          lipgloss.AdaptiveColor{Light: "#8BE9FD", Dark: "#8BE9FD"},
		Text:          lipgloss.AdaptiveColor{Light: "#F8F8F2", Dark: "#F8F8F2"},
		TextMuted:     lipgloss.AdaptiveColor{Light: "#6272A4", Dark: "#6272A4"},
		TextSubtle:    lipgloss.AdaptiveColor{Light: "#44475A", Dark: "#44475A"},
		Background:    lipgloss.AdaptiveColor{Light: "#282A36", Dark: "#282A36"},
		BackgroundAlt: lipgloss.AdaptiveColor{Light: "#44475A", Dark: "#44475A"},
		Surface:       lipgloss.AdaptiveColor{Light: "#343746", Dark: "#343746"},
		Overlay:       lipgloss.AdaptiveColor{Light: "#1E1F29", Dark: "#1E1F29"},
		Border:        lipgloss.AdaptiveColor{Light: "#44475A", Dark: "#44475A"},
		BorderFocus:   lipgloss.AdaptiveColor{Light: "#BD93F9", Dark: "#BD93F9"},
		Selection:     lipgloss.AdaptiveColor{Light: "#44475A", Dark: "#44475A"},
		Cursor:        lipgloss.AdaptiveColor{Light: "#F8F8F2", Dark: "#F8F8F2"},
		Link:          lipgloss.AdaptiveColor{Light: "#8BE9FD", Dark: "#8BE9FD"},
	}
}

// NordPalette returns the Nord theme color palette
func NordPalette() ColorPalette {
	return ColorPalette{
		Primary:       lipgloss.AdaptiveColor{Light: "#88C0D0", Dark: "#88C0D0"},
		Secondary:     lipgloss.AdaptiveColor{Light: "#81A1C1", Dark: "#81A1C1"},
		Accent:        lipgloss.AdaptiveColor{Light: "#5E81AC", Dark: "#5E81AC"},
		Success:       lipgloss.AdaptiveColor{Light: "#A3BE8C", Dark: "#A3BE8C"},
		Warning:       lipgloss.AdaptiveColor{Light: "#EBCB8B", Dark: "#EBCB8B"},
		Error:         lipgloss.AdaptiveColor{Light: "#BF616A", Dark: "#BF616A"},
		Info:          lipgloss.AdaptiveColor{Light: "#88C0D0", Dark: "#88C0D0"},
		Text:          lipgloss.AdaptiveColor{Light: "#ECEFF4", Dark: "#ECEFF4"},
		TextMuted:     lipgloss.AdaptiveColor{Light: "#D8DEE9", Dark: "#D8DEE9"},
		TextSubtle:    lipgloss.AdaptiveColor{Light: "#4C566A", Dark: "#4C566A"},
		Background:    lipgloss.AdaptiveColor{Light: "#2E3440", Dark: "#2E3440"},
		BackgroundAlt: lipgloss.AdaptiveColor{Light: "#3B4252", Dark: "#3B4252"},
		Surface:       lipgloss.AdaptiveColor{Light: "#434C5E", Dark: "#434C5E"},
		Overlay:       lipgloss.AdaptiveColor{Light: "#2E3440", Dark: "#2E3440"},
		Border:        lipgloss.AdaptiveColor{Light: "#4C566A", Dark: "#4C566A"},
		BorderFocus:   lipgloss.AdaptiveColor{Light: "#88C0D0", Dark: "#88C0D0"},
		Selection:     lipgloss.AdaptiveColor{Light: "#434C5E", Dark: "#434C5E"},
		Cursor:        lipgloss.AdaptiveColor{Light: "#D8DEE9", Dark: "#D8DEE9"},
		Link:          lipgloss.AdaptiveColor{Light: "#81A1C1", Dark: "#81A1C1"},
	}
}

// GruvboxPalette returns the Gruvbox theme color palette
func GruvboxPalette() ColorPalette {
	return ColorPalette{
		Primary:       lipgloss.AdaptiveColor{Light: "#D79921", Dark: "#FABD2F"},
		Secondary:     lipgloss.AdaptiveColor{Light: "#458588", Dark: "#83A598"},
		Accent:        lipgloss.AdaptiveColor{Light: "#B16286", Dark: "#D3869B"},
		Success:       lipgloss.AdaptiveColor{Light: "#98971A", Dark: "#B8BB26"},
		Warning:       lipgloss.AdaptiveColor{Light: "#D79921", Dark: "#FABD2F"},
		Error:         lipgloss.AdaptiveColor{Light: "#CC241D", Dark: "#FB4934"},
		Info:          lipgloss.AdaptiveColor{Light: "#458588", Dark: "#83A598"},
		Text:          lipgloss.AdaptiveColor{Light: "#EBDBB2", Dark: "#EBDBB2"},
		TextMuted:     lipgloss.AdaptiveColor{Light: "#A89984", Dark: "#A89984"},
		TextSubtle:    lipgloss.AdaptiveColor{Light: "#665C54", Dark: "#665C54"},
		Background:    lipgloss.AdaptiveColor{Light: "#282828", Dark: "#282828"},
		BackgroundAlt: lipgloss.AdaptiveColor{Light: "#3C3836", Dark: "#3C3836"},
		Surface:       lipgloss.AdaptiveColor{Light: "#504945", Dark: "#504945"},
		Overlay:       lipgloss.AdaptiveColor{Light: "#1D2021", Dark: "#1D2021"},
		Border:        lipgloss.AdaptiveColor{Light: "#504945", Dark: "#504945"},
		BorderFocus:   lipgloss.AdaptiveColor{Light: "#D79921", Dark: "#FABD2F"},
		Selection:     lipgloss.AdaptiveColor{Light: "#504945", Dark: "#504945"},
		Cursor:        lipgloss.AdaptiveColor{Light: "#EBDBB2", Dark: "#EBDBB2"},
		Link:          lipgloss.AdaptiveColor{Light: "#458588", Dark: "#83A598"},
	}
}
