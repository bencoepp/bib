// Package layout provides responsive layout primitives for TUI components.
package layout

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Direction specifies flex direction
type Direction int

const (
	Row Direction = iota
	Column
)

// Justify specifies main-axis alignment
type Justify int

const (
	JustifyStart Justify = iota
	JustifyCenter
	JustifyEnd
	JustifySpaceBetween
	JustifySpaceAround
	JustifySpaceEvenly
)

// Align specifies cross-axis alignment
type Align int

const (
	AlignStart Align = iota
	AlignCenter
	AlignEnd
	AlignStretch
)

// FlexItem represents an item in a flex container
type FlexItem struct {
	Content string
	Grow    int // Flex grow factor (0 = no grow)
	Shrink  int // Flex shrink factor (1 = can shrink)
	Basis   int // Initial size (0 = auto)
	Style   lipgloss.Style
}

// Flex creates a flexible layout container
type Flex struct {
	direction Direction
	justify   Justify
	align     Align
	gap       int
	wrap      bool
	width     int
	height    int
	items     []FlexItem
}

// NewFlex creates a new flex container
func NewFlex() *Flex {
	return &Flex{
		direction: Row,
		justify:   JustifyStart,
		align:     AlignStart,
		gap:       0,
		wrap:      false,
	}
}

// Direction sets the flex direction
func (f *Flex) Direction(d Direction) *Flex {
	f.direction = d
	return f
}

// Justify sets the main-axis alignment
func (f *Flex) Justify(j Justify) *Flex {
	f.justify = j
	return f
}

// Align sets the cross-axis alignment
func (f *Flex) Align(a Align) *Flex {
	f.align = a
	return f
}

// Gap sets the gap between items
func (f *Flex) Gap(g int) *Flex {
	f.gap = g
	return f
}

// Wrap enables/disables wrapping
func (f *Flex) Wrap(w bool) *Flex {
	f.wrap = w
	return f
}

// Width sets the container width
func (f *Flex) Width(w int) *Flex {
	f.width = w
	return f
}

// Height sets the container height
func (f *Flex) Height(h int) *Flex {
	f.height = h
	return f
}

// Item adds an item with default settings
func (f *Flex) Item(content string) *Flex {
	f.items = append(f.items, FlexItem{
		Content: content,
		Grow:    0,
		Shrink:  1,
	})
	return f
}

// ItemWithGrow adds an item with grow factor
func (f *Flex) ItemWithGrow(content string, grow int) *Flex {
	f.items = append(f.items, FlexItem{
		Content: content,
		Grow:    grow,
		Shrink:  1,
	})
	return f
}

// ItemFlex adds a fully configured flex item
func (f *Flex) ItemFlex(item FlexItem) *Flex {
	f.items = append(f.items, item)
	return f
}

// Items adds multiple items at once
func (f *Flex) Items(contents ...string) *Flex {
	for _, c := range contents {
		f.Item(c)
	}
	return f
}

// Render renders the flex layout
func (f *Flex) Render() string {
	if len(f.items) == 0 {
		return ""
	}

	if f.direction == Row {
		return f.renderRow()
	}
	return f.renderColumn()
}

func (f *Flex) renderRow() string {
	// Calculate content widths
	contentWidths := make([]int, len(f.items))
	totalContentWidth := 0
	totalGrow := 0

	for i, item := range f.items {
		if item.Basis > 0 {
			contentWidths[i] = item.Basis
		} else {
			contentWidths[i] = lipgloss.Width(item.Content)
		}
		totalContentWidth += contentWidths[i]
		totalGrow += item.Grow
	}

	// Add gaps
	totalGapWidth := f.gap * (len(f.items) - 1)
	availableWidth := f.width - totalGapWidth

	// Distribute extra space based on grow factors
	if f.width > 0 && totalGrow > 0 {
		extraSpace := availableWidth - totalContentWidth
		if extraSpace > 0 {
			for i, item := range f.items {
				if item.Grow > 0 {
					growShare := (extraSpace * item.Grow) / totalGrow
					contentWidths[i] += growShare
				}
			}
		}
	}

	// Build rendered items
	rendered := make([]string, len(f.items))
	maxHeight := 0

	for i, item := range f.items {
		style := item.Style
		if f.width > 0 {
			style = style.Width(contentWidths[i])
		}
		rendered[i] = style.Render(item.Content)
		h := lipgloss.Height(rendered[i])
		if h > maxHeight {
			maxHeight = h
		}
	}

	// Apply cross-axis alignment
	for i := range rendered {
		h := lipgloss.Height(rendered[i])
		if h < maxHeight {
			switch f.align {
			case AlignCenter:
				rendered[i] = lipgloss.PlaceVertical(maxHeight, lipgloss.Center, rendered[i])
			case AlignEnd:
				rendered[i] = lipgloss.PlaceVertical(maxHeight, lipgloss.Bottom, rendered[i])
			case AlignStretch, AlignStart:
				rendered[i] = lipgloss.PlaceVertical(maxHeight, lipgloss.Top, rendered[i])
			}
		}
	}

	// Join with gaps and justification
	return f.joinHorizontal(rendered, maxHeight)
}

