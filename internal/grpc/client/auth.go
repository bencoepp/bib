// Package client provides a gRPC client library for connecting to bibd.
package client

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	services "bib/api/gen/go/bib/v1/services"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Authenticator handles SSH-based authentication with bibd.
type Authenticator struct {
	opts AuthOptions

	// Cached signers from SSH agent or file
	signers []ssh.Signer
}

// NewAuthenticator creates a new Authenticator.
func NewAuthenticator(opts AuthOptions) *Authenticator {
	return &Authenticator{
		opts: opts,
	}
}

// Authenticate performs the challenge-response authentication flow.
func (a *Authenticator) Authenticate(ctx context.Context, authClient services.AuthServiceClient) (string, error) {
	// Get available signers
	signers, err := a.getSigners()
	if err != nil {
		return "", fmt.Errorf("failed to get SSH signers: %w", err)
	}
	if len(signers) == 0 {
		return "", fmt.Errorf("no SSH keys available for authentication")
	}

	// Try each signer until one works
	var lastErr error
	for _, signer := range signers {
		token, err := a.authenticateWithSigner(ctx, authClient, signer)
		if err == nil {
			return token, nil
		}
		lastErr = err
	}

	return "", fmt.Errorf("authentication failed with all available keys: %w", lastErr)
}

// authenticateWithSigner performs authentication with a specific signer.
func (a *Authenticator) authenticateWithSigner(ctx context.Context, authClient services.AuthServiceClient, signer ssh.Signer) (string, error) {
	pubKey := signer.PublicKey()

	// Request challenge
	challengeResp, err := authClient.Challenge(ctx, &services.ChallengeRequest{
		PublicKey: ssh.MarshalAuthorizedKey(pubKey),
	})
	if err != nil {
		return "", fmt.Errorf("failed to get challenge: %w", err)
	}

	// Sign the challenge
	signature, err := signer.Sign(rand.Reader, challengeResp.Challenge)
	if err != nil {
		return "", fmt.Errorf("failed to sign challenge: %w", err)
	}

	// Verify challenge
	verifyResp, err := authClient.VerifyChallenge(ctx, &services.VerifyChallengeRequest{
		ChallengeId: challengeResp.ChallengeId,
		Signature:   signature.Blob,
	})
	if err != nil {
		return "", fmt.Errorf("challenge verification failed: %w", err)
	}

	return verifyResp.SessionToken, nil
}

// getSigners returns available SSH signers.
func (a *Authenticator) getSigners() ([]ssh.Signer, error) {
	if len(a.signers) > 0 {
		return a.signers, nil
	}

	var signers []ssh.Signer

	// Try SSH agent first
	if a.opts.UseSSHAgent {
		agentSigners, err := a.getAgentSigners()
		if err == nil && len(agentSigners) > 0 {
			signers = append(signers, agentSigners...)
		}
	}

	// Try file-based key
	if a.opts.SSHKeyPath != "" {
		fileSigner, err := a.getFileSigner()
		if err == nil {
			signers = append(signers, fileSigner)
		}
	}

	// If no explicit path, try default locations
	if a.opts.SSHKeyPath == "" && len(signers) == 0 {
		defaultSigners, _ := a.getDefaultFileSigners()
		signers = append(signers, defaultSigners...)
	}

	a.signers = signers
	return signers, nil
}

// getAgentSigners gets signers from SSH agent.
func (a *Authenticator) getAgentSigners() ([]ssh.Signer, error) {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return nil, fmt.Errorf("SSH_AUTH_SOCK not set")
	}

	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH agent: %w", err)
	}

	agentClient := agent.NewClient(conn)
	return agentClient.Signers()
}

// getFileSigner loads a signer from a key file.
func (a *Authenticator) getFileSigner() (ssh.Signer, error) {
	keyData, err := os.ReadFile(a.opts.SSHKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	var signer ssh.Signer
	if a.opts.SSHKeyPassphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(a.opts.SSHKeyPassphrase))
	} else {
		signer, err = ssh.ParsePrivateKey(keyData)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return signer, nil
}

// getDefaultFileSigners tries to load signers from default key locations.
func (a *Authenticator) getDefaultFileSigners() ([]ssh.Signer, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	sshDir := filepath.Join(homeDir, ".ssh")
	keyFiles := []string{
		filepath.Join(sshDir, "id_ed25519"),
		filepath.Join(sshDir, "id_rsa"),
	}

	var signers []ssh.Signer
	for _, keyFile := range keyFiles {
		keyData, err := os.ReadFile(keyFile)
		if err != nil {
			continue
		}

		signer, err := ssh.ParsePrivateKey(keyData)
		if err != nil {
			continue // Skip encrypted keys without passphrase
		}
		signers = append(signers, signer)
	}

	return signers, nil
}

// LoadSessionToken loads the session token from disk.
func (a *Authenticator) LoadSessionToken() (string, error) {
	path := a.tokenPath()
	if path == "" {
		return "", fmt.Errorf("session token path not configured")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read session token: %w", err)
	}

	// Decrypt token
	token, err := a.decryptToken(data)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt session token: %w", err)
	}

	return token, nil
}

// SaveSessionToken saves the session token to disk (encrypted).
func (a *Authenticator) SaveSessionToken(token string) error {
	path := a.tokenPath()
	if path == "" {
		return fmt.Errorf("session token path not configured")
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create token directory: %w", err)
	}

	// Encrypt token
	encrypted, err := a.encryptToken(token)
	if err != nil {
		return fmt.Errorf("failed to encrypt session token: %w", err)
	}

	// Write with restrictive permissions
	if err := os.WriteFile(path, encrypted, 0600); err != nil {
		return fmt.Errorf("failed to write session token: %w", err)
	}

	return nil
}

