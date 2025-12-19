-- Topic membership table for role-based access to topics
CREATE INDEX idx_banned_peers_expires_at ON banned_peers(expires_at);

CREATE INDEX idx_topic_invitations_status ON topic_invitations(status);
CREATE INDEX idx_topic_invitations_token ON topic_invitations(token);
CREATE INDEX idx_topic_invitations_invitee_email ON topic_invitations(invitee_email);
CREATE INDEX idx_topic_invitations_invitee_user_id ON topic_invitations(invitee_user_id);
CREATE INDEX idx_topic_invitations_topic_id ON topic_invitations(topic_id);

CREATE INDEX idx_topic_members_role ON topic_members(role);
CREATE INDEX idx_topic_members_user_id ON topic_members(user_id);
CREATE INDEX idx_topic_members_topic_id ON topic_members(topic_id);
-- Indexes for efficient queries

);
    FOREIGN KEY (banned_by) REFERENCES users(id) ON DELETE SET NULL
    metadata TEXT, -- JSON
    expires_at TEXT, -- NULL = permanent
    banned_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    banned_by TEXT,
    reason TEXT NOT NULL,
    peer_id TEXT PRIMARY KEY,
CREATE TABLE banned_peers (
-- Banned peers table for persistent peer bans

);
    FOREIGN KEY (invitee_user_id) REFERENCES users(id) ON DELETE CASCADE
    FOREIGN KEY (inviter_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (topic_id) REFERENCES topics(id) ON DELETE CASCADE,
    responded_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    expires_at TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'declined', 'expired', 'cancelled')),
    message TEXT,
    token TEXT UNIQUE NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('owner', 'editor', 'viewer')),
    invitee_user_id TEXT,
    invitee_email TEXT,
    inviter_id TEXT NOT NULL,
    topic_id TEXT NOT NULL,
    id TEXT PRIMARY KEY,
CREATE TABLE topic_invitations (
-- Topic invitations for pending invites

);
    FOREIGN KEY (invited_by) REFERENCES users(id) ON DELETE SET NULL
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (topic_id) REFERENCES topics(id) ON DELETE CASCADE,
    UNIQUE (topic_id, user_id),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    accepted_at TEXT,
    invited_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    invited_by TEXT,
    role TEXT NOT NULL CHECK (role IN ('owner', 'editor', 'viewer')),
    user_id TEXT NOT NULL,
    topic_id TEXT NOT NULL,
    id TEXT PRIMARY KEY,
CREATE TABLE topic_members (

