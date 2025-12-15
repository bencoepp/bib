// Package logger provides structured logging for bib and bibd using log/slog.
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"bib/internal/config"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger wraps slog.Logger with additional functionality.
type Logger struct {
	*slog.Logger
	cfg    config.LogConfig
	closer io.Closer
}

// New creates a new Logger from the given configuration.
func New(cfg config.LogConfig) (*Logger, error) {
	// Build writers
	writers, closer, err := buildWriters(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to build log writers: %w", err)
	}

	// Parse log level
	level, err := parseLevel(cfg.Level)
	if err != nil {
		if closer != nil {
			closer.Close()
		}
		return nil, fmt.Errorf("invalid log level: %w", err)
	}

	// Build handler options
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: cfg.EnableCaller,
	}

	// Create base handler based on format
	var handler slog.Handler
	switch strings.ToLower(cfg.Format) {
	case "json":
		handler = slog.NewJSONHandler(writers, opts)
	case "pretty":
		handler = NewConsoleHandler(writers, &ConsoleHandlerOptions{
			Level:   level,
			NoColor: cfg.NoColor,
		})
	default: // "text" or anything else
		handler = slog.NewTextHandler(writers, opts)
	}

	// Wrap with redacting handler if redact fields are configured
	if len(cfg.RedactFields) > 0 {
		handler = NewRedactingHandler(handler, cfg.RedactFields)
	}

	return &Logger{
		Logger: slog.New(handler),
		cfg:    cfg,
		closer: closer,
	}, nil
}

// Close closes any open file handles.
func (l *Logger) Close() error {
	if l.closer != nil {
		return l.closer.Close()
	}
	return nil
}

// With returns a new Logger with the given attributes.
func (l *Logger) With(attrs ...any) *Logger {
	return &Logger{
		Logger: l.Logger.With(attrs...),
		cfg:    l.cfg,
		closer: nil, // Don't transfer ownership of closer
	}
}

// WithGroup returns a new Logger with the given group name.
func (l *Logger) WithGroup(name string) *Logger {
	return &Logger{
		Logger: l.Logger.WithGroup(name),
		cfg:    l.cfg,
		closer: nil,
	}
}

// buildWriters creates the appropriate io.Writer based on configuration.
func buildWriters(cfg config.LogConfig) (io.Writer, io.Closer, error) {
	var writers []io.Writer
	var closers []io.Closer

	// Add console output
	switch strings.ToLower(cfg.Output) {
	case "stdout":
		writers = append(writers, os.Stdout)
	case "stderr":
		writers = append(writers, os.Stderr)
	case "":
		// No console output
	default:
		// Treat as file path for backward compatibility
		lj := newLumberjack(cfg.Output, cfg)
		writers = append(writers, lj)
		closers = append(closers, lj)
	}

	// Add file output if configured
	if cfg.FilePath != "" {
		lj := newLumberjack(cfg.FilePath, cfg)
		writers = append(writers, lj)
		closers = append(closers, lj)
	}

	// If no writers configured, default to stderr
	if len(writers) == 0 {
		writers = append(writers, os.Stderr)
	}

	var writer io.Writer
	if len(writers) == 1 {
		writer = writers[0]
	} else {
		writer = io.MultiWriter(writers...)
	}

	// Create a combined closer
	var closer io.Closer
	if len(closers) > 0 {
		closer = &multiCloser{closers: closers}
	}

	return writer, closer, nil
}

// newLumberjack creates a new lumberjack logger for file rotation.
func newLumberjack(path string, cfg config.LogConfig) *lumberjack.Logger {
	maxSize := cfg.MaxSizeMB
	if maxSize <= 0 {
		maxSize = 100
	}
	maxBackups := cfg.MaxBackups
	if maxBackups <= 0 {
		maxBackups = 3
	}
	maxAge := cfg.MaxAgeDays
	if maxAge <= 0 {
		maxAge = 28
	}

	return &lumberjack.Logger{
		Filename:   path,
		MaxSize:    maxSize,
		MaxBackups: maxBackups,
		MaxAge:     maxAge,
		Compress:   true,
	}
}

// parseLevel converts a string log level to slog.Level.
func parseLevel(level string) (slog.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unknown log level: %s", level)
	}
}

// multiCloser implements io.Closer for multiple closers.
type multiCloser struct {
	closers []io.Closer
}

func (mc *multiCloser) Close() error {
	var errs []error
	for _, c := range mc.closers {
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to close %d writers: %v", len(errs), errs)
	}
	return nil
}

// Default returns a default logger that writes to stderr.
func Default() *Logger {
	return &Logger{
		Logger: slog.Default(),
		cfg:    config.LogConfig{},
	}
}
