package storage

import "errors"

// Storage errors
var (
	// ErrNotFound is returned when an entity is not found.
	ErrNotFound = errors.New("not found")

	// ErrAlreadyExists is returned when trying to create a duplicate entity.
	ErrAlreadyExists = errors.New("already exists")

	// ErrInvalidInput is returned for invalid input parameters.
	ErrInvalidInput = errors.New("invalid input")

	// ErrNotAuthoritative is returned when a non-authoritative store tries to perform
	// an operation that requires authoritative status.
	ErrNotAuthoritative = errors.New("storage is not authoritative; operation requires PostgreSQL backend")

	// ErrConnectionFailed is returned when database connection fails.
	ErrConnectionFailed = errors.New("database connection failed")

	// ErrMigrationFailed is returned when migrations fail.
	ErrMigrationFailed = errors.New("migration failed")

	// ErrTransactionFailed is returned when a transaction fails.
	ErrTransactionFailed = errors.New("transaction failed")

	// ErrAuditChainBroken is returned when audit log tamper detection fails.
	ErrAuditChainBroken = errors.New("audit log chain integrity check failed")

	// ErrInvalidRole is returned when an invalid database role is used.
	ErrInvalidRole = errors.New("invalid database role")

	// ErrPermissionDenied is returned when an operation is not allowed for the current role.
	ErrPermissionDenied = errors.New("permission denied for this role")

	// ErrCacheMiss is returned when a cache lookup fails (SQLite cache mode).
	ErrCacheMiss = errors.New("cache miss")

	// ErrCacheExpired is returned when cached data has expired.
	ErrCacheExpired = errors.New("cache entry expired")
)

// IsNotFound checks if the error is a not found error.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsAlreadyExists checks if the error is a duplicate error.
func IsAlreadyExists(err error) bool {
	return errors.Is(err, ErrAlreadyExists)
}

// IsNotAuthoritative checks if the error is due to non-authoritative storage.
func IsNotAuthoritative(err error) bool {
	return errors.Is(err, ErrNotAuthoritative)
}
