// Package backup provides database backup and recovery functionality.
package backup

import (
	"time"
)

// BackupMetadata holds information about a backup.
type BackupMetadata struct {
	// ID is the unique backup identifier
	ID string `json:"id"`

	// Timestamp is when the backup was created
	Timestamp time.Time `json:"timestamp"`

	// Backend is the storage backend (postgres or sqlite)
	Backend string `json:"backend"`

	// Format is the backup format (pg_dump, wal, snapshot, sqlite)
	Format BackupFormat `json:"format"`

	// Size is the backup size in bytes
	Size int64 `json:"size"`

	// Compressed indicates if the backup is compressed
	Compressed bool `json:"compressed"`

	// Encrypted indicates if the backup is encrypted
	Encrypted bool `json:"encrypted"`

	// NodeID is the node that created the backup
	NodeID string `json:"node_id"`

	// Version is the database version
	Version string `json:"version"`

	// WALPosition is the WAL position (for PostgreSQL PITR)
	WALPosition string `json:"wal_position,omitempty"`

	// Location is where the backup is stored
	Location BackupLocation `json:"location"`

	// Path is the file path or S3 key
	Path string `json:"path"`

	// IntegrityHash is the SHA-256 hash of the backup
	IntegrityHash string `json:"integrity_hash"`

	// Notes are optional user notes
	Notes string `json:"notes,omitempty"`
}

// BackupFormat defines the backup format.
type BackupFormat string

const (
	// FormatPgDump uses pg_dump for PostgreSQL
	FormatPgDump BackupFormat = "pg_dump"

	// FormatWAL uses WAL archiving for PostgreSQL
	FormatWAL BackupFormat = "wal"

	// FormatSnapshot uses snapshot for PostgreSQL
	FormatSnapshot BackupFormat = "snapshot"

	// FormatSQLite uses SQLite backup API
	FormatSQLite BackupFormat = "sqlite"
)

// BackupLocation defines where backups are stored.
type BackupLocation string

const (
	// LocationLocal stores backups on local filesystem
	LocationLocal BackupLocation = "local"

	// LocationS3 stores backups in S3-compatible storage
	LocationS3 BackupLocation = "s3"
)

// BackupConfig holds backup configuration.
type BackupConfig struct {
	// Enabled enables automatic backups
	Enabled bool `mapstructure:"enabled"`

	// Schedule is the backup schedule (cron format)
	Schedule string `mapstructure:"schedule"`

	// Location is where to store backups
	Location BackupLocation `mapstructure:"location"`

	// LocalPath is the local backup directory
	LocalPath string `mapstructure:"local_path"`

	// S3 configuration (for LocationS3)
	S3 S3Config `mapstructure:"s3"`

	// Compression enables backup compression
	Compression bool `mapstructure:"compression"`

	// Encryption enables backup encryption
	Encryption bool `mapstructure:"encryption"`

	// RetentionDays is how long to keep backups
	RetentionDays int `mapstructure:"retention_days"`

	// MaxBackups is the maximum number of backups to keep
	MaxBackups int `mapstructure:"max_backups"`

	// WALArchiving enables WAL archiving for PITR (PostgreSQL only)
	WALArchiving bool `mapstructure:"wal_archiving"`

	// VerifyAfterBackup verifies backup integrity after creation
	VerifyAfterBackup bool `mapstructure:"verify_after_backup"`
}

// S3Config holds S3-compatible storage configuration.
type S3Config struct {
	// Endpoint is the S3 endpoint URL
	Endpoint string `mapstructure:"endpoint"`

	// Bucket is the S3 bucket name
	Bucket string `mapstructure:"bucket"`

	// Region is the S3 region
	Region string `mapstructure:"region"`

	// AccessKeyID is the S3 access key
	AccessKeyID string `mapstructure:"access_key_id"`

	// SecretAccessKey is the S3 secret key
	SecretAccessKey string `mapstructure:"secret_access_key"`

	// Prefix is the key prefix for backups
	Prefix string `mapstructure:"prefix"`

	// UseSSL enables SSL for S3 connections
	UseSSL bool `mapstructure:"use_ssl"`
}

// DefaultBackupConfig returns the default backup configuration.
func DefaultBackupConfig() BackupConfig {
	return BackupConfig{
		Enabled:           false,
		Schedule:          "0 2 * * *", // Daily at 2 AM
		Location:          LocationLocal,
		Compression:       true,
		Encryption:        true,
		RetentionDays:     30,
		MaxBackups:        7,
		WALArchiving:      false,
		VerifyAfterBackup: true,
	}
}

// RestoreOptions holds options for restore operations.
type RestoreOptions struct {
	// BackupID is the backup to restore
	BackupID string

	// TargetTime is the target time for PITR (PostgreSQL only)
	TargetTime *time.Time

	// Force forces restore even if data exists
	Force bool

	// VerifyBefore verifies backup integrity before restore
	VerifyBefore bool
}
