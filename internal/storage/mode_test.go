package storage

import (
	"testing"
)

func TestEnforceMode_SQLiteFull(t *testing.T) {
	result := EnforceMode(NodeModeFull, BackendSQLite)

	if result.Valid != true {
		t.Error("should be valid (downgraded)")
	}

	if result.Downgraded != true {
		t.Error("should be downgraded")
	}

	if result.EffectiveMode != NodeModeSelective {
		t.Errorf("effective mode should be selective, got %s", result.EffectiveMode)
	}

	if result.IsTrustedStorage != false {
		t.Error("SQLite should not be trusted storage")
	}

	if result.Warning == "" {
		t.Error("should have a warning message")
	}
}

func TestEnforceMode_SQLiteSelective(t *testing.T) {
	result := EnforceMode(NodeModeSelective, BackendSQLite)

	if result.Valid != true {
		t.Error("should be valid")
	}

	if result.Downgraded != false {
		t.Error("should not be downgraded")
	}

	if result.EffectiveMode != NodeModeSelective {
		t.Errorf("effective mode should be selective, got %s", result.EffectiveMode)
	}

	if result.IsTrustedStorage != false {
		t.Error("SQLite should not be trusted storage")
	}
}

func TestEnforceMode_SQLiteProxy(t *testing.T) {
	result := EnforceMode(NodeModeProxy, BackendSQLite)

	if result.Valid != true {
		t.Error("should be valid")
	}

	if result.Downgraded != false {
		t.Error("should not be downgraded")
	}

	if result.EffectiveMode != NodeModeProxy {
		t.Errorf("effective mode should be proxy, got %s", result.EffectiveMode)
	}
}

func TestEnforceMode_PostgresFull(t *testing.T) {
	result := EnforceMode(NodeModeFull, BackendPostgres)

	if result.Valid != true {
		t.Error("should be valid")
	}

	if result.Downgraded != false {
		t.Error("should not be downgraded")
	}

	if result.EffectiveMode != NodeModeFull {
		t.Errorf("effective mode should be full, got %s", result.EffectiveMode)
	}

	if result.IsTrustedStorage != true {
		t.Error("PostgreSQL should be trusted storage")
	}

	if result.Warning != "" {
		t.Errorf("should not have a warning, got: %s", result.Warning)
	}
}

func TestEnforceMode_PostgresSelective(t *testing.T) {
	result := EnforceMode(NodeModeSelective, BackendPostgres)

	if result.Valid != true {
		t.Error("should be valid")
	}

	if result.IsTrustedStorage != true {
		t.Error("PostgreSQL should be trusted storage")
	}
}

func TestEnforceMode_UnknownBackend(t *testing.T) {
	result := EnforceMode(NodeModeFull, BackendType("unknown"))

	if result.Valid != false {
		t.Error("should be invalid")
	}

	if result.Error == "" {
		t.Error("should have an error message")
	}
}

func TestValidateModeChange_SQLiteToFull(t *testing.T) {
	err := ValidateModeChange(NodeModeProxy, NodeModeFull, BackendSQLite)

	if err == nil {
		t.Error("should return an error")
	}

	// Should not be a restart required error
	if IsModeChangeRestartRequired(err) {
		t.Error("should not be a restart required error")
	}
}

func TestValidateModeChange_RestartRequired(t *testing.T) {
	err := ValidateModeChange(NodeModeProxy, NodeModeSelective, BackendPostgres)

	if err == nil {
		t.Error("should return an error (restart required)")
	}

	if !IsModeChangeRestartRequired(err) {
		t.Error("should be a restart required error")
	}
}

func TestValidateModeChange_NoChange(t *testing.T) {
	err := ValidateModeChange(NodeModeFull, NodeModeFull, BackendPostgres)

	if err != nil {
		t.Errorf("should not return an error for no change: %v", err)
	}
}

func TestPeerStorageMetadata(t *testing.T) {
	// PostgreSQL full mode
	meta := PeerStorageMetadata(BackendPostgres, NodeModeFull)

	if meta["storage_backend"] != "postgres" {
		t.Errorf("wrong backend: %s", meta["storage_backend"])
	}

	if meta["trusted_storage"] != "true" {
		t.Errorf("PostgreSQL should be trusted: %s", meta["trusted_storage"])
	}

	if meta["authoritative"] != "true" {
		t.Errorf("PostgreSQL full should be authoritative: %s", meta["authoritative"])
	}

	if meta["cache_only"] != "false" {
		t.Errorf("PostgreSQL should not be cache only: %s", meta["cache_only"])
	}

	// SQLite proxy mode
	meta = PeerStorageMetadata(BackendSQLite, NodeModeProxy)

	if meta["trusted_storage"] != "false" {
		t.Errorf("SQLite should not be trusted: %s", meta["trusted_storage"])
	}

	if meta["cache_only"] != "true" {
		t.Errorf("SQLite should be cache only: %s", meta["cache_only"])
	}
}

func TestNodeMode_IsValid(t *testing.T) {
	tests := []struct {
		mode  NodeMode
		valid bool
	}{
		{NodeModeFull, true},
		{NodeModeSelective, true},
		{NodeModeProxy, true},
		{NodeMode("unknown"), false},
		{NodeMode(""), false},
	}

	for _, tt := range tests {
		if tt.mode.IsValid() != tt.valid {
			t.Errorf("NodeMode(%q).IsValid() = %v, want %v", tt.mode, tt.mode.IsValid(), tt.valid)
		}
	}
}
