-- Topic membership table for role-based access to topics

CREATE TABLE topic_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    topic_id UUID NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('owner', 'editor', 'viewer')),
    invited_by TEXT REFERENCES users(id) ON DELETE SET NULL,
    invited_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accepted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (topic_id, user_id)
);

-- Topic invitations for pending invites
CREATE TABLE topic_invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    topic_id UUID NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    inviter_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invitee_email TEXT, -- Optional: invite by email
    invitee_user_id TEXT REFERENCES users(id) ON DELETE CASCADE, -- Optional: invite by user ID
    role TEXT NOT NULL CHECK (role IN ('owner', 'editor', 'viewer')),
    token TEXT UNIQUE NOT NULL, -- Unique token for accepting invite
    message TEXT, -- Optional invite message
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'declined', 'expired', 'cancelled')),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    responded_at TIMESTAMPTZ,
    CONSTRAINT invitation_target CHECK (invitee_email IS NOT NULL OR invitee_user_id IS NOT NULL)
);

-- Banned peers table for persistent peer bans
CREATE TABLE banned_peers (
    peer_id TEXT PRIMARY KEY,
    reason TEXT NOT NULL,
    banned_by TEXT REFERENCES users(id) ON DELETE SET NULL,
    banned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ, -- NULL = permanent
    metadata JSONB
);

-- Indexes for efficient queries
CREATE INDEX idx_topic_members_topic_id ON topic_members(topic_id);
CREATE INDEX idx_topic_members_user_id ON topic_members(user_id);
CREATE INDEX idx_topic_members_role ON topic_members(role);

CREATE INDEX idx_topic_invitations_topic_id ON topic_invitations(topic_id);
CREATE INDEX idx_topic_invitations_invitee_user_id ON topic_invitations(invitee_user_id) WHERE invitee_user_id IS NOT NULL;
CREATE INDEX idx_topic_invitations_invitee_email ON topic_invitations(invitee_email) WHERE invitee_email IS NOT NULL;
CREATE INDEX idx_topic_invitations_token ON topic_invitations(token);
CREATE INDEX idx_topic_invitations_status ON topic_invitations(status);
CREATE INDEX idx_topic_invitations_expires_at ON topic_invitations(expires_at) WHERE status = 'pending';

CREATE INDEX idx_banned_peers_expires_at ON banned_peers(expires_at) WHERE expires_at IS NOT NULL;

-- Trigger to update updated_at on topic_members
CREATE TRIGGER topic_members_updated_at_trigger
    BEFORE UPDATE ON topic_members
    FOR EACH ROW
    EXECUTE FUNCTION update_users_updated_at();
