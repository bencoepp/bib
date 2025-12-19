package certs

import (
	"os"
	"testing"
	"time"
)

func TestRevocationList(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "bib-revocation-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	rl, err := NewRevocationList(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to create revocation list: %v", err)
	}

	// Initially empty
	if rl.IsRevoked("somefingerprint") {
		t.Error("should not be revoked initially")
	}

	// Add revocation
	entry := &RevokedCertificate{
		Fingerprint: "abc123",
		SerialHex:   "deadbeef",
		Subject:     "CN=Test",
		Reason:      ReasonKeyCompromise,
		RevokedBy:   "admin",
		ExpiresAt:   time.Now().Add(365 * 24 * time.Hour),
	}

	if err := rl.Revoke(entry); err != nil {
		t.Fatalf("failed to revoke: %v", err)
	}

	// Check revocation
	if !rl.IsRevoked("abc123") {
		t.Error("certificate should be revoked")
	}

	// Version should increment
	if rl.Version() != 1 {
		t.Errorf("version should be 1, got %d", rl.Version())
	}

	// List should have entry
	entries := rl.List()
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}

func TestRevocationListPersistence(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "bib-revocation-persist-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Create and add revocation
	rl1, _ := NewRevocationList(tmpFile.Name())
	rl1.Revoke(&RevokedCertificate{
		Fingerprint: "persist123",
		Reason:      ReasonSuperseded,
	})

	// Create new list (simulating restart)
	rl2, err := NewRevocationList(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	if !rl2.IsRevoked("persist123") {
		t.Error("revocation should persist")
	}
}

func TestRevocationListMerge(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "bib-revocation-merge-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	rl, _ := NewRevocationList(tmpFile.Name())
	rl.Revoke(&RevokedCertificate{Fingerprint: "local1"})

	// Simulate remote data
	remoteData := &RevocationListData{
		Version: 5,
		Entries: []*RevokedCertificate{
			{Fingerprint: "remote1"},
			{Fingerprint: "remote2"},
			{Fingerprint: "local1"}, // Duplicate
		},
	}

	added, err := rl.Merge(remoteData)
	if err != nil {
		t.Fatalf("merge failed: %v", err)
	}

	// Should only add 2 new entries (remote1, remote2)
	if added != 2 {
		t.Errorf("expected 2 added, got %d", added)
	}

	// All should be revoked
	for _, fp := range []string{"local1", "remote1", "remote2"} {
		if !rl.IsRevoked(fp) {
			t.Errorf("%s should be revoked", fp)
		}
	}
}

func TestRevocationListPrune(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "bib-revocation-prune-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	rl, _ := NewRevocationList(tmpFile.Name())

	// Add old expired entry
	rl.Revoke(&RevokedCertificate{
		Fingerprint: "old",
		ExpiresAt:   time.Now().Add(-365 * 24 * time.Hour), // Expired 1 year ago
	})

	// Add recent entry
	rl.Revoke(&RevokedCertificate{
		Fingerprint: "recent",
		ExpiresAt:   time.Now().Add(365 * 24 * time.Hour), // Expires in 1 year
	})

	// Prune entries older than 30 days past expiry
	pruned, err := rl.Prune(30 * 24 * time.Hour)
	if err != nil {
		t.Fatalf("prune failed: %v", err)
	}

	if pruned != 1 {
		t.Errorf("expected 1 pruned, got %d", pruned)
	}

	if rl.IsRevoked("old") {
		t.Error("old entry should be pruned")
	}

	if !rl.IsRevoked("recent") {
		t.Error("recent entry should remain")
	}
}

func TestRevocationListRemove(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "bib-revocation-remove-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	rl, _ := NewRevocationList(tmpFile.Name())
	rl.Revoke(&RevokedCertificate{Fingerprint: "toremove"})

	if err := rl.Remove("toremove"); err != nil {
		t.Fatalf("remove failed: %v", err)
	}

	if rl.IsRevoked("toremove") {
		t.Error("should no longer be revoked")
	}

	// Remove nonexistent should error
	if err := rl.Remove("nonexistent"); err == nil {
		t.Error("removing nonexistent should error")
	}
}

func TestRevocationListExport(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "bib-revocation-export-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	rl, _ := NewRevocationList(tmpFile.Name())
	rl.Revoke(&RevokedCertificate{Fingerprint: "export1"})
	rl.Revoke(&RevokedCertificate{Fingerprint: "export2"})

	data := rl.Export()
	if data.Version != 2 {
		t.Errorf("version should be 2, got %d", data.Version)
	}
	if len(data.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(data.Entries))
	}
}
