package setup

import (
	"testing"

	"bib/internal/deploy/local"
	"bib/internal/tui"
)

// TestDaemonSetupDataFlow tests the daemon setup data flow
func TestDaemonSetupDataFlow(t *testing.T) {
	// Create setup data with defaults
	data := tui.DefaultSetupData()

	// Set identity
	data.Name = "Test Node"
	data.Email = "node@example.com"

	// Set server config
	data.Host = "0.0.0.0"
	data.Port = 4000
	data.TLSEnabled = true
	data.LogLevel = "info"
	data.LogFormat = "json"

	// Set P2P mode
	data.P2PMode = "proxy"

	// Verify data is set
	if data.Host != "0.0.0.0" {
		t.Error("host not set correctly")
	}

	if data.Port != 4000 {
		t.Error("port not set correctly")
	}

	// Convert to config
	cfg := data.ToBibdConfig()

	if cfg == nil {
		t.Fatal("config is nil")
	}

	// Verify config
	if cfg.Server.Host != "0.0.0.0" {
		t.Error("server host not converted correctly")
	}

	if cfg.Server.Port != 4000 {
		t.Error("server port not converted correctly")
	}
}

// TestServiceConfigGeneration tests service configuration generation
func TestServiceConfigGeneration(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := local.DefaultServiceConfig()
	cfg.ConfigPath = tmpDir + "/config.yaml"
	cfg.WorkingDirectory = tmpDir

	installer := local.NewServiceInstaller(cfg)

	// Generate service content
	content, err := installer.Generate()
	if err != nil {
		t.Fatalf("failed to generate service: %v", err)
	}

	if content == "" {
		t.Error("service content is empty")
	}
}

// TestLocalDeploymentDetection tests local deployment detection
func TestLocalDeploymentDetection(t *testing.T) {
	// Get service type for current platform
	serviceType := local.DetectServiceType()

	// Should return a valid type
	validTypes := []local.ServiceType{
		local.ServiceTypeSystemd,
		local.ServiceTypeLaunchd,
		local.ServiceTypeWindows,
	}

	found := false
	for _, vt := range validTypes {
		if serviceType == vt {
			found = true
			break
		}
	}

	// Service type should be one of the valid types (or empty for unsupported platforms)
	if !found && serviceType != "" {
		t.Errorf("unexpected service type: %s", serviceType)
	}
}

// TestDaemonP2PModeValidation tests P2P mode validation
func TestDaemonP2PModeValidation(t *testing.T) {
	tests := []struct {
		mode        string
		storageType string
		wantErr     bool
	}{
		{"proxy", "sqlite", false},
		{"selective", "sqlite", false},
		{"full", "sqlite", true}, // SQLite not supported with full mode
		{"full", "postgres", false},
		{"invalid", "sqlite", true},
	}

	for _, tt := range tests {
		t.Run(tt.mode+"_"+tt.storageType, func(t *testing.T) {
			err := validateP2PMode(tt.mode, tt.storageType)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateP2PMode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// validateP2PMode validates P2P mode with storage type
func validateP2PMode(mode, storageType string) error {
	validModes := []string{"proxy", "selective", "full"}
	isValid := false
	for _, m := range validModes {
		if mode == m {
			isValid = true
			break
		}
	}

	if !isValid {
		return &localValidationError{"invalid P2P mode"}
	}

	// Full mode requires PostgreSQL
	if mode == "full" && storageType == "sqlite" {
		return &localValidationError{"full mode requires PostgreSQL"}
	}

	return nil
}

type localValidationError struct {
	msg string
}

func (e *localValidationError) Error() string {
	return e.msg
}
