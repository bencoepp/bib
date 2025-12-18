// Package encryption provides encryption at rest for PostgreSQL data.
// It supports multiple encryption methods: LUKS (Linux), application-level,
// and PostgreSQL TDE.
package encryption

import (
	"context"
	"errors"
	"runtime"
	"time"
)

var (
	// ErrNotSupported indicates the encryption method is not supported on this platform.
	ErrNotSupported = errors.New("encryption method not supported on this platform")

	// ErrNotInitialized indicates the encryption has not been initialized.
	ErrNotInitialized = errors.New("encryption not initialized")

	// ErrAlreadyInitialized indicates the encryption is already initialized.
	ErrAlreadyInitialized = errors.New("encryption already initialized")

	// ErrInvalidKey indicates the encryption key is invalid.
	ErrInvalidKey = errors.New("invalid encryption key")

	// ErrMountFailed indicates mounting the encrypted volume failed.
	ErrMountFailed = errors.New("failed to mount encrypted volume")

	// ErrDecryptFailed indicates decryption failed.
	ErrDecryptFailed = errors.New("decryption failed")
)

// Method represents an encryption at rest method.
type Method string

const (
	// MethodNone disables encryption at rest.
	MethodNone Method = "none"

	// MethodLUKS uses LUKS/dm-crypt for full disk encryption (Linux only).
	MethodLUKS Method = "luks"

	// MethodTDE uses PostgreSQL Transparent Data Encryption.
	MethodTDE Method = "tde"

	// MethodApplication uses application-level field encryption.
	MethodApplication Method = "application"

	// MethodHybrid uses multiple encryption methods together.
	MethodHybrid Method = "hybrid"
)

// String returns the method name.
func (m Method) String() string {
	return string(m)
}

// IsValid returns true if the method is valid.
func (m Method) IsValid() bool {
	switch m {
	case MethodNone, MethodLUKS, MethodTDE, MethodApplication, MethodHybrid:
		return true
	default:
		return false
	}
}

// EncryptionStatus represents the status of encryption at rest.
type EncryptionStatus struct {
	Method         Method    `json:"method"`
	Initialized    bool      `json:"initialized"`
	Active         bool      `json:"active"`
	KeyCreatedAt   time.Time `json:"key_created_at,omitempty"`
	LastVerifiedAt time.Time `json:"last_verified_at,omitempty"`
	VolumeSize     int64     `json:"volume_size,omitempty"`
	UsedSpace      int64     `json:"used_space,omitempty"`
	Warnings       []string  `json:"warnings,omitempty"`
}

// VolumeEncryption provides disk-level encryption.
type VolumeEncryption interface {
	// IsSupported checks if this encryption method is available.
	IsSupported() bool

	// Initialize sets up encryption for the data directory.
	Initialize(ctx context.Context, dataDir string, key []byte) error

	// Mount makes the encrypted volume accessible.
	Mount(ctx context.Context, key []byte) error

	// Unmount securely unmounts the encrypted volume.
	Unmount(ctx context.Context) error

	// Status returns the encryption status.
	Status(ctx context.Context) (*EncryptionStatus, error)

	// Method returns the encryption method.
	Method() Method
}

// ColumnEncryption provides application-level field encryption.
type ColumnEncryption interface {
	// Encrypt encrypts a value for storage.
	Encrypt(plaintext []byte) ([]byte, error)

	// Decrypt decrypts a stored value.
	Decrypt(ciphertext []byte) ([]byte, error)

	// EncryptString is a convenience method for strings.
	EncryptString(plaintext string) (string, error)

	// DecryptString decrypts a string.
	DecryptString(ciphertext string) (string, error)

	// EncryptJSON encrypts a JSON-serializable value.
	EncryptJSON(value interface{}) ([]byte, error)

	// DecryptJSON decrypts and unmarshals a JSON value.
	DecryptJSON(ciphertext []byte, target interface{}) error
}

// Config holds encryption at rest configuration.
type Config struct {
	// Enabled controls whether encryption at rest is active.
	Enabled bool `mapstructure:"enabled"`

	// Method is the primary encryption method.
	Method Method `mapstructure:"method"`

	// LUKS holds LUKS-specific configuration.
	LUKS LUKSConfig `mapstructure:"luks"`

	// TDE holds PostgreSQL TDE configuration.
	TDE TDEConfig `mapstructure:"tde"`

	// Application holds application-level encryption configuration.
	Application ApplicationConfig `mapstructure:"application"`

	// Recovery holds key recovery configuration.
	Recovery RecoveryConfig `mapstructure:"recovery"`
}

