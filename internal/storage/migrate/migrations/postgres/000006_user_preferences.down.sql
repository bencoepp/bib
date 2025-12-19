-- Drop user preferences
DROP TRIGGER IF EXISTS users_create_preferences_trigger ON users;
DROP FUNCTION IF EXISTS create_default_user_preferences();
DROP TRIGGER IF EXISTS user_preferences_updated_at_trigger ON user_preferences;
DROP TABLE IF EXISTS user_preferences;

