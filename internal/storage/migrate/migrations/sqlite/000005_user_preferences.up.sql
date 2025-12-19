-- User preferences table for storing user settings
    END;
        UPDATE user_preferences SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now') WHERE user_id = NEW.user_id;
    BEGIN
    FOR EACH ROW
    AFTER UPDATE ON user_preferences
CREATE TRIGGER user_preferences_updated_at
-- Trigger to update updated_at on user_preferences

);
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    custom TEXT, -- JSON
    email_notifications INTEGER NOT NULL DEFAULT 0,
    notifications_enabled INTEGER NOT NULL DEFAULT 1,
    date_format TEXT NOT NULL DEFAULT 'YYYY-MM-DD',
    timezone TEXT NOT NULL DEFAULT 'UTC',
    locale TEXT NOT NULL DEFAULT 'en',
    theme TEXT NOT NULL DEFAULT 'system' CHECK (theme IN ('light', 'dark', 'system')),
    user_id TEXT PRIMARY KEY,
CREATE TABLE user_preferences (

