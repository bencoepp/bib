package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bib/internal/storage"
)

func TestAuditRepository_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := storage.SQLiteConfig{
		Path:         dbPath,
		MaxOpenConns: 5,
	}

	store, err := New(cfg, tmpDir, "test-node")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	auditRepo := store.Audit()

	// Test Log
	entry := &storage.AuditEntry{
		Timestamp:       time.Now().UTC(),
		NodeID:          "test-node",
		OperationID:     "op-123",
		RoleUsed:        "bibd_query",
		Action:          "SELECT",
		TableName:       "users",
		Query:           "SELECT * FROM users WHERE id = $1",
		QueryHash:       "abc123",
		RowsAffected:    10,
		DurationMS:      50,
		SourceComponent: "test",
		Actor:           "test-user",
		Metadata:        map[string]any{"key": "value"},
		Flags: storage.AuditEntryFlags{
			Suspicious: true,
		},
	}

	err = auditRepo.Log(ctx, entry)
	if err != nil {
		t.Fatalf("Log() error = %v", err)
	}

	if entry.ID == 0 {
		t.Error("Entry ID should be set after Log()")
	}

	if entry.EntryHash == "" {
		t.Error("Entry hash should be set after Log()")
	}

	// Test Query
	entries, err := auditRepo.Query(ctx, storage.AuditFilter{
		NodeID: "test-node",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("Query() returned %d entries, want 1", len(entries))
	}

	if entries[0].OperationID != "op-123" {
		t.Errorf("OperationID = %s, want op-123", entries[0].OperationID)
	}

	if !entries[0].Flags.Suspicious {
		t.Error("Suspicious flag should be true")
	}

	// Test Count
	count, err := auditRepo.Count(ctx, storage.AuditFilter{
		NodeID: "test-node",
	})
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}

	if count != 1 {
		t.Errorf("Count() = %d, want 1", count)
	}

	// Test GetByOperationID
	entries, err = auditRepo.GetByOperationID(ctx, "op-123")
	if err != nil {
		t.Fatalf("GetByOperationID() error = %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("GetByOperationID() returned %d entries, want 1", len(entries))
	}

	// Test GetLastHash
	lastHash, err := auditRepo.GetLastHash(ctx)
	if err != nil {
		t.Fatalf("GetLastHash() error = %v", err)
	}

	if lastHash == "" {
		t.Error("GetLastHash() should return non-empty hash")
	}

	if lastHash != entry.EntryHash {
		t.Errorf("GetLastHash() = %s, want %s", lastHash, entry.EntryHash)
	}

	// Test hash chain with multiple entries
	entry2 := &storage.AuditEntry{
		Timestamp:       time.Now().UTC(),
		NodeID:          "test-node",
		OperationID:     "op-456",
		RoleUsed:        "bibd_query",
		Action:          "INSERT",
		TableName:       "users",
		SourceComponent: "test",
	}

	err = auditRepo.Log(ctx, entry2)
	if err != nil {
		t.Fatalf("Log() second entry error = %v", err)
	}

	// Verify chain
	valid, err := auditRepo.VerifyChain(ctx, entry.ID, entry2.ID)
	if err != nil {
		t.Fatalf("VerifyChain() error = %v", err)
	}

	if !valid {
		t.Error("VerifyChain() should return true for valid chain")
	}

	// Test filtering by Action
	entries, err = auditRepo.Query(ctx, storage.AuditFilter{
		Action: "SELECT",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("Query() by action error = %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("Query() by action returned %d entries, want 1", len(entries))
	}

	// Test filtering by Suspicious flag
	suspicious := true
	entries, err = auditRepo.Query(ctx, storage.AuditFilter{
		Suspicious: &suspicious,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("Query() by suspicious error = %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("Query() by suspicious returned %d entries, want 1", len(entries))
	}
}

func TestAuditRepository_HashChain(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := storage.SQLiteConfig{
		Path:         dbPath,
		MaxOpenConns: 5,
	}

	store, err := New(cfg, tmpDir, "test-node")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	auditRepo := store.Audit()

	// Create multiple entries
	var entries []*storage.AuditEntry
	for i := 0; i < 5; i++ {
		entry := &storage.AuditEntry{
			Timestamp:       time.Now().UTC(),
			NodeID:          "test-node",
			OperationID:     "op-" + string(rune('a'+i)),
			RoleUsed:        "bibd_query",
			Action:          "SELECT",
			SourceComponent: "test",
			RowsAffected:    i,
		}

		err = auditRepo.Log(ctx, entry)
		if err != nil {
			t.Fatalf("Log() error = %v", err)
		}
		entries = append(entries, entry)
	}

	// Verify entire chain
	valid, err := auditRepo.VerifyChain(ctx, entries[0].ID, entries[4].ID)
	if err != nil {
		t.Fatalf("VerifyChain() error = %v", err)
	}

	if !valid {
		t.Error("VerifyChain() should return true for valid chain")
	}
}

func TestAuditRepository_EmptyDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := storage.SQLiteConfig{
		Path:         dbPath,
		MaxOpenConns: 5,
	}

	store, err := New(cfg, tmpDir, "test-node")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	auditRepo := store.Audit()

	// Test GetLastHash on empty database
	lastHash, err := auditRepo.GetLastHash(ctx)
	if err != nil {
		t.Fatalf("GetLastHash() error = %v", err)
	}

	if lastHash != "" {
		t.Errorf("GetLastHash() on empty db = %s, want empty string", lastHash)
	}

	// Test Query on empty database
	entries, err := auditRepo.Query(ctx, storage.AuditFilter{Limit: 10})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Query() on empty db returned %d entries, want 0", len(entries))
	}

	// Test Count on empty database
	count, err := auditRepo.Count(ctx, storage.AuditFilter{})
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}

	if count != 0 {
		t.Errorf("Count() on empty db = %d, want 0", count)
	}
}

func TestNewStore_CreatesAuditRepository(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	os.Remove(dbPath)

	cfg := storage.SQLiteConfig{
		Path:         dbPath,
		MaxOpenConns: 5,
	}

	store, err := New(cfg, tmpDir, "test-node")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	auditRepo := store.Audit()
	if auditRepo == nil {
		t.Fatal("Audit() returned nil")
	}

	entry := &storage.AuditEntry{
		NodeID:          "test-node",
		OperationID:     "op-test",
		RoleUsed:        "bibd_query",
		Action:          "SELECT",
		SourceComponent: "test",
	}

	err = auditRepo.Log(ctx, entry)
	if err != nil {
		t.Fatalf("Log() error = %v", err)
	}

	if entry.ID == 0 {
		t.Error("Entry ID should be set")
	}
}
