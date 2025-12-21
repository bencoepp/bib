package local

import (
	"runtime"
	"strings"
	"testing"
)

func TestDefaultServiceConfig(t *testing.T) {
	config := DefaultServiceConfig()

	if config == nil {
		t.Fatal("config is nil")
	}

	if config.Name != "bibd" {
		t.Errorf("expected name 'bibd', got %q", config.Name)
	}

	if config.DisplayName != "bib Daemon" {
		t.Errorf("expected display name 'bib Daemon', got %q", config.DisplayName)
	}

	if config.RestartPolicy != "on-failure" {
		t.Errorf("expected restart policy 'on-failure', got %q", config.RestartPolicy)
	}

	if config.RestartDelaySec != 5 {
		t.Errorf("expected restart delay 5, got %d", config.RestartDelaySec)
	}
}

func TestNewServiceInstaller(t *testing.T) {
	t.Run("with config", func(t *testing.T) {
		config := &ServiceConfig{Name: "test"}
		installer := NewServiceInstaller(config)

		if installer.Config.Name != "test" {
			t.Error("config not set correctly")
		}
	})

	t.Run("nil config", func(t *testing.T) {
		installer := NewServiceInstaller(nil)

		if installer.Config == nil {
			t.Error("should use default config when nil")
		}
		if installer.Config.Name != "bibd" {
			t.Error("should have default name")
		}
	})
}

func TestDetectServiceType(t *testing.T) {
	serviceType := DetectServiceType()

	switch runtime.GOOS {
	case "linux":
		if serviceType != ServiceTypeSystemd {
			t.Errorf("expected systemd on Linux, got %q", serviceType)
		}
	case "darwin":
		if serviceType != ServiceTypeLaunchd {
			t.Errorf("expected launchd on macOS, got %q", serviceType)
		}
	case "windows":
		if serviceType != ServiceTypeWindows {
			t.Errorf("expected windows on Windows, got %q", serviceType)
		}
	}
}

func TestServiceType_Constants(t *testing.T) {
	tests := []struct {
		serviceType ServiceType
		expected    string
	}{
		{ServiceTypeSystemd, "systemd"},
		{ServiceTypeLaunchd, "launchd"},
		{ServiceTypeWindows, "windows"},
	}

	for _, tt := range tests {
		if string(tt.serviceType) != tt.expected {
			t.Errorf("expected %q, got %q", tt.expected, string(tt.serviceType))
		}
	}
}

func TestServiceInstaller_GenerateSystemd(t *testing.T) {
	config := &ServiceConfig{
		Name:             "bibd",
		DisplayName:      "bib Daemon",
		Description:      "bib distributed database daemon",
		ExecutablePath:   "/usr/local/bin/bibd",
		ConfigPath:       "/etc/bibd/config.yaml",
		WorkingDirectory: "/var/lib/bibd",
		User:             "bibd",
		Group:            "bibd",
		UserService:      false,
		RestartPolicy:    "on-failure",
		RestartDelaySec:  5,
		Environment:      map[string]string{"BIBD_LOG_LEVEL": "info"},
	}

	installer := NewServiceInstaller(config)
	content := installer.generateSystemd()

	// Check required sections
	if !strings.Contains(content, "[Unit]") {
		t.Error("missing [Unit] section")
	}
	if !strings.Contains(content, "[Service]") {
		t.Error("missing [Service] section")
	}
	if !strings.Contains(content, "[Install]") {
		t.Error("missing [Install] section")
	}

	// Check content
	if !strings.Contains(content, "Description=bib distributed database daemon") {
		t.Error("missing description")
	}
	if !strings.Contains(content, "ExecStart=/usr/local/bin/bibd serve --config /etc/bibd/config.yaml") {
		t.Error("missing ExecStart")
	}
	if !strings.Contains(content, "User=bibd") {
		t.Error("missing User")
	}
	if !strings.Contains(content, "Restart=on-failure") {
		t.Error("missing Restart policy")
	}
	if !strings.Contains(content, "Environment=BIBD_LOG_LEVEL=info") {
		t.Error("missing environment variable")
	}
	if !strings.Contains(content, "WantedBy=multi-user.target") {
		t.Error("missing WantedBy for system service")
	}
}

func TestServiceInstaller_GenerateSystemd_UserService(t *testing.T) {
	config := &ServiceConfig{
		Name:             "bibd",
		Description:      "bib daemon",
		ExecutablePath:   "/usr/local/bin/bibd",
		ConfigPath:       "/home/user/.config/bibd/config.yaml",
		WorkingDirectory: "/home/user/.config/bibd",
		UserService:      true,
		RestartPolicy:    "always",
		RestartDelaySec:  10,
	}

	installer := NewServiceInstaller(config)
	content := installer.generateSystemd()

	// Should NOT have User/Group for user service
	if strings.Contains(content, "User=") {
		t.Error("user service should not have User directive")
	}

	// Should have user target
	if !strings.Contains(content, "WantedBy=default.target") {
		t.Error("user service should want default.target")
	}

	// Check restart policy
	if !strings.Contains(content, "Restart=always") {
		t.Error("missing Restart=always")
	}
}

