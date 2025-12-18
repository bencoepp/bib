package postgres

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// NetworkSecurityLevel represents the achieved network security level.
type NetworkSecurityLevel string

const (
	// NetworkSecurityMaximum indicates all security requirements are met.
	NetworkSecurityMaximum NetworkSecurityLevel = "maximum"

	// NetworkSecurityHigh indicates minor fallbacks were used.
	NetworkSecurityHigh NetworkSecurityLevel = "high"

	// NetworkSecurityModerate indicates significant fallbacks were used.
	NetworkSecurityModerate NetworkSecurityLevel = "moderate"

	// NetworkSecurityReduced indicates major security fallbacks.
	NetworkSecurityReduced NetworkSecurityLevel = "reduced"
)

// NetworkSecurityConfig holds network security configuration.
type NetworkSecurityConfig struct {
	// UseUnixSocket uses Unix sockets instead of TCP (Linux only).
	UseUnixSocket bool `mapstructure:"use_unix_socket"`

	// UseBridgeNetwork creates an isolated Docker network.
	UseBridgeNetwork bool `mapstructure:"use_bridge_network"`

	// BridgeNetworkName is the name of the isolated network.
	BridgeNetworkName string `mapstructure:"bridge_network_name"`

	// BindAddress is the address PostgreSQL binds to (default: 127.0.0.1).
	BindAddress string `mapstructure:"bind_address"`

	// RequireClientCert requires client certificate authentication.
	RequireClientCert bool `mapstructure:"require_client_cert"`

	// AllowedClientCNs are the allowed client certificate common names.
	AllowedClientCNs []string `mapstructure:"allowed_client_cns"`

	// FallbackMode controls behavior when requirements can't be met.
	// Options: "strict", "warn", "permissive"
	FallbackMode string `mapstructure:"fallback_mode"`
}

// DefaultNetworkSecurityConfig returns secure defaults.
func DefaultNetworkSecurityConfig() NetworkSecurityConfig {
	return NetworkSecurityConfig{
		UseUnixSocket:     runtime.GOOS == "linux",
		UseBridgeNetwork:  true,
		BridgeNetworkName: "bibd-internal",
		BindAddress:       "127.0.0.1",
		RequireClientCert: true,
		AllowedClientCNs:  []string{"bibd-client"},
		FallbackMode:      "warn",
	}
}

// NetworkManager handles network isolation for PostgreSQL.
type NetworkManager struct {
	config      NetworkSecurityConfig
	runtime     RuntimeType
	nodeID      string
	networkName string
	warnings    []string
	level       NetworkSecurityLevel
}

// NewNetworkManager creates a new network manager.
func NewNetworkManager(cfg NetworkSecurityConfig, rt RuntimeType, nodeID string) *NetworkManager {
	networkName := cfg.BridgeNetworkName
	if networkName == "" {
		networkName = fmt.Sprintf("bibd-internal-%s", nodeID[:8])
	}

	return &NetworkManager{
		config:      cfg,
		runtime:     rt,
		nodeID:      nodeID,
		networkName: networkName,
		warnings:    make([]string, 0),
		level:       NetworkSecurityMaximum,
	}
}

// Setup configures network isolation based on the runtime and platform.
func (nm *NetworkManager) Setup(ctx context.Context) error {
	// Check platform capabilities
	nm.checkPlatformCapabilities()

	switch nm.runtime {
	case RuntimeDocker, RuntimePodman:
		return nm.setupContainerNetwork(ctx)
	case RuntimeKubernetes:
		// Kubernetes networking is handled separately
		return nil
	default:
		nm.addWarning("Unknown runtime, network isolation may be incomplete")
		return nil
	}
}

// checkPlatformCapabilities checks what security features are available.
func (nm *NetworkManager) checkPlatformCapabilities() {
	switch runtime.GOOS {
	case "linux":
		// Linux supports Unix sockets
		if !nm.config.UseUnixSocket {
			nm.addWarning("Unix sockets disabled on Linux, using TCP instead")
			nm.downgrade(NetworkSecurityHigh)
		}
	case "darwin", "windows":
		// macOS and Windows don't support Unix sockets for Docker
		if nm.config.UseUnixSocket {
			nm.addWarning(fmt.Sprintf("Unix sockets not supported on %s, falling back to TCP with mTLS", runtime.GOOS))
			nm.config.UseUnixSocket = false
			nm.downgrade(NetworkSecurityHigh)
		}
	}

	// Verify mTLS requirement for TCP
	if !nm.config.UseUnixSocket && !nm.config.RequireClientCert {
		if nm.config.FallbackMode == "strict" {
			nm.addWarning("Client certificates should be required for TCP connections")
		} else {
			nm.addWarning("Client certificates not required, connection security reduced")
			nm.downgrade(NetworkSecurityModerate)
		}
	}
}

// setupContainerNetwork creates an isolated Docker/Podman network.
func (nm *NetworkManager) setupContainerNetwork(ctx context.Context) error {
	if !nm.config.UseBridgeNetwork {
		nm.addWarning("Isolated bridge network disabled, using default network")
		nm.downgrade(NetworkSecurityModerate)
		return nil
	}

	// Check if network already exists
	exists, err := nm.networkExists(ctx)
	if err != nil {
		return fmt.Errorf("failed to check network existence: %w", err)
	}

	if exists {
		return nil
	}

	// Create isolated internal network
	return nm.createIsolatedNetwork(ctx)
}

