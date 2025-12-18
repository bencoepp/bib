package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// ApplicationEncryption implements application-level field encryption.
type ApplicationEncryption struct {
	key       []byte
	algorithm string
	aead      cipher.AEAD
}

// NewApplicationEncryption creates a new application-level encryptor.
func NewApplicationEncryption(config ApplicationConfig, key []byte) (*ApplicationEncryption, error) {
	if len(key) < 32 {
		return nil, ErrInvalidKey
	}

	var aead cipher.AEAD

	switch config.Algorithm {
	case "aes-256-gcm", "":
		block, err := aes.NewCipher(key[:32])
		if err != nil {
			return nil, fmt.Errorf("failed to create AES cipher: %w", err)
		}
		var gcmErr error
		aead, gcmErr = cipher.NewGCM(block)
		if gcmErr != nil {
			return nil, fmt.Errorf("failed to create GCM: %w", gcmErr)
		}
	case "chacha20-poly1305":
		// Would require golang.org/x/crypto/chacha20poly1305
		return nil, errors.New("chacha20-poly1305 not implemented")
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", config.Algorithm)
	}

	return &ApplicationEncryption{
		key:       key[:32],
		algorithm: config.Algorithm,
		aead:      aead,
	}, nil
}

// Encrypt encrypts plaintext bytes.
func (e *ApplicationEncryption) Encrypt(plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return plaintext, nil
	}

	// Generate random nonce
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt with authentication
	// Format: [nonce][ciphertext+tag]
	ciphertext := e.aead.Seal(nonce, nonce, plaintext, nil)

	return ciphertext, nil
}

// Decrypt decrypts ciphertext bytes.
func (e *ApplicationEncryption) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return ciphertext, nil
	}

	nonceSize := e.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrDecryptFailed
	}

	nonce := ciphertext[:nonceSize]
	ciphertextOnly := ciphertext[nonceSize:]

	plaintext, err := e.aead.Open(nil, nonce, ciphertextOnly, nil)
	if err != nil {
		return nil, ErrDecryptFailed
	}

	return plaintext, nil
}

// EncryptString encrypts a string and returns base64-encoded ciphertext.
func (e *ApplicationEncryption) EncryptString(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	ciphertext, err := e.Encrypt([]byte(plaintext))
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptString decrypts a base64-encoded ciphertext string.
func (e *ApplicationEncryption) DecryptString(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	plaintext, err := e.Decrypt(data)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// EncryptJSON encrypts a JSON-serializable value.
func (e *ApplicationEncryption) EncryptJSON(value interface{}) ([]byte, error) {
	if value == nil {
		return nil, nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return e.Encrypt(data)
}

// DecryptJSON decrypts and unmarshals a JSON value.
func (e *ApplicationEncryption) DecryptJSON(ciphertext []byte, target interface{}) error {
	if len(ciphertext) == 0 {
		return nil
	}

	plaintext, err := e.Decrypt(ciphertext)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(plaintext, target); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return nil
}

// SensitiveFields defines the default fields to encrypt.
var SensitiveFields = map[string][]string{
	"datasets": {"content", "metadata"},
	"jobs":     {"parameters", "result"},
	"nodes":    {"metadata"},
}

// ShouldEncrypt checks if a table/column combination should be encrypted.
func ShouldEncrypt(table, column string, config ApplicationConfig) bool {
	for _, field := range config.EncryptedFields {
		if field.Table == table {
			for _, col := range field.Columns {
				if col == column {
					return true
				}
			}
		}
	}
	return false
}

// EncryptedValue wraps an encrypted value with metadata.
type EncryptedValue struct {
	// Algorithm used for encryption.
	Algorithm string `json:"alg"`

	// Version of the encryption format.
	Version int `json:"ver"`

	// Data is the base64-encoded ciphertext.
	Data string `json:"data"`
}

// EncryptWithMetadata encrypts a value and includes metadata.
func (e *ApplicationEncryption) EncryptWithMetadata(plaintext []byte) (*EncryptedValue, error) {
	ciphertext, err := e.Encrypt(plaintext)
	if err != nil {
		return nil, err
	}

	return &EncryptedValue{
		Algorithm: e.algorithm,
		Version:   1,
		Data:      base64.StdEncoding.EncodeToString(ciphertext),
	}, nil
}

// DecryptWithMetadata decrypts a value with embedded metadata.
func (e *ApplicationEncryption) DecryptWithMetadata(value *EncryptedValue) ([]byte, error) {
	if value == nil {
		return nil, nil
	}

	if value.Algorithm != e.algorithm {
		return nil, fmt.Errorf("algorithm mismatch: expected %s, got %s", e.algorithm, value.Algorithm)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(value.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	return e.Decrypt(ciphertext)
}

// FieldEncryptor provides a wrapper for encrypting specific database fields.
type FieldEncryptor struct {
	encryption *ApplicationEncryption
	config     ApplicationConfig
}

// NewFieldEncryptor creates a new field encryptor.
func NewFieldEncryptor(encryption *ApplicationEncryption, config ApplicationConfig) *FieldEncryptor {
	return &FieldEncryptor{
		encryption: encryption,
		config:     config,
	}
}

// EncryptField encrypts a field if it's in the encrypted fields list.
func (fe *FieldEncryptor) EncryptField(table, column string, value []byte) ([]byte, error) {
	if !ShouldEncrypt(table, column, fe.config) {
		return value, nil
	}
	return fe.encryption.Encrypt(value)
}

// DecryptField decrypts a field if it's in the encrypted fields list.
func (fe *FieldEncryptor) DecryptField(table, column string, value []byte) ([]byte, error) {
	if !ShouldEncrypt(table, column, fe.config) {
		return value, nil
	}
	return fe.encryption.Decrypt(value)
}
