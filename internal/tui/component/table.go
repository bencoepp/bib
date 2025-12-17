package component

import (
	"fmt"
	"strings"

	"bib/internal/tui/themes"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TableColumn defines a table column
type TableColumn struct {
	Title    string
	Width    int
	MinWidth int
	MaxWidth int
	Flex     int // Flex grow factor (0 = fixed width)
	Align    lipgloss.Position
}

// TableRow represents a row of data
type TableRow struct {
	ID    string
	Cells []string
	Data  interface{} // Optional attached data
}

// Table is an interactive data table
type Table struct {
	BaseComponent
	FocusState
	ScrollState

	columns       []TableColumn
	rows          []TableRow
	selectedIndex int
	multiSelect   bool
	selectedRows  map[int]bool
	width         int
	height        int
	showHeader    bool
	showBorder    bool
	striped       bool
	highlightRow  bool
}

// NewTable creates a new table
func NewTable() *Table {
	return &Table{
		BaseComponent: NewBaseComponent(),
		columns:       make([]TableColumn, 0),
		rows:          make([]TableRow, 0),
		selectedRows:  make(map[int]bool),
		showHeader:    true,
		showBorder:    false,
		striped:       true,
		highlightRow:  true,
	}
}

// WithColumns sets the table columns
func (t *Table) WithColumns(columns ...TableColumn) *Table {
	t.columns = columns
	return t
}

// WithRows sets the table rows
func (t *Table) WithRows(rows ...TableRow) *Table {
	t.rows = rows
	t.ScrollState.SetMaxOffset(max(0, len(rows)-t.visibleRows()))
	return t
}

// AddRow adds a row to the table
func (t *Table) AddRow(row TableRow) *Table {
	t.rows = append(t.rows, row)
	t.ScrollState.SetMaxOffset(max(0, len(t.rows)-t.visibleRows()))
	return t
}

// WithSize sets the table dimensions
func (t *Table) WithSize(width, height int) *Table {
	t.width = width
	t.height = height
	t.ScrollState.SetMaxOffset(max(0, len(t.rows)-t.visibleRows()))
	return t
}

// WithHeader enables/disables the header
func (t *Table) WithHeader(show bool) *Table {
	t.showHeader = show
	return t
}

// WithBorder enables/disables borders
func (t *Table) WithBorder(show bool) *Table {
	t.showBorder = show
	return t
}

// WithStriped enables/disables striped rows
func (t *Table) WithStriped(striped bool) *Table {
	t.striped = striped
	return t
}

// WithMultiSelect enables/disables multi-selection
func (t *Table) WithMultiSelect(multi bool) *Table {
	t.multiSelect = multi
	return t
}

// WithTheme sets the theme
func (t *Table) WithTheme(theme *themes.Theme) *Table {
	t.SetTheme(theme)
	return t
}

func (t *Table) visibleRows() int {
	if t.height <= 0 {
		return len(t.rows)
	}
	visible := t.height
	if t.showHeader {
		visible -= 2 // Header + separator
	}
	return max(1, visible)
}

// Init implements tea.Model
func (t *Table) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (t *Table) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !t.Focused() {
		return t, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if t.selectedIndex > 0 {
				t.selectedIndex--
				// Scroll if needed
				if t.selectedIndex < t.Offset() {
					t.SetOffset(t.selectedIndex)
				}
			}

		case "down", "j":
			if t.selectedIndex < len(t.rows)-1 {
				t.selectedIndex++
				// Scroll if needed
				visibleEnd := t.Offset() + t.visibleRows()
				if t.selectedIndex >= visibleEnd {
					t.SetOffset(t.selectedIndex - t.visibleRows() + 1)
				}
			}

		case "pgup":
			t.selectedIndex = max(0, t.selectedIndex-t.visibleRows())
			t.SetOffset(max(0, t.Offset()-t.visibleRows()))

		case "pgdown":
			t.selectedIndex = min(len(t.rows)-1, t.selectedIndex+t.visibleRows())
			t.SetOffset(min(t.MaxOffset(), t.Offset()+t.visibleRows()))

		case "home", "g":
			t.selectedIndex = 0
			t.SetOffset(0)

		case "end", "G":
			t.selectedIndex = len(t.rows) - 1
			t.SetOffset(t.MaxOffset())

		case " ":
			if t.multiSelect {
				if t.selectedRows[t.selectedIndex] {
					delete(t.selectedRows, t.selectedIndex)
				} else {
					t.selectedRows[t.selectedIndex] = true
				}
			}

		case "a":
			if t.multiSelect {
				// Select all
				for i := range t.rows {
					t.selectedRows[i] = true
				}
			}

		case "A":
			if t.multiSelect {
				// Deselect all
				t.selectedRows = make(map[int]bool)
			}
		}

	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height
		t.ScrollState.SetMaxOffset(max(0, len(t.rows)-t.visibleRows()))
	}

	return t, nil
}

// View implements tea.Model
func (t *Table) View() string {
	return t.ViewWidth(t.width)
}

