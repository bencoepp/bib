package auth

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"crypto/md5"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"bib/internal/domain"

	"golang.org/x/crypto/ssh"
)

// ParseSSHPublicKey parses an SSH public key and returns the key bytes and type.
// Supports Ed25519 and RSA keys.
func ParseSSHPublicKey(pubKey ssh.PublicKey) ([]byte, domain.KeyType, error) {
	switch key := pubKey.(type) {
	case ssh.CryptoPublicKey:
		cryptoKey := key.CryptoPublicKey()
		switch k := cryptoKey.(type) {
		case ed25519.PublicKey:
			return []byte(k), domain.KeyTypeEd25519, nil
		case *rsa.PublicKey:
			// Serialize RSA public key
			keyBytes, err := x509.MarshalPKIXPublicKey(k)
			if err != nil {
				return nil, "", fmt.Errorf("failed to marshal RSA public key: %w", err)
			}
			return keyBytes, domain.KeyTypeRSA, nil
		default:
			return nil, "", fmt.Errorf("unsupported key type: %T", k)
		}
	default:
		// Fallback to raw key bytes
		keyBytes := pubKey.Marshal()
		keyType := pubKey.Type()
		switch keyType {
		case ssh.KeyAlgoED25519:
			// Extract the actual Ed25519 public key bytes (skip the type prefix)
			if len(keyBytes) > ed25519.PublicKeySize {
				// SSH format includes a type prefix, extract just the key
				return keyBytes[len(keyBytes)-ed25519.PublicKeySize:], domain.KeyTypeEd25519, nil
			}
			return keyBytes, domain.KeyTypeEd25519, nil
		case ssh.KeyAlgoRSA:
			return keyBytes, domain.KeyTypeRSA, nil
		default:
			return nil, "", fmt.Errorf("unsupported SSH key type: %s", keyType)
		}
	}
}

// ParseAuthorizedKey parses an authorized_keys format public key.
func ParseAuthorizedKey(keyData []byte) ([]byte, domain.KeyType, error) {
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(keyData)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse authorized key: %w", err)
	}
	return ParseSSHPublicKey(pubKey)
}

// MarshalPublicKeyToAuthorizedKey converts a public key back to authorized_keys format.
func MarshalPublicKeyToAuthorizedKey(pubKey []byte, keyType domain.KeyType) ([]byte, error) {
	var sshPubKey ssh.PublicKey
	var err error

	switch keyType {
	case domain.KeyTypeEd25519:
		if len(pubKey) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("invalid Ed25519 public key size")
		}
		sshPubKey, err = ssh.NewPublicKey(ed25519.PublicKey(pubKey))
	case domain.KeyTypeRSA:
		// Parse the PKIX-encoded RSA public key
		key, parseErr := x509.ParsePKIXPublicKey(pubKey)
		if parseErr != nil {
			return nil, fmt.Errorf("failed to parse RSA public key: %w", parseErr)
		}
		rsaKey, ok := key.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("key is not RSA")
		}
		sshPubKey, err = ssh.NewPublicKey(rsaKey)
	default:
		return nil, fmt.Errorf("unsupported key type: %s", keyType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create SSH public key: %w", err)
	}

	return ssh.MarshalAuthorizedKey(sshPubKey), nil
}

// ParsePublicKeyAuto parses a public key in various formats:
// - OpenSSH authorized_keys format (ssh-ed25519 AAAA... or ssh-rsa AAAA...)
// - Raw Ed25519 public key bytes (32 bytes)
// - SSH wire format (from ssh.PublicKey.Marshal())
func ParsePublicKeyAuto(keyData []byte) ([]byte, domain.KeyType, error) {
	// Try OpenSSH authorized_keys format first (most common user input)
	if bytes.HasPrefix(keyData, []byte("ssh-")) || bytes.HasPrefix(keyData, []byte("ecdsa-")) {
		return ParseAuthorizedKey(keyData)
	}

	// Check if it's raw Ed25519 (exactly 32 bytes)
	if len(keyData) == ed25519.PublicKeySize {
		return keyData, domain.KeyTypeEd25519, nil
	}

	// Try to parse as SSH wire format
	sshPubKey, err := ssh.ParsePublicKey(keyData)
	if err == nil {
		return ParseSSHPublicKey(sshPubKey)
	}

	// Try base64 decode (in case it's the base64 part of authorized_keys without prefix)
	if decoded, err := base64.StdEncoding.DecodeString(string(keyData)); err == nil {
		if sshPubKey, err := ssh.ParsePublicKey(decoded); err == nil {
			return ParseSSHPublicKey(sshPubKey)
		}
	}

	return nil, "", fmt.Errorf("unable to parse public key: unrecognized format")
}