func (f *Flex) joinHorizontal(items []string, height int) string {
	if len(items) == 0 {
		return ""
	}

	totalWidth := 0
	for _, item := range items {
		totalWidth += lipgloss.Width(item)
	}

	if f.width == 0 || f.justify == JustifyStart {
		// Simple join with gaps
		gap := strings.Repeat(" ", f.gap)
		return strings.Join(items, gap)
	}

	availableSpace := f.width - totalWidth
	if availableSpace <= 0 {
		gap := strings.Repeat(" ", f.gap)
		return strings.Join(items, gap)
	}

	switch f.justify {
	case JustifyCenter:
		padding := availableSpace / 2
		gap := strings.Repeat(" ", f.gap)
		return strings.Repeat(" ", padding) + strings.Join(items, gap)

	case JustifyEnd:
		gap := strings.Repeat(" ", f.gap)
		return strings.Repeat(" ", availableSpace-(f.gap*(len(items)-1))) + strings.Join(items, gap)

	case JustifySpaceBetween:
		if len(items) == 1 {
			return items[0]
		}
		gapSize := availableSpace / (len(items) - 1)
		gap := strings.Repeat(" ", gapSize)
		return strings.Join(items, gap)

	case JustifySpaceAround:
		gapSize := availableSpace / len(items)
		halfGap := strings.Repeat(" ", gapSize/2)
		var result strings.Builder
		for i, item := range items {
			if i > 0 {
				result.WriteString(halfGap)
			}
			result.WriteString(halfGap)
			result.WriteString(item)
		}
		return result.String()

	case JustifySpaceEvenly:
		gapSize := availableSpace / (len(items) + 1)
		gap := strings.Repeat(" ", gapSize)
		var result strings.Builder
		for _, item := range items {
			result.WriteString(gap)
			result.WriteString(item)
		}
		return result.String()
	}

	gap := strings.Repeat(" ", f.gap)
	return strings.Join(items, gap)
}

func (f *Flex) renderColumn() string {
	// Calculate content heights
	contentHeights := make([]int, len(f.items))
	totalContentHeight := 0
	totalGrow := 0

	for i, item := range f.items {
		if item.Basis > 0 {
			contentHeights[i] = item.Basis
		} else {
			contentHeights[i] = lipgloss.Height(item.Content)
		}
		totalContentHeight += contentHeights[i]
		totalGrow += item.Grow
	}

	// Add gaps
	totalGapHeight := f.gap * (len(f.items) - 1)
	availableHeight := f.height - totalGapHeight

	// Distribute extra space based on grow factors
	if f.height > 0 && totalGrow > 0 {
		extraSpace := availableHeight - totalContentHeight
		if extraSpace > 0 {
			for i, item := range f.items {
				if item.Grow > 0 {
					growShare := (extraSpace * item.Grow) / totalGrow
					contentHeights[i] += growShare
				}
			}
		}
	}

	// Build rendered items
	rendered := make([]string, len(f.items))
	maxWidth := 0

	for i, item := range f.items {
		style := item.Style
		if f.height > 0 {
			style = style.Height(contentHeights[i])
		}
		rendered[i] = style.Render(item.Content)
		w := lipgloss.Width(rendered[i])
		if w > maxWidth {
			maxWidth = w
		}
	}

	// Apply cross-axis alignment (horizontal for columns)
	for i := range rendered {
		w := lipgloss.Width(rendered[i])
		if w < maxWidth && f.width > 0 {
			targetWidth := f.width
			if targetWidth == 0 {
				targetWidth = maxWidth
			}
			switch f.align {
			case AlignCenter:
				rendered[i] = lipgloss.PlaceHorizontal(targetWidth, lipgloss.Center, rendered[i])
			case AlignEnd:
				rendered[i] = lipgloss.PlaceHorizontal(targetWidth, lipgloss.Right, rendered[i])
			case AlignStretch:
				rendered[i] = lipgloss.NewStyle().Width(targetWidth).Render(rendered[i])
			case AlignStart:
				rendered[i] = lipgloss.PlaceHorizontal(targetWidth, lipgloss.Left, rendered[i])
			}
		}
	}

	// Join with gaps
	gap := strings.Repeat("\n", f.gap+1)
	if f.gap == 0 {
		gap = "\n"
	}
	return strings.Join(rendered, gap)
}

// FlexRow is a convenience function for horizontal flex layout
func FlexRow(items ...string) string {
	return NewFlex().Direction(Row).Items(items...).Render()
}

// FlexRowWithGap creates a row with specified gap
func FlexRowWithGap(gap int, items ...string) string {
	return NewFlex().Direction(Row).Gap(gap).Items(items...).Render()
}

// FlexRowCentered creates a centered row
func FlexRowCentered(width int, items ...string) string {
	return NewFlex().Direction(Row).Width(width).Justify(JustifyCenter).Items(items...).Render()
}

// FlexRowSpaceBetween creates a row with space between items
func FlexRowSpaceBetween(width int, items ...string) string {
	return NewFlex().Direction(Row).Width(width).Justify(JustifySpaceBetween).Items(items...).Render()
}

// FlexCol is a convenience function for vertical flex layout
func FlexCol(items ...string) string {
	return NewFlex().Direction(Column).Items(items...).Render()
}

// FlexColWithGap creates a column with specified gap
func FlexColWithGap(gap int, items ...string) string {
	return NewFlex().Direction(Column).Gap(gap).Items(items...).Render()
}

// FlexColCentered creates a centered column
func FlexColCentered(height int, items ...string) string {
	return NewFlex().Direction(Column).Height(height).Justify(JustifyCenter).Items(items...).Render()
}
