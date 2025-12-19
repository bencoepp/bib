package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bib/internal/domain"
	"bib/internal/storage"
)

func TestStore_CreateAndMigrate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-sqlite-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := storage.SQLiteConfig{
		Path:         filepath.Join(tmpDir, "test.db"),
		MaxOpenConns: 5,
	}

	store, err := New(cfg, tmpDir, "test-node-id")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := storage.RunMigrations(ctx, store, storage.DefaultMigrationsConfig()); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Verify store properties
	if store.IsAuthoritative() {
		t.Error("SQLite store should not be authoritative")
	}

	if store.Backend() != storage.BackendSQLite {
		t.Errorf("expected backend SQLite, got %s", store.Backend())
	}

	if err := store.Ping(ctx); err != nil {
		t.Errorf("ping failed: %v", err)
	}
}

func TestTopicRepository_CRUD(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := storage.WithOperationContext(context.Background(),
		storage.NewOperationContext(storage.RoleAdmin, "test"),
	)

	repo := store.Topics()

	// Create
	topic := &domain.Topic{
		ID:          domain.TopicID("topic-1"),
		Name:        "Test Topic",
		Description: "A test topic",
		Status:      domain.TopicStatusActive,
		Owners:      []domain.UserID{"user-1"},
		CreatedBy:   "user-1",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := repo.Create(ctx, topic); err != nil {
		t.Fatalf("failed to create topic: %v", err)
	}

	// Get
	got, err := repo.Get(ctx, topic.ID)
	if err != nil {
		t.Fatalf("failed to get topic: %v", err)
	}

	if got.Name != topic.Name {
		t.Errorf("expected name %s, got %s", topic.Name, got.Name)
	}

	// GetByName
	gotByName, err := repo.GetByName(ctx, topic.Name)
	if err != nil {
		t.Fatalf("failed to get topic by name: %v", err)
	}

	if gotByName.ID != topic.ID {
		t.Errorf("expected ID %s, got %s", topic.ID, gotByName.ID)
	}

	// Update
	topic.Description = "Updated description"
	if err := repo.Update(ctx, topic); err != nil {
		t.Fatalf("failed to update topic: %v", err)
	}

	got, _ = repo.Get(ctx, topic.ID)
	if got.Description != "Updated description" {
		t.Errorf("expected updated description, got %s", got.Description)
	}

	// List
	topics, err := repo.List(ctx, storage.TopicFilter{})
	if err != nil {
		t.Fatalf("failed to list topics: %v", err)
	}

	if len(topics) != 1 {
		t.Errorf("expected 1 topic, got %d", len(topics))
	}

	// Count
	count, err := repo.Count(ctx, storage.TopicFilter{})
	if err != nil {
		t.Fatalf("failed to count topics: %v", err)
	}

	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}

	// Delete (soft delete)
	if err := repo.Delete(ctx, topic.ID); err != nil {
		t.Fatalf("failed to delete topic: %v", err)
	}

	got, _ = repo.Get(ctx, topic.ID)
	if got.Status != domain.TopicStatusDeleted {
		t.Errorf("expected status deleted, got %s", got.Status)
	}
}

