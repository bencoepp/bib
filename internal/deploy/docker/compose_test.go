package docker

import (
	"strings"
	"testing"
)

func TestDefaultComposeConfig(t *testing.T) {
	config := DefaultComposeConfig()

	if config == nil {
		t.Fatal("config is nil")
	}

	if config.ProjectName != "bibd" {
		t.Errorf("expected project name 'bibd', got %q", config.ProjectName)
	}

	if config.BibdImage != "ghcr.io/bib-project/bibd" {
		t.Errorf("expected bibd image 'ghcr.io/bib-project/bibd', got %q", config.BibdImage)
	}

	if config.APIPort != 4000 {
		t.Errorf("expected API port 4000, got %d", config.APIPort)
	}

	if config.StorageBackend != "sqlite" {
		t.Errorf("expected storage backend 'sqlite', got %q", config.StorageBackend)
	}
}

func TestNewComposeGenerator(t *testing.T) {
	t.Run("with config", func(t *testing.T) {
		config := &ComposeConfig{ProjectName: "test"}
		generator := NewComposeGenerator(config)

		if generator.Config.ProjectName != "test" {
			t.Error("config not set correctly")
		}
	})

	t.Run("nil config", func(t *testing.T) {
		generator := NewComposeGenerator(nil)

		if generator.Config == nil {
			t.Error("should use default config when nil")
		}
		if generator.Config.ProjectName != "bibd" {
			t.Error("should have default project name")
		}
	})
}

func TestGeneratePassword(t *testing.T) {
	tests := []struct {
		length int
	}{
		{16},
		{32},
		{64},
	}

	for _, tt := range tests {
		password := GeneratePassword(tt.length)
		if len(password) != tt.length {
			t.Errorf("expected password length %d, got %d", tt.length, len(password))
		}
	}

	// Passwords should be different
	p1 := GeneratePassword(32)
	p2 := GeneratePassword(32)
	if p1 == p2 {
		t.Error("generated passwords should be different")
	}
}

func TestComposeGenerator_generateCompose_SQLite(t *testing.T) {
	config := DefaultComposeConfig()
	config.StorageBackend = "sqlite"
	config.P2PEnabled = true

	generator := NewComposeGenerator(config)
	compose, err := generator.generateCompose()

	if err != nil {
		t.Fatalf("generateCompose failed: %v", err)
	}

	// Check for required elements
	checks := []string{
		"services:",
		"bibd:",
		"image: ghcr.io/bib-project/bibd:latest",
		"4000}:4000", // API port with env var
		"4001}:4001", // P2P port with env var
		"volumes:",
		"bibd-data:",
		"bibd-sqlite:",
		"bibd-network:",
	}

	for _, check := range checks {
		if !strings.Contains(compose, check) {
			t.Errorf("compose missing %q", check)
		}
	}

	// Should NOT contain postgres
	if strings.Contains(compose, "postgres:") {
		t.Error("SQLite config should not include postgres service")
	}
}

func TestComposeGenerator_generateCompose_Postgres(t *testing.T) {
	config := DefaultComposeConfig()
	config.StorageBackend = "postgres"
	config.PostgresPassword = "testpassword"
	config.P2PEnabled = true

	generator := NewComposeGenerator(config)
	compose, err := generator.generateCompose()

	if err != nil {
		t.Fatalf("generateCompose failed: %v", err)
	}

	// Check for required elements
	checks := []string{
		"services:",
		"bibd:",
		"postgres:",
		"image: postgres:16-alpine",
		"POSTGRES_DB",
		"POSTGRES_USER",
		"POSTGRES_PASSWORD",
		"depends_on:",
		"postgres-data:",
		"healthcheck:",
		"pg_isready",
	}

	for _, check := range checks {
		if !strings.Contains(compose, check) {
			t.Errorf("compose missing %q", check)
		}
	}
}

func TestComposeGenerator_generateEnvFile(t *testing.T) {
	config := DefaultComposeConfig()
	config.StorageBackend = "postgres"
	config.PostgresPassword = "supersecret"
	config.PostgresDatabase = "testdb"
	config.PostgresUser = "testuser"
	config.APIPort = 8000

	generator := NewComposeGenerator(config)
	envFile, err := generator.generateEnvFile()

	if err != nil {
		t.Fatalf("generateEnvFile failed: %v", err)
	}

	checks := []string{
		"BIBD_API_PORT=8000",
		"POSTGRES_DB=testdb",
		"POSTGRES_USER=testuser",
		"POSTGRES_PASSWORD=supersecret",
	}

	for _, check := range checks {
		if !strings.Contains(envFile, check) {
			t.Errorf("env file missing %q", check)
		}
	}
}

func TestComposeGenerator_generateConfigYaml(t *testing.T) {
	config := DefaultComposeConfig()
	config.Name = "Test Node"
	config.Email = "test@example.com"
	config.P2PEnabled = true
	config.P2PMode = "selective"
	config.StorageBackend = "sqlite"

	generator := NewComposeGenerator(config)
	configYaml, err := generator.generateConfigYaml()

	if err != nil {
		t.Fatalf("generateConfigYaml failed: %v", err)
	}

	checks := []string{
		"name: \"Test Node\"",
		"email: \"test@example.com\"",
		"enabled: true",
		"mode: selective",
		"backend: sqlite",
	}

	for _, check := range checks {
		if !strings.Contains(configYaml, check) {
			t.Errorf("config yaml missing %q", check)
		}
	}
}

