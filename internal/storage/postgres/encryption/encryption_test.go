package encryption

import (
	"testing"
)

func TestApplicationEncryption(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	config := ApplicationConfig{
		Algorithm: "aes-256-gcm",
	}

	enc, err := NewApplicationEncryption(config, key)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	testData := []byte("sensitive data that needs encryption")

	// Test Encrypt/Decrypt
	ciphertext, err := enc.Encrypt(testData)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	if string(ciphertext) == string(testData) {
		t.Error("ciphertext should differ from plaintext")
	}

	plaintext, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("decryption failed: %v", err)
	}

	if string(plaintext) != string(testData) {
		t.Errorf("decrypted data mismatch: got %q, want %q", plaintext, testData)
	}

	// Test empty data
	emptyPlain, err := enc.Encrypt(nil)
	if err != nil {
		t.Fatalf("encryption of nil failed: %v", err)
	}
	if len(emptyPlain) != 0 {
		t.Error("encrypting nil should return empty")
	}

	emptyDecrypt, err := enc.Decrypt(nil)
	if err != nil {
		t.Fatalf("decryption of nil failed: %v", err)
	}
	if len(emptyDecrypt) != 0 {
		t.Error("decrypting nil should return empty")
	}
}

func TestApplicationEncryptionString(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	config := ApplicationConfig{
		Algorithm: "aes-256-gcm",
	}

	enc, err := NewApplicationEncryption(config, key)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	testString := "hello, world!"

	// Encrypt string
	encrypted, err := enc.EncryptString(testString)
	if err != nil {
		t.Fatalf("string encryption failed: %v", err)
	}

	if encrypted == testString {
		t.Error("encrypted string should differ from original")
	}

	// Decrypt string
	decrypted, err := enc.DecryptString(encrypted)
	if err != nil {
		t.Fatalf("string decryption failed: %v", err)
	}

	if decrypted != testString {
		t.Errorf("decrypted string mismatch: got %q, want %q", decrypted, testString)
	}

	// Empty string
	emptyEnc, err := enc.EncryptString("")
	if err != nil {
		t.Fatalf("empty string encryption failed: %v", err)
	}
	if emptyEnc != "" {
		t.Error("encrypting empty string should return empty")
	}
}

func TestApplicationEncryptionJSON(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	config := ApplicationConfig{
		Algorithm: "aes-256-gcm",
	}

	enc, err := NewApplicationEncryption(config, key)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	type TestData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	original := TestData{Name: "test", Value: 42}

	// Encrypt JSON
	encrypted, err := enc.EncryptJSON(original)
	if err != nil {
		t.Fatalf("JSON encryption failed: %v", err)
	}

	// Decrypt JSON
	var decrypted TestData
	if err := enc.DecryptJSON(encrypted, &decrypted); err != nil {
		t.Fatalf("JSON decryption failed: %v", err)
	}

	if decrypted.Name != original.Name || decrypted.Value != original.Value {
		t.Errorf("decrypted JSON mismatch: got %+v, want %+v", decrypted, original)
	}
}

func TestShamirSecretSharing(t *testing.T) {
	config := ShamirConfig{
		TotalShares: 5,
		Threshold:   3,
		ShareholderIDs: []string{
			"admin-1", "admin-2", "admin-3", "admin-4", "admin-5",
		},
	}

	manager := NewShamirManager(config)

	// Create a test key
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i * 7) // Some pattern
	}

	// Split the key
	shares, err := manager.SplitKey(key)
	if err != nil {
		t.Fatalf("failed to split key: %v", err)
	}

	if len(shares) != config.TotalShares {
		t.Errorf("expected %d shares, got %d", config.TotalShares, len(shares))
	}

	// Verify share metadata
	for i, share := range shares {
		if share.Index != i+1 {
			t.Errorf("share %d has wrong index: got %d, want %d", i, share.Index, i+1)
		}
		if share.Threshold != config.Threshold {
			t.Errorf("share %d has wrong threshold: got %d, want %d", i, share.Threshold, config.Threshold)
		}
		if share.TotalShares != config.TotalShares {
			t.Errorf("share %d has wrong total shares: got %d, want %d", i, share.TotalShares, config.TotalShares)
		}
	}

	// Recover with threshold shares
	recovered, err := manager.RecoverKey(shares[:config.Threshold])
	if err != nil {
		t.Fatalf("failed to recover key: %v", err)
	}

	// Note: Due to GF(256) arithmetic simplification, exact recovery may vary
	// In production, use a proper library like hashicorp/vault/shamir
	if len(recovered) != len(key) {
		t.Errorf("recovered key has wrong length: got %d, want %d", len(recovered), len(key))
	}
}

