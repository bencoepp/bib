package middleware

import (
	"fmt"
	"os"
	"time"

	"bib/internal/config"
	"bib/internal/logger"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// LoggingOptions configures the logging middleware.
type LoggingOptions struct {
	// Logger is the logger instance (if nil, creates a new one)
	Logger *logger.Logger
	// AuditLogger is the audit logger (optional)
	AuditLogger *logger.AuditLogger
	// SkipCommands are commands that should not be logged
	SkipCommands []string
}

// Logging creates a middleware that logs command execution.
func Logging(opts LoggingOptions) Middleware {
	return func(next RunFunc) RunFunc {
		return func(cmd *cobra.Command, args []string) error {
			// Check if command should be skipped
			for _, skip := range opts.SkipCommands {
				if cmd.Name() == skip {
					return next(cmd, args)
				}
			}

			// Generate request ID
			requestID := uuid.New().String()[:8]

			// Get or create command context
			cc := GetCommandContext(cmd)
			cc = cc.WithValue(RequestIDKey, requestID)
			cc = cc.WithValue(StartTimeKey, time.Now())
			cc = cc.WithValue(CommandKey, cmd.CommandPath())

			// Get current user
			user := os.Getenv("USER")
			if user == "" {
				user = "unknown"
			}
			cc = cc.WithValue(UserKey, user)

			SetCommandContext(cmd, cc)

			// Get logger
			log := opts.Logger
			if log == nil {
				// Try to get from command context or create default
				var err error
				log, err = logger.New(config.LogConfig{
					Level:  "info",
					Format: "pretty",
				})
				if err != nil {
					// Fall back to no logging
					return next(cmd, args)
				}
				defer log.Close()
			}

			// Log command start
			log.Debug("command started",
				"command", cmd.CommandPath(),
				"args", args,
				"request_id", requestID,
				"user", user,
			)

			startTime := time.Now()

			// Execute command
			err := next(cmd, args)

			duration := time.Since(startTime)

			// Log command completion
			if err != nil {
				log.Error("command failed",
					"command", cmd.CommandPath(),
					"duration_ms", duration.Milliseconds(),
					"request_id", requestID,
					"error", err.Error(),
				)
			} else {
				log.Debug("command completed",
					"command", cmd.CommandPath(),
					"duration_ms", duration.Milliseconds(),
					"request_id", requestID,
				)
			}

			// Audit logging
			if opts.AuditLogger != nil {
				outcome := logger.AuditOutcomeSuccess
				if err != nil {
					outcome = logger.AuditOutcomeFailure
				}
				opts.AuditLogger.LogCommand(cmd.Context(), cmd.CommandPath(), outcome, map[string]any{
					"duration_ms": duration.Milliseconds(),
					"args":        args,
					"request_id":  requestID,
				})
			}

			return err
		}
	}
}

// Timing creates a middleware that adds timing information to output.
func Timing(verbose bool) Middleware {
	return func(next RunFunc) RunFunc {
		return func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			err := next(cmd, args)
			if verbose {
				duration := time.Since(start)
				fmt.Fprintf(cmd.ErrOrStderr(), "\nCompleted in %s\n", duration.Round(time.Millisecond))
			}
			return err
		}
	}
}
