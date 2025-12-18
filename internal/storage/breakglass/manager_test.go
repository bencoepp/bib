package breakglass

import (
	"context"
	"crypto/ed25519"
	"testing"
	"time"
)

func TestAccessLevelIsValid(t *testing.T) {
	tests := []struct {
		level AccessLevel
		valid bool
	}{
		{AccessReadOnly, true},
		{AccessReadWrite, true},
		{AccessLevel("invalid"), false},
		{AccessLevel(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			if got := tt.level.IsValid(); got != tt.valid {
				t.Errorf("AccessLevel(%q).IsValid() = %v, want %v", tt.level, got, tt.valid)
			}
		})
	}
}

func TestSessionIsActive(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		session  Session
		expected bool
	}{
		{
			name: "active session",
			session: Session{
				State:     StateActive,
				ExpiresAt: now.Add(1 * time.Hour),
			},
			expected: true,
		},
		{
			name: "expired session",
			session: Session{
				State:     StateActive,
				ExpiresAt: now.Add(-1 * time.Hour),
			},
			expected: false,
		},
		{
			name: "disabled session",
			session: Session{
				State:     StatePendingAck,
				ExpiresAt: now.Add(1 * time.Hour),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.session.IsActive(); got != tt.expected {
				t.Errorf("Session.IsActive() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestChallengeIsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt time.Time
		expected  bool
	}{
		{
			name:      "not expired",
			expiresAt: time.Now().Add(5 * time.Minute),
			expected:  false,
		},
		{
			name:      "expired",
			expiresAt: time.Now().Add(-1 * time.Minute),
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Challenge{ExpiresAt: tt.expiresAt}
			if got := c.IsExpired(); got != tt.expected {
				t.Errorf("Challenge.IsExpired() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestManagerCreateChallenge(t *testing.T) {
	// Generate test key
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	config := Config{
		Enabled:            true,
		MaxDuration:        1 * time.Hour,
		DefaultAccessLevel: AccessReadOnly,
		AllowedUsers: []User{
			{
				Name:            "test_user",
				PublicKey:       pub,
				PublicKeyString: "ssh-ed25519 test",
			},
		},
	}

	manager := NewManager(config, "test-node")

	t.Run("create challenge for valid user", func(t *testing.T) {
		challenge, err := manager.CreateChallenge("test_user")
		if err != nil {
			t.Fatalf("CreateChallenge failed: %v", err)
		}
		if challenge.ID == "" {
			t.Error("challenge ID should not be empty")
		}
		if len(challenge.Nonce) != 32 {
			t.Errorf("nonce should be 32 bytes, got %d", len(challenge.Nonce))
		}
		if challenge.Username != "test_user" {
			t.Errorf("username = %q, want %q", challenge.Username, "test_user")
		}
	})

	t.Run("create challenge for unknown user", func(t *testing.T) {
		_, err := manager.CreateChallenge("unknown_user")
		if err == nil {
			t.Error("expected error for unknown user")
		}
	})

	t.Run("create challenge when disabled", func(t *testing.T) {
		disabledManager := NewManager(Config{Enabled: false}, "test-node")
		_, err := disabledManager.CreateChallenge("test_user")
		if err == nil {
			t.Error("expected error when break glass is disabled")
		}
	})
}

func TestManagerVerifyChallenge(t *testing.T) {
	// Generate test key
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	config := Config{
		Enabled:            true,
		MaxDuration:        1 * time.Hour,
		DefaultAccessLevel: AccessReadOnly,
		AllowedUsers: []User{
			{
				Name:            "test_user",
				PublicKey:       pub,
				PublicKeyString: "ssh-ed25519 test",
			},
		},
	}

	manager := NewManager(config, "test-node")

	t.Run("verify valid signature", func(t *testing.T) {
		challenge, err := manager.CreateChallenge("test_user")
		if err != nil {
			t.Fatalf("CreateChallenge failed: %v", err)
		}

		signature := ed25519.Sign(priv, challenge.Nonce)
		user, err := manager.VerifyChallenge(challenge.ID, signature)
		if err != nil {
			t.Fatalf("VerifyChallenge failed: %v", err)
		}
		if user.Name != "test_user" {
			t.Errorf("user.Name = %q, want %q", user.Name, "test_user")
		}
	})

	t.Run("verify invalid signature", func(t *testing.T) {
		challenge, err := manager.CreateChallenge("test_user")
		if err != nil {
			t.Fatalf("CreateChallenge failed: %v", err)
		}

		// Wrong signature
		_, err = manager.VerifyChallenge(challenge.ID, []byte("invalid"))
		if err == nil {
			t.Error("expected error for invalid signature")
		}
	})

	t.Run("verify unknown challenge", func(t *testing.T) {
		_, err := manager.VerifyChallenge("unknown-id", []byte("signature"))
		if err == nil {
			t.Error("expected error for unknown challenge")
		}
	})
}

func TestManagerEnableDisable(t *testing.T) {
	// Generate test key
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	config := Config{
		Enabled:               true,
		MaxDuration:           1 * time.Hour,
		DefaultAccessLevel:    AccessReadOnly,
		RequireAcknowledgment: true,
		AllowedUsers: []User{
			{
				Name:            "test_user",
				PublicKey:       pub,
				PublicKeyString: "ssh-ed25519 test",
			},
		},
	}

	manager := NewManager(config, "test-node")

	t.Run("enable session", func(t *testing.T) {
		user := &config.AllowedUsers[0]
		session, err := manager.Enable(context.Background(), user, "test reason", 30*time.Minute, "admin")
		if err != nil {
			t.Fatalf("Enable failed: %v", err)
		}
		if session.ID == "" {
			t.Error("session ID should not be empty")
		}
		if !manager.HasActiveSession() {
			t.Error("should have active session")
		}
		if session.Reason != "test reason" {
			t.Errorf("reason = %q, want %q", session.Reason, "test reason")
		}
	})

	t.Run("cannot enable while session active", func(t *testing.T) {
		user := &config.AllowedUsers[0]
		_, err := manager.Enable(context.Background(), user, "second session", 30*time.Minute, "admin")
		if err == nil {
			t.Error("expected error when session already active")
		}
	})

	t.Run("disable session", func(t *testing.T) {
		report, err := manager.Disable(context.Background(), "admin")
		if err != nil {
			t.Fatalf("Disable failed: %v", err)
		}
		if report == nil {
			t.Error("report should not be nil")
		}
		if manager.HasActiveSession() {
			t.Error("should not have active session after disable")
		}
	})

	t.Run("pending acknowledgment", func(t *testing.T) {
		pending := manager.GetPendingReports()
		if len(pending) != 1 {
			t.Fatalf("expected 1 pending report, got %d", len(pending))
		}
	})
}

func TestIsValidUsername(t *testing.T) {
	tests := []struct {
		username string
		valid    bool
	}{
		{"breakglass_abc123", true},
		{"breakglass_ABC", true},
		{"breakglass_a", true},
		{"breakglass_", false},
		{"breakglass_abc-123", false}, // hyphen not allowed
		{"breakglass_abc_123", false}, // underscore after prefix not allowed
		{"other_abc123", false},       // wrong prefix
		{"breakglassabc123", false},   // missing underscore
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.username, func(t *testing.T) {
			if got := isValidUsername(tt.username); got != tt.valid {
				t.Errorf("isValidUsername(%q) = %v, want %v", tt.username, got, tt.valid)
			}
		})
	}
}
