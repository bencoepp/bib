package postgres

import (
	"strings"
)

// nullString returns a pointer to s if non-empty, otherwise nil.
func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// nullableString is an alias for nullString for compatibility.
func nullableString(s string) *string {
	return nullString(s)
}

// isUniqueViolation checks if an error is a unique constraint violation.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	// PostgreSQL unique violation error code is 23505
	errStr := err.Error()
	return strings.Contains(errStr, "23505") ||
		strings.Contains(errStr, "unique constraint") ||
		strings.Contains(errStr, "duplicate key")
}
