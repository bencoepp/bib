package layout

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Context provides layout information to components
type Context struct {
	// Terminal dimensions
	Width  int
	Height int

	// Current breakpoints
	WidthBreakpoint  Breakpoint
	HeightBreakpoint HeightBreakpoint

	// Computed helpers
	ContentWidth  int // Width minus standard margins
	ContentHeight int // Height minus standard margins
}

// NewContext creates a new layout context
func NewContext(width, height int) *Context {
	padding := ResponsivePadding(width)
	return &Context{
		Width:            width,
		Height:           height,
		WidthBreakpoint:  GetBreakpoint(width),
		HeightBreakpoint: GetHeightBreakpoint(height),
		ContentWidth:     width - (padding * 2),
		ContentHeight:    height - (padding * 2),
	}
}

// Update updates the context with new dimensions
func (c *Context) Update(width, height int) {
	c.Width = width
	c.Height = height
	c.WidthBreakpoint = GetBreakpoint(width)
	c.HeightBreakpoint = GetHeightBreakpoint(height)
	padding := ResponsivePadding(width)
	c.ContentWidth = width - (padding * 2)
	c.ContentHeight = height - (padding * 2)
}

// IsSmall returns true if the terminal is considered small
func (c *Context) IsSmall() bool {
	return c.WidthBreakpoint <= BreakpointSM
}

// IsMedium returns true if the terminal is medium sized
func (c *Context) IsMedium() bool {
	return c.WidthBreakpoint == BreakpointMD
}

// IsLarge returns true if the terminal is large
func (c *Context) IsLarge() bool {
	return c.WidthBreakpoint >= BreakpointLG
}

// Columns returns recommended number of columns for current width
func (c *Context) Columns() int {
	return ResponsiveColumns(c.Width)
}

// Padding returns recommended padding for current width
func (c *Context) Padding() int {
	return ResponsivePadding(c.Width)
}

// Gap returns recommended gap for current width
func (c *Context) Gap() int {
	return ResponsiveGap(c.Width)
}

// SidebarWidth returns recommended sidebar width
func (c *Context) SidebarWidth() int {
	switch c.WidthBreakpoint {
	case BreakpointXS, BreakpointSM:
		return 0 // No sidebar on small screens
	case BreakpointMD:
		return 20
	case BreakpointLG:
		return 25
	case BreakpointXL:
		return 30
	default:
		return 25
	}
}

// MainWidth returns the main content width (excluding sidebar)
func (c *Context) MainWidth() int {
	sidebar := c.SidebarWidth()
	if sidebar == 0 {
		return c.ContentWidth
	}
	return c.ContentWidth - sidebar - c.Gap()
}

// ModalWidth returns recommended modal width
func (c *Context) ModalWidth() int {
	switch c.WidthBreakpoint {
	case BreakpointXS:
		return c.Width - 2
	case BreakpointSM:
		return c.Width - 4
	case BreakpointMD:
		return 50
	case BreakpointLG:
		return 60
	case BreakpointXL:
		return 70
	default:
		return 60
	}
}

// ModalHeight returns recommended modal height
func (c *Context) ModalHeight() int {
	switch c.HeightBreakpoint {
	case HeightBreakpointXS:
		return c.Height - 2
	case HeightBreakpointSM:
		return c.Height - 4
	case HeightBreakpointMD:
		return 15
	case HeightBreakpointLG:
		return 20
	case HeightBreakpointXL:
		return 25
	default:
		return 15
	}
}

// CardWidth returns recommended card width for current columns
func (c *Context) CardWidth() int {
	cols := c.Columns()
	if cols <= 1 {
		return c.ContentWidth
	}
	return (c.ContentWidth - (c.Gap() * (cols - 1))) / cols
}

// WithSidebar returns true if a sidebar should be shown
func (c *Context) WithSidebar() bool {
	return c.WidthBreakpoint >= BreakpointMD
}

// WithHelp returns true if help text should be shown
func (c *Context) WithHelp() bool {
	return c.HeightBreakpoint >= HeightBreakpointSM
}

// WithFooter returns true if footer should be shown
func (c *Context) WithFooter() bool {
	return c.HeightBreakpoint >= HeightBreakpointMD
}

// ContextMsg is sent when the layout context changes
type ContextMsg struct {
	Context *Context
}

// HandleWindowSize creates a ContextMsg from a WindowSizeMsg
func HandleWindowSize(msg tea.WindowSizeMsg) ContextMsg {
	return ContextMsg{
		Context: NewContext(msg.Width, msg.Height),
	}
}

// GlobalContext is a shared layout context
var GlobalContext = NewContext(80, 24)

// UpdateGlobalContext updates the global context
func UpdateGlobalContext(width, height int) {
	GlobalContext.Update(width, height)
}