// PublicKeyInfo contains detailed information about a public key.
type PublicKeyInfo struct {
	// KeyType is the type of key (ed25519, rsa).
	KeyType domain.KeyType

	// KeyBytes is the canonical key bytes.
	KeyBytes []byte

	// FingerprintSHA256 is the SHA256 fingerprint in hex format.
	FingerprintSHA256 string

	// FingerprintMD5 is the MD5 fingerprint in hex format (for compatibility).
	FingerprintMD5 string

	// KeySize is the key size in bits.
	KeySize int

	// OpenSSHFormat is the key in OpenSSH authorized_keys format.
	OpenSSHFormat string
}

// GetPublicKeyInfo extracts detailed information from a public key.
func GetPublicKeyInfo(keyData []byte) (*PublicKeyInfo, error) {
	keyBytes, keyType, err := ParsePublicKeyAuto(keyData)
	if err != nil {
		return nil, err
	}

	info := &PublicKeyInfo{
		KeyType:  keyType,
		KeyBytes: keyBytes,
	}

	// Calculate fingerprints
	info.FingerprintSHA256 = calculateFingerprintSHA256(keyBytes)
	info.FingerprintMD5 = calculateFingerprintMD5(keyBytes)

	// Determine key size
	switch keyType {
	case domain.KeyTypeEd25519:
		info.KeySize = 256 // Ed25519 is always 256-bit
	case domain.KeyTypeRSA:
		if key, err := x509.ParsePKIXPublicKey(keyBytes); err == nil {
			if rsaKey, ok := key.(*rsa.PublicKey); ok {
				info.KeySize = rsaKey.N.BitLen()
			}
		}
	}

	// Generate OpenSSH format
	if openssh, err := MarshalPublicKeyToAuthorizedKey(keyBytes, keyType); err == nil {
		info.OpenSSHFormat = strings.TrimSpace(string(openssh))
	}

	return info, nil
}

// calculateFingerprintSHA256 calculates SHA256 fingerprint of key bytes.
func calculateFingerprintSHA256(keyBytes []byte) string {
	hash := sha256.Sum256(keyBytes)
	return hex.EncodeToString(hash[:])
}

// calculateFingerprintMD5 calculates MD5 fingerprint of key bytes.
func calculateFingerprintMD5(keyBytes []byte) string {
	hash := md5.Sum(keyBytes)
	return hex.EncodeToString(hash[:])
}

// VerifySignature verifies a signature against a message using the given public key.
func VerifySignature(keyBytes []byte, keyType domain.KeyType, message, signature []byte) error {
	switch keyType {
	case domain.KeyTypeEd25519:
		if len(keyBytes) != ed25519.PublicKeySize {
			return fmt.Errorf("invalid Ed25519 public key size: got %d, want %d", len(keyBytes), ed25519.PublicKeySize)
		}
		if !ed25519.Verify(keyBytes, message, signature) {
			return fmt.Errorf("Ed25519 signature verification failed")
		}
		return nil

	case domain.KeyTypeRSA:
		// Parse PKIX-encoded RSA public key
		key, err := x509.ParsePKIXPublicKey(keyBytes)
		if err != nil {
			return fmt.Errorf("failed to parse RSA public key: %w", err)
		}
		rsaKey, ok := key.(*rsa.PublicKey)
		if !ok {
			return fmt.Errorf("key is not RSA")
		}

		// Calculate SHA256 hash of message
		hash := sha256.Sum256(message)

		// Verify PKCS1v15 signature with SHA256
		if err := rsa.VerifyPKCS1v15(rsaKey, crypto.SHA256, hash[:], signature); err != nil {
			return fmt.Errorf("RSA signature verification failed: %w", err)
		}
		return nil

	default:
		return fmt.Errorf("unsupported key type: %s", keyType)
	}
}

// VerifySSHSignature verifies an SSH signature against a message.
// This handles the SSH signature format which includes algorithm info.
func VerifySSHSignature(keyBytes []byte, keyType domain.KeyType, message, signature []byte) error {
	// First try direct verification
	if err := VerifySignature(keyBytes, keyType, message, signature); err == nil {
		return nil
	}

	// Try parsing as SSH signature format
	sshSig := new(ssh.Signature)
	if err := ssh.Unmarshal(signature, sshSig); err != nil {
		// Not SSH format, return original error
		return VerifySignature(keyBytes, keyType, message, signature)
	}

	// Verify using extracted signature blob
	return VerifySignature(keyBytes, keyType, message, sshSig.Blob)
}
