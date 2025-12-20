package client

import (
	"testing"

	"google.golang.org/grpc/codes"
)

func TestErrorTypes(t *testing.T) {
	// Test that error variables are defined
	errors := []error{
		ErrNotConnected,
		ErrNotAuthenticated,
		ErrConnectionFailed,
		ErrAuthenticationFailed,
		ErrSessionExpired,
		ErrNoSSHKeys,
		ErrPermissionDenied,
		ErrNotFound,
		ErrAlreadyExists,
		ErrInvalidArgument,
		ErrInternal,
		ErrUnavailable,
	}

	for _, err := range errors {
		if err == nil {
			t.Error("error should not be nil")
		}
		if err.Error() == "" {
			t.Error("error message should not be empty")
		}
	}
}

func TestError_Error(t *testing.T) {
	err := &Error{
		Code:    5,
		Message: "test error",
	}

	msg := err.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}
}

func TestError_ErrorWithCause(t *testing.T) {
	cause := ErrNotFound
	err := &Error{
		Code:    5,
		Message: "test error",
		Cause:   cause,
	}

	msg := err.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}
}

func TestError_Unwrap(t *testing.T) {
	cause := ErrNotFound
	err := &Error{
		Code:    5,
		Message: "test error",
		Cause:   cause,
	}

	if err.Unwrap() != cause {
		t.Error("Unwrap should return cause")
	}
}

func TestError_IsRetryable(t *testing.T) {
	tests := []struct {
		code     int
		expected bool
	}{
		{14, true},  // Unavailable
		{8, true},   // ResourceExhausted
		{10, true},  // Aborted
		{4, true},   // DeadlineExceeded
		{5, false},  // NotFound
		{7, false},  // PermissionDenied
		{16, false}, // Unauthenticated
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			err := &Error{Code: codes.Code(tt.code)}
			if err.IsRetryable() != tt.expected {
				t.Errorf("IsRetryable() for code %d = %v, expected %v", tt.code, err.IsRetryable(), tt.expected)
			}
		})
	}
}

func TestIsNotFound(t *testing.T) {
	if !IsNotFound(ErrNotFound) {
		t.Error("expected IsNotFound(ErrNotFound) to be true")
	}

	if IsNotFound(ErrPermissionDenied) {
		t.Error("expected IsNotFound(ErrPermissionDenied) to be false")
	}

	if IsNotFound(nil) {
		t.Error("expected IsNotFound(nil) to be false")
	}
}

func TestIsPermissionDenied(t *testing.T) {
	if !IsPermissionDenied(ErrPermissionDenied) {
		t.Error("expected IsPermissionDenied(ErrPermissionDenied) to be true")
	}

	if IsPermissionDenied(ErrNotFound) {
		t.Error("expected IsPermissionDenied(ErrNotFound) to be false")
	}

	if IsPermissionDenied(nil) {
		t.Error("expected IsPermissionDenied(nil) to be false")
	}
}

func TestIsUnauthenticated(t *testing.T) {
	if !IsUnauthenticated(ErrNotAuthenticated) {
		t.Error("expected IsUnauthenticated(ErrNotAuthenticated) to be true")
	}

	if !IsUnauthenticated(ErrSessionExpired) {
		t.Error("expected IsUnauthenticated(ErrSessionExpired) to be true")
	}

	if IsUnauthenticated(ErrNotFound) {
		t.Error("expected IsUnauthenticated(ErrNotFound) to be false")
	}

	if IsUnauthenticated(nil) {
		t.Error("expected IsUnauthenticated(nil) to be false")
	}
}

func TestIsUnavailable(t *testing.T) {
	if !IsUnavailable(ErrUnavailable) {
		t.Error("expected IsUnavailable(ErrUnavailable) to be true")
	}

	if !IsUnavailable(ErrConnectionFailed) {
		t.Error("expected IsUnavailable(ErrConnectionFailed) to be true")
	}

	if IsUnavailable(ErrNotFound) {
		t.Error("expected IsUnavailable(ErrNotFound) to be false")
	}

	if IsUnavailable(nil) {
		t.Error("expected IsUnavailable(nil) to be false")
	}
}

func TestIsInvalidArgument(t *testing.T) {
	if !IsInvalidArgument(ErrInvalidArgument) {
		t.Error("expected IsInvalidArgument(ErrInvalidArgument) to be true")
	}

	if IsInvalidArgument(ErrNotFound) {
		t.Error("expected IsInvalidArgument(ErrNotFound) to be false")
	}

	if IsInvalidArgument(nil) {
		t.Error("expected IsInvalidArgument(nil) to be false")
	}
}

func TestWrapError(t *testing.T) {
	err := WrapError(ErrNotFound, "failed to get user")
	if err == nil {
		t.Error("expected non-nil error")
	}

	msg := err.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}

	// Nil should return nil
	if WrapError(nil, "test") != nil {
		t.Error("expected nil for nil error")
	}
}