func TestComposeGenerator_Generate(t *testing.T) {
	config := DefaultComposeConfig()
	config.Name = "Test"
	config.Email = "test@example.com"

	generator := NewComposeGenerator(config)
	files, err := generator.Generate()

	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if files == nil {
		t.Fatal("files is nil")
	}

	// Check all expected files are generated
	expectedFiles := []string{
		"docker-compose.yaml",
		".env",
		"config/config.yaml",
	}

	for _, f := range expectedFiles {
		if _, ok := files.Files[f]; !ok {
			t.Errorf("missing file %q", f)
		}
	}
}

func TestComposeGenerator_Generate_PostgresPasswordGeneration(t *testing.T) {
	config := DefaultComposeConfig()
	config.StorageBackend = "postgres"
	config.PostgresPassword = "" // Empty - should be auto-generated

	generator := NewComposeGenerator(config)
	files, err := generator.Generate()

	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Check that password was generated
	envFile := files.Files[".env"]
	if !strings.Contains(envFile, "POSTGRES_PASSWORD=") {
		t.Error("env file missing POSTGRES_PASSWORD")
	}

	// Password should not be empty
	if strings.Contains(envFile, "POSTGRES_PASSWORD=\n") {
		t.Error("POSTGRES_PASSWORD should not be empty")
	}
}

func TestComposeGenerator_GetComposeUpCommand(t *testing.T) {
	generator := NewComposeGenerator(nil)

	t.Run("docker compose plugin", func(t *testing.T) {
		info := &DockerInfo{ComposeCommand: "docker compose"}
		cmd := generator.GetComposeUpCommand(info)

		expected := []string{"docker", "compose", "up", "-d"}
		if len(cmd) != len(expected) {
			t.Errorf("GetComposeUpCommand() = %v, expected %v", cmd, expected)
			return
		}
		for i := range cmd {
			if cmd[i] != expected[i] {
				t.Errorf("GetComposeUpCommand()[%d] = %q, expected %q", i, cmd[i], expected[i])
			}
		}
	})

	t.Run("docker-compose standalone", func(t *testing.T) {
		info := &DockerInfo{ComposeCommand: "docker-compose"}
		cmd := generator.GetComposeUpCommand(info)

		expected := []string{"docker-compose", "up", "-d"}
		if len(cmd) != len(expected) {
			t.Errorf("GetComposeUpCommand() = %v, expected %v", cmd, expected)
			return
		}
		for i := range cmd {
			if cmd[i] != expected[i] {
				t.Errorf("GetComposeUpCommand()[%d] = %q, expected %q", i, cmd[i], expected[i])
			}
		}
	})
}

func TestComposeGenerator_FormatStartInstructions(t *testing.T) {
	config := DefaultComposeConfig()
	config.OutputDir = "/path/to/output"

	generator := NewComposeGenerator(config)
	info := &DockerInfo{ComposeCommand: "docker compose"}

	instructions := generator.FormatStartInstructions(info)

	checks := []string{
		"Docker Compose Deployment",
		"/path/to/output",
		"docker compose up -d",
		"docker compose logs -f",
		"docker compose down",
		"docker compose ps",
	}

	for _, check := range checks {
		if !strings.Contains(instructions, check) {
			t.Errorf("instructions missing %q", check)
		}
	}
}

func TestComposeConfig_Fields(t *testing.T) {
	config := ComposeConfig{
		ProjectName:          "test-project",
		BibdImage:            "custom/bibd",
		BibdTag:              "v1.0.0",
		P2PEnabled:           true,
		P2PMode:              "full",
		StorageBackend:       "postgres",
		PostgresImage:        "postgres",
		PostgresTag:          "15",
		PostgresDatabase:     "mydb",
		PostgresUser:         "myuser",
		PostgresPassword:     "mypass",
		APIPort:              8080,
		P2PPort:              8081,
		MetricsPort:          9999,
		TLSEnabled:           true,
		UsePublicBootstrap:   false,
		CustomBootstrapPeers: []string{"/ip4/1.2.3.4/tcp/4001/p2p/Qm..."},
		Name:                 "My Node",
		Email:                "node@example.com",
		OutputDir:            "/custom/output",
		ExtraEnv: map[string]string{
			"CUSTOM_VAR": "value",
		},
	}

	if config.ProjectName != "test-project" {
		t.Error("ProjectName mismatch")
	}
	if config.P2PMode != "full" {
		t.Error("P2PMode mismatch")
	}
	if len(config.CustomBootstrapPeers) != 1 {
		t.Error("CustomBootstrapPeers mismatch")
	}
	if config.ExtraEnv["CUSTOM_VAR"] != "value" {
		t.Error("ExtraEnv mismatch")
	}
}
