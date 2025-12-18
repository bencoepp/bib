package encryption

import (
	"crypto/sha256"
	"fmt"
	"io"
	"time"

	"golang.org/x/crypto/hkdf"
)

// KeyManager handles encryption key management and derivation.
type KeyManager struct {
	masterKey []byte
	config    RecoveryConfig
	shamir    *ShamirManager
}

// NewKeyManager creates a new key manager.
func NewKeyManager(identityKey []byte, config RecoveryConfig) (*KeyManager, error) {
	if len(identityKey) < 32 {
		return nil, ErrInvalidKey
	}

	// Derive master key from identity key using HKDF
	masterKey := deriveKey(identityKey, "bibd-master-encryption-key", 32)

	km := &KeyManager{
		masterKey: masterKey,
		config:    config,
	}

	// Set up Shamir's Secret Sharing if configured
	if config.Method == "shamir" {
		km.shamir = NewShamirManager(config.Shamir)
	}

	return km, nil
}

// DeriveKey derives a purpose-specific key from the master key.
func (km *KeyManager) DeriveKey(purpose string) []byte {
	return deriveKey(km.masterKey, purpose, 32)
}

// DeriveKeyWithSize derives a key of specific size.
func (km *KeyManager) DeriveKeyWithSize(purpose string, size int) []byte {
	return deriveKey(km.masterKey, purpose, size)
}

// GenerateRecoveryShares creates Shamir's Secret Sharing shares for key recovery.
func (km *KeyManager) GenerateRecoveryShares() ([]Share, error) {
	if km.shamir == nil {
		return nil, fmt.Errorf("Shamir's Secret Sharing not configured")
	}

	return km.shamir.SplitKey(km.masterKey)
}

// RecoverFromShares recovers the master key from shares.
func (km *KeyManager) RecoverFromShares(shares []Share) error {
	if km.shamir == nil {
		return fmt.Errorf("Shamir's Secret Sharing not configured")
	}

	recoveredKey, err := km.shamir.RecoverKey(shares)
	if err != nil {
		return err
	}

	km.masterKey = recoveredKey
	return nil
}

// ExportShare exports a share in a portable format.
func (km *KeyManager) ExportShare(share Share, recipientPublicKey []byte) (string, error) {
	return share.Export(recipientPublicKey)
}

// ImportShare imports a share from a portable format.
func (km *KeyManager) ImportShare(encoded string, privateKey []byte) (*Share, error) {
	return ImportShare(encoded, privateKey)
}

// RotateMasterKey rotates the master key.
// This requires re-encrypting all data with the new key.
func (km *KeyManager) RotateMasterKey(newIdentityKey []byte) ([]byte, error) {
	oldKey := km.masterKey
	newKey := deriveKey(newIdentityKey, "bibd-master-encryption-key", 32)

	km.masterKey = newKey
	return oldKey, nil
}

// VerifyKey verifies the master key is correct by checking a known value.
func (km *KeyManager) VerifyKey(knownHash []byte) bool {
	h := sha256.Sum256(km.masterKey)
	if len(knownHash) != len(h) {
		return false
	}
	for i := range h {
		if h[i] != knownHash[i] {
			return false
		}
	}
	return true
}

// KeyHash returns a hash of the master key for verification.
func (km *KeyManager) KeyHash() []byte {
	h := sha256.Sum256(km.masterKey)
	return h[:]
}

// deriveKey derives a key using HKDF.
func deriveKey(secret []byte, info string, size int) []byte {
	salt := []byte("bibd-encryption-v1")
	reader := hkdf.New(sha256.New, secret, salt, []byte(info))
	key := make([]byte, size)
	io.ReadFull(reader, key)
	return key
}

// KeyInfo holds information about an encryption key.
type KeyInfo struct {
	// ID is a unique identifier for the key.
	ID string `json:"id"`

	// Purpose describes what the key is used for.
	Purpose string `json:"purpose"`

	// CreatedAt is when the key was created.
	CreatedAt time.Time `json:"created_at"`

	// ExpiresAt is when the key expires (if applicable).
	ExpiresAt *time.Time `json:"expires_at,omitempty"`

	// Algorithm is the encryption algorithm.
	Algorithm string `json:"algorithm"`

	// Version is the key version for rotation tracking.
	Version int `json:"version"`
}

// KeyStore interface for persistent key storage.
type KeyStore interface {
	// Store stores an encrypted key.
	Store(id string, encryptedKey []byte, info KeyInfo) error

	// Load loads an encrypted key.
	Load(id string) ([]byte, KeyInfo, error)

	// Delete removes a key.
	Delete(id string) error

	// List returns all key IDs.
	List() ([]string, error)
}
