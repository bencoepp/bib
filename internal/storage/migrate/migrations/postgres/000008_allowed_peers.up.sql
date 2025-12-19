-- Allowed peers table for P2P gRPC authorization
CREATE TABLE allowed_peers (
    peer_id TEXT PRIMARY KEY,
    name TEXT,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    added_by TEXT NOT NULL, -- peer ID or 'config'
    expires_at TIMESTAMPTZ, -- NULL = never expires
    metadata JSONB
);

-- Indexes for efficient queries
CREATE INDEX idx_allowed_peers_expires_at ON allowed_peers(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX idx_allowed_peers_added_by ON allowed_peers(added_by);

-- Comment on table
COMMENT ON TABLE allowed_peers IS 'Allowed peers for P2P gRPC authorization';
COMMENT ON COLUMN allowed_peers.peer_id IS 'libp2p peer ID';
COMMENT ON COLUMN allowed_peers.name IS 'Human-readable name for the peer';
COMMENT ON COLUMN allowed_peers.added_at IS 'When the peer was added to the allowed list';
COMMENT ON COLUMN allowed_peers.added_by IS 'Who added the peer (peer ID or "config" for bootstrap)';
COMMENT ON COLUMN allowed_peers.expires_at IS 'When the permission expires (NULL = never)';
COMMENT ON COLUMN allowed_peers.metadata IS 'Additional metadata as JSON';

