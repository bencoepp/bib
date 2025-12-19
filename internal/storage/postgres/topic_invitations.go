// Package postgres provides a PostgreSQL implementation of the storage interfaces.
package postgres

import (
	"context"
	"strconv"
	"time"

	"bib/internal/domain"
	"bib/internal/storage"

	"github.com/jackc/pgx/v5"
)

// TopicInvitationRepository implements storage.TopicInvitationRepository for PostgreSQL.
type TopicInvitationRepository struct {
	store *Store
}

// Create creates a new invitation.
func (r *TopicInvitationRepository) Create(ctx context.Context, inv *storage.TopicInvitation) error {
	query := `
		INSERT INTO topic_invitations (
			id, topic_id, inviter_id, invitee_email, invitee_user_id,
			role, token, message, status, expires_at, created_at, responded_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	var inviteeEmail, inviteeUserID, message *string

	if inv.InviteeEmail != "" {
		inviteeEmail = &inv.InviteeEmail
	}
	if inv.InviteeUserID != "" {
		s := string(inv.InviteeUserID)
		inviteeUserID = &s
	}
	if inv.Message != "" {
		message = &inv.Message
	}

	_, err := r.store.pool.Exec(ctx, query,
		inv.ID,
		string(inv.TopicID),
		string(inv.InviterID),
		inviteeEmail,
		inviteeUserID,
		string(inv.Role),
		inv.Token,
		message,
		string(inv.Status),
		inv.ExpiresAt,
		inv.CreatedAt,
		inv.RespondedAt,
	)

	return err
}

// Get retrieves an invitation by ID.
func (r *TopicInvitationRepository) Get(ctx context.Context, id string) (*storage.TopicInvitation, error) {
	query := `
		SELECT id, topic_id, inviter_id, invitee_email, invitee_user_id,
		       role, token, message, status, expires_at, created_at, responded_at
		FROM topic_invitations
		WHERE id = $1
	`

	return r.scanInvitation(r.store.pool.QueryRow(ctx, query, id))
}

// GetByToken retrieves an invitation by token.
func (r *TopicInvitationRepository) GetByToken(ctx context.Context, token string) (*storage.TopicInvitation, error) {
	query := `
		SELECT id, topic_id, inviter_id, invitee_email, invitee_user_id,
		       role, token, message, status, expires_at, created_at, responded_at
		FROM topic_invitations
		WHERE token = $1
	`

	return r.scanInvitation(r.store.pool.QueryRow(ctx, query, token))
}

// ListByTopic lists invitations for a topic.
func (r *TopicInvitationRepository) ListByTopic(ctx context.Context, topicID domain.TopicID, filter storage.InvitationFilter) ([]*storage.TopicInvitation, error) {
	query := `
		SELECT id, topic_id, inviter_id, invitee_email, invitee_user_id,
		       role, token, message, status, expires_at, created_at, responded_at
		FROM topic_invitations
		WHERE topic_id = $1
	`
	args := []interface{}{string(topicID)}
	argNum := 2

	if filter.Status != "" {
		query += " AND status = $" + strconv.Itoa(argNum)
		args = append(args, string(filter.Status))
		argNum++
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT $" + strconv.Itoa(argNum)
		args = append(args, filter.Limit)
		argNum++
	}
	if filter.Offset > 0 {
		query += " OFFSET $" + strconv.Itoa(argNum)
		args = append(args, filter.Offset)
	}

	rows, err := r.store.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanInvitations(rows)
}

// ListByUser lists invitations for a user (as invitee).
func (r *TopicInvitationRepository) ListByUser(ctx context.Context, userID domain.UserID) ([]*storage.TopicInvitation, error) {
	query := `
		SELECT id, topic_id, inviter_id, invitee_email, invitee_user_id,
		       role, token, message, status, expires_at, created_at, responded_at
		FROM topic_invitations
		WHERE invitee_user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.store.pool.Query(ctx, query, string(userID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanInvitations(rows)
}

// ListByEmail lists invitations for an email address.
func (r *TopicInvitationRepository) ListByEmail(ctx context.Context, email string) ([]*storage.TopicInvitation, error) {
	query := `
		SELECT id, topic_id, inviter_id, invitee_email, invitee_user_id,
		       role, token, message, status, expires_at, created_at, responded_at
		FROM topic_invitations
		WHERE invitee_email = $1
		ORDER BY created_at DESC
	`

	rows, err := r.store.pool.Query(ctx, query, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanInvitations(rows)
}

// Update updates an invitation.
func (r *TopicInvitationRepository) Update(ctx context.Context, inv *storage.TopicInvitation) error {
	query := `
		UPDATE topic_invitations
		SET status = $1, responded_at = $2
		WHERE id = $3
	`

	_, err := r.store.pool.Exec(ctx, query,
		string(inv.Status),
		inv.RespondedAt,
		inv.ID,
	)

	return err
}

// Delete removes an invitation.
func (r *TopicInvitationRepository) Delete(ctx context.Context, id string) error {
	_, err := r.store.pool.Exec(ctx,
		"DELETE FROM topic_invitations WHERE id = $1",
		id,
	)
	return err
}

// ExpirePending expires all pending invitations that have passed their expiration.
func (r *TopicInvitationRepository) ExpirePending(ctx context.Context) (int64, error) {
	result, err := r.store.pool.Exec(ctx,
		`UPDATE topic_invitations 
		 SET status = 'expired' 
		 WHERE status = 'pending' AND expires_at < $1`,
		time.Now().UTC(),
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// scanInvitation scans a single invitation from a row.
func (r *TopicInvitationRepository) scanInvitation(row pgx.Row) (*storage.TopicInvitation, error) {
	var inv storage.TopicInvitation
	var inviteeEmail, inviteeUserID, message *string

	err := row.Scan(
		&inv.ID,
		&inv.TopicID,
		&inv.InviterID,
		&inviteeEmail,
		&inviteeUserID,
		&inv.Role,
		&inv.Token,
		&message,
		&inv.Status,
		&inv.ExpiresAt,
		&inv.CreatedAt,
		&inv.RespondedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, domain.ErrOwnerNotFound
	}
	if err != nil {
		return nil, err
	}

	if inviteeEmail != nil {
		inv.InviteeEmail = *inviteeEmail
	}
	if inviteeUserID != nil {
		inv.InviteeUserID = domain.UserID(*inviteeUserID)
	}
	if message != nil {
		inv.Message = *message
	}

	return &inv, nil
}

// scanInvitations scans multiple invitations from rows.
func (r *TopicInvitationRepository) scanInvitations(rows pgx.Rows) ([]*storage.TopicInvitation, error) {
	var invitations []*storage.TopicInvitation

	for rows.Next() {
		var inv storage.TopicInvitation
		var inviteeEmail, inviteeUserID, message *string

		err := rows.Scan(
			&inv.ID,
			&inv.TopicID,
			&inv.InviterID,
			&inviteeEmail,
			&inviteeUserID,
			&inv.Role,
			&inv.Token,
			&message,
			&inv.Status,
			&inv.ExpiresAt,
			&inv.CreatedAt,
			&inv.RespondedAt,
		)
		if err != nil {
			return nil, err
		}

		if inviteeEmail != nil {
			inv.InviteeEmail = *inviteeEmail
		}
		if inviteeUserID != nil {
			inv.InviteeUserID = domain.UserID(*inviteeUserID)
		}
		if message != nil {
			inv.Message = *message
		}

		invitations = append(invitations, &inv)
	}

	return invitations, rows.Err()
}
