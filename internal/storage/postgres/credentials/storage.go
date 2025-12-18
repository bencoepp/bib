package credentials

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	// FilePermissions for credential storage files.
	FilePermissions = 0600

	// DirPermissions for credential storage directories.
	DirPermissions = 0700
)

var (
	// ErrCredentialsNotFound indicates no credentials file exists.
	ErrCredentialsNotFound = errors.New("credentials file not found")

	// ErrCorruptedCredentials indicates the credentials file is corrupted.
	ErrCorruptedCredentials = errors.New("corrupted credentials file")
)

// Storage handles encrypted credential persistence.
type Storage struct {
	path      string
	encryptor Encryptor
	mu        sync.RWMutex
}

// NewStorage creates a new credential storage.
func NewStorage(path string, encryptor Encryptor) (*Storage, error) {
	if path == "" {
		return nil, errors.New("storage path is required")
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, DirPermissions); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &Storage{
		path:      path,
		encryptor: encryptor,
	}, nil
}

// Load reads and decrypts credentials from disk.
func (s *Storage) Load() (*Credentials, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Read encrypted file
	encrypted, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrCredentialsNotFound
		}
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}

	if len(encrypted) == 0 {
		return nil, ErrCorruptedCredentials
	}

	// Decrypt
	plaintext, err := s.encryptor.Decrypt(encrypted)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt credentials: %w", err)
	}

	// Unmarshal
	var creds Credentials
	if err := json.Unmarshal(plaintext, &creds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal credentials: %w", err)
	}

	return &creds, nil
}

// Save encrypts and writes credentials to disk.
func (s *Storage) Save(creds *Credentials) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Marshal credentials
	plaintext, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	// Encrypt
	encrypted, err := s.encryptor.Encrypt(plaintext)
	if err != nil {
		return fmt.Errorf("failed to encrypt credentials: %w", err)
	}

	// Write to temp file first for atomic operation
	tempPath := s.path + ".tmp"
	if err := os.WriteFile(tempPath, encrypted, FilePermissions); err != nil {
		return fmt.Errorf("failed to write temp credentials file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, s.path); err != nil {
		os.Remove(tempPath) // Clean up temp file
		return fmt.Errorf("failed to rename credentials file: %w", err)
	}

	return nil
}

// Exists checks if credentials file exists.
func (s *Storage) Exists() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, err := os.Stat(s.path)
	return err == nil
}

// Delete removes the credentials file.
func (s *Storage) Delete() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete credentials file: %w", err)
	}
	return nil
}

// Path returns the storage path.
func (s *Storage) Path() string {
	return s.path
}

// Backup creates a backup of the current credentials.
func (s *Storage) Backup() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Read current file
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to backup
		}
		return fmt.Errorf("failed to read credentials for backup: %w", err)
	}

	// Write backup with timestamp
	backupPath := fmt.Sprintf("%s.backup", s.path)
	if err := os.WriteFile(backupPath, data, FilePermissions); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	return nil
}

// Restore restores credentials from backup.
func (s *Storage) Restore() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	backupPath := fmt.Sprintf("%s.backup", s.path)

	// Read backup file
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	// Write to main file
	if err := os.WriteFile(s.path, data, FilePermissions); err != nil {
		return fmt.Errorf("failed to restore credentials: %w", err)
	}

	return nil
}

// SecureDelete overwrites the credentials file before deletion.
func (s *Storage) SecureDelete() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get file size
	info, err := os.Stat(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to stat credentials file: %w", err)
	}

	// Open file for writing
	f, err := os.OpenFile(s.path, os.O_WRONLY, FilePermissions)
	if err != nil {
		return fmt.Errorf("failed to open credentials file: %w", err)
	}

	// Overwrite with zeros
	zeros := make([]byte, info.Size())
	if _, err := f.Write(zeros); err != nil {
		f.Close()
		return fmt.Errorf("failed to overwrite credentials file: %w", err)
	}

	// Sync to disk
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("failed to sync credentials file: %w", err)
	}

	f.Close()

	// Delete file
	if err := os.Remove(s.path); err != nil {
		return fmt.Errorf("failed to delete credentials file: %w", err)
	}

	return nil
}
