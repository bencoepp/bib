// Package layout provides terminal capability detection and configuration.
package layout

import (
	"os"
	"strings"
	"sync"
)

// TerminalCapabilities describes what the terminal supports
type TerminalCapabilities struct {
	// Unicode support
	Unicode bool
	// True color (24-bit) support
	TrueColor bool
	// 256 color support
	Color256 bool
	// Basic 16 color support
	Color16 bool
	// Mouse support
	Mouse bool
	// Bracketed paste support
	BracketedPaste bool
}

var (
	detectedCaps     *TerminalCapabilities
	capsOnce         sync.Once
	capsOverride     *TerminalCapabilities
	capsOverrideLock sync.RWMutex
)

// DetectCapabilities auto-detects terminal capabilities
func DetectCapabilities() *TerminalCapabilities {
	capsOnce.Do(func() {
		detectedCaps = &TerminalCapabilities{
			Unicode:        detectUnicode(),
			TrueColor:      detectTrueColor(),
			Color256:       detectColor256(),
			Color16:        true, // Assume basic color support
			Mouse:          true, // Most modern terminals support mouse
			BracketedPaste: true,
		}
	})
	return detectedCaps
}

// GetCapabilities returns the current terminal capabilities,
// checking for overrides first
func GetCapabilities() *TerminalCapabilities {
	capsOverrideLock.RLock()
	if capsOverride != nil {
		defer capsOverrideLock.RUnlock()
		return capsOverride
	}
	capsOverrideLock.RUnlock()
	return DetectCapabilities()
}

// SetCapabilities overrides auto-detected capabilities
func SetCapabilities(caps *TerminalCapabilities) {
	capsOverrideLock.Lock()
	defer capsOverrideLock.Unlock()
	capsOverride = caps
}

// ResetCapabilities clears any override and re-detects
func ResetCapabilities() {
	capsOverrideLock.Lock()
	defer capsOverrideLock.Unlock()
	capsOverride = nil
	capsOnce = sync.Once{}
}

// detectUnicode checks if the terminal likely supports Unicode
func detectUnicode() bool {
	// Check LANG and LC_ALL environment variables
	lang := os.Getenv("LANG")
	lcAll := os.Getenv("LC_ALL")
	lcCtype := os.Getenv("LC_CTYPE")

	for _, v := range []string{lcAll, lcCtype, lang} {
		if v != "" {
			lower := strings.ToLower(v)
			if strings.Contains(lower, "utf-8") || strings.Contains(lower, "utf8") {
				return true
			}
		}
	}

	// Check terminal type
	term := os.Getenv("TERM")
	termProgram := os.Getenv("TERM_PROGRAM")

	// Modern terminals that support Unicode
	unicodeTerms := []string{
		"xterm-256color",
		"screen-256color",
		"tmux-256color",
		"alacritty",
		"kitty",
		"wezterm",
		"iterm",
		"vscode",
	}

	for _, t := range unicodeTerms {
		if strings.Contains(strings.ToLower(term), t) {
			return true
		}
		if strings.Contains(strings.ToLower(termProgram), t) {
			return true
		}
	}

	// Windows Terminal and VS Code integrated terminal
	if os.Getenv("WT_SESSION") != "" {
		return true
	}
	if os.Getenv("VSCODE_INJECTION") != "" || termProgram == "vscode" {
		return true
	}

	// Default to true on modern systems
	// Most terminals today support UTF-8
	return true
}

// detectTrueColor checks if the terminal supports 24-bit color
func detectTrueColor() bool {
	colorTerm := os.Getenv("COLORTERM")
	if colorTerm == "truecolor" || colorTerm == "24bit" {
		return true
	}

	termProgram := os.Getenv("TERM_PROGRAM")
	trueColorTerms := []string{
		"iTerm.app",
		"Apple_Terminal",
		"Hyper",
		"vscode",
		"alacritty",
		"kitty",
		"wezterm",
	}

	for _, t := range trueColorTerms {
		if termProgram == t {
			return true
		}
	}

	// Windows Terminal
	if os.Getenv("WT_SESSION") != "" {
		return true
	}

	return false
}

// detectColor256 checks if the terminal supports 256 colors
func detectColor256() bool {
	// If true color is supported, 256 is also supported
	if detectTrueColor() {
		return true
	}

	term := os.Getenv("TERM")
	if strings.Contains(term, "256color") {
		return true
	}

	return false
}

// --- Icon Sets ---

// IconSet provides icons for UI elements
type IconSet struct {
	// Status indicators
	Check    string
	Cross    string
	Warning  string
	Info     string
	Question string

	// Shapes
	Bullet     string
	Circle     string
	CircleFill string
	Diamond    string
	Square     string
	Star       string

	// Arrows
	ArrowUp      string
	ArrowDown    string
	ArrowLeft    string
	ArrowRight   string
	ChevronUp    string
	ChevronDown  string
	ChevronLeft  string
	ChevronRight string

	// Tree
	TreeBranch     string
	TreeLastBranch string
	TreeVertical   string
	TreeExpanded   string
	TreeCollapsed  string

	// Progress
	ProgressFull  string
	ProgressEmpty string
	ProgressHalf  string

	// Misc
	Ellipsis     string
	Menu         string
	User         string
	Folder       string
	File         string
	Connected    string
	Disconnected string
	Upload       string
	Download     string
	Sync         string
	Time         string
	Peers        string
}

