// Package middleware provides command middleware for the bib CLI.
//
// Middleware allows wrapping command execution with cross-cutting concerns
// like logging, configuration loading, authentication, and output formatting.
package middleware

import (
	"context"
	"time"

	"github.com/spf13/cobra"
)

// RunFunc is the function signature for cobra command execution.
type RunFunc func(cmd *cobra.Command, args []string) error

// Middleware wraps a RunFunc with additional behavior.
type Middleware func(next RunFunc) RunFunc

// Chain combines multiple middleware into a single middleware.
// Middleware is applied in the order provided (first middleware wraps outermost).
func Chain(middlewares ...Middleware) Middleware {
	return func(final RunFunc) RunFunc {
		for i := len(middlewares) - 1; i >= 0; i-- {
			final = middlewares[i](final)
		}
		return final
	}
}

// Apply applies middleware to a cobra command's RunE function.
func Apply(cmd *cobra.Command, middlewares ...Middleware) {
	if cmd.RunE == nil {
		return
	}

	original := cmd.RunE
	chained := Chain(middlewares...)(original)
	cmd.RunE = chained
}

// ApplyRecursive applies middleware to a command and all its subcommands.
func ApplyRecursive(cmd *cobra.Command, middlewares ...Middleware) {
	Apply(cmd, middlewares...)
	for _, child := range cmd.Commands() {
		ApplyRecursive(child, middlewares...)
	}
}

// Context keys for middleware data
type contextKey string

const (
	// RequestIDKey is the context key for request ID
	RequestIDKey contextKey = "request_id"
	// StartTimeKey is the context key for command start time
	StartTimeKey contextKey = "start_time"
	// CommandKey is the context key for command name
	CommandKey contextKey = "command"
	// UserKey is the context key for current user
	UserKey contextKey = "user"
)

// CommandContext holds data passed through the middleware chain.
type CommandContext struct {
	ctx context.Context
}

// NewCommandContext creates a new command context.
func NewCommandContext() *CommandContext {
	return &CommandContext{
		ctx: context.Background(),
	}
}

// WithValue adds a value to the context.
func (c *CommandContext) WithValue(key contextKey, value any) *CommandContext {
	c.ctx = context.WithValue(c.ctx, key, value)
	return c
}

// Value retrieves a value from the context.
func (c *CommandContext) Value(key contextKey) any {
	return c.ctx.Value(key)
}

// RequestID returns the request ID from context.
func (c *CommandContext) RequestID() string {
	if v := c.ctx.Value(RequestIDKey); v != nil {
		return v.(string)
	}
	return ""
}

// StartTime returns the command start time from context.
func (c *CommandContext) StartTime() time.Time {
	if v := c.ctx.Value(StartTimeKey); v != nil {
		return v.(time.Time)
	}
	return time.Time{}
}

// Command returns the command name from context.
func (c *CommandContext) Command() string {
	if v := c.ctx.Value(CommandKey); v != nil {
		return v.(string)
	}
	return ""
}

// contextKeyCmd is the key for storing CommandContext in cobra.Command.Context()
var contextKeyCmd = &struct{}{}

// SetCommandContext stores the CommandContext in the cobra command.
func SetCommandContext(cmd *cobra.Command, cc *CommandContext) {
	cmd.SetContext(context.WithValue(cmd.Context(), contextKeyCmd, cc))
}

// GetCommandContext retrieves the CommandContext from the cobra command.
func GetCommandContext(cmd *cobra.Command) *CommandContext {
	if cmd.Context() == nil {
		return NewCommandContext()
	}
	if v := cmd.Context().Value(contextKeyCmd); v != nil {
		return v.(*CommandContext)
	}
	return NewCommandContext()
}
