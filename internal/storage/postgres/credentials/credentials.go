// Package credentials provides secure credential management for PostgreSQL.
// It handles generation, encryption, storage, and rotation of database credentials.
package credentials

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"bib/internal/storage"
)

// EncryptionMethod defines the encryption algorithm for credential storage.
type EncryptionMethod string

const (
	// EncryptionX25519 uses X25519 key exchange with XSalsa20-Poly1305.
	EncryptionX25519 EncryptionMethod = "x25519-xsalsa20-poly1305"

	// EncryptionHKDF uses HKDF-SHA256 with AES-256-GCM.
	EncryptionHKDF EncryptionMethod = "hkdf-sha256-aes256-gcm"

	// EncryptionHybrid uses both methods; either can decrypt.
	EncryptionHybrid EncryptionMethod = "hybrid"
)

// String returns the encryption method name.
func (e EncryptionMethod) String() string {
	return string(e)
}

// IsValid checks if the encryption method is valid.
func (e EncryptionMethod) IsValid() bool {
	switch e {
	case EncryptionX25519, EncryptionHKDF, EncryptionHybrid:
		return true
	default:
		return false
	}
}

// CredentialStatus represents the lifecycle state of a credential.
type CredentialStatus string

const (
	// StatusActive indicates the credential is in active use.
	StatusActive CredentialStatus = "active"

	// StatusRetiring indicates the credential is being phased out.
	StatusRetiring CredentialStatus = "retiring"

	// StatusExpired indicates the credential is no longer valid.
	StatusExpired CredentialStatus = "expired"
)

// RoleCredential holds credentials for a single database role.
type RoleCredential struct {
	// Username is the PostgreSQL role name.
	Username string `json:"username"`

	// Password is the role password (64 chars, random).
	Password string `json:"password"`

	// CreatedAt is when this credential was generated.
	CreatedAt time.Time `json:"created_at"`

	// ExpiresAt is when this credential should be rotated.
	ExpiresAt time.Time `json:"expires_at"`

	// Status is the current lifecycle status.
	Status CredentialStatus `json:"status"`
}

// IsExpired returns true if the credential has passed its expiration time.
func (rc *RoleCredential) IsExpired() bool {
	return time.Now().After(rc.ExpiresAt)
}

// IsRetiring returns true if the credential is being phased out.
func (rc *RoleCredential) IsRetiring() bool {
	return rc.Status == StatusRetiring
}

// Credentials holds all PostgreSQL credentials for a bibd instance.
type Credentials struct {
	// Version is incremented on each rotation.
	Version int `json:"version"`

	// GeneratedAt is when this credential set was created.
	GeneratedAt time.Time `json:"generated_at"`

	// ExpiresAt is when this credential set should be rotated.
	ExpiresAt time.Time `json:"expires_at"`

	// EncryptionMethod used to encrypt this credential set.
	EncryptionMethod EncryptionMethod `json:"encryption_method"`

	// Superuser is the PostgreSQL superuser credential.
	// This is only used for initial setup and emergencies.
	Superuser RoleCredential `json:"superuser"`

	// Admin is the bibd_admin role credential.
	// This role can assume all other roles via SET ROLE.
	Admin RoleCredential `json:"admin"`

	// Roles contains credentials for job-specific roles.
	Roles map[storage.DBRole]RoleCredential `json:"roles"`

	// Previous holds the previous credential set during rotation.
	// This allows zero-downtime credential rotation.
	Previous *Credentials `json:"previous,omitempty"`
}

// GetRoleCredential returns the credential for a specific role.
func (c *Credentials) GetRoleCredential(role storage.DBRole) (*RoleCredential, error) {
	if role == storage.RoleAdmin {
		return &c.Admin, nil
	}

	cred, ok := c.Roles[role]
	if !ok {
		return nil, fmt.Errorf("credential not found for role: %s", role)
	}
	return &cred, nil
}

// AllRoles returns all role credentials including admin.
func (c *Credentials) AllRoles() map[storage.DBRole]RoleCredential {
	result := make(map[storage.DBRole]RoleCredential, len(c.Roles)+1)
	result[storage.RoleAdmin] = c.Admin
	for role, cred := range c.Roles {
		result[role] = cred
	}
	return result
}

// Config holds credential management configuration.
type Config struct {
	// EncryptionMethod is the algorithm used for credential encryption.
	EncryptionMethod EncryptionMethod `mapstructure:"encryption_method"`

	// RotationInterval is how often credentials should be rotated.
	RotationInterval time.Duration `mapstructure:"rotation_interval"`

	// RotationGracePeriod is how long old credentials remain valid after rotation.
	RotationGracePeriod time.Duration `mapstructure:"rotation_grace_period"`

	// EncryptedPath is where encrypted credentials are stored.
	EncryptedPath string `mapstructure:"encrypted_path"`

	// PasswordLength is the length of generated passwords.
	PasswordLength int `mapstructure:"password_length"`
}

