package p2p

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/libp2p/go-libp2p/core/crypto"
)

const (
	// PEMTypeEd25519Private is the PEM block type for Ed25519 private keys.
	PEMTypeEd25519Private = "ED25519 PRIVATE KEY"

	// DefaultIdentityFileName is the default filename for the identity key.
	DefaultIdentityFileName = "identity.pem"
)

var (
	// ErrIdentityNotFound indicates the identity key file does not exist.
	ErrIdentityNotFound = errors.New("identity key not found")

	// ErrInvalidPEMBlock indicates the PEM file has an invalid or missing block.
	ErrInvalidPEMBlock = errors.New("invalid or missing PEM block")

	// ErrInvalidKeyType indicates the PEM block has an unexpected type.
	ErrInvalidKeyType = errors.New("invalid key type in PEM block")

	// ErrInvalidKeyLength indicates the key has an unexpected length.
	ErrInvalidKeyLength = errors.New("invalid key length")
)

// Identity represents a node's cryptographic identity.
type Identity struct {
	// PrivKey is the libp2p private key.
	PrivKey crypto.PrivKey
}

// RawPrivateKey returns the raw bytes of the private key.
// This can be used for deriving encryption keys (e.g., for CA key encryption).
func (i *Identity) RawPrivateKey() ([]byte, error) {
	return i.PrivKey.Raw()
}

// LoadIdentity loads the node identity from the specified key path.
// If keyPath is empty, it defaults to configDir/identity.pem.
func LoadIdentity(keyPath, configDir string) (*Identity, error) {
	path := resolveKeyPath(keyPath, configDir)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrIdentityNotFound, path)
		}
		return nil, fmt.Errorf("failed to read identity key: %w", err)
	}

	privKey, err := parsePrivateKeyPEM(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse identity key: %w", err)
	}

	return &Identity{PrivKey: privKey}, nil
}

// GenerateIdentity creates a new Ed25519 identity and saves it to the specified path.
// If keyPath is empty, it defaults to configDir/identity.pem.
// If force is true, an existing key file will be overwritten.
func GenerateIdentity(keyPath, configDir string, force bool) (*Identity, error) {
	path := resolveKeyPath(keyPath, configDir)

	// Check if file already exists
	if !force {
		if _, err := os.Stat(path); err == nil {
			return nil, fmt.Errorf("identity key already exists at %s (use --force to overwrite)", path)
		}
	}

	// Generate new Ed25519 keypair
	privKey, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Ed25519 key: %w", err)
	}

	// Save to file
	if err := savePrivateKeyPEM(path, privKey); err != nil {
		return nil, err
	}

	return &Identity{PrivKey: privKey}, nil
}

// LoadOrGenerateIdentity loads an existing identity or generates a new one if it doesn't exist.
// If keyPath is empty, it defaults to configDir/identity.pem.
func LoadOrGenerateIdentity(keyPath, configDir string) (*Identity, error) {
	identity, err := LoadIdentity(keyPath, configDir)
	if err == nil {
		return identity, nil
	}

	if !errors.Is(err, ErrIdentityNotFound) {
		return nil, err
	}

	// Identity doesn't exist, generate a new one
	return GenerateIdentity(keyPath, configDir, false)
}

// resolveKeyPath returns the full path to the identity key file.
func resolveKeyPath(keyPath, configDir string) string {
	if keyPath != "" {
		return keyPath
	}
	return filepath.Join(configDir, DefaultIdentityFileName)
}

// parsePrivateKeyPEM parses a PEM-encoded Ed25519 private key.
func parsePrivateKeyPEM(data []byte) (crypto.PrivKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, ErrInvalidPEMBlock
	}

	if block.Type != PEMTypeEd25519Private {
		return nil, fmt.Errorf("%w: expected %s, got %s", ErrInvalidKeyType, PEMTypeEd25519Private, block.Type)
	}

	// Ed25519 private keys are 64 bytes (32-byte seed + 32-byte public key)
	// Or 32 bytes if just the seed is stored
	var seed []byte
	switch len(block.Bytes) {
	case ed25519.SeedSize:
		seed = block.Bytes
	case ed25519.PrivateKeySize:
		seed = block.Bytes[:ed25519.SeedSize]
	default:
		return nil, fmt.Errorf("%w: expected %d or %d bytes, got %d",
			ErrInvalidKeyLength, ed25519.SeedSize, ed25519.PrivateKeySize, len(block.Bytes))
	}

	// Reconstruct the full private key from the seed
	stdPrivKey := ed25519.NewKeyFromSeed(seed)

	// Convert to libp2p key
	privKey, _, err := crypto.KeyPairFromStdKey(&stdPrivKey)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to libp2p key: %w", err)
	}

	return privKey, nil
}

// savePrivateKeyPEM saves an Ed25519 private key in PEM format.
func savePrivateKeyPEM(path string, privKey crypto.PrivKey) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Extract the raw key bytes
	raw, err := privKey.Raw()
	if err != nil {
		return fmt.Errorf("failed to get raw key bytes: %w", err)
	}

	// Create PEM block
	block := &pem.Block{
		Type:  PEMTypeEd25519Private,
		Bytes: raw,
	}

	// Write to file with restrictive permissions
	data := pem.EncodeToMemory(block)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write identity key: %w", err)
	}

	return nil
}

// KeyPath returns the resolved path to the identity key file.
func KeyPath(keyPath, configDir string) string {
	return resolveKeyPath(keyPath, configDir)
}
