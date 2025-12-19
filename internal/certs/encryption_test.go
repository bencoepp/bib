package certs

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestEncryptDecryptKey(t *testing.T) {
	// Generate a dummy key PEM
	keyPEM := []byte(`-----BEGIN EC PRIVATE KEY-----
MHQCAQEEIFakeKeyDataHereForTestingPurposesOnlyNotReal
-----END EC PRIVATE KEY-----
`)

	// Generate a secret (simulating P2P identity key)
	secret := make([]byte, 64)
	if _, err := rand.Read(secret); err != nil {
		t.Fatalf("failed to generate secret: %v", err)
	}

	// Encrypt
	encrypted, err := EncryptKey(keyPEM, secret)
	if err != nil {
		t.Fatalf("failed to encrypt key: %v", err)
	}

	// Verify it's different from original
	if bytes.Equal(encrypted, keyPEM) {
		t.Error("encrypted data should differ from original")
	}

	// Verify it's recognized as encrypted
	if !IsEncryptedKey(encrypted) {
		t.Error("should be recognized as encrypted key")
	}

	// Decrypt
	decrypted, err := DecryptKey(encrypted, secret)
	if err != nil {
		t.Fatalf("failed to decrypt key: %v", err)
	}

	// Verify decrypted matches original
	if !bytes.Equal(decrypted, keyPEM) {
		t.Error("decrypted data should match original")
	}
}

func TestEncryptKeyWithWrongSecret(t *testing.T) {
	keyPEM := []byte(`-----BEGIN EC PRIVATE KEY-----
MHQCAQEEIFakeKeyDataHereForTestingPurposesOnlyNotReal
-----END EC PRIVATE KEY-----
`)

	secret1 := make([]byte, 64)
	secret2 := make([]byte, 64)
	rand.Read(secret1)
	rand.Read(secret2)

	encrypted, err := EncryptKey(keyPEM, secret1)
	if err != nil {
		t.Fatalf("failed to encrypt key: %v", err)
	}

	// Decrypt with wrong secret should fail
	_, err = DecryptKey(encrypted, secret2)
	if err == nil {
		t.Error("decryption with wrong secret should fail")
	}
}

func TestIsEncryptedKey(t *testing.T) {
	// Unencrypted PEM
	unencrypted := []byte(`-----BEGIN EC PRIVATE KEY-----
MHQCAQEEIFakeKeyData
-----END EC PRIVATE KEY-----
`)

	if IsEncryptedKey(unencrypted) {
		t.Error("unencrypted key should not be recognized as encrypted")
	}

	// Invalid PEM
	invalid := []byte("not a pem file")
	if IsEncryptedKey(invalid) {
		t.Error("invalid data should not be recognized as encrypted")
	}
}

func TestDeriveEncryptionKey(t *testing.T) {
	secret := []byte("test-secret-key-data")
	salt := make([]byte, saltSize)
	rand.Read(salt)

	key1, err := DeriveEncryptionKey(secret, salt, "test-info")
	if err != nil {
		t.Fatalf("failed to derive key: %v", err)
	}

	if len(key1) != 32 {
		t.Errorf("expected 32-byte key, got %d bytes", len(key1))
	}

	// Same inputs should produce same key
	key2, _ := DeriveEncryptionKey(secret, salt, "test-info")
	if !bytes.Equal(key1, key2) {
		t.Error("same inputs should produce same key")
	}

	// Different salt should produce different key
	salt2 := make([]byte, saltSize)
	rand.Read(salt2)
	key3, _ := DeriveEncryptionKey(secret, salt2, "test-info")
	if bytes.Equal(key1, key3) {
		t.Error("different salt should produce different key")
	}

	// Different info should produce different key
	key4, _ := DeriveEncryptionKey(secret, salt, "different-info")
	if bytes.Equal(key1, key4) {
		t.Error("different info should produce different key")
	}
}