func TestDatasetRepository_CRUD(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := storage.WithOperationContext(context.Background(),
		storage.NewOperationContext(storage.RoleAdmin, "test"),
	)

	// First create a topic
	topic := &domain.Topic{
		ID:        domain.TopicID("topic-1"),
		Name:      "Test Topic",
		Status:    domain.TopicStatusActive,
		Owners:    []domain.UserID{"user-1"},
		CreatedBy: "user-1",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := store.Topics().Create(ctx, topic); err != nil {
		t.Fatalf("failed to create topic: %v", err)
	}

	repo := store.Datasets()

	// Create dataset
	dataset := &domain.Dataset{
		ID:          domain.DatasetID("dataset-1"),
		TopicID:     topic.ID,
		Name:        "Test Dataset",
		Description: "A test dataset",
		Status:      domain.DatasetStatusActive,
		Owners:      []domain.UserID{"user-1"},
		CreatedBy:   "user-1",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := repo.Create(ctx, dataset); err != nil {
		t.Fatalf("failed to create dataset: %v", err)
	}

	// Get
	got, err := repo.Get(ctx, dataset.ID)
	if err != nil {
		t.Fatalf("failed to get dataset: %v", err)
	}

	if got.Name != dataset.Name {
		t.Errorf("expected name %s, got %s", dataset.Name, got.Name)
	}

	// List
	datasets, err := repo.List(ctx, storage.DatasetFilter{TopicID: &topic.ID})
	if err != nil {
		t.Fatalf("failed to list datasets: %v", err)
	}

	if len(datasets) != 1 {
		t.Errorf("expected 1 dataset, got %d", len(datasets))
	}

	// Count
	count, err := repo.Count(ctx, storage.DatasetFilter{})
	if err != nil {
		t.Fatalf("failed to count datasets: %v", err)
	}

	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}
}

func TestJobRepository_CRUD(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := storage.WithOperationContext(context.Background(),
		storage.NewOperationContext(storage.RoleAdmin, "test"),
	)

	repo := store.Jobs()

	// Create job
	job := &domain.Job{
		ID:            domain.JobID("job-1"),
		Type:          domain.JobTypeScrape,
		Status:        domain.JobStatusPending,
		ExecutionMode: domain.ExecutionModeGoroutine,
		Priority:      10,
		CreatedBy:     "user-1",
		CreatedAt:     time.Now().UTC(),
		InlineInstructions: []domain.Instruction{
			{ID: "inst-1", Operation: domain.OpHTTPGet, Params: map[string]any{"url": "http://example.com"}},
		},
	}

	if err := repo.Create(ctx, job); err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	// Get
	got, err := repo.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("failed to get job: %v", err)
	}

	if got.Priority != job.Priority {
		t.Errorf("expected priority %d, got %d", job.Priority, got.Priority)
	}

	// UpdateStatus
	if err := repo.UpdateStatus(ctx, job.ID, domain.JobStatusRunning); err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	got, _ = repo.Get(ctx, job.ID)
	if got.Status != domain.JobStatusRunning {
		t.Errorf("expected status running, got %s", got.Status)
	}

	if got.StartedAt == nil {
		t.Error("expected StartedAt to be set")
	}

	// GetPending
	job2 := &domain.Job{
		ID:            domain.JobID("job-2"),
		Type:          domain.JobTypeScrape,
		Status:        domain.JobStatusPending,
		ExecutionMode: domain.ExecutionModeGoroutine,
		Priority:      20,
		CreatedBy:     "user-1",
		CreatedAt:     time.Now().UTC(),
		InlineInstructions: []domain.Instruction{
			{ID: "inst-1", Operation: domain.OpHTTPGet},
		},
	}
	repo.Create(ctx, job2)

	pending, err := repo.GetPending(ctx, 10)
	if err != nil {
		t.Fatalf("failed to get pending: %v", err)
	}

	if len(pending) != 1 {
		t.Errorf("expected 1 pending job, got %d", len(pending))
	}

	if pending[0].ID != job2.ID {
		t.Errorf("expected job-2, got %s", pending[0].ID)
	}
}

func TestNodeRepository_CRUD(t *testing.T) {
	store := setupTestStore(t)
	defer store.Close()

	ctx := storage.WithOperationContext(context.Background(),
		storage.NewOperationContext(storage.RoleAdmin, "test"),
	)

	repo := store.Nodes()

	// Upsert
	node := &storage.NodeInfo{
		PeerID:         "peer-1",
		Addresses:      []string{"/ip4/127.0.0.1/tcp/4001"},
		Mode:           "full",
		StorageType:    "postgres",
		TrustedStorage: true,
		LastSeen:       time.Now().UTC(),
	}

	if err := repo.Upsert(ctx, node); err != nil {
		t.Fatalf("failed to upsert node: %v", err)
	}

	// Get
	got, err := repo.Get(ctx, node.PeerID)
	if err != nil {
		t.Fatalf("failed to get node: %v", err)
	}

	if got.Mode != node.Mode {
		t.Errorf("expected mode %s, got %s", node.Mode, got.Mode)
	}

	// List
	nodes, err := repo.List(ctx, storage.NodeFilter{TrustedOnly: true})
	if err != nil {
		t.Fatalf("failed to list nodes: %v", err)
	}

	if len(nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(nodes))
	}

	// UpdateLastSeen
	if err := repo.UpdateLastSeen(ctx, node.PeerID); err != nil {
		t.Fatalf("failed to update last seen: %v", err)
	}

	// Delete
	if err := repo.Delete(ctx, node.PeerID); err != nil {
		t.Fatalf("failed to delete node: %v", err)
	}

	_, err = repo.Get(ctx, node.PeerID)
	if !storage.IsNotFound(err) {
		t.Errorf("expected not found error, got %v", err)
	}
}

func setupTestStore(t *testing.T) *Store {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "bib-sqlite-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	cfg := storage.SQLiteConfig{
		Path:         filepath.Join(tmpDir, "test.db"),
		MaxOpenConns: 5,
	}

	store, err := New(cfg, tmpDir, "test-node-id")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	if err := storage.RunMigrations(ctx, store, storage.DefaultMigrationsConfig()); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return store
}
