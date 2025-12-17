package layout

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Container provides a centered, max-width container for content
type Container struct {
	maxWidth int
	minWidth int
	width    int
	height   int
	padding  int
	paddingX int
	paddingY int
	center   bool
	centerV  bool
	style    lipgloss.Style
}

// NewContainer creates a new container
func NewContainer() *Container {
	return &Container{
		maxWidth: 0,
		minWidth: 0,
		padding:  0,
		center:   false,
		centerV:  false,
	}
}

// MaxWidth sets the maximum width
func (c *Container) MaxWidth(w int) *Container {
	c.maxWidth = w
	return c
}

// MinWidth sets the minimum width
func (c *Container) MinWidth(w int) *Container {
	c.minWidth = w
	return c
}

// Width sets the available width
func (c *Container) Width(w int) *Container {
	c.width = w
	return c
}

// Height sets the available height
func (c *Container) Height(h int) *Container {
	c.height = h
	return c
}

// Padding sets padding on all sides
func (c *Container) Padding(p int) *Container {
	c.padding = p
	c.paddingX = p
	c.paddingY = p
	return c
}

// PaddingX sets horizontal padding
func (c *Container) PaddingX(p int) *Container {
	c.paddingX = p
	return c
}

// PaddingY sets vertical padding
func (c *Container) PaddingY(p int) *Container {
	c.paddingY = p
	return c
}

// Center enables horizontal centering
func (c *Container) Center() *Container {
	c.center = true
	return c
}

// CenterVertical enables vertical centering
func (c *Container) CenterVertical() *Container {
	c.centerV = true
	return c
}

// CenterBoth enables both horizontal and vertical centering
func (c *Container) CenterBoth() *Container {
	c.center = true
	c.centerV = true
	return c
}

// Style sets a custom style
func (c *Container) Style(s lipgloss.Style) *Container {
	c.style = s
	return c
}

// Render renders content within the container
func (c *Container) Render(content string) string {
	// Calculate effective width
	effectiveWidth := c.width
	if c.maxWidth > 0 && effectiveWidth > c.maxWidth {
		effectiveWidth = c.maxWidth
	}
	if c.minWidth > 0 && effectiveWidth < c.minWidth {
		effectiveWidth = c.minWidth
	}

	// Apply padding
	paddedContent := content
	if c.paddingX > 0 || c.paddingY > 0 {
		paddedStyle := lipgloss.NewStyle().
			PaddingLeft(c.paddingX).
			PaddingRight(c.paddingX).
			PaddingTop(c.paddingY).
			PaddingBottom(c.paddingY)
		paddedContent = paddedStyle.Render(content)
	}

	// Apply custom style
	if c.style.Value() != "" {
		paddedContent = c.style.Render(paddedContent)
	}

	// Apply centering
	result := paddedContent
	if c.center && c.width > 0 {
		contentWidth := lipgloss.Width(paddedContent)
		if contentWidth < c.width {
			result = lipgloss.PlaceHorizontal(c.width, lipgloss.Center, paddedContent)
		}
	}

	if c.centerV && c.height > 0 {
		contentHeight := lipgloss.Height(result)
		if contentHeight < c.height {
			result = lipgloss.PlaceVertical(c.height, lipgloss.Center, result)
		}
	}

	return result
}

// ResponsiveContainer creates a container that adapts to breakpoints
type ResponsiveContainer struct {
	width     int
	height    int
	maxWidths ResponsiveInt
	paddings  ResponsiveInt
}

// NewResponsiveContainer creates a responsive container
func NewResponsiveContainer(width, height int) *ResponsiveContainer {
	return &ResponsiveContainer{
		width:  width,
		height: height,
		maxWidths: ResponsiveInt{
			XS: 40,
			SM: 55,
			MD: 70,
			LG: 90,
			XL: 120,
		},
		paddings: ResponsiveInt{
			XS: 0,
			SM: 1,
			MD: 2,
			LG: 2,
			XL: 3,
		},
	}
}

// MaxWidths sets responsive max widths
func (c *ResponsiveContainer) MaxWidths(widths ResponsiveInt) *ResponsiveContainer {
	c.maxWidths = widths
	return c
}

// Paddings sets responsive paddings
func (c *ResponsiveContainer) Paddings(paddings ResponsiveInt) *ResponsiveContainer {
	c.paddings = paddings
	return c
}

// Render renders content with responsive settings
func (c *ResponsiveContainer) Render(content string) string {
	bp := GetBreakpoint(c.width)
	maxWidth := c.maxWidths.Get(bp)
	padding := c.paddings.Get(bp)

	return NewContainer().
		Width(c.width).
		Height(c.height).
		MaxWidth(maxWidth).
		Padding(padding).
		Center().
		Render(content)
}

// Spacer creates vertical space
func Spacer(lines int) string {
	if lines <= 0 {
		return ""
	}
	return strings.Repeat("\n", lines)
}

// HorizontalSpacer creates horizontal space
func HorizontalSpacer(width int) string {
	if width <= 0 {
		return ""
	}
	return strings.Repeat(" ", width)
}

// Divider creates a horizontal divider line
func Divider(width int, style lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	return style.Render(strings.Repeat("─", width))
}

// DoubleDivider creates a double-line divider
func DoubleDivider(width int, style lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	return style.Render(strings.Repeat("═", width))
}

// DashedDivider creates a dashed divider
func DashedDivider(width int, style lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	return style.Render(strings.Repeat("╌", width))
}

// Place places content at a specific position within bounds
func Place(width, height int, hPos, vPos lipgloss.Position, content string) string {
	return lipgloss.Place(width, height, hPos, vPos, content)
}

// CenterContent centers content both horizontally and vertically
func CenterContent(width, height int, content string) string {
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

// Stack stacks multiple contents vertically with optional gap
func Stack(gap int, contents ...string) string {
	if len(contents) == 0 {
		return ""
	}

	separator := "\n"
	if gap > 0 {
		separator = strings.Repeat("\n", gap+1)
	}

	return strings.Join(contents, separator)
}

// Inline joins contents horizontally with optional gap
func Inline(gap int, contents ...string) string {
	if len(contents) == 0 {
		return ""
	}

	separator := ""
	if gap > 0 {
		separator = strings.Repeat(" ", gap)
	}

	return strings.Join(contents, separator)
}
