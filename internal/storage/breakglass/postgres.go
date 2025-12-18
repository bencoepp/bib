package breakglass

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresCallback implements DatabaseCallback for PostgreSQL.
type PostgresCallback struct {
	pool     *pgxpool.Pool
	host     string
	port     int
	database string
	sslMode  string
}

// NewPostgresCallback creates a new PostgreSQL callback.
func NewPostgresCallback(pool *pgxpool.Pool, host string, port int, database, sslMode string) *PostgresCallback {
	return &PostgresCallback{
		pool:     pool,
		host:     host,
		port:     port,
		database: database,
		sslMode:  sslMode,
	}
}

// CreateBreakGlassUser creates a temporary PostgreSQL user for the session.
// The user has restricted permissions based on the access level:
// - readonly: SELECT on all tables except audit_log
// - readwrite: SELECT, INSERT, UPDATE, DELETE on all tables except audit_log
// Note: audit_log is always off-limits regardless of access level.
func (p *PostgresCallback) CreateBreakGlassUser(ctx context.Context, username, password string, accessLevel AccessLevel) error {
	// Validate username to prevent SQL injection
	if !isValidUsername(username) {
		return fmt.Errorf("invalid username: %s", username)
	}

	// Create the user with the generated password
	createUserSQL := fmt.Sprintf(
		"CREATE ROLE %s WITH LOGIN PASSWORD '%s' VALID UNTIL (NOW() + INTERVAL '2 hours')",
		username,
		escapePassword(password),
	)

	if _, err := p.pool.Exec(ctx, createUserSQL); err != nil {
		return fmt.Errorf("failed to create break glass user: %w", err)
	}

	// Grant permissions based on access level
	var grantSQL string
	switch accessLevel {
	case AccessReadOnly:
		// Grant SELECT on all tables except audit_log
		grantSQL = fmt.Sprintf(`
			DO $$
			DECLARE
				t text;
			BEGIN
				FOR t IN 
					SELECT tablename FROM pg_tables 
					WHERE schemaname = 'public' AND tablename != 'audit_log'
				LOOP
					EXECUTE 'GRANT SELECT ON ' || quote_ident(t) || ' TO %s';
				END LOOP;
			END $$;
		`, username)

	case AccessReadWrite:
		// Grant SELECT, INSERT, UPDATE, DELETE on all tables except audit_log
		grantSQL = fmt.Sprintf(`
			DO $$
			DECLARE
				t text;
			BEGIN
				FOR t IN 
					SELECT tablename FROM pg_tables 
					WHERE schemaname = 'public' AND tablename != 'audit_log'
				LOOP
					EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON ' || quote_ident(t) || ' TO %s';
				END LOOP;
			END $$;
		`, username)

	default:
		// Default to read-only
		grantSQL = fmt.Sprintf(`
			DO $$
			DECLARE
				t text;
			BEGIN
				FOR t IN 
					SELECT tablename FROM pg_tables 
					WHERE schemaname = 'public' AND tablename != 'audit_log'
				LOOP
					EXECUTE 'GRANT SELECT ON ' || quote_ident(t) || ' TO %s';
				END LOOP;
			END $$;
		`, username)
	}

	if _, err := p.pool.Exec(ctx, grantSQL); err != nil {
		// Try to clean up the user on failure
		_, _ = p.pool.Exec(ctx, fmt.Sprintf("DROP ROLE IF EXISTS %s", username))
		return fmt.Errorf("failed to grant permissions: %w", err)
	}

	// Explicitly revoke all access to audit_log (defense in depth)
	revokeAuditSQL := fmt.Sprintf("REVOKE ALL ON audit_log FROM %s", username)
	if _, err := p.pool.Exec(ctx, revokeAuditSQL); err != nil {
		// This might fail if the table doesn't exist, which is fine
		// The important thing is we never granted access in the first place
	}

	return nil
}

// DropBreakGlassUser removes the temporary PostgreSQL user.
func (p *PostgresCallback) DropBreakGlassUser(ctx context.Context, username string) error {
	if !isValidUsername(username) {
		return fmt.Errorf("invalid username: %s", username)
	}

	// Terminate any active connections for this user
	terminateSQL := fmt.Sprintf(`
		SELECT pg_terminate_backend(pid) 
		FROM pg_stat_activity 
		WHERE usename = '%s'
	`, username)
	_, _ = p.pool.Exec(ctx, terminateSQL)

	// Drop the user
	dropSQL := fmt.Sprintf("DROP ROLE IF EXISTS %s", username)
	if _, err := p.pool.Exec(ctx, dropSQL); err != nil {
		return fmt.Errorf("failed to drop break glass user: %w", err)
	}

	return nil
}

// GetConnectionString returns the connection string for the break glass user.
func (p *PostgresCallback) GetConnectionString(username, password string) string {
	return fmt.Sprintf(
		"postgresql://%s:%s@%s:%d/%s?sslmode=%s",
		username,
		password,
		p.host,
		p.port,
		p.database,
		p.sslMode,
	)
}

// isValidUsername checks if a username is valid and safe to use in SQL.
// Break glass usernames must start with "breakglass_" and contain only
// alphanumeric characters.
func isValidUsername(username string) bool {
	if !strings.HasPrefix(username, "breakglass_") {
		return false
	}

	suffix := strings.TrimPrefix(username, "breakglass_")
	if len(suffix) == 0 || len(suffix) > 32 {
		return false
	}

	for _, c := range suffix {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return false
		}
	}

	return true
}

// escapePassword escapes a password for use in SQL.
// Note: In production, use parameterized queries where possible.
func escapePassword(password string) string {
	return strings.ReplaceAll(password, "'", "''")
}
