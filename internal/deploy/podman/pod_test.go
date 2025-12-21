package podman

import (
	"strings"
	"testing"
)

func TestDefaultPodConfig(t *testing.T) {
	config := DefaultPodConfig()

	if config == nil {
		t.Fatal("config is nil")
	}

	if config.PodName != "bibd" {
		t.Errorf("expected pod name 'bibd', got %q", config.PodName)
	}

	if config.BibdImage != "ghcr.io/bib-project/bibd" {
		t.Errorf("expected bibd image 'ghcr.io/bib-project/bibd', got %q", config.BibdImage)
	}

	if config.APIPort != 4000 {
		t.Errorf("expected API port 4000, got %d", config.APIPort)
	}

	if config.DeployStyle != "pod" {
		t.Errorf("expected deploy style 'pod', got %q", config.DeployStyle)
	}

	if !config.Rootless {
		t.Error("expected Rootless to be true by default")
	}
}

func TestNewPodGenerator(t *testing.T) {
	t.Run("with config", func(t *testing.T) {
		config := &PodConfig{PodName: "test"}
		generator := NewPodGenerator(config)

		if generator.Config.PodName != "test" {
			t.Error("config not set correctly")
		}
	})

	t.Run("nil config", func(t *testing.T) {
		generator := NewPodGenerator(nil)

		if generator.Config == nil {
			t.Error("should use default config when nil")
		}
		if generator.Config.PodName != "bibd" {
			t.Error("should have default pod name")
		}
	})
}

func TestPodGenerator_generatePodYaml(t *testing.T) {
	config := DefaultPodConfig()
	config.Name = "Test Node"
	config.Email = "test@example.com"
	config.StorageBackend = "sqlite"
	config.P2PEnabled = true
	config.OutputDir = "/home/user/bibd"

	generator := NewPodGenerator(config)
	podYaml, err := generator.generatePodYaml()

	if err != nil {
		t.Fatalf("generatePodYaml failed: %v", err)
	}

	// Check for required elements
	checks := []string{
		"apiVersion: v1",
		"kind: Pod",
		"name: bibd",
		"ghcr.io/bib-project/bibd:latest",
		"containerPort: 4000",
		"containerPort: 4001",
		"containerPort: 9090",
		"/etc/bibd",
		"/var/lib/bibd",
	}

	for _, check := range checks {
		if !strings.Contains(podYaml, check) {
			t.Errorf("pod yaml missing %q", check)
		}
	}
}

func TestPodGenerator_generatePodYaml_Postgres(t *testing.T) {
	config := DefaultPodConfig()
	config.StorageBackend = "postgres"
	config.PostgresPassword = "testpassword"
	config.OutputDir = "/home/user/bibd"

	generator := NewPodGenerator(config)
	podYaml, err := generator.generatePodYaml()

	if err != nil {
		t.Fatalf("generatePodYaml failed: %v", err)
	}

	// Check for postgres container
	checks := []string{
		"name: postgres",
		"POSTGRES_DB",
		"POSTGRES_USER",
		"POSTGRES_PASSWORD",
		"postgres-data",
	}

	for _, check := range checks {
		if !strings.Contains(podYaml, check) {
			t.Errorf("pod yaml missing %q", check)
		}
	}
}

func TestPodGenerator_generateCompose(t *testing.T) {
	config := DefaultPodConfig()
	config.DeployStyle = "compose"
	config.Name = "Test Node"
	config.Email = "test@example.com"
	config.StorageBackend = "sqlite"
	config.P2PEnabled = true

	generator := NewPodGenerator(config)
	compose, err := generator.generateCompose()

	if err != nil {
		t.Fatalf("generateCompose failed: %v", err)
	}

	// Check for required elements
	checks := []string{
		"services:",
		"bibd:",
		"ghcr.io/bib-project/bibd:latest",
		"4000:4000",
		"4001:4001",
		"volumes:",
		"bibd-data:",
	}

	for _, check := range checks {
		if !strings.Contains(compose, check) {
			t.Errorf("compose missing %q", check)
		}
	}
}

func TestPodGenerator_generateCompose_Rootless(t *testing.T) {
	config := DefaultPodConfig()
	config.DeployStyle = "compose"
	config.Rootless = true

	generator := NewPodGenerator(config)
	compose, err := generator.generateCompose()

	if err != nil {
		t.Fatalf("generateCompose failed: %v", err)
	}

	// Check for rootless options
	if !strings.Contains(compose, "userns_mode: keep-id") {
		t.Error("compose missing userns_mode for rootless")
	}

	// Check for SELinux labels
	if !strings.Contains(compose, ":Z") {
		t.Error("compose missing SELinux labels (:Z)")
	}
}

