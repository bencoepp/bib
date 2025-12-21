package auth

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGenerateIdentityKey(t *testing.T) {
	key, err := GenerateIdentityKey()
	if err != nil {
		t.Fatalf("failed to generate identity key: %v", err)
	}

	if key.PrivateKey == nil {
		t.Error("private key is nil")
	}

	if key.PublicKey == nil {
		t.Error("public key is nil")
	}

	if len(key.PrivateKey) != ed25519.PrivateKeySize {
		t.Errorf("expected private key size %d, got %d", ed25519.PrivateKeySize, len(key.PrivateKey))
	}

	if len(key.PublicKey) != ed25519.PublicKeySize {
		t.Errorf("expected public key size %d, got %d", ed25519.PublicKeySize, len(key.PublicKey))
	}
}

func TestIdentityKeyFingerprint(t *testing.T) {
	key, err := GenerateIdentityKey()
	if err != nil {
		t.Fatalf("failed to generate identity key: %v", err)
	}

	fingerprint := key.Fingerprint()
	if fingerprint == "" {
		t.Error("fingerprint is empty")
	}

	// SHA256 fingerprints start with "SHA256:"
	if !strings.HasPrefix(fingerprint, "SHA256:") {
		t.Errorf("expected fingerprint to start with 'SHA256:', got %q", fingerprint)
	}
}

func TestIdentityKeyFingerprintLegacy(t *testing.T) {
	key, err := GenerateIdentityKey()
	if err != nil {
		t.Fatalf("failed to generate identity key: %v", err)
	}

	fingerprint := key.FingerprintLegacy()
	if fingerprint == "" {
		t.Error("legacy fingerprint is empty")
	}

	// MD5 fingerprints contain colons
	if !strings.Contains(fingerprint, ":") {
		t.Errorf("expected legacy fingerprint to contain colons, got %q", fingerprint)
	}
}

func TestIdentityKeyAuthorizedKey(t *testing.T) {
	key, err := GenerateIdentityKey()
	if err != nil {
		t.Fatalf("failed to generate identity key: %v", err)
	}

	authKey := key.AuthorizedKey()
	if authKey == "" {
		t.Error("authorized key is empty")
	}

	// Should start with ssh-ed25519
	if !strings.HasPrefix(authKey, "ssh-ed25519 ") {
		t.Errorf("expected authorized key to start with 'ssh-ed25519 ', got %q", authKey)
	}
}

func TestIdentityKeySaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "identity.pem")

	// Generate a key
	key, err := GenerateIdentityKey()
	if err != nil {
		t.Fatalf("failed to generate identity key: %v", err)
	}

	originalFingerprint := key.Fingerprint()
	originalAuthKey := key.AuthorizedKey()

	// Save it
	if err := key.Save(keyPath); err != nil {
		t.Fatalf("failed to save identity key: %v", err)
	}

	// Verify file exists with correct permissions
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("key file does not exist: %v", err)
	}

	// On Unix, check permissions (0600) - Windows doesn't support Unix permissions
	if runtime.GOOS != "windows" {
		if info.Mode().Perm() != 0600 {
			t.Errorf("expected file permissions 0600, got %o", info.Mode().Perm())
		}
	}

	// Load it back
	loadedKey, err := LoadIdentityKey(keyPath)
	if err != nil {
		t.Fatalf("failed to load identity key: %v", err)
	}

	// Verify it's the same key
	if loadedKey.Fingerprint() != originalFingerprint {
		t.Errorf("fingerprint mismatch: expected %q, got %q", originalFingerprint, loadedKey.Fingerprint())
	}

	if loadedKey.AuthorizedKey() != originalAuthKey {
		t.Errorf("authorized key mismatch")
	}

	if loadedKey.Path != keyPath {
		t.Errorf("path mismatch: expected %q, got %q", keyPath, loadedKey.Path)
	}
}

func TestLoadOrGenerateIdentityKey_Generate(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "identity.pem")

	// Should generate new key since it doesn't exist
	key, isNew, err := LoadOrGenerateIdentityKey(keyPath)
	if err != nil {
		t.Fatalf("failed to load or generate: %v", err)
	}

	if !isNew {
		t.Error("expected isNew to be true for new key")
	}

	if key == nil {
		t.Fatal("key is nil")
	}

	// Verify file was created
	if !IdentityKeyExists(keyPath) {
		t.Error("key file was not created")
	}
}