// LUKSConfig holds LUKS-specific configuration.
type LUKSConfig struct {
	// VolumeSize is the size of the encrypted volume.
	VolumeSize string `mapstructure:"volume_size"`

	// Cipher is the encryption cipher (e.g., "aes-xts-plain64").
	Cipher string `mapstructure:"cipher"`

	// KeySize is the encryption key size in bits.
	KeySize int `mapstructure:"key_size"`

	// HashAlgorithm is the hash for key derivation.
	HashAlgorithm string `mapstructure:"hash_algorithm"`
}

// TDEConfig holds PostgreSQL TDE configuration.
type TDEConfig struct {
	// Algorithm is the encryption algorithm.
	Algorithm string `mapstructure:"algorithm"`

	// EncryptWAL enables WAL encryption.
	EncryptWAL bool `mapstructure:"encrypt_wal"`
}

// ApplicationConfig holds application-level encryption configuration.
type ApplicationConfig struct {
	// Algorithm is the encryption algorithm ("aes-256-gcm" or "chacha20-poly1305").
	Algorithm string `mapstructure:"algorithm"`

	// EncryptedFields defines which fields to encrypt.
	EncryptedFields []EncryptedField `mapstructure:"encrypted_fields"`
}

// EncryptedField specifies a database field to encrypt.
type EncryptedField struct {
	Table   string   `mapstructure:"table"`
	Columns []string `mapstructure:"columns"`
}

// RecoveryConfig holds key recovery configuration.
type RecoveryConfig struct {
	// Method is the recovery method ("shamir" or "backup").
	Method string `mapstructure:"method"`

	// Shamir holds Shamir's Secret Sharing configuration.
	Shamir ShamirConfig `mapstructure:"shamir"`
}

// ShamirConfig holds Shamir's Secret Sharing configuration.
type ShamirConfig struct {
	// TotalShares is the total number of shares to create.
	TotalShares int `mapstructure:"total_shares"`

	// Threshold is the minimum shares needed to recover.
	Threshold int `mapstructure:"threshold"`

	// ShareholderIDs are identifiers for each share.
	ShareholderIDs []string `mapstructure:"shareholder_ids"`
}

// DefaultConfig returns sensible encryption defaults.
func DefaultConfig() Config {
	return Config{
		Enabled: false, // Disabled by default
		Method:  MethodApplication,
		LUKS: LUKSConfig{
			VolumeSize:    "50GB",
			Cipher:        "aes-xts-plain64",
			KeySize:       512,
			HashAlgorithm: "sha512",
		},
		TDE: TDEConfig{
			Algorithm:  "aes-256",
			EncryptWAL: true,
		},
		Application: ApplicationConfig{
			Algorithm: "aes-256-gcm",
			EncryptedFields: []EncryptedField{
				{Table: "datasets", Columns: []string{"content", "metadata"}},
				{Table: "jobs", Columns: []string{"parameters", "result"}},
				{Table: "nodes", Columns: []string{"metadata"}},
			},
		},
		Recovery: RecoveryConfig{
			Method: "shamir",
			Shamir: ShamirConfig{
				TotalShares: 5,
				Threshold:   3,
			},
		},
	}
}

// Manager manages encryption at rest.
type Manager struct {
	config           Config
	volumeEncryption VolumeEncryption
	columnEncryption ColumnEncryption
	keyManager       *KeyManager
	initialized      bool
}