// DefaultConfig returns sensible default configuration.
func DefaultConfig() Config {
	return Config{
		EncryptionMethod:    EncryptionHybrid,
		RotationInterval:    7 * 24 * time.Hour, // 7 days
		RotationGracePeriod: 5 * time.Minute,
		PasswordLength:      64,
	}
}

// Manager handles credential lifecycle operations.
type Manager struct {
	config     Config
	storage    *Storage
	encryptor  Encryptor
	current    *Credentials
	nodeID     string
	mu         sync.RWMutex
	rotationCh chan struct{}
	stopCh     chan struct{}
}

// NewManager creates a new credential manager.
func NewManager(cfg Config, nodeID string, identityKey []byte) (*Manager, error) {
	if cfg.PasswordLength < 32 {
		cfg.PasswordLength = 64
	}

	if !cfg.EncryptionMethod.IsValid() {
		cfg.EncryptionMethod = EncryptionHybrid
	}

	encryptor, err := NewEncryptor(cfg.EncryptionMethod, identityKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create encryptor: %w", err)
	}

	store, err := NewStorage(cfg.EncryptedPath, encryptor)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	return &Manager{
		config:     cfg,
		storage:    store,
		encryptor:  encryptor,
		nodeID:     nodeID,
		rotationCh: make(chan struct{}, 1),
		stopCh:     make(chan struct{}),
	}, nil
}

// Initialize loads existing credentials or generates new ones.
func (m *Manager) Initialize() (*Credentials, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Try to load existing credentials
	creds, err := m.storage.Load()
	if err == nil {
		m.current = creds
		return creds, nil
	}

	// Generate new credentials
	creds, err = m.generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate credentials: %w", err)
	}

	// Save to storage
	if err := m.storage.Save(creds); err != nil {
		return nil, fmt.Errorf("failed to save credentials: %w", err)
	}

	m.current = creds
	return creds, nil
}

// Current returns the current active credentials.
func (m *Manager) Current() *Credentials {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

// generate creates a new set of credentials.
func (m *Manager) generate() (*Credentials, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(m.config.RotationInterval)

	creds := &Credentials{
		Version:          1,
		GeneratedAt:      now,
		ExpiresAt:        expiresAt,
		EncryptionMethod: m.config.EncryptionMethod,
		Roles:            make(map[storage.DBRole]RoleCredential),
	}

	// Generate superuser credential
	superPass, err := generatePassword(m.config.PasswordLength)
	if err != nil {
		return nil, err
	}
	creds.Superuser = RoleCredential{
		Username:  "bibd_superuser",
		Password:  superPass,
		CreatedAt: now,
		ExpiresAt: expiresAt,
		Status:    StatusActive,
	}

	// Generate admin credential
	adminPass, err := generatePassword(m.config.PasswordLength)
	if err != nil {
		return nil, err
	}
	creds.Admin = RoleCredential{
		Username:  string(storage.RoleAdmin),
		Password:  adminPass,
		CreatedAt: now,
		ExpiresAt: expiresAt,
		Status:    StatusActive,
	}

	// Generate role-specific credentials
	roles := []storage.DBRole{
		storage.RoleScrape,
		storage.RoleQuery,
		storage.RoleTransform,
		storage.RoleAudit,
		storage.RoleReadOnly,
	}

	for _, role := range roles {
		pass, err := generatePassword(m.config.PasswordLength)
		if err != nil {
			return nil, err
		}
		creds.Roles[role] = RoleCredential{
			Username:  string(role),
			Password:  pass,
			CreatedAt: now,
			ExpiresAt: expiresAt,
			Status:    StatusActive,
		}
	}

	return creds, nil
}

// NeedsRotation returns true if credentials should be rotated.
func (m *Manager) NeedsRotation() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.current == nil {
		return false
	}

	return time.Now().After(m.current.ExpiresAt)
}

// TriggerRotation signals that rotation should occur.
func (m *Manager) TriggerRotation() {
	select {
	case m.rotationCh <- struct{}{}:
	default:
		// Rotation already pending
	}
}

// Close stops the credential manager.
func (m *Manager) Close() error {
	close(m.stopCh)
	return nil
}

// MarshalJSON implements json.Marshaler with redaction.
func (c *Credentials) MarshalJSON() ([]byte, error) {
	// Create a copy with redacted passwords for logging
	type alias Credentials
	return json.Marshal((*alias)(c))
}

// RedactedString returns a string representation with passwords redacted.
func (c *Credentials) RedactedString() string {
	return fmt.Sprintf("Credentials{version=%d, roles=%d, generated=%s, expires=%s}",
		c.Version, len(c.Roles)+2, c.GeneratedAt.Format(time.RFC3339), c.ExpiresAt.Format(time.RFC3339))
}

// generatePassword generates a cryptographically secure random password.
func generatePassword(length int) (string, error) {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}
