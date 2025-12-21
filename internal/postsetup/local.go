// Package postsetup provides post-setup verification utilities.
package postsetup

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// LocalStatus contains the status of a local bibd installation
type LocalStatus struct {
	// Running indicates if bibd is running
	Running bool

	// PID is the process ID if running
	PID int

	// Address is the listening address
	Address string

	// Version is the bibd version
	Version string

	// Uptime is how long bibd has been running
	Uptime time.Duration

	// Healthy indicates if health check passed
	Healthy bool

	// HealthStatus is the health check status message
	HealthStatus string

	// Mode is the P2P mode
	Mode string

	// Error contains any error message
	Error string
}

// LocalVerifier verifies local bibd installation
type LocalVerifier struct {
	// Address is the bibd address to check
	Address string

	// Timeout is the verification timeout
	Timeout time.Duration
}

// NewLocalVerifier creates a new local verifier
func NewLocalVerifier(address string) *LocalVerifier {
	if address == "" {
		address = "localhost:4000"
	}
	return &LocalVerifier{
		Address: address,
		Timeout: 10 * time.Second,
	}
}

// Verify checks the local bibd installation
func (v *LocalVerifier) Verify(ctx context.Context) *LocalStatus {
	status := &LocalStatus{
		Address: v.Address,
	}

	// Check if port is open
	conn, err := net.DialTimeout("tcp", v.Address, 2*time.Second)
	if err != nil {
		status.Running = false
		status.Error = fmt.Sprintf("Cannot connect to %s: %v", v.Address, err)
		return status
	}
	conn.Close()
	status.Running = true

	// Try to get version via health check
	// This would normally use the gRPC client, but for post-setup
	// we just verify connectivity
	status.Healthy = true
	status.HealthStatus = "reachable"

	return status
}

// CheckServiceStatus checks if bibd is running as a service
func CheckServiceStatus() *ServiceStatus {
	status := &ServiceStatus{}

	switch runtime.GOOS {
	case "linux":
		status = checkSystemdService()
	case "darwin":
		status = checkLaunchdService()
	case "windows":
		status = checkWindowsService()
	default:
		status.Error = "unsupported operating system"
	}

	return status
}

// ServiceStatus contains service status information
type ServiceStatus struct {
	// Installed indicates if service is installed
	Installed bool

	// Running indicates if service is running
	Running bool

	// Enabled indicates if service is enabled at boot
	Enabled bool

	// ServiceName is the name of the service
	ServiceName string

	// Error contains any error
	Error string
}

func checkSystemdService() *ServiceStatus {
	status := &ServiceStatus{
		ServiceName: "bibd",
	}

	// Check if service is installed
	cmd := exec.Command("systemctl", "cat", "bibd.service")
	if err := cmd.Run(); err == nil {
		status.Installed = true
	}

	// Check if running
	cmd = exec.Command("systemctl", "is-active", "bibd.service")
	output, _ := cmd.Output()
	status.Running = strings.TrimSpace(string(output)) == "active"

	// Check if enabled
	cmd = exec.Command("systemctl", "is-enabled", "bibd.service")
	output, _ = cmd.Output()
	status.Enabled = strings.TrimSpace(string(output)) == "enabled"

	return status
}

func checkLaunchdService() *ServiceStatus {
	status := &ServiceStatus{
		ServiceName: "com.bib.bibd",
	}

	// Check if plist exists
	cmd := exec.Command("launchctl", "list", "com.bib.bibd")
	if err := cmd.Run(); err == nil {
		status.Installed = true
		status.Running = true
	}

	return status
}

func checkWindowsService() *ServiceStatus {
	status := &ServiceStatus{
		ServiceName: "bibd",
	}

	// Query service status
	cmd := exec.Command("sc", "query", "bibd")
	output, err := cmd.Output()
	if err == nil {
		status.Installed = true
		status.Running = strings.Contains(string(output), "RUNNING")
	}

	return status
}

// FormatLocalStatus formats local status for display
func FormatLocalStatus(status *LocalStatus) string {
	var sb strings.Builder

	if status.Running {
		sb.WriteString("🟢 bibd is running\n")
	} else {
		sb.WriteString("🔴 bibd is not running\n")
	}

	sb.WriteString(fmt.Sprintf("   Address: %s\n", status.Address))

	if status.Version != "" {
		sb.WriteString(fmt.Sprintf("   Version: %s\n", status.Version))
	}

	if status.Uptime > 0 {
		sb.WriteString(fmt.Sprintf("   Uptime:  %s\n", status.Uptime))
	}

	if status.Mode != "" {
		sb.WriteString(fmt.Sprintf("   Mode:    %s\n", status.Mode))
	}

	if status.Healthy {
		sb.WriteString(fmt.Sprintf("   Health:  ✓ %s\n", status.HealthStatus))
	} else if status.Error != "" {
		sb.WriteString(fmt.Sprintf("   Error:   %s\n", status.Error))
	}

	return sb.String()
}

// FormatServiceStatus formats service status for display
func FormatServiceStatus(status *ServiceStatus) string {
	var sb strings.Builder

	if !status.Installed {
		sb.WriteString(fmt.Sprintf("📦 Service '%s' is not installed\n", status.ServiceName))
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("📦 Service: %s\n", status.ServiceName))

	if status.Running {
		sb.WriteString("   Status:  🟢 Running\n")
	} else {
		sb.WriteString("   Status:  🔴 Stopped\n")
	}

	if status.Enabled {
		sb.WriteString("   Enabled: ✓ Yes (starts at boot)\n")
	} else {
		sb.WriteString("   Enabled: ✗ No\n")
	}

	return sb.String()
}

// GetLocalManagementCommands returns commands for managing local bibd
func GetLocalManagementCommands() []string {
	switch runtime.GOOS {
	case "linux":
		return []string{
			"sudo systemctl status bibd",
			"sudo systemctl start bibd",
			"sudo systemctl stop bibd",
			"sudo systemctl restart bibd",
			"sudo journalctl -u bibd -f",
		}
	case "darwin":
		return []string{
			"launchctl list com.bib.bibd",
			"launchctl start com.bib.bibd",
			"launchctl stop com.bib.bibd",
			"tail -f ~/Library/Logs/bibd.log",
		}
	case "windows":
		return []string{
			"sc query bibd",
			"sc start bibd",
			"sc stop bibd",
			"Get-Content $env:ProgramData\\bib\\logs\\bibd.log -Wait",
		}
	default:
		return []string{
			"bibd --config <path>",
		}
	}
}
