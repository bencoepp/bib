package domain

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// UserID is a unique identifier for a user, derived from their public key.
type UserID string

// String returns the string representation.
func (id UserID) String() string {
	return string(id)
}

// UserStatus represents the status of a user account.
type UserStatus string

const (
	// UserStatusActive indicates the user is active and can authenticate.
	UserStatusActive UserStatus = "active"

	// UserStatusPending indicates the user is pending approval.
	UserStatusPending UserStatus = "pending"

	// UserStatusSuspended indicates the user is suspended.
	UserStatusSuspended UserStatus = "suspended"

	// UserStatusDeleted indicates the user is deleted (soft delete).
	UserStatusDeleted UserStatus = "deleted"
)

// IsValid returns true if the status is valid.
func (s UserStatus) IsValid() bool {
	switch s {
	case UserStatusActive, UserStatusPending, UserStatusSuspended, UserStatusDeleted:
		return true
	default:
		return false
	}
}

// UserRole represents the role/permission level of a user.
type UserRole string

const (
	// UserRoleAdmin has full administrative access.
	UserRoleAdmin UserRole = "admin"

	// UserRoleUser has standard user access.
	UserRoleUser UserRole = "user"

	// UserRoleReadonly has read-only access.
	UserRoleReadonly UserRole = "readonly"
)

// IsValid returns true if the role is valid.
func (r UserRole) IsValid() bool {
	switch r {
	case UserRoleAdmin, UserRoleUser, UserRoleReadonly:
		return true
	default:
		return false
	}
}

// KeyType represents the type of public key.
type KeyType string

const (
	// KeyTypeEd25519 is an Ed25519 public key.
	KeyTypeEd25519 KeyType = "ed25519"

	// KeyTypeRSA is an RSA public key (for SSH compatibility).
	KeyTypeRSA KeyType = "rsa"
)

// User represents a bib user with cryptographic identity.
// User identity is independent from bibd node identity.
type User struct {
	// ID is the unique identifier derived from the public key.
	ID UserID `json:"id"`

	// PublicKey is the public key bytes (Ed25519 or RSA).
	PublicKey []byte `json:"public_key"`

	// KeyType is the type of public key.
	KeyType KeyType `json:"key_type"`

	// PublicKeyFingerprint is the SHA256 fingerprint of the public key.
	PublicKeyFingerprint string `json:"public_key_fingerprint"`

	// Name is the human-readable display name.
	Name string `json:"name"`

	// Email is the optional contact email.
	Email string `json:"email,omitempty"`

	// Status is the account status.
	Status UserStatus `json:"status"`

	// Role is the user's permission level.
	Role UserRole `json:"role"`

	// Locale is the user's preferred locale for i18n.
	Locale string `json:"locale,omitempty"`

	// CreatedAt is when the user identity was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the user profile was last modified.
	UpdatedAt time.Time `json:"updated_at"`

	// LastLoginAt is when the user last authenticated.
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`

	// Metadata holds additional user profile data.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Validate validates the user.
func (u *User) Validate() error {
	if u.ID == "" {
		return ErrInvalidUserID
	}
	if len(u.PublicKey) == 0 {
		return ErrInvalidPublicKey
	}
	// Validate key size based on type
	switch u.KeyType {
	case KeyTypeEd25519:
		if len(u.PublicKey) != ed25519.PublicKeySize {
			return ErrInvalidPublicKey
		}
	case KeyTypeRSA:
		// RSA keys have variable size, just check it's not empty
		if len(u.PublicKey) < 64 {
			return ErrInvalidPublicKey
		}
	default:
		return ErrInvalidKeyType
	}
	if u.Name == "" {
		return ErrInvalidUserName
	}
	if !u.Status.IsValid() {
		return ErrInvalidUserStatus
	}
	if !u.Role.IsValid() {
		return ErrInvalidUserRole
	}
	return nil
}

// VerifySignature verifies a signature against a message using the user's public key.
// Only supported for Ed25519 keys.
func (u *User) VerifySignature(message, signature []byte) bool {
	if u.KeyType != KeyTypeEd25519 {
		return false
	}
	if len(u.PublicKey) != ed25519.PublicKeySize {
		return false
	}
	return ed25519.Verify(u.PublicKey, message, signature)
}

// IsAdmin returns true if the user has admin role.
func (u *User) IsAdmin() bool {
	return u.Role == UserRoleAdmin
}

// CanWrite returns true if the user can write (admin or user role).
func (u *User) CanWrite() bool {
	return u.Role == UserRoleAdmin || u.Role == UserRoleUser
}

// IsActive returns true if the user account is active.
func (u *User) IsActive() bool {
	return u.Status == UserStatusActive
}

// UserIDFromPublicKey derives a UserID from a public key.
// The ID is the hex-encoded SHA256 hash of the public key (first 20 bytes).
func UserIDFromPublicKey(pubKey []byte) UserID {
	if len(pubKey) == 0 {
		return ""
	}
	hash := sha256.Sum256(pubKey)
	return UserID(hex.EncodeToString(hash[:20]))
}

// PublicKeyFingerprint returns the SHA256 fingerprint of a public key.
func PublicKeyFingerprint(pubKey []byte) string {
	if len(pubKey) == 0 {
		return ""
	}
	hash := sha256.Sum256(pubKey)
	return hex.EncodeToString(hash[:])
}

// NewUser creates a new user from a public key.
// The first user is automatically an admin; subsequent users are regular users.
func NewUser(publicKey []byte, keyType KeyType, name, email string, isFirstUser bool) *User {
	now := time.Now().UTC()
	role := UserRoleUser
	if isFirstUser {
		role = UserRoleAdmin
	}

	return &User{
		ID:                   UserIDFromPublicKey(publicKey),
		PublicKey:            publicKey,
		KeyType:              keyType,
		PublicKeyFingerprint: PublicKeyFingerprint(publicKey),
		Name:                 name,
		Email:                email,
		Status:               UserStatusActive,
		Role:                 role,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
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
