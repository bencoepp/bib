package postgres

import (
	"context"
	"time"

	"bib/internal/domain"
	"bib/internal/storage"
)

// QueryHistoryRepository implements storage.QueryHistoryRepository for PostgreSQL.
type QueryHistoryRepository struct {
	store *Store
}

// Add adds a query to history.
func (r *QueryHistoryRepository) Add(ctx context.Context, query *storage.SavedQuery) error {
	// TODO: Implement when Phase 3 CEL query system is ready
	return nil
}

// List returns recent query history for a user.
func (r *QueryHistoryRepository) List(ctx context.Context, userID domain.UserID, limit int) ([]*storage.SavedQuery, error) {
	// TODO: Implement when Phase 3 CEL query system is ready
	return []*storage.SavedQuery{}, nil
}

// Clear clears all query history for a user.
func (r *QueryHistoryRepository) Clear(ctx context.Context, userID domain.UserID) error {
	// TODO: Implement when Phase 3 CEL query system is ready
	return nil
}

// Cleanup removes old history entries before the given time.
func (r *QueryHistoryRepository) Cleanup(ctx context.Context, before time.Time) (int64, error) {
	// TODO: Implement when Phase 3 CEL query system is ready
	return 0, nil
}
