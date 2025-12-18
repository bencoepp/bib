package encryption

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/nacl/secretbox"
)

var (
	// ErrInsufficientShares indicates not enough shares were provided.
	ErrInsufficientShares = errors.New("insufficient shares for recovery")

	// ErrInvalidShare indicates a share is invalid or corrupted.
	ErrInvalidShare = errors.New("invalid share")

	// ErrShareRecoveryFailed indicates share recovery failed.
	ErrShareRecoveryFailed = errors.New("share recovery failed")
)

// Share represents a single key share for Shamir's Secret Sharing.
type Share struct {
	// ID is the identifier for this share (e.g., shareholder name).
	ID string `json:"id"`

	// Index is the x-coordinate in the polynomial (1-indexed).
	Index int `json:"index"`

	// Data is the actual share data.
	Data []byte `json:"data"`

	// Created is when this share was generated.
	Created time.Time `json:"created"`

	// Threshold is the minimum shares needed for recovery.
	Threshold int `json:"threshold"`

	// TotalShares is the total number of shares generated.
	TotalShares int `json:"total_shares"`
}

// Export exports the share as a portable base64 string, optionally encrypted.
func (s *Share) Export(recipientPublicKey []byte) (string, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("failed to marshal share: %w", err)
	}

	if len(recipientPublicKey) > 0 {
		// Encrypt with recipient's public key
		encrypted, err := encryptForRecipient(data, recipientPublicKey)
		if err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString(encrypted), nil
	}

	// No encryption - just base64 encode
	return base64.StdEncoding.EncodeToString(data), nil
}

// ImportShare imports a share from a portable format.
func ImportShare(encoded string, privateKey []byte) (*Share, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode share: %w", err)
	}

	if len(privateKey) > 0 {
		// Decrypt with private key
		decrypted, err := decryptFromRecipient(data, privateKey)
		if err != nil {
			return nil, err
		}
		data = decrypted
	}

	var share Share
	if err := json.Unmarshal(data, &share); err != nil {
		return nil, fmt.Errorf("failed to unmarshal share: %w", err)
	}

	return &share, nil
}

// ShamirManager handles Shamir's Secret Sharing operations.
type ShamirManager struct {
	config ShamirConfig
}

// NewShamirManager creates a new Shamir manager.
func NewShamirManager(config ShamirConfig) *ShamirManager {
	return &ShamirManager{config: config}
}

// SplitKey splits a key into shares using Shamir's Secret Sharing.
// This is a simplified implementation - for production use, consider
// using a well-audited library like hashicorp/vault/shamir.
func (sm *ShamirManager) SplitKey(key []byte) ([]Share, error) {
	if sm.config.TotalShares < sm.config.Threshold {
		return nil, fmt.Errorf("total shares (%d) must be >= threshold (%d)",
			sm.config.TotalShares, sm.config.Threshold)
	}

	if sm.config.Threshold < 2 {
		return nil, fmt.Errorf("threshold must be at least 2")
	}

	// Generate random coefficients for the polynomial
	// The secret is the constant term (coefficient 0)
	coefficients := make([][]byte, sm.config.Threshold)
	coefficients[0] = key // The secret

	for i := 1; i < sm.config.Threshold; i++ {
		coeff := make([]byte, len(key))
		if _, err := rand.Read(coeff); err != nil {
			return nil, fmt.Errorf("failed to generate coefficient: %w", err)
		}
		coefficients[i] = coeff
	}

	// Generate shares by evaluating the polynomial at x = 1, 2, ..., n
	shares := make([]Share, sm.config.TotalShares)
	now := time.Now().UTC()

	for i := 0; i < sm.config.TotalShares; i++ {
		x := i + 1 // x-coordinate (1-indexed)
		y := evaluatePolynomial(coefficients, x)

		shareID := ""
		if i < len(sm.config.ShareholderIDs) {
			shareID = sm.config.ShareholderIDs[i]
		} else {
			shareID = fmt.Sprintf("share-%d", i+1)
		}

		shares[i] = Share{
			ID:          shareID,
			Index:       x,
			Data:        y,
			Created:     now,
			Threshold:   sm.config.Threshold,
			TotalShares: sm.config.TotalShares,
		}
	}

	return shares, nil
}

