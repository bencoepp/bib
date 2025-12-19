// Package grpc provides gRPC service implementations for the bib daemon.
package grpc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"bib/internal/domain"
	"bib/internal/storage"

	"github.com/google/uuid"
)

// TopicMembershipManager handles topic membership operations.
// This can be used by TopicService or exposed through a separate gRPC service.
type TopicMembershipManager struct {
	store       storage.Store
	auditLogger *AuditMiddleware
}

// NewTopicMembershipManager creates a new membership manager.
func NewTopicMembershipManager(store storage.Store, auditLogger *AuditMiddleware) *TopicMembershipManager {
	return &TopicMembershipManager{
		store:       store,
		auditLogger: auditLogger,
	}
}

// InviteMemberRequest contains the parameters for inviting a member.
type InviteMemberRequest struct {
	TopicID      domain.TopicID
	InviterID    domain.UserID
	InviteeEmail string        // Optional: invite by email
	InviteeID    domain.UserID // Optional: invite by user ID
	Role         storage.TopicMemberRole
	Message      string        // Optional: invite message
	ExpiresIn    time.Duration // How long the invite is valid
}

// InviteMemberResponse contains the result of inviting a member.
type InviteMemberResponse struct {
	Invitation *storage.TopicInvitation
	Token      string
}

// InviteMember creates an invitation for a user to join a topic.
func (m *TopicMembershipManager) InviteMember(ctx context.Context, req InviteMemberRequest) (*InviteMemberResponse, error) {
	// Validate that inviter is an owner
	role, err := m.store.TopicMembers().GetRole(ctx, req.TopicID, req.InviterID)
	if err != nil || role != storage.TopicMemberRoleOwner {
		return nil, domain.ErrNotOwner
	}

	// Validate target exists
	if req.InviteeEmail == "" && req.InviteeID == "" {
		return nil, domain.ErrInvalidOperation
	}

	// Check if already a member
	if req.InviteeID != "" {
		existing, _ := m.store.TopicMembers().Get(ctx, req.TopicID, req.InviteeID)
		if existing != nil {
			return nil, domain.ErrUserExists
		}
	}

	// Generate token
	token := generateSecureToken()

	// Set expiration
	expiresAt := time.Now().UTC().Add(req.ExpiresIn)
	if req.ExpiresIn == 0 {
		expiresAt = time.Now().UTC().Add(7 * 24 * time.Hour) // Default 7 days
	}

	invitation := &storage.TopicInvitation{
		ID:            uuid.New().String(),
		TopicID:       req.TopicID,
		InviterID:     req.InviterID,
		InviteeEmail:  req.InviteeEmail,
		InviteeUserID: req.InviteeID,
		Role:          req.Role,
		Token:         token,
		Message:       req.Message,
		Status:        storage.InvitationStatusPending,
		ExpiresAt:     expiresAt,
		CreatedAt:     time.Now().UTC(),
	}

	if err := m.store.TopicInvitations().Create(ctx, invitation); err != nil {
		return nil, err
	}

	// Audit log
	if m.auditLogger != nil {
		m.auditLogger.LogMutation(ctx, "CREATE", "topic_invitation", invitation.ID,
			"Invited member to topic: "+string(req.TopicID))
	}

	return &InviteMemberResponse{
		Invitation: invitation,
		Token:      token,
	}, nil
}

