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
				Name:      "Test User",
				CreatedAt: time.Now(),
			},
			wantErr: nil,
		},
		{
			name: "empty ID",
			user: &User{
				ID:        "",
				PublicKey: validPubKey,
				Name:      "Test User",
			},
			wantErr: ErrInvalidUserID,
		},
		{
			name: "invalid public key length",
			user: &User{
				ID:        "user-123",
				PublicKey: []byte("too-short"),
				Name:      "Test User",
			},
			wantErr: ErrInvalidPublicKey,
		},
		{
			name: "empty name",
			user: &User{
				ID:        "user-123",
				PublicKey: validPubKey,
				Name:      "",
			},
			wantErr: ErrInvalidUserName,
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
		Name:      "Test User",
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
			Name:      "Test User",
		}
		if invalidUser.VerifySignature(message, signature) {
			t.Error("expected false for invalid public key")
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
		// Should be 40 hex characters (20 bytes * 2)
		if len(userID) != 40 {
			t.Errorf("expected 40 character user ID, got %d", len(userID))
		}
	})

	t.Run("public key too short", func(t *testing.T) {
		shortKey := []byte("short")
		userID := UserIDFromPublicKey(shortKey)
		if userID != "" {
			t.Error("expected empty user ID for short key")
		}
	})

	t.Run("deterministic", func(t *testing.T) {
		id1 := UserIDFromPublicKey(pubKey)
		id2 := UserIDFromPublicKey(pubKey)
		if id1 != id2 {
			t.Error("expected same user ID for same public key")
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
