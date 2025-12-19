package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"bib/internal/domain"
	"bib/internal/storage"
)

// UserRepository implements storage.UserRepository for SQLite.
type UserRepository struct {
	store *Store
}

// Create creates a new user.
func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	if err := user.Validate(); err != nil {
		return err
	}

	metadataJSON, err := json.Marshal(user.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	var lastLoginAt *string
	if user.LastLoginAt != nil {
		s := user.LastLoginAt.UTC().Format(time.RFC3339Nano)
		lastLoginAt = &s
	}

	_, err = r.store.execWithAudit(ctx, "INSERT", "users", `
		INSERT INTO users (id, public_key, key_type, public_key_fingerprint, name, email, status, role, locale, created_at, updated_at, last_login_at, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		string(user.ID),
		user.PublicKey,
		string(user.KeyType),
		user.PublicKeyFingerprint,
		user.Name,
		nullString(user.Email),
		string(user.Status),
		string(user.Role),
		nullString(user.Locale),
		user.CreatedAt.UTC().Format(time.RFC3339Nano),
		user.UpdatedAt.UTC().Format(time.RFC3339Nano),
		lastLoginAt,
		string(metadataJSON),
	)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return domain.ErrUserExists
		}
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// Get retrieves a user by ID.
func (r *UserRepository) Get(ctx context.Context, id domain.UserID) (*domain.User, error) {
	rows, err := r.store.queryWithAudit(ctx, "users", `
		SELECT id, public_key, key_type, public_key_fingerprint, name, email, status, role, locale, created_at, updated_at, last_login_at, metadata
		FROM users WHERE id = ? AND status != 'deleted'
	`, string(id))
	if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, domain.ErrUserNotFound
	}

	return scanUser(rows)
}

// GetByPublicKey retrieves a user by their public key.
func (r *UserRepository) GetByPublicKey(ctx context.Context, publicKey []byte) (*domain.User, error) {
	rows, err := r.store.queryWithAudit(ctx, "users", `
		SELECT id, public_key, key_type, public_key_fingerprint, name, email, status, role, locale, created_at, updated_at, last_login_at, metadata
		FROM users WHERE public_key = ? AND status != 'deleted'
	`, publicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, domain.ErrUserNotFound
	}

	return scanUser(rows)
}

// GetByEmail retrieves a user by email.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	rows, err := r.store.queryWithAudit(ctx, "users", `
		SELECT id, public_key, key_type, public_key_fingerprint, name, email, status, role, locale, created_at, updated_at, last_login_at, metadata
		FROM users WHERE email = ? AND status != 'deleted'
	`, email)
	if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, domain.ErrUserNotFound
	}

	return scanUser(rows)
}

// List retrieves users matching the filter.
func (r *UserRepository) List(ctx context.Context, filter storage.UserFilter) ([]*domain.User, error) {
	query := `
		SELECT id, public_key, key_type, public_key_fingerprint, name, email, status, role, locale, created_at, updated_at, last_login_at, metadata
		FROM users WHERE status != 'deleted'
	`
	args := []any{}

	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, string(filter.Status))
	}

	if filter.Role != "" {
		query += " AND role = ?"
		args = append(args, string(filter.Role))
	}

	if filter.Search != "" {
		query += " AND (name LIKE ? OR email LIKE ?)"
		search := "%" + filter.Search + "%"
		args = append(args, search, search)
	}

	// Order by
	orderBy := "name"
	if filter.OrderBy != "" {
		orderBy = filter.OrderBy
	}
	order := "ASC"
	if filter.OrderDesc {
		order = "DESC"
	}
	query += fmt.Sprintf(" ORDER BY %s %s", orderBy, order)

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := r.store.queryWithAudit(ctx, "users", query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

// Update updates an existing user.
func (r *UserRepository) Update(ctx context.Context, user *domain.User) error {
	if err := user.Validate(); err != nil {
		return err
	}

	metadataJSON, err := json.Marshal(user.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	var lastLoginAt *string
	if user.LastLoginAt != nil {
		s := user.LastLoginAt.UTC().Format(time.RFC3339Nano)
		lastLoginAt = &s
	}

	user.UpdatedAt = time.Now().UTC()

	result, err := r.store.execWithAudit(ctx, "UPDATE", "users", `
		UPDATE users SET
			name = ?,
			email = ?,
			status = ?,
			role = ?,
			locale = ?,
			updated_at = ?,
			last_login_at = ?,
			metadata = ?
		WHERE id = ?
	`,
		user.Name,
		nullString(user.Email),
		string(user.Status),
		string(user.Role),
		nullString(user.Locale),
		user.UpdatedAt.UTC().Format(time.RFC3339Nano),
		lastLoginAt,
		string(metadataJSON),
		string(user.ID),
	)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return domain.ErrUserNotFound
	}

	return nil
}

// Delete deletes a user (soft delete).
func (r *UserRepository) Delete(ctx context.Context, id domain.UserID) error {
	result, err := r.store.execWithAudit(ctx, "UPDATE", "users", `
		UPDATE users SET status = 'deleted', updated_at = ? WHERE id = ?
	`, time.Now().UTC().Format(time.RFC3339Nano), string(id))
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return domain.ErrUserNotFound
	}

	return nil
}

// Count returns the number of users matching the filter.
func (r *UserRepository) Count(ctx context.Context, filter storage.UserFilter) (int64, error) {
	query := "SELECT COUNT(*) FROM users WHERE status != 'deleted'"
	args := []any{}

	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, string(filter.Status))
	}

	if filter.Role != "" {
		query += " AND role = ?"
		args = append(args, string(filter.Role))
	}

	if filter.Search != "" {
		query += " AND (name LIKE ? OR email LIKE ?)"
		search := "%" + filter.Search + "%"
		args = append(args, search, search)
	}

	var count int64
	err := r.store.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}

	return count, nil
}

// Exists checks if a user with the given public key exists.
func (r *UserRepository) Exists(ctx context.Context, publicKey []byte) (bool, error) {
	var count int
	err := r.store.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM users WHERE public_key = ? AND status != 'deleted'
	`, publicKey).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check user existence: %w", err)
	}
	return count > 0, nil
}

