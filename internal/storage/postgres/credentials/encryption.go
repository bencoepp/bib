package credentials

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/nacl/secretbox"
)

const (
	// NonceSize is the size of the nonce for encryption.
	NonceSize = 24

	// KeySize is the size of encryption keys.
	KeySize = 32

	// GCMNonceSize is the nonce size for AES-GCM.
	GCMNonceSize = 12

	// SaltSize is the size of the salt for key derivation.
	SaltSize = 32
)

var (
	// ErrDecryptionFailed indicates decryption failed (wrong key or corrupted data).
	ErrDecryptionFailed = errors.New("decryption failed")

	// ErrInvalidKeyLength indicates the provided key has wrong length.
	ErrInvalidKeyLength = errors.New("invalid key length")

	// ErrUnsupportedMethod indicates an unknown encryption method.
	ErrUnsupportedMethod = errors.New("unsupported encryption method")
)

// Encryptor provides encryption and decryption operations.
type Encryptor interface {
	// Encrypt encrypts plaintext and returns ciphertext.
	Encrypt(plaintext []byte) ([]byte, error)

	// Decrypt decrypts ciphertext and returns plaintext.
	Decrypt(ciphertext []byte) ([]byte, error)

	// Method returns the encryption method used.
	Method() EncryptionMethod
}

// NewEncryptor creates an encryptor based on the method and identity key.
func NewEncryptor(method EncryptionMethod, identityKey []byte) (Encryptor, error) {
	if len(identityKey) < 32 {
		return nil, ErrInvalidKeyLength
	}

	switch method {
	case EncryptionX25519:
		return newX25519Encryptor(identityKey)
	case EncryptionHKDF:
		return newHKDFEncryptor(identityKey)
	case EncryptionHybrid:
		return newHybridEncryptor(identityKey)
	default:
		return nil, ErrUnsupportedMethod
	}
}

// x25519Encryptor implements encryption using X25519 key derivation and XSalsa20-Poly1305.
type x25519Encryptor struct {
	key [KeySize]byte
}

func newX25519Encryptor(identityKey []byte) (*x25519Encryptor, error) {
	// Convert Ed25519 seed to X25519 key using standard conversion.
	// The first 32 bytes of an Ed25519 private key is the seed.
	seed := identityKey[:32]

	// Hash the seed with SHA-512 and take first 32 bytes (Ed25519 convention).
	h := sha512.Sum512(seed)

	// Apply X25519 clamping to the first 32 bytes.
	h[0] &= 248
	h[31] &= 127
	h[31] |= 64

	var key [KeySize]byte
	copy(key[:], h[:32])

	return &x25519Encryptor{key: key}, nil
}

func (e *x25519Encryptor) Method() EncryptionMethod {
	return EncryptionX25519
}

func (e *x25519Encryptor) Encrypt(plaintext []byte) ([]byte, error) {
	// Generate a random nonce.
	var nonce [NonceSize]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt using NaCl secretbox (XSalsa20-Poly1305).
	encrypted := secretbox.Seal(nonce[:], plaintext, &nonce, &e.key)

	// Prepend method identifier.
	result := make([]byte, 1+len(encrypted))
	result[0] = byte(EncryptionX25519[0]) // 'x' for x25519
	copy(result[1:], encrypted)

	return result, nil
}

