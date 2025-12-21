package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// setTestHomeDir sets the home directory environment variables for testing.
// On Windows, it sets USERPROFILE; on Unix, it sets HOME.
// Returns a cleanup function to restore the original values.
func setTestHomeDir(t *testing.T, tempDir string) func() {
	t.Helper()
	if runtime.GOOS == "windows" {
		origUserProfile := os.Getenv("USERPROFILE")
		os.Setenv("USERPROFILE", tempDir)
		return func() { os.Setenv("USERPROFILE", origUserProfile) }
	}
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	return func() { os.Setenv("HOME", origHome) }
}

// ==================== Types Tests ====================

func TestDefaultBibConfig(t *testing.T) {
	cfg := DefaultBibConfig()

	if cfg == nil {
		t.Fatal("DefaultBibConfig returned nil")
	}

	// Log configuration
	if cfg.Log.Level != "info" {
		t.Errorf("expected log level 'info', got %q", cfg.Log.Level)
	}
	if cfg.Log.Format != "text" {
		t.Errorf("expected log format 'text', got %q", cfg.Log.Format)
	}
	if cfg.Log.Output != "stderr" {
		t.Errorf("expected log output 'stderr', got %q", cfg.Log.Output)
	}
	if cfg.Log.MaxSizeMB != 100 {
		t.Errorf("expected log max size 100, got %d", cfg.Log.MaxSizeMB)
	}
	if cfg.Log.MaxBackups != 3 {
		t.Errorf("expected log max backups 3, got %d", cfg.Log.MaxBackups)
	}
	if cfg.Log.MaxAgeDays != 28 {
		t.Errorf("expected log max age 28, got %d", cfg.Log.MaxAgeDays)
	}
	if cfg.Log.EnableCaller {
		t.Error("expected enable caller to be false")
	}
	if cfg.Log.AuditMaxAgeDays != 365 {
		t.Errorf("expected audit max age 365, got %d", cfg.Log.AuditMaxAgeDays)
	}
	if len(cfg.Log.RedactFields) == 0 {
		t.Error("expected redact fields to have default values")
	}

	// Output configuration
	if cfg.Output.Format != "table" {
		t.Errorf("expected output format 'table', got %q", cfg.Output.Format)
	}
	if !cfg.Output.Color {
		t.Error("expected output color to be true")
	}

	// Connection configuration
	if !cfg.Connection.AutoDetect {
		t.Error("expected auto detect to be true")
	}
}

func TestDefaultBibdConfig(t *testing.T) {
	cfg := DefaultBibdConfig()

	// Log configuration
	if cfg.Log.Level != "info" {
		t.Errorf("expected log level 'info', got %q", cfg.Log.Level)
	}
	if cfg.Log.Format != "pretty" {
		t.Errorf("expected log format 'pretty', got %q", cfg.Log.Format)
	}
	if cfg.Log.Output != "stdout" {
		t.Errorf("expected log output 'stdout', got %q", cfg.Log.Output)
	}
	if !cfg.Log.EnableCaller {
		t.Error("expected enable caller to be true")
	}

	// Server configuration
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected server host '0.0.0.0', got %q", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected server port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.PIDFile != "~/bibd.pid" {
		t.Errorf("expected PID file '~/bibd.pid', got %q", cfg.Server.PIDFile)
	}
	// Data dir path is platform-specific
	expectedDataDir := "~/.local/share/bibd"
	if runtime.GOOS == "windows" {
		expectedDataDir = "~/AppData/Local/bibd"
	}
	if cfg.Server.DataDir != expectedDataDir {
		t.Errorf("expected data dir %q, got %q", expectedDataDir, cfg.Server.DataDir)
	}
	if cfg.Server.TLS.Enabled {
		t.Error("expected TLS to be disabled")
	}
}