// IsFirstUser returns true if no users exist yet.
func (r *UserRepository) IsFirstUser(ctx context.Context) (bool, error) {
	var count int
	err := r.store.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM users WHERE status != 'deleted'
	`).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check first user: %w", err)
	}
	return count == 0, nil
}

// scanUser scans a user row into a User struct.
func scanUser(rows *sql.Rows) (*domain.User, error) {
	var (
		user         domain.User
		id           string
		keyType      string
		email        sql.NullString
		status       string
		role         string
		locale       sql.NullString
		createdAt    string
		updatedAt    string
		lastLoginAt  sql.NullString
		metadataJSON sql.NullString
	)

	err := rows.Scan(
		&id,
		&user.PublicKey,
		&keyType,
		&user.PublicKeyFingerprint,
		&user.Name,
		&email,
		&status,
		&role,
		&locale,
		&createdAt,
		&updatedAt,
		&lastLoginAt,
		&metadataJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan user: %w", err)
	}

	user.ID = domain.UserID(id)
	user.KeyType = domain.KeyType(keyType)
	user.Status = domain.UserStatus(status)
	user.Role = domain.UserRole(role)

	if email.Valid {
		user.Email = email.String
	}
	if locale.Valid {
		user.Locale = locale.String
	}

	if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
		user.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339Nano, updatedAt); err == nil {
		user.UpdatedAt = t
	}
	if lastLoginAt.Valid {
		if t, err := time.Parse(time.RFC3339Nano, lastLoginAt.String); err == nil {
			user.LastLoginAt = &t
		}
	}

	if metadataJSON.Valid && metadataJSON.String != "" {
		if err := json.Unmarshal([]byte(metadataJSON.String), &user.Metadata); err != nil {
			// Ignore metadata parse errors
		}
	}

	return &user, nil
}