// ViewWidth renders the table at a specific width (implements Component)
func (t *Table) ViewWidth(width int) string {
	if width == 0 {
		width = t.width
	}
	if width == 0 {
		width = 80
	}

	theme := t.Theme()
	var b strings.Builder

	// Calculate column widths
	colWidths := t.calculateColumnWidths(width)

	// Render header
	if t.showHeader {
		var headerCells []string
		for i, col := range t.columns {
			cellWidth := colWidths[i]
			cell := t.formatCell(col.Title, cellWidth, col.Align)
			headerCells = append(headerCells, theme.TableHeader.Render(cell))
		}
		b.WriteString(strings.Join(headerCells, theme.TableBorder.Render("│")))
		b.WriteString("\n")

		// Separator
		var sepParts []string
		for _, w := range colWidths {
			sepParts = append(sepParts, strings.Repeat("─", w))
		}
		b.WriteString(theme.TableBorder.Render(strings.Join(sepParts, "┼")))
		b.WriteString("\n")
	}

	// Render rows
	startRow := t.Offset()
	endRow := min(startRow+t.visibleRows(), len(t.rows))

	for i := startRow; i < endRow; i++ {
		row := t.rows[i]
		var rowCells []string

		for j, cell := range row.Cells {
			if j >= len(t.columns) {
				break
			}
			col := t.columns[j]
			cellWidth := colWidths[j]
			formattedCell := t.formatCell(cell, cellWidth, col.Align)

			// Apply row style
			var cellStyle lipgloss.Style
			if i == t.selectedIndex && t.highlightRow {
				cellStyle = theme.TableRowSelected
			} else if t.striped && i%2 == 1 {
				cellStyle = theme.TableRowAlt
			} else {
				cellStyle = theme.TableRow
			}

			// Multi-select indicator
			if t.multiSelect && t.selectedRows[i] {
				formattedCell = themes.IconCheck + " " + formattedCell[2:]
			}

			rowCells = append(rowCells, cellStyle.Render(formattedCell))
		}

		b.WriteString(strings.Join(rowCells, theme.TableBorder.Render("│")))
		if i < endRow-1 {
			b.WriteString("\n")
		}
	}

	// Scroll indicator
	if len(t.rows) > t.visibleRows() {
		b.WriteString("\n")
		scrollInfo := fmt.Sprintf("(%d-%d of %d)", startRow+1, endRow, len(t.rows))
		b.WriteString(theme.Blurred.Render(scrollInfo))
	}

	return b.String()
}

func (t *Table) calculateColumnWidths(totalWidth int) []int {
	widths := make([]int, len(t.columns))

	// Subtract space for separators
	availableWidth := totalWidth - len(t.columns) + 1

	// First pass: fixed widths and minimums
	flexTotal := 0
	usedWidth := 0
	for i, col := range t.columns {
		if col.Width > 0 {
			widths[i] = col.Width
			usedWidth += col.Width
		} else if col.MinWidth > 0 {
			widths[i] = col.MinWidth
			usedWidth += col.MinWidth
		} else {
			widths[i] = 10 // Default minimum
			usedWidth += 10
		}
		flexTotal += col.Flex
	}

	// Second pass: distribute remaining space to flex columns
	remainingWidth := availableWidth - usedWidth
	if remainingWidth > 0 && flexTotal > 0 {
		for i, col := range t.columns {
			if col.Flex > 0 {
				extraWidth := (remainingWidth * col.Flex) / flexTotal
				widths[i] += extraWidth

				// Apply max width
				if col.MaxWidth > 0 && widths[i] > col.MaxWidth {
					widths[i] = col.MaxWidth
				}
			}
		}
	}

	return widths
}

func (t *Table) formatCell(content string, width int, align lipgloss.Position) string {
	// Truncate if needed
	if len(content) > width {
		content = Truncate(content, width)
	}

	// Pad to width
	switch align {
	case lipgloss.Right:
		return PadLeft(content, width)
	case lipgloss.Center:
		return PadCenter(content, width)
	default:
		return PadRight(content, width)
	}
}

// SelectedIndex returns the currently selected row index
func (t *Table) SelectedIndex() int {
	return t.selectedIndex
}

// SetSelectedIndex sets the selected row
func (t *Table) SetSelectedIndex(index int) {
	t.selectedIndex = clamp(index, 0, len(t.rows)-1)
}

// SelectedRow returns the currently selected row
func (t *Table) SelectedRow() *TableRow {
	if t.selectedIndex >= 0 && t.selectedIndex < len(t.rows) {
		return &t.rows[t.selectedIndex]
	}
	return nil
}

// SelectedRows returns all selected rows (for multi-select)
func (t *Table) SelectedRows() []TableRow {
	var selected []TableRow
	for i, row := range t.rows {
		if t.selectedRows[i] {
			selected = append(selected, row)
		}
	}
	return selected
}

// SelectedValue returns the data of the selected row
func (t *Table) SelectedValue() interface{} {
	if row := t.SelectedRow(); row != nil {
		return row.Data
	}
	return nil
}

// ClearSelection clears all selections
func (t *Table) ClearSelection() {
	t.selectedRows = make(map[int]bool)
}

// Focus implements FocusableComponent
func (t *Table) Focus() tea.Cmd {
	t.FocusState.Focus()
	return nil
}

// RowCount returns the number of rows
func (t *Table) RowCount() int {
	return len(t.rows)
}
