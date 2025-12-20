// Package topic implements the TopicService gRPC service.
package topic

import (
	"context"
	"time"

	"bib/internal/domain"
	"bib/internal/grpc/interfaces"
	"bib/internal/storage"

	"github.com/google/uuid"
)

// MembershipManager handles topic membership operations.
type MembershipManager struct {
	store       storage.Store
	auditLogger interfaces.AuditLogger
}

// NewMembershipManager creates a new membership manager.
func NewMembershipManager(store storage.Store, auditLogger interfaces.AuditLogger) *MembershipManager {
	return &MembershipManager{
		store:       store,
		auditLogger: auditLogger,
	}
}

// InviteMemberRequest contains the parameters for inviting a member.
type InviteMemberRequest struct {
	TopicID      domain.TopicID
	InviterID    domain.UserID
	InviteeEmail string
	InviteeID    domain.UserID
	Role         storage.TopicMemberRole
	Message      string
	ExpiresIn    time.Duration
}

// InviteMemberResponse contains the result of inviting a member.
type InviteMemberResponse struct {
	Invitation *storage.TopicInvitation
	Token      string
}

// InviteMember creates an invitation for a user to join a topic.
func (m *MembershipManager) InviteMember(ctx context.Context, req InviteMemberRequest) (*InviteMemberResponse, error) {
	role, err := m.store.TopicMembers().GetRole(ctx, req.TopicID, req.InviterID)
	if err != nil || role != storage.TopicMemberRoleOwner {
		return nil, domain.ErrNotOwner
	}

	if req.InviteeEmail == "" && req.InviteeID == "" {
		return nil, domain.ErrInvalidOperation
	}

	if req.InviteeID != "" {
		existing, _ := m.store.TopicMembers().Get(ctx, req.TopicID, req.InviteeID)
		if existing != nil {
			return nil, domain.ErrUserExists
		}
	}

	token := generateSecureToken()

	expiresAt := time.Now().UTC().Add(req.ExpiresIn)
	if req.ExpiresIn == 0 {
		expiresAt = time.Now().UTC().Add(7 * 24 * time.Hour)
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

	if m.auditLogger != nil {
		_ = m.auditLogger.LogServiceAction(ctx, "CREATE", "topic_invitation", invitation.ID, map[string]interface{}{
			"topic_id": string(req.TopicID),
		})
	}

	return &InviteMemberResponse{
		Invitation: invitation,
		Token:      token,
	}, nil
}

// AcceptInvitation accepts an invitation to join a topic.
func (m *MembershipManager) AcceptInvitation(ctx context.Context, token string, userID domain.UserID) (*storage.TopicMember, error) {
	invitation, err := m.store.TopicInvitations().GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	if invitation.Status != storage.InvitationStatusPending {
		return nil, domain.ErrInvalidOperation
	}

	if time.Now().After(invitation.ExpiresAt) {
		invitation.Status = storage.InvitationStatusExpired
		now := time.Now().UTC()
		invitation.RespondedAt = &now
		_ = m.store.TopicInvitations().Update(ctx, invitation)
		return nil, domain.ErrSessionExpired
	}

	if invitation.InviteeUserID != "" && invitation.InviteeUserID != userID {
		return nil, domain.ErrUnauthorized
	}

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

	invitation.Status = storage.InvitationStatusAccepted
	invitation.RespondedAt = &now
	_ = m.store.TopicInvitations().Update(ctx, invitation)

	if m.auditLogger != nil {
		_ = m.auditLogger.LogServiceAction(ctx, "CREATE", "topic_member", member.ID, map[string]interface{}{
			"topic_id": string(invitation.TopicID),
		})
	}

	return member, nil
}

// DeclineInvitation declines an invitation.
func (m *MembershipManager) DeclineInvitation(ctx context.Context, token string, userID domain.UserID) error {
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

// CancelInvitation cancels an invitation.
func (m *MembershipManager) CancelInvitation(ctx context.Context, invitationID string, cancellerID domain.UserID) error {
	invitation, err := m.store.TopicInvitations().Get(ctx, invitationID)
	if err != nil {
		return err
	}

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
func (m *MembershipManager) UpdateMemberRole(ctx context.Context, topicID domain.TopicID, updaterID domain.UserID, memberID domain.UserID, newRole storage.TopicMemberRole) error {
	updaterRole, err := m.store.TopicMembers().GetRole(ctx, topicID, updaterID)
	if err != nil || updaterRole != storage.TopicMemberRoleOwner {
		return domain.ErrNotOwner
	}

	member, err := m.store.TopicMembers().Get(ctx, topicID, memberID)
	if err != nil {
		return err
	}

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

	if m.auditLogger != nil {
		_ = m.auditLogger.LogServiceAction(ctx, "UPDATE", "topic_member", member.ID, map[string]interface{}{
			"new_role": string(newRole),
		})
	}

	return nil
}

// RemoveMember removes a member from a topic.
func (m *MembershipManager) RemoveMember(ctx context.Context, topicID domain.TopicID, removerID domain.UserID, memberID domain.UserID) error {
	removerRole, err := m.store.TopicMembers().GetRole(ctx, topicID, removerID)
	if err != nil || removerRole != storage.TopicMemberRoleOwner {
		return domain.ErrNotOwner
	}

	member, err := m.store.TopicMembers().Get(ctx, topicID, memberID)
	if err != nil {
		return err
	}

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

	if m.auditLogger != nil {
		_ = m.auditLogger.LogServiceAction(ctx, "DELETE", "topic_member", member.ID, map[string]interface{}{
			"topic_id": string(topicID),
		})
	}

	return nil
}

// ListMembers lists all members of a topic.
func (m *MembershipManager) ListMembers(ctx context.Context, topicID domain.TopicID, requesterID domain.UserID) ([]*storage.TopicMember, error) {
	hasAccess, err := m.store.TopicMembers().HasAccess(ctx, topicID, requesterID)
	if err != nil || !hasAccess {
		return nil, domain.ErrUnauthorized
	}

	return m.store.TopicMembers().ListByTopic(ctx, topicID, storage.TopicMemberFilter{})
}

// ListPendingInvitations lists pending invitations for a topic.
func (m *MembershipManager) ListPendingInvitations(ctx context.Context, topicID domain.TopicID, requesterID domain.UserID) ([]*storage.TopicInvitation, error) {
	role, err := m.store.TopicMembers().GetRole(ctx, topicID, requesterID)
	if err != nil || role != storage.TopicMemberRoleOwner {
		return nil, domain.ErrNotOwner
	}

	return m.store.TopicInvitations().ListByTopic(ctx, topicID, storage.InvitationFilter{
		Status: storage.InvitationStatusPending,
	})
}

// ListMyInvitations lists pending invitations for the current user.
func (m *MembershipManager) ListMyInvitations(ctx context.Context, userID domain.UserID) ([]*storage.TopicInvitation, error) {
	return m.store.TopicInvitations().ListByUser(ctx, userID)
}

// TransferOwnership transfers ownership to another member.
func (m *MembershipManager) TransferOwnership(ctx context.Context, topicID domain.TopicID, fromID domain.UserID, toID domain.UserID) error {
	fromRole, err := m.store.TopicMembers().GetRole(ctx, topicID, fromID)
	if err != nil || fromRole != storage.TopicMemberRoleOwner {
		return domain.ErrNotOwner
	}

	toMember, err := m.store.TopicMembers().Get(ctx, topicID, toID)
	if err != nil {
		return domain.ErrOwnerNotFound
	}

	toMember.Role = storage.TopicMemberRoleOwner
	toMember.UpdatedAt = time.Now().UTC()

	if err := m.store.TopicMembers().Update(ctx, toMember); err != nil {
		return err
	}

	if m.auditLogger != nil {
		_ = m.auditLogger.LogServiceAction(ctx, "UPDATE", "topic_member", toMember.ID, map[string]interface{}{
			"action": "transfer_ownership",
			"from":   string(fromID),
			"to":     string(toID),
		})
	}

	return nil
}

// RestoreTopic restores a deleted topic.
func (m *MembershipManager) RestoreTopic(ctx context.Context, topicID domain.TopicID, ownerID domain.UserID) error {
	role, err := m.store.TopicMembers().GetRole(ctx, topicID, ownerID)
	if err != nil || role != storage.TopicMemberRoleOwner {
		return domain.ErrNotOwner
	}

	topic, err := m.store.Topics().Get(ctx, topicID)
	if err != nil {
		return err
	}

	if topic.Status != domain.TopicStatusDeleted {
		return domain.ErrInvalidOperation
	}

	topic.Status = domain.TopicStatusActive
	topic.UpdatedAt = time.Now().UTC()

	if err := m.store.Topics().Update(ctx, topic); err != nil {
		return err
	}

	members, err := m.store.TopicMembers().ListByTopic(ctx, topicID, storage.TopicMemberFilter{})
	if err != nil {
		return err
	}

	for _, member := range members {
		if member.Role != storage.TopicMemberRoleOwner {
			_ = m.store.TopicMembers().Delete(ctx, topicID, member.UserID)
		}
	}

	if m.auditLogger != nil {
		_ = m.auditLogger.LogServiceAction(ctx, "UPDATE", "topic", string(topicID), map[string]interface{}{
			"action": "restore",
		})
	}

	return nil
}
