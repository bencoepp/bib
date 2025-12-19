// Package output provides structured output formatting for the bib CLI.
//
// Output supports multiple formats:
//   - table: Human-readable tables (default)
//   - json: Machine-readable JSON
//   - yaml: Machine-readable YAML
//   - quiet: Minimal output (IDs only)
//
// The package is designed to work with the middleware chain for
// consistent output handling across all commands.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

// Format represents an output format.
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
	FormatQuiet Format = "quiet"
)

// ParseFormat parses a format string.
func ParseFormat(s string) Format {
	switch strings.ToLower(s) {
	case "json":
		return FormatJSON
	case "yaml", "yml":
		return FormatYAML
	case "quiet", "q":
		return FormatQuiet
	default:
		return FormatTable
	}
}

// Writer handles formatted output based on the configured format.
type Writer struct {
	format Format
	out    io.Writer
	err    io.Writer
}

// NewWriter creates a new output writer with the specified format.
func NewWriter(format Format) *Writer {
	return &Writer{
		format: format,
		out:    os.Stdout,
		err:    os.Stderr,
	}
}

// WithOutput sets the output writer.
func (w *Writer) WithOutput(out io.Writer) *Writer {
	w.out = out
	return w
}

// WithError sets the error writer.
func (w *Writer) WithError(err io.Writer) *Writer {
	w.err = err
	return w
}

// Format returns the current format.
func (w *Writer) Format() Format {
	return w.format
}

// Write outputs data according to the configured format.
func (w *Writer) Write(data any) error {
	switch w.format {
	case FormatJSON:
		return w.writeJSON(data)
	case FormatYAML:
		return w.writeYAML(data)
	case FormatQuiet:
		return w.writeQuiet(data)
	default:
		return w.writeTable(data)
	}
}

// writeJSON outputs data as pretty-printed JSON.
func (w *Writer) writeJSON(data any) error {
	encoder := json.NewEncoder(w.out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// writeYAML outputs data as YAML.
func (w *Writer) writeYAML(data any) error {
	return yaml.NewEncoder(w.out).Encode(data)
}

// writeQuiet outputs minimal data (IDs only).
func (w *Writer) writeQuiet(data any) error {
	switch v := data.(type) {
	case string:
		fmt.Fprintln(w.out, v)
	case []string:
		for _, s := range v {
			fmt.Fprintln(w.out, s)
		}
	case Identifiable:
		fmt.Fprintln(w.out, v.ID())
	case []Identifiable:
		for _, item := range v {
			fmt.Fprintln(w.out, item.ID())
		}
	case map[string]any:
		if id, ok := v["id"]; ok {
			fmt.Fprintln(w.out, id)
		}
	case []map[string]any:
		for _, m := range v {
			if id, ok := m["id"]; ok {
				fmt.Fprintln(w.out, id)
			}
		}
	default:
		return w.writeJSON(data)
	}
	return nil
}

// writeTable outputs data as a formatted table.
func (w *Writer) writeTable(data any) error {
	switch v := data.(type) {
	case Tabular:
		return w.renderTable(v.TableData())
	case *Table:
		return w.renderTable(v)
	case string:
		fmt.Fprintln(w.out, v)
	default:
		return w.writeJSON(data)
	}
	return nil
}

// renderTable renders a table to the output.
func (w *Writer) renderTable(t *Table) error {
	if t == nil || len(t.Headers) == 0 {
		return nil
	}

	tw := tabwriter.NewWriter(w.out, 0, 0, 2, ' ', 0)

	// Headers
	for i, h := range t.Headers {
		if i > 0 {
			fmt.Fprint(tw, "\t")
		}
		fmt.Fprint(tw, strings.ToUpper(h))
	}
	fmt.Fprintln(tw)

	// Rows
	for _, row := range t.Rows {
		for i, cell := range row {
			if i > 0 {
				fmt.Fprint(tw, "\t")
			}
			fmt.Fprint(tw, cell)
		}
		fmt.Fprintln(tw)
	}

	return tw.Flush()
}

// Println writes a line to output.
func (w *Writer) Println(a ...any) {
	fmt.Fprintln(w.out, a...)
}

// Printf writes formatted output.
func (w *Writer) Printf(format string, a ...any) {
	fmt.Fprintf(w.out, format, a...)
}

// Errorf writes an error message.
func (w *Writer) Errorf(format string, a ...any) {
	fmt.Fprintf(w.err, format, a...)
}

// Success writes a success message with icon.
func (w *Writer) Success(message string) {
	fmt.Fprintf(w.out, "✓ %s\n", message)
}

// Warn writes a warning message with icon.
func (w *Writer) Warn(message string) {
	fmt.Fprintf(w.err, "⚠ %s\n", message)
}

// Info writes an info message with icon.
func (w *Writer) Info(message string) {
	fmt.Fprintf(w.out, "ℹ %s\n", message)
}

// Identifiable is an interface for objects with an ID.
type Identifiable interface {
	ID() string
}

// Tabular is an interface for objects that can be rendered as a table.
type Tabular interface {
	TableData() *Table
}

// Table represents tabular data.
type Table struct {
	Headers []string
	Rows    [][]string
}

// NewTable creates a new table with headers.
func NewTable(headers ...string) *Table {
	return &Table{
		Headers: headers,
		Rows:    make([][]string, 0),
	}
}

// AddRow adds a row to the table.
func (t *Table) AddRow(cells ...string) *Table {
	t.Rows = append(t.Rows, cells)
	return t
}

// AddRowFromMap adds a row using a map (matches headers by key).
func (t *Table) AddRowFromMap(data map[string]string) *Table {
	row := make([]string, len(t.Headers))
	for i, h := range t.Headers {
		if v, ok := data[h]; ok {
			row[i] = v
		} else if v, ok := data[strings.ToLower(h)]; ok {
			row[i] = v
		}
	}
	t.Rows = append(t.Rows, row)
	return t
}

// TableData implements Tabular for Table.
func (t *Table) TableData() *Table {
	return t
}
