// Package recovery provides error recovery and graceful interruption for setup.
package recovery

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// InterruptHandler handles graceful interruption
type InterruptHandler struct {
	// OnInterrupt is called when Ctrl+C is pressed
	OnInterrupt func() bool // Return true to exit, false to continue

	// Context is the context for the handler
	Context context.Context

	// Cancel is the cancel function
	Cancel context.CancelFunc

	// interrupted tracks if we've been interrupted
	interrupted bool
	mu          sync.Mutex

	// signalCh receives interrupt signals
	signalCh chan os.Signal

	// done indicates the handler is stopped
	done chan struct{}
}

// NewInterruptHandler creates a new interrupt handler
func NewInterruptHandler() *InterruptHandler {
	ctx, cancel := context.WithCancel(context.Background())

	return &InterruptHandler{
		Context:  ctx,
		Cancel:   cancel,
		signalCh: make(chan os.Signal, 1),
		done:     make(chan struct{}),
	}
}

// Start begins listening for interrupt signals
func (h *InterruptHandler) Start() {
	signal.Notify(h.signalCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		defer close(h.done)

		for {
			select {
			case <-h.Context.Done():
				return
			case sig := <-h.signalCh:
				if sig == nil {
					return
				}

				h.mu.Lock()
				h.interrupted = true
				h.mu.Unlock()

				if h.OnInterrupt != nil {
					if h.OnInterrupt() {
						h.Cancel()
						return
					}
				} else {
					h.Cancel()
					return
				}
			}
		}
	}()
}

// Stop stops listening for interrupt signals
func (h *InterruptHandler) Stop() {
	signal.Stop(h.signalCh)
	close(h.signalCh)
	h.Cancel()
	<-h.done
}

// IsInterrupted returns true if interrupted
func (h *InterruptHandler) IsInterrupted() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.interrupted
}

// ErrorAction represents an action to take when an error occurs
type ErrorAction string

const (
	ErrorActionRetry       ErrorAction = "retry"
	ErrorActionReconfigure ErrorAction = "reconfigure"
	ErrorActionSkip        ErrorAction = "skip"
	ErrorActionAbort       ErrorAction = "abort"
)

// ErrorHandler handles errors during setup
type ErrorHandler struct {
	// OnError is called when an error occurs
	// Returns the action to take
	OnError func(step string, err error) ErrorAction

	// MaxRetries is the maximum number of retries
	MaxRetries int

	// retryCount tracks retries per step
	retryCount map[string]int
	mu         sync.Mutex
}

// NewErrorHandler creates a new error handler
func NewErrorHandler() *ErrorHandler {
	return &ErrorHandler{
		MaxRetries: 3,
		retryCount: make(map[string]int),
	}
}

// HandleError handles an error and returns the action to take
func (h *ErrorHandler) HandleError(step string, err error) ErrorAction {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.OnError != nil {
		return h.OnError(step, err)
	}

	// Default behavior: retry up to MaxRetries
	h.retryCount[step]++
	if h.retryCount[step] <= h.MaxRetries {
		return ErrorActionRetry
	}

	return ErrorActionAbort
}

// ResetRetries resets the retry count for a step
func (h *ErrorHandler) ResetRetries(step string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.retryCount, step)
}

// GetRetryCount returns the retry count for a step
func (h *ErrorHandler) GetRetryCount(step string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.retryCount[step]
}

// SetupError represents an error during setup with context
type SetupError struct {
	// Step is the step that failed
	Step string

	// Message is the error message
	Message string

	// Cause is the underlying error
	Cause error

	// Recoverable indicates if the error is recoverable
	Recoverable bool

	// Suggestions are suggested actions
	Suggestions []string
}

// Error implements the error interface
func (e *SetupError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Step, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Step, e.Message)
}

// Unwrap returns the underlying error
func (e *SetupError) Unwrap() error {
	return e.Cause
}

// NewSetupError creates a new setup error
func NewSetupError(step, message string, cause error) *SetupError {
	return &SetupError{
		Step:        step,
		Message:     message,
		Cause:       cause,
		Recoverable: true,
	}
}

// WithRecoverable sets whether the error is recoverable
func (e *SetupError) WithRecoverable(recoverable bool) *SetupError {
	e.Recoverable = recoverable
	return e
}

