package component

import (
	"strings"
	"unicode/utf8"
)

// clamp restricts a value to a range
func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// max returns the larger of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// truncate truncates a string to the given width, adding ellipsis if needed
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= width {
		return s
	}
	if width <= 3 {
		return s[:width]
	}
	runes := []rune(s)
	return string(runes[:width-1]) + "…"
}

// truncateLeft truncates from the left, keeping the end of the string
func truncateLeft(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= width {
		return s
	}
	if width <= 3 {
		runes := []rune(s)
		return string(runes[len(runes)-width:])
	}
	runes := []rune(s)
	return "…" + string(runes[len(runes)-width+1:])
}

// truncateMiddle truncates from the middle
func truncateMiddle(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width <= 3 {
		return string(runes[:width])
	}
	leftWidth := (width - 1) / 2
	rightWidth := width - 1 - leftWidth
	return string(runes[:leftWidth]) + "…" + string(runes[len(runes)-rightWidth:])
}

// padRight pads a string on the right to the given width
func padRight(s string, width int) string {
	runeCount := utf8.RuneCountInString(s)
	if runeCount >= width {
		return s
	}
	return s + strings.Repeat(" ", width-runeCount)
}

// padLeft pads a string on the left to the given width
func padLeft(s string, width int) string {
	runeCount := utf8.RuneCountInString(s)
	if runeCount >= width {
		return s
	}
	return strings.Repeat(" ", width-runeCount) + s
}

// padCenter pads a string on both sides to center it
func padCenter(s string, width int) string {
	runeCount := utf8.RuneCountInString(s)
	if runeCount >= width {
		return s
	}
	totalPadding := width - runeCount
	leftPadding := totalPadding / 2
	rightPadding := totalPadding - leftPadding
	return strings.Repeat(" ", leftPadding) + s + strings.Repeat(" ", rightPadding)
}

// wrapText wraps text to fit within the given width
func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}

		currentLine := words[0]
		for _, word := range words[1:] {
			if utf8.RuneCountInString(currentLine)+1+utf8.RuneCountInString(word) <= width {
				currentLine += " " + word
			} else {
				lines = append(lines, currentLine)
				currentLine = word
			}
		}
		lines = append(lines, currentLine)
	}

	return lines
}

// repeat repeats a string n times
func repeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(s, n)
}

// joinLines joins strings with newlines
func joinLines(lines ...string) string {
	return strings.Join(lines, "\n")
}

// joinInline joins strings with spaces
func joinInline(items ...string) string {
	return strings.Join(items, " ")
}

// countLines returns the number of lines in a string
func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// firstLine returns the first line of a string
func firstLine(s string) string {
	if idx := strings.Index(s, "\n"); idx >= 0 {
		return s[:idx]
	}
	return s
}

// lastLine returns the last line of a string
func lastLine(s string) string {
	if idx := strings.LastIndex(s, "\n"); idx >= 0 {
		return s[idx+1:]
	}
	return s
}

// stripAnsi removes ANSI escape codes from a string
// This is a simplified version; for full support use a library
func stripAnsi(s string) string {
	var result strings.Builder
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}

// visibleWidth returns the visible width of a string (excluding ANSI codes)
func visibleWidth(s string) int {
	return utf8.RuneCountInString(stripAnsi(s))
}

// indent indents each line of a string
func indent(s string, spaces int) string {
	if spaces <= 0 {
		return s
	}
	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}

// dedent removes common leading whitespace from all lines
func dedent(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) == 0 {
		return s
	}

	// Find minimum indent
	minIndent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if minIndent == -1 || indent < minIndent {
			minIndent = indent
		}
	}

	if minIndent <= 0 {
		return s
	}

	// Remove common indent
	for i, line := range lines {
		if len(line) >= minIndent {
			lines[i] = line[minIndent:]
		}
	}

	return strings.Join(lines, "\n")
}

// Exported versions for use by other packages

// Truncate truncates a string to the given width with ellipsis
func Truncate(s string, width int) string {
	return truncate(s, width)
}

// TruncateLeft truncates from the left
func TruncateLeft(s string, width int) string {
	return truncateLeft(s, width)
}

// TruncateMiddle truncates from the middle
func TruncateMiddle(s string, width int) string {
	return truncateMiddle(s, width)
}

// PadRight pads a string on the right
func PadRight(s string, width int) string {
	return padRight(s, width)
}

// PadLeft pads a string on the left
func PadLeft(s string, width int) string {
	return padLeft(s, width)
}

// PadCenter pads a string on both sides
func PadCenter(s string, width int) string {
	return padCenter(s, width)
}

// WrapText wraps text to fit within width
func WrapText(text string, width int) []string {
	return wrapText(text, width)
}

// Indent indents text
func Indent(s string, spaces int) string {
	return indent(s, spaces)
}

// Dedent removes common leading whitespace
func Dedent(s string) string {
	return dedent(s)
}

// Clamp restricts a value to a range
func Clamp(v, min, max int) int {
	return clamp(v, min, max)
}

// Max returns the larger of two integers
func Max(a, b int) int {
	return max(a, b)
}

// Min returns the smaller of two integers
func Min(a, b int) int {
	return min(a, b)
}