// NewManager creates a new encryption manager.
func NewManager(cfg Config, identityKey []byte) (*Manager, error) {
	m := &Manager{
		config: cfg,
	}

	if !cfg.Enabled {
		return m, nil
	}

	// Create key manager
	keyMgr, err := NewKeyManager(identityKey, cfg.Recovery)
	if err != nil {
		return nil, err
	}
	m.keyManager = keyMgr

	// Set up volume encryption if applicable
	switch cfg.Method {
	case MethodLUKS:
		if runtime.GOOS != "linux" {
			return nil, ErrNotSupported
		}
		m.volumeEncryption = NewLUKSEncryption(cfg.LUKS)
	case MethodTDE:
		m.volumeEncryption = NewTDEEncryption(cfg.TDE)
	case MethodApplication:
		colEnc, err := NewApplicationEncryption(cfg.Application, keyMgr.DeriveKey("application-encryption"))
		if err != nil {
			return nil, err
		}
		m.columnEncryption = colEnc
	case MethodHybrid:
		// Set up both volume and column encryption
		if runtime.GOOS == "linux" {
			m.volumeEncryption = NewLUKSEncryption(cfg.LUKS)
		}
		colEnc, err := NewApplicationEncryption(cfg.Application, keyMgr.DeriveKey("application-encryption"))
		if err != nil {
			return nil, err
		}
		m.columnEncryption = colEnc
	}

	return m, nil
}

// Initialize sets up encryption at rest.
func (m *Manager) Initialize(ctx context.Context, dataDir string) error {
	if !m.config.Enabled {
		return nil
	}

	if m.initialized {
		return ErrAlreadyInitialized
	}

	if m.volumeEncryption != nil && m.volumeEncryption.IsSupported() {
		key := m.keyManager.DeriveKey("volume-encryption")
		if err := m.volumeEncryption.Initialize(ctx, dataDir, key); err != nil {
			return err
		}
	}

	m.initialized = true
	return nil
}

// Mount makes the encrypted storage accessible.
func (m *Manager) Mount(ctx context.Context) error {
	if !m.config.Enabled || m.volumeEncryption == nil {
		return nil
	}

	if !m.volumeEncryption.IsSupported() {
		return nil
	}

	key := m.keyManager.DeriveKey("volume-encryption")
	return m.volumeEncryption.Mount(ctx, key)
}

// Unmount securely unmounts the encrypted storage.
func (m *Manager) Unmount(ctx context.Context) error {
	if !m.config.Enabled || m.volumeEncryption == nil {
		return nil
	}

	if !m.volumeEncryption.IsSupported() {
		return nil
	}

	return m.volumeEncryption.Unmount(ctx)
}

// EncryptField encrypts a field value for storage.
func (m *Manager) EncryptField(table, column string, value []byte) ([]byte, error) {
	if !m.config.Enabled || m.columnEncryption == nil {
		return value, nil
	}

	if !m.shouldEncrypt(table, column) {
		return value, nil
	}

	return m.columnEncryption.Encrypt(value)
}

// DecryptField decrypts a field value from storage.
func (m *Manager) DecryptField(table, column string, value []byte) ([]byte, error) {
	if !m.config.Enabled || m.columnEncryption == nil {
		return value, nil
	}

	if !m.shouldEncrypt(table, column) {
		return value, nil
	}

	return m.columnEncryption.Decrypt(value)
}

// shouldEncrypt checks if a field should be encrypted.
func (m *Manager) shouldEncrypt(table, column string) bool {
	for _, field := range m.config.Application.EncryptedFields {
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

// Status returns the encryption status.
func (m *Manager) Status(ctx context.Context) (*EncryptionStatus, error) {
	status := &EncryptionStatus{
		Method:      m.config.Method,
		Initialized: m.initialized,
		Active:      m.config.Enabled,
	}

	if m.volumeEncryption != nil && m.volumeEncryption.IsSupported() {
		volStatus, err := m.volumeEncryption.Status(ctx)
		if err == nil {
			status.VolumeSize = volStatus.VolumeSize
			status.UsedSpace = volStatus.UsedSpace
			status.KeyCreatedAt = volStatus.KeyCreatedAt
		}
	}

	return status, nil
}

// GenerateRecoveryShares generates key recovery shares using Shamir's Secret Sharing.
func (m *Manager) GenerateRecoveryShares() ([]Share, error) {
	if m.keyManager == nil {
		return nil, ErrNotInitialized
	}

	return m.keyManager.GenerateRecoveryShares()
}

// RecoverFromShares recovers the encryption key from shares.
func (m *Manager) RecoverFromShares(shares []Share) error {
	if m.keyManager == nil {
		return ErrNotInitialized
	}

	return m.keyManager.RecoverFromShares(shares)
}

// Close cleans up encryption resources.
func (m *Manager) Close(ctx context.Context) error {
	return m.Unmount(ctx)
}
