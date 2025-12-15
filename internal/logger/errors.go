package logger

import (
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
)

// WrappedError is an error that includes additional context and stack information.
type WrappedError struct {
	msg    string
	cause  error
	caller string
}

// Error implements the error interface.
func (e *WrappedError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.msg, e.cause)
	}
	return e.msg
}

// Unwrap returns the underlying error.
func (e *WrappedError) Unwrap() error {
	return e.cause
}

// Caller returns the caller information.
func (e *WrappedError) Caller() string {
	return e.caller
}

// WrapError wraps an error with a message and caller information.
func WrapError(err error, msg string) error {
	if err == nil {
		return nil
	}

	_, file, line, ok := runtime.Caller(1)
	caller := "unknown"
	if ok {
		// Shorten the file path
		parts := strings.Split(file, "/")
		if len(parts) > 2 {
			file = strings.Join(parts[len(parts)-2:], "/")
		}
		caller = fmt.Sprintf("%s:%d", file, line)
	}

	return &WrappedError{
		msg:    msg,
		cause:  err,
		caller: caller,
	}
}

// WithError creates an slog.Attr for an error with detailed information.
func WithError(err error) slog.Attr {
	if err == nil {
		return slog.Attr{}
	}

	attrs := []any{
		slog.String("message", err.Error()),
		slog.String("type", fmt.Sprintf("%T", err)),
	}

	// Add unwrapped cause if available
	if cause := errors.Unwrap(err); cause != nil {
		attrs = append(attrs, slog.String("cause", cause.Error()))
	}

	// Add caller info if it's a WrappedError
	if we, ok := err.(*WrappedError); ok {
		attrs = append(attrs, slog.String("caller", we.Caller()))
	}

	return slog.Group("error", attrs...)
}

// WithStack captures the current stack trace as an slog.Attr.
func WithStack() slog.Attr {
	return slog.String("stack", captureStack(2))
}

// WithStackSkip captures the stack trace, skipping the specified number of frames.
func WithStackSkip(skip int) slog.Attr {
	return slog.String("stack", captureStack(skip+1))
}

// captureStack captures the current stack trace as a string.
func captureStack(skip int) string {
	const maxDepth = 32
	var pcs [maxDepth]uintptr
	n := runtime.Callers(skip+1, pcs[:])
	if n == 0 {
		return ""
	}

	frames := runtime.CallersFrames(pcs[:n])
	var sb strings.Builder

	for {
		frame, more := frames.Next()
		// Skip runtime frames
		if strings.Contains(frame.File, "runtime/") {
			if !more {
				break
			}
			continue
		}

		// Shorten file path
		file := frame.File
		parts := strings.Split(file, "/")
		if len(parts) > 3 {
			file = strings.Join(parts[len(parts)-3:], "/")
		}

		fmt.Fprintf(&sb, "%s:%d %s\n", file, frame.Line, frame.Function)

		if !more {
			break
		}
	}

	return strings.TrimSpace(sb.String())
}

// ErrorGroup creates a group of error-related attributes.
func ErrorGroup(err error, includeStack bool) slog.Attr {
	if err == nil {
		return slog.Attr{}
	}

	attrs := []any{
		slog.String("message", err.Error()),
		slog.String("type", fmt.Sprintf("%T", err)),
	}

	// Walk the error chain
	var chain []string
	for e := err; e != nil; e = errors.Unwrap(e) {
		chain = append(chain, fmt.Sprintf("%T: %s", e, e.Error()))
	}
	if len(chain) > 1 {
		attrs = append(attrs, slog.Any("chain", chain))
	}

	if includeStack {
		attrs = append(attrs, slog.String("stack", captureStack(2)))
	}

	return slog.Group("error", attrs...)
}
