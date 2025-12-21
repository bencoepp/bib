package partial

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewPartialConfig(t *testing.T) {
	config := NewPartialConfig("cli")

	if config.SetupType != "cli" {
		t.Errorf("expected setup type 'cli', got %q", config.SetupType)
	}

	if config.Version != 1 {
		t.Errorf("expected version 1, got %d", config.Version)
	}

	if len(config.CompletedSteps) != 0 {
		t.Error("completed steps should be empty")
	}

	if config.StartedAt.IsZero() {
		t.Error("started_at should be set")
	}
}

func TestPartialConfig_CompleteStep(t *testing.T) {
	config := NewPartialConfig("daemon")

	config.CompleteStep(StepIdentity)

	if !config.IsStepCompleted(StepIdentity) {
		t.Error("identity step should be completed")
	}

	if config.CurrentStep != StepIdentity {
		t.Errorf("current step should be identity, got %s", config.CurrentStep)
	}

	// Completing again should not duplicate
	config.CompleteStep(StepIdentity)
	if len(config.CompletedSteps) != 1 {
		t.Error("should not duplicate completed steps")
	}

	// Complete another step
	config.CompleteStep(StepNetwork)
	if len(config.CompletedSteps) != 2 {
		t.Error("should have 2 completed steps")
	}
}

func TestPartialConfig_GetNextStep(t *testing.T) {
	config := NewPartialConfig("cli")

	// Should start with identity
	next := config.GetNextStep()
	if next != StepIdentity {
		t.Errorf("expected identity, got %s", next)
	}

	// Complete some steps
	config.CompleteStep(StepIdentity)
	config.CompleteStep(StepNetwork)

	next = config.GetNextStep()
	if next != StepStorage {
		t.Errorf("expected storage, got %s", next)
	}
}

func TestPartialConfig_Data(t *testing.T) {
	config := NewPartialConfig("cli")

	config.SetData("name", "John Doe")
	config.SetData("email", "john@example.com")
	config.SetData("port", 4000)
	config.SetData("tls_enabled", true)

	if config.GetString("name") != "John Doe" {
		t.Error("name mismatch")
	}

	if config.GetString("email") != "john@example.com" {
		t.Error("email mismatch")
	}

	if config.GetInt("port") != 4000 {
		t.Error("port mismatch")
	}

	if !config.GetBool("tls_enabled") {
		t.Error("tls_enabled should be true")
	}

	// Non-existent key
	if config.GetString("nonexistent") != "" {
		t.Error("nonexistent key should return empty string")
	}
}

func TestPartialConfig_SetError(t *testing.T) {
	config := NewPartialConfig("cli")

	config.SetError(nil)
	if config.Error != "" {
		t.Error("error should be empty for nil")
	}

	config.SetError(os.ErrNotExist)
	if config.Error == "" {
		t.Error("error should be set")
	}
}

func TestManager_SaveLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "partial-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	manager := NewManager(tmpDir)

	// Should not have partial
	if manager.HasPartial("cli") {
		t.Error("should not have partial config initially")
	}

	// Create and save
	config := NewPartialConfig("cli")
	config.SetData("name", "Test User")
	config.CompleteStep(StepIdentity)

	if err := manager.Save(config); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Should have partial now
	if !manager.HasPartial("cli") {
		t.Error("should have partial config after save")
	}

	// Load and verify
	loaded, err := manager.Load("cli")
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if loaded == nil {
		t.Fatal("loaded config is nil")
	}

	if loaded.GetString("name") != "Test User" {
		t.Error("data not preserved")
	}

	if !loaded.IsStepCompleted(StepIdentity) {
		t.Error("completed steps not preserved")
	}
}