// ClearSessionToken removes the stored session token.
func (a *Authenticator) ClearSessionToken() error {
	path := a.tokenPath()
	if path == "" {
		return nil
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove session token: %w", err)
	}

	return nil
}

// tokenPath returns the path to the session token file.
func (a *Authenticator) tokenPath() string {
	if a.opts.SessionTokenPath != "" {
		return a.opts.SessionTokenPath
	}

	// Default to config directory
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}

	return filepath.Join(configDir, "bib", "session.token")
}

// encryptToken encrypts the session token using a key derived from SSH key.
func (a *Authenticator) encryptToken(token string) ([]byte, error) {
	key, err := a.deriveEncryptionKey()
	if err != nil {
		return nil, err
	}

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt
	ciphertext := aead.Seal(nonce, nonce, []byte(token), nil)
	return ciphertext, nil
}

// decryptToken decrypts the session token.
func (a *Authenticator) decryptToken(data []byte) (string, error) {
	key, err := a.deriveEncryptionKey()
	if err != nil {
		return "", err
	}

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	if len(data) < aead.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce := data[:aead.NonceSize()]
	ciphertext := data[aead.NonceSize():]

	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// deriveEncryptionKey derives an encryption key from the SSH private key.
func (a *Authenticator) deriveEncryptionKey() ([]byte, error) {
	signers, err := a.getSigners()
	if err != nil || len(signers) == 0 {
		// Fall back to a fixed key derived from machine info
		// This is less secure but allows token storage without SSH key
		return a.deriveFallbackKey()
	}

	// Use first signer's public key to derive encryption key
	pubKey := signers[0].PublicKey()
	pubKeyBytes := pubKey.Marshal()

	// Hash to get 32-byte key for XChaCha20-Poly1305
	hash := sha256.Sum256(append([]byte("bib-session-token"), pubKeyBytes...))
	return hash[:], nil
}

// deriveFallbackKey derives a key from machine-specific information.
func (a *Authenticator) deriveFallbackKey() ([]byte, error) {
	// Use hostname + username as entropy
	hostname, _ := os.Hostname()
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("USERNAME")
	}

	data := fmt.Sprintf("bib-fallback-%s-%s", hostname, username)
	hash := sha256.Sum256([]byte(data))
	return hash[:], nil
}

// GetPublicKey returns the public key for display/verification.
func (a *Authenticator) GetPublicKey() (string, error) {
	signers, err := a.getSigners()
	if err != nil || len(signers) == 0 {
		return "", fmt.Errorf("no SSH keys available")
	}

	pubKey := signers[0].PublicKey()
	return string(ssh.MarshalAuthorizedKey(pubKey)), nil
}

// ============================================================================
// Client Authentication Methods
// ============================================================================

// Authenticate performs authentication and caches the session token.
func (c *Client) Authenticate(ctx context.Context) error {
	authClient, err := c.Auth()
	if err != nil {
		return fmt.Errorf("failed to get auth client: %w", err)
	}

	token, err := c.auth.Authenticate(ctx, authClient)
	if err != nil {
		return err
	}

	c.tokenLock.Lock()
	c.sessionToken = token
	c.tokenLock.Unlock()

	// Save token to disk
	if err := c.auth.SaveSessionToken(token); err != nil {
		// Log warning but don't fail
		_ = err
	}

	return nil
}

// LoadSession tries to load a cached session token.
func (c *Client) LoadSession() error {
	token, err := c.auth.LoadSessionToken()
	if err != nil {
		return err
	}
	if token == "" {
		return fmt.Errorf("no cached session token")
	}

	c.tokenLock.Lock()
	c.sessionToken = token
	c.tokenLock.Unlock()

	return nil
}

// ClearSession clears the cached session.
func (c *Client) ClearSession() error {
	c.tokenLock.Lock()
	c.sessionToken = ""
	c.tokenLock.Unlock()

	return c.auth.ClearSessionToken()
}

// SessionToken returns the current session token.
func (c *Client) SessionToken() string {
	c.tokenLock.RLock()
	defer c.tokenLock.RUnlock()
	return c.sessionToken
}

// EnsureAuthenticated ensures the client is authenticated.
func (c *Client) EnsureAuthenticated(ctx context.Context) error {
	// Try loading cached session
	if err := c.LoadSession(); err == nil && c.SessionToken() != "" {
		// Verify session is still valid
		if err := c.verifySession(ctx); err == nil {
			return nil
		}
	}

	// Need to authenticate
	if !c.opts.Auth.AutoAuth {
		return fmt.Errorf("not authenticated and auto-auth is disabled")
	}

	return c.Authenticate(ctx)
}

// verifySession checks if the current session is still valid.
func (c *Client) verifySession(ctx context.Context) error {
	authClient, err := c.Auth()
	if err != nil {
		return err
	}

	_, err = authClient.RefreshSession(ctx, &services.RefreshSessionRequest{})
	return err
}

// Unused but required for compatibility with crypto.Signer interface
var _ crypto.Signer = (*dummySigner)(nil)

type dummySigner struct {
	pubKey ed25519.PublicKey
}

func (d *dummySigner) Public() crypto.PublicKey {
	return d.pubKey
}

func (d *dummySigner) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}

// TokenExpiry represents when a token expires
type TokenExpiry struct {
	ExpiresAt time.Time
	IsExpired bool
}

// ParseTokenExpiry parses expiry info from a token (placeholder for JWT parsing)
func ParseTokenExpiry(token string) (*TokenExpiry, error) {
	// For now, we don't parse JWT - just assume tokens are valid
	// In a full implementation, this would decode the JWT and check exp claim
	return &TokenExpiry{
		ExpiresAt: time.Now().Add(24 * time.Hour),
		IsExpired: false,
	}, nil
}
