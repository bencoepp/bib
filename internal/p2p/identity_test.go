package p2p

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateAndLoadIdentity(t *testing.T) {
	// Create a temp directory for testing
	tmpDir, err := os.MkdirTemp("", "bib-p2p-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test generating a new identity
	identity, err := GenerateIdentity("", tmpDir, false)
	if err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}

	if identity.PrivKey == nil {
		t.Fatal("identity.PrivKey is nil")
	}

	// Verify the key file was created
	keyPath := filepath.Join(tmpDir, DefaultIdentityFileName)
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Fatalf("identity key file was not created at %s", keyPath)
	}

	// Test loading the identity back
	loadedIdentity, err := LoadIdentity("", tmpDir)
	if err != nil {
		t.Fatalf("failed to load identity: %v", err)
	}

	// Verify the keys match
	originalBytes, _ := identity.PrivKey.Raw()
	loadedBytes, _ := loadedIdentity.PrivKey.Raw()

	if string(originalBytes) != string(loadedBytes) {
		t.Fatal("loaded identity does not match original")
	}
}

func TestGenerateIdentityNoForce(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-p2p-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Generate first identity
	_, err = GenerateIdentity("", tmpDir, false)
	if err != nil {
		t.Fatalf("failed to generate first identity: %v", err)
	}

	// Try to generate again without force - should fail
	_, err = GenerateIdentity("", tmpDir, false)
	if err == nil {
		t.Fatal("expected error when generating identity without force, got nil")
	}
}

func TestGenerateIdentityWithForce(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-p2p-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Generate first identity
	firstIdentity, err := GenerateIdentity("", tmpDir, false)
	if err != nil {
		t.Fatalf("failed to generate first identity: %v", err)
	}
	firstBytes, _ := firstIdentity.PrivKey.Raw()

	// Generate again with force - should succeed and create new key
	secondIdentity, err := GenerateIdentity("", tmpDir, true)
	if err != nil {
		t.Fatalf("failed to generate second identity with force: %v", err)
	}
	secondBytes, _ := secondIdentity.PrivKey.Raw()

	// Keys should be different
	if string(firstBytes) == string(secondBytes) {
		t.Fatal("second identity should be different from first")
	}
}

func TestLoadOrGenerateIdentity(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-p2p-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// First call should generate
	identity1, err := LoadOrGenerateIdentity("", tmpDir)
	if err != nil {
		t.Fatalf("failed to load or generate identity: %v", err)
	}

	// Second call should load the same identity
	identity2, err := LoadOrGenerateIdentity("", tmpDir)
	if err != nil {
		t.Fatalf("failed to load or generate identity second time: %v", err)
	}

	// Keys should be the same
	bytes1, _ := identity1.PrivKey.Raw()
	bytes2, _ := identity2.PrivKey.Raw()

	if string(bytes1) != string(bytes2) {
		t.Fatal("loaded identity should be the same as original")
	}
}

func TestCustomKeyPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-p2p-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	customPath := filepath.Join(tmpDir, "custom", "my-identity.pem")

	// Generate with custom path
	identity, err := GenerateIdentity(customPath, tmpDir, false)
	if err != nil {
		t.Fatalf("failed to generate identity with custom path: %v", err)
	}

	// Verify file exists at custom path
	if _, err := os.Stat(customPath); os.IsNotExist(err) {
		t.Fatalf("identity key file was not created at custom path %s", customPath)
	}

	// Load from custom path
	loadedIdentity, err := LoadIdentity(customPath, tmpDir)
	if err != nil {
		t.Fatalf("failed to load identity from custom path: %v", err)
	}

	originalBytes, _ := identity.PrivKey.Raw()
	loadedBytes, _ := loadedIdentity.PrivKey.Raw()

	if string(originalBytes) != string(loadedBytes) {
		t.Fatal("loaded identity does not match original")
	}
}
