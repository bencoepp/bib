package certs

import (
	"os"
	"testing"
)

func TestTrustStore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-trust-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ts, err := NewTrustStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create trust store: %v", err)
	}

	// Initially empty
	nodes := ts.List()
	if len(nodes) != 0 {
		t.Errorf("expected empty list, got %d nodes", len(nodes))
	}

	// Add a node
	node := &TrustedNode{
		NodeID:      "12D3KooWTestNode1",
		Fingerprint: "abc123def456",
		Certificate: "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
		Alias:       "test-node",
		Address:     "/ip4/127.0.0.1/tcp/4001",
		TrustMethod: TrustMethodTOFU,
	}

	if err := ts.Add(node); err != nil {
		t.Fatalf("failed to add node: %v", err)
	}

	// Verify file was created
	files, _ := os.ReadDir(tmpDir)
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}

	// Get node
	retrieved, ok := ts.Get("12D3KooWTestNode1")
	if !ok {
		t.Fatal("node not found")
	}
	if retrieved.Alias != "test-node" {
		t.Errorf("alias mismatch: got %s, want test-node", retrieved.Alias)
	}
	if retrieved.FirstSeen.IsZero() {
		t.Error("FirstSeen should be set")
	}

	// Check trust
	trusted, err := ts.IsTrusted("12D3KooWTestNode1", "abc123def456")
	if err != nil {
		t.Fatalf("IsTrusted failed: %v", err)
	}
	if !trusted {
		t.Error("node should be trusted")
	}

	// Check with wrong fingerprint
	_, err = ts.IsTrusted("12D3KooWTestNode1", "wrongfingerprint")
	if err == nil {
		t.Error("should return error for mismatched fingerprint")
	}

	// Unknown node should not be trusted
	trusted, err = ts.IsTrusted("unknownnode", "abc123")
	if err != nil {
		t.Fatalf("IsTrusted should not error for unknown node: %v", err)
	}
	if trusted {
		t.Error("unknown node should not be trusted")
	}
}

func TestTrustStorePersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-trust-persist-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create and add node
	ts1, _ := NewTrustStore(tmpDir)
	ts1.Add(&TrustedNode{
		NodeID:      "persistnode",
		Fingerprint: "fingerprint123",
		TrustMethod: TrustMethodManual,
		Verified:    true,
	})

	// Create new trust store (simulating restart)
	ts2, err := NewTrustStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to reload trust store: %v", err)
	}

	// Verify node was loaded
	node, ok := ts2.Get("persistnode")
	if !ok {
		t.Fatal("node should be loaded from disk")
	}
	if node.Fingerprint != "fingerprint123" {
		t.Errorf("fingerprint mismatch: got %s", node.Fingerprint)
	}
	if node.TrustMethod != TrustMethodManual {
		t.Errorf("trust method mismatch: got %s", node.TrustMethod)
	}
	if !node.Verified {
		t.Error("verified should be true")
	}
}

func TestTrustStorePin(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-trust-pin-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ts, _ := NewTrustStore(tmpDir)
	ts.Add(&TrustedNode{
		NodeID:      "pinnode",
		Fingerprint: "fp",
		TrustMethod: TrustMethodTOFU,
	})

	// Pin the node
	if err := ts.Pin("pinnode"); err != nil {
		t.Fatalf("failed to pin: %v", err)
	}

	node, _ := ts.Get("pinnode")
	if node.TrustMethod != TrustMethodPinned {
		t.Errorf("trust method should be pinned: got %s", node.TrustMethod)
	}
	if node.PinnedAt == nil {
		t.Error("PinnedAt should be set")
	}
}

func TestTrustStoreRemove(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-trust-remove-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ts, _ := NewTrustStore(tmpDir)
	ts.Add(&TrustedNode{
		NodeID:      "removenode",
		Fingerprint: "fp",
	})

	if err := ts.Remove("removenode"); err != nil {
		t.Fatalf("failed to remove: %v", err)
	}

	_, ok := ts.Get("removenode")
	if ok {
		t.Error("node should be removed")
	}

	// File should be deleted
	files, _ := os.ReadDir(tmpDir)
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestTrustStoreGetByFingerprint(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-trust-fp-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ts, _ := NewTrustStore(tmpDir)
	ts.Add(&TrustedNode{
		NodeID:      "node1",
		Fingerprint: "uniquefp1",
	})
	ts.Add(&TrustedNode{
		NodeID:      "node2",
		Fingerprint: "uniquefp2",
	})

	node, ok := ts.GetByFingerprint("uniquefp2")
	if !ok {
		t.Fatal("should find node by fingerprint")
	}
	if node.NodeID != "node2" {
		t.Errorf("wrong node: got %s", node.NodeID)
	}

	_, ok = ts.GetByFingerprint("nonexistent")
	if ok {
		t.Error("should not find nonexistent fingerprint")
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with-dash", "with-dash"},
		{"with_underscore", "with_underscore"},
		{"with/slash", "with_slash"},
		{"with:colon", "with_colon"},
		{"12D3KooW...", "12D3KooW___"},
	}

	for _, tt := range tests {
		result := sanitizeFilename(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
