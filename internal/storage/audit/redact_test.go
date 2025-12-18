package audit

import (
	"testing"
)

func TestNewRedactor(t *testing.T) {
	cfg := DefaultRedactorConfig()
	redactor := NewRedactor(cfg)

	if redactor == nil {
		t.Fatal("NewRedactor returned nil")
	}

	if len(redactor.sensitiveFields) == 0 {
		t.Error("sensitiveFields should not be empty")
	}
}

func TestRedactor_IsSensitiveField(t *testing.T) {
	redactor := NewRedactor(DefaultRedactorConfig())

	tests := []struct {
		field    string
		expected bool
	}{
		{"password", true},
		{"PASSWORD", true},
		{"Password", true},
		{"user_password", true},
		{"password_hash", true},
		{"token", true},
		{"access_token", true},
		{"api_key", true},
		{"apikey", true},
		{"secret", true},
		{"credential", true},
		{"auth", true},
		{"auth_token", true},
		{"bearer", true},
		{"session", true},
		{"cookie", true},
		{"encryption_key", true},
		{"private_key", true},
		{"name", false},
		{"email", false},
		{"user_id", false},
		{"created_at", false},
		{"status", false},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			result := redactor.IsSensitiveField(tt.field)
			if result != tt.expected {
				t.Errorf("IsSensitiveField(%q) = %v, want %v", tt.field, result, tt.expected)
			}
		})
	}
}

func TestRedactor_RedactQuery(t *testing.T) {
	redactor := NewRedactor(DefaultRedactorConfig())

	tests := []struct {
		name     string
		query    string
		args     []any
		wantArgs []any
	}{
		{
			name:     "no sensitive fields",
			query:    "SELECT id, name FROM users WHERE id = $1",
			args:     []any{123},
			wantArgs: []any{123},
		},
		{
			name:     "password field",
			query:    "INSERT INTO users (name, password) VALUES ($1, $2)",
			args:     []any{"john", "secret123"},
			wantArgs: []any{"john", "[REDACTED]"},
		},
		{
			name:     "token field",
			query:    "UPDATE users SET token = $1 WHERE id = $2",
			args:     []any{"abc123token", 1},
			wantArgs: []any{"[REDACTED]", 1},
		},
		{
			name:     "multiple sensitive fields",
			query:    "INSERT INTO credentials (name, password, api_key) VALUES ($1, $2, $3)",
			args:     []any{"service", "pass", "key123"},
			wantArgs: []any{"service", "[REDACTED]", "[REDACTED]"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, gotArgs := redactor.RedactQuery(tt.query, tt.args)

			if len(gotArgs) != len(tt.wantArgs) {
				t.Errorf("RedactQuery() args length = %d, want %d", len(gotArgs), len(tt.wantArgs))
				return
			}

			for i, want := range tt.wantArgs {
				if gotArgs[i] != want {
					t.Errorf("RedactQuery() args[%d] = %v, want %v", i, gotArgs[i], want)
				}
			}
		})
	}
}

func TestRedactor_HashQuery(t *testing.T) {
	redactor := NewRedactor(DefaultRedactorConfig())

	// Same logical query should produce same hash
	query1 := "SELECT * FROM users WHERE id = $1"
	query2 := "SELECT * FROM users WHERE id = $2"
	query3 := "SELECT * FROM users WHERE id = $99"

	hash1 := redactor.HashQuery(query1)
	hash2 := redactor.HashQuery(query2)
	hash3 := redactor.HashQuery(query3)

	if hash1 != hash2 || hash2 != hash3 {
		t.Errorf("Same query structure should produce same hash: %s, %s, %s", hash1, hash2, hash3)
	}

	// Different query should produce different hash
	query4 := "SELECT * FROM orders WHERE id = $1"
	hash4 := redactor.HashQuery(query4)

	if hash1 == hash4 {
		t.Error("Different queries should produce different hashes")
	}

	// Whitespace normalization
	query5 := "  SELECT   *   FROM   users   WHERE   id = $1  "
	hash5 := redactor.HashQuery(query5)

	if hash1 != hash5 {
		t.Error("Query hash should normalize whitespace")
	}
}

func TestRedactor_RedactMetadata(t *testing.T) {
	redactor := NewRedactor(DefaultRedactorConfig())

	tests := []struct {
		name     string
		metadata map[string]any
		expected map[string]any
	}{
		{
			name:     "nil metadata",
			metadata: nil,
			expected: nil,
		},
		{
			name: "no sensitive fields",
			metadata: map[string]any{
				"user_id": 123,
				"action":  "login",
			},
			expected: map[string]any{
				"user_id": 123,
				"action":  "login",
			},
		},
		{
			name: "sensitive field",
			metadata: map[string]any{
				"user_id":  123,
				"password": "secret",
			},
			expected: map[string]any{
				"user_id":  123,
				"password": "[REDACTED]",
			},
		},
		{
			name: "nested sensitive field",
			metadata: map[string]any{
				"user": map[string]any{
					"id":       123,
					"password": "secret",
				},
			},
			expected: map[string]any{
				"user": map[string]any{
					"id":       123,
					"password": "[REDACTED]",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactor.RedactMetadata(tt.metadata)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("Expected nil, got %v", result)
				}
				return
			}

			for k, v := range tt.expected {
				// Check nested maps specially
				if nested, ok := v.(map[string]any); ok {
					resultNested, ok := result[k].(map[string]any)
					if !ok {
						t.Errorf("Expected nested map for key %s", k)
						continue
					}
					for nk, nv := range nested {
						if resultNested[nk] != nv {
							t.Errorf("Nested key %s.%s = %v, want %v", k, nk, resultNested[nk], nv)
						}
					}
				} else if result[k] != v {
					t.Errorf("Key %s = %v, want %v", k, result[k], v)
				}
			}
		})
	}
}

func TestRedactor_CustomConfig(t *testing.T) {
	cfg := RedactorConfig{
		SensitiveFields:      []string{"ssn", "credit_card"},
		ParameterPlaceholder: "***",
	}
	redactor := NewRedactor(cfg)

	// Custom fields should be sensitive
	if !redactor.IsSensitiveField("ssn") {
		t.Error("ssn should be sensitive")
	}
	if !redactor.IsSensitiveField("credit_card") {
		t.Error("credit_card should be sensitive")
	}

	// Default fields should not be sensitive with custom config
	if redactor.IsSensitiveField("password") {
		t.Error("password should not be sensitive with custom config")
	}

	// Custom placeholder
	_, args := redactor.RedactQuery("SELECT * FROM users WHERE ssn = $1", []any{"123-45-6789"})
	if args[0] != "***" {
		t.Errorf("Expected custom placeholder ***, got %v", args[0])
	}
}
