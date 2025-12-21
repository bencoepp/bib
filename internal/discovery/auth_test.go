package discovery

import (
	"context"
	"testing"
	"time"

	"bib/internal/auth"
)

func TestNewAuthTester(t *testing.T) {
	// Create a test identity key
	key, err := auth.GenerateIdentityKey()
	if err != nil {
		t.Fatalf("failed to generate identity key: %v", err)
	}

	tester := NewAuthTester(key)

	if tester == nil {
		t.Fatal("tester is nil")
	}

	if tester.IdentityKey != key {
		t.Error("identity key not set")
	}

	if tester.Timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", tester.Timeout)
	}
}

func TestAuthTester_WithTimeout(t *testing.T) {
	key, _ := auth.GenerateIdentityKey()
	tester := NewAuthTester(key).WithTimeout(5 * time.Second)

	if tester.Timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", tester.Timeout)
	}
}

func TestAuthTester_WithRegistrationInfo(t *testing.T) {
	key, _ := auth.GenerateIdentityKey()
	tester := NewAuthTester(key).WithRegistrationInfo("Test User", "test@example.com")

	if tester.Name != "Test User" {
		t.Errorf("expected name 'Test User', got %q", tester.Name)
	}

	if tester.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got %q", tester.Email)
	}
}

func TestAuthStatus_Constants(t *testing.T) {
	tests := []struct {
		status   AuthStatus
		expected string
	}{
		{AuthStatusSuccess, "success"},
		{AuthStatusFailed, "failed"},
		{AuthStatusNoKey, "no_key"},
		{AuthStatusKeyRejected, "key_rejected"},
		{AuthStatusNotRegistered, "not_registered"},
		{AuthStatusAutoRegistered, "auto_registered"},
		{AuthStatusConnectionError, "connection_error"},
		{AuthStatusUnknown, "unknown"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.expected {
			t.Errorf("expected %q, got %q", tt.expected, string(tt.status))
		}
	}
}

func TestAuthTester_TestAuth_NoKey(t *testing.T) {
	tester := &AuthTester{
		IdentityKey: nil,
		Timeout:     1 * time.Second,
	}

	ctx := context.Background()
	result := tester.TestAuth(ctx, "localhost:4000")

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Status != AuthStatusNoKey {
		t.Errorf("expected status %q, got %q", AuthStatusNoKey, result.Status)
	}

	if result.Error == "" {
		t.Error("expected error message for no key")
	}
}

func TestAuthTester_TestAuth_ConnectionError(t *testing.T) {
	key, err := auth.GenerateIdentityKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	tester := NewAuthTester(key).WithTimeout(1 * time.Second)

	ctx := context.Background()
	// Test against a port that should be closed
	result := tester.TestAuth(ctx, "127.0.0.1:59999")

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Status != AuthStatusConnectionError {
		t.Errorf("expected status %q, got %q", AuthStatusConnectionError, result.Status)
	}

	if result.Duration == 0 {
		t.Error("expected duration to be set")
	}
}

func TestAuthTester_ClassifyAuthError(t *testing.T) {
	tester := &AuthTester{}

	tests := []struct {
		name     string
		errMsg   string
		expected AuthStatus
	}{
		{"nil error", "", AuthStatusSuccess},
		{"not registered", "user not registered on this node", AuthStatusNotRegistered},
		{"unknown user", "unknown user", AuthStatusNotRegistered},
		{"key rejected", "key rejected by server", AuthStatusKeyRejected},
		{"invalid signature", "invalid signature", AuthStatusKeyRejected},
		{"connection error", "connection refused", AuthStatusConnectionError},
		{"unavailable", "service unavailable", AuthStatusConnectionError},
		{"deadline exceeded", "context deadline exceeded", AuthStatusConnectionError},
		{"generic error", "some random error", AuthStatusFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.errMsg != "" {
				err = &testError{msg: tt.errMsg}
			}
			status := tester.classifyAuthError(err)
			if status != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, status)
			}
		})
	}
}

func TestAuthTestResult_Fields(t *testing.T) {
	result := AuthTestResult{
		Address: "test:4000",
		Status:  AuthStatusSuccess,
		SessionInfo: &SessionInfo{
			UserID:    "user-123",
			Username:  "testuser",
			Role:      "admin",
			ExpiresAt: time.Now().Add(24 * time.Hour),
			IsNewUser: false,
		},
		ServerConfig: &ServerAuthConfig{
			AllowAutoRegistration: true,
			RequireEmail:          false,
			SupportedKeyTypes:     []string{"ed25519", "rsa"},
		},
		SessionToken: "token-abc",
		Duration:     100 * time.Millisecond,
		TestedAt:     time.Now(),
	}

	if result.Address != "test:4000" {
		t.Error("address mismatch")
	}
	if result.SessionInfo.Username != "testuser" {
		t.Error("username mismatch")
	}
	if result.ServerConfig.AllowAutoRegistration != true {
		t.Error("allow auto registration mismatch")
	}
	if len(result.ServerConfig.SupportedKeyTypes) != 2 {
		t.Error("supported key types length mismatch")
	}
}

