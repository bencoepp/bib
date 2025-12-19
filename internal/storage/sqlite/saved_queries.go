package sqlite

import (
	"context"

	"bib/internal/storage"
)

// SavedQueryRepository implements storage.SavedQueryRepository for SQLite.
type SavedQueryRepository struct {
	store *Store
}

// Create creates a new saved query.
func (r *SavedQueryRepository) Create(ctx context.Context, query *storage.SavedQuery) error {
	// TODO: Implement when Phase 3 CEL query system is ready
	return nil
}

// Get retrieves a saved query by ID.
func (r *SavedQueryRepository) Get(ctx context.Context, id string) (*storage.SavedQuery, error) {
	// TODO: Implement when Phase 3 CEL query system is ready
	return nil, storage.ErrNotFound
}

// Update updates a saved query.
func (r *SavedQueryRepository) Update(ctx context.Context, query *storage.SavedQuery) error {
	// TODO: Implement when Phase 3 CEL query system is ready
	return nil
}

// Delete deletes a saved query.
func (r *SavedQueryRepository) Delete(ctx context.Context, id string) error {
	// TODO: Implement when Phase 3 CEL query system is ready
	return nil
}

// List lists saved queries with optional filtering.
func (r *SavedQueryRepository) List(ctx context.Context, filter storage.SavedQueryFilter) ([]*storage.SavedQuery, error) {
	// TODO: Implement when Phase 3 CEL query system is ready
	return []*storage.SavedQuery{}, nil
}

// Count returns the count of saved queries matching a filter.
func (r *SavedQueryRepository) Count(ctx context.Context, filter storage.SavedQueryFilter) (int64, error) {
	// TODO: Implement when Phase 3 CEL query system is ready
	return 0, nil
}
