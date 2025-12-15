package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	charmlog "github.com/charmbracelet/log"
)

// Lipgloss styles for custom elements
var (
	// Prefix style (for app name)
	PrefixStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212")) // Pink
)

// CharmHandler wraps charmbracelet/log to implement slog.Handler.
type CharmHandler struct {
	logger *charmlog.Logger
	writer io.Writer
	opts   CharmHandlerOptions
	attrs  []slog.Attr
	groups []string
}

// CharmHandlerOptions configures the Charm handler.
type CharmHandlerOptions struct {
	// Level is the minimum level to log.
	Level slog.Leveler
	// NoColor disables colored output.
	NoColor bool
	// TimeFormat is the format for timestamps.
	TimeFormat string
	// ShowCaller shows file:func:line in logs.
	ShowCaller bool
	// Prefix is prepended to all log messages.
	Prefix string
}

// applyCharmStyles applies custom styles to a charm logger.
func applyCharmStyles(logger *charmlog.Logger) {
	styles := charmlog.DefaultStyles()

	// Level styles with emojis
	styles.Levels[charmlog.DebugLevel] = lipgloss.NewStyle().
		SetString("🔍 DEBUG").
		Bold(true).
		Foreground(lipgloss.Color("63")) // Purple

	styles.Levels[charmlog.InfoLevel] = lipgloss.NewStyle().
		SetString("✨ INFO ").
		Bold(true).
		Foreground(lipgloss.Color("42")) // Green

	styles.Levels[charmlog.WarnLevel] = lipgloss.NewStyle().
		SetString("⚠️  WARN ").
		Bold(true).
		Foreground(lipgloss.Color("214")) // Orange

	styles.Levels[charmlog.ErrorLevel] = lipgloss.NewStyle().
		SetString("❌ ERROR").
		Bold(true).
		Foreground(lipgloss.Color("196")) // Red

	styles.Levels[charmlog.FatalLevel] = lipgloss.NewStyle().
		SetString("💀 FATAL").
		Bold(true).
		Background(lipgloss.Color("196")).
		Foreground(lipgloss.Color("231")) // White on red

	// Key style
	styles.Key = lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")). // Cyan
		Bold(true)

	// Value style
	styles.Value = lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")) // Light gray

	// Separator style
	styles.Separator = lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")) // Dark gray

	// Timestamp style
	styles.Timestamp = lipgloss.NewStyle().
		Foreground(lipgloss.Color("243")) // Medium gray

	// Caller style
	styles.Caller = lipgloss.NewStyle().
		Foreground(lipgloss.Color("139")) // Light purple

	// Message style
	styles.Message = lipgloss.NewStyle().
		Foreground(lipgloss.Color("255")) // White

	// Prefix style
	styles.Prefix = PrefixStyle

	logger.SetStyles(styles)
}

// NewCharmHandler creates a new Charm-based slog handler.
func NewCharmHandler(w io.Writer, opts *CharmHandlerOptions) *CharmHandler {
	if opts == nil {
		opts = &CharmHandlerOptions{}
	}
	if opts.Level == nil {
		opts.Level = slog.LevelInfo
	}
	if opts.TimeFormat == "" {
		opts.TimeFormat = "15:04:05"
	}

	// Create charm logger
	logger := charmlog.NewWithOptions(w, charmlog.Options{
		ReportCaller:    opts.ShowCaller,
		ReportTimestamp: true,
		TimeFormat:      opts.TimeFormat,
		Prefix:          opts.Prefix,
		Level:           charmLogLevel(opts.Level.Level()),
	})

	// Apply custom styles
	applyCharmStyles(logger)

	return &CharmHandler{
		logger: logger,
		writer: w,
		opts:   *opts,
	}
}

