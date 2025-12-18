package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"bib/internal/storage/migrate"

	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver for database/sql
)

// RunMigrations runs database migrations for the given store.
func RunMigrations(ctx context.Context, store Store, cfg MigrationsConfig) error {
	if cfg.LockTimeoutSeconds == 0 {
		cfg.LockTimeoutSeconds = 15
	}
	if cfg.OnChecksumMismatch == "" {
		cfg.OnChecksumMismatch = "fail"
	}

	migrateCfg := migrate.Config{
		VerifyChecksums:    cfg.VerifyChecksums,
		OnChecksumMismatch: cfg.OnChecksumMismatch,
		LockTimeout:        time.Duration(cfg.LockTimeoutSeconds) * time.Second,
	}

	switch store.Backend() {
	case BackendPostgres:
		return runPostgresMigrations(ctx, store, migrateCfg)
	case BackendSQLite:
		return runSQLiteMigrations(ctx, store, migrateCfg)
	default:
		return fmt.Errorf("unsupported backend: %s", store.Backend())
	}
}

// runPostgresMigrations runs PostgreSQL migrations.
func runPostgresMigrations(ctx context.Context, store Store, cfg migrate.Config) error {
	// PostgreSQL store exposes ConnString() method
	type postgresStore interface {
		ConnString() string
	}

	s, ok := store.(postgresStore)
	if !ok {
		return fmt.Errorf("PostgreSQL store does not expose ConnString() method")
	}

	// Open a stdlib connection for golang-migrate
	db, err := sql.Open("pgx", s.ConnString())
	if err != nil {
		return fmt.Errorf("failed to open stdlib connection: %w", err)
	}
	defer db.Close()

	// Create migration manager
	mgr, err := migrate.NewPostgresManager(db, cfg)
	if err != nil {
		return fmt.Errorf("failed to create migration manager: %w", err)
	}
	defer mgr.Close()

	// Run migrations
	if err := mgr.Up(ctx); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// runSQLiteMigrations runs SQLite migrations.
func runSQLiteMigrations(ctx context.Context, store Store, cfg migrate.Config) error {
	// SQLite store exposes DB() method
	type sqliteStore interface {
		DB() *sql.DB
	}

	s, ok := store.(sqliteStore)
	if !ok {
		return fmt.Errorf("SQLite store does not expose DB() method")
	}

	db := s.DB()

	// Check for dirty state and clean it automatically for SQLite
	// This is safe because SQLite is cache-only (no authoritative data)
	var version uint
	var dirty bool
	err := db.QueryRowContext(ctx, "SELECT version, dirty FROM bib_schema_migrations LIMIT 1").Scan(&version, &dirty)
	if err == nil && dirty {
		// Database is in dirty state - clean it for SQLite
		_, err = db.ExecContext(ctx, "UPDATE bib_schema_migrations SET dirty = 0")
		if err != nil {
			return fmt.Errorf("failed to clean dirty state: %w", err)
		}
		// Log that we cleaned it
		getLogger("migrations").Warn("cleaned dirty migration state for SQLite cache database", "version", version)
	}
	// Ignore error if table doesn't exist (first run)

	mgr, err := migrate.NewSQLiteManager(db, cfg)
	if err != nil {
		return fmt.Errorf("failed to create migration manager: %w", err)
	}
	// Note: We don't defer mgr.Close() here because it would close the database
	// connection that the store is still using. The store owns the connection
	// and will close it when the store is closed.

	if err := mgr.Up(ctx); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// RollbackMigration rolls back the last migration.
// This should only be called by admin commands with explicit confirmation.
func RollbackMigration(ctx context.Context, store Store, cfg MigrationsConfig) error {
	if cfg.LockTimeoutSeconds == 0 {
		cfg.LockTimeoutSeconds = 15
	}

	migrateCfg := migrate.Config{
		VerifyChecksums:    false, // Skip checksum verification on rollback
		OnChecksumMismatch: "ignore",
		LockTimeout:        time.Duration(cfg.LockTimeoutSeconds) * time.Second,
	}

	var mgr *migrate.Manager
	var err error

	switch store.Backend() {
	case BackendPostgres:
		type postgresStore interface {
			ConnString() string
		}
		s, ok := store.(postgresStore)
		if !ok {
			return fmt.Errorf("PostgreSQL store does not expose ConnString() method")
		}
		db, err := sql.Open("pgx", s.ConnString())
		if err != nil {
			return fmt.Errorf("failed to open stdlib connection: %w", err)
		}
		defer db.Close()
		mgr, err = migrate.NewPostgresManager(db, migrateCfg)
	case BackendSQLite:
		type sqliteStore interface {
			DB() *sql.DB
		}
		s, ok := store.(sqliteStore)
		if !ok {
			return fmt.Errorf("SQLite store does not expose DB() method")
		}
		mgr, err = migrate.NewSQLiteManager(s.DB(), migrateCfg)
	default:
		return fmt.Errorf("unsupported backend: %s", store.Backend())
	}

	if err != nil {
		return fmt.Errorf("failed to create migration manager: %w", err)
	}

	// Only close if we created a new connection (PostgreSQL)
	// Don't close for SQLite as we're using the store's connection
	if store.Backend() == BackendPostgres {
		defer mgr.Close()
	}

	if err := mgr.Down(ctx); err != nil {
		return fmt.Errorf("failed to rollback migration: %w", err)
	}

	return nil
}

// GetMigrationStatus returns the current migration status.
func GetMigrationStatus(ctx context.Context, store Store, cfg MigrationsConfig) (version uint, dirty bool, err error) {
	if cfg.LockTimeoutSeconds == 0 {
		cfg.LockTimeoutSeconds = 15
	}

	migrateCfg := migrate.Config{
		VerifyChecksums:    cfg.VerifyChecksums,
		OnChecksumMismatch: cfg.OnChecksumMismatch,
		LockTimeout:        time.Duration(cfg.LockTimeoutSeconds) * time.Second,
	}

	var mgr *migrate.Manager

	switch store.Backend() {
	case BackendPostgres:
		type postgresStore interface {
			ConnString() string
		}
		s, ok := store.(postgresStore)
		if !ok {
			return 0, false, fmt.Errorf("PostgreSQL store does not expose ConnString() method")
		}
		db, dbErr := sql.Open("pgx", s.ConnString())
		if dbErr != nil {
			return 0, false, fmt.Errorf("failed to open stdlib connection: %w", dbErr)
		}
		defer db.Close()
		mgr, err = migrate.NewPostgresManager(db, migrateCfg)
	case BackendSQLite:
		type sqliteStore interface {
			DB() *sql.DB
		}
		s, ok := store.(sqliteStore)
		if !ok {
			return 0, false, fmt.Errorf("SQLite store does not expose DB() method")
		}
		mgr, err = migrate.NewSQLiteManager(s.DB(), migrateCfg)
	default:
		return 0, false, fmt.Errorf("unsupported backend: %s", store.Backend())
	}

	if err != nil {
		return 0, false, fmt.Errorf("failed to create migration manager: %w", err)
	}

	// Only close if we created a new connection (PostgreSQL)
	if store.Backend() == BackendPostgres {
		defer mgr.Close()
	}

	return mgr.Version()
}

// ListMigrations lists all migrations and their status.
func ListMigrations(ctx context.Context, store Store, cfg MigrationsConfig) ([]migrate.MigrationInfo, error) {
	if cfg.LockTimeoutSeconds == 0 {
		cfg.LockTimeoutSeconds = 15
	}

	migrateCfg := migrate.Config{
		VerifyChecksums:    cfg.VerifyChecksums,
		OnChecksumMismatch: cfg.OnChecksumMismatch,
		LockTimeout:        time.Duration(cfg.LockTimeoutSeconds) * time.Second,
	}

	var mgr *migrate.Manager
	var err error

	switch store.Backend() {
	case BackendPostgres:
		type postgresStore interface {
			ConnString() string
		}
		s, ok := store.(postgresStore)
		if !ok {
			return nil, fmt.Errorf("PostgreSQL store does not expose ConnString() method")
		}
		db, dbErr := sql.Open("pgx", s.ConnString())
		if dbErr != nil {
			return nil, fmt.Errorf("failed to open stdlib connection: %w", dbErr)
		}
		defer db.Close()
		mgr, err = migrate.NewPostgresManager(db, migrateCfg)
	case BackendSQLite:
		type sqliteStore interface {
			DB() *sql.DB
		}
		s, ok := store.(sqliteStore)
		if !ok {
			return nil, fmt.Errorf("SQLite store does not expose DB() method")
		}
		mgr, err = migrate.NewSQLiteManager(s.DB(), migrateCfg)
	default:
		return nil, fmt.Errorf("unsupported backend: %s", store.Backend())
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create migration manager: %w", err)
	}

	// Only close if we created a new connection (PostgreSQL)
	if store.Backend() == BackendPostgres {
		defer mgr.Close()
	}

	return mgr.List(ctx)
}
