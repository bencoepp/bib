// Package postgres provides a PostgreSQL implementation of the storage interfaces.
package postgres

import (
	"context"
	"strconv"

	"bib/internal/domain"
	"bib/internal/storage"

	"github.com/jackc/pgx/v5"
)

// TopicMemberRepository implements storage.TopicMemberRepository for PostgreSQL.
type TopicMemberRepository struct {
	store *Store
}

// Create creates a new membership.
func (r *TopicMemberRepository) Create(ctx context.Context, member *storage.TopicMember) error {
	query := `
		INSERT INTO topic_members (
			id, topic_id, user_id, role, invited_by,
			invited_at, accepted_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.store.pool.Exec(ctx, query,
		member.ID,
		string(member.TopicID),
		string(member.UserID),
		string(member.Role),
		string(member.InvitedBy),
		member.InvitedAt,
		member.AcceptedAt,
		member.CreatedAt,
		member.UpdatedAt,
	)

	return err
}

// Get retrieves a membership by topic and user.
func (r *TopicMemberRepository) Get(ctx context.Context, topicID domain.TopicID, userID domain.UserID) (*storage.TopicMember, error) {
	query := `
		SELECT id, topic_id, user_id, role, invited_by,
		       invited_at, accepted_at, created_at, updated_at
		FROM topic_members
		WHERE topic_id = $1 AND user_id = $2
	`

	return r.scanMember(r.store.pool.QueryRow(ctx, query, string(topicID), string(userID)))
}

// GetByID retrieves a membership by ID.
func (r *TopicMemberRepository) GetByID(ctx context.Context, id string) (*storage.TopicMember, error) {
	query := `
		SELECT id, topic_id, user_id, role, invited_by,
		       invited_at, accepted_at, created_at, updated_at
		FROM topic_members
		WHERE id = $1
	`

	return r.scanMember(r.store.pool.QueryRow(ctx, query, id))
}

// ListByTopic lists all members of a topic.
func (r *TopicMemberRepository) ListByTopic(ctx context.Context, topicID domain.TopicID, filter storage.TopicMemberFilter) ([]*storage.TopicMember, error) {
	query := `
		SELECT id, topic_id, user_id, role, invited_by,
		       invited_at, accepted_at, created_at, updated_at
		FROM topic_members
		WHERE topic_id = $1
	`
	args := []interface{}{string(topicID)}
	argNum := 2

	if filter.Role != "" {
		query += " AND role = $" + strconv.Itoa(argNum)
		args = append(args, string(filter.Role))
		argNum++
	}

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

	return r.scanMembers(rows)
}

// ListByUser lists all topic memberships for a user.
func (r *TopicMemberRepository) ListByUser(ctx context.Context, userID domain.UserID) ([]*storage.TopicMember, error) {
	query := `
		SELECT id, topic_id, user_id, role, invited_by,
		       invited_at, accepted_at, created_at, updated_at
		FROM topic_members
		WHERE user_id = $1
	`

	rows, err := r.store.pool.Query(ctx, query, string(userID))
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
		SET role = $1, accepted_at = $2, updated_at = NOW()
		WHERE id = $3
	`

	_, err := r.store.pool.Exec(ctx, query,
		string(member.Role),
		member.AcceptedAt,
		member.ID,
	)

	return err
}

// Delete removes a membership.
func (r *TopicMemberRepository) Delete(ctx context.Context, topicID domain.TopicID, userID domain.UserID) error {
	_, err := r.store.pool.Exec(ctx,
		"DELETE FROM topic_members WHERE topic_id = $1 AND user_id = $2",
		string(topicID), string(userID),
	)
	return err
}

// CountOwners counts the number of owners for a topic.
func (r *TopicMemberRepository) CountOwners(ctx context.Context, topicID domain.TopicID) (int, error) {
	var count int
	err := r.store.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM topic_members WHERE topic_id = $1 AND role = 'owner'",
		string(topicID),
	).Scan(&count)
	return count, err
}

// HasAccess checks if a user has access to a topic.
func (r *TopicMemberRepository) HasAccess(ctx context.Context, topicID domain.TopicID, userID domain.UserID) (bool, error) {
	var count int
	err := r.store.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM topic_members WHERE topic_id = $1 AND user_id = $2",
		string(topicID), string(userID),
	).Scan(&count)
	return count > 0, err
}

// GetRole gets the role of a user in a topic.
func (r *TopicMemberRepository) GetRole(ctx context.Context, topicID domain.TopicID, userID domain.UserID) (storage.TopicMemberRole, error) {
	var role string
	err := r.store.pool.QueryRow(ctx,
		"SELECT role FROM topic_members WHERE topic_id = $1 AND user_id = $2",
		string(topicID), string(userID),
	).Scan(&role)

	if err == pgx.ErrNoRows {
		return "", domain.ErrNotOwner
	}
	if err != nil {
		return "", err
	}

	return storage.TopicMemberRole(role), nil
}

// scanMember scans a single member from a row.
func (r *TopicMemberRepository) scanMember(row pgx.Row) (*storage.TopicMember, error) {
	var member storage.TopicMember
	var invitedBy *string

	err := row.Scan(
		&member.ID,
		&member.TopicID,
		&member.UserID,
		&member.Role,
		&invitedBy,
		&member.InvitedAt,
		&member.AcceptedAt,
		&member.CreatedAt,
		&member.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, domain.ErrNotOwner
	}
	if err != nil {
		return nil, err
	}

	if invitedBy != nil {
		member.InvitedBy = domain.UserID(*invitedBy)
	}

	return &member, nil
}

// scanMembers scans multiple members from rows.
func (r *TopicMemberRepository) scanMembers(rows pgx.Rows) ([]*storage.TopicMember, error) {
	var members []*storage.TopicMember

	for rows.Next() {
		var member storage.TopicMember
		var invitedBy *string

		err := rows.Scan(
			&member.ID,
			&member.TopicID,
			&member.UserID,
			&member.Role,
			&invitedBy,
			&member.InvitedAt,
			&member.AcceptedAt,
			&member.CreatedAt,
			&member.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if invitedBy != nil {
			member.InvitedBy = domain.UserID(*invitedBy)
		}

		members = append(members, &member)
	}

	return members, rows.Err()
}
