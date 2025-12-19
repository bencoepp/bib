package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"bib/internal/domain"
	"bib/internal/storage"

	"github.com/jackc/pgx/v5"
)

// UserRepository implements storage.UserRepository for PostgreSQL.
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

	_, err = r.store.pool.Exec(ctx, `
		INSERT INTO users (id, public_key, key_type, public_key_fingerprint, name, email, status, role, locale, created_at, updated_at, last_login_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
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
		user.CreatedAt,
		user.UpdatedAt,
		user.LastLoginAt,
		metadataJSON,
	)

	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrUserExists
		}
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// Get retrieves a user by ID.
func (r *UserRepository) Get(ctx context.Context, id domain.UserID) (*domain.User, error) {
	row := r.store.pool.QueryRow(ctx, `
		SELECT id, public_key, key_type, public_key_fingerprint, name, email, status, role, locale, created_at, updated_at, last_login_at, metadata
		FROM users WHERE id = $1 AND status != 'deleted'
	`, string(id))

	return scanUserRow(row)
}

// GetByPublicKey retrieves a user by their public key.
func (r *UserRepository) GetByPublicKey(ctx context.Context, publicKey []byte) (*domain.User, error) {
	row := r.store.pool.QueryRow(ctx, `
		SELECT id, public_key, key_type, public_key_fingerprint, name, email, status, role, locale, created_at, updated_at, last_login_at, metadata
		FROM users WHERE public_key = $1 AND status != 'deleted'
	`, publicKey)

	return scanUserRow(row)
}

// GetByEmail retrieves a user by email.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	row := r.store.pool.QueryRow(ctx, `
		SELECT id, public_key, key_type, public_key_fingerprint, name, email, status, role, locale, created_at, updated_at, last_login_at, metadata
		FROM users WHERE email = $1 AND status != 'deleted'
	`, email)

	return scanUserRow(row)
}

// List retrieves users matching the filter.
func (r *UserRepository) List(ctx context.Context, filter storage.UserFilter) ([]*domain.User, error) {
	query := `
		SELECT id, public_key, key_type, public_key_fingerprint, name, email, status, role, locale, created_at, updated_at, last_login_at, metadata
		FROM users WHERE status != 'deleted'
	`
	args := []any{}
	argIdx := 1

	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, string(filter.Status))
		argIdx++
	}

	if filter.Role != "" {
		query += fmt.Sprintf(" AND role = $%d", argIdx)
		args = append(args, string(filter.Role))
		argIdx++
	}

	if filter.Search != "" {
		query += fmt.Sprintf(" AND (name ILIKE $%d OR email ILIKE $%d)", argIdx, argIdx+1)
		search := "%" + filter.Search + "%"
		args = append(args, search, search)
		argIdx += 2
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
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, filter.Limit)
		argIdx++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, filter.Offset)
	}

	rows, err := r.store.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		user, err := scanUserRows(rows)
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

	user.UpdatedAt = time.Now().UTC()

	result, err := r.store.pool.Exec(ctx, `
		UPDATE users SET
			name = $1,
			email = $2,
			status = $3,
			role = $4,
			locale = $5,
			updated_at = $6,
			last_login_at = $7,
			metadata = $8
		WHERE id = $9
	`,
		user.Name,
		nullString(user.Email),
		string(user.Status),
		string(user.Role),
		nullString(user.Locale),
		user.UpdatedAt,
		user.LastLoginAt,
		metadataJSON,
		string(user.ID),
	)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}

	return nil
}

// Delete deletes a user (soft delete).
func (r *UserRepository) Delete(ctx context.Context, id domain.UserID) error {
	result, err := r.store.pool.Exec(ctx, `
		UPDATE users SET status = 'deleted', updated_at = $1 WHERE id = $2
	`, time.Now().UTC(), string(id))
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	if result.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}

	return nil
}

// Count returns the number of users matching the filter.
func (r *UserRepository) Count(ctx context.Context, filter storage.UserFilter) (int64, error) {
	query := "SELECT COUNT(*) FROM users WHERE status != 'deleted'"
	args := []any{}
	argIdx := 1

	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, string(filter.Status))
		argIdx++
	}

	if filter.Role != "" {
		query += fmt.Sprintf(" AND role = $%d", argIdx)
		args = append(args, string(filter.Role))
		argIdx++
	}

	if filter.Search != "" {
		query += fmt.Sprintf(" AND (name ILIKE $%d OR email ILIKE $%d)", argIdx, argIdx+1)
		search := "%" + filter.Search + "%"
		args = append(args, search, search)
	}

	var count int64
	err := r.store.pool.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}

	return count, nil
}

// Exists checks if a user with the given public key exists.
func (r *UserRepository) Exists(ctx context.Context, publicKey []byte) (bool, error) {
	var count int
	err := r.store.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM users WHERE public_key = $1 AND status != 'deleted'
	`, publicKey).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check user existence: %w", err)
	}
	return count > 0, nil
}

// IsFirstUser returns true if no users exist yet.
func (r *UserRepository) IsFirstUser(ctx context.Context) (bool, error) {
	var count int
	err := r.store.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM users WHERE status != 'deleted'
	`).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check first user: %w", err)
	}
	return count == 0, nil
}

// scanUserRow scans a single user row.
func scanUserRow(row pgx.Row) (*domain.User, error) {
	var (
		user         domain.User
		id           string
		keyType      string
		email        *string
		status       string
		role         string
		locale       *string
		metadataJSON []byte
	)

	err := row.Scan(
		&id,
		&user.PublicKey,
		&keyType,
		&user.PublicKeyFingerprint,
		&user.Name,
		&email,
		&status,
		&role,
		&locale,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.LastLoginAt,
		&metadataJSON,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to scan user: %w", err)
	}

	user.ID = domain.UserID(id)
	user.KeyType = domain.KeyType(keyType)
	user.Status = domain.UserStatus(status)
	user.Role = domain.UserRole(role)

	if email != nil {
		user.Email = *email
	}
	if locale != nil {
		user.Locale = *locale
	}

	if len(metadataJSON) > 0 {
		_ = json.Unmarshal(metadataJSON, &user.Metadata)
	}

	return &user, nil
}

// scanUserRows scans user rows.
func scanUserRows(rows pgx.Rows) (*domain.User, error) {
	var (
		user         domain.User
		id           string
		keyType      string
		email        *string
		status       string
		role         string
		locale       *string
		metadataJSON []byte
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
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.LastLoginAt,
		&metadataJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan user: %w", err)
	}

	user.ID = domain.UserID(id)
	user.KeyType = domain.KeyType(keyType)
	user.Status = domain.UserStatus(status)
	user.Role = domain.UserRole(role)

	if email != nil {
		user.Email = *email
	}
	if locale != nil {
		user.Locale = *locale
	}

	if len(metadataJSON) > 0 {
		_ = json.Unmarshal(metadataJSON, &user.Metadata)
	}

	return &user, nil
}

// nullString returns a pointer to s if non-empty, otherwise nil.
// Note: This is defined in topics.go, reusing it here.