// WithSuggestions adds suggestions
func (e *SetupError) WithSuggestions(suggestions ...string) *SetupError {
	e.Suggestions = append(e.Suggestions, suggestions...)
	return e
}

// FormatSetupError formats a setup error for display
func FormatSetupError(err *SetupError) string {
	result := fmt.Sprintf("❌ Error in %s step:\n", err.Step)
	result += fmt.Sprintf("   %s\n", err.Message)

	if err.Cause != nil {
		result += fmt.Sprintf("   Cause: %v\n", err.Cause)
	}

	if len(err.Suggestions) > 0 {
		result += "\n   Suggestions:\n"
		for _, s := range err.Suggestions {
			result += fmt.Sprintf("   • %s\n", s)
		}
	}

	if err.Recoverable {
		result += "\n   This error may be recoverable. You can:\n"
		result += "   • Retry the step\n"
		result += "   • Skip this step\n"
		result += "   • Reconfigure settings\n"
	}

	return result
}

// CommonErrors provides common error scenarios
var CommonErrors = struct {
	NetworkUnavailable   func() *SetupError
	DatabaseConnection   func(err error) *SetupError
	DockerNotRunning     func() *SetupError
	KubectlNotFound      func() *SetupError
	PermissionDenied     func(path string) *SetupError
	ConfigExists         func(path string) *SetupError
	InvalidConfiguration func(field, reason string) *SetupError
	SQLiteWithFullMode   func() *SetupError
	ServiceInstallFailed func(err error) *SetupError
	ConnectionFailed     func(address string, err error) *SetupError
}{
	NetworkUnavailable: func() *SetupError {
		return NewSetupError("network", "Network is unavailable", nil).
			WithSuggestions(
				"Check your internet connection",
				"Try again later",
				"Use private network mode",
			)
	},
	DatabaseConnection: func(err error) *SetupError {
		return NewSetupError("database", "Failed to connect to database", err).
			WithSuggestions(
				"Verify database is running",
				"Check connection settings",
				"Ensure database user has correct permissions",
			)
	},
	DockerNotRunning: func() *SetupError {
		return NewSetupError("deployment", "Docker is not running", nil).
			WithRecoverable(true).
			WithSuggestions(
				"Start Docker Desktop or docker daemon",
				"Run: sudo systemctl start docker",
				"Choose a different deployment target",
			)
	},
	KubectlNotFound: func() *SetupError {
		return NewSetupError("deployment", "kubectl not found", nil).
			WithSuggestions(
				"Install kubectl: https://kubernetes.io/docs/tasks/tools/",
				"Ensure kubectl is in your PATH",
				"Choose a different deployment target",
			)
	},
	PermissionDenied: func(path string) *SetupError {
		return NewSetupError("filesystem", fmt.Sprintf("Permission denied: %s", path), nil).
			WithSuggestions(
				"Run with elevated privileges (sudo)",
				"Check file/directory permissions",
				"Choose a different output directory",
			)
	},
	ConfigExists: func(path string) *SetupError {
		return NewSetupError("config", fmt.Sprintf("Configuration already exists: %s", path), nil).
			WithRecoverable(true).
			WithSuggestions(
				"Use --fresh to start fresh",
				"Back up and delete existing config",
				"Use --reconfigure to modify existing config",
			)
	},
	InvalidConfiguration: func(field, reason string) *SetupError {
		return NewSetupError("config", fmt.Sprintf("Invalid %s: %s", field, reason), nil).
			WithSuggestions(
				"Check the configuration value",
				"Reconfigure this setting",
			)
	},
	SQLiteWithFullMode: func() *SetupError {
		return NewSetupError("storage", "SQLite is not supported with full replication mode", nil).
			WithRecoverable(true).
			WithSuggestions(
				"Switch to PostgreSQL for full replication",
				"Use proxy or selective mode with SQLite",
			)
	},
	ServiceInstallFailed: func(err error) *SetupError {
		return NewSetupError("service", "Failed to install service", err).
			WithSuggestions(
				"Run with administrator/root privileges",
				"Skip service installation and run manually",
				"Check system logs for details",
			)
	},
	ConnectionFailed: func(address string, err error) *SetupError {
		return NewSetupError("connection", fmt.Sprintf("Failed to connect to %s", address), err).
			WithSuggestions(
				"Verify the address is correct",
				"Check that the node is running",
				"Check firewall settings",
			)
	},
}
