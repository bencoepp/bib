// Package postgres provides a PostgreSQL implementation of the storage interfaces.
package postgres

import (
	"context"
	"encoding/json"

	"bib/internal/domain"
	"bib/internal/storage"

	"github.com/jackc/pgx/v5"
)

// UserPreferencesRepository implements storage.UserPreferencesRepository for PostgreSQL.
type UserPreferencesRepository struct {
	store *Store
}

// Get retrieves preferences for a user.
func (r *UserPreferencesRepository) Get(ctx context.Context, userID domain.UserID) (*storage.UserPreferences, error) {
	query := `
		SELECT user_id, theme, locale, timezone, date_format, 
		       notifications_enabled, email_notifications, custom,
		       created_at, updated_at
		FROM user_preferences
		WHERE user_id = $1
	`

	var prefs storage.UserPreferences
	var customJSON []byte

	err := r.store.pool.QueryRow(ctx, query, string(userID)).Scan(
		&prefs.UserID,
		&prefs.Theme,
		&prefs.Locale,
		&prefs.Timezone,
		&prefs.DateFormat,
		&prefs.NotificationsEnabled,
		&prefs.EmailNotifications,
		&customJSON,
		&prefs.CreatedAt,
		&prefs.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		// Return default preferences if not found
		return &storage.UserPreferences{
			UserID:               userID,
			Theme:                "system",
			Locale:               "en",
			Timezone:             "UTC",
			DateFormat:           "YYYY-MM-DD",
			NotificationsEnabled: true,
			EmailNotifications:   false,
		}, nil
	}
	if err != nil {
		return nil, err
	}

	// Parse custom JSON
	if len(customJSON) > 0 {
		json.Unmarshal(customJSON, &prefs.Custom)
	}

	return &prefs, nil
}

// Upsert creates or updates preferences for a user.
func (r *UserPreferencesRepository) Upsert(ctx context.Context, prefs *storage.UserPreferences) error {
	var customJSON []byte
	if prefs.Custom != nil {
		customJSON, _ = json.Marshal(prefs.Custom)
	}

	query := `
		INSERT INTO user_preferences (
			user_id, theme, locale, timezone, date_format,
			notifications_enabled, email_notifications, custom
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT(user_id) DO UPDATE SET
			theme = EXCLUDED.theme,
			locale = EXCLUDED.locale,
			timezone = EXCLUDED.timezone,
			date_format = EXCLUDED.date_format,
			notifications_enabled = EXCLUDED.notifications_enabled,
			email_notifications = EXCLUDED.email_notifications,
			custom = EXCLUDED.custom,
			updated_at = NOW()
	`

	_, err := r.store.pool.Exec(ctx, query,
		string(prefs.UserID),
		prefs.Theme,
		prefs.Locale,
		prefs.Timezone,
		prefs.DateFormat,
		prefs.NotificationsEnabled,
		prefs.EmailNotifications,
		customJSON,
	)

	return err
}

// Delete removes preferences for a user.
func (r *UserPreferencesRepository) Delete(ctx context.Context, userID domain.UserID) error {
	_, err := r.store.pool.Exec(ctx,
		"DELETE FROM user_preferences WHERE user_id = $1",
		string(userID),
	)
	return err
}
