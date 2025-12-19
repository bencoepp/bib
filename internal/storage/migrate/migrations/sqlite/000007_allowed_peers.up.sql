-- Allowed peers table for P2P gRPC authorization
CREATE TABLE allowed_peers (
    peer_id TEXT PRIMARY KEY,
    name TEXT,
    added_at TEXT NOT NULL DEFAULT (datetime('now')),
    added_by TEXT NOT NULL, -- peer ID or 'config'
    expires_at TEXT, -- NULL = never expires
    metadata TEXT -- JSON stored as text
);

-- Indexes for efficient queries
CREATE INDEX idx_allowed_peers_expires_at ON allowed_peers(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX idx_allowed_peers_added_by ON allowed_peers(added_by);

