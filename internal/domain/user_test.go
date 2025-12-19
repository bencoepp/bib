package domain

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"
)

func TestUserID_String(t *testing.T) {
	id := UserID("user-123")
	if id.String() != "user-123" {
		t.Errorf("expected 'user-123', got %q", id.String())
	}
}

func TestUser_Validate(t *testing.T) {
	// Generate a valid Ed25519 public key for testing
	validPubKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	tests := []struct {
		name    string
		user    *User
		wantErr error
	}{
		{
			name: "valid user",
			user: &User{
				ID:        "user-123",
				PublicKey: validPubKey,
				KeyType:   KeyTypeEd25519,
				Name:      "Test User",
				Status:    UserStatusActive,
				Role:      UserRoleUser,
				CreatedAt: time.Now(),
			},
			wantErr: nil,
		},
		{
			name: "empty ID",
			user: &User{
				ID:        "",
				PublicKey: validPubKey,
				KeyType:   KeyTypeEd25519,
				Name:      "Test User",
				Status:    UserStatusActive,
				Role:      UserRoleUser,
			},
			wantErr: ErrInvalidUserID,
		},
		{
			name: "invalid public key length",
			user: &User{
				ID:        "user-123",
				PublicKey: []byte("too-short"),
				KeyType:   KeyTypeEd25519,
				Name:      "Test User",
				Status:    UserStatusActive,
				Role:      UserRoleUser,
			},
			wantErr: ErrInvalidPublicKey,
		},
		{
			name: "empty name",
			user: &User{
				ID:        "user-123",
				PublicKey: validPubKey,
				KeyType:   KeyTypeEd25519,
				Name:      "",
				Status:    UserStatusActive,
				Role:      UserRoleUser,
			},
			wantErr: ErrInvalidUserName,
		},
		{
			name: "invalid status",
			user: &User{
				ID:        "user-123",
				PublicKey: validPubKey,
				KeyType:   KeyTypeEd25519,
				Name:      "Test User",
				Status:    "invalid",
				Role:      UserRoleUser,
			},
			wantErr: ErrInvalidUserStatus,
		},
		{
			name: "invalid role",
			user: &User{
				ID:        "user-123",
				PublicKey: validPubKey,
				KeyType:   KeyTypeEd25519,
				Name:      "Test User",
				Status:    UserStatusActive,
				Role:      "invalid",
			},
			wantErr: ErrInvalidUserRole,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.user.Validate()
			if err != tt.wantErr {
				t.Errorf("User.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUser_VerifySignature(t *testing.T) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	user := &User{
		ID:        "user-123",
		PublicKey: pubKey,
		KeyType:   KeyTypeEd25519,
		Name:      "Test User",
		Status:    UserStatusActive,
		Role:      UserRoleUser,
	}

	message := []byte("test message")
	signature := ed25519.Sign(privKey, message)

	t.Run("valid signature", func(t *testing.T) {
		if !user.VerifySignature(message, signature) {
			t.Error("expected valid signature verification")
		}
	})

	t.Run("invalid signature", func(t *testing.T) {
		invalidSig := make([]byte, len(signature))
		copy(invalidSig, signature)
		invalidSig[0] ^= 0xFF // Flip bits

		if user.VerifySignature(message, invalidSig) {
			t.Error("expected invalid signature verification")
		}
	})

	t.Run("wrong message", func(t *testing.T) {
		wrongMessage := []byte("wrong message")
		if user.VerifySignature(wrongMessage, signature) {
			t.Error("expected invalid signature for wrong message")
		}
	})

	t.Run("invalid public key", func(t *testing.T) {
		invalidUser := &User{
			ID:        "user-123",
			PublicKey: []byte("invalid"),
			KeyType:   KeyTypeEd25519,
			Name:      "Test User",
			Status:    UserStatusActive,
			Role:      UserRoleUser,
		}
		if invalidUser.VerifySignature(message, signature) {
			t.Error("expected false for invalid public key")
		}
	})

	t.Run("rsa key type returns false", func(t *testing.T) {
		rsaUser := &User{
			ID:        "user-123",
			PublicKey: pubKey,
			KeyType:   KeyTypeRSA,
			Name:      "Test User",
			Status:    UserStatusActive,
			Role:      UserRoleUser,
		}
		if rsaUser.VerifySignature(message, signature) {
			t.Error("expected false for RSA key type (not supported for VerifySignature)")
		}
	})
}

func TestUserIDFromPublicKey(t *testing.T) {
	pubKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	t.Run("valid public key", func(t *testing.T) {
		userID := UserIDFromPublicKey(pubKey)
		if userID == "" {
			t.Error("expected non-empty user ID")
		}
		// Should be 40 hex characters (20 bytes * 2) from SHA256 hash
		if len(userID) != 40 {
			t.Errorf("expected 40 character user ID, got %d", len(userID))
		}
	})

	t.Run("empty key", func(t *testing.T) {
		userID := UserIDFromPublicKey(nil)
		if userID != "" {
			t.Error("expected empty user ID for nil key")
		}
		userID = UserIDFromPublicKey([]byte{})
		if userID != "" {
			t.Error("expected empty user ID for empty key")
		}
	})

	t.Run("deterministic", func(t *testing.T) {
		id1 := UserIDFromPublicKey(pubKey)
		id2 := UserIDFromPublicKey(pubKey)
		if id1 != id2 {
			t.Error("expected same user ID for same public key")
		}
	})

	t.Run("short key still works", func(t *testing.T) {
		// The new implementation uses SHA256 so any length key works
		shortKey := []byte("short")
		userID := UserIDFromPublicKey(shortKey)
		if userID == "" {
			t.Error("expected non-empty user ID for short key (SHA256 hashes any input)")
		}
	})
}

func TestSignedOperation_Validate(t *testing.T) {
	validSig := make([]byte, ed25519.SignatureSize)

	tests := []struct {
		name    string
		op      *SignedOperation
		wantErr error
	}{
		{
			name: "valid operation",
			op: &SignedOperation{
				UserID:    "user-123",
				Operation: "create_topic",
				Payload:   []byte(`{"name":"test"}`),
				Timestamp: time.Now(),
				Signature: validSig,
			},
			wantErr: nil,
		},
		{
			name: "empty user ID",
			op: &SignedOperation{
				UserID:    "",
				Operation: "create_topic",
				Signature: validSig,
			},
			wantErr: ErrInvalidUserID,
		},
		{
			name: "empty operation",
			op: &SignedOperation{
				UserID:    "user-123",
				Operation: "",
				Signature: validSig,
			},
			wantErr: ErrInvalidOperation,
		},
		{
			name: "invalid signature length",
			op: &SignedOperation{
				UserID:    "user-123",
				Operation: "create_topic",
				Signature: []byte("short"),
			},
			wantErr: ErrInvalidSignature,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.op.Validate()
			if err != tt.wantErr {
				t.Errorf("SignedOperation.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewUser(t *testing.T) {
	pubKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	t.Run("first user is admin", func(t *testing.T) {
		user := NewUser(pubKey, KeyTypeEd25519, "First User", "first@example.com", true)
		if user.Role != UserRoleAdmin {
			t.Errorf("expected first user to be admin, got %s", user.Role)
		}
		if user.Status != UserStatusActive {
			t.Errorf("expected active status, got %s", user.Status)
		}
	})

	t.Run("subsequent user is regular user", func(t *testing.T) {
		user := NewUser(pubKey, KeyTypeEd25519, "Second User", "second@example.com", false)
		if user.Role != UserRoleUser {
			t.Errorf("expected regular user role, got %s", user.Role)
		}
	})

	t.Run("generates correct ID and fingerprint", func(t *testing.T) {
		user := NewUser(pubKey, KeyTypeEd25519, "Test User", "test@example.com", false)
		expectedID := UserIDFromPublicKey(pubKey)
		if user.ID != expectedID {
			t.Errorf("expected ID %s, got %s", expectedID, user.ID)
		}
		expectedFP := PublicKeyFingerprint(pubKey)
		if user.PublicKeyFingerprint != expectedFP {
			t.Errorf("expected fingerprint %s, got %s", expectedFP, user.PublicKeyFingerprint)
		}
	})
}

func TestUserStatus_IsValid(t *testing.T) {
	tests := []struct {
		status UserStatus
		valid  bool
	}{
		{UserStatusActive, true},
		{UserStatusPending, true},
		{UserStatusSuspended, true},
		{UserStatusDeleted, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := tt.status.IsValid(); got != tt.valid {
			t.Errorf("UserStatus(%q).IsValid() = %v, want %v", tt.status, got, tt.valid)
		}
	}
}

func TestUserRole_IsValid(t *testing.T) {
	tests := []struct {
		role  UserRole
		valid bool
	}{
		{UserRoleAdmin, true},
		{UserRoleUser, true},
		{UserRoleReadonly, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := tt.role.IsValid(); got != tt.valid {
			t.Errorf("UserRole(%q).IsValid() = %v, want %v", tt.role, got, tt.valid)
		}
	}
}

func TestUser_Helpers(t *testing.T) {
	pubKey, _, _ := ed25519.GenerateKey(rand.Reader)

	t.Run("IsAdmin", func(t *testing.T) {
		admin := NewUser(pubKey, KeyTypeEd25519, "Admin", "", true)
		if !admin.IsAdmin() {
			t.Error("expected IsAdmin to return true for admin")
		}

		user := NewUser(pubKey, KeyTypeEd25519, "User", "", false)
		if user.IsAdmin() {
			t.Error("expected IsAdmin to return false for regular user")
		}
	})

	t.Run("CanWrite", func(t *testing.T) {
		admin := NewUser(pubKey, KeyTypeEd25519, "Admin", "", true)
		if !admin.CanWrite() {
			t.Error("expected admin to be able to write")
		}

		user := NewUser(pubKey, KeyTypeEd25519, "User", "", false)
		if !user.CanWrite() {
			t.Error("expected user to be able to write")
		}

		readonly := NewUser(pubKey, KeyTypeEd25519, "Readonly", "", false)
		readonly.Role = UserRoleReadonly
		if readonly.CanWrite() {
			t.Error("expected readonly user to not be able to write")
		}
	})

	t.Run("IsActive", func(t *testing.T) {
		user := NewUser(pubKey, KeyTypeEd25519, "User", "", false)
		if !user.IsActive() {
			t.Error("expected new user to be active")
		}

		user.Status = UserStatusSuspended
		if user.IsActive() {
			t.Error("expected suspended user to not be active")
		}
	})
}

func TestPublicKeyFingerprint(t *testing.T) {
	pubKey, _, _ := ed25519.GenerateKey(rand.Reader)

	t.Run("returns SHA256 hex", func(t *testing.T) {
		fp := PublicKeyFingerprint(pubKey)
		// SHA256 = 32 bytes = 64 hex chars
		if len(fp) != 64 {
			t.Errorf("expected 64 character fingerprint, got %d", len(fp))
		}
	})

	t.Run("deterministic", func(t *testing.T) {
		fp1 := PublicKeyFingerprint(pubKey)
		fp2 := PublicKeyFingerprint(pubKey)
		if fp1 != fp2 {
			t.Error("expected same fingerprint for same key")
		}
	})

	t.Run("empty key returns empty", func(t *testing.T) {
		fp := PublicKeyFingerprint(nil)
		if fp != "" {
			t.Error("expected empty fingerprint for nil key")
		}
	})
}
