// Package local provides local deployment utilities for bibd.
package local

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

// ServiceType represents the type of service to install
type ServiceType string

const (
	ServiceTypeSystemd ServiceType = "systemd"
	ServiceTypeLaunchd ServiceType = "launchd"
	ServiceTypeWindows ServiceType = "windows"
)

// ServiceConfig contains configuration for service installation
type ServiceConfig struct {
	// Name is the service name
	Name string

	// DisplayName is the human-readable name
	DisplayName string

	// Description is the service description
	Description string

	// ExecutablePath is the path to the bibd executable
	ExecutablePath string

	// ConfigPath is the path to the configuration file
	ConfigPath string

	// WorkingDirectory is the working directory for the service
	WorkingDirectory string

	// User is the user to run the service as (Linux/macOS)
	User string

	// Group is the group to run the service as (Linux)
	Group string

	// UserService indicates if this is a user-level service (vs system)
	UserService bool

	// Environment variables to set
	Environment map[string]string

	// RestartPolicy is the restart policy (always, on-failure, never)
	RestartPolicy string

	// RestartDelaySec is the delay before restart in seconds
	RestartDelaySec int
}

// DefaultServiceConfig returns a default service configuration
func DefaultServiceConfig() *ServiceConfig {
	execPath, _ := os.Executable()
	if execPath == "" {
		execPath = "/usr/local/bin/bibd"
	}

	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".config", "bibd", "config.yaml")

	currentUser, _ := user.Current()
	username := "bibd"
	if currentUser != nil {
		username = currentUser.Username
	}

	return &ServiceConfig{
		Name:             "bibd",
		DisplayName:      "bib Daemon",
		Description:      "bib distributed database daemon",
		ExecutablePath:   execPath,
		ConfigPath:       configPath,
		WorkingDirectory: filepath.Dir(configPath),
		User:             username,
		Group:            username,
		UserService:      false,
		Environment:      make(map[string]string),
		RestartPolicy:    "on-failure",
		RestartDelaySec:  5,
	}
}

// ServiceInstaller provides methods to install and manage services
type ServiceInstaller struct {
	Config *ServiceConfig
}

// NewServiceInstaller creates a new service installer
func NewServiceInstaller(config *ServiceConfig) *ServiceInstaller {
	if config == nil {
		config = DefaultServiceConfig()
	}
	return &ServiceInstaller{Config: config}
}

// DetectServiceType detects the appropriate service type for this system
func DetectServiceType() ServiceType {
	switch runtime.GOOS {
	case "linux":
		// Check for systemd
		if _, err := os.Stat("/run/systemd/system"); err == nil {
			return ServiceTypeSystemd
		}
		// Fallback to systemd anyway on Linux
		return ServiceTypeSystemd
	case "darwin":
		return ServiceTypeLaunchd
	case "windows":
		return ServiceTypeWindows
	default:
		return ServiceTypeSystemd
	}
}

// GetServiceFilePath returns the path where the service file should be installed
func (s *ServiceInstaller) GetServiceFilePath() string {
	serviceType := DetectServiceType()

	switch serviceType {
	case ServiceTypeSystemd:
		if s.Config.UserService {
			homeDir, _ := os.UserHomeDir()
			return filepath.Join(homeDir, ".config", "systemd", "user", s.Config.Name+".service")
		}
		return filepath.Join("/etc/systemd/system", s.Config.Name+".service")

	case ServiceTypeLaunchd:
		if s.Config.UserService {
			homeDir, _ := os.UserHomeDir()
			return filepath.Join(homeDir, "Library", "LaunchAgents", "dev.bib."+s.Config.Name+".plist")
		}
		return filepath.Join("/Library/LaunchDaemons", "dev.bib."+s.Config.Name+".plist")

	case ServiceTypeWindows:
		// Windows services don't use file paths in the same way
		return ""

	default:
		return ""
	}
}

