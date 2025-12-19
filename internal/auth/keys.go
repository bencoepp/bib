package auth

import (
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"fmt"

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
