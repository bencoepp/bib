// Package partial provides partial configuration management for setup recovery.
package partial

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SetupStep represents a step in the setup process
type SetupStep string

const (
	StepIdentity   SetupStep = "identity"
	StepNetwork    SetupStep = "network"
	StepStorage    SetupStep = "storage"
	StepDatabase   SetupStep = "database"
	StepDeployment SetupStep = "deployment"
	StepService    SetupStep = "service"
	StepNodes      SetupStep = "nodes"
	StepConfig     SetupStep = "config"
	StepComplete   SetupStep = "complete"
)

// AllSteps returns all setup steps in order
func AllSteps() []SetupStep {
	return []SetupStep{
		StepIdentity,
		StepNetwork,
		StepStorage,
		StepDatabase,
		StepDeployment,
		StepService,
		StepNodes,
		StepConfig,
		StepComplete,
	}
}

// StepDescription returns a human-readable description of a step
func StepDescription(step SetupStep) string {
	switch step {
	case StepIdentity:
		return "Identity (name, email, key)"
	case StepNetwork:
		return "Network (public/private)"
	case StepStorage:
		return "Storage (SQLite/PostgreSQL)"
	case StepDatabase:
		return "Database configuration"
	case StepDeployment:
		return "Deployment target"
	case StepService:
		return "Service installation"
	case StepNodes:
		return "Node selection"
	case StepConfig:
		return "Configuration saving"
	case StepComplete:
		return "Setup complete"
	default:
		return string(step)
	}
}

// PartialConfig stores partial setup progress
type PartialConfig struct {
	// Version is the schema version
	Version int `json:"version"`

	// SetupType is "cli" or "daemon"
	SetupType string `json:"setup_type"`

	// DeployTarget is the deployment target (local, docker, podman, kubernetes)
	DeployTarget string `json:"deploy_target,omitempty"`

	// CurrentStep is the last completed step
	CurrentStep SetupStep `json:"current_step"`

	// CompletedSteps is the list of completed steps
	CompletedSteps []SetupStep `json:"completed_steps"`

	// Data contains the partial configuration data
	Data map[string]interface{} `json:"data"`

	// StartedAt is when the setup started
	StartedAt time.Time `json:"started_at"`

	// UpdatedAt is when the config was last updated
	UpdatedAt time.Time `json:"updated_at"`

	// Error is any error that caused the setup to stop
	Error string `json:"error,omitempty"`
}

// NewPartialConfig creates a new partial config
func NewPartialConfig(setupType string) *PartialConfig {
	return &PartialConfig{
		Version:        1,
		SetupType:      setupType,
		CompletedSteps: make([]SetupStep, 0),
		Data:           make(map[string]interface{}),
		StartedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

// CompleteStep marks a step as completed
func (p *PartialConfig) CompleteStep(step SetupStep) {
	// Check if already completed
	for _, s := range p.CompletedSteps {
		if s == step {
			return
		}
	}

	p.CompletedSteps = append(p.CompletedSteps, step)
	p.CurrentStep = step
	p.UpdatedAt = time.Now()
}

// IsStepCompleted checks if a step is completed
func (p *PartialConfig) IsStepCompleted(step SetupStep) bool {
	for _, s := range p.CompletedSteps {
		if s == step {
			return true
		}
	}
	return false
}

// GetNextStep returns the next step to complete
func (p *PartialConfig) GetNextStep() SetupStep {
	steps := AllSteps()
	for _, step := range steps {
		if !p.IsStepCompleted(step) {
			return step
		}
	}
	return StepComplete
}

// SetData sets a data value
func (p *PartialConfig) SetData(key string, value interface{}) {
	p.Data[key] = value
	p.UpdatedAt = time.Now()
}

// GetData gets a data value
func (p *PartialConfig) GetData(key string) (interface{}, bool) {
	v, ok := p.Data[key]
	return v, ok
}

// GetString gets a string data value
func (p *PartialConfig) GetString(key string) string {
	if v, ok := p.Data[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetBool gets a boolean data value
func (p *PartialConfig) GetBool(key string) bool {
	if v, ok := p.Data[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// GetInt gets an integer data value
func (p *PartialConfig) GetInt(key string) int {
	if v, ok := p.Data[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		}
	}
	return 0
}

// SetError sets an error message
func (p *PartialConfig) SetError(err error) {
	if err != nil {
		p.Error = err.Error()
	} else {
		p.Error = ""
	}
	p.UpdatedAt = time.Now()
}

// Manager manages partial config files
type Manager struct {
	// ConfigDir is the directory for config files
	ConfigDir string
}

// NewManager creates a new partial config manager
func NewManager(configDir string) *Manager {
	return &Manager{
		ConfigDir: configDir,
	}
}

// GetPartialPath returns the path to a partial config file
func (m *Manager) GetPartialPath(setupType string) string {
	return filepath.Join(m.ConfigDir, fmt.Sprintf("%s.partial.json", setupType))
}

// HasPartial checks if a partial config exists
func (m *Manager) HasPartial(setupType string) bool {
	path := m.GetPartialPath(setupType)
	_, err := os.Stat(path)
	return err == nil
}

// Load loads a partial config
func (m *Manager) Load(setupType string) (*PartialConfig, error) {
	path := m.GetPartialPath(setupType)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read partial config: %w", err)
	}

	var config PartialConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse partial config: %w", err)
	}

	return &config, nil
}

// Save saves a partial config
func (m *Manager) Save(config *PartialConfig) error {
	// Ensure directory exists
	if err := os.MkdirAll(m.ConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	path := m.GetPartialPath(config.SetupType)

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal partial config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write partial config: %w", err)
	}

	return nil
}

// Delete removes a partial config
func (m *Manager) Delete(setupType string) error {
	path := m.GetPartialPath(setupType)

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete partial config: %w", err)
	}

	return nil
}

// List returns all partial configs
func (m *Manager) List() ([]*PartialConfig, error) {
	entries, err := os.ReadDir(m.ConfigDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var configs []*PartialConfig

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !isPartialFile(name) {
			continue
		}

		path := filepath.Join(m.ConfigDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var config PartialConfig
		if err := json.Unmarshal(data, &config); err != nil {
			continue
		}

		configs = append(configs, &config)
	}

	return configs, nil
}

// isPartialFile checks if a filename is a partial config file
func isPartialFile(name string) bool {
	return len(name) > 13 && name[len(name)-13:] == ".partial.json"
}

// FormatPartialSummary formats a summary of the partial config
func FormatPartialSummary(config *PartialConfig) string {
	if config == nil {
		return "No partial config"
	}

	elapsed := time.Since(config.StartedAt).Round(time.Minute)

	summary := fmt.Sprintf("Type: %s\n", config.SetupType)
	if config.DeployTarget != "" {
		summary += fmt.Sprintf("Target: %s\n", config.DeployTarget)
	}
	summary += fmt.Sprintf("Started: %s ago\n", elapsed)
	summary += fmt.Sprintf("Progress: %d/%d steps\n", len(config.CompletedSteps), len(AllSteps())-1)
	summary += fmt.Sprintf("Last Step: %s\n", StepDescription(config.CurrentStep))
	if config.Error != "" {
		summary += fmt.Sprintf("Error: %s\n", config.Error)
	}

	return summary
}

// ResumeOption represents an option for resuming setup
type ResumeOption string

const (
	ResumeOptionContinue ResumeOption = "continue"
	ResumeOptionRestart  ResumeOption = "restart"
	ResumeOptionCancel   ResumeOption = "cancel"
)
