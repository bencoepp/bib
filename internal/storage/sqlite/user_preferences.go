// Package sqlite provides a SQLite implementation of the storage interfaces.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"bib/internal/domain"
	"bib/internal/storage"
)

// UserPreferencesRepository implements storage.UserPreferencesRepository for SQLite.
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
		WHERE user_id = ?
	`

	var prefs storage.UserPreferences
	var customJSON sql.NullString
	var createdAt, updatedAt string

	err := r.store.db.QueryRowContext(ctx, query, string(userID)).Scan(
		&prefs.UserID,
		&prefs.Theme,
		&prefs.Locale,
		&prefs.Timezone,
		&prefs.DateFormat,
		&prefs.NotificationsEnabled,
		&prefs.EmailNotifications,
		&customJSON,
		&createdAt,
		&updatedAt,
	)

	if err == sql.ErrNoRows {
		// Return default preferences if not found
		return &storage.UserPreferences{
			UserID:               userID,
			Theme:                "system",
			Locale:               "en",
			Timezone:             "UTC",
			DateFormat:           "YYYY-MM-DD",
			NotificationsEnabled: true,
			EmailNotifications:   false,
			CreatedAt:            time.Now().UTC(),
			UpdatedAt:            time.Now().UTC(),
		}, nil
	}
	if err != nil {
		return nil, err
	}

	// Parse timestamps
	prefs.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	prefs.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	// Parse custom JSON
	if customJSON.Valid && customJSON.String != "" {
		json.Unmarshal([]byte(customJSON.String), &prefs.Custom)
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
			notifications_enabled, email_notifications, custom,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			theme = excluded.theme,
			locale = excluded.locale,
			timezone = excluded.timezone,
			date_format = excluded.date_format,
			notifications_enabled = excluded.notifications_enabled,
			email_notifications = excluded.email_notifications,
			custom = excluded.custom,
			updated_at = excluded.updated_at
	`

	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.store.db.ExecContext(ctx, query,
		string(prefs.UserID),
		prefs.Theme,
		prefs.Locale,
		prefs.Timezone,
		prefs.DateFormat,
		prefs.NotificationsEnabled,
		prefs.EmailNotifications,
		string(customJSON),
		now,
		now,
	)

	return err
}

// Delete removes preferences for a user.
func (r *UserPreferencesRepository) Delete(ctx context.Context, userID domain.UserID) error {
	_, err := r.store.db.ExecContext(ctx,
		"DELETE FROM user_preferences WHERE user_id = ?",
		string(userID),
	)
	return err
}
