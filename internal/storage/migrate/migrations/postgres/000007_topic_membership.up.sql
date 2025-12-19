-- Topic membership table for role-based access to topics
    EXECUTE FUNCTION update_users_updated_at();
    FOR EACH ROW
    BEFORE UPDATE ON topic_members
CREATE TRIGGER topic_members_updated_at_trigger
-- Trigger to update updated_at on topic_members

CREATE INDEX idx_banned_peers_expires_at ON banned_peers(expires_at) WHERE expires_at IS NOT NULL;

CREATE INDEX idx_topic_invitations_expires_at ON topic_invitations(expires_at) WHERE status = 'pending';
CREATE INDEX idx_topic_invitations_status ON topic_invitations(status);
CREATE INDEX idx_topic_invitations_token ON topic_invitations(token);
CREATE INDEX idx_topic_invitations_invitee_email ON topic_invitations(invitee_email) WHERE invitee_email IS NOT NULL;
CREATE INDEX idx_topic_invitations_invitee_user_id ON topic_invitations(invitee_user_id) WHERE invitee_user_id IS NOT NULL;
CREATE INDEX idx_topic_invitations_topic_id ON topic_invitations(topic_id);

CREATE INDEX idx_topic_members_role ON topic_members(role);
CREATE INDEX idx_topic_members_user_id ON topic_members(user_id);
CREATE INDEX idx_topic_members_topic_id ON topic_members(topic_id);
-- Indexes for efficient queries

);
    metadata JSONB
    expires_at TIMESTAMPTZ, -- NULL = permanent
    banned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    banned_by TEXT REFERENCES users(id) ON DELETE SET NULL,
    reason TEXT NOT NULL,
    peer_id TEXT PRIMARY KEY,
CREATE TABLE banned_peers (
-- Banned peers table for persistent peer bans

);
    CONSTRAINT invitation_target CHECK (invitee_email IS NOT NULL OR invitee_user_id IS NOT NULL)
    responded_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'declined', 'expired', 'cancelled')),
    message TEXT, -- Optional invite message
    token TEXT UNIQUE NOT NULL, -- Unique token for accepting invite
    role TEXT NOT NULL CHECK (role IN ('owner', 'editor', 'viewer')),
    invitee_user_id TEXT REFERENCES users(id) ON DELETE CASCADE, -- Optional: invite by user ID
    invitee_email TEXT, -- Optional: invite by email
    inviter_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    topic_id UUID NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
CREATE TABLE topic_invitations (
-- Topic invitations for pending invites

);
    UNIQUE (topic_id, user_id)
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accepted_at TIMESTAMPTZ,
    invited_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    invited_by TEXT REFERENCES users(id) ON DELETE SET NULL,
    role TEXT NOT NULL CHECK (role IN ('owner', 'editor', 'viewer')),
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    topic_id UUID NOT NULL REFERENCES topics(id) ON DELETE CASCADE,
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
CREATE TABLE topic_members (

