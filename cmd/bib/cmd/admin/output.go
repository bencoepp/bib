package admin

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// outputFormat stores the current output format
var outputFormat = "table"

// SetOutputFormat sets the output format (called from root command)
func SetOutputFormat(format string) {
	outputFormat = format
}

// OutputWriter handles formatted output
type OutputWriter struct {
	format string
	out    io.Writer
}

// NewOutputWriter creates a new output writer
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
		return nil
	default:
		return o.writeText(data)
	}
}

func (o *OutputWriter) writeJSON(data any) error {
	output, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Fprintln(o.out, string(output))
	return nil
}

func (o *OutputWriter) writeYAML(data any) error {
	output, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}
	fmt.Fprint(o.out, string(output))
	return nil
}

func (o *OutputWriter) writeText(data any) error {
	fmt.Fprintln(o.out, data)
	return nil
}

// Printf prints formatted output
func (o *OutputWriter) Printf(format string, args ...any) {
	fmt.Fprintf(o.out, format, args...)
}

// Println prints a line
func (o *OutputWriter) Println(args ...any) {
	fmt.Fprintln(o.out, args...)
}

// WriteSuccess prints a success message
func (o *OutputWriter) WriteSuccess(msg string) {
	fmt.Fprintln(o.out, "✓", msg)
}

// WriteError prints an error message
func (o *OutputWriter) WriteError(msg string) {
	fmt.Fprintln(o.out, "✗", msg)
}

// WriteWarning prints a warning message
func (o *OutputWriter) WriteWarning(msg string) {
	fmt.Fprintln(o.out, "⚠", msg)
}

// WriteInfo prints an info message
func (o *OutputWriter) WriteInfo(msg string) {
	fmt.Fprintln(o.out, "ℹ", msg)
}