// UnicodeIcons returns the Unicode icon set
func UnicodeIcons() IconSet {
	return IconSet{
		Check:    "✓",
		Cross:    "✗",
		Warning:  "⚠",
		Info:     "ℹ",
		Question: "?",

		Bullet:     "•",
		Circle:     "○",
		CircleFill: "●",
		Diamond:    "◆",
		Square:     "■",
		Star:       "★",

		ArrowUp:      "↑",
		ArrowDown:    "↓",
		ArrowLeft:    "←",
		ArrowRight:   "→",
		ChevronUp:    "˄",
		ChevronDown:  "˅",
		ChevronLeft:  "‹",
		ChevronRight: "›",

		TreeBranch:     "├",
		TreeLastBranch: "└",
		TreeVertical:   "│",
		TreeExpanded:   "▼",
		TreeCollapsed:  "▶",

		ProgressFull:  "█",
		ProgressEmpty: "░",
		ProgressHalf:  "▓",

		Ellipsis:     "…",
		Menu:         "≡",
		User:         "👤",
		Folder:       "📁",
		File:         "📄",
		Connected:    "●",
		Disconnected: "○",
		Upload:       "↑",
		Download:     "↓",
		Sync:         "🔄",
		Time:         "🕐",
		Peers:        "👥",
	}
}

// ASCIIIcons returns the ASCII fallback icon set
func ASCIIIcons() IconSet {
	return IconSet{
		Check:    "[x]",
		Cross:    "[X]",
		Warning:  "[!]",
		Info:     "[i]",
		Question: "[?]",

		Bullet:     "*",
		Circle:     "o",
		CircleFill: "*",
		Diamond:    "<>",
		Square:     "#",
		Star:       "*",

		ArrowUp:      "^",
		ArrowDown:    "v",
		ArrowLeft:    "<",
		ArrowRight:   ">",
		ChevronUp:    "^",
		ChevronDown:  "v",
		ChevronLeft:  "<",
		ChevronRight: ">",

		TreeBranch:     "+",
		TreeLastBranch: "`",
		TreeVertical:   "|",
		TreeExpanded:   "v",
		TreeCollapsed:  ">",

		ProgressFull:  "#",
		ProgressEmpty: "-",
		ProgressHalf:  "=",

		Ellipsis:     "...",
		Menu:         "=",
		User:         "[U]",
		Folder:       "[D]",
		File:         "[F]",
		Connected:    "[*]",
		Disconnected: "[ ]",
		Upload:       "^",
		Download:     "v",
		Sync:         "[~]",
		Time:         "[T]",
		Peers:        "[P]",
	}
}

// GetIcons returns the appropriate icon set based on terminal capabilities
func GetIcons() IconSet {
	if GetCapabilities().Unicode {
		return UnicodeIcons()
	}
	return ASCIIIcons()
}

// --- Box Drawing ---

// BoxChars provides characters for drawing boxes
type BoxChars struct {
	TopLeft     string
	TopRight    string
	BottomLeft  string
	BottomRight string
	Horizontal  string
	Vertical    string
	LeftT       string
	RightT      string
	TopT        string
	BottomT     string
	Cross       string
}

// UnicodeBoxChars returns Unicode box drawing characters (rounded)
func UnicodeBoxChars() BoxChars {
	return BoxChars{
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "╰",
		BottomRight: "╯",
		Horizontal:  "─",
		Vertical:    "│",
		LeftT:       "├",
		RightT:      "┤",
		TopT:        "┬",
		BottomT:     "┴",
		Cross:       "┼",
	}
}

// UnicodeBoxCharsSharp returns Unicode box drawing characters (sharp corners)
func UnicodeBoxCharsSharp() BoxChars {
	return BoxChars{
		TopLeft:     "┌",
		TopRight:    "┐",
		BottomLeft:  "└",
		BottomRight: "┘",
		Horizontal:  "─",
		Vertical:    "│",
		LeftT:       "├",
		RightT:      "┤",
		TopT:        "┬",
		BottomT:     "┴",
		Cross:       "┼",
	}
}

// ASCIIBoxChars returns ASCII box drawing characters
func ASCIIBoxChars() BoxChars {
	return BoxChars{
		TopLeft:     "+",
		TopRight:    "+",
		BottomLeft:  "+",
		BottomRight: "+",
		Horizontal:  "-",
		Vertical:    "|",
		LeftT:       "+",
		RightT:      "+",
		TopT:        "+",
		BottomT:     "+",
		Cross:       "+",
	}
}

// GetBoxChars returns appropriate box characters based on terminal capabilities
func GetBoxChars() BoxChars {
	if GetCapabilities().Unicode {
		return UnicodeBoxChars()
	}
	return ASCIIBoxChars()
}
