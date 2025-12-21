package recovery

import (
	"errors"
	"testing"
)

func TestNewInterruptHandler(t *testing.T) {
	handler := NewInterruptHandler()

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if handler.Context == nil {
		t.Error("context is nil")
	}

	if handler.Cancel == nil {
		t.Error("cancel is nil")
	}

	if handler.IsInterrupted() {
		t.Error("should not be interrupted initially")
	}
}

func TestNewErrorHandler(t *testing.T) {
	handler := NewErrorHandler()

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if handler.MaxRetries != 3 {
		t.Errorf("expected max retries 3, got %d", handler.MaxRetries)
	}
}

func TestErrorHandler_HandleError(t *testing.T) {
	handler := NewErrorHandler()
	err := errors.New("test error")

	// Should retry first
	action := handler.HandleError("step1", err)
	if action != ErrorActionRetry {
		t.Errorf("expected retry, got %s", action)
	}

	// Retry count should increase
	if handler.GetRetryCount("step1") != 1 {
		t.Error("retry count should be 1")
	}

	// Exhaust retries
	handler.HandleError("step1", err)
	handler.HandleError("step1", err)

	// Should abort after max retries
	action = handler.HandleError("step1", err)
	if action != ErrorActionAbort {
		t.Errorf("expected abort after max retries, got %s", action)
	}
}

func TestErrorHandler_ResetRetries(t *testing.T) {
	handler := NewErrorHandler()

	handler.HandleError("step1", errors.New("error"))
	handler.HandleError("step1", errors.New("error"))

	if handler.GetRetryCount("step1") != 2 {
		t.Error("retry count should be 2")
	}

	handler.ResetRetries("step1")

	if handler.GetRetryCount("step1") != 0 {
		t.Error("retry count should be 0 after reset")
	}
}

func TestErrorHandler_CustomOnError(t *testing.T) {
	handler := NewErrorHandler()
	called := false

	handler.OnError = func(step string, err error) ErrorAction {
		called = true
		if step == "skip_step" {
			return ErrorActionSkip
		}
		return ErrorActionAbort
	}

	action := handler.HandleError("skip_step", errors.New("error"))

	if !called {
		t.Error("OnError should be called")
	}

	if action != ErrorActionSkip {
		t.Errorf("expected skip, got %s", action)
	}
}

func TestNewSetupError(t *testing.T) {
	cause := errors.New("underlying error")
	err := NewSetupError("network", "connection failed", cause)

	if err.Step != "network" {
		t.Error("step mismatch")
	}

	if err.Message != "connection failed" {
		t.Error("message mismatch")
	}

	if err.Cause != cause {
		t.Error("cause mismatch")
	}

	if !err.Recoverable {
		t.Error("should be recoverable by default")
	}
}

func TestSetupError_Error(t *testing.T) {
	t.Run("with cause", func(t *testing.T) {
		err := NewSetupError("step", "message", errors.New("cause"))
		errorStr := err.Error()

		if errorStr == "" {
			t.Error("error string should not be empty")
		}

		if !containsSubstring(errorStr, "step") {
			t.Error("should contain step")
		}
		if !containsSubstring(errorStr, "message") {
			t.Error("should contain message")
		}
		if !containsSubstring(errorStr, "cause") {
			t.Error("should contain cause")
		}
	})

	t.Run("without cause", func(t *testing.T) {
		err := NewSetupError("step", "message", nil)
		errorStr := err.Error()

		if !containsSubstring(errorStr, "step") {
			t.Error("should contain step")
		}
		if !containsSubstring(errorStr, "message") {
			t.Error("should contain message")
		}
	})
}

func TestSetupError_Unwrap(t *testing.T) {
	cause := errors.New("cause")
	err := NewSetupError("step", "message", cause)

	unwrapped := err.Unwrap()
	if unwrapped != cause {
		t.Error("unwrap should return cause")
	}
}

func TestSetupError_WithRecoverable(t *testing.T) {
	err := NewSetupError("step", "message", nil).WithRecoverable(false)

	if err.Recoverable {
		t.Error("should not be recoverable")
	}
}

func TestSetupError_WithSuggestions(t *testing.T) {
	err := NewSetupError("step", "message", nil).
		WithSuggestions("suggestion 1", "suggestion 2")

	if len(err.Suggestions) != 2 {
		t.Errorf("expected 2 suggestions, got %d", len(err.Suggestions))
	}
}

func TestFormatSetupError(t *testing.T) {
	err := NewSetupError("network", "connection failed", errors.New("timeout")).
		WithSuggestions("Check connection", "Try again")

	formatted := FormatSetupError(err)

	if formatted == "" {
		t.Error("formatted should not be empty")
	}

	if !containsSubstring(formatted, "network") {
		t.Error("should contain step")
	}
	if !containsSubstring(formatted, "connection failed") {
		t.Error("should contain message")
	}
	if !containsSubstring(formatted, "Suggestions") {
		t.Error("should contain suggestions")
	}
}

func TestCommonErrors_NetworkUnavailable(t *testing.T) {
	err := CommonErrors.NetworkUnavailable()

	if err.Step != "network" {
		t.Error("step mismatch")
	}

	if len(err.Suggestions) == 0 {
		t.Error("should have suggestions")
	}
}

func TestCommonErrors_DatabaseConnection(t *testing.T) {
	cause := errors.New("connection refused")
	err := CommonErrors.DatabaseConnection(cause)

	if err.Step != "database" {
		t.Error("step mismatch")
	}

	if err.Cause != cause {
		t.Error("cause mismatch")
	}
}

func TestCommonErrors_DockerNotRunning(t *testing.T) {
	err := CommonErrors.DockerNotRunning()

	if err.Step != "deployment" {
		t.Error("step mismatch")
	}

	if !err.Recoverable {
		t.Error("should be recoverable")
	}
}

func TestCommonErrors_PermissionDenied(t *testing.T) {
	err := CommonErrors.PermissionDenied("/etc/bibd")

	if !containsSubstring(err.Message, "/etc/bibd") {
		t.Error("should contain path")
	}
}

func TestCommonErrors_SQLiteWithFullMode(t *testing.T) {
	err := CommonErrors.SQLiteWithFullMode()

	if err.Step != "storage" {
		t.Error("step mismatch")
	}

	if !err.Recoverable {
		t.Error("should be recoverable")
	}

	// Should suggest PostgreSQL
	hasPGSuggestion := false
	for _, s := range err.Suggestions {
		if containsSubstring(s, "PostgreSQL") {
			hasPGSuggestion = true
			break
		}
	}
	if !hasPGSuggestion {
		t.Error("should suggest PostgreSQL")
	}
}

func TestCommonErrors_ConnectionFailed(t *testing.T) {
	cause := errors.New("connection refused")
	err := CommonErrors.ConnectionFailed("localhost:4000", cause)

	if !containsSubstring(err.Message, "localhost:4000") {
		t.Error("should contain address")
	}

	if err.Cause != cause {
		t.Error("cause mismatch")
	}
}

func TestErrorAction_Values(t *testing.T) {
	if ErrorActionRetry != "retry" {
		t.Error("retry mismatch")
	}
	if ErrorActionReconfigure != "reconfigure" {
		t.Error("reconfigure mismatch")
	}
	if ErrorActionSkip != "skip" {
		t.Error("skip mismatch")
	}
	if ErrorActionAbort != "abort" {
		t.Error("abort mismatch")
	}
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