func (e *x25519Encryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < 1+NonceSize+secretbox.Overhead {
		return nil, ErrDecryptionFailed
	}

	// Skip method identifier.
	data := ciphertext[1:]

	// Extract nonce.
	var nonce [NonceSize]byte
	copy(nonce[:], data[:NonceSize])

	// Decrypt.
	plaintext, ok := secretbox.Open(nil, data[NonceSize:], &nonce, &e.key)
	if !ok {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// hkdfEncryptor implements encryption using HKDF-SHA256 key derivation and AES-256-GCM.
type hkdfEncryptor struct {
	key []byte
}

func newHKDFEncryptor(identityKey []byte) (*hkdfEncryptor, error) {
	// Use HKDF to derive an encryption key from the identity key.
	// Info provides context binding to prevent key reuse across different purposes.
	info := []byte("bibd-credential-encryption-v1")
	salt := []byte("bibd-static-salt-v1") // Static salt is fine since identity key is unique.

	hkdfReader := hkdf.New(sha256.New, identityKey[:32], salt, info)
	key := make([]byte, KeySize)
	if _, err := io.ReadFull(hkdfReader, key); err != nil {
		return nil, fmt.Errorf("failed to derive key: %w", err)
	}

	return &hkdfEncryptor{key: key}, nil
}

func (e *hkdfEncryptor) Method() EncryptionMethod {
	return EncryptionHKDF
}

func (e *hkdfEncryptor) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce.
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt.
	encrypted := gcm.Seal(nonce, nonce, plaintext, nil)

	// Prepend method identifier.
	result := make([]byte, 1+len(encrypted))
	result[0] = byte(EncryptionHKDF[0]) // 'h' for hkdf
	copy(result[1:], encrypted)

	return result, nil
}

func (e *hkdfEncryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < 1+GCMNonceSize+16 { // 16 is GCM tag size
		return nil, ErrDecryptionFailed
	}

	// Skip method identifier.
	data := ciphertext[1:]

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, ErrDecryptionFailed
	}

	nonce, encrypted := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// hybridEncryptor uses both X25519 and HKDF methods.
// Data is encrypted with HKDF but can also be decrypted with X25519 envelope.
type hybridEncryptor struct {
	x25519 *x25519Encryptor
	hkdf   *hkdfEncryptor
}

func newHybridEncryptor(identityKey []byte) (*hybridEncryptor, error) {
	x25519Enc, err := newX25519Encryptor(identityKey)
	if err != nil {
		return nil, err
	}

	hkdfEnc, err := newHKDFEncryptor(identityKey)
	if err != nil {
		return nil, err
	}

	return &hybridEncryptor{
		x25519: x25519Enc,
		hkdf:   hkdfEnc,
	}, nil
}

func (e *hybridEncryptor) Method() EncryptionMethod {
	return EncryptionHybrid
}

func (e *hybridEncryptor) Encrypt(plaintext []byte) ([]byte, error) {
	// Encrypt with HKDF method (primary).
	hkdfCiphertext, err := e.hkdf.Encrypt(plaintext)
	if err != nil {
		return nil, err
	}

	// Also encrypt with X25519 method.
	x25519Ciphertext, err := e.x25519.Encrypt(plaintext)
	if err != nil {
		return nil, err
	}

	// Format: [method byte 'H'][4 bytes HKDF len][HKDF ciphertext][X25519 ciphertext]
	hkdfLen := len(hkdfCiphertext)
	result := make([]byte, 1+4+hkdfLen+len(x25519Ciphertext))
	result[0] = 'H' // Hybrid
	result[1] = byte(hkdfLen >> 24)
	result[2] = byte(hkdfLen >> 16)
	result[3] = byte(hkdfLen >> 8)
	result[4] = byte(hkdfLen)
	copy(result[5:5+hkdfLen], hkdfCiphertext)
	copy(result[5+hkdfLen:], x25519Ciphertext)

	return result, nil
}

func (e *hybridEncryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < 6 {
		return nil, ErrDecryptionFailed
	}

	// Check if this is hybrid format.
	if ciphertext[0] == 'H' {
		// Extract HKDF ciphertext length.
		hkdfLen := int(ciphertext[1])<<24 | int(ciphertext[2])<<16 | int(ciphertext[3])<<8 | int(ciphertext[4])

		if len(ciphertext) < 5+hkdfLen {
			return nil, ErrDecryptionFailed
		}

		// Try HKDF first.
		hkdfCiphertext := ciphertext[5 : 5+hkdfLen]
		plaintext, err := e.hkdf.Decrypt(hkdfCiphertext)
		if err == nil {
			return plaintext, nil
		}

		// Fall back to X25519.
		x25519Ciphertext := ciphertext[5+hkdfLen:]
		return e.x25519.Decrypt(x25519Ciphertext)
	}

	// Try to detect method from first byte.
	switch ciphertext[0] {
	case 'h':
		return e.hkdf.Decrypt(ciphertext)
	case 'x':
		return e.x25519.Decrypt(ciphertext)
	default:
		// Try both methods.
		if plaintext, err := e.hkdf.Decrypt(ciphertext); err == nil {
			return plaintext, nil
		}
		return e.x25519.Decrypt(ciphertext)
	}
}

// DeriveX25519Key derives an X25519 key from an Ed25519 private key seed.
func DeriveX25519Key(ed25519Seed []byte) ([]byte, error) {
	if len(ed25519Seed) < 32 {
		return nil, ErrInvalidKeyLength
	}

	// Standard Ed25519 to X25519 conversion.
	h := sha512.Sum512(ed25519Seed[:32])
	h[0] &= 248
	h[31] &= 127
	h[31] |= 64

	var x25519Key [32]byte
	copy(x25519Key[:], h[:32])

	return x25519Key[:], nil
}

// DeriveX25519PublicKey derives the X25519 public key from the private key.
func DeriveX25519PublicKey(privateKey []byte) ([]byte, error) {
	if len(privateKey) != 32 {
		return nil, ErrInvalidKeyLength
	}

	var privKey [32]byte
	copy(privKey[:], privateKey)

	publicKey, err := curve25519.X25519(privKey[:], curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("failed to derive public key: %w", err)
	}

	return publicKey, nil
}
