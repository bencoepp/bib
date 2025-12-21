package layout

import (
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Breakpoint represents a responsive breakpoint
type Breakpoint int

const (
	// BreakpointXS is for minimal terminals (30-49 cols)
	BreakpointXS Breakpoint = iota
	// BreakpointSM is for compact terminals (50-79 cols)
	BreakpointSM
	// BreakpointMD is for standard terminals (80-119 cols)
	BreakpointMD
	// BreakpointLG is for extended terminals (120-179 cols)
	BreakpointLG
	// BreakpointXL is for wide terminals (180-239 cols)
	BreakpointXL
	// BreakpointXXL is for ultrawide terminals (240+ cols)
	BreakpointXXL
)

// BreakpointThresholds defines the width thresholds for each breakpoint
var BreakpointThresholds = map[Breakpoint]int{
	BreakpointXS:  30,
	BreakpointSM:  50,
	BreakpointMD:  80,
	BreakpointLG:  120,
	BreakpointXL:  180,
	BreakpointXXL: 240,
}

// HeightBreakpoint represents a vertical breakpoint
type HeightBreakpoint int

const (
	HeightBreakpointXS HeightBreakpoint = iota // < 15 rows
	HeightBreakpointSM                         // 15-23 rows
	HeightBreakpointMD                         // 24-39 rows
	HeightBreakpointLG                         // 40-59 rows
	HeightBreakpointXL                         // 60+ rows
)

// HeightBreakpointThresholds defines the height thresholds
var HeightBreakpointThresholds = map[HeightBreakpoint]int{
	HeightBreakpointXS: 0,
	HeightBreakpointSM: 15,
	HeightBreakpointMD: 24,
	HeightBreakpointLG: 40,
	HeightBreakpointXL: 60,
}

// Responsive holds responsive configuration for different breakpoints
type Responsive[T any] struct {
	XS  T
	SM  T
	MD  T
	LG  T
	XL  T
	XXL T
}

// Get returns the value for the given breakpoint
func (r Responsive[T]) Get(bp Breakpoint) T {
	switch bp {
	case BreakpointXS:
		return r.XS
	case BreakpointSM:
		return r.SM
	case BreakpointMD:
		return r.MD
	case BreakpointLG:
		return r.LG
	case BreakpointXL:
		return r.XL
	case BreakpointXXL:
		return r.XXL
	default:
		return r.MD
	}
}

// GetBreakpoint returns the breakpoint for a given width
func GetBreakpoint(width int) Breakpoint {
	if width >= BreakpointThresholds[BreakpointXXL] {
		return BreakpointXXL
	}
	if width >= BreakpointThresholds[BreakpointXL] {
		return BreakpointXL
	}
	if width >= BreakpointThresholds[BreakpointLG] {
		return BreakpointLG
	}
	if width >= BreakpointThresholds[BreakpointMD] {
		return BreakpointMD
	}
	if width >= BreakpointThresholds[BreakpointSM] {
		return BreakpointSM
	}
	return BreakpointXS
}

// GetHeightBreakpoint returns the height breakpoint for a given height
func GetHeightBreakpoint(height int) HeightBreakpoint {
	if height >= HeightBreakpointThresholds[HeightBreakpointXL] {
		return HeightBreakpointXL
	}
	if height >= HeightBreakpointThresholds[HeightBreakpointLG] {
		return HeightBreakpointLG
	}
	if height >= HeightBreakpointThresholds[HeightBreakpointMD] {
		return HeightBreakpointMD
	}
	if height >= HeightBreakpointThresholds[HeightBreakpointSM] {
		return HeightBreakpointSM
	}
	return HeightBreakpointXS
}

// BreakpointName returns the name of a breakpoint
func BreakpointName(bp Breakpoint) string {
	switch bp {
	case BreakpointXS:
		return "xs"
	case BreakpointSM:
		return "sm"
	case BreakpointMD:
		return "md"
	case BreakpointLG:
		return "lg"
	case BreakpointXL:
		return "xl"
	case BreakpointXXL:
		return "xxl"
	default:
		return "unknown"
	}
}

// BreakpointDescription returns a human-readable description
func BreakpointDescription(bp Breakpoint) string {
	switch bp {
	case BreakpointXS:
		return "Minimal (30-49)"
	case BreakpointSM:
		return "Compact (50-79)"
	case BreakpointMD:
		return "Standard (80-119)"
	case BreakpointLG:
		return "Extended (120-179)"
	case BreakpointXL:
		return "Wide (180-239)"
	case BreakpointXXL:
		return "Ultrawide (240+)"
	default:
		return "Unknown"
	}
}

// IsAtLeast checks if the current breakpoint is at least the given breakpoint
func IsAtLeast(current, minimum Breakpoint) bool {
	return current >= minimum
}

// IsAtMost checks if the current breakpoint is at most the given breakpoint
func IsAtMost(current, maximum Breakpoint) bool {
	return current <= maximum
}

// IsBetween checks if the current breakpoint is between min and max (inclusive)
func IsBetween(current, min, max Breakpoint) bool {
	return current >= min && current <= max
}

// ResponsiveValue returns the appropriate value based on width
func ResponsiveValue[T any](width int, values Responsive[T]) T {
	return values.Get(GetBreakpoint(width))
}

// ResponsiveInt is a helper for responsive integer values
type ResponsiveInt = Responsive[int]

// ResponsiveString is a helper for responsive string values
type ResponsiveString = Responsive[string]

// ResponsiveBool is a helper for responsive boolean values
type ResponsiveBool = Responsive[bool]

// NewResponsive creates a Responsive with all values set to the same default
func NewResponsive[T any](defaultValue T) Responsive[T] {
	return Responsive[T]{
		XS:  defaultValue,
		SM:  defaultValue,
		MD:  defaultValue,
		LG:  defaultValue,
		XL:  defaultValue,
		XXL: defaultValue,
	}
}

// NewResponsiveScale creates a Responsive with scaled values
func NewResponsiveScale(xs, sm, md, lg, xl, xxl int) ResponsiveInt {
	return Responsive[int]{
		XS:  xs,
		SM:  sm,
		MD:  md,
		LG:  lg,
		XL:  xl,
		XXL: xxl,
	}
}

// ResponsiveColumns returns recommended column count for width
func ResponsiveColumns(width int) int {
	bp := GetBreakpoint(width)
	switch bp {
	case BreakpointXS:
		return 1
	case BreakpointSM:
		return 1
	case BreakpointMD:
		return 2
	case BreakpointLG:
		return 3
	case BreakpointXL:
		return 4
	case BreakpointXXL:
		return 6
	default:
		return 2
	}
}

// ResponsivePadding returns recommended padding for width
func ResponsivePadding(width int) int {
	bp := GetBreakpoint(width)
	switch bp {
	case BreakpointXS:
		return 0
	case BreakpointSM:
		return 1
	case BreakpointMD:
		return 1
	case BreakpointLG:
		return 2
	case BreakpointXL:
		return 2
	case BreakpointXXL:
		return 3
	default:
		return 1
	}
}

// ResponsiveGap returns recommended gap for width
func ResponsiveGap(width int) int {
	bp := GetBreakpoint(width)
	switch bp {
	case BreakpointXS:
		return 0
	case BreakpointSM:
		return 1
	case BreakpointMD:
		return 1
	case BreakpointLG:
		return 2
	case BreakpointXL:
		return 2
	case BreakpointXXL:
		return 2
	default:
		return 1
	}
}

// --- Debounced Window Size Handling ---

// DebouncedResizeMsg is sent after debounce delay
type DebouncedResizeMsg struct {
	Width  int
	Height int
}

// ResizeDebouncer handles debounced window resize events
type ResizeDebouncer struct {
	mu            sync.Mutex
	pendingWidth  int
	pendingHeight int
	delay         time.Duration
	timer         *time.Timer
}

// NewResizeDebouncer creates a new debouncer with the specified delay
func NewResizeDebouncer(delay time.Duration) *ResizeDebouncer {
	return &ResizeDebouncer{
		delay: delay,
	}
}

// DefaultResizeDebouncer creates a debouncer with 50ms delay
func DefaultResizeDebouncer() *ResizeDebouncer {
	return NewResizeDebouncer(50 * time.Millisecond)
}

// Debounce queues a resize event and returns a command that fires after the delay
func (d *ResizeDebouncer) Debounce(width, height int) tea.Cmd {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.pendingWidth = width
	d.pendingHeight = height

	// Cancel existing timer
	if d.timer != nil {
		d.timer.Stop()
	}

	return func() tea.Msg {
		time.Sleep(d.delay)
		d.mu.Lock()
		w, h := d.pendingWidth, d.pendingHeight
		d.mu.Unlock()
		return DebouncedResizeMsg{Width: w, Height: h}
	}
}

// --- Layout Mode ---

// LayoutMode describes the current layout configuration
type LayoutMode int

const (
	// LayoutMinimal - single panel, no sidebar (XS)
	LayoutMinimal LayoutMode = iota
	// LayoutCompact - collapsed sidebar, reduced info (SM)
	LayoutCompact
	// LayoutStandard - default layout with full sidebar (MD)
	LayoutStandard
	// LayoutExtended - additional columns, previews (LG)
	LayoutExtended
	// LayoutWide - side-by-side panels (XL)
	LayoutWide
	// LayoutUltrawide - multi-panel workspace (XXL)
	LayoutUltrawide
)

// GetLayoutMode returns the layout mode for a breakpoint
func GetLayoutMode(bp Breakpoint) LayoutMode {
	switch bp {
	case BreakpointXS:
		return LayoutMinimal
	case BreakpointSM:
		return LayoutCompact
	case BreakpointMD:
		return LayoutStandard
	case BreakpointLG:
		return LayoutExtended
	case BreakpointXL:
		return LayoutWide
	case BreakpointXXL:
		return LayoutUltrawide
	default:
		return LayoutStandard
	}
}

// LayoutModeName returns the name of a layout mode
func LayoutModeName(mode LayoutMode) string {
	switch mode {
	case LayoutMinimal:
		return "minimal"
	case LayoutCompact:
		return "compact"
	case LayoutStandard:
		return "standard"
	case LayoutExtended:
		return "extended"
	case LayoutWide:
		return "wide"
	case LayoutUltrawide:
		return "ultrawide"
	default:
		return "unknown"
	}
}

// --- Layout Constraints ---

// LayoutConstraints defines sizing constraints for layout regions
type LayoutConstraints struct {
	// Sidebar constraints
	SidebarMinWidth     int
	SidebarMaxWidth     int
	SidebarDefaultWidth int
	SidebarCollapsed    int // Icon-only width

	// Log panel constraints
	LogPanelMinHeight     int
	LogPanelMaxHeight     int // As percentage of total height
	LogPanelDefaultHeight int

	// Info bar
	InfoBarHeight int

	// Status bar
	StatusBarHeight int

	// Tab bar
	TabBarHeight int
}

// DefaultConstraints returns default layout constraints
func DefaultConstraints() LayoutConstraints {
	return LayoutConstraints{
		SidebarMinWidth:       3,
		SidebarMaxWidth:       40,
		SidebarDefaultWidth:   20,
		SidebarCollapsed:      3,
		LogPanelMinHeight:     4,
		LogPanelMaxHeight:     50, // percentage
		LogPanelDefaultHeight: 6,
		InfoBarHeight:         1,
		StatusBarHeight:       1,
		TabBarHeight:          1,
	}
}

// ConstraintsForBreakpoint returns adjusted constraints for a breakpoint
func ConstraintsForBreakpoint(bp Breakpoint) LayoutConstraints {
	c := DefaultConstraints()

	switch bp {
	case BreakpointXS:
		c.SidebarDefaultWidth = 0 // Hidden
		c.LogPanelDefaultHeight = 0
	case BreakpointSM:
		c.SidebarDefaultWidth = 3 // Icons only
		c.LogPanelDefaultHeight = 0
	case BreakpointMD:
		c.SidebarDefaultWidth = 20
	case BreakpointLG:
		c.SidebarDefaultWidth = 22
	case BreakpointXL:
		c.SidebarDefaultWidth = 24
	case BreakpointXXL:
		c.SidebarDefaultWidth = 26
	}

	return c
}