func TestServiceInstaller_GenerateLaunchd(t *testing.T) {
	config := &ServiceConfig{
		Name:             "bibd",
		ExecutablePath:   "/usr/local/bin/bibd",
		ConfigPath:       "/etc/bibd/config.yaml",
		WorkingDirectory: "/var/lib/bibd",
		UserService:      false,
		RestartPolicy:    "on-failure",
		RestartDelaySec:  5,
		Environment:      map[string]string{"BIBD_LOG_LEVEL": "debug"},
	}

	installer := NewServiceInstaller(config)
	content := installer.generateLaunchd()

	// Check XML structure
	if !strings.Contains(content, "<?xml version=") {
		t.Error("missing XML declaration")
	}
	if !strings.Contains(content, "<plist version=\"1.0\">") {
		t.Error("missing plist tag")
	}

	// Check content
	if !strings.Contains(content, "<key>Label</key>") {
		t.Error("missing Label key")
	}
	if !strings.Contains(content, "dev.bib.bibd") {
		t.Error("missing label value")
	}
	if !strings.Contains(content, "<key>ProgramArguments</key>") {
		t.Error("missing ProgramArguments")
	}
	if !strings.Contains(content, "/usr/local/bin/bibd") {
		t.Error("missing executable path")
	}
	if !strings.Contains(content, "<key>RunAtLoad</key>") {
		t.Error("missing RunAtLoad")
	}
	if !strings.Contains(content, "<key>KeepAlive</key>") {
		t.Error("missing KeepAlive")
	}
	if !strings.Contains(content, "<key>EnvironmentVariables</key>") {
		t.Error("missing EnvironmentVariables")
	}
	if !strings.Contains(content, "BIBD_LOG_LEVEL") {
		t.Error("missing environment variable")
	}
}

func TestServiceInstaller_GenerateWindowsPowerShell(t *testing.T) {
	config := &ServiceConfig{
		Name:             "bibd",
		DisplayName:      "bib Daemon",
		Description:      "bib distributed database daemon",
		ExecutablePath:   "C:\\Program Files\\bib\\bibd.exe",
		ConfigPath:       "C:\\ProgramData\\bib\\config.yaml",
		WorkingDirectory: "C:\\ProgramData\\bib",
	}

	installer := NewServiceInstaller(config)
	content := installer.generateWindowsPowerShell()

	// Check for PowerShell content
	if !strings.Contains(content, "# PowerShell script") {
		t.Error("missing PowerShell header")
	}
	if !strings.Contains(content, "nssm install") {
		t.Error("missing NSSM instructions")
	}
	if !strings.Contains(content, "New-Service") {
		t.Error("missing New-Service command")
	}
	if !strings.Contains(content, "Start-Service") {
		t.Error("missing Start-Service command")
	}
	if !strings.Contains(content, "bibd") {
		t.Error("missing service name")
	}
}

func TestServiceInstaller_GetServiceFilePath(t *testing.T) {
	t.Run("system service", func(t *testing.T) {
		config := &ServiceConfig{
			Name:        "bibd",
			UserService: false,
		}
		installer := NewServiceInstaller(config)
		path := installer.GetServiceFilePath()

		switch runtime.GOOS {
		case "linux":
			if !strings.Contains(path, "/etc/systemd/system") {
				t.Errorf("expected system path for Linux, got %q", path)
			}
		case "darwin":
			if !strings.Contains(path, "/Library/LaunchDaemons") {
				t.Errorf("expected LaunchDaemons path for macOS, got %q", path)
			}
		case "windows":
			if path != "" {
				t.Error("Windows should return empty path")
			}
		}
	})

	t.Run("user service", func(t *testing.T) {
		config := &ServiceConfig{
			Name:        "bibd",
			UserService: true,
		}
		installer := NewServiceInstaller(config)
		path := installer.GetServiceFilePath()

		switch runtime.GOOS {
		case "linux":
			if !strings.Contains(path, ".config/systemd/user") {
				t.Errorf("expected user path for Linux, got %q", path)
			}
		case "darwin":
			if !strings.Contains(path, "Library/LaunchAgents") {
				t.Errorf("expected LaunchAgents path for macOS, got %q", path)
			}
		}
	})
}

func TestServiceInstaller_Generate(t *testing.T) {
	config := DefaultServiceConfig()
	installer := NewServiceInstaller(config)

	content, err := installer.Generate()
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if content == "" {
		t.Error("generated content is empty")
	}

	// Content should be appropriate for the platform
	switch runtime.GOOS {
	case "linux":
		if !strings.Contains(content, "[Unit]") {
			t.Error("expected systemd format on Linux")
		}
	case "darwin":
		if !strings.Contains(content, "plist") {
			t.Error("expected plist format on macOS")
		}
	case "windows":
		if !strings.Contains(content, "PowerShell") {
			t.Error("expected PowerShell format on Windows")
		}
	}
}

func TestServiceInstaller_InstallInstructions(t *testing.T) {
	config := DefaultServiceConfig()
	installer := NewServiceInstaller(config)

	instructions := installer.InstallInstructions()

	if instructions == "" {
		t.Error("instructions are empty")
	}

	// Should contain installation steps
	if !strings.Contains(instructions, "1.") || !strings.Contains(instructions, "2.") {
		t.Error("missing numbered steps")
	}
}

func TestServiceConfig_Fields(t *testing.T) {
	config := ServiceConfig{
		Name:             "test",
		DisplayName:      "Test Service",
		Description:      "A test service",
		ExecutablePath:   "/usr/bin/test",
		ConfigPath:       "/etc/test/config.yaml",
		WorkingDirectory: "/var/lib/test",
		User:             "testuser",
		Group:            "testgroup",
		UserService:      true,
		Environment: map[string]string{
			"KEY1": "value1",
			"KEY2": "value2",
		},
		RestartPolicy:   "always",
		RestartDelaySec: 10,
	}

	if config.Name != "test" {
		t.Error("name mismatch")
	}
	if len(config.Environment) != 2 {
		t.Error("environment mismatch")
	}
	if config.Environment["KEY1"] != "value1" {
		t.Error("environment value mismatch")
	}
}
