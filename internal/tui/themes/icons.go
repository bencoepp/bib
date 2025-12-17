package themes

// Icons for consistent UI elements across all components
var (
	// Status icons
	IconCheck    = "✓"
	IconCross    = "✗"
	IconWarning  = "⚠"
	IconInfo     = "ℹ"
	IconQuestion = "?"

	// Shape icons
	IconCircle       = "○"
	IconCircleFill   = "●"
	IconDot          = "•"
	IconStar         = "★"
	IconStarEmpty    = "☆"
	IconHeart        = "♥"
	IconHeartEmpty   = "♡"
	IconDiamond      = "◆"
	IconDiamondEmpty = "◇"
	IconSquare       = "■"
	IconSquareEmpty  = "□"

	// Arrow icons
	IconArrowRight    = "→"
	IconArrowLeft     = "←"
	IconArrowUp       = "↑"
	IconArrowDown     = "↓"
	IconChevronRight  = "›"
	IconChevronLeft   = "‹"
	IconChevronUp     = "˄"
	IconChevronDown   = "˅"
	IconTriangleRight = "▶"
	IconTriangleLeft  = "◀"
	IconTriangleUp    = "▲"
	IconTriangleDown  = "▼"

	// Tree icons
	IconTreeBranch     = "├"
	IconTreeLastBranch = "└"
	IconTreeVertical   = "│"
	IconTreeHorizontal = "─"
	IconTreeExpanded   = "▼"
	IconTreeCollapsed  = "▶"
	IconTreeLeaf       = "•"

	// Progress icons
	IconSpinner       = "◐"
	IconSpinnerFrames = []string{"◐", "◓", "◑", "◒"}
	IconLoading       = "⋯"
	IconProgress      = "█"
	IconProgressEmpty = "░"
	IconProgressHalf  = "▓"

	// File icons
	IconFolder     = "📁"
	IconFolderOpen = "📂"
	IconFile       = "📄"
	IconFileCode   = "📝"

	// Misc icons
	IconBullet    = "•"
	IconEllipsis  = "…"
	IconPipe      = "|"
	IconSlash     = "/"
	IconBackslash = "\\"
	IconTilde     = "~"
	IconAt        = "@"
	IconHash      = "#"
	IconPlus      = "+"
	IconMinus     = "-"
	IconEquals    = "="
	IconAsterisk  = "*"

	// Box drawing (for manual layouts)
	BoxTopLeft     = "╭"
	BoxTopRight    = "╰"
	BoxBottomLeft  = "╮"
	BoxBottomRight = "╯"
	BoxHorizontal  = "─"
	BoxVertical    = "│"
)

// SpinnerDots returns a dots-style spinner frames
func SpinnerDots() []string {
	return []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
}

// SpinnerLine returns a line-style spinner frames
func SpinnerLine() []string {
	return []string{"-", "\\", "|", "/"}
}

// SpinnerCircle returns a circle-style spinner frames
func SpinnerCircle() []string {
	return []string{"◐", "◓", "◑", "◒"}
}

// SpinnerBounce returns a bounce-style spinner frames
func SpinnerBounce() []string {
	return []string{"⠁", "⠂", "⠄", "⠂"}
}

// SpinnerPulse returns a pulse-style spinner frames
func SpinnerPulse() []string {
	return []string{"█", "▓", "▒", "░", "▒", "▓"}
}

// SpinnerGrow returns a grow-style spinner frames
func SpinnerGrow() []string {
	return []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█", "▇", "▆", "▅", "▄", "▃", "▂"}
}
