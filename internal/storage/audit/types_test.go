package audit

import (
	"testing"
	"time"
)

func TestParseAction(t *testing.T) {
	tests := []struct {
		query    string
		expected Action
	}{
		{"SELECT * FROM users", ActionSelect},
		{"select id from users", ActionSelect},
		{"  SELECT id FROM users WHERE id = 1", ActionSelect},
		{"INSERT INTO users (name) VALUES ('test')", ActionInsert},
		{"insert into users (name) values ('test')", ActionInsert},
		{"UPDATE users SET name = 'test' WHERE id = 1", ActionUpdate},
		{"update users set name = 'test'", ActionUpdate},
		{"DELETE FROM users WHERE id = 1", ActionDelete},
		{"delete from users", ActionDelete},
		{"CREATE TABLE users (id INT)", ActionDDL},
		{"ALTER TABLE users ADD COLUMN email TEXT", ActionDDL},
		{"DROP TABLE users", ActionDDL},
		{"TRUNCATE TABLE users", ActionDDL},
		{"EXPLAIN SELECT * FROM users", ActionOther},
		{"SET search_path = public", ActionOther},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := ParseAction(tt.query)
			if result != tt.expected {
				t.Errorf("ParseAction(%q) = %v, want %v", tt.query, result, tt.expected)
			}
		})
	}
}

func TestExtractTableName(t *testing.T) {
	tests := []struct {
		query    string
		expected string
	}{
		{"SELECT * FROM users", "users"},
		{"SELECT * FROM users WHERE id = 1", "users"},
		{"SELECT * FROM \"users\" WHERE id = 1", "users"},
		{"INSERT INTO users (name) VALUES ('test')", "users"},
		{"INSERT INTO public.users (name) VALUES ('test')", "public.users"},
		{"UPDATE users SET name = 'test'", "users"},
		{"DELETE FROM users WHERE id = 1", "users"},
		{"CREATE TABLE users (id INT)", "users"},
		{"CREATE TABLE IF NOT EXISTS users (id INT)", "users"},
		{"DROP TABLE users", "users"},
		{"DROP TABLE IF EXISTS users", "users"},
		{"ALTER TABLE users ADD COLUMN email TEXT", "users"},
		{"TRUNCATE TABLE users", "users"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := ExtractTableName(tt.query)
			if result != tt.expected {
				t.Errorf("ExtractTableName(%q) = %q, want %q", tt.query, result, tt.expected)
			}
		})
	}
}

func TestNewEntry(t *testing.T) {
	entry := NewEntry("node-1", "op-123", "bibd_query", "test", ActionSelect)

	if entry.NodeID != "node-1" {
		t.Errorf("NodeID = %q, want %q", entry.NodeID, "node-1")
	}
	if entry.OperationID != "op-123" {
		t.Errorf("OperationID = %q, want %q", entry.OperationID, "op-123")
	}
	if entry.RoleUsed != "bibd_query" {
		t.Errorf("RoleUsed = %q, want %q", entry.RoleUsed, "bibd_query")
	}
	if entry.SourceComponent != "test" {
		t.Errorf("SourceComponent = %q, want %q", entry.SourceComponent, "test")
	}
	if entry.Action != ActionSelect {
		t.Errorf("Action = %v, want %v", entry.Action, ActionSelect)
	}
	if entry.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
	if entry.Metadata == nil {
		t.Error("Metadata should not be nil")
	}
}

func TestEntry_CalculateHash(t *testing.T) {
	entry := &Entry{
		Timestamp:       time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		NodeID:          "node-1",
		OperationID:     "op-123",
		RoleUsed:        "bibd_query",
		Action:          ActionSelect,
		TableName:       "users",
		SourceComponent: "test",
		RowsAffected:    10,
		DurationMS:      100,
		PrevHash:        "abc123",
		JobID:           "job-456",
		QueryHash:       "qhash",
	}

	hash1 := entry.CalculateHash()
	hash2 := entry.CalculateHash()

	if hash1 != hash2 {
		t.Error("CalculateHash should be deterministic")
	}

	// Change a field and verify hash changes
	entry.RowsAffected = 20
	hash3 := entry.CalculateHash()

	if hash1 == hash3 {
		t.Error("Hash should change when entry changes")
	}
}

func TestEntry_SetHashChain(t *testing.T) {
	entry := NewEntry("node-1", "op-123", "bibd_query", "test", ActionSelect)
	entry.TableName = "users"
	entry.RowsAffected = 10
	entry.DurationMS = 100

	prevHash := "previous-hash-value"
	entry.SetHashChain(prevHash)

	if entry.PrevHash != prevHash {
		t.Errorf("PrevHash = %q, want %q", entry.PrevHash, prevHash)
	}
	if entry.EntryHash == "" {
		t.Error("EntryHash should not be empty")
	}
}

func TestEntry_ToJSON(t *testing.T) {
	entry := NewEntry("node-1", "op-123", "bibd_query", "test", ActionSelect)
	entry.TableName = "users"

	data, err := entry.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}

	if len(data) == 0 {
		t.Error("ToJSON should return non-empty data")
	}
}

func TestGenerateOperationID(t *testing.T) {
	id1 := GenerateOperationID()
	id2 := GenerateOperationID()

	if id1 == "" {
		t.Error("GenerateOperationID should not return empty string")
	}
	if id1 == id2 {
		t.Error("GenerateOperationID should return unique IDs")
	}
}