// AcceptInvitation accepts an invitation to join a topic.
func (m *TopicMembershipManager) AcceptInvitation(ctx context.Context, token string, userID domain.UserID) (*storage.TopicMember, error) {
	// Get invitation by token
	invitation, err := m.store.TopicInvitations().GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	// Validate invitation
	if invitation.Status != storage.InvitationStatusPending {
		return nil, domain.ErrInvalidOperation
	}

	if time.Now().After(invitation.ExpiresAt) {
		// Mark as expired
		invitation.Status = storage.InvitationStatusExpired
		now := time.Now().UTC()
		invitation.RespondedAt = &now
		m.store.TopicInvitations().Update(ctx, invitation)
		return nil, domain.ErrSessionExpired
	}

	// Verify the user matches the invitation
	if invitation.InviteeUserID != "" && invitation.InviteeUserID != userID {
		return nil, domain.ErrUnauthorized
	}

	// Create membership
	now := time.Now().UTC()
	member := &storage.TopicMember{
		ID:         uuid.New().String(),
		TopicID:    invitation.TopicID,
		UserID:     userID,
		Role:       invitation.Role,
		InvitedBy:  invitation.InviterID,
		InvitedAt:  invitation.CreatedAt,
		AcceptedAt: &now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := m.store.TopicMembers().Create(ctx, member); err != nil {
		return nil, err
	}

	// Update invitation status
	invitation.Status = storage.InvitationStatusAccepted
	invitation.RespondedAt = &now
	m.store.TopicInvitations().Update(ctx, invitation)

	// Audit log
	if m.auditLogger != nil {
		m.auditLogger.LogMutation(ctx, "CREATE", "topic_member", member.ID,
			"Accepted invitation to topic: "+string(invitation.TopicID))
	}

	return member, nil
}

// DeclineInvitation declines an invitation.
func (m *TopicMembershipManager) DeclineInvitation(ctx context.Context, token string, userID domain.UserID) error {
	invitation, err := m.store.TopicInvitations().GetByToken(ctx, token)
	if err != nil {
		return err
	}

	if invitation.Status != storage.InvitationStatusPending {
		return domain.ErrInvalidOperation
	}

	if invitation.InviteeUserID != "" && invitation.InviteeUserID != userID {
		return domain.ErrUnauthorized
	}

	now := time.Now().UTC()
	invitation.Status = storage.InvitationStatusDeclined
	invitation.RespondedAt = &now

	return m.store.TopicInvitations().Update(ctx, invitation)
}

// CancelInvitation cancels an invitation (by the inviter or owner).
func (m *TopicMembershipManager) CancelInvitation(ctx context.Context, invitationID string, cancellerID domain.UserID) error {
	invitation, err := m.store.TopicInvitations().Get(ctx, invitationID)
	if err != nil {
		return err
	}

	// Only inviter or owners can cancel
	if invitation.InviterID != cancellerID {
		role, err := m.store.TopicMembers().GetRole(ctx, invitation.TopicID, cancellerID)
		if err != nil || role != storage.TopicMemberRoleOwner {
			return domain.ErrNotOwner
		}
	}

	if invitation.Status != storage.InvitationStatusPending {
		return domain.ErrInvalidOperation
	}

	now := time.Now().UTC()
	invitation.Status = storage.InvitationStatusCancelled
	invitation.RespondedAt = &now

	return m.store.TopicInvitations().Update(ctx, invitation)
}

// UpdateMemberRole updates a member's role.
func (m *TopicMembershipManager) UpdateMemberRole(ctx context.Context, topicID domain.TopicID, updaterID domain.UserID, memberID domain.UserID, newRole storage.TopicMemberRole) error {
	// Only owners can update roles
	updaterRole, err := m.store.TopicMembers().GetRole(ctx, topicID, updaterID)
	if err != nil || updaterRole != storage.TopicMemberRoleOwner {
		return domain.ErrNotOwner
	}

	member, err := m.store.TopicMembers().Get(ctx, topicID, memberID)
	if err != nil {
		return err
	}

	// Can't demote the last owner
	if member.Role == storage.TopicMemberRoleOwner && newRole != storage.TopicMemberRoleOwner {
		count, err := m.store.TopicMembers().CountOwners(ctx, topicID)
		if err != nil {
			return err
		}
		if count <= 1 {
			return domain.ErrCannotRemoveLastOwner
		}
	}

	member.Role = newRole
	member.UpdatedAt = time.Now().UTC()

	if err := m.store.TopicMembers().Update(ctx, member); err != nil {
		return err
	}

	// Audit log
	if m.auditLogger != nil {
		m.auditLogger.LogMutation(ctx, "UPDATE", "topic_member", member.ID,
			"Updated member role to "+string(newRole))
	}

	return nil
}

// RemoveMember removes a member from a topic.
func (m *TopicMembershipManager) RemoveMember(ctx context.Context, topicID domain.TopicID, removerID domain.UserID, memberID domain.UserID) error {
	// Only owners can remove members
	removerRole, err := m.store.TopicMembers().GetRole(ctx, topicID, removerID)
	if err != nil || removerRole != storage.TopicMemberRoleOwner {
		return domain.ErrNotOwner
	}

	member, err := m.store.TopicMembers().Get(ctx, topicID, memberID)
	if err != nil {
		return err
	}

	// Can't remove the last owner
	if member.Role == storage.TopicMemberRoleOwner {
		count, err := m.store.TopicMembers().CountOwners(ctx, topicID)
		if err != nil {
			return err
		}
		if count <= 1 {
			return domain.ErrCannotRemoveLastOwner
		}
	}

	if err := m.store.TopicMembers().Delete(ctx, topicID, memberID); err != nil {
		return err
	}

	// Audit log
	if m.auditLogger != nil {
		m.auditLogger.LogMutation(ctx, "DELETE", "topic_member", member.ID,
			"Removed member from topic: "+string(topicID))
	}

	return nil
}

// ListMembers lists all members of a topic.
func (m *TopicMembershipManager) ListMembers(ctx context.Context, topicID domain.TopicID, requesterID domain.UserID) ([]*storage.TopicMember, error) {
	// Requester must have access to topic
	hasAccess, err := m.store.TopicMembers().HasAccess(ctx, topicID, requesterID)
	if err != nil || !hasAccess {
		return nil, domain.ErrUnauthorized
	}

	return m.store.TopicMembers().ListByTopic(ctx, topicID, storage.TopicMemberFilter{})
}

// ListPendingInvitations lists pending invitations for a topic.
func (m *TopicMembershipManager) ListPendingInvitations(ctx context.Context, topicID domain.TopicID, requesterID domain.UserID) ([]*storage.TopicInvitation, error) {
	// Only owners can see pending invitations
	role, err := m.store.TopicMembers().GetRole(ctx, topicID, requesterID)
	if err != nil || role != storage.TopicMemberRoleOwner {
		return nil, domain.ErrNotOwner
	}

	return m.store.TopicInvitations().ListByTopic(ctx, topicID, storage.InvitationFilter{
		Status: storage.InvitationStatusPending,
	})
}

// ListMyInvitations lists pending invitations for the current user.
func (m *TopicMembershipManager) ListMyInvitations(ctx context.Context, userID domain.UserID) ([]*storage.TopicInvitation, error) {
	return m.store.TopicInvitations().ListByUser(ctx, userID)
}

// TransferOwnership transfers ownership to another member.
// The transferor must be an owner and cannot transfer if they're the last owner
// unless the target is already a member.
func (m *TopicMembershipManager) TransferOwnership(ctx context.Context, topicID domain.TopicID, fromID domain.UserID, toID domain.UserID) error {
	// Validate fromID is an owner
	fromRole, err := m.store.TopicMembers().GetRole(ctx, topicID, fromID)
	if err != nil || fromRole != storage.TopicMemberRoleOwner {
		return domain.ErrNotOwner
	}

	// Validate toID is a member
	toMember, err := m.store.TopicMembers().Get(ctx, topicID, toID)
	if err != nil {
		return domain.ErrOwnerNotFound
	}

	// Promote target to owner
	toMember.Role = storage.TopicMemberRoleOwner
	toMember.UpdatedAt = time.Now().UTC()

	if err := m.store.TopicMembers().Update(ctx, toMember); err != nil {
		return err
	}

	// Audit log
	if m.auditLogger != nil {
		m.auditLogger.LogMutation(ctx, "UPDATE", "topic_member", toMember.ID,
			"Transferred ownership from "+string(fromID)+" to "+string(toID))
	}

	return nil
}

// RestoreTopic restores a deleted topic (owner only).
// Subscriptions are cancelled and members need to re-subscribe.
func (m *TopicMembershipManager) RestoreTopic(ctx context.Context, topicID domain.TopicID, ownerID domain.UserID) error {
	// Validate ownerID is an owner
	role, err := m.store.TopicMembers().GetRole(ctx, topicID, ownerID)
	if err != nil || role != storage.TopicMemberRoleOwner {
		return domain.ErrNotOwner
	}

	// Get topic
	topic, err := m.store.Topics().Get(ctx, topicID)
	if err != nil {
		return err
	}

	// Check if deleted
	if topic.Status != domain.TopicStatusDeleted {
		return domain.ErrInvalidOperation
	}

	// Restore topic
	topic.Status = domain.TopicStatusActive
	topic.UpdatedAt = time.Now().UTC()

	if err := m.store.Topics().Update(ctx, topic); err != nil {
		return err
	}

	// Remove all non-owner members (subscriptions cancelled)
	members, err := m.store.TopicMembers().ListByTopic(ctx, topicID, storage.TopicMemberFilter{})
	if err != nil {
		return err
	}

	for _, member := range members {
		if member.Role != storage.TopicMemberRoleOwner {
			m.store.TopicMembers().Delete(ctx, topicID, member.UserID)
		}
	}

	// Audit log
	if m.auditLogger != nil {
		m.auditLogger.LogMutation(ctx, "UPDATE", "topic", string(topicID),
			"Restored topic, cancelled "+string(rune(len(members)-1))+" subscriptions")
	}

	return nil
}

// generateSecureToken generates a cryptographically secure token.
func generateSecureToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Fallback to UUID if random fails
		return uuid.New().String()
	}
	return hex.EncodeToString(b)
}
