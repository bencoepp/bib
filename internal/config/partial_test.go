package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSetupProgress(t *testing.T) {
	// Create a new progress tracker
	progress := NewSetupProgress("bib", false, 10)

	if progress.Version != partialConfigVersion {
		t.Errorf("expected version %d, got %d", partialConfigVersion, progress.Version)
	}

	if progress.AppName != "bib" {
		t.Errorf("expected app name 'bib', got %q", progress.AppName)
	}

	if progress.TotalSteps != 10 {
		t.Errorf("expected 10 total steps, got %d", progress.TotalSteps)
	}

	if progress.IsDaemon {
		t.Error("expected IsDaemon to be false")
	}
}

func TestSetupProgressSteps(t *testing.T) {
	progress := NewSetupProgress("bibd", true, 5)

	// Set current step
	progress.SetCurrentStep("identity", 1)

	if progress.CurrentStepID != "identity" {
		t.Errorf("expected current step 'identity', got %q", progress.CurrentStepID)
	}

	if progress.CurrentStepIndex != 1 {
		t.Errorf("expected step index 1, got %d", progress.CurrentStepIndex)
	}

	// Mark steps as completed
	progress.MarkStepCompleted("welcome")
	progress.MarkStepCompleted("identity")

	if !progress.IsStepCompleted("welcome") {
		t.Error("expected 'welcome' step to be completed")
	}

	if !progress.IsStepCompleted("identity") {
		t.Error("expected 'identity' step to be completed")
	}

	if progress.IsStepCompleted("output") {
		t.Error("expected 'output' step to not be completed")
	}

	// Test duplicate marking
	progress.MarkStepCompleted("welcome")
	if len(progress.CompletedSteps) != 2 {
		t.Errorf("expected 2 completed steps, got %d", len(progress.CompletedSteps))
	}
}

func TestSetupProgressPercentage(t *testing.T) {
	progress := NewSetupProgress("bib", false, 4)

	if progress.ProgressPercentage() != 0 {
		t.Errorf("expected 0%%, got %d%%", progress.ProgressPercentage())
	}

	progress.MarkStepCompleted("step1")
	if progress.ProgressPercentage() != 25 {
		t.Errorf("expected 25%%, got %d%%", progress.ProgressPercentage())
	}

	progress.MarkStepCompleted("step2")
	if progress.ProgressPercentage() != 50 {
		t.Errorf("expected 50%%, got %d%%", progress.ProgressPercentage())
	}

	progress.MarkStepCompleted("step3")
	progress.MarkStepCompleted("step4")
	if progress.ProgressPercentage() != 100 {
		t.Errorf("expected 100%%, got %d%%", progress.ProgressPercentage())
	}
}

func TestSetupProgressData(t *testing.T) {
	progress := NewSetupProgress("bib", false, 5)

	// Test setting data
	testData := struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}{
		Name:  "Test User",
		Email: "test@example.com",
	}

	if err := progress.SetData(testData); err != nil {
		t.Fatalf("failed to set data: %v", err)
	}

	// Test getting data
	var retrievedData struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	if err := progress.GetData(&retrievedData); err != nil {
		t.Fatalf("failed to get data: %v", err)
	}

	if retrievedData.Name != testData.Name {
		t.Errorf("expected name %q, got %q", testData.Name, retrievedData.Name)
	}

	if retrievedData.Email != testData.Email {
		t.Errorf("expected email %q, got %q", testData.Email, retrievedData.Email)
	}
}

func TestSetupProgressSummary(t *testing.T) {
	progress := NewSetupProgress("bib", false, 10)
	progress.SetCurrentStep("identity", 1)
	progress.MarkStepCompleted("welcome")

	summary := progress.Summary()
	if summary == "" {
		t.Error("expected non-empty summary")
	}

	// Summary should contain step info and percentage
	if !containsSubstring(summary, "2 of 10") {
		t.Errorf("expected summary to contain '2 of 10', got %q", summary)
	}

	if !containsSubstring(summary, "10%") {
		t.Errorf("expected summary to contain '10%%', got %q", summary)
	}
}

func TestPartialConfigSaveLoad(t *testing.T) {
	// Use a temp directory for testing
	tmpDir := t.TempDir()

	// Override UserConfigDir for testing
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Create config directory
	configDir := filepath.Join(tmpDir, ".config", "bib")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	// Create progress
	progress := NewSetupProgress("bib", false, 5)
	progress.SetCurrentStep("identity", 1)
	progress.MarkStepCompleted("welcome")

	testData := map[string]string{"name": "Test"}
	if err := progress.SetData(testData); err != nil {
		t.Fatalf("failed to set data: %v", err)
	}

	// Save
	if err := SavePartialConfig(progress); err != nil {
		t.Fatalf("failed to save partial config: %v", err)
	}

	// Verify file exists
	path, err := PartialConfigPath("bib")
	if err != nil {
		t.Fatalf("failed to get partial config path: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("partial config file was not created")
	}

	// Load
	loaded, err := LoadPartialConfig("bib")
	if err != nil {
		t.Fatalf("failed to load partial config: %v", err)
	}

	if loaded == nil {
		t.Fatal("loaded config is nil")
	}

	if loaded.CurrentStepID != "identity" {
		t.Errorf("expected current step 'identity', got %q", loaded.CurrentStepID)
	}

	if !loaded.IsStepCompleted("welcome") {
		t.Error("expected 'welcome' to be completed")
	}

	// Test detect
	detected, err := DetectPartialConfig("bib")
	if err != nil {
		t.Fatalf("failed to detect partial config: %v", err)
	}

	if detected == nil {
		t.Fatal("detect returned nil")
	}

	// Delete
	if err := DeletePartialConfig("bib"); err != nil {
		t.Fatalf("failed to delete partial config: %v", err)
	}

	// Verify deleted
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("partial config file was not deleted")
	}

	// Detect should return nil after delete
	detected, err = DetectPartialConfig("bib")
	if err != nil {
		t.Fatalf("failed to detect after delete: %v", err)
	}

	if detected != nil {
		t.Error("detect should return nil after delete")
	}
}

func TestTimeSince(t *testing.T) {
	progress := NewSetupProgress("bib", false, 5)

	// Wait a tiny bit
	time.Sleep(10 * time.Millisecond)

	if progress.TimeSinceStart() < 10*time.Millisecond {
		t.Error("TimeSinceStart should be at least 10ms")
	}

	if progress.TimeSinceLastUpdate() < 10*time.Millisecond {
		t.Error("TimeSinceLastUpdate should be at least 10ms")
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s[1:], substr) || s[:len(substr)] == substr)
}