func TestLogConfigFields(t *testing.T) {
	cfg := LogConfig{
		Level:           "debug",
		Format:          "json",
		Output:          "stdout",
		FilePath:        "/var/log/test.log",
		MaxSizeMB:       50,
		MaxBackups:      5,
		MaxAgeDays:      14,
		EnableCaller:    true,
		NoColor:         true,
		AuditPath:       "/var/log/audit.log",
		AuditMaxAgeDays: 730,
		RedactFields:    []string{"password", "secret"},
	}

	if cfg.Level != "debug" {
		t.Errorf("expected level 'debug', got %q", cfg.Level)
	}
	if cfg.Format != "json" {
		t.Errorf("expected format 'json', got %q", cfg.Format)
	}
	if len(cfg.RedactFields) != 2 {
		t.Errorf("expected 2 redact fields, got %d", len(cfg.RedactFields))
	}
}

func TestIdentityConfigFields(t *testing.T) {
	cfg := IdentityConfig{
		Name:  "Test User",
		Email: "test@example.com",
		Key:   "/path/to/key",
	}

	if cfg.Name != "Test User" {
		t.Errorf("expected name 'Test User', got %q", cfg.Name)
	}
	if cfg.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got %q", cfg.Email)
	}
	if cfg.Key != "/path/to/key" {
		t.Errorf("expected key '/path/to/key', got %q", cfg.Key)
	}
}

func TestServerConfigFields(t *testing.T) {
	cfg := ServerConfig{
		Host:    "127.0.0.1",
		Port:    9090,
		PIDFile: "/tmp/test.pid",
		DataDir: "/tmp/data",
		TLS: TLSConfig{
			Enabled:  true,
			CertFile: "/path/to/cert.pem",
			KeyFile:  "/path/to/key.pem",
		},
	}

	if cfg.Host != "127.0.0.1" {
		t.Errorf("expected host '127.0.0.1', got %q", cfg.Host)
	}
	if cfg.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Port)
	}
	if !cfg.TLS.Enabled {
		t.Error("expected TLS to be enabled")
	}
	if cfg.TLS.CertFile != "/path/to/cert.pem" {
		t.Errorf("expected cert file '/path/to/cert.pem', got %q", cfg.TLS.CertFile)
	}
}

// ==================== Generator Tests ====================

func TestIsValidFormat(t *testing.T) {
	tests := []struct {
		format   string
		expected bool
	}{
		{"yaml", true},
		{"toml", true},
		{"json", true},
		{"xml", false},
		{"ini", false},
		{"", false},
		{"YAML", false}, // case sensitive
		{"JSON", false},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			result := isValidFormat(tt.format)
			if result != tt.expected {
				t.Errorf("isValidFormat(%q) = %v, expected %v", tt.format, result, tt.expected)
			}
		})
	}
}

func TestGenerateConfig_InvalidFormat(t *testing.T) {
	_, err := GenerateConfig(AppBib, "invalid")
	if err == nil {
		t.Error("expected error for invalid format")
	}
}

func TestGenerateConfig_UnknownApp(t *testing.T) {
	// Create a temp directory for the test
	tempDir := t.TempDir()
	cleanup := setTestHomeDir(t, tempDir)
	defer cleanup()

	_, err := GenerateConfig("unknownapp", "yaml")
	if err == nil {
		t.Error("expected error for unknown app")
	}
}

