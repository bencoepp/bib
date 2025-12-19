// Package sqlite provides a SQLite implementation of the storage interfaces.
package sqlite

import (
	"context"
	"database/sql"
	"time"

	"bib/internal/domain"
	"bib/internal/storage"
)

// TopicInvitationRepository implements storage.TopicInvitationRepository for SQLite.
type TopicInvitationRepository struct {
	store *Store
}

// Create creates a new invitation.
func (r *TopicInvitationRepository) Create(ctx context.Context, inv *storage.TopicInvitation) error {
	query := `
		INSERT INTO topic_invitations (
			id, topic_id, inviter_id, invitee_email, invitee_user_id,
			role, token, message, status, expires_at, created_at, responded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var inviteeEmail, inviteeUserID, message *string
	var respondedAt *string

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
	if inv.RespondedAt != nil {
		s := inv.RespondedAt.Format(time.RFC3339)
		respondedAt = &s
	}

	_, err := r.store.db.ExecContext(ctx, query,
		inv.ID,
		string(inv.TopicID),
		string(inv.InviterID),
		inviteeEmail,
		inviteeUserID,
		string(inv.Role),
		inv.Token,
		message,
		string(inv.Status),
		inv.ExpiresAt.Format(time.RFC3339),
		inv.CreatedAt.Format(time.RFC3339),
		respondedAt,
	)

	return err
}

// Get retrieves an invitation by ID.
func (r *TopicInvitationRepository) Get(ctx context.Context, id string) (*storage.TopicInvitation, error) {
	query := `
		SELECT id, topic_id, inviter_id, invitee_email, invitee_user_id,
		       role, token, message, status, expires_at, created_at, responded_at
		FROM topic_invitations
		WHERE id = ?
	`

	return r.scanInvitation(r.store.db.QueryRowContext(ctx, query, id))
}

// GetByToken retrieves an invitation by token.
func (r *TopicInvitationRepository) GetByToken(ctx context.Context, token string) (*storage.TopicInvitation, error) {
	query := `
		SELECT id, topic_id, inviter_id, invitee_email, invitee_user_id,
		       role, token, message, status, expires_at, created_at, responded_at
		FROM topic_invitations
		WHERE token = ?
	`

	return r.scanInvitation(r.store.db.QueryRowContext(ctx, query, token))
}

// ListByTopic lists invitations for a topic.
func (r *TopicInvitationRepository) ListByTopic(ctx context.Context, topicID domain.TopicID, filter storage.InvitationFilter) ([]*storage.TopicInvitation, error) {
	query := `
		SELECT id, topic_id, inviter_id, invitee_email, invitee_user_id,
		       role, token, message, status, expires_at, created_at, responded_at
		FROM topic_invitations
		WHERE topic_id = ?
	`
	args := []interface{}{string(topicID)}

	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, string(filter.Status))
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := r.store.db.QueryContext(ctx, query, args...)
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
		WHERE invitee_user_id = ?
		ORDER BY created_at DESC
	`

	rows, err := r.store.db.QueryContext(ctx, query, string(userID))
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
		WHERE invitee_email = ?
		ORDER BY created_at DESC
	`

	rows, err := r.store.db.QueryContext(ctx, query, email)
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
		SET status = ?, responded_at = ?
		WHERE id = ?
	`

	var respondedAt *string
	if inv.RespondedAt != nil {
		s := inv.RespondedAt.Format(time.RFC3339)
		respondedAt = &s
	}

	_, err := r.store.db.ExecContext(ctx, query,
		string(inv.Status),
		respondedAt,
		inv.ID,
	)

	return err
}

// Delete removes an invitation.
func (r *TopicInvitationRepository) Delete(ctx context.Context, id string) error {
	_, err := r.store.db.ExecContext(ctx,
		"DELETE FROM topic_invitations WHERE id = ?",
		id,
	)
	return err
}

// ExpirePending expires all pending invitations that have passed their expiration.
func (r *TopicInvitationRepository) ExpirePending(ctx context.Context) (int64, error) {
	result, err := r.store.db.ExecContext(ctx,
		`UPDATE topic_invitations 
		 SET status = 'expired' 
		 WHERE status = 'pending' AND expires_at < ?`,
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// scanInvitation scans a single invitation from a row.
func (r *TopicInvitationRepository) scanInvitation(row *sql.Row) (*storage.TopicInvitation, error) {
	var inv storage.TopicInvitation
	var inviteeEmail, inviteeUserID, message sql.NullString
	var expiresAt, createdAt string
	var respondedAt sql.NullString

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
		&expiresAt,
		&createdAt,
		&respondedAt,
	)

	if err == sql.ErrNoRows {
		return nil, domain.ErrOwnerNotFound
	}
	if err != nil {
		return nil, err
	}

	if inviteeEmail.Valid {
		inv.InviteeEmail = inviteeEmail.String
	}
	if inviteeUserID.Valid {
		inv.InviteeUserID = domain.UserID(inviteeUserID.String)
	}
	if message.Valid {
		inv.Message = message.String
	}

	inv.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
	inv.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)

	if respondedAt.Valid {
		t, _ := time.Parse(time.RFC3339, respondedAt.String)
		inv.RespondedAt = &t
	}

	return &inv, nil
}

// scanInvitations scans multiple invitations from rows.
func (r *TopicInvitationRepository) scanInvitations(rows *sql.Rows) ([]*storage.TopicInvitation, error) {
	var invitations []*storage.TopicInvitation

	for rows.Next() {
		var inv storage.TopicInvitation
		var inviteeEmail, inviteeUserID, message sql.NullString
		var expiresAt, createdAt string
		var respondedAt sql.NullString

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
			&expiresAt,
			&createdAt,
			&respondedAt,
		)
		if err != nil {
			return nil, err
		}

		if inviteeEmail.Valid {
			inv.InviteeEmail = inviteeEmail.String
		}
		if inviteeUserID.Valid {
			inv.InviteeUserID = domain.UserID(inviteeUserID.String)
		}
		if message.Valid {
			inv.Message = message.String
		}

		inv.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
		inv.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)

		if respondedAt.Valid {
			t, _ := time.Parse(time.RFC3339, respondedAt.String)
			inv.RespondedAt = &t
		}

		invitations = append(invitations, &inv)
	}

	return invitations, rows.Err()
}
