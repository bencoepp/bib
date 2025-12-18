//go:build integration

// Package storage_test contains SQLite integration tests.
package storage_test

import (
	"context"
	"testing"
	"time"

	"bib/internal/domain"
	"bib/internal/storage"
	"bib/internal/storage/sqlite"
	"bib/test/testutil"
	"bib/test/testutil/fixtures"
)

// TestSQLiteStore_Integration tests the SQLite store implementation.
func TestSQLiteStore_Integration(t *testing.T) {
	ctx := testutil.TestContext(t)
	dataDir := testutil.TempDir(t, "sqlite")

	cfg := storage.SQLiteConfig{
		Path: dataDir + "/bib.db",
	}

	store, err := sqlite.New(cfg, dataDir, "test-node")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Run migrations
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Verify connection
	if err := store.Ping(ctx); err != nil {
		t.Fatalf("failed to ping: %v", err)
	}

	// Verify backend type
	if store.Backend() != storage.BackendSQLite {
		t.Errorf("expected backend %s, got %s", storage.BackendSQLite, store.Backend())
	}

	// SQLite stores are not authoritative
	if store.IsAuthoritative() {
		t.Error("expected SQLite store to not be authoritative")
	}
}

// TestSQLiteTopicRepository_Integration tests topic CRUD operations with SQLite.
func TestSQLiteTopicRepository_Integration(t *testing.T) {
	ctx := testutil.TestContext(t)
	dataDir := testutil.TempDir(t, "sqlite")

	store := createSQLiteStore(t, ctx, dataDir)
	defer store.Close()

	topics := store.Topics()

	t.Run("Create", func(t *testing.T) {
		topic := fixtures.TestTopic("topic-1")
		if err := topics.Create(ctx, topic); err != nil {
			t.Fatalf("failed to create topic: %v", err)
		}
	})

	t.Run("Get", func(t *testing.T) {
		topic, err := topics.Get(ctx, "topic-1")
		if err != nil {
			t.Fatalf("failed to get topic: %v", err)
		}
		if topic.Name != "Test Topic topic-1" {
			t.Errorf("unexpected topic name: %s", topic.Name)
		}
	})

	t.Run("GetByName", func(t *testing.T) {
		topic, err := topics.GetByName(ctx, "Test Topic topic-1")
		if err != nil {
			t.Fatalf("failed to get topic by name: %v", err)
		}
		if string(topic.ID) != "topic-1" {
			t.Errorf("unexpected topic ID: %s", topic.ID)
		}
	})

	t.Run("Update", func(t *testing.T) {
		topic, _ := topics.Get(ctx, "topic-1")
		topic.Description = "Updated description"
		topic.UpdatedAt = time.Now()

		if err := topics.Update(ctx, topic); err != nil {
			t.Fatalf("failed to update topic: %v", err)
		}

		updated, _ := topics.Get(ctx, "topic-1")
		if updated.Description != "Updated description" {
			t.Errorf("unexpected description: %s", updated.Description)
		}
	})

	t.Run("List", func(t *testing.T) {
		list, err := topics.List(ctx, storage.TopicFilter{Limit: 10})
		if err != nil {
			t.Fatalf("failed to list topics: %v", err)
		}
		if len(list) < 1 {
			t.Error("expected at least one topic")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		if err := topics.Delete(ctx, "topic-1"); err != nil {
			t.Fatalf("failed to delete topic: %v", err)
		}
	})
}

// TestSQLiteDatasetRepository_Integration tests dataset CRUD operations.
func TestSQLiteDatasetRepository_Integration(t *testing.T) {
	ctx := testutil.TestContext(t)
	dataDir := testutil.TempDir(t, "sqlite")

	store := createSQLiteStore(t, ctx, dataDir)
	defer store.Close()

	topics := store.Topics()
	datasets := store.Datasets()

	// Create parent topic first
	topic := fixtures.TestTopic("parent-topic")
	if err := topics.Create(ctx, topic); err != nil {
		t.Fatalf("failed to create topic: %v", err)
	}

	t.Run("Create", func(t *testing.T) {
		dataset := fixtures.TestDataset("dataset-1", "parent-topic")
		if err := datasets.Create(ctx, dataset); err != nil {
			t.Fatalf("failed to create dataset: %v", err)
		}
	})

	t.Run("Get", func(t *testing.T) {
		dataset, err := datasets.Get(ctx, "dataset-1")
		if err != nil {
			t.Fatalf("failed to get dataset: %v", err)
		}
		if dataset.Name != "Test Dataset dataset-1" {
			t.Errorf("unexpected dataset name: %s", dataset.Name)
		}
	})

	t.Run("ListByTopic", func(t *testing.T) {
		topicID := domain.TopicID("parent-topic")
		list, err := datasets.List(ctx, storage.DatasetFilter{
			TopicID: &topicID,
			Limit:   10,
		})
		if err != nil {
			t.Fatalf("failed to list datasets: %v", err)
		}
		if len(list) < 1 {
			t.Error("expected at least one dataset")
		}
	})
}

// TestSQLiteJobRepository_Integration tests job CRUD operations.
func TestSQLiteJobRepository_Integration(t *testing.T) {
	ctx := testutil.TestContext(t)
	dataDir := testutil.TempDir(t, "sqlite")

	store := createSQLiteStore(t, ctx, dataDir)
	defer store.Close()

	topics := store.Topics()
	jobs := store.Jobs()

	// Create parent topic first
	topic := fixtures.TestTopic("job-topic")
	if err := topics.Create(ctx, topic); err != nil {
		t.Fatalf("failed to create topic: %v", err)
	}

	t.Run("Create", func(t *testing.T) {
		job := fixtures.TestJob("job-1", "job-topic")
		if err := jobs.Create(ctx, job); err != nil {
			t.Fatalf("failed to create job: %v", err)
		}
	})

	t.Run("Get", func(t *testing.T) {
		job, err := jobs.Get(ctx, "job-1")
		if err != nil {
			t.Fatalf("failed to get job: %v", err)
		}
		if job.ID != "job-1" {
			t.Errorf("unexpected job ID: %s", job.ID)
		}
	})
}

// createSQLiteStore creates a configured SQLite store for testing.
func createSQLiteStore(t *testing.T, ctx context.Context, dataDir string) storage.Store {
	t.Helper()

	cfg := storage.SQLiteConfig{
		Path: dataDir + "/bib.db",
	}

	store, err := sqlite.New(cfg, dataDir, "test-node")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	if err := store.Migrate(ctx); err != nil {
		store.Close()
		t.Fatalf("failed to migrate: %v", err)
	}

	return store
}