func TestGenerateConfig_BibApp(t *testing.T) {
	tempDir := t.TempDir()
	cleanup := setTestHomeDir(t, tempDir)
	defer cleanup()

	path, err := GenerateConfig(AppBib, "yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPath := filepath.Join(tempDir, ".config", "bib", "config.yaml")
	if path != expectedPath {
		t.Errorf("expected path %q, got %q", expectedPath, path)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("config file was not created")
	}
}

func TestGenerateConfig_BibdApp(t *testing.T) {
	tempDir := t.TempDir()
	cleanup := setTestHomeDir(t, tempDir)
	defer cleanup()

	path, err := GenerateConfig(AppBibd, "toml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPath := filepath.Join(tempDir, ".config", "bibd", "config.toml")
	if path != expectedPath {
		t.Errorf("expected path %q, got %q", expectedPath, path)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("config file was not created")
	}
}

func TestGenerateConfig_AlreadyExists(t *testing.T) {
	tempDir := t.TempDir()
	cleanup := setTestHomeDir(t, tempDir)
	defer cleanup()

	// Create the first config
	_, err := GenerateConfig(AppBib, "yaml")
	if err != nil {
		t.Fatalf("unexpected error creating first config: %v", err)
	}

	// Try to create again
	_, err = GenerateConfig(AppBib, "yaml")
	if err == nil {
		t.Error("expected error when config already exists")
	}
}

func TestGenerateConfigIfNotExists_NewConfig(t *testing.T) {
	tempDir := t.TempDir()
	cleanup := setTestHomeDir(t, tempDir)
	defer cleanup()

	path, created, err := GenerateConfigIfNotExists(AppBib, "yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !created {
		t.Error("expected created to be true for new config")
	}

	if path == "" {
		t.Error("expected non-empty path")
	}
}

func TestGenerateConfigIfNotExists_ExistingConfig(t *testing.T) {
	tempDir := t.TempDir()
	cleanup := setTestHomeDir(t, tempDir)
	defer cleanup()

	// Create initial config
	_, _, err := GenerateConfigIfNotExists(AppBib, "yaml")
	if err != nil {
		t.Fatalf("unexpected error creating initial config: %v", err)
	}

	// Try to create again
	path, created, err := GenerateConfigIfNotExists(AppBib, "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if created {
		t.Error("expected created to be false for existing config")
	}

	// Should return the existing YAML file, not create a JSON one
	if filepath.Ext(path) != ".yaml" {
		t.Errorf("expected .yaml extension, got %q", filepath.Ext(path))
	}
}

func TestSupportedFormats(t *testing.T) {
	expected := []string{"yaml", "toml", "json"}
	if len(SupportedFormats) != len(expected) {
		t.Errorf("expected %d formats, got %d", len(expected), len(SupportedFormats))
	}

	for i, format := range expected {
		if SupportedFormats[i] != format {
			t.Errorf("expected format %q at index %d, got %q", format, i, SupportedFormats[i])
		}
	}
}

// ==================== Secrets Tests ====================

func TestResolveSecretValue_PlainValue(t *testing.T) {
	value := "plain-value"
	resolved, err := resolveSecretValue(value)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != value {
		t.Errorf("expected %q, got %q", value, resolved)
	}
}

func TestResolveSecretValue_EnvPrefix(t *testing.T) {
	envName := "TEST_SECRET_VALUE"
	envValue := "my-secret-value"
	os.Setenv(envName, envValue)
	defer os.Unsetenv(envName)

	value := "env://" + envName
	resolved, err := resolveSecretValue(value)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != envValue {
		t.Errorf("expected %q, got %q", envValue, resolved)
	}
}

func TestResolveSecretValue_EnvPrefix_NotSet(t *testing.T) {
	value := "env://NON_EXISTENT_ENV_VAR_12345"
	_, err := resolveSecretValue(value)
	if err == nil {
		t.Error("expected error for non-existent env var")
	}
}

func TestResolveSecretValue_FilePrefix(t *testing.T) {
	// Create temp file with secret
	tempFile, err := os.CreateTemp("", "secret-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	secretContent := "  file-secret-value  \n"
	if _, err := tempFile.WriteString(secretContent); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tempFile.Close()

	value := "file://" + tempFile.Name()
	resolved, err := resolveSecretValue(value)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "file-secret-value" // trimmed
	if resolved != expected {
		t.Errorf("expected %q, got %q", expected, resolved)
	}
}

func TestResolveSecretValue_FilePrefix_NotFound(t *testing.T) {
	value := "file:///non/existent/path/secret.txt"
	_, err := resolveSecretValue(value)
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestResolveSecrets_StructWithSecrets(t *testing.T) {
	envName := "TEST_IDENTITY_KEY"
	envValue := "secret-key-value"
	os.Setenv(envName, envValue)
	defer os.Unsetenv(envName)

	cfg := &BibConfig{
		Identity: IdentityConfig{
			Key: "env://" + envName,
		},
	}

	err := resolveSecrets(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Identity.Key != envValue {
		t.Errorf("expected identity key %q, got %q", envValue, cfg.Identity.Key)
	}
}

func TestResolveSecrets_NestedStruct(t *testing.T) {
	envName := "TEST_NESTED_SECRET"
	envValue := "nested-secret-value"
	os.Setenv(envName, envValue)
	defer os.Unsetenv(envName)

	cfg := &BibdConfig{
		Identity: IdentityConfig{
			Key: "env://" + envName,
		},
		Server: ServerConfig{
			TLS: TLSConfig{
				CertFile: "plain-cert-path",
			},
		},
	}

	err := resolveSecrets(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Identity.Key != envValue {
		t.Errorf("expected identity key %q, got %q", envValue, cfg.Identity.Key)
	}
	if cfg.Server.TLS.CertFile != "plain-cert-path" {
		t.Errorf("expected cert file 'plain-cert-path', got %q", cfg.Server.TLS.CertFile)
	}
}

func TestResolveSecrets_NilPointer(t *testing.T) {
	var cfg *BibConfig
	err := resolveSecrets(cfg)
	if err != nil {
		t.Fatalf("unexpected error for nil pointer: %v", err)
	}
}

// ==================== Loader Tests ====================

func TestUserConfigDir(t *testing.T) {
	dir, err := UserConfigDir(AppBib)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "bib")
	if dir != expected {
		t.Errorf("expected %q, got %q", expected, dir)
	}
}

func TestUserConfigDir_Bibd(t *testing.T) {
	dir, err := UserConfigDir(AppBibd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config", "bibd")
	if dir != expected {
		t.Errorf("expected %q, got %q", expected, dir)
	}
}

func TestConfigSearchPaths(t *testing.T) {
	paths := configSearchPaths(AppBib)

	if len(paths) == 0 {
		t.Error("expected non-empty paths")
	}

	// On Unix, should contain /etc/bib; on Windows, this path doesn't exist
	if runtime.GOOS != "windows" {
		foundEtc := false
		for _, p := range paths {
			if p == "/etc/bib" {
				foundEtc = true
				break
			}
		}
		if !foundEtc {
			t.Error("expected /etc/bib in search paths")
		}
	}
}

func TestLoadBib_Defaults(t *testing.T) {
	// Check if a user config exists that would override defaults
	configDir, _ := UserConfigDir(AppBib)
	for _, ext := range SupportedFormats {
		path := filepath.Join(configDir, "config."+ext)
		if _, err := os.Stat(path); err == nil {
			t.Skipf("Skipping test because user config exists at %s", path)
		}
	}

	// Load with no config file - should use defaults
	cfg, err := LoadBib("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defaults := DefaultBibConfig()
	if cfg.Log.Level != defaults.Log.Level {
		t.Errorf("expected log level %q, got %q", defaults.Log.Level, cfg.Log.Level)
	}
	if cfg.Connection.AutoDetect != defaults.Connection.AutoDetect {
		t.Errorf("expected auto detect %v, got %v", defaults.Connection.AutoDetect, cfg.Connection.AutoDetect)
	}
}

func TestLoadBib_WithConfigFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `
log:
  level: debug
  format: json
connection:
  default_node: "custom-server:9090"
  favorite_nodes:
    - alias: "Custom Server"
      address: "custom-server:9090"
      default: true
identity:
  name: "Test User"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadBib(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Log.Level != "debug" {
		t.Errorf("expected log level 'debug', got %q", cfg.Log.Level)
	}
	if cfg.Log.Format != "json" {
		t.Errorf("expected log format 'json', got %q", cfg.Log.Format)
	}
	if cfg.Connection.DefaultNode != "custom-server:9090" {
		t.Errorf("expected default_node 'custom-server:9090', got %q", cfg.Connection.DefaultNode)
	}
	if cfg.Identity.Name != "Test User" {
		t.Errorf("expected identity name 'Test User', got %q", cfg.Identity.Name)
	}
}

func TestLoadBib_InvalidConfigFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Write invalid YAML
	if err := os.WriteFile(configPath, []byte("invalid: yaml: content: ["), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err := LoadBib(configPath)
	if err == nil {
		t.Error("expected error for invalid config file")
	}
}

func TestLoadBib_WithEnvVars(t *testing.T) {
	os.Setenv("BIB_LOG_LEVEL", "error")
	os.Setenv("BIB_CONNECTION_DEFAULT_NODE", "env-server:1234")
	defer func() {
		os.Unsetenv("BIB_LOG_LEVEL")
		os.Unsetenv("BIB_CONNECTION_DEFAULT_NODE")
	}()

	cfg, err := LoadBib("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Log.Level != "error" {
		t.Errorf("expected log level 'error' from env, got %q", cfg.Log.Level)
	}
	if cfg.Connection.DefaultNode != "env-server:1234" {
		t.Errorf("expected connection.default_node 'env-server:1234' from env, got %q", cfg.Connection.DefaultNode)
	}
}

func TestLoadBibd_Defaults(t *testing.T) {
	// Check if a user config exists that would override defaults
	configDir, _ := UserConfigDir(AppBibd)
	for _, ext := range SupportedFormats {
		path := filepath.Join(configDir, "config."+ext)
		if _, err := os.Stat(path); err == nil {
			t.Skipf("Skipping test because user config exists at %s", path)
		}
	}

	cfg, err := LoadBibd("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defaults := DefaultBibdConfig()
	if cfg.Log.Level != defaults.Log.Level {
		t.Errorf("expected log level %q, got %q", defaults.Log.Level, cfg.Log.Level)
	}
	if cfg.Server.Port != defaults.Server.Port {
		t.Errorf("expected server port %d, got %d", defaults.Server.Port, cfg.Server.Port)
	}
}

func TestLoadBibd_WithConfigFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `
log:
  level: warn
  format: pretty
server:
  host: "192.168.1.1"
  port: 9999
  tls:
    enabled: true
    cert_file: "/path/to/cert.pem"
    key_file: "/path/to/key.pem"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadBibd(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Log.Level != "warn" {
		t.Errorf("expected log level 'warn', got %q", cfg.Log.Level)
	}
	if cfg.Server.Host != "192.168.1.1" {
		t.Errorf("expected server host '192.168.1.1', got %q", cfg.Server.Host)
	}
	if cfg.Server.Port != 9999 {
		t.Errorf("expected server port 9999, got %d", cfg.Server.Port)
	}
	if !cfg.Server.TLS.Enabled {
		t.Error("expected TLS to be enabled")
	}
}

func TestLoadBibd_WithSecrets(t *testing.T) {
	os.Setenv("TEST_TLS_KEY_SECRET", "secret-key-content")
	defer os.Unsetenv("TEST_TLS_KEY_SECRET")

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `
identity:
  key: "env://TEST_TLS_KEY_SECRET"
server:
  port: 8080
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadBibd(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Identity.Key != "secret-key-content" {
		t.Errorf("expected resolved secret, got %q", cfg.Identity.Key)
	}
}

func TestNewViperFromConfig_Bib(t *testing.T) {
	cfg := &BibConfig{
		Log: LogConfig{
			Level:  "debug",
			Format: "json",
		},
		Connection: ConnectionConfig{
			DefaultNode: "test:8080",
		},
		Output: OutputConfig{
			Format: "table",
			Color:  false,
		},
	}

	v := NewViperFromConfig(AppBib, cfg)

	if v.GetString("log.level") != "debug" {
		t.Errorf("expected log.level 'debug', got %q", v.GetString("log.level"))
	}
	if v.GetString("connection.default_node") != "test:8080" {
		t.Errorf("expected connection.default_node 'test:8080', got %q", v.GetString("connection.default_node"))
	}
	if v.GetBool("output.color") != false {
		t.Error("expected output.color to be false")
	}
}

func TestNewViperFromConfig_Bibd(t *testing.T) {
	cfg := &BibdConfig{
		Log: LogConfig{
			Level: "error",
		},
		Server: ServerConfig{
			Host: "10.0.0.1",
			Port: 3000,
			TLS: TLSConfig{
				Enabled:  true,
				CertFile: "/certs/cert.pem",
			},
		},
	}

	v := NewViperFromConfig(AppBibd, cfg)

	if v.GetString("log.level") != "error" {
		t.Errorf("expected log.level 'error', got %q", v.GetString("log.level"))
	}
	if v.GetString("server.host") != "10.0.0.1" {
		t.Errorf("expected server.host '10.0.0.1', got %q", v.GetString("server.host"))
	}
	if v.GetInt("server.port") != 3000 {
		t.Errorf("expected server.port 3000, got %d", v.GetInt("server.port"))
	}
	if !v.GetBool("server.tls.enabled") {
		t.Error("expected server.tls.enabled to be true")
	}
}

func TestConfigFileUsed(t *testing.T) {
	// Just verify it doesn't panic and returns a string
	path := ConfigFileUsed(AppBib)
	_ = path // may be empty if no config file found
}

// ==================== Edge Cases ====================

func TestLoadBib_NonExistentConfigFile(t *testing.T) {
	_, err := LoadBib("/non/existent/path/config.yaml")
	if err == nil {
		t.Error("expected error for non-existent config file path")
	}
}

func TestLoadBib_SecretResolutionError(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	configContent := `
identity:
  key: "env://NON_EXISTENT_ENV_VAR_FOR_TEST"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err := LoadBib(configPath)
	if err == nil {
		t.Error("expected error for unresolvable secret")
	}
}

func TestGenerateConfig_AllFormats(t *testing.T) {
	for _, format := range SupportedFormats {
		t.Run(format, func(t *testing.T) {
			tempDir := t.TempDir()
			cleanup := setTestHomeDir(t, tempDir)
			defer cleanup()

			path, err := GenerateConfig(AppBib, format)
			if err != nil {
				t.Fatalf("unexpected error for format %s: %v", format, err)
			}

			expectedExt := "." + format
			if filepath.Ext(path) != expectedExt {
				t.Errorf("expected extension %q, got %q", expectedExt, filepath.Ext(path))
			}
		})
	}
}

func TestLoadBib_JSONFormat(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	configContent := `{
	"log": {
		"level": "warn",
		"format": "text"
	},
	"connection": {
		"default_node": "json-server:8080"
	}
}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadBib(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Log.Level != "warn" {
		t.Errorf("expected log level 'warn', got %q", cfg.Log.Level)
	}
	if cfg.Connection.DefaultNode != "json-server:8080" {
		t.Errorf("expected connection.default_node 'json-server:8080', got %q", cfg.Connection.DefaultNode)
	}
}

func TestLoadBib_TOMLFormat(t *testing.T) {

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	configContent := `[log]
level = "error"
format = "json"

[connection]
default_node = "toml-server:8080"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadBib(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Log.Level != "error" {
		t.Errorf("expected log level 'error', got %q", cfg.Log.Level)
	}
	if cfg.Connection.DefaultNode != "toml-server:8080" {
		t.Errorf("expected connection.default_node 'toml-server:8080', got %q", cfg.Connection.DefaultNode)
	}
}

// Benchmark tests
func BenchmarkLoadBib(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = LoadBib("")
	}
}

func BenchmarkDefaultBibConfig(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = DefaultBibConfig()
	}
}

func BenchmarkResolveSecrets(b *testing.B) {
	os.Setenv("BENCH_SECRET", "value")
	defer os.Unsetenv("BENCH_SECRET")

	for i := 0; i < b.N; i++ {
		cfg := &BibConfig{
			Identity: IdentityConfig{
				Key: "env://BENCH_SECRET",
			},
		}
		_ = resolveSecrets(cfg)
	}
}

// ==================== Multi-Node Configuration Tests ====================

func TestBibConfig_GetDefaultServerAddress(t *testing.T) {
	tests := []struct {
		name     string
		cfg      BibConfig
		expected string
	}{
		{
			name: "uses Connection.DefaultNode first",
			cfg: BibConfig{
				Connection: ConnectionConfig{
					DefaultNode: "default:4000",
				},
			},
			expected: "default:4000",
		},
		{
			name: "uses default FavoriteNode if no DefaultNode",
			cfg: BibConfig{
				Connection: ConnectionConfig{
					FavoriteNodes: []FavoriteNode{
						{Address: "node1:4000", Default: false},
						{Address: "node2:4000", Default: true},
					},
				},
			},
			expected: "node2:4000",
		},
		{
			name: "uses first FavoriteNode if no default",
			cfg: BibConfig{
				Connection: ConnectionConfig{
					FavoriteNodes: []FavoriteNode{
						{Address: "first:4000"},
						{Address: "second:4000"},
					},
				},
			},
			expected: "first:4000",
		},
		{
			name:     "returns empty if nothing configured",
			cfg:      BibConfig{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cfg.GetDefaultServerAddress()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestBibConfig_GetFavoriteNodes(t *testing.T) {
	t.Run("returns FavoriteNodes if configured", func(t *testing.T) {
		cfg := BibConfig{
			Connection: ConnectionConfig{
				FavoriteNodes: []FavoriteNode{
					{Alias: "node1", Address: "node1:4000"},
					{Alias: "node2", Address: "node2:4000"},
				},
			},
		}
		nodes := cfg.GetFavoriteNodes()
		if len(nodes) != 2 {
			t.Errorf("expected 2 nodes, got %d", len(nodes))
		}
	})

	t.Run("returns nil if nothing configured", func(t *testing.T) {
		cfg := BibConfig{}
		nodes := cfg.GetFavoriteNodes()
		if nodes != nil {
			t.Errorf("expected nil, got %v", nodes)
		}
	})
}

func TestBibConfig_HasBibDevNode(t *testing.T) {
	t.Run("returns true for bib.dev:4000 address", func(t *testing.T) {
		cfg := BibConfig{
			Connection: ConnectionConfig{
				FavoriteNodes: []FavoriteNode{
					{Address: "localhost:4000"},
					{Address: "bib.dev:4000"},
				},
			},
		}
		if !cfg.HasBibDevNode() {
			t.Error("expected true for bib.dev:4000")
		}
	})

	t.Run("returns true for public discovery method", func(t *testing.T) {
		cfg := BibConfig{
			Connection: ConnectionConfig{
				FavoriteNodes: []FavoriteNode{
					{Address: "some-address:4000", DiscoveryMethod: "public"},
				},
			},
		}
		if !cfg.HasBibDevNode() {
			t.Error("expected true for public discovery method")
		}
	})

	t.Run("returns false for no bib.dev node", func(t *testing.T) {
		cfg := BibConfig{
			Connection: ConnectionConfig{
				FavoriteNodes: []FavoriteNode{
					{Address: "localhost:4000"},
				},
			},
		}
		if cfg.HasBibDevNode() {
			t.Error("expected false for no bib.dev node")
		}
	})
}

func TestBibConfig_IsBibDevConfirmed(t *testing.T) {
	t.Run("returns true when confirmed", func(t *testing.T) {
		cfg := BibConfig{
			Connection: ConnectionConfig{
				BibDevConfirmed: true,
			},
		}
		if !cfg.IsBibDevConfirmed() {
			t.Error("expected true")
		}
	})

	t.Run("returns false when not confirmed", func(t *testing.T) {
		cfg := BibConfig{}
		if cfg.IsBibDevConfirmed() {
			t.Error("expected false")
		}
	})
}

func TestFavoriteNode_NewFields(t *testing.T) {
	node := FavoriteNode{
		ID:              "peer-id-123",
		Alias:           "My Node",
		Priority:        1,
		Address:         "localhost:4000",
		UnixSocket:      "/var/run/bibd.sock",
		Default:         true,
		DiscoveryMethod: "local",
	}

	if !node.Default {
		t.Error("expected Default to be true")
	}
	if node.DiscoveryMethod != "local" {
		t.Errorf("expected DiscoveryMethod 'local', got %q", node.DiscoveryMethod)
	}
}