// Enabled implements slog.Handler.
func (h *CharmHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

// Handle implements slog.Handler.
func (h *CharmHandler) Handle(ctx context.Context, r slog.Record) error {
	// Collect all key-value pairs
	kvs := make([]interface{}, 0, (len(h.attrs)+r.NumAttrs())*2)

	// Add handler-level attrs first
	for _, attr := range h.attrs {
		k, v := h.formatAttr(attr)
		if k != "" {
			kvs = append(kvs, k, v)
		}
	}

	// Add record attrs
	r.Attrs(func(a slog.Attr) bool {
		k, v := h.formatAttr(a)
		if k != "" {
			kvs = append(kvs, k, v)
		}
		return true
	})

	// Log at appropriate level
	switch {
	case r.Level >= slog.LevelError:
		h.logger.Error(r.Message, kvs...)
	case r.Level >= slog.LevelWarn:
		h.logger.Warn(r.Message, kvs...)
	case r.Level >= slog.LevelInfo:
		h.logger.Info(r.Message, kvs...)
	default:
		h.logger.Debug(r.Message, kvs...)
	}

	return nil
}

// formatAttr formats a slog.Attr for charm log.
func (h *CharmHandler) formatAttr(attr slog.Attr) (string, interface{}) {
	if attr.Key == "" {
		return "", nil
	}

	key := attr.Key
	if len(h.groups) > 0 {
		key = strings.Join(h.groups, ".") + "." + key
	}

	// Handle groups - flatten them with dot notation
	if attr.Value.Kind() == slog.KindGroup {
		groupAttrs := attr.Value.Group()
		if len(groupAttrs) == 0 {
			return "", nil
		}
		var parts []string
		for _, ga := range groupAttrs {
			k, v := h.formatAttr(ga)
			if k != "" {
				parts = append(parts, fmt.Sprintf("%s=%v", k, v))
			}
		}
		return key, strings.Join(parts, " ")
	}

	return key, formatSlogValue(attr.Value)
}

// WithAttrs implements slog.Handler.
func (h *CharmHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandler := h.clone()
	newHandler.attrs = append(newHandler.attrs, attrs...)
	return newHandler
}

// WithGroup implements slog.Handler.
func (h *CharmHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	newHandler := h.clone()
	newHandler.groups = append(newHandler.groups, name)
	return newHandler
}

// clone creates a copy of the handler.
func (h *CharmHandler) clone() *CharmHandler {
	newLogger := charmlog.NewWithOptions(h.writer, charmlog.Options{
		ReportCaller:    h.opts.ShowCaller,
		ReportTimestamp: true,
		TimeFormat:      h.opts.TimeFormat,
		Prefix:          h.opts.Prefix,
		Level:           charmLogLevel(h.opts.Level.Level()),
	})
	applyCharmStyles(newLogger)

	return &CharmHandler{
		logger: newLogger,
		writer: h.writer,
		opts:   h.opts,
		attrs:  append([]slog.Attr{}, h.attrs...),
		groups: append([]string{}, h.groups...),
	}
}

// formatSlogValue converts slog.Value to a display value.
func formatSlogValue(v slog.Value) interface{} {
	switch v.Kind() {
	case slog.KindTime:
		return v.Time().Format(time.RFC3339)
	case slog.KindDuration:
		return v.Duration().String()
	case slog.KindAny:
		val := v.Any()
		if err, ok := val.(error); ok {
			return err.Error()
		}
		return val
	case slog.KindGroup:
		return "[group]"
	default:
		return v.Any()
	}
}

// charmLogLevel converts slog.Level to charmlog.Level.
func charmLogLevel(level slog.Level) charmlog.Level {
	switch {
	case level >= slog.LevelError:
		return charmlog.ErrorLevel
	case level >= slog.LevelWarn:
		return charmlog.WarnLevel
	case level >= slog.LevelInfo:
		return charmlog.InfoLevel
	default:
		return charmlog.DebugLevel
	}
}

// isTerminal checks if the writer is a terminal.
func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		stat, err := f.Stat()
		if err != nil {
			return false
		}
		return (stat.Mode() & os.ModeCharDevice) != 0
	}
	return false
}

// ConsoleHandlerOptions is an alias for CharmHandlerOptions for backward compatibility.
type ConsoleHandlerOptions = CharmHandlerOptions

// NewConsoleHandler creates a new Charm-based console handler.
func NewConsoleHandler(w io.Writer, opts *ConsoleHandlerOptions) slog.Handler {
	if opts == nil {
		opts = &ConsoleHandlerOptions{}
	}
	return NewCharmHandler(w, &CharmHandlerOptions{
		Level:      opts.Level,
		NoColor:    opts.NoColor,
		TimeFormat: opts.TimeFormat,
		ShowCaller: true,
		Prefix:     "",
	})
}

// getFrame retrieves the runtime.Frame for a given PC.
func getFrame(pc uintptr) runtime.Frame {
	frames := runtime.CallersFrames([]uintptr{pc})
	frame, _ := frames.Next()
	return frame
}

// shortFuncName extracts just the function name from a fully qualified name.
func shortFuncName(name string) string {
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	if idx := strings.Index(name, "."); idx >= 0 {
		name = name[idx+1:]
	}
	return name
}
