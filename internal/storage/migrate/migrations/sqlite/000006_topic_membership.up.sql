-- Topic membership table for role-based access to topics
CREATE TABLE topic_members (
    id TEXT PRIMARY KEY,
    topic_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('owner', 'editor', 'viewer')),
    invited_by TEXT,
    invited_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    accepted_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (topic_id, user_id),
    FOREIGN KEY (topic_id) REFERENCES topics(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (invited_by) REFERENCES users(id) ON DELETE SET NULL
);

-- Topic invitations for pending invites
CREATE TABLE topic_invitations (
    id TEXT PRIMARY KEY,
    topic_id TEXT NOT NULL,
    inviter_id TEXT NOT NULL,
    invitee_email TEXT,
    invitee_user_id TEXT,
    role TEXT NOT NULL CHECK (role IN ('owner', 'editor', 'viewer')),
    token TEXT UNIQUE NOT NULL,
    message TEXT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'declined', 'expired', 'cancelled')),
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    responded_at TEXT,
    FOREIGN KEY (topic_id) REFERENCES topics(id) ON DELETE CASCADE,
    FOREIGN KEY (inviter_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (invitee_user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Banned peers table for persistent peer bans
CREATE TABLE banned_peers (
    peer_id TEXT PRIMARY KEY,
    reason TEXT NOT NULL,
    banned_by TEXT,
    banned_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    expires_at TEXT, -- NULL = permanent
    metadata TEXT, -- JSON
    FOREIGN KEY (banned_by) REFERENCES users(id) ON DELETE SET NULL
);

-- Indexes for efficient queries
CREATE INDEX idx_topic_members_topic_id ON topic_members(topic_id);
CREATE INDEX idx_topic_members_user_id ON topic_members(user_id);
CREATE INDEX idx_topic_members_role ON topic_members(role);

CREATE INDEX idx_topic_invitations_topic_id ON topic_invitations(topic_id);
CREATE INDEX idx_topic_invitations_invitee_user_id ON topic_invitations(invitee_user_id);
CREATE INDEX idx_topic_invitations_invitee_email ON topic_invitations(invitee_email);
CREATE INDEX idx_topic_invitations_token ON topic_invitations(token);
CREATE INDEX idx_topic_invitations_status ON topic_invitations(status);

CREATE INDEX idx_banned_peers_expires_at ON banned_peers(expires_at);

