-- Drop topic membership and banned peers tables
DROP INDEX IF EXISTS idx_banned_peers_expires_at;
DROP INDEX IF EXISTS idx_topic_invitations_status;
DROP INDEX IF EXISTS idx_topic_invitations_token;
DROP INDEX IF EXISTS idx_topic_invitations_invitee_email;
DROP INDEX IF EXISTS idx_topic_invitations_invitee_user_id;
DROP INDEX IF EXISTS idx_topic_invitations_topic_id;
DROP INDEX IF EXISTS idx_topic_members_role;
DROP INDEX IF EXISTS idx_topic_members_user_id;
DROP INDEX IF EXISTS idx_topic_members_topic_id;
DROP TABLE IF EXISTS banned_peers;
DROP TABLE IF EXISTS topic_invitations;
DROP TABLE IF EXISTS topic_members;

