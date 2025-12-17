package layout

// Breakpoint represents a responsive breakpoint
type Breakpoint int

const (
	// BreakpointXS is for very small terminals (< 40 cols)
	BreakpointXS Breakpoint = iota
	// BreakpointSM is for small terminals (40-59 cols)
	BreakpointSM
	// BreakpointMD is for medium terminals (60-79 cols)
	BreakpointMD
	// BreakpointLG is for large terminals (80-119 cols)
	BreakpointLG
	// BreakpointXL is for extra large terminals (120+ cols)
	BreakpointXL
)

// BreakpointThresholds defines the width thresholds for each breakpoint
var BreakpointThresholds = map[Breakpoint]int{
	BreakpointXS: 0,
	BreakpointSM: 40,
	BreakpointMD: 60,
	BreakpointLG: 80,
	BreakpointXL: 120,
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
	XS T
	SM T
	MD T
	LG T
	XL T
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
	default:
		return r.MD
	}
}

// GetBreakpoint returns the breakpoint for a given width
func GetBreakpoint(width int) Breakpoint {
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
	default:
		return "unknown"
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
		XS: defaultValue,
		SM: defaultValue,
		MD: defaultValue,
		LG: defaultValue,
		XL: defaultValue,
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
		return 2
	case BreakpointLG:
		return 2
	case BreakpointXL:
		return 3
	default:
		return 2
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
	default:
		return 1
	}
}
