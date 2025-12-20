// Package client provides a gRPC client library for connecting to bibd.
package client

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Common client errors
var (
	// ErrNotConnected indicates the client is not connected.
	ErrNotConnected = errors.New("client not connected")

	// ErrNotAuthenticated indicates the client is not authenticated.
	ErrNotAuthenticated = errors.New("not authenticated")

	// ErrConnectionFailed indicates all connection attempts failed.
	ErrConnectionFailed = errors.New("connection failed")

	// ErrAuthenticationFailed indicates authentication failed.
	ErrAuthenticationFailed = errors.New("authentication failed")

	// ErrSessionExpired indicates the session has expired.
	ErrSessionExpired = errors.New("session expired")

	// ErrNoSSHKeys indicates no SSH keys are available.
	ErrNoSSHKeys = errors.New("no SSH keys available")

	// ErrPermissionDenied indicates insufficient permissions.
	ErrPermissionDenied = errors.New("permission denied")

	// ErrNotFound indicates the requested resource was not found.
	ErrNotFound = errors.New("not found")

	// ErrAlreadyExists indicates the resource already exists.
	ErrAlreadyExists = errors.New("already exists")

	// ErrInvalidArgument indicates invalid input.
	ErrInvalidArgument = errors.New("invalid argument")

	// ErrInternal indicates an internal server error.
	ErrInternal = errors.New("internal error")

	// ErrUnavailable indicates the service is unavailable.
	ErrUnavailable = errors.New("service unavailable")
)

// Error wraps a gRPC error with additional context.
type Error struct {
	// Code is the gRPC status code.
	Code codes.Code

	// Message is the error message.
	Message string

	// Details contains additional error details.
	Details map[string]string

	// Cause is the underlying error.
	Cause error
}

// Error returns the error message.
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error.
func (e *Error) Unwrap() error {
	return e.Cause
}

// IsRetryable returns true if the error is retryable.
func (e *Error) IsRetryable() bool {
	switch e.Code {
	case codes.Unavailable, codes.ResourceExhausted, codes.Aborted, codes.DeadlineExceeded:
		return true
	default:
		return false
	}
}

// FromGRPCError converts a gRPC error to a client Error.
func FromGRPCError(err error) *Error {
	if err == nil {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok {
		return &Error{
			Code:    codes.Unknown,
			Message: err.Error(),
			Cause:   err,
		}
	}

	return &Error{
		Code:    st.Code(),
		Message: st.Message(),
		Cause:   err,
	}
}

// IsNotFound returns true if the error indicates a not found condition.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrNotFound) {
		return true
	}
	if e, ok := err.(*Error); ok {
		return e.Code == codes.NotFound
	}
	st, ok := status.FromError(err)
	return ok && st.Code() == codes.NotFound
}

// IsPermissionDenied returns true if the error indicates permission denied.
func IsPermissionDenied(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrPermissionDenied) {
		return true
	}
	if e, ok := err.(*Error); ok {
		return e.Code == codes.PermissionDenied
	}
	st, ok := status.FromError(err)
	return ok && st.Code() == codes.PermissionDenied
}

// IsUnauthenticated returns true if the error indicates unauthenticated.
func IsUnauthenticated(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrNotAuthenticated) || errors.Is(err, ErrSessionExpired) {
		return true
	}
	if e, ok := err.(*Error); ok {
		return e.Code == codes.Unauthenticated
	}
	st, ok := status.FromError(err)
	return ok && st.Code() == codes.Unauthenticated
}

// IsUnavailable returns true if the error indicates the service is unavailable.
func IsUnavailable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrUnavailable) || errors.Is(err, ErrConnectionFailed) {
		return true
	}
	if e, ok := err.(*Error); ok {
		return e.Code == codes.Unavailable
	}
	st, ok := status.FromError(err)
	return ok && st.Code() == codes.Unavailable
}

// IsInvalidArgument returns true if the error indicates invalid input.
func IsInvalidArgument(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrInvalidArgument) {
		return true
	}
	if e, ok := err.(*Error); ok {
		return e.Code == codes.InvalidArgument
	}
	st, ok := status.FromError(err)
	return ok && st.Code() == codes.InvalidArgument
}

// IsRetryable returns true if the error is retryable.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*Error); ok {
		return e.IsRetryable()
	}
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	switch st.Code() {
	case codes.Unavailable, codes.ResourceExhausted, codes.Aborted, codes.DeadlineExceeded:
		return true
	default:
		return false
	}
}

// WrapError wraps an error with context.
func WrapError(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}
