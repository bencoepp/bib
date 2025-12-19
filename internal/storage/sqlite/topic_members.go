// Package sqlite provides a SQLite implementation of the storage interfaces.
package sqlite

import (
	"context"
	"database/sql"
	"time"

	"bib/internal/domain"
	"bib/internal/storage"
)

// TopicMemberRepository implements storage.TopicMemberRepository for SQLite.
type TopicMemberRepository struct {
	store *Store
}

// Create creates a new membership.
func (r *TopicMemberRepository) Create(ctx context.Context, member *storage.TopicMember) error {
	query := `
		INSERT INTO topic_members (
			id, topic_id, user_id, role, invited_by,
			invited_at, accepted_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var acceptedAt *string
	if member.AcceptedAt != nil {
		s := member.AcceptedAt.Format(time.RFC3339)
		acceptedAt = &s
	}

	_, err := r.store.db.ExecContext(ctx, query,
		member.ID,
		string(member.TopicID),
		string(member.UserID),
		string(member.Role),
		string(member.InvitedBy),
		member.InvitedAt.Format(time.RFC3339),
		acceptedAt,
		member.CreatedAt.Format(time.RFC3339),
		member.UpdatedAt.Format(time.RFC3339),
	)

	return err
}

// Get retrieves a membership by topic and user.
func (r *TopicMemberRepository) Get(ctx context.Context, topicID domain.TopicID, userID domain.UserID) (*storage.TopicMember, error) {
	query := `
		SELECT id, topic_id, user_id, role, invited_by,
		       invited_at, accepted_at, created_at, updated_at
		FROM topic_members
		WHERE topic_id = ? AND user_id = ?
	`

	return r.scanMember(r.store.db.QueryRowContext(ctx, query, string(topicID), string(userID)))
}

// GetByID retrieves a membership by ID.
func (r *TopicMemberRepository) GetByID(ctx context.Context, id string) (*storage.TopicMember, error) {
	query := `
		SELECT id, topic_id, user_id, role, invited_by,
		       invited_at, accepted_at, created_at, updated_at
		FROM topic_members
		WHERE id = ?
	`

	return r.scanMember(r.store.db.QueryRowContext(ctx, query, id))
}

// ListByTopic lists all members of a topic.
func (r *TopicMemberRepository) ListByTopic(ctx context.Context, topicID domain.TopicID, filter storage.TopicMemberFilter) ([]*storage.TopicMember, error) {
	query := `
		SELECT id, topic_id, user_id, role, invited_by,
		       invited_at, accepted_at, created_at, updated_at
		FROM topic_members
		WHERE topic_id = ?
	`
	args := []interface{}{string(topicID)}

	if filter.Role != "" {
		query += " AND role = ?"
		args = append(args, string(filter.Role))
	}

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

	return r.scanMembers(rows)
}

// ListByUser lists all topic memberships for a user.
func (r *TopicMemberRepository) ListByUser(ctx context.Context, userID domain.UserID) ([]*storage.TopicMember, error) {
	query := `
		SELECT id, topic_id, user_id, role, invited_by,
		       invited_at, accepted_at, created_at, updated_at
		FROM topic_members
		WHERE user_id = ?
	`

	rows, err := r.store.db.QueryContext(ctx, query, string(userID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanMembers(rows)
}

// Update updates a membership.
func (r *TopicMemberRepository) Update(ctx context.Context, member *storage.TopicMember) error {
	query := `
		UPDATE topic_members
		SET role = ?, accepted_at = ?, updated_at = ?
		WHERE id = ?
	`

	var acceptedAt *string
	if member.AcceptedAt != nil {
		s := member.AcceptedAt.Format(time.RFC3339)
		acceptedAt = &s
	}

	_, err := r.store.db.ExecContext(ctx, query,
		string(member.Role),
		acceptedAt,
		time.Now().UTC().Format(time.RFC3339),
		member.ID,
	)

	return err
}

// Delete removes a membership.
func (r *TopicMemberRepository) Delete(ctx context.Context, topicID domain.TopicID, userID domain.UserID) error {
	_, err := r.store.db.ExecContext(ctx,
		"DELETE FROM topic_members WHERE topic_id = ? AND user_id = ?",
		string(topicID), string(userID),
	)
	return err
}

// CountOwners counts the number of owners for a topic.
func (r *TopicMemberRepository) CountOwners(ctx context.Context, topicID domain.TopicID) (int, error) {
	var count int
	err := r.store.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM topic_members WHERE topic_id = ? AND role = 'owner'",
		string(topicID),
	).Scan(&count)
	return count, err
}

// HasAccess checks if a user has access to a topic.
func (r *TopicMemberRepository) HasAccess(ctx context.Context, topicID domain.TopicID, userID domain.UserID) (bool, error) {
	var count int
	err := r.store.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM topic_members WHERE topic_id = ? AND user_id = ?",
		string(topicID), string(userID),
	).Scan(&count)
	return count > 0, err
}

// GetRole gets the role of a user in a topic.
func (r *TopicMemberRepository) GetRole(ctx context.Context, topicID domain.TopicID, userID domain.UserID) (storage.TopicMemberRole, error) {
	var role string
	err := r.store.db.QueryRowContext(ctx,
		"SELECT role FROM topic_members WHERE topic_id = ? AND user_id = ?",
		string(topicID), string(userID),
	).Scan(&role)

	if err == sql.ErrNoRows {
		return "", domain.ErrNotOwner
	}
	if err != nil {
		return "", err
	}

	return storage.TopicMemberRole(role), nil
}

// scanMember scans a single member from a row.
func (r *TopicMemberRepository) scanMember(row *sql.Row) (*storage.TopicMember, error) {
	var member storage.TopicMember
	var invitedAt, createdAt, updatedAt string
	var acceptedAt sql.NullString
	var invitedBy sql.NullString

	err := row.Scan(
		&member.ID,
		&member.TopicID,
		&member.UserID,
		&member.Role,
		&invitedBy,
		&invitedAt,
		&acceptedAt,
		&createdAt,
		&updatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, domain.ErrNotOwner
	}
	if err != nil {
		return nil, err
	}

	member.InvitedAt, _ = time.Parse(time.RFC3339, invitedAt)
	member.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	member.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	if invitedBy.Valid {
		member.InvitedBy = domain.UserID(invitedBy.String)
	}

	if acceptedAt.Valid {
		t, _ := time.Parse(time.RFC3339, acceptedAt.String)
		member.AcceptedAt = &t
	}

	return &member, nil
}

// scanMembers scans multiple members from rows.
func (r *TopicMemberRepository) scanMembers(rows *sql.Rows) ([]*storage.TopicMember, error) {
	var members []*storage.TopicMember

	for rows.Next() {
		var member storage.TopicMember
		var invitedAt, createdAt, updatedAt string
		var acceptedAt sql.NullString
		var invitedBy sql.NullString

		err := rows.Scan(
			&member.ID,
			&member.TopicID,
			&member.UserID,
			&member.Role,
			&invitedBy,
			&invitedAt,
			&acceptedAt,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, err
		}

		member.InvitedAt, _ = time.Parse(time.RFC3339, invitedAt)
		member.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		member.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

		if invitedBy.Valid {
			member.InvitedBy = domain.UserID(invitedBy.String)
		}

		if acceptedAt.Valid {
			t, _ := time.Parse(time.RFC3339, acceptedAt.String)
			member.AcceptedAt = &t
		}

		members = append(members, &member)
	}

	return members, rows.Err()
}