// Generate generates the service file content
func (s *ServiceInstaller) Generate() (string, error) {
	serviceType := DetectServiceType()

	switch serviceType {
	case ServiceTypeSystemd:
		return s.generateSystemd(), nil
	case ServiceTypeLaunchd:
		return s.generateLaunchd(), nil
	case ServiceTypeWindows:
		return s.generateWindowsPowerShell(), nil
	default:
		return "", fmt.Errorf("unsupported service type: %s", serviceType)
	}
}

// generateSystemd generates a systemd service file
func (s *ServiceInstaller) generateSystemd() string {
	var sb strings.Builder

	sb.WriteString("[Unit]\n")
	sb.WriteString(fmt.Sprintf("Description=%s\n", s.Config.Description))
	sb.WriteString("Documentation=https://github.com/bib-project/bib\n")
	sb.WriteString("After=network-online.target\n")
	sb.WriteString("Wants=network-online.target\n")
	sb.WriteString("\n")

	sb.WriteString("[Service]\n")
	sb.WriteString("Type=simple\n")
	sb.WriteString(fmt.Sprintf("ExecStart=%s serve --config %s\n", s.Config.ExecutablePath, s.Config.ConfigPath))
	sb.WriteString(fmt.Sprintf("WorkingDirectory=%s\n", s.Config.WorkingDirectory))

	if !s.Config.UserService {
		sb.WriteString(fmt.Sprintf("User=%s\n", s.Config.User))
		sb.WriteString(fmt.Sprintf("Group=%s\n", s.Config.Group))
	}

	// Restart policy
	switch s.Config.RestartPolicy {
	case "always":
		sb.WriteString("Restart=always\n")
	case "on-failure":
		sb.WriteString("Restart=on-failure\n")
	case "never":
		sb.WriteString("Restart=no\n")
	default:
		sb.WriteString("Restart=on-failure\n")
	}
	sb.WriteString(fmt.Sprintf("RestartSec=%d\n", s.Config.RestartDelaySec))

	// Environment variables
	for key, value := range s.Config.Environment {
		sb.WriteString(fmt.Sprintf("Environment=%s=%s\n", key, value))
	}

	// Security hardening
	sb.WriteString("\n# Security hardening\n")
	sb.WriteString("NoNewPrivileges=true\n")
	sb.WriteString("ProtectSystem=strict\n")
	sb.WriteString("ProtectHome=read-only\n")
	sb.WriteString("PrivateTmp=true\n")
	sb.WriteString("PrivateDevices=true\n")
	sb.WriteString("ProtectKernelTunables=true\n")
	sb.WriteString("ProtectKernelModules=true\n")
	sb.WriteString("ProtectControlGroups=true\n")

	// Allow writing to config and data directories
	sb.WriteString(fmt.Sprintf("ReadWritePaths=%s\n", s.Config.WorkingDirectory))

	sb.WriteString("\n")
	sb.WriteString("[Install]\n")
	if s.Config.UserService {
		sb.WriteString("WantedBy=default.target\n")
	} else {
		sb.WriteString("WantedBy=multi-user.target\n")
	}

	return sb.String()
}

