// Package migrate provides database migration management with checksums and locking.
package migrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/postgres/*.sql
var postgresFS embed.FS

//go:embed migrations/sqlite/*.sql
var sqliteFS embed.FS

// Config holds migration configuration.
type Config struct {
	// VerifyChecksums determines if checksums should be verified on startup.
	// Default: true
	VerifyChecksums bool

	// OnChecksumMismatch determines behavior when checksum verification fails.
	// Options: "fail" (abort startup), "warn" (log warning), "ignore"
	// Default: "fail"
	OnChecksumMismatch string

	// LockTimeout is how long to wait for migration lock.
	// Default: 15 seconds
	LockTimeout time.Duration
}

// DefaultConfig returns default migration configuration.
func DefaultConfig() Config {
	return Config{
		VerifyChecksums:    true,
		OnChecksumMismatch: "fail",
		LockTimeout:        15 * time.Second,
	}
}

// Manager handles database migrations.
type Manager struct {
	cfg       Config
	backend   string
	m         *migrate.Migrate
	checksums map[string]string // version -> checksum
}

// NewPostgresManager creates a migration manager for PostgreSQL.
func NewPostgresManager(db *sql.DB, cfg Config) (*Manager, error) {
	driver, err := postgres.WithInstance(db, &postgres.Config{
		MigrationsTable: "bib_schema_migrations", // Use custom table name to avoid conflicts
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres driver: %w", err)
	}

	return newManager("postgres", driver, postgresFS, "migrations/postgres", cfg)
}

// NewSQLiteManager creates a migration manager for SQLite.
func NewSQLiteManager(db *sql.DB, cfg Config) (*Manager, error) {
	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{
		MigrationsTable: "bib_schema_migrations", // Use custom table name to avoid conflicts
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create sqlite driver: %w", err)
	}

	return newManager("sqlite", driver, sqliteFS, "migrations/sqlite", cfg)
}

// newManager creates a migration manager.
func newManager(backend string, driver database.Driver, fsys embed.FS, path string, cfg Config) (*Manager, error) {
	// Create source from embedded filesystem
	// Note: iofs expects the path relative to the embed root
	sourceDriver, err := iofs.New(fsys, path)
	if err != nil {
		return nil, fmt.Errorf("failed to create source driver: %w", err)
	}

	// Create migrate instance
	m, err := migrate.NewWithInstance("iofs", sourceDriver, "database", driver)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrate instance: %w", err)
	}

	mgr := &Manager{
		cfg:       cfg,
		backend:   backend,
		m:         m,
		checksums: make(map[string]string),
	}

	// Calculate checksums for all migration files
	if err := mgr.calculateChecksums(fsys, path); err != nil {
		return nil, fmt.Errorf("failed to calculate checksums: %w", err)
	}

	return mgr, nil
}

// calculateChecksums computes SHA-256 checksums for all migration files.
func (m *Manager) calculateChecksums(fsys embed.FS, path string) error {
	entries, err := fs.ReadDir(fsys, path)
	if err != nil {
		return fmt.Errorf("failed to read migration directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		// Read file content
		content, err := fs.ReadFile(fsys, path+"/"+entry.Name())
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", entry.Name(), err)
		}

		// Calculate checksum
		hash := sha256.Sum256(content)
		checksum := fmt.Sprintf("%x", hash)

		// Extract version from filename (e.g., "000001_initial_schema.up.sql" -> "1")
		parts := strings.SplitN(entry.Name(), "_", 2)
		if len(parts) < 2 {
			continue
		}
		version := strings.TrimLeft(parts[0], "0")
		if version == "" {
			version = "0"
		}

		m.checksums[version+"/"+entry.Name()] = checksum
	}

	return nil
}

// Up runs all pending migrations.
func (m *Manager) Up(ctx context.Context) error {
	// Acquire lock with timeout
	lockCtx, cancel := context.WithTimeout(ctx, m.cfg.LockTimeout)
	defer cancel()

	if err := m.acquireLock(lockCtx); err != nil {
		return fmt.Errorf("failed to acquire migration lock: %w", err)
	}
	defer m.releaseLock(ctx)

	// Verify checksums if enabled
	if m.cfg.VerifyChecksums {
		if err := m.verifyChecksums(ctx); err != nil {
			switch m.cfg.OnChecksumMismatch {
			case "fail":
				return fmt.Errorf("checksum verification failed: %w", err)
			case "warn":
				// Log warning (TODO: add logger parameter)
				fmt.Printf("WARNING: Checksum verification failed: %v\n", err)
			case "ignore":
				// Silently ignore
			default:
				return fmt.Errorf("checksum verification failed: %w", err)
			}
		}
	}

	// Run migrations
	if err := m.m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration failed: %w", err)
	}

	// Store checksums after successful migration
	if err := m.storeChecksums(ctx); err != nil {
		// Don't fail if checksum storage fails (non-critical)
		fmt.Printf("WARNING: Failed to store checksums: %v\n", err)
	}

	return nil
}

// Down rolls back one migration.
// This should only be called by admin commands with explicit confirmation.
func (m *Manager) Down(ctx context.Context) error {
	// Acquire lock with timeout
	lockCtx, cancel := context.WithTimeout(ctx, m.cfg.LockTimeout)
	defer cancel()

	if err := m.acquireLock(lockCtx); err != nil {
		return fmt.Errorf("failed to acquire migration lock: %w", err)
	}
	defer m.releaseLock(ctx)

	// Run down migration (steps = 1)
	if err := m.m.Steps(-1); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}

	return nil
}

// Version returns the current migration version.
func (m *Manager) Version() (uint, bool, error) {
	return m.m.Version()
}

// acquireLock acquires a migration lock to prevent concurrent migrations.
func (m *Manager) acquireLock(ctx context.Context) error {
	// Use PostgreSQL advisory locks or SQLite application-level lock
	// For now, golang-migrate handles locking internally
	// We add this method for future custom lock implementation
	return nil
}

// releaseLock releases the migration lock.
func (m *Manager) releaseLock(ctx context.Context) error {
	// golang-migrate handles lock release
	return nil
}

// verifyChecksums verifies that stored checksums match current migration files.
func (m *Manager) verifyChecksums(ctx context.Context) error {
	// TODO: Query stored checksums from migration_checksums table
	// Compare with m.checksums
	// Return error if mismatch detected
	return nil
}

// storeChecksums stores checksums for applied migrations.
func (m *Manager) storeChecksums(ctx context.Context) error {
	// TODO: Store checksums in migration_checksums table
	// This allows verification on next startup
	return nil
}

// Close closes the migration manager.
func (m *Manager) Close() error {
	srcErr, dbErr := m.m.Close()
	if srcErr != nil {
		return srcErr
	}
	return dbErr
}

// MigrationInfo contains information about a migration.
type MigrationInfo struct {
	Version     uint
	Description string
	Applied     bool
	AppliedAt   *time.Time
	Checksum    string
}

// List returns information about all migrations.
func (m *Manager) List(ctx context.Context) ([]MigrationInfo, error) {
	var migrations []MigrationInfo

	// Get current version
	currentVersion, dirty, err := m.m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return nil, fmt.Errorf("failed to get version: %w", err)
	}

	// Parse migration files to build list
	for key := range m.checksums {
		parts := strings.SplitN(key, "/", 2)
		if len(parts) != 2 {
			continue
		}

		version := parts[0]
		filename := parts[1]

		// Skip down migrations for list
		if !strings.Contains(filename, ".up.sql") {
			continue
		}

		// Extract description from filename
		desc := strings.TrimSuffix(filename, ".up.sql")
		desc = strings.TrimPrefix(desc, parts[0]+"_")
		desc = strings.ReplaceAll(desc, "_", " ")

		// Parse version as uint
		var v uint
		fmt.Sscanf(version, "%d", &v)

		applied := !dirty && v <= currentVersion

		migrations = append(migrations, MigrationInfo{
			Version:     v,
			Description: desc,
			Applied:     applied,
			Checksum:    m.checksums[key],
		})
	}

	// Sort by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}
