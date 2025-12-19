-- Users table for user identity management
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    public_key BLOB NOT NULL UNIQUE,
    key_type TEXT NOT NULL CHECK (key_type IN ('ed25519', 'rsa')),
    public_key_fingerprint TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    email TEXT UNIQUE,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'pending', 'suspended', 'deleted')),
    role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user', 'readonly')),
    locale TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    last_login_at TEXT,
    metadata TEXT -- JSON
);

-- User sessions for security auditing
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('ssh', 'grpc', 'api')),
    client_ip TEXT NOT NULL,
    client_agent TEXT,
    public_key_fingerprint TEXT NOT NULL,
    node_id TEXT NOT NULL,
    started_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    ended_at TEXT,
    last_activity_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    metadata TEXT, -- JSON
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Indexes for efficient queries
CREATE INDEX idx_users_email ON users(email) WHERE email IS NOT NULL;
CREATE INDEX idx_users_status ON users(status);
CREATE INDEX idx_users_role ON users(role);
CREATE INDEX idx_users_public_key_fingerprint ON users(public_key_fingerprint);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_type ON sessions(type);
CREATE INDEX idx_sessions_active ON sessions(user_id, ended_at) WHERE ended_at IS NULL;
CREATE INDEX idx_sessions_started_at ON sessions(started_at);
CREATE INDEX idx_sessions_node_id ON sessions(node_id);