// generateLaunchd generates a launchd plist file
func (s *ServiceInstaller) generateLaunchd() string {
	var sb strings.Builder

	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString("\n")
	sb.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">`)
	sb.WriteString("\n")
	sb.WriteString(`<plist version="1.0">`)
	sb.WriteString("\n<dict>\n")

	// Label
	sb.WriteString(fmt.Sprintf("    <key>Label</key>\n    <string>dev.bib.%s</string>\n", s.Config.Name))

	// Program arguments
	sb.WriteString("    <key>ProgramArguments</key>\n    <array>\n")
	sb.WriteString(fmt.Sprintf("        <string>%s</string>\n", s.Config.ExecutablePath))
	sb.WriteString("        <string>serve</string>\n")
	sb.WriteString("        <string>--config</string>\n")
	sb.WriteString(fmt.Sprintf("        <string>%s</string>\n", s.Config.ConfigPath))
	sb.WriteString("    </array>\n")

	// Working directory
	sb.WriteString(fmt.Sprintf("    <key>WorkingDirectory</key>\n    <string>%s</string>\n", s.Config.WorkingDirectory))

	// Run at load
	sb.WriteString("    <key>RunAtLoad</key>\n    <true/>\n")

	// Keep alive
	if s.Config.RestartPolicy == "always" || s.Config.RestartPolicy == "on-failure" {
		sb.WriteString("    <key>KeepAlive</key>\n")
		if s.Config.RestartPolicy == "always" {
			sb.WriteString("    <true/>\n")
		} else {
			sb.WriteString("    <dict>\n")
			sb.WriteString("        <key>SuccessfulExit</key>\n        <false/>\n")
			sb.WriteString("    </dict>\n")
		}
	}

	// Throttle interval (restart delay)
	sb.WriteString(fmt.Sprintf("    <key>ThrottleInterval</key>\n    <integer>%d</integer>\n", s.Config.RestartDelaySec))

	// Environment variables
	if len(s.Config.Environment) > 0 {
		sb.WriteString("    <key>EnvironmentVariables</key>\n    <dict>\n")
		for key, value := range s.Config.Environment {
			sb.WriteString(fmt.Sprintf("        <key>%s</key>\n        <string>%s</string>\n", key, value))
		}
		sb.WriteString("    </dict>\n")
	}

	// Standard output and error logs
	logDir := filepath.Join(s.Config.WorkingDirectory, "logs")
	sb.WriteString(fmt.Sprintf("    <key>StandardOutPath</key>\n    <string>%s/bibd.log</string>\n", logDir))
	sb.WriteString(fmt.Sprintf("    <key>StandardErrorPath</key>\n    <string>%s/bibd.error.log</string>\n", logDir))

	sb.WriteString("</dict>\n</plist>\n")

	return sb.String()
}

// generateWindowsPowerShell generates PowerShell commands for Windows service installation
func (s *ServiceInstaller) generateWindowsPowerShell() string {
	var sb strings.Builder

	sb.WriteString("# PowerShell script to install bibd as a Windows Service\n")
	sb.WriteString("# Run this script as Administrator\n\n")

	// Check for NSSM or use native sc.exe
	sb.WriteString("# Option 1: Using NSSM (recommended)\n")
	sb.WriteString("# Download NSSM from https://nssm.cc/download\n")
	sb.WriteString(fmt.Sprintf("# nssm install %s \"%s\"\n", s.Config.Name, s.Config.ExecutablePath))
	sb.WriteString(fmt.Sprintf("# nssm set %s AppParameters \"serve --config %s\"\n", s.Config.Name, s.Config.ConfigPath))
	sb.WriteString(fmt.Sprintf("# nssm set %s AppDirectory \"%s\"\n", s.Config.Name, s.Config.WorkingDirectory))
	sb.WriteString(fmt.Sprintf("# nssm set %s Description \"%s\"\n", s.Config.Name, s.Config.Description))
	sb.WriteString(fmt.Sprintf("# nssm set %s Start SERVICE_AUTO_START\n", s.Config.Name))
	sb.WriteString(fmt.Sprintf("# nssm start %s\n\n", s.Config.Name))

	// Native sc.exe approach
	sb.WriteString("# Option 2: Using sc.exe (native)\n")
	binPath := fmt.Sprintf("\"%s\" serve --config \"%s\"", s.Config.ExecutablePath, s.Config.ConfigPath)
	sb.WriteString(fmt.Sprintf("$serviceName = \"%s\"\n", s.Config.Name))
	sb.WriteString(fmt.Sprintf("$displayName = \"%s\"\n", s.Config.DisplayName))
	sb.WriteString(fmt.Sprintf("$description = \"%s\"\n", s.Config.Description))
	sb.WriteString(fmt.Sprintf("$binPath = '%s'\n\n", binPath))

	sb.WriteString("# Create the service\n")
	sb.WriteString("New-Service -Name $serviceName -BinaryPathName $binPath -DisplayName $displayName -Description $description -StartupType Automatic\n\n")

	sb.WriteString("# Start the service\n")
	sb.WriteString("Start-Service -Name $serviceName\n\n")

	sb.WriteString("# Check service status\n")
	sb.WriteString("Get-Service -Name $serviceName\n")

	return sb.String()
}

