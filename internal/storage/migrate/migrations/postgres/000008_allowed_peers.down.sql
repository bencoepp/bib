-- Drop allowed_peers table and related objects
DROP INDEX IF EXISTS idx_allowed_peers_added_by;
DROP INDEX IF EXISTS idx_allowed_peers_expires_at;
DROP TABLE IF EXISTS allowed_peers;

