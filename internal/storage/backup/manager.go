package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	"bib/internal/storage"
)

// Manager handles backup and restore operations.
type Manager struct {
	cfg       BackupConfig
	backend   storage.BackendType
	dataDir   string
	nodeID    string
	connStr   string // For PostgreSQL connections
	dbPath    string // For SQLite databases
	encryptor Encryptor
}

// Encryptor handles backup encryption.
type Encryptor interface {
	Encrypt(src io.Reader, dst io.Writer) error
	Decrypt(src io.Reader, dst io.Writer) error
}

// NewManager creates a new backup manager.
func NewManager(cfg BackupConfig, backend storage.BackendType, dataDir, nodeID string) (*Manager, error) {
	if cfg.LocalPath == "" {
		cfg.LocalPath = filepath.Join(dataDir, "backups")
	}

	// Create backup directory if it doesn't exist
	if err := os.MkdirAll(cfg.LocalPath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	m := &Manager{
		cfg:     cfg,
		backend: backend,
		dataDir: dataDir,
		nodeID:  nodeID,
	}

	// Initialize encryptor if encryption is enabled
	if cfg.Encryption {
		// TODO: Initialize actual encryptor using node identity
		// For now, we'll skip encryption in the implementation
	}

	return m, nil
}

// SetConnectionString sets the PostgreSQL connection string.
func (m *Manager) SetConnectionString(connStr string) {
	m.connStr = connStr
}

// SetDatabasePath sets the SQLite database path.
func (m *Manager) SetDatabasePath(dbPath string) {
	m.dbPath = dbPath
}

// Backup creates a new backup.
func (m *Manager) Backup(ctx context.Context, notes string) (*BackupMetadata, error) {
	backupID := generateBackupID()
	timestamp := time.Now().UTC()

	var metadata *BackupMetadata
	var err error

	switch m.backend {
	case storage.BackendPostgres:
		metadata, err = m.backupPostgres(ctx, backupID, timestamp, notes)
	case storage.BackendSQLite:
		metadata, err = m.backupSQLite(ctx, backupID, timestamp, notes)
	default:
		return nil, fmt.Errorf("unsupported backend: %s", m.backend)
	}

	if err != nil {
		return nil, fmt.Errorf("backup failed: %w", err)
	}

	// Verify backup if enabled
	if m.cfg.VerifyAfterBackup {
		if err := m.verifyBackup(metadata); err != nil {
			return nil, fmt.Errorf("backup verification failed: %w", err)
		}
	}

	// Save metadata
	if err := m.saveMetadata(metadata); err != nil {
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}

	// Clean up old backups
	if err := m.cleanupOldBackups(); err != nil {
		// Non-fatal - log warning
		fmt.Printf("Warning: failed to clean up old backups: %v\n", err)
	}

	return metadata, nil
}

// backupPostgres creates a PostgreSQL backup using pg_dump.
func (m *Manager) backupPostgres(ctx context.Context, backupID string, timestamp time.Time, notes string) (*BackupMetadata, error) {
	if m.connStr == "" {
		return nil, fmt.Errorf("PostgreSQL connection string not set")
	}

	// Build backup filename
	filename := fmt.Sprintf("%s_%s_%s.sql", m.nodeID, backupID, timestamp.Format("20060102_150405"))
	if m.cfg.Compression {
		filename += ".gz"
	}
	backupPath := filepath.Join(m.cfg.LocalPath, filename)

	// Build pg_dump command
	args := []string{
		"--format=custom",
		"--compress=9",
		"--no-password",
		"--file=" + backupPath,
	}

	// Parse connection string to extract connection parameters
	// For simplicity, we'll use the full connection string
	args = append(args, m.connStr)

	cmd := exec.CommandContext(ctx, "pg_dump", args...)

	// Set environment variables for authentication
	cmd.Env = append(os.Environ(),
		"PGPASSWORD="+m.extractPassword(),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("pg_dump failed: %w (output: %s)", err, string(output))
	}

	// Calculate file size and hash
	size, hash, err := m.calculateFileInfo(backupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate file info: %w", err)
	}

	// Get PostgreSQL version
	version, _ := m.getPostgresVersion(ctx)

	metadata := &BackupMetadata{
		ID:            backupID,
		Timestamp:     timestamp,
		Backend:       string(m.backend),
		Format:        FormatPgDump,
		Size:          size,
		Compressed:    m.cfg.Compression,
		Encrypted:     m.cfg.Encryption,
		NodeID:        m.nodeID,
		Version:       version,
		Location:      m.cfg.Location,
		Path:          backupPath,
		IntegrityHash: hash,
		Notes:         notes,
	}

	return metadata, nil
}

// backupSQLite creates a SQLite backup.
func (m *Manager) backupSQLite(ctx context.Context, backupID string, timestamp time.Time, notes string) (*BackupMetadata, error) {
	if m.dbPath == "" {
		return nil, fmt.Errorf("SQLite database path not set")
	}

	// Build backup filename
	filename := fmt.Sprintf("%s_%s_%s.db", m.nodeID, backupID, timestamp.Format("20060102_150405"))
	if m.cfg.Compression {
		filename += ".gz"
	}
	backupPath := filepath.Join(m.cfg.LocalPath, filename)

	// Use SQLite backup API via file copy
	// For a production implementation, we should use the SQLite backup API
	// through the database/sql driver or a C library binding
	src, err := os.Open(m.dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open source database: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(backupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return nil, fmt.Errorf("failed to copy database: %w", err)
	}

	// Calculate file size and hash
	size, hash, err := m.calculateFileInfo(backupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate file info: %w", err)
	}

	metadata := &BackupMetadata{
		ID:            backupID,
		Timestamp:     timestamp,
		Backend:       string(m.backend),
		Format:        FormatSQLite,
		Size:          size,
		Compressed:    false, // TODO: Add compression support
		Encrypted:     m.cfg.Encryption,
		NodeID:        m.nodeID,
		Location:      m.cfg.Location,
		Path:          backupPath,
		IntegrityHash: hash,
		Notes:         notes,
	}

	return metadata, nil
}

// Restore restores a backup.
func (m *Manager) Restore(ctx context.Context, opts RestoreOptions) error {
	// Load backup metadata
	metadata, err := m.loadMetadata(opts.BackupID)
	if err != nil {
		return fmt.Errorf("failed to load backup metadata: %w", err)
	}

	// Verify backup if requested
	if opts.VerifyBefore {
		if err := m.verifyBackup(metadata); err != nil {
			return fmt.Errorf("backup verification failed: %w", err)
		}
	}

	// Check if force is needed
	if !opts.Force {
		// Check if database exists and has data
		if m.hasData() && !opts.Force {
			return fmt.Errorf("database contains data; use --force to overwrite")
		}
	}

	// Perform restore based on backend
	switch storage.BackendType(metadata.Backend) {
	case storage.BackendPostgres:
		return m.restorePostgres(ctx, metadata)
	case storage.BackendSQLite:
		return m.restoreSQLite(ctx, metadata)
	default:
		return fmt.Errorf("unsupported backend: %s", metadata.Backend)
	}
}

// restorePostgres restores a PostgreSQL backup.
func (m *Manager) restorePostgres(ctx context.Context, metadata *BackupMetadata) error {
	if m.connStr == "" {
		return fmt.Errorf("PostgreSQL connection string not set")
	}

	// Build pg_restore command
	args := []string{
		"--clean",
		"--if-exists",
		"--no-password",
		"--dbname=" + m.connStr,
		metadata.Path,
	}

	cmd := exec.CommandContext(ctx, "pg_restore", args...)

	// Set environment variables for authentication
	cmd.Env = append(os.Environ(),
		"PGPASSWORD="+m.extractPassword(),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pg_restore failed: %w (output: %s)", err, string(output))
	}

	return nil
}

// restoreSQLite restores a SQLite backup.
func (m *Manager) restoreSQLite(ctx context.Context, metadata *BackupMetadata) error {
	if m.dbPath == "" {
		return fmt.Errorf("SQLite database path not set")
	}

	// Copy backup to database location
	src, err := os.Open(metadata.Path)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(m.dbPath)
	if err != nil {
		return fmt.Errorf("failed to create database file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy backup: %w", err)
	}

	return nil
}

// List returns a list of available backups.
func (m *Manager) List() ([]*BackupMetadata, error) {
	metadataDir := filepath.Join(m.cfg.LocalPath, ".metadata")

	entries, err := os.ReadDir(metadataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*BackupMetadata{}, nil
		}
		return nil, fmt.Errorf("failed to read metadata directory: %w", err)
	}

	var backups []*BackupMetadata
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		backupID := entry.Name()[:len(entry.Name())-5] // Remove .json extension
		metadata, err := m.loadMetadata(backupID)
		if err != nil {
			// Skip invalid metadata files
			continue
		}

		backups = append(backups, metadata)
	}

	// Sort by timestamp (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp.After(backups[j].Timestamp)
	})

	return backups, nil
}

