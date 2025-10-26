package styles

import (
	"os"
	"strconv"
	"strings"
)

func DetectDefault() (Theme, string) {
	mode := "dark"
	if v := os.Getenv("COLORFGBG"); v != "" {
		// Common formats: "15;0" (fg;bg) or "0;15;..."; we care about the last (bg).
		parts := strings.Split(v, ";")
		bgPart := parts[len(parts)-1]
		if n, err := strconv.Atoi(bgPart); err == nil {
			// Rough heuristic: >= 7 is light background palette slot.
			if n >= 7 {
				mode = "light"
			}
		}
	}
	if mode == "light" {
		return DefaultLight, mode
	}
	return DefaultDark, mode
}

func FromMode(mode string) (Theme, string) {
	switch strings.ToLower(mode) {
	case "light":
		return DefaultLight, "light"
	case "dark":
		fallthrough
	default:
		return DefaultDark, "dark"
	}
}