func TestPodGenerator_generateEnvFile(t *testing.T) {
	config := DefaultPodConfig()
	config.StorageBackend = "postgres"
	config.PostgresPassword = "supersecret"
	config.PostgresDatabase = "testdb"
	config.PostgresUser = "testuser"

	generator := NewPodGenerator(config)
	envFile := generator.generateEnvFile()

	checks := []string{
		"BIBD_API_PORT=4000",
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

func TestPodGenerator_generateEnvFile_PortOffset(t *testing.T) {
	config := DefaultPodConfig()
	config.PortOffset = 8000

	generator := NewPodGenerator(config)
	envFile := generator.generateEnvFile()

	// API port should be 4000 + 8000 = 12000
	if !strings.Contains(envFile, "BIBD_API_PORT=12000") {
		t.Error("env file should have offset port 12000")
	}
}

func TestPodGenerator_generateConfigYaml(t *testing.T) {
	config := DefaultPodConfig()
	config.Name = "Test Node"
	config.Email = "test@example.com"
	config.P2PEnabled = true
	config.P2PMode = "selective"
	config.StorageBackend = "sqlite"

	generator := NewPodGenerator(config)
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

func TestPodGenerator_generateStartScript(t *testing.T) {
	t.Run("pod style", func(t *testing.T) {
		config := DefaultPodConfig()
		config.DeployStyle = "pod"

		generator := NewPodGenerator(config)
		script := generator.generateStartScript()

		if !strings.Contains(script, "#!/bin/bash") {
			t.Error("script missing shebang")
		}
		if !strings.Contains(script, "podman kube play pod.yaml") {
			t.Error("script missing podman kube play command")
		}
	})

	t.Run("compose style", func(t *testing.T) {
		config := DefaultPodConfig()
		config.DeployStyle = "compose"

		generator := NewPodGenerator(config)
		script := generator.generateStartScript()

		if !strings.Contains(script, "podman-compose") {
			t.Error("script missing podman-compose")
		}
		if !strings.Contains(script, "podman compose") {
			t.Error("script missing podman compose fallback")
		}
	})
}

func TestPodGenerator_generateStopScript(t *testing.T) {
	t.Run("pod style", func(t *testing.T) {
		config := DefaultPodConfig()
		config.DeployStyle = "pod"

		generator := NewPodGenerator(config)
		script := generator.generateStopScript()

		if !strings.Contains(script, "podman kube down") {
			t.Error("script missing podman kube down command")
		}
	})

	t.Run("compose style", func(t *testing.T) {
		config := DefaultPodConfig()
		config.DeployStyle = "compose"

		generator := NewPodGenerator(config)
		script := generator.generateStopScript()

		if !strings.Contains(script, "down") {
			t.Error("script missing down command")
		}
	})
}

func TestPodGenerator_generateStatusScript(t *testing.T) {
	config := DefaultPodConfig()
	config.PodName = "testpod"
	config.DeployStyle = "pod"

	generator := NewPodGenerator(config)
	script := generator.generateStatusScript()

	if !strings.Contains(script, "#!/bin/bash") {
		t.Error("script missing shebang")
	}
	if !strings.Contains(script, "testpod") {
		t.Error("script missing pod name")
	}
}

func TestPodGenerator_Generate(t *testing.T) {
	t.Run("pod style", func(t *testing.T) {
		config := DefaultPodConfig()
		config.DeployStyle = "pod"
		config.Name = "Test"
		config.Email = "test@example.com"
		config.OutputDir = "/home/user/bibd"

		generator := NewPodGenerator(config)
		files, err := generator.Generate()

		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}

		expectedFiles := []string{
			"pod.yaml",
			".env",
			"config/config.yaml",
			"start.sh",
			"stop.sh",
			"status.sh",
		}

		for _, f := range expectedFiles {
			if _, ok := files.Files[f]; !ok {
				t.Errorf("missing file %q", f)
			}
		}
	})

	t.Run("compose style", func(t *testing.T) {
		config := DefaultPodConfig()
		config.DeployStyle = "compose"
		config.Name = "Test"
		config.Email = "test@example.com"

		generator := NewPodGenerator(config)
		files, err := generator.Generate()

		if err != nil {
			t.Fatalf("Generate failed: %v", err)
		}

		if _, ok := files.Files["podman-compose.yaml"]; !ok {
			t.Error("missing podman-compose.yaml")
		}
	})
}

func TestPodGenerator_Generate_PasswordGeneration(t *testing.T) {
	config := DefaultPodConfig()
	config.StorageBackend = "postgres"
	config.PostgresPassword = "" // Should be auto-generated

	generator := NewPodGenerator(config)
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

func TestPodGenerator_FormatStartInstructions(t *testing.T) {
	config := DefaultPodConfig()
	config.OutputDir = "/path/to/output"
	config.DeployStyle = "pod"

	generator := NewPodGenerator(config)
	instructions := generator.FormatStartInstructions(nil)

	checks := []string{
		"Podman Deployment",
		"/path/to/output",
		"./start.sh",
		"./stop.sh",
		"./status.sh",
		"podman kube",
	}

	for _, check := range checks {
		if !strings.Contains(instructions, check) {
			t.Errorf("instructions missing %q", check)
		}
	}
}

func TestPodConfig_Fields(t *testing.T) {
	config := PodConfig{
		PodName:              "test-pod",
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
		Rootless:             false,
		PortOffset:           1000,
		TLSEnabled:           true,
		UsePublicBootstrap:   false,
		CustomBootstrapPeers: []string{"/ip4/1.2.3.4/tcp/4001/p2p/Qm..."},
		Name:                 "My Node",
		Email:                "node@example.com",
		OutputDir:            "/custom/output",
		DeployStyle:          "compose",
		ExtraEnv: map[string]string{
			"CUSTOM_VAR": "value",
		},
	}

	if config.PodName != "test-pod" {
		t.Error("PodName mismatch")
	}
	if config.P2PMode != "full" {
		t.Error("P2PMode mismatch")
	}
	if config.PortOffset != 1000 {
		t.Error("PortOffset mismatch")
	}
	if len(config.CustomBootstrapPeers) != 1 {
		t.Error("CustomBootstrapPeers mismatch")
	}
	if config.ExtraEnv["CUSTOM_VAR"] != "value" {
		t.Error("ExtraEnv mismatch")
	}
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
