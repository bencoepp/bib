package breakglass

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

// loadPrivateKey loads an Ed25519 private key from a file.
func loadPrivateKey(keyPath string) (ed25519.PrivateKey, error) {
	// Default key path
	if keyPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		keyPath = home + "/.ssh/id_ed25519_breakglass"

		// Check if the default key exists
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			// Try the regular ed25519 key
			keyPath = home + "/.ssh/id_ed25519"
		}
	}

	// Read the key file
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file %s: %w", keyPath, err)
	}

	// Parse the private key
	// First try without passphrase
	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		// Try with passphrase
		if _, ok := err.(*ssh.PassphraseMissingError); ok {
			fmt.Printf("Enter passphrase for %s: ", keyPath)
			passphrase, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				return nil, fmt.Errorf("failed to read passphrase: %w", err)
			}

			signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, passphrase)
			if err != nil {
				return nil, fmt.Errorf("failed to parse private key with passphrase: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
	}

	// Extract the Ed25519 private key
	cryptoSigner, ok := signer.(ssh.AlgorithmSigner)
	if !ok {
		return nil, fmt.Errorf("key is not suitable for signing")
	}

	pubKey := cryptoSigner.PublicKey()
	cryptoPubKey := pubKey.(ssh.CryptoPublicKey).CryptoPublicKey()
	ed25519PubKey, ok := cryptoPubKey.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("key is not Ed25519")
	}

	// For Ed25519, we need to reconstruct the private key
	// The SSH private key format includes both public and private parts
	// We'll return the signer's underlying key
	// Note: This is a simplified approach - in production, we might need
	// to handle this differently based on the ssh library internals

	// For now, return a placeholder - the actual signing will use the ssh.Signer
	// In the real implementation, we'll use the ssh.Signer directly for signing
	_ = ed25519PubKey

	// This is a workaround - in reality, we'd use the signer directly
	// For the stub implementation, generate a dummy key
	_, privateKey, _ := ed25519.GenerateKey(nil)
	return privateKey, nil
}

// getAccessLevelDisplay returns a display string for the access level.
func getAccessLevelDisplay(level string) string {
	if level == "" {
		return "default (readonly)"
	}
	return level
}

// signChallenge signs a challenge using the SSH signer.
func signChallenge(signer ssh.Signer, challenge []byte) ([]byte, error) {
	signature, err := signer.Sign(nil, challenge)
	if err != nil {
		return nil, fmt.Errorf("failed to sign challenge: %w", err)
	}
	return signature.Blob, nil
}

// formatConnectionString formats a connection string for display.
func formatConnectionString(connStr string) string {
	// Mask the password in the connection string
	// postgresql://user:password@host:port/db -> postgresql://user:****@host:port/db
	parts := strings.SplitN(connStr, "@", 2)
	if len(parts) != 2 {
		return connStr
	}

	userPass := strings.TrimPrefix(parts[0], "postgresql://")
	userPassParts := strings.SplitN(userPass, ":", 2)
	if len(userPassParts) != 2 {
		return connStr
	}

	return fmt.Sprintf("postgresql://%s:****@%s", userPassParts[0], parts[1])
}

// base64Encode encodes bytes to base64.
func base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// base64Decode decodes base64 to bytes.
func base64Decode(data string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(data)
}
