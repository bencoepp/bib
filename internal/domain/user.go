package domain

import (
	"crypto/ed25519"
	"encoding/hex"
	"time"
)

// UserID is a unique identifier for a user, derived from their public key.
type UserID string

// String returns the string representation.
func (id UserID) String() string {
	return string(id)
}

// User represents a bib user with cryptographic identity.
// User identity is independent from bibd node identity.
type User struct {
	// ID is the unique identifier derived from the public key.
	ID UserID `json:"id"`

	// PublicKey is the Ed25519 public key bytes.
	PublicKey []byte `json:"public_key"`

	// Name is the human-readable display name.
	Name string `json:"name"`

	// Email is the optional contact email.
	Email string `json:"email,omitempty"`

	// CreatedAt is when the user identity was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the user profile was last modified.
	UpdatedAt time.Time `json:"updated_at"`

	// Metadata holds additional user profile data.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Validate validates the user.
func (u *User) Validate() error {
	if u.ID == "" {
		return ErrInvalidUserID
	}
	if len(u.PublicKey) != ed25519.PublicKeySize {
		return ErrInvalidPublicKey
	}
	if u.Name == "" {
		return ErrInvalidUserName
	}
	return nil
}

// VerifySignature verifies a signature against a message using the user's public key.
func (u *User) VerifySignature(message, signature []byte) bool {
	if len(u.PublicKey) != ed25519.PublicKeySize {
		return false
	}
	return ed25519.Verify(u.PublicKey, message, signature)
}

// UserIDFromPublicKey derives a UserID from a public key.
// The ID is the hex-encoded first 20 bytes of the public key.
func UserIDFromPublicKey(pubKey []byte) UserID {
	if len(pubKey) < 20 {
		return ""
	}
	return UserID(hex.EncodeToString(pubKey[:20]))
}

// SignedOperation represents an operation signed by a user.
type SignedOperation struct {
	// UserID is the user who signed the operation.
	UserID UserID `json:"user_id"`

	// Operation is the operation type.
	Operation string `json:"operation"`

	// Payload is the operation payload (JSON encoded).
	Payload []byte `json:"payload"`

	// Timestamp is when the operation was signed.
	Timestamp time.Time `json:"timestamp"`

	// Signature is the Ed25519 signature of the operation.
	Signature []byte `json:"signature"`
}

// Validate validates the signed operation.
func (s *SignedOperation) Validate() error {
	if s.UserID == "" {
		return ErrInvalidUserID
	}
	if s.Operation == "" {
		return ErrInvalidOperation
	}
	if len(s.Signature) != ed25519.SignatureSize {
		return ErrInvalidSignature
	}
	return nil
}
