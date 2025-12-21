// Package auth provides authentication and identity management for bib.
package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

// IdentityKey represents a user's identity keypair
type IdentityKey struct {
	// PrivateKey is the Ed25519 private key
	PrivateKey ed25519.PrivateKey

	// PublicKey is the Ed25519 public key
	PublicKey ed25519.PublicKey

	// Path is the filesystem path where the key is stored
	Path string
}

// GenerateIdentityKey generates a new Ed25519 keypair for user identity
func GenerateIdentityKey() (*IdentityKey, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Ed25519 keypair: %w", err)
	}

	return &IdentityKey{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

// LoadIdentityKey loads an existing identity key from a PEM file
func LoadIdentityKey(path string) (*IdentityKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read identity key file: %w", err)
	}

	// Try to parse as OpenSSH private key first
	privateKey, err := ssh.ParseRawPrivateKey(data)
	if err != nil {
		// Try as PEM-encoded PKCS8
		block, _ := pem.Decode(data)
		if block == nil {
			return nil, fmt.Errorf("failed to decode PEM block from identity key")
		}

		// Parse based on block type
		switch block.Type {
		case "OPENSSH PRIVATE KEY":
			privateKey, err = ssh.ParseRawPrivateKey(data)
			if err != nil {
				return nil, fmt.Errorf("failed to parse OpenSSH private key: %w", err)
			}
		default:
			return nil, fmt.Errorf("unsupported key format: %s", block.Type)
		}
	}

	// Extract Ed25519 key
	switch key := privateKey.(type) {
	case *ed25519.PrivateKey:
		return &IdentityKey{
			PrivateKey: *key,
			PublicKey:  key.Public().(ed25519.PublicKey),
			Path:       path,
		}, nil
	case ed25519.PrivateKey:
		return &IdentityKey{
			PrivateKey: key,
			PublicKey:  key.Public().(ed25519.PublicKey),
			Path:       path,
		}, nil
	default:
		return nil, fmt.Errorf("identity key must be Ed25519, got %T", privateKey)
	}
}

// Save saves the identity key to a PEM file in OpenSSH format
func (k *IdentityKey) Save(path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Generate OpenSSH format private key
	pemBlock, err := ssh.MarshalPrivateKey(k.PrivateKey, "")
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	// Write with restrictive permissions (owner read/write only)
	pemData := pem.EncodeToMemory(pemBlock)
	if err := os.WriteFile(path, pemData, 0600); err != nil {
		return fmt.Errorf("failed to write identity key: %w", err)
	}

	k.Path = path
	return nil
}

// Fingerprint returns the SHA256 fingerprint of the public key in SSH format
func (k *IdentityKey) Fingerprint() string {
	sshPubKey, err := ssh.NewPublicKey(k.PublicKey)
	if err != nil {
		return ""
	}
	return ssh.FingerprintSHA256(sshPubKey)
}

// FingerprintLegacy returns the MD5 fingerprint of the public key (legacy format)
func (k *IdentityKey) FingerprintLegacy() string {
	sshPubKey, err := ssh.NewPublicKey(k.PublicKey)
	if err != nil {
		return ""
	}
	return ssh.FingerprintLegacyMD5(sshPubKey)
}

// AuthorizedKey returns the public key in OpenSSH authorized_keys format
func (k *IdentityKey) AuthorizedKey() string {
	sshPubKey, err := ssh.NewPublicKey(k.PublicKey)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPubKey)))
}

// PublicKeyBytes returns the raw public key bytes
func (k *IdentityKey) PublicKeyBytes() []byte {
	return k.PublicKey
}

// Signer returns an ssh.Signer for authentication
func (k *IdentityKey) Signer() (ssh.Signer, error) {
	return ssh.NewSignerFromKey(k.PrivateKey)
}

// Sign signs a message with the private key
func (k *IdentityKey) Sign(message []byte) ([]byte, error) {
	signer, err := k.Signer()
	if err != nil {
		return nil, err
	}

	sig, err := signer.Sign(rand.Reader, message)
	if err != nil {
		return nil, fmt.Errorf("failed to sign message: %w", err)
	}

	return sig.Blob, nil
}

// IdentityKeyExists checks if an identity key exists at the given path
func IdentityKeyExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// DefaultIdentityKeyPath returns the default path for the identity key
func DefaultIdentityKeyPath(appName string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".config", appName, "identity.pem"), nil
}

// LoadOrGenerateIdentityKey loads an existing identity key or generates a new one
func LoadOrGenerateIdentityKey(path string) (*IdentityKey, bool, error) {
	// Try to load existing key
	if IdentityKeyExists(path) {
		key, err := LoadIdentityKey(path)
		if err != nil {
			return nil, false, fmt.Errorf("failed to load existing identity key: %w", err)
		}
		return key, false, nil // false = not newly generated
	}

	// Generate new key
	key, err := GenerateIdentityKey()
	if err != nil {
		return nil, false, err
	}

	// Save it
	if err := key.Save(path); err != nil {
		return nil, false, err
	}

	return key, true, nil // true = newly generated
}

// IdentityInfo contains displayable information about an identity key
type IdentityInfo struct {
	// Path is where the key is stored
	Path string

	// Fingerprint is the SHA256 fingerprint
	Fingerprint string

	// FingerprintMD5 is the legacy MD5 fingerprint
	FingerprintMD5 string

	// PublicKey is the public key in authorized_keys format
	PublicKey string

	// KeyType is always "ed25519" for identity keys
	KeyType string

	// KeySize is always 256 bits for Ed25519
	KeySize int
}

// Info returns displayable information about the identity key
func (k *IdentityKey) Info() *IdentityInfo {
	return &IdentityInfo{
		Path:           k.Path,
		Fingerprint:    k.Fingerprint(),
		FingerprintMD5: k.FingerprintLegacy(),
		PublicKey:      k.AuthorizedKey(),
		KeyType:        "ed25519",
		KeySize:        256,
	}
}
