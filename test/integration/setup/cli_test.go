// Package setup provides integration tests for the bib setup command.
package setup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bib/internal/auth"
	"bib/internal/config"
	"bib/internal/discovery"
	"bib/internal/setup/partial"
	"bib/internal/tui"
)

// TestCLISetupDataFlow tests the CLI setup data flow
func TestCLISetupDataFlow(t *testing.T) {
	// Create setup data with defaults
	data := tui.DefaultSetupData()

	// Verify defaults
	if data.Host != "" {
		t.Error("host should be empty by default for CLI")
	}

	// Set identity
	data.Name = "Test User"
	data.Email = "test@example.com"

	// Set nodes
	data.SelectedNodes = append(data.SelectedNodes, tui.NodeSelection{
		Address:   "localhost:4000",
		Alias:     "local",
		IsDefault: true,
	})

	// Verify data is set
	if data.Name != "Test User" {
		t.Error("name not set correctly")
	}

	if len(data.SelectedNodes) != 1 {
		t.Error("nodes not set correctly")
	}

	// Convert to config
	cfg := data.ToBibConfig()

	if cfg == nil {
		t.Fatal("config is nil")
	}

	// Verify config
	if len(cfg.Connection.FavoriteNodes) != 1 {
		t.Error("favorite nodes not converted correctly")
	}
}

// TestIdentityKeyGeneration tests identity key generation for setup
func TestIdentityKeyGeneration(t *testing.T) {
	tmpDir := t.TempDir()

	// Generate key
	key, err := auth.GenerateIdentityKey()
	if err != nil {
		t.Fatalf("failed to generate identity key: %v", err)
	}

	// Get fingerprint
	fingerprint := key.Fingerprint()
	if fingerprint == "" {
		t.Error("fingerprint is empty")
	}

	// Save key
	keyPath := filepath.Join(tmpDir, "identity.pem")
	if err := key.Save(keyPath); err != nil {
		t.Fatalf("failed to save key: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Error("key file not created")
	}

	// Load key
	loadedKey, err := auth.LoadIdentityKey(keyPath)
	if err != nil {
		t.Fatalf("failed to load key: %v", err)
	}

	// Verify fingerprint matches
	if loadedKey.Fingerprint() != fingerprint {
		t.Error("fingerprint mismatch after load")
	}
}

// TestNodeDiscoveryMocked tests node discovery with mocked responses
func TestNodeDiscoveryMocked(t *testing.T) {
	opts := discovery.DefaultOptions()
	opts.Timeout = 2 * time.Second
	opts.EnableMDNS = false // Disable mDNS for faster test
	opts.EnableP2P = false  // Disable P2P for faster test

	d := discovery.New(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result := d.Discover(ctx)

	// Result should not be nil even if no nodes found
	if result == nil {
		t.Fatal("discovery result is nil")
	}

	// Check that nodes list exists (may be empty)
	_ = result.Nodes
}

// TestPartialConfigIntegration tests partial config save/load cycle
func TestPartialConfigIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	manager := partial.NewManager(tmpDir)

	// Create partial config
	cfg := partial.NewPartialConfig("cli")
	cfg.SetData("name", "Test User")
	cfg.SetData("email", "test@example.com")
	cfg.CompleteStep(partial.StepIdentity)

	// Save
	if err := manager.Save(cfg); err != nil {
		t.Fatalf("failed to save partial config: %v", err)
	}

	// Verify file exists
	if !manager.HasPartial("cli") {
		t.Error("partial config not saved")
	}

	// Load
	loaded, err := manager.Load("cli")
	if err != nil {
		t.Fatalf("failed to load partial config: %v", err)
	}

	// Verify data
	if loaded.GetString("name") != "Test User" {
		t.Error("name not preserved")
	}

	if loaded.GetString("email") != "test@example.com" {
		t.Error("email not preserved")
	}

	if !loaded.IsStepCompleted(partial.StepIdentity) {
		t.Error("step completion not preserved")
	}

	// Delete
	if err := manager.Delete("cli"); err != nil {
		t.Fatalf("failed to delete partial config: %v", err)
	}

	if manager.HasPartial("cli") {
		t.Error("partial config not deleted")
	}
}

// TestConfigGeneration tests config generation for CLI
func TestConfigGeneration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create setup data
	data := tui.DefaultSetupData()
	data.Name = "Test User"
	data.Email = "test@example.com"
	data.ServerAddr = "localhost:4000"
	data.BibDevConfirmed = true

	// Convert to config
	cfg := data.ToBibConfig()

	// Save config
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := config.SaveBib(cfg, configPath); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file not created")
	}

	// Load config - use LoadBib with empty path and set env
	os.Setenv("BIB_CONFIG", configPath)
	defer os.Unsetenv("BIB_CONFIG")

	loadedCfg, err := config.LoadBib(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify config
	if loadedCfg.Identity.Name != "Test User" {
		t.Errorf("name mismatch: got %s", loadedCfg.Identity.Name)
	}
}

// TestSetupDataValidation tests setup data validation
func TestSetupDataValidation(t *testing.T) {
	tests := []struct {
		name    string
		data    *tui.SetupData
		wantErr bool
	}{
		{
			name: "valid data",
			data: &tui.SetupData{
				Name:  "Test User",
				Email: "test@example.com",
			},
			wantErr: false,
		},
		{
			name: "empty name",
			data: &tui.SetupData{
				Name:  "",
				Email: "test@example.com",
			},
			wantErr: true,
		},
		{
			name: "invalid email",
			data: &tui.SetupData{
				Name:  "Test User",
				Email: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSetupData(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSetupData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// validateSetupData validates setup data
func validateSetupData(data *tui.SetupData) error {
	if data.Name == "" {
		return &validationError{"name is required"}
	}

	if data.Email == "" {
		return &validationError{"email is required"}
	}

	if !isValidEmail(data.Email) {
		return &validationError{"invalid email format"}
	}

	return nil
}

type validationError struct {
	msg string
}

func (e *validationError) Error() string {
	return e.msg
}

func isValidEmail(email string) bool {
	// Simple validation - just check for @
	for _, c := range email {
		if c == '@' {
			return true
		}
	}
	return false
}
