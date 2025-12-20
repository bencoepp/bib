//go:build integration

// Package storage_test contains integration tests for the storage layer.
package storage_test

import (
	"context"
	"testing"
	"time"

	"bib/internal/storage"
	"bib/internal/storage/postgres"
	"bib/test/testutil"
	"bib/test/testutil/containers"
	"bib/test/testutil/fixtures"
)

// TestPostgresStore_Integration tests the PostgreSQL store implementation.
func TestPostgresStore_Integration(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	// Start PostgreSQL container
	cm := containers.NewManager(t)
	pgCfg := containers.DefaultPostgresConfig()
	pgContainer, err := cm.StartPostgres(ctx, pgCfg)
	if err != nil {
		t.Fatalf("failed to start postgres: %v", err)
	}

	// Create store
	dataDir := testutil.TempDir(t, "postgres")
	storeCfg := storage.PostgresConfig{
		Managed: false,
		Advanced: &storage.AdvancedPostgresConfig{
			Host:     "localhost",
			Port:     pgContainer.HostPort(5432),
			Database: pgCfg.Database,
			User:     pgCfg.User,
			Password: pgCfg.Password,
			SSLMode:  "disable",
		},
		MaxConnections: 10,
	}

	store, err := postgres.New(ctx, storeCfg, dataDir, "test-node")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Run migrations
	if err := storage.RunMigrations(ctx, store, storage.DefaultMigrationsConfig()); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Verify connection
	if err := store.Ping(ctx); err != nil {
		t.Fatalf("failed to ping: %v", err)
	}

	// Verify backend type
	if store.Backend() != storage.BackendPostgres {
		t.Errorf("expected backend %s, got %s", storage.BackendPostgres, store.Backend())
	}

	// Verify authoritative
	if !store.IsAuthoritative() {
		t.Error("expected PostgreSQL store to be authoritative")
	}
}

// TestPostgresTopicRepository_Integration tests topic CRUD operations.
func TestPostgresTopicRepository_Integration(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	// Start PostgreSQL container
	cm := containers.NewManager(t)
	pgCfg := containers.DefaultPostgresConfig()
	pgContainer, err := cm.StartPostgres(ctx, pgCfg)
	if err != nil {
		t.Fatalf("failed to start postgres: %v", err)
	}

	// Create and initialize store
	dataDir := testutil.TempDir(t, "postgres")
	store := createPostgresStore(t, ctx, pgContainer, pgCfg, dataDir)
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
		// Create more topics
		for i := 2; i <= 5; i++ {
			topic := fixtures.TestTopic("topic-" + string(rune('0'+i)))
			_ = topics.Create(ctx, topic)
		}

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

// createPostgresStore creates a configured PostgreSQL store for testing.
func createPostgresStore(t *testing.T, ctx context.Context, container *containers.Container, pgCfg containers.PostgresConfig, dataDir string) storage.Store {
	t.Helper()

	storeCfg := storage.PostgresConfig{
		Managed: false,
		Advanced: &storage.AdvancedPostgresConfig{
			Host:     "localhost",
			Port:     container.HostPort(5432),
			Database: pgCfg.Database,
			User:     pgCfg.User,
			Password: pgCfg.Password,
			SSLMode:  "disable",
		},
		MaxConnections: 10,
	}

	store, err := postgres.New(ctx, storeCfg, dataDir, "test-node")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	if err := storage.RunMigrations(ctx, store, storage.DefaultMigrationsConfig()); err != nil {
		store.Close()
		t.Fatalf("failed to migrate: %v", err)
	}

	return store
}