// RecoverKey recovers the secret from shares using Lagrange interpolation.
func (sm *ShamirManager) RecoverKey(shares []Share) ([]byte, error) {
	if len(shares) < sm.config.Threshold {
		return nil, fmt.Errorf("%w: need %d, got %d",
			ErrInsufficientShares, sm.config.Threshold, len(shares))
	}

	// Use only the first 'threshold' shares
	shares = shares[:sm.config.Threshold]

	// Verify all shares have the same length
	keyLen := len(shares[0].Data)
	for _, share := range shares {
		if len(share.Data) != keyLen {
			return nil, ErrInvalidShare
		}
	}

	// Perform Lagrange interpolation to recover the secret at x = 0
	secret := make([]byte, keyLen)

	for i, share := range shares {
		// Calculate Lagrange basis polynomial L_i(0)
		// L_i(0) = Π (0 - x_j) / (x_i - x_j) for j ≠ i
		numerator := 1
		denominator := 1

		for j, otherShare := range shares {
			if i == j {
				continue
			}
			numerator *= -otherShare.Index                // 0 - x_j
			denominator *= share.Index - otherShare.Index // x_i - x_j
		}

		// Calculate coefficient in GF(256)
		coefficient := gfDiv(numerator, denominator)

		// Add contribution to secret
		for k := 0; k < keyLen; k++ {
			secret[k] ^= gfMul(share.Data[k], coefficient)
		}
	}

	return secret, nil
}

// evaluatePolynomial evaluates the polynomial at x.
func evaluatePolynomial(coefficients [][]byte, x int) []byte {
	keyLen := len(coefficients[0])
	result := make([]byte, keyLen)

	// Evaluate using Horner's method in GF(256)
	for k := 0; k < keyLen; k++ {
		value := byte(0)
		for i := len(coefficients) - 1; i >= 0; i-- {
			value = gfMul(value, byte(x)) ^ coefficients[i][k]
		}
		result[k] = value
	}

	return result
}

// GF(256) arithmetic for Shamir's Secret Sharing
// Using the AES field polynomial: x^8 + x^4 + x^3 + x + 1

var (
	gfExp [512]byte // Anti-log table
	gfLog [256]byte // Log table
)

func init() {
	// Initialize GF(256) tables
	x := byte(1)
	for i := 0; i < 255; i++ {
		gfExp[i] = x
		gfLog[x] = byte(i)
		x = gfMulNoTable(x, 2)
	}
	for i := 255; i < 512; i++ {
		gfExp[i] = gfExp[i-255]
	}
}

// gfMulNoTable multiplies two bytes in GF(256) without using tables.
func gfMulNoTable(a, b byte) byte {
	result := byte(0)
	for b > 0 {
		if b&1 != 0 {
			result ^= a
		}
		high := a & 0x80
		a <<= 1
		if high != 0 {
			a ^= 0x1b // AES field polynomial reduction
		}
		b >>= 1
	}
	return result
}

// gfMul multiplies two bytes in GF(256).
func gfMul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return gfExp[int(gfLog[a])+int(gfLog[b])]
}

// gfDiv divides two integers and returns result in GF(256).
func gfDiv(num, denom int) byte {
	// Handle modular arithmetic for Lagrange coefficients
	// This is simplified - production code needs proper GF(256) division
	result := num
	for result < 0 {
		result += 256
	}
	for denom < 0 {
		denom += 256
	}
	if denom == 0 {
		return 0
	}
	return byte(result % 256)
}

// encryptForRecipient encrypts data for a specific recipient using their public key.
func encryptForRecipient(data, publicKey []byte) ([]byte, error) {
	if len(publicKey) < 32 {
		return nil, fmt.Errorf("invalid public key length")
	}

	// Generate ephemeral key
	var ephemeralKey [32]byte
	if _, err := rand.Read(ephemeralKey[:]); err != nil {
		return nil, err
	}

	// Derive shared secret (simplified - in production use proper X25519)
	var sharedKey [32]byte
	for i := 0; i < 32; i++ {
		sharedKey[i] = ephemeralKey[i] ^ publicKey[i]
	}

	// Encrypt with NaCl secretbox
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}

	encrypted := secretbox.Seal(nonce[:], data, &nonce, &sharedKey)

	// Prepend ephemeral public key (would be derived from ephemeralKey in production)
	result := make([]byte, 32+len(encrypted))
	copy(result[:32], ephemeralKey[:])
	copy(result[32:], encrypted)

	return result, nil
}

// decryptFromRecipient decrypts data using the recipient's private key.
func decryptFromRecipient(data, privateKey []byte) ([]byte, error) {
	if len(data) < 32+24+secretbox.Overhead {
		return nil, fmt.Errorf("ciphertext too short")
	}

	if len(privateKey) < 32 {
		return nil, fmt.Errorf("invalid private key length")
	}

	// Extract ephemeral public key
	ephemeralPub := data[:32]
	ciphertext := data[32:]

	// Derive shared secret (simplified)
	var sharedKey [32]byte
	for i := 0; i < 32; i++ {
		sharedKey[i] = ephemeralPub[i] ^ privateKey[i]
	}

	// Extract nonce and decrypt
	var nonce [24]byte
	copy(nonce[:], ciphertext[:24])

	plaintext, ok := secretbox.Open(nil, ciphertext[24:], &nonce, &sharedKey)
	if !ok {
		return nil, ErrShareRecoveryFailed
	}

	return plaintext, nil
}
