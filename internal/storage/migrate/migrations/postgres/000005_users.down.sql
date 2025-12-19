-- Drop trigger first
DROP TRIGGER IF EXISTS users_updated_at_trigger ON users;
DROP FUNCTION IF EXISTS update_users_updated_at();

-- Drop sessions first due to foreign key constraint
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;

