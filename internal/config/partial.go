// Package config provides configuration loading and management for bib and bibd.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SetupProgress tracks the state of an interrupted setup wizard
type SetupProgress struct {
	// Version is the schema version for forward compatibility
	Version int `json:"version"`

	// AppName is either "bib" or "bibd"
	AppName string `json:"app_name"`

	// StartedAt is when the setup was started
	StartedAt time.Time `json:"started_at"`

	// LastUpdatedAt is when the progress was last saved
	LastUpdatedAt time.Time `json:"last_updated_at"`

	// CurrentStepID is the ID of the current/interrupted step
	CurrentStepID string `json:"current_step_id"`

	// CurrentStepIndex is the 0-based index of the current step
	CurrentStepIndex int `json:"current_step_index"`

	// TotalSteps is the total number of steps in the wizard
	TotalSteps int `json:"total_steps"`

	// CompletedSteps is a list of step IDs that have been completed
	CompletedSteps []string `json:"completed_steps"`

	// Data holds the partially collected setup data as JSON
	// This is stored as raw JSON to allow for schema evolution
	Data json.RawMessage `json:"data"`

	// DeploymentTarget is stored separately for quick access
	DeploymentTarget string `json:"deployment_target,omitempty"`

	// IsDaemon indicates if this is a daemon setup
	IsDaemon bool `json:"is_daemon"`
}

const (
	// partialConfigVersion is the current schema version
	partialConfigVersion = 1

	// partialConfigSuffix is the file suffix for partial configs
	partialConfigSuffix = ".partial"
)

// PartialConfigPath returns the path to the partial config file
func PartialConfigPath(appName string) (string, error) {
	configDir, err := UserConfigDir(appName)
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.yaml"+partialConfigSuffix), nil
}

// DetectPartialConfig checks if a partial config exists and returns it
// Returns nil, nil if no partial config exists
func DetectPartialConfig(appName string) (*SetupProgress, error) {
	path, err := PartialConfigPath(appName)
	if err != nil {
		return nil, err
	}

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}

	// Load and parse
	return LoadPartialConfig(appName)
}

// LoadPartialConfig loads a partial config from disk
func LoadPartialConfig(appName string) (*SetupProgress, error) {
	path, err := PartialConfigPath(appName)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read partial config: %w", err)
	}

	var progress SetupProgress
	if err := json.Unmarshal(data, &progress); err != nil {
		return nil, fmt.Errorf("failed to parse partial config: %w", err)
	}

	// Version check for forward compatibility
	if progress.Version > partialConfigVersion {
		return nil, fmt.Errorf("partial config version %d is newer than supported version %d",
			progress.Version, partialConfigVersion)
	}

	return &progress, nil
}

// SavePartialConfig saves the current setup progress to disk
func SavePartialConfig(progress *SetupProgress) error {
	if progress.AppName == "" {
		return fmt.Errorf("app name is required")
	}

	// Ensure version and timestamps are set
	progress.Version = partialConfigVersion
	progress.LastUpdatedAt = time.Now()
	if progress.StartedAt.IsZero() {
		progress.StartedAt = progress.LastUpdatedAt
	}

	path, err := PartialConfigPath(progress.AppName)
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to JSON with indentation for readability
	data, err := json.MarshalIndent(progress, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal partial config: %w", err)
	}

	// Write atomically using temp file + rename
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write partial config: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath) // Clean up on failure, ignore error
		return fmt.Errorf("failed to save partial config: %w", err)
	}

	return nil
}

// DeletePartialConfig removes the partial config file
func DeletePartialConfig(appName string) error {
	path, err := PartialConfigPath(appName)
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete partial config: %w", err)
	}

	return nil
}

// NewSetupProgress creates a new SetupProgress for tracking wizard state
func NewSetupProgress(appName string, isDaemon bool, totalSteps int) *SetupProgress {
	return &SetupProgress{
		Version:          partialConfigVersion,
		AppName:          appName,
		StartedAt:        time.Now(),
		LastUpdatedAt:    time.Now(),
		CurrentStepID:    "",
		CurrentStepIndex: 0,
		TotalSteps:       totalSteps,
		CompletedSteps:   []string{},
		IsDaemon:         isDaemon,
	}
}

// MarkStepCompleted marks a step as completed and advances to the next
func (p *SetupProgress) MarkStepCompleted(stepID string) {
	// Add to completed list if not already there
	for _, s := range p.CompletedSteps {
		if s == stepID {
			return
		}
	}
	p.CompletedSteps = append(p.CompletedSteps, stepID)
}

// IsStepCompleted checks if a step has been completed
func (p *SetupProgress) IsStepCompleted(stepID string) bool {
	for _, s := range p.CompletedSteps {
		if s == stepID {
			return true
		}
	}
	return false
}

// SetCurrentStep updates the current step
func (p *SetupProgress) SetCurrentStep(stepID string, stepIndex int) {
	p.CurrentStepID = stepID
	p.CurrentStepIndex = stepIndex
	p.LastUpdatedAt = time.Now()
}

// SetData stores the setup data as JSON
func (p *SetupProgress) SetData(data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal setup data: %w", err)
	}
	p.Data = jsonData
	return nil
}

// GetData unmarshals the stored setup data into the provided struct
func (p *SetupProgress) GetData(target interface{}) error {
	if p.Data == nil {
		return nil // No data stored yet
	}
	if err := json.Unmarshal(p.Data, target); err != nil {
		return fmt.Errorf("failed to unmarshal setup data: %w", err)
	}
	return nil
}

// ProgressPercentage returns the completion percentage (0-100)
func (p *SetupProgress) ProgressPercentage() int {
	if p.TotalSteps == 0 {
		return 0
	}
	return (len(p.CompletedSteps) * 100) / p.TotalSteps
}

// TimeSinceStart returns the duration since setup was started
func (p *SetupProgress) TimeSinceStart() time.Duration {
	return time.Since(p.StartedAt)
}

// TimeSinceLastUpdate returns the duration since the last update
func (p *SetupProgress) TimeSinceLastUpdate() time.Duration {
	return time.Since(p.LastUpdatedAt)
}

// Summary returns a human-readable summary of the progress
func (p *SetupProgress) Summary() string {
	stepInfo := fmt.Sprintf("Step %d of %d", p.CurrentStepIndex+1, p.TotalSteps)
	if p.CurrentStepID != "" {
		stepInfo = fmt.Sprintf("%s (%s)", stepInfo, p.CurrentStepID)
	}
	return fmt.Sprintf("%s - %d%% complete", stepInfo, p.ProgressPercentage())
}
