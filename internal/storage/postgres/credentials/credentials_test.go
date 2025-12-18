package credentials

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"bib/internal/storage"
)

func TestGeneratePassword(t *testing.T) {
	password, err := generatePassword(64)
	if err != nil {
		t.Fatalf("failed to generate password: %v", err)
	}

	if len(password) != 64 {
		t.Errorf("expected password length 64, got %d", len(password))
	}

	// Verify randomness - generate another and ensure they're different
	password2, err := generatePassword(64)
	if err != nil {
		t.Fatalf("failed to generate second password: %v", err)
	}

	if password == password2 {
		t.Error("two generated passwords should be different")
	}
}

func TestEncryptionMethods(t *testing.T) {
	identityKey := make([]byte, 64)
	for i := range identityKey {
		identityKey[i] = byte(i)
	}

	testData := []byte("test credential data for encryption")

	methods := []EncryptionMethod{
		EncryptionX25519,
		EncryptionHKDF,
		EncryptionHybrid,
	}

	for _, method := range methods {
		t.Run(string(method), func(t *testing.T) {
			encryptor, err := NewEncryptor(method, identityKey)
			if err != nil {
				t.Fatalf("failed to create encryptor: %v", err)
			}

			if encryptor.Method() != method {
				t.Errorf("expected method %s, got %s", method, encryptor.Method())
			}

			// Encrypt
			ciphertext, err := encryptor.Encrypt(testData)
			if err != nil {
				t.Fatalf("encryption failed: %v", err)
			}

			// Ciphertext should be different from plaintext
			if string(ciphertext) == string(testData) {
				t.Error("ciphertext should differ from plaintext")
			}

			// Decrypt
			plaintext, err := encryptor.Decrypt(ciphertext)
			if err != nil {
				t.Fatalf("decryption failed: %v", err)
			}

			if string(plaintext) != string(testData) {
				t.Errorf("decrypted data doesn't match: got %q, want %q", plaintext, testData)
			}
		})
	}
}

func TestCredentialsStorage(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "bib-credentials-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	identityKey := make([]byte, 64)
	for i := range identityKey {
		identityKey[i] = byte(i)
	}

	encryptor, err := NewEncryptor(EncryptionHybrid, identityKey)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	storagePath := filepath.Join(tmpDir, "secrets", "db.enc")
	store, err := NewStorage(storagePath, encryptor)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Create test credentials
	creds := &Credentials{
		Version:          1,
		GeneratedAt:      time.Now().UTC(),
		ExpiresAt:        time.Now().UTC().Add(7 * 24 * time.Hour),
		EncryptionMethod: EncryptionHybrid,
		Superuser: RoleCredential{
			Username:  "bibd_superuser",
			Password:  "super-secret-password",
			CreatedAt: time.Now().UTC(),
			ExpiresAt: time.Now().UTC().Add(7 * 24 * time.Hour),
			Status:    StatusActive,
		},
		Admin: RoleCredential{
			Username:  "bibd_admin",
			Password:  "admin-password",
			CreatedAt: time.Now().UTC(),
			ExpiresAt: time.Now().UTC().Add(7 * 24 * time.Hour),
			Status:    StatusActive,
		},
		Roles: map[storage.DBRole]RoleCredential{
			storage.RoleScrape: {
				Username:  "bibd_scrape",
				Password:  "scrape-password",
				CreatedAt: time.Now().UTC(),
				ExpiresAt: time.Now().UTC().Add(7 * 24 * time.Hour),
				Status:    StatusActive,
			},
		},
	}

	// Save credentials
	if err := store.Save(creds); err != nil {
		t.Fatalf("failed to save credentials: %v", err)
	}

	// Verify file exists
	if !store.Exists() {
		t.Error("credentials file should exist after save")
	}

	// Load credentials
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("failed to load credentials: %v", err)
	}

	// Verify loaded data
	if loaded.Version != creds.Version {
		t.Errorf("version mismatch: got %d, want %d", loaded.Version, creds.Version)
	}

	if loaded.Superuser.Password != creds.Superuser.Password {
		t.Error("superuser password mismatch")
	}

	if loaded.Admin.Password != creds.Admin.Password {
		t.Error("admin password mismatch")
	}

	scrapeCred, ok := loaded.Roles[storage.RoleScrape]
	if !ok {
		t.Error("scrape role not found in loaded credentials")
	} else if scrapeCred.Password != creds.Roles[storage.RoleScrape].Password {
		t.Error("scrape password mismatch")
	}
}

func TestCredentialsManager(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "bib-credentials-manager-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	identityKey := make([]byte, 64)
	for i := range identityKey {
		identityKey[i] = byte(i)
	}

	cfg := Config{
		EncryptionMethod:    EncryptionHybrid,
		RotationInterval:    7 * 24 * time.Hour,
		RotationGracePeriod: 5 * time.Minute,
		EncryptedPath:       filepath.Join(tmpDir, "secrets", "db.enc"),
		PasswordLength:      64,
	}

	manager, err := NewManager(cfg, "test-node-id", identityKey)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer manager.Close()

	// Initialize (should generate new credentials)
	creds, err := manager.Initialize()
	if err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}

	if creds == nil {
		t.Fatal("credentials should not be nil")
	}

	if creds.Version != 1 {
		t.Errorf("expected version 1, got %d", creds.Version)
	}

	// Verify all expected roles exist
	expectedRoles := []storage.DBRole{
		storage.RoleScrape,
		storage.RoleQuery,
		storage.RoleTransform,
		storage.RoleAudit,
		storage.RoleReadOnly,
	}

	for _, role := range expectedRoles {
		if _, ok := creds.Roles[role]; !ok {
			t.Errorf("role %s not found in credentials", role)
		}
	}

	// Current should return the same credentials
	current := manager.Current()
	if current != creds {
		t.Error("Current() should return the initialized credentials")
	}

	// Should not need rotation yet
	if manager.NeedsRotation() {
		t.Error("should not need rotation immediately after initialization")
	}
}

func TestRoleCredentialExpiration(t *testing.T) {
	// Create a credential that is already expired
	expired := RoleCredential{
		Username:  "test",
		Password:  "test",
		CreatedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
		Status:    StatusActive,
	}

	if !expired.IsExpired() {
		t.Error("credential should be expired")
	}

	// Create a valid credential
	valid := RoleCredential{
		Username:  "test",
		Password:  "test",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(1 * time.Hour), // Valid for 1 more hour
		Status:    StatusActive,
	}

	if valid.IsExpired() {
		t.Error("credential should not be expired")
	}
}