// InstallInstructions returns human-readable installation instructions
func (s *ServiceInstaller) InstallInstructions() string {
	serviceType := DetectServiceType()
	filePath := s.GetServiceFilePath()

	var sb strings.Builder

	switch serviceType {
	case ServiceTypeSystemd:
		sb.WriteString("📋 Systemd Service Installation\n\n")
		if s.Config.UserService {
			sb.WriteString("User service (no root required):\n\n")
			sb.WriteString(fmt.Sprintf("1. Create the service file:\n   mkdir -p %s\n", filepath.Dir(filePath)))
			sb.WriteString(fmt.Sprintf("   # Save the service file to: %s\n\n", filePath))
			sb.WriteString("2. Reload systemd:\n   systemctl --user daemon-reload\n\n")
			sb.WriteString(fmt.Sprintf("3. Enable and start:\n   systemctl --user enable %s\n", s.Config.Name))
			sb.WriteString(fmt.Sprintf("   systemctl --user start %s\n\n", s.Config.Name))
			sb.WriteString(fmt.Sprintf("4. Check status:\n   systemctl --user status %s\n", s.Config.Name))
		} else {
			sb.WriteString("System service (requires root):\n\n")
			sb.WriteString(fmt.Sprintf("1. Save the service file to: %s\n\n", filePath))
			sb.WriteString("2. Reload systemd:\n   sudo systemctl daemon-reload\n\n")
			sb.WriteString(fmt.Sprintf("3. Enable and start:\n   sudo systemctl enable %s\n", s.Config.Name))
			sb.WriteString(fmt.Sprintf("   sudo systemctl start %s\n\n", s.Config.Name))
			sb.WriteString(fmt.Sprintf("4. Check status:\n   sudo systemctl status %s\n", s.Config.Name))
		}

	case ServiceTypeLaunchd:
		sb.WriteString("📋 Launchd Service Installation\n\n")
		if s.Config.UserService {
			sb.WriteString("User agent (no root required):\n\n")
			sb.WriteString(fmt.Sprintf("1. Create the logs directory:\n   mkdir -p %s/logs\n\n", s.Config.WorkingDirectory))
			sb.WriteString(fmt.Sprintf("2. Save the plist to: %s\n\n", filePath))
			sb.WriteString(fmt.Sprintf("3. Load the agent:\n   launchctl load %s\n\n", filePath))
			sb.WriteString(fmt.Sprintf("4. Check status:\n   launchctl list | grep %s\n", s.Config.Name))
		} else {
			sb.WriteString("System daemon (requires root):\n\n")
			sb.WriteString(fmt.Sprintf("1. Save the plist to: %s\n\n", filePath))
			sb.WriteString(fmt.Sprintf("2. Set permissions:\n   sudo chown root:wheel %s\n", filePath))
			sb.WriteString(fmt.Sprintf("   sudo chmod 644 %s\n\n", filePath))
			sb.WriteString(fmt.Sprintf("3. Load the daemon:\n   sudo launchctl load %s\n\n", filePath))
			sb.WriteString(fmt.Sprintf("4. Check status:\n   sudo launchctl list | grep %s\n", s.Config.Name))
		}

	case ServiceTypeWindows:
		sb.WriteString("📋 Windows Service Installation\n\n")
		sb.WriteString("Run the generated PowerShell script as Administrator.\n\n")
		sb.WriteString("Using NSSM (recommended):\n")
		sb.WriteString("1. Download NSSM from https://nssm.cc/download\n")
		sb.WriteString("2. Run the NSSM commands from the script\n\n")
		sb.WriteString("Using native sc.exe:\n")
		sb.WriteString("1. Open PowerShell as Administrator\n")
		sb.WriteString("2. Run the New-Service command from the script\n")
	}

	return sb.String()
}