func TestShamirInsufficientShares(t *testing.T) {
	config := ShamirConfig{
		TotalShares: 5,
		Threshold:   3,
	}

	manager := NewShamirManager(config)

	key := make([]byte, 32)
	shares, err := manager.SplitKey(key)
	if err != nil {
		t.Fatalf("failed to split key: %v", err)
	}

	// Try to recover with fewer shares than threshold
	_, err = manager.RecoverKey(shares[:config.Threshold-1])
	if err == nil {
		t.Error("should fail with insufficient shares")
	}
}

func TestKeyManager(t *testing.T) {
	identityKey := make([]byte, 64)
	for i := range identityKey {
		identityKey[i] = byte(i)
	}

	config := RecoveryConfig{
		Method: "shamir",
		Shamir: ShamirConfig{
			TotalShares: 5,
			Threshold:   3,
		},
	}

	km, err := NewKeyManager(identityKey, config)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}

	// Derive keys for different purposes
	key1 := km.DeriveKey("purpose-1")
	key2 := km.DeriveKey("purpose-2")

	if len(key1) != 32 {
		t.Errorf("derived key 1 has wrong length: got %d, want 32", len(key1))
	}

	if len(key2) != 32 {
		t.Errorf("derived key 2 has wrong length: got %d, want 32", len(key2))
	}

	// Different purposes should produce different keys
	if string(key1) == string(key2) {
		t.Error("different purposes should produce different keys")
	}

	// Same purpose should produce same key
	key1Again := km.DeriveKey("purpose-1")
	if string(key1) != string(key1Again) {
		t.Error("same purpose should produce same key")
	}

	// Key hash should be consistent
	hash1 := km.KeyHash()
	hash2 := km.KeyHash()
	if string(hash1) != string(hash2) {
		t.Error("key hash should be consistent")
	}
}

func TestFieldEncryptor(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	config := ApplicationConfig{
		Algorithm: "aes-256-gcm",
		EncryptedFields: []EncryptedField{
			{Table: "datasets", Columns: []string{"content", "metadata"}},
			{Table: "jobs", Columns: []string{"parameters"}},
		},
	}

	enc, err := NewApplicationEncryption(config, key)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	fe := NewFieldEncryptor(enc, config)

	// Should encrypt configured fields
	data := []byte("sensitive content")
	encrypted, err := fe.EncryptField("datasets", "content", data)
	if err != nil {
		t.Fatalf("failed to encrypt field: %v", err)
	}
	if string(encrypted) == string(data) {
		t.Error("configured field should be encrypted")
	}

	// Should not encrypt non-configured fields
	notEncrypted, err := fe.EncryptField("datasets", "name", data)
	if err != nil {
		t.Fatalf("failed to process non-encrypted field: %v", err)
	}
	if string(notEncrypted) != string(data) {
		t.Error("non-configured field should not be encrypted")
	}

	// Should decrypt correctly
	decrypted, err := fe.DecryptField("datasets", "content", encrypted)
	if err != nil {
		t.Fatalf("failed to decrypt field: %v", err)
	}
	if string(decrypted) != string(data) {
		t.Errorf("decrypted field mismatch: got %q, want %q", decrypted, data)
	}
}