func TestManager_Delete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "partial-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	manager := NewManager(tmpDir)

	// Create and save
	config := NewPartialConfig("daemon")
	manager.Save(config)

	if !manager.HasPartial("daemon") {
		t.Error("should have partial")
	}

	// Delete
	if err := manager.Delete("daemon"); err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	if manager.HasPartial("daemon") {
		t.Error("should not have partial after delete")
	}

	// Delete non-existent should not error
	if err := manager.Delete("nonexistent"); err != nil {
		t.Error("deleting non-existent should not error")
	}
}

func TestManager_List(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "partial-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	manager := NewManager(tmpDir)

	// Empty initially
	configs, err := manager.List()
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(configs) != 0 {
		t.Error("should be empty initially")
	}

	// Add some configs
	manager.Save(NewPartialConfig("cli"))
	manager.Save(NewPartialConfig("daemon"))

	configs, err = manager.List()
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(configs) != 2 {
		t.Errorf("expected 2 configs, got %d", len(configs))
	}
}

func TestManager_GetPartialPath(t *testing.T) {
	manager := NewManager("/home/user/.config/bib")

	path := manager.GetPartialPath("cli")
	expected := filepath.Join("/home/user/.config/bib", "cli.partial.json")

	if path != expected {
		t.Errorf("expected %s, got %s", expected, path)
	}
}

func TestManager_LoadNonExistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "partial-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	manager := NewManager(tmpDir)

	config, err := manager.Load("nonexistent")
	if err != nil {
		t.Error("loading non-existent should not error")
	}
	if config != nil {
		t.Error("should return nil for non-existent")
	}
}

func TestAllSteps(t *testing.T) {
	steps := AllSteps()

	if len(steps) < 5 {
		t.Error("should have at least 5 steps")
	}

	// First should be identity
	if steps[0] != StepIdentity {
		t.Error("first step should be identity")
	}

	// Last should be complete
	if steps[len(steps)-1] != StepComplete {
		t.Error("last step should be complete")
	}
}

func TestStepDescription(t *testing.T) {
	tests := []struct {
		step     SetupStep
		expected string
	}{
		{StepIdentity, "Identity (name, email, key)"},
		{StepNetwork, "Network (public/private)"},
		{StepComplete, "Setup complete"},
	}

	for _, tt := range tests {
		desc := StepDescription(tt.step)
		if desc != tt.expected {
			t.Errorf("StepDescription(%s) = %q, expected %q", tt.step, desc, tt.expected)
		}
	}
}

func TestFormatPartialSummary(t *testing.T) {
	config := NewPartialConfig("daemon")
	config.DeployTarget = "docker"
	config.CompleteStep(StepIdentity)
	config.CompleteStep(StepNetwork)

	summary := FormatPartialSummary(config)

	if summary == "" {
		t.Error("summary should not be empty")
	}

	// Check for key elements
	if !containsAll(summary, "daemon", "docker", "Progress") {
		t.Error("summary missing expected content")
	}
}

func TestFormatPartialSummary_Nil(t *testing.T) {
	summary := FormatPartialSummary(nil)
	if summary != "No partial config" {
		t.Errorf("expected 'No partial config', got %q", summary)
	}
}

func TestResumeOption(t *testing.T) {
	if ResumeOptionContinue != "continue" {
		t.Error("continue option mismatch")
	}
	if ResumeOptionRestart != "restart" {
		t.Error("restart option mismatch")
	}
	if ResumeOptionCancel != "cancel" {
		t.Error("cancel option mismatch")
	}
}

func TestPartialConfig_Timestamps(t *testing.T) {
	config := NewPartialConfig("cli")

	startedAt := config.StartedAt
	updatedAt := config.UpdatedAt

	time.Sleep(10 * time.Millisecond)

	config.SetData("test", "value")

	if config.UpdatedAt.Equal(updatedAt) || config.UpdatedAt.Before(updatedAt) {
		t.Error("updated_at should be updated after SetData")
	}

	if !config.StartedAt.Equal(startedAt) {
		t.Error("started_at should not change")
	}
}

func containsAll(s string, substrs ...string) bool {
	for _, sub := range substrs {
		found := false
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
