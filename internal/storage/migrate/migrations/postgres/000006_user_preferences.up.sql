-- User preferences table for storing user settings
    EXECUTE FUNCTION create_default_user_preferences();
    FOR EACH ROW
    AFTER INSERT ON users
CREATE TRIGGER users_create_preferences_trigger

$$ LANGUAGE plpgsql;
END;
    RETURN NEW;
    INSERT INTO user_preferences (user_id) VALUES (NEW.id);
BEGIN
RETURNS TRIGGER AS $$
CREATE OR REPLACE FUNCTION create_default_user_preferences()
-- Create default preferences when a user is created

    EXECUTE FUNCTION update_users_updated_at();
    FOR EACH ROW
    BEFORE UPDATE ON user_preferences
CREATE TRIGGER user_preferences_updated_at_trigger
-- Trigger to update updated_at on user_preferences

);
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    custom JSONB,
    email_notifications BOOLEAN NOT NULL DEFAULT false,
    notifications_enabled BOOLEAN NOT NULL DEFAULT true,
    date_format TEXT NOT NULL DEFAULT 'YYYY-MM-DD',
    timezone TEXT NOT NULL DEFAULT 'UTC',
    locale TEXT NOT NULL DEFAULT 'en',
    theme TEXT NOT NULL DEFAULT 'system' CHECK (theme IN ('light', 'dark', 'system')),
    user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
CREATE TABLE user_preferences (