func TestFormatAuthResult(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		result := &AuthTestResult{
			Address:  "localhost:4000",
			Status:   AuthStatusSuccess,
			Duration: 50 * time.Millisecond,
			SessionInfo: &SessionInfo{
				Username: "testuser",
				Role:     "admin",
			},
		}
		output := FormatAuthResult(result)
		if !containsStr(output, "✓") {
			t.Error("expected checkmark for success")
		}
		if !containsStr(output, "localhost:4000") {
			t.Error("expected address in output")
		}
		if !containsStr(output, "testuser") {
			t.Error("expected username in output")
		}
		if !containsStr(output, "admin") {
			t.Error("expected role in output")
		}
	})

	t.Run("auto registered", func(t *testing.T) {
		result := &AuthTestResult{
			Address:  "localhost:4000",
			Status:   AuthStatusAutoRegistered,
			Duration: 100 * time.Millisecond,
			SessionInfo: &SessionInfo{
				Username:  "newuser",
				IsNewUser: true,
			},
		}
		output := FormatAuthResult(result)
		if !containsStr(output, "✓+") {
			t.Error("expected ✓+ for auto-registered")
		}
		if !containsStr(output, "auto-registered") {
			t.Error("expected 'auto-registered' in output")
		}
	})

	t.Run("not registered", func(t *testing.T) {
		result := &AuthTestResult{
			Address: "localhost:4000",
			Status:  AuthStatusNotRegistered,
		}
		output := FormatAuthResult(result)
		if !containsStr(output, "?") {
			t.Error("expected ? for not registered")
		}
		if !containsStr(output, "not registered") {
			t.Error("expected 'not registered' in output")
		}
	})

	t.Run("key rejected", func(t *testing.T) {
		result := &AuthTestResult{
			Address: "localhost:4000",
			Status:  AuthStatusKeyRejected,
		}
		output := FormatAuthResult(result)
		if !containsStr(output, "✗") {
			t.Error("expected ✗ for key rejected")
		}
		if !containsStr(output, "key rejected") {
			t.Error("expected 'key rejected' in output")
		}
	})

	t.Run("no key", func(t *testing.T) {
		result := &AuthTestResult{
			Address: "localhost:4000",
			Status:  AuthStatusNoKey,
		}
		output := FormatAuthResult(result)
		if !containsStr(output, "🔑") {
			t.Error("expected key icon for no key")
		}
	})

	t.Run("connection error", func(t *testing.T) {
		result := &AuthTestResult{
			Address: "localhost:4000",
			Status:  AuthStatusConnectionError,
			Error:   "connection refused",
		}
		output := FormatAuthResult(result)
		if !containsStr(output, "⚡") {
			t.Error("expected ⚡ for connection error")
		}
		if !containsStr(output, "connection refused") {
			t.Error("expected error message in output")
		}
	})
}

func TestFormatAuthResults(t *testing.T) {
	results := []*AuthTestResult{
		{Address: "localhost:4000", Status: AuthStatusSuccess, Duration: 50 * time.Millisecond},
		{Address: "localhost:4001", Status: AuthStatusAutoRegistered, Duration: 100 * time.Millisecond},
		{Address: "localhost:4002", Status: AuthStatusNotRegistered},
	}

	output := FormatAuthResults(results)

	if !containsStr(output, "1 success") {
		t.Error("expected '1 success' in output")
	}
	if !containsStr(output, "1 auto-registered") {
		t.Error("expected '1 auto-registered' in output")
	}
	if !containsStr(output, "1 failed") {
		t.Error("expected '1 failed' in output")
	}
}

func TestAuthSummary(t *testing.T) {
	results := []*AuthTestResult{
		{Status: AuthStatusSuccess},
		{Status: AuthStatusAutoRegistered},
		{Status: AuthStatusNotRegistered},
		{Status: AuthStatusFailed},
	}

	summary := AuthSummary(results)

	if !containsStr(summary, "2 authenticated") {
		t.Errorf("expected '2 authenticated' in summary, got %q", summary)
	}
	if !containsStr(summary, "2 failed") {
		t.Errorf("expected '2 failed' in summary, got %q", summary)
	}
}

func TestSessionInfo_Fields(t *testing.T) {
	expiry := time.Now().Add(24 * time.Hour)
	info := SessionInfo{
		UserID:    "user-123",
		Username:  "testuser",
		Role:      "admin",
		ExpiresAt: expiry,
		IsNewUser: true,
	}

	if info.UserID != "user-123" {
		t.Error("user ID mismatch")
	}
	if info.Username != "testuser" {
		t.Error("username mismatch")
	}
	if info.Role != "admin" {
		t.Error("role mismatch")
	}
	if !info.ExpiresAt.Equal(expiry) {
		t.Error("expires at mismatch")
	}
	if !info.IsNewUser {
		t.Error("is new user should be true")
	}
}

func TestServerAuthConfig_Fields(t *testing.T) {
	config := ServerAuthConfig{
		AllowAutoRegistration: true,
		RequireEmail:          false,
		SupportedKeyTypes:     []string{"ed25519", "rsa", "ecdsa"},
	}

	if !config.AllowAutoRegistration {
		t.Error("allow auto registration should be true")
	}
	if config.RequireEmail {
		t.Error("require email should be false")
	}
	if len(config.SupportedKeyTypes) != 3 {
		t.Errorf("expected 3 key types, got %d", len(config.SupportedKeyTypes))
	}
}
