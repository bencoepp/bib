package layout

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// GridItem represents an item in a grid
type GridItem struct {
	Content string
	ColSpan int // Number of columns to span (default 1)
	RowSpan int // Number of rows to span (default 1)
	Style   lipgloss.Style
}

// Grid creates a CSS-grid-like layout
type Grid struct {
	columns    int   // Number of columns
	rows       int   // Number of rows (0 = auto)
	colWidths  []int // Individual column widths (nil = equal)
	rowHeights []int // Individual row heights (nil = auto)
	gap        int   // Gap between cells
	rowGap     int   // Gap between rows (overrides gap for rows)
	colGap     int   // Gap between columns (overrides gap for cols)
	width      int   // Total container width
	height     int   // Total container height
	items      []GridItem
	style      lipgloss.Style
}

// NewGrid creates a new grid with specified columns
func NewGrid(columns int) *Grid {
	return &Grid{
		columns: columns,
		rows:    0,
		gap:     1,
	}
}

// Columns sets the number of columns
func (g *Grid) Columns(c int) *Grid {
	g.columns = c
	return g
}

// Rows sets the number of rows
func (g *Grid) Rows(r int) *Grid {
	g.rows = r
	return g
}

// ColWidths sets individual column widths
func (g *Grid) ColWidths(widths ...int) *Grid {
	g.colWidths = widths
	return g
}

// RowHeights sets individual row heights
func (g *Grid) RowHeights(heights ...int) *Grid {
	g.rowHeights = heights
	return g
}

// Gap sets the gap between all cells
func (g *Grid) Gap(gap int) *Grid {
	g.gap = gap
	g.rowGap = gap
	g.colGap = gap
	return g
}

// RowGap sets the gap between rows
func (g *Grid) RowGap(gap int) *Grid {
	g.rowGap = gap
	return g
}

// ColGap sets the gap between columns
func (g *Grid) ColGap(gap int) *Grid {
	g.colGap = gap
	return g
}

// Width sets the total container width
func (g *Grid) Width(w int) *Grid {
	g.width = w
	return g
}

// Height sets the total container height
func (g *Grid) Height(h int) *Grid {
	g.height = h
	return g
}

// Style sets the container style
func (g *Grid) Style(s lipgloss.Style) *Grid {
	g.style = s
	return g
}

// Item adds an item to the grid
func (g *Grid) Item(content string) *Grid {
	g.items = append(g.items, GridItem{
		Content: content,
		ColSpan: 1,
		RowSpan: 1,
	})
	return g
}

// ItemSpan adds an item with column span
func (g *Grid) ItemSpan(content string, colSpan int) *Grid {
	g.items = append(g.items, GridItem{
		Content: content,
		ColSpan: colSpan,
		RowSpan: 1,
	})
	return g
}

// ItemFull adds a full-width item
func (g *Grid) ItemFull(content string) *Grid {
	return g.ItemSpan(content, g.columns)
}

// ItemGrid adds a fully configured grid item
func (g *Grid) ItemGrid(item GridItem) *Grid {
	g.items = append(g.items, item)
	return g
}

// Items adds multiple items at once
func (g *Grid) Items(contents ...string) *Grid {
	for _, c := range contents {
		g.Item(c)
	}
	return g
}

// Render renders the grid
func (g *Grid) Render() string {
	if len(g.items) == 0 || g.columns == 0 {
		return ""
	}

	// Calculate column widths
	colWidths := g.calculateColumnWidths()

	// Build grid rows
	var rows []string
	currentRow := make([]string, 0, g.columns)
	currentColIndex := 0

	for _, item := range g.items {
		span := item.ColSpan
		if span <= 0 {
			span = 1
		}
		if span > g.columns {
			span = g.columns
		}

		// If item doesn't fit in current row, start new row
		if currentColIndex+span > g.columns {
			if len(currentRow) > 0 {
				rows = append(rows, g.renderRow(currentRow, colWidths))
			}
			currentRow = make([]string, 0, g.columns)
			currentColIndex = 0
		}

		// Calculate width for this cell
		cellWidth := 0
		for i := 0; i < span && currentColIndex+i < len(colWidths); i++ {
			cellWidth += colWidths[currentColIndex+i]
			if i > 0 {
				cellWidth += g.colGap
			}
		}

		// Render item with calculated width
		style := item.Style.Width(cellWidth)
		currentRow = append(currentRow, style.Render(item.Content))

		currentColIndex += span
	}

	// Add final row
	if len(currentRow) > 0 {
		rows = append(rows, g.renderRow(currentRow, colWidths))
	}

	// Join rows with row gap
	rowGapStr := strings.Repeat("\n", g.rowGap)
	if g.rowGap == 0 {
		rowGapStr = "\n"
	}

	result := strings.Join(rows, rowGapStr)

	if g.style.Value() != "" {
		return g.style.Render(result)
	}
	return result
}

func (g *Grid) calculateColumnWidths() []int {
	if len(g.colWidths) >= g.columns {
		return g.colWidths[:g.columns]
	}

	// Auto-calculate equal widths if container width is set
	if g.width > 0 {
		totalGaps := g.colGap * (g.columns - 1)
		availableWidth := g.width - totalGaps

		widths := make([]int, g.columns)

		// Fill with specified widths first
		usedWidth := 0
		autoCount := 0
		for i := 0; i < g.columns; i++ {
			if i < len(g.colWidths) && g.colWidths[i] > 0 {
				widths[i] = g.colWidths[i]
				usedWidth += g.colWidths[i]
			} else {
				autoCount++
			}
		}

		// Distribute remaining width to auto columns
		if autoCount > 0 {
			remainingWidth := availableWidth - usedWidth
			autoWidth := remainingWidth / autoCount
			for i := 0; i < g.columns; i++ {
				if widths[i] == 0 {
					widths[i] = autoWidth
				}
			}
		}

		return widths
	}

	// No width specified, use content-based widths
	widths := make([]int, g.columns)
	for i := range widths {
		if i < len(g.colWidths) {
			widths[i] = g.colWidths[i]
		} else {
			widths[i] = 20 // Default width
		}
	}
	return widths
}

func (g *Grid) renderRow(cells []string, colWidths []int) string {
	if len(cells) == 0 {
		return ""
	}

	// Normalize heights
	maxHeight := 0
	for _, cell := range cells {
		h := lipgloss.Height(cell)
		if h > maxHeight {
			maxHeight = h
		}
	}

	normalized := make([]string, len(cells))
	for i, cell := range cells {
		normalized[i] = lipgloss.PlaceVertical(maxHeight, lipgloss.Top, cell)
	}

	// Join with column gap
	gap := strings.Repeat(" ", g.colGap)
	return strings.Join(normalized, gap)
}

// SimpleGrid creates a simple grid with equal-width columns
func SimpleGrid(columns, width int, items ...string) string {
	return NewGrid(columns).Width(width).Items(items...).Render()
}

// TwoColumn creates a two-column grid
func TwoColumn(width int, left, right string) string {
	return NewGrid(2).Width(width).Items(left, right).Render()
}

// ThreeColumn creates a three-column grid
func ThreeColumn(width int, left, center, right string) string {
	return NewGrid(3).Width(width).Items(left, center, right).Render()
}

// SidebarLayout creates a sidebar + main content layout
func SidebarLayout(totalWidth, sidebarWidth int, sidebar, main string) string {
	mainWidth := totalWidth - sidebarWidth - 1 // 1 for gap
	return NewGrid(2).
		Width(totalWidth).
		ColWidths(sidebarWidth, mainWidth).
		ColGap(1).
		Items(sidebar, main).
		Render()
}