func TestLoadOrGenerateIdentityKey_Load(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "identity.pem")

	// First, generate and save a key
	originalKey, err := GenerateIdentityKey()
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}
	if err := originalKey.Save(keyPath); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	originalFingerprint := originalKey.Fingerprint()

	// Now load or generate - should load existing
	key, isNew, err := LoadOrGenerateIdentityKey(keyPath)
	if err != nil {
		t.Fatalf("failed to load or generate: %v", err)
	}

	if isNew {
		t.Error("expected isNew to be false for existing key")
	}

	if key.Fingerprint() != originalFingerprint {
		t.Error("loaded key has different fingerprint than original")
	}
}

func TestIdentityKeyExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Non-existent path
	if IdentityKeyExists(filepath.Join(tmpDir, "nonexistent.pem")) {
		t.Error("expected false for non-existent path")
	}

	// Create a file
	keyPath := filepath.Join(tmpDir, "exists.pem")
	if err := os.WriteFile(keyPath, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	if !IdentityKeyExists(keyPath) {
		t.Error("expected true for existing path")
	}
}

func TestDefaultIdentityKeyPath(t *testing.T) {
	path, err := DefaultIdentityKeyPath("bib")
	if err != nil {
		t.Fatalf("failed to get default path: %v", err)
	}

	if path == "" {
		t.Error("path is empty")
	}

	// Should contain .config/bib
	if !strings.Contains(path, ".config") || !strings.Contains(path, "bib") {
		t.Errorf("unexpected path format: %q", path)
	}

	// Should end with identity.pem
	if !strings.HasSuffix(path, "identity.pem") {
		t.Errorf("expected path to end with 'identity.pem', got %q", path)
	}
}

func TestIdentityKeySign(t *testing.T) {
	key, err := GenerateIdentityKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	message := []byte("test message to sign")

	sig, err := key.Sign(message)
	if err != nil {
		t.Fatalf("failed to sign: %v", err)
	}

	if len(sig) == 0 {
		t.Error("signature is empty")
	}

	// Verify signature using ed25519.Verify
	if !ed25519.Verify(key.PublicKey, message, sig) {
		t.Error("signature verification failed")
	}
}

func TestIdentityKeyInfo(t *testing.T) {
	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "identity.pem")

	key, err := GenerateIdentityKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	if err := key.Save(keyPath); err != nil {
		t.Fatalf("failed to save key: %v", err)
	}

	info := key.Info()

	if info.Path != keyPath {
		t.Errorf("expected path %q, got %q", keyPath, info.Path)
	}

	if info.KeyType != "ed25519" {
		t.Errorf("expected key type 'ed25519', got %q", info.KeyType)
	}

	if info.KeySize != 256 {
		t.Errorf("expected key size 256, got %d", info.KeySize)
	}

	if !strings.HasPrefix(info.Fingerprint, "SHA256:") {
		t.Errorf("expected fingerprint to start with 'SHA256:', got %q", info.Fingerprint)
	}

	if !strings.HasPrefix(info.PublicKey, "ssh-ed25519 ") {
		t.Errorf("expected public key to start with 'ssh-ed25519 ', got %q", info.PublicKey)
	}
}

func TestIdentityKeySigner(t *testing.T) {
	key, err := GenerateIdentityKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	signer, err := key.Signer()
	if err != nil {
		t.Fatalf("failed to get signer: %v", err)
	}

	if signer == nil {
		t.Error("signer is nil")
	}

	// Verify signer public key matches
	signerPubKey := signer.PublicKey()
	if signerPubKey == nil {
		t.Error("signer public key is nil")
	}
}

func TestIdentityKeyPublicKeyBytes(t *testing.T) {
	key, err := GenerateIdentityKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	pubBytes := key.PublicKeyBytes()

	if len(pubBytes) != ed25519.PublicKeySize {
		t.Errorf("expected public key size %d, got %d", ed25519.PublicKeySize, len(pubBytes))
	}
}
