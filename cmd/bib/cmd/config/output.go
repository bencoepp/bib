package configcmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

// outputFormat stores the current output format (set by root command)
var outputFormat = "table"

// SetOutputFormat sets the output format (called from root command)
func SetOutputFormat(format string) {
	outputFormat = format
}

// OutputWriter handles formatted output based on the global output format flag
type OutputWriter struct {
	format string
	out    io.Writer
}

// NewOutputWriter creates a new output writer with the current format
func NewOutputWriter() *OutputWriter {
	return &OutputWriter{
		format: outputFormat,
		out:    os.Stdout,
	}
}

// Write outputs data according to the configured format
func (o *OutputWriter) Write(data any) error {
	switch o.format {
	case "json":
		return o.writeJSON(data)
	case "yaml":
		return o.writeYAML(data)
	case "quiet":
		return o.writeQuiet(data)
	case "table":
		return o.writeTable(data)
	default:
		return o.writeTable(data)
	}
}

// writeJSON outputs data as JSON
func (o *OutputWriter) writeJSON(data any) error {
	output, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Fprintln(o.out, string(output))
	return nil
}

// writeYAML outputs data as YAML
func (o *OutputWriter) writeYAML(data any) error {
	output, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}
	fmt.Fprint(o.out, string(output))
	return nil
}

// writeQuiet outputs minimal data (IDs only for slices, single value for scalars)
func (o *OutputWriter) writeQuiet(data any) error {
	switch v := data.(type) {
	case string:
		fmt.Fprintln(o.out, v)
	case []string:
		for _, s := range v {
			fmt.Fprintln(o.out, s)
		}
	case map[string]any:
		// Try to output just ID if present
		if id, ok := v["id"]; ok {
			fmt.Fprintln(o.out, id)
		}
	default:
		// Fall back to JSON for complex types
		return o.writeJSON(data)
	}
	return nil
}

// writeTable outputs data as a formatted table
func (o *OutputWriter) writeTable(data any) error {
	switch v := data.(type) {
	case TableData:
		return o.renderTable(v)
	case string:
		fmt.Fprintln(o.out, v)
	default:
		// Fall back to JSON for non-table types
		return o.writeJSON(data)
	}
	return nil
}

// TableData represents data that can be rendered as a table
type TableData struct {
	Headers []string
	Rows    [][]string
}

// renderTable renders TableData as a formatted table
func (o *OutputWriter) renderTable(data TableData) error {
	w := tabwriter.NewWriter(o.out, 0, 0, 2, ' ', 0)

	// Print headers
	if len(data.Headers) > 0 {
		fmt.Fprintln(w, strings.Join(data.Headers, "\t"))
		// Print separator
		sep := make([]string, len(data.Headers))
		for i, h := range data.Headers {
			sep[i] = strings.Repeat("-", len(h))
		}
		fmt.Fprintln(w, strings.Join(sep, "\t"))
	}

	// Print rows
	for _, row := range data.Rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}

	return w.Flush()
}

// WriteSuccess writes a success message (only in non-quiet mode)
func (o *OutputWriter) WriteSuccess(msg string) {
	if o.format != "quiet" {
		fmt.Fprintln(o.out, msg)
	}
}

// WriteError writes an error message
func (o *OutputWriter) WriteError(msg string) {
	fmt.Fprintln(os.Stderr, msg)
}
