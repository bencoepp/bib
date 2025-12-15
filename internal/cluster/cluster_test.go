package cluster

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"bib/internal/config"
)

func TestNewCluster(t *testing.T) {
	// Test that cluster returns nil when disabled
	cfg := config.ClusterConfig{
		Enabled: false,
	}

	c, err := New(cfg, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c != nil {
		t.Fatal("expected nil cluster when disabled")
	}
}

func TestNewCluster_Enabled(t *testing.T) {
	cfg := config.ClusterConfig{
		Enabled:     true,
		ClusterName: "test-cluster",
		ListenAddr:  "127.0.0.1:0", // Random port
		Raft: config.RaftConfig{
			HeartbeatTimeout: 1 * time.Second,
			ElectionTimeout:  5 * time.Second,
		},
	}

	c, err := New(cfg, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil cluster when enabled")
	}

	// Verify node ID was generated
	if c.NodeID() == "" {
		t.Fatal("expected node ID to be generated")
	}

	// Verify initial state
	if c.State() != StateFollower {
		t.Errorf("expected initial state to be follower, got %s", c.State())
	}
}

func TestNewCluster_CustomNodeID(t *testing.T) {
	cfg := config.ClusterConfig{
		Enabled:     true,
		NodeID:      "custom-node-id",
		ClusterName: "test-cluster",
	}

	c, err := New(cfg, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if c.NodeID() != "custom-node-id" {
		t.Errorf("expected node ID 'custom-node-id', got %s", c.NodeID())
	}
}

func TestGenerateNodeID(t *testing.T) {
	id1, err := generateNodeID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(id1) != 32 { // 16 bytes * 2 hex chars
		t.Errorf("expected 32 char node ID, got %d", len(id1))
	}

	id2, err := generateNodeID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id1 == id2 {
		t.Error("expected unique node IDs")
	}
}

func TestStorage(t *testing.T) {
	tempDir := t.TempDir()
	cfg := config.ClusterConfig{
		DataDir: tempDir,
		Snapshot: config.SnapshotConfig{
			RetainCount: 3,
		},
	}

	s, err := NewStorage(cfg, tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer s.Close()

	// Verify directories were created
	snapshotDir := filepath.Join(tempDir, "snapshots")
	if _, err := os.Stat(snapshotDir); os.IsNotExist(err) {
		t.Error("snapshot directory was not created")
	}

	// Test hard state operations
	state := &HardState{
		Term:   5,
		Vote:   "node-1",
		Commit: 100,
	}
	if err := s.SetHardState(state); err != nil {
		t.Fatalf("failed to set hard state: %v", err)
	}

	loaded, err := s.GetHardState()
	if err != nil {
		t.Fatalf("failed to get hard state: %v", err)
	}
	if loaded.Term != state.Term {
		t.Errorf("term mismatch: expected %d, got %d", state.Term, loaded.Term)
	}
	if loaded.Vote != state.Vote {
		t.Errorf("vote mismatch: expected %s, got %s", state.Vote, loaded.Vote)
	}
	if loaded.Commit != state.Commit {
		t.Errorf("commit mismatch: expected %d, got %d", state.Commit, loaded.Commit)
	}
}

func TestStorageLogOperations(t *testing.T) {
	tempDir := t.TempDir()
	cfg := config.ClusterConfig{
		DataDir: tempDir,
	}

	s, err := NewStorage(cfg, tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer s.Close()

	// Test empty log
	first, err := s.FirstIndex()
	if err != nil {
		t.Fatalf("failed to get first index: %v", err)
	}
	if first != 0 {
		t.Errorf("expected first index 0, got %d", first)
	}

	// Store some logs
	entries := []*LogEntry{
		{Index: 1, Term: 1, Type: 0, Data: []byte("entry1")},
		{Index: 2, Term: 1, Type: 0, Data: []byte("entry2")},
		{Index: 3, Term: 2, Type: 0, Data: []byte("entry3")},
	}
	if err := s.StoreLogs(entries); err != nil {
		t.Fatalf("failed to store logs: %v", err)
	}

	// Verify first and last
	first, err = s.FirstIndex()
	if err != nil {
		t.Fatalf("failed to get first index: %v", err)
	}
	if first != 1 {
		t.Errorf("expected first index 1, got %d", first)
	}

	last, err := s.LastIndex()
	if err != nil {
		t.Fatalf("failed to get last index: %v", err)
	}
	if last != 3 {
		t.Errorf("expected last index 3, got %d", last)
	}

	// Get specific log
	entry, err := s.GetLog(2)
	if err != nil {
		t.Fatalf("failed to get log: %v", err)
	}
	if entry == nil {
		t.Fatal("expected entry, got nil")
	}
	if entry.Term != 1 {
		t.Errorf("expected term 1, got %d", entry.Term)
	}
	if string(entry.Data) != "entry2" {
		t.Errorf("expected data 'entry2', got %s", string(entry.Data))
	}

	// Delete range
	if err := s.DeleteRange(1, 2); err != nil {
		t.Fatalf("failed to delete range: %v", err)
	}

	first, err = s.FirstIndex()
	if err != nil {
		t.Fatalf("failed to get first index after delete: %v", err)
	}
	if first != 3 {
		t.Errorf("expected first index 3 after delete, got %d", first)
	}
}

func TestStorageMemberOperations(t *testing.T) {
	tempDir := t.TempDir()
	cfg := config.ClusterConfig{
		DataDir: tempDir,
	}

	s, err := NewStorage(cfg, tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer s.Close()

	// Add members
	member1 := &ClusterMember{
		NodeID:  "node-1",
		Address: "127.0.0.1:4002",
		Role:    RoleVoter,
	}
	if err := s.AddMember(member1); err != nil {
		t.Fatalf("failed to add member: %v", err)
	}

	member2 := &ClusterMember{
		NodeID:  "node-2",
		Address: "127.0.0.1:4003",
		Role:    RoleNonVoter,
	}
	if err := s.AddMember(member2); err != nil {
		t.Fatalf("failed to add member: %v", err)
	}

	// Get members
	members, err := s.GetMembers()
	if err != nil {
		t.Fatalf("failed to get members: %v", err)
	}
	if len(members) != 2 {
		t.Errorf("expected 2 members, got %d", len(members))
	}

	// Remove member
	if err := s.RemoveMember("node-1"); err != nil {
		t.Fatalf("failed to remove member: %v", err)
	}

	members, err = s.GetMembers()
	if err != nil {
		t.Fatalf("failed to get members after remove: %v", err)
	}
	if len(members) != 1 {
		t.Errorf("expected 1 member after remove, got %d", len(members))
	}
}

func TestStorageJoinToken(t *testing.T) {
	tempDir := t.TempDir()
	cfg := config.ClusterConfig{
		DataDir: tempDir,
	}

	s, err := NewStorage(cfg, tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer s.Close()

	// Create token
	token := &JoinToken{
		ClusterName: "test-cluster",
		LeaderAddr:  "127.0.0.1:4002",
		Token:       "test-token-123",
		ExpiresAt:   time.Now().Add(1 * time.Hour),
	}
	if err := s.StoreJoinToken(token); err != nil {
		t.Fatalf("failed to store join token: %v", err)
	}

	// Validate token
	validated, err := s.ValidateJoinToken("test-token-123")
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}
	if validated.ClusterName != token.ClusterName {
		t.Errorf("cluster name mismatch")
	}

	// Try to use again (should fail)
	_, err = s.ValidateJoinToken("test-token-123")
	if err == nil {
		t.Error("expected error for already used token")
	}
}

func TestStorageSnapshot(t *testing.T) {
	tempDir := t.TempDir()
	cfg := config.ClusterConfig{
		DataDir: tempDir,
		Snapshot: config.SnapshotConfig{
			RetainCount: 2,
		},
	}

	s, err := NewStorage(cfg, tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer s.Close()

	// Create snapshot
	data := []byte("snapshot data")
	config := []byte(`{"node-1":"127.0.0.1:4002"}`)

	meta, err := s.CreateSnapshot(100, 5, config, data)
	if err != nil {
		t.Fatalf("failed to create snapshot: %v", err)
	}
	if meta.Index != 100 {
		t.Errorf("expected index 100, got %d", meta.Index)
	}
	if meta.Term != 5 {
		t.Errorf("expected term 5, got %d", meta.Term)
	}

	// Get latest snapshot
	latest, err := s.GetLatestSnapshot()
	if err != nil {
		t.Fatalf("failed to get latest snapshot: %v", err)
	}
	if latest.ID != meta.ID {
		t.Errorf("expected snapshot ID %s, got %s", meta.ID, latest.ID)
	}

	// Read snapshot data
	readData, err := s.ReadSnapshot(meta.ID)
	if err != nil {
		t.Fatalf("failed to read snapshot: %v", err)
	}
	if string(readData) != string(data) {
		t.Errorf("snapshot data mismatch")
	}

	// List snapshots
	snapshots, err := s.ListSnapshots()
	if err != nil {
		t.Fatalf("failed to list snapshots: %v", err)
	}
	if len(snapshots) != 1 {
		t.Errorf("expected 1 snapshot, got %d", len(snapshots))
	}
}

func TestFSM(t *testing.T) {
	tempDir := t.TempDir()
	cfg := config.ClusterConfig{
		DataDir: tempDir,
	}

	s, err := NewStorage(cfg, tempDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer s.Close()

	fsm := NewFSM(s)

	// Test catalog update
	entry := ReplicatedCatalogEntry{
		TopicID:     "topic-1",
		DatasetID:   "dataset-1",
		Hash:        "abc123",
		Size:        1024,
		ChunkCount:  4,
		OwnerPeerID: "peer-1",
		UpdatedAt:   time.Now(),
	}

	cmd, err := CreateCommand(CmdCatalogUpdate, entry)
	if err != nil {
		t.Fatalf("failed to create command: %v", err)
	}

	if err := fsm.Apply(cmd); err != nil {
		t.Fatalf("failed to apply command: %v", err)
	}

	// Verify catalog
	catalog := fsm.GetCatalog()
	if len(catalog) != 1 {
		t.Errorf("expected 1 catalog entry, got %d", len(catalog))
	}

	// Test config set
	configCmd, err := CreateCommand(CmdConfigSet, struct {
		Key   string `json:"key"`
		Value []byte `json:"value"`
	}{
		Key:   "test-key",
		Value: []byte("test-value"),
	})
	if err != nil {
		t.Fatalf("failed to create config command: %v", err)
	}

	if err := fsm.Apply(configCmd); err != nil {
		t.Fatalf("failed to apply config command: %v", err)
	}

	value := fsm.GetConfig("test-key")
	if string(value) != "test-value" {
		t.Errorf("expected config value 'test-value', got %s", string(value))
	}

	// Test snapshot and restore
	snapshotData, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("failed to create snapshot: %v", err)
	}

	// Create new FSM and restore
	s2, err := NewStorage(cfg, t.TempDir())
	if err != nil {
		t.Fatalf("failed to create storage2: %v", err)
	}
	defer s2.Close()

	fsm2 := NewFSM(s2)
	if err := fsm2.Restore(snapshotData); err != nil {
		t.Fatalf("failed to restore: %v", err)
	}

	// Verify restored data
	catalog2 := fsm2.GetCatalog()
	if len(catalog2) != 1 {
		t.Errorf("expected 1 catalog entry after restore, got %d", len(catalog2))
	}

	value2 := fsm2.GetConfig("test-key")
	if string(value2) != "test-value" {
		t.Errorf("expected restored config value 'test-value', got %s", string(value2))
	}
}
