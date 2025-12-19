package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"bib/internal/domain"
)

func TestParsePublicKeyAuto_Ed25519Raw(t *testing.T) {
	// Generate a real Ed25519 key pair
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	keyBytes, keyType, err := ParsePublicKeyAuto(pub)
	if err != nil {
		t.Fatalf("ParsePublicKeyAuto failed: %v", err)
	}

	if keyType != domain.KeyTypeEd25519 {
		t.Errorf("Expected Ed25519 key type, got %s", keyType)
	}
	if len(keyBytes) != ed25519.PublicKeySize {
		t.Errorf("Expected %d bytes, got %d", ed25519.PublicKeySize, len(keyBytes))
	}
}

func TestParsePublicKeyAuto_OpenSSHFormat(t *testing.T) {
	// Real OpenSSH Ed25519 public key (test key, not for production use)
	opensshKey := []byte("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKxL5wU+iKMnj7dCRoXHJ0nRKP9NXkm7v7p7L4t7n8dJ test@example.com")

	keyBytes, keyType, err := ParsePublicKeyAuto(opensshKey)
	if err != nil {
		t.Fatalf("ParsePublicKeyAuto failed: %v", err)
	}

	if keyType != domain.KeyTypeEd25519 {
		t.Errorf("Expected Ed25519 key type, got %s", keyType)
	}
	if len(keyBytes) != ed25519.PublicKeySize {
		t.Errorf("Expected %d bytes, got %d", ed25519.PublicKeySize, len(keyBytes))
	}
}

func TestGetPublicKeyInfo(t *testing.T) {
	// Generate a real Ed25519 key pair
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	info, err := GetPublicKeyInfo(pub)
	if err != nil {
		t.Fatalf("GetPublicKeyInfo failed: %v", err)
	}

	if info.KeyType != domain.KeyTypeEd25519 {
		t.Errorf("Expected Ed25519 key type, got %s", info.KeyType)
	}
	if info.KeySize != 256 {
		t.Errorf("Expected key size 256, got %d", info.KeySize)
	}
	if info.FingerprintSHA256 == "" {
		t.Error("FingerprintSHA256 should not be empty")
	}
	if info.FingerprintMD5 == "" {
		t.Error("FingerprintMD5 should not be empty")
	}
	if info.OpenSSHFormat == "" {
		t.Error("OpenSSHFormat should not be empty")
	}
	if len(info.OpenSSHFormat) < 20 {
		t.Error("OpenSSHFormat seems too short")
	}
}

func TestVerifySignature_Ed25519(t *testing.T) {
	// Generate a real Ed25519 key pair
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	message := []byte("test message to sign")
	signature := ed25519.Sign(priv, message)

	err = VerifySignature(pub, domain.KeyTypeEd25519, message, signature)
	if err != nil {
		t.Fatalf("VerifySignature failed: %v", err)
	}
}

func TestVerifySignature_Ed25519_Invalid(t *testing.T) {
	// Generate a real Ed25519 key pair
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	message := []byte("test message to sign")
	signature := ed25519.Sign(priv, message)

	// Modify the signature
	signature[0] ^= 0xFF

	err = VerifySignature(pub, domain.KeyTypeEd25519, message, signature)
	if err == nil {
		t.Error("VerifySignature should fail with invalid signature")
	}
}

func TestVerifySignature_WrongMessage(t *testing.T) {
	// Generate a real Ed25519 key pair
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	message := []byte("test message to sign")
	wrongMessage := []byte("different message")
	signature := ed25519.Sign(priv, message)

	err = VerifySignature(pub, domain.KeyTypeEd25519, wrongMessage, signature)
	if err == nil {
		t.Error("VerifySignature should fail with wrong message")
	}
}

func TestVerifySSHSignature_DirectSignature(t *testing.T) {
	// Generate a real Ed25519 key pair
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	message := []byte("test message to sign")
	signature := ed25519.Sign(priv, message)

	// VerifySSHSignature should handle direct signatures
	err = VerifySSHSignature(pub, domain.KeyTypeEd25519, message, signature)
	if err != nil {
		t.Fatalf("VerifySSHSignature failed: %v", err)
	}
}

func TestMarshalPublicKeyToAuthorizedKey_Roundtrip(t *testing.T) {
	// Generate a real Ed25519 key pair
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	// Marshal to authorized key format
	authKey, err := MarshalPublicKeyToAuthorizedKey(pub, domain.KeyTypeEd25519)
	if err != nil {
		t.Fatalf("MarshalPublicKeyToAuthorizedKey failed: %v", err)
	}

	// Parse it back
	keyBytes, keyType, err := ParseAuthorizedKey(authKey)
	if err != nil {
		t.Fatalf("ParseAuthorizedKey failed: %v", err)
	}

	if keyType != domain.KeyTypeEd25519 {
		t.Errorf("Expected Ed25519 key type, got %s", keyType)
	}

	// Compare key bytes
	if len(keyBytes) != len(pub) {
		t.Errorf("Key bytes length mismatch: got %d, want %d", len(keyBytes), len(pub))
	}
	for i := range keyBytes {
		if keyBytes[i] != pub[i] {
			t.Errorf("Key bytes mismatch at index %d", i)
			break
		}
	}
}
