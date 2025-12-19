package certs

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/crypto/hkdf"
)

// EncryptedKeyFile represents an encrypted private key file.
const (
	encryptedKeyHeader = "ENCRYPTED PRIVATE KEY"
	saltSize           = 32
	nonceSize          = 12 // GCM standard nonce size
)

// DeriveEncryptionKey derives an AES-256 encryption key from a secret (e.g., P2P identity key).
// Uses HKDF with SHA-256 for key derivation.
func DeriveEncryptionKey(secret []byte, salt []byte, info string) ([]byte, error) {
	if len(salt) == 0 {
		salt = make([]byte, saltSize)
		if _, err := rand.Read(salt); err != nil {
			return nil, fmt.Errorf("failed to generate salt: %w", err)
		}
	}

	reader := hkdf.New(sha256.New, secret, salt, []byte(info))
	key := make([]byte, 32) // AES-256
	if _, err := io.ReadFull(reader, key); err != nil {
		return nil, fmt.Errorf("failed to derive key: %w", err)
	}

	return key, nil
}

// EncryptKey encrypts a PEM-encoded private key using AES-256-GCM.
// The secret is typically the node's P2P identity private key.
func EncryptKey(keyPEM []byte, secret []byte) ([]byte, error) {
	// Generate random salt
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	// Derive encryption key
	encKey, err := DeriveEncryptionKey(secret, salt, "bibd-ca-key-encryption")
	if err != nil {
		return nil, err
	}

	// Create AES-GCM cipher
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate nonce
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt the key
	ciphertext := gcm.Seal(nil, nonce, keyPEM, nil)

	// Combine salt + nonce + ciphertext and encode as PEM
	combined := make([]byte, 0, len(salt)+len(nonce)+len(ciphertext))
	combined = append(combined, salt...)
	combined = append(combined, nonce...)
	combined = append(combined, ciphertext...)

	return pem.EncodeToMemory(&pem.Block{
		Type:  encryptedKeyHeader,
		Bytes: combined,
	}), nil
}

// DecryptKey decrypts an encrypted private key using the secret.
func DecryptKey(encryptedPEM []byte, secret []byte) ([]byte, error) {
	block, _ := pem.Decode(encryptedPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode encrypted PEM")
	}

	if block.Type != encryptedKeyHeader {
		return nil, fmt.Errorf("unexpected PEM type: %s", block.Type)
	}

	data := block.Bytes
	if len(data) < saltSize+nonceSize+16 { // 16 is minimum GCM tag size
		return nil, fmt.Errorf("encrypted data too short")
	}

	salt := data[:saltSize]
	nonce := data[saltSize : saltSize+nonceSize]
	ciphertext := data[saltSize+nonceSize:]

	// Derive encryption key
	encKey, err := DeriveEncryptionKey(secret, salt, "bibd-ca-key-encryption")
	if err != nil {
		return nil, err
	}

	// Create AES-GCM cipher
	aesBlock, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(aesBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// SaveEncryptedKey saves an encrypted private key to a file.
func SaveEncryptedKey(path string, keyPEM, secret []byte) error {
	encrypted, err := EncryptKey(keyPEM, secret)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(path, encrypted, 0600); err != nil {
		return fmt.Errorf("failed to write encrypted key: %w", err)
	}

	return nil
}

// LoadEncryptedKey loads and decrypts a private key from a file.
func LoadEncryptedKey(path string, secret []byte) ([]byte, error) {
	encrypted, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read encrypted key: %w", err)
	}

	return DecryptKey(encrypted, secret)
}

// IsEncryptedKey checks if a PEM file contains an encrypted key.
func IsEncryptedKey(pemData []byte) bool {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return false
	}
	return block.Type == encryptedKeyHeader
}
