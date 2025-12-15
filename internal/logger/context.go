package logger

import (
	"context"
	"log/slog"
	"os"
	"os/user"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	commandContextKey contextKey = "command_context"
	loggerContextKey  contextKey = "logger"
)

// CommandContext holds metadata about a command execution.
type CommandContext struct {
	Command    string    `json:"command"`
	Args       []string  `json:"args"`
	User       string    `json:"user"`
	Hostname   string    `json:"hostname"`
	WorkingDir string    `json:"working_dir"`
	Timestamp  time.Time `json:"timestamp"`
	RequestID  string    `json:"request_id"`
}

// NewCommandContext creates a new CommandContext from a Cobra command.
func NewCommandContext(cmd *cobra.Command, args []string) *CommandContext {
	cc := &CommandContext{
		Command:   cmd.CommandPath(),
		Args:      args,
		Timestamp: time.Now(),
		RequestID: generateRequestID(),
	}

	// Get current user
	if u, err := user.Current(); err == nil {
		cc.User = u.Username
	}

	// Get hostname
	if hostname, err := os.Hostname(); err == nil {
		cc.Hostname = hostname
	}

	// Get working directory
	if cwd, err := os.Getwd(); err == nil {
		cc.WorkingDir = cwd
	}

	return cc
}

// NewDaemonContext creates a CommandContext for daemon operations.
func NewDaemonContext(operation string) *CommandContext {
	cc := &CommandContext{
		Command:   operation,
		Args:      nil,
		Timestamp: time.Now(),
		RequestID: generateRequestID(),
	}

	if u, err := user.Current(); err == nil {
		cc.User = u.Username
	}

	if hostname, err := os.Hostname(); err == nil {
		cc.Hostname = hostname
	}

	if cwd, err := os.Getwd(); err == nil {
		cc.WorkingDir = cwd
	}

	return cc
}

// WithCommandContext stores a CommandContext in the context.
func WithCommandContext(ctx context.Context, cc *CommandContext) context.Context {
	return context.WithValue(ctx, commandContextKey, cc)
}

// CommandContextFrom retrieves the CommandContext from the context.
func CommandContextFrom(ctx context.Context) *CommandContext {
	if cc, ok := ctx.Value(commandContextKey).(*CommandContext); ok {
		return cc
	}
	return nil
}

// WithLogger stores a Logger in the context.
func WithLogger(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey, l)
}

// LoggerFrom retrieves the Logger from the context.
func LoggerFrom(ctx context.Context) *Logger {
	if l, ok := ctx.Value(loggerContextKey).(*Logger); ok {
		return l
	}
	return Default()
}

// LogAttrs returns the CommandContext as slog attributes.
func (cc *CommandContext) LogAttrs() []slog.Attr {
	if cc == nil {
		return nil
	}

	attrs := []slog.Attr{
		slog.String("request_id", cc.RequestID),
		slog.String("command", cc.Command),
		slog.String("user", cc.User),
		slog.String("hostname", cc.Hostname),
		slog.String("working_dir", cc.WorkingDir),
		slog.Time("timestamp", cc.Timestamp),
	}

	if len(cc.Args) > 0 {
		attrs = append(attrs, slog.Any("args", cc.Args))
	}

	return attrs
}

// LogGroup returns the CommandContext as a grouped slog attribute.
func (cc *CommandContext) LogGroup() slog.Attr {
	if cc == nil {
		return slog.Attr{}
	}

	args := make([]any, 0, len(cc.LogAttrs()))
	for _, attr := range cc.LogAttrs() {
		args = append(args, attr)
	}

	return slog.Group("context", args...)
}

// generateRequestID creates a unique request ID.
func generateRequestID() string {
	return uuid.New().String()
}