// Delete deletes a backup.
func (m *Manager) Delete(backupID string) error {
	metadata, err := m.loadMetadata(backupID)
	if err != nil {
		return fmt.Errorf("failed to load backup metadata: %w", err)
	}

	// Delete backup file
	if err := os.Remove(metadata.Path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete backup file: %w", err)
	}

	// Delete metadata file
	metadataPath := filepath.Join(m.cfg.LocalPath, ".metadata", backupID+".json")
	if err := os.Remove(metadataPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete metadata file: %w", err)
	}

	return nil
}

// Helper functions

func generateBackupID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func (m *Manager) calculateFileInfo(path string) (int64, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer func() { _ = f.Close() }()

	stat, err := f.Stat()
	if err != nil {
		return 0, "", err
	}

	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return 0, "", err
	}

	return stat.Size(), hex.EncodeToString(hash.Sum(nil)), nil
}

func (m *Manager) verifyBackup(metadata *BackupMetadata) error {
	// Verify file exists
	if _, err := os.Stat(metadata.Path); err != nil {
		return fmt.Errorf("backup file not found: %w", err)
	}

	// Verify hash
	_, hash, err := m.calculateFileInfo(metadata.Path)
	if err != nil {
		return fmt.Errorf("failed to calculate hash: %w", err)
	}

	if hash != metadata.IntegrityHash {
		return fmt.Errorf("integrity check failed: hash mismatch")
	}

	return nil
}