// networkExists checks if the isolated network already exists.
func (nm *NetworkManager) networkExists(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, string(nm.runtime), "network", "inspect", nm.networkName)
	err := cmd.Run()
	return err == nil, nil
}

// createIsolatedNetwork creates a new isolated Docker/Podman network.
func (nm *NetworkManager) createIsolatedNetwork(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	args := []string{
		"network", "create",
		"--internal", // No external connectivity
		"--driver", "bridge",
		"--opt", "com.docker.network.bridge.enable_ip_masquerade=false",
		"--label", fmt.Sprintf("bibd.node=%s", nm.nodeID),
		"--label", "bibd.purpose=postgres-isolation",
		nm.networkName,
	}

	cmd := exec.CommandContext(ctx, string(nm.runtime), args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if error is "already exists"
		if strings.Contains(string(output), "already exists") {
			return nil
		}
		return fmt.Errorf("failed to create isolated network: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// Cleanup removes the isolated network.
func (nm *NetworkManager) Cleanup(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, string(nm.runtime), "network", "rm", nm.networkName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Ignore "not found" errors
		if strings.Contains(string(output), "not found") ||
			strings.Contains(string(output), "No such network") {
			return nil
		}
		return fmt.Errorf("failed to remove network: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// NetworkName returns the isolated network name.
func (nm *NetworkManager) NetworkName() string {
	return nm.networkName
}

// SecurityLevel returns the achieved security level.
func (nm *NetworkManager) SecurityLevel() NetworkSecurityLevel {
	return nm.level
}

// Warnings returns any security warnings.
func (nm *NetworkManager) Warnings() []string {
	return nm.warnings
}

// addWarning adds a security warning.
func (nm *NetworkManager) addWarning(msg string) {
	nm.warnings = append(nm.warnings, msg)
}

// downgrade lowers the security level if necessary.
func (nm *NetworkManager) downgrade(level NetworkSecurityLevel) {
	// Only downgrade, never upgrade
	switch nm.level {
	case NetworkSecurityMaximum:
		nm.level = level
	case NetworkSecurityHigh:
		if level == NetworkSecurityModerate || level == NetworkSecurityReduced {
			nm.level = level
		}
	case NetworkSecurityModerate:
		if level == NetworkSecurityReduced {
			nm.level = level
		}
	}
}

// GetContainerNetworkArgs returns Docker/Podman arguments for network configuration.
func (nm *NetworkManager) GetContainerNetworkArgs() []string {
	args := make([]string, 0)

	if nm.config.UseBridgeNetwork {
		args = append(args, "--network", nm.networkName)
	}

	// Port binding - only to localhost
	if !nm.config.UseUnixSocket {
		args = append(args, "-p", fmt.Sprintf("%s:5432:5432", nm.config.BindAddress))
	}

	return args
}

// GenerateReport generates a network security report.
func (nm *NetworkManager) GenerateReport() NetworkSecurityReport {
	return NetworkSecurityReport{
		Level:             nm.level,
		Platform:          runtime.GOOS,
		Runtime:           string(nm.runtime),
		UseUnixSocket:     nm.config.UseUnixSocket,
		UseBridgeNetwork:  nm.config.UseBridgeNetwork,
		NetworkName:       nm.networkName,
		BindAddress:       nm.config.BindAddress,
		RequireClientCert: nm.config.RequireClientCert,
		Warnings:          nm.warnings,
	}
}

// NetworkSecurityReport contains network security assessment.
type NetworkSecurityReport struct {
	Level             NetworkSecurityLevel `json:"level"`
	Platform          string               `json:"platform"`
	Runtime           string               `json:"runtime"`
	UseUnixSocket     bool                 `json:"use_unix_socket"`
	UseBridgeNetwork  bool                 `json:"use_bridge_network"`
	NetworkName       string               `json:"network_name"`
	BindAddress       string               `json:"bind_address"`
	RequireClientCert bool                 `json:"require_client_cert"`
	Warnings          []string             `json:"warnings,omitempty"`
}

// TCPSecurityConfig holds TCP-specific security settings for non-Linux platforms.
type TCPSecurityConfig struct {
	// BindAddress is always 127.0.0.1 for security.
	BindAddress string

	// RequireClientCert is always true for managed mode.
	RequireClientCert bool

	// MinTLSVersion is the minimum TLS version (1.3 preferred).
	MinTLSVersion string

	// AllowedClientCNs are the allowed client certificate common names.
	AllowedClientCNs []string

	// MaxConnections limits connections.
	MaxConnections int

	// ConnectionTimeout for idle connections.
	ConnectionTimeout time.Duration
}

// DefaultTCPSecurityConfig returns secure TCP defaults.
func DefaultTCPSecurityConfig() TCPSecurityConfig {
	return TCPSecurityConfig{
		BindAddress:       "127.0.0.1",
		RequireClientCert: true,
		MinTLSVersion:     "TLSv1.3",
		AllowedClientCNs:  []string{"bibd-client"},
		MaxConnections:    100,
		ConnectionTimeout: 30 * time.Second,
	}
}