func (m *Manager) saveMetadata(metadata *BackupMetadata) error {
	metadataDir := filepath.Join(m.cfg.LocalPath, ".metadata")
	if err := os.MkdirAll(metadataDir, 0700); err != nil {
		return err
	}

	metadataPath := filepath.Join(metadataDir, metadata.ID+".json")
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metadataPath, data, 0600)
}

func (m *Manager) loadMetadata(backupID string) (*BackupMetadata, error) {
	metadataPath := filepath.Join(m.cfg.LocalPath, ".metadata", backupID+".json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, err
	}

	var metadata BackupMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}

	return &metadata, nil
}

func (m *Manager) cleanupOldBackups() error {
	backups, err := m.List()
	if err != nil {
		return err
	}

	// Delete by age
	cutoff := time.Now().AddDate(0, 0, -m.cfg.RetentionDays)
	for _, backup := range backups {
		if backup.Timestamp.Before(cutoff) {
			if err := m.Delete(backup.ID); err != nil {
				return err
			}
		}
	}

	// Delete by count
	if len(backups) > m.cfg.MaxBackups {
		for i := m.cfg.MaxBackups; i < len(backups); i++ {
			if err := m.Delete(backups[i].ID); err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *Manager) extractPassword() string {
	// Extract password from connection string
	// Format: "host=X port=Y user=Z password=W dbname=D sslmode=S"
	parts := splitConnectionString(m.connStr)
	for _, part := range parts {
		if len(part) > 9 && part[:9] == "password=" {
			return part[9:]
		}
	}
	return ""
}

func (m *Manager) getPostgresVersion(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "psql", "--version")
	output, err := cmd.Output()
	if err != nil {
		return "unknown", err
	}
	return string(output), nil
}

func (m *Manager) hasData() bool {
	// This is a simplified check
	// In production, we should query the database
	switch m.backend {
	case storage.BackendPostgres:
		// Check if database has tables
		return true // Conservative - assume data exists
	case storage.BackendSQLite:
		// Check if database file exists
		if _, err := os.Stat(m.dbPath); err == nil {
			return true
		}
	}
	return false
}

func splitConnectionString(connStr string) []string {
	_ = connStr // unused parameter - simplified implementation
	// Simple space-based split
	return []string{} // Simplified for now
}
