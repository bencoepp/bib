// Package postgres provides PostgreSQL lifecycle management for bibd.
// This includes container management (Docker/Podman) and Kubernetes integration.
package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"bib/internal/storage"
	"bib/internal/storage/postgres/lifecycle/certs"
)

// RuntimeType represents the container runtime type.
type RuntimeType string

const (
	RuntimeDocker     RuntimeType = "docker"
	RuntimePodman     RuntimeType = "podman"
	RuntimeKubernetes RuntimeType = "kubernetes"
	RuntimeManual     RuntimeType = "manual" // User-managed PostgreSQL
)

// HealthAction defines what happens on repeated health check failures.
type HealthAction string

const (
	HealthActionShutdown    HealthAction = "shutdown"     // Shutdown bibd on failure
	HealthActionRetryAlways HealthAction = "retry_always" // Keep retrying forever
	HealthActionRetryLimit  HealthAction = "retry_limit"  // Retry up to MaxRetries
)

// LifecycleConfig holds configuration for PostgreSQL lifecycle management.
type LifecycleConfig struct {
	// Runtime is the container runtime type (auto-detected if empty)
	Runtime RuntimeType `mapstructure:"runtime"`

	// SocketPath is the path to the container runtime socket
	// Auto-detected if empty
	SocketPath string `mapstructure:"socket_path"`

	// KubeconfigPath is the path to kubeconfig file (for Kubernetes runtime)
	KubeconfigPath string `mapstructure:"kubeconfig_path"`

	// ContainerName is the name of the PostgreSQL container
	// Defaults to bibd-postgres-<node-id>
	ContainerName string `mapstructure:"container_name"`

	// Image is the PostgreSQL container image
	Image string `mapstructure:"image"`

	// DataDir is where PostgreSQL data is stored
	DataDir string `mapstructure:"data_dir"`

	// Port is the PostgreSQL port (internal)
	Port int `mapstructure:"port"`

	// Network configuration
	Network NetworkConfig `mapstructure:"network"`

	// Resources for the container
	Resources ResourceConfig `mapstructure:"resources"`

	// Health check configuration
	Health HealthConfig `mapstructure:"health"`

	// TLS configuration
	TLS TLSConfig `mapstructure:"tls"`

	// Kubernetes configuration (for Kubernetes runtime)
	Kubernetes storage.KubernetesConfig `mapstructure:"kubernetes"`

	// Credentials configuration
	Credentials CredentialsConfig `mapstructure:"credentials"`
}

// NetworkConfig holds network configuration for PostgreSQL.
type NetworkConfig struct {
	// UseBridgeNetwork creates a private bridge network for isolation
	UseBridgeNetwork bool `mapstructure:"use_bridge_network"`

	// BridgeNetworkName is the name of the bridge network
	BridgeNetworkName string `mapstructure:"bridge_network_name"`

	// UseUnixSocket uses Unix socket only (no TCP)
	UseUnixSocket bool `mapstructure:"use_unix_socket"`

	// BindAddress is the address to bind to (default: 127.0.0.1)
	BindAddress string `mapstructure:"bind_address"`
}

// ResourceConfig holds resource limits for the container.
type ResourceConfig struct {
	// MemoryMB is the memory limit in megabytes
	MemoryMB int `mapstructure:"memory_mb"`

	// CPUCores is the CPU limit (can be fractional)
	CPUCores float64 `mapstructure:"cpu_cores"`
}

// HealthConfig holds health check configuration.
type HealthConfig struct {
	// Interval is how often to check health
	Interval time.Duration `mapstructure:"interval"`

	// Timeout is the timeout for each health check
	Timeout time.Duration `mapstructure:"timeout"`

	// StartupTimeout is how long to wait for initial startup
	StartupTimeout time.Duration `mapstructure:"startup_timeout"`

	// Action defines what happens on repeated failures
	Action HealthAction `mapstructure:"action"`

	// MaxRetries is the maximum retries (for HealthActionRetryLimit)
	MaxRetries int `mapstructure:"max_retries"`

	// RetryBackoff is the backoff duration between retries
	RetryBackoff time.Duration `mapstructure:"retry_backoff"`
}

// TLSConfig holds TLS configuration for PostgreSQL connections.
type TLSConfig struct {
	// Enabled controls whether mTLS is enabled (always true for managed)
	Enabled bool `mapstructure:"enabled"`

	// CertDir is where certificates are stored
	CertDir string `mapstructure:"cert_dir"`

	// AutoGenerate automatically generates certificates from node identity
	AutoGenerate bool `mapstructure:"auto_generate"`
}

// CredentialsConfig holds credential management configuration.
type CredentialsConfig struct {
	// EncryptedPath is where encrypted credentials are stored
	EncryptedPath string `mapstructure:"encrypted_path"`

	// RotationInterval is how often to rotate credentials
	RotationInterval time.Duration `mapstructure:"rotation_interval"`
}

// DefaultLifecycleConfig returns sensible defaults.
func DefaultLifecycleConfig() LifecycleConfig {
	// On Windows, we can't use Unix sockets for PostgreSQL connections
	useUnixSocket := true
	if filepath.Separator == '\\' { // Windows uses backslash
		useUnixSocket = false
	}

	return LifecycleConfig{
		Runtime:    "", // Auto-detect
		SocketPath: "", // Auto-detect
		Image:      "postgres:16-alpine",
		Port:       5432,
		Network: NetworkConfig{
			UseBridgeNetwork:  true,
			BridgeNetworkName: "bibd-network",
			UseUnixSocket:     useUnixSocket,
			BindAddress:       "127.0.0.1",
		},
		Resources: ResourceConfig{
			MemoryMB: 512,
			CPUCores: 1.0,
		},
		Health: HealthConfig{
			Interval:       5 * time.Second,
			Timeout:        5 * time.Second,
			StartupTimeout: 60 * time.Second,
			Action:         HealthActionRetryLimit,
			MaxRetries:     5,
			RetryBackoff:   10 * time.Second,
		},
		TLS: TLSConfig{
			Enabled:      true,
			AutoGenerate: true,
		},
		Kubernetes: storage.KubernetesConfig{
			// Use defaults from storage.DefaultConfig()
			StorageSize:          "10Gi",
			BackupEnabled:        false, // Disabled by default for container runtimes
			NetworkPolicyEnabled: false, // Not applicable for container runtimes
			PodAntiAffinity:      false,
			UpdateStrategy:       "RollingUpdate",
			DeleteOnCleanup:      true,
		},
		Credentials: CredentialsConfig{
			RotationInterval: 7 * 24 * time.Hour,
		},
	}
}

// Manager handles PostgreSQL lifecycle management.
type Manager struct {
	cfg     LifecycleConfig
	nodeID  string
	dataDir string
	runtime RuntimeType
	store   storage.Store

	// State
	mu           sync.RWMutex
	containerID  string
	ready        bool
	lastHealth   time.Time
	healthErrors int
	shutdownCh   chan struct{}
	credentials  *Credentials

	// Kubernetes manager (if using Kubernetes runtime)
	k8sManager *KubernetesManager
}

// Credentials holds PostgreSQL credentials.
type Credentials struct {
	SuperuserPassword string
	AdminPassword     string
	ScrapePassword    string
	QueryPassword     string
	TransformPassword string
	AuditPassword     string
	ReadOnlyPassword  string
	GeneratedAt       time.Time
}

// NewManager creates a new PostgreSQL lifecycle manager.
func NewManager(cfg LifecycleConfig, nodeID, dataDir string) (*Manager, error) {
	m := &Manager{
		cfg:        cfg,
		nodeID:     nodeID,
		dataDir:    dataDir,
		shutdownCh: make(chan struct{}),
	}

	// Detect or validate runtime
	runtime, err := m.detectRuntime()
	if err != nil {
		return nil, fmt.Errorf("failed to detect runtime: %w", err)
	}
	m.runtime = runtime

	// Set defaults based on node ID
	if m.cfg.ContainerName == "" {
		m.cfg.ContainerName = fmt.Sprintf("bibd-postgres-%s", nodeID)
	}

	if m.cfg.DataDir == "" {
		m.cfg.DataDir = filepath.Join(dataDir, "postgres")
	}

	if m.cfg.TLS.CertDir == "" {
		m.cfg.TLS.CertDir = filepath.Join(m.cfg.DataDir, "certs")
	}

	if m.cfg.Credentials.EncryptedPath == "" {
		m.cfg.Credentials.EncryptedPath = filepath.Join(dataDir, "secrets", "db.enc")
	}

	return m, nil
}

// detectRuntime detects the available container runtime.
func (m *Manager) detectRuntime() (RuntimeType, error) {
	// If explicitly configured, validate and use it
	if m.cfg.Runtime != "" {
		switch m.cfg.Runtime {
		case RuntimeDocker, RuntimePodman, RuntimeKubernetes, RuntimeManual:
			return m.cfg.Runtime, nil
		default:
			return "", fmt.Errorf("unknown runtime: %s", m.cfg.Runtime)
		}
	}

	// Auto-detect: check Kubernetes first
	if m.isKubernetes() {
		// But prefer container if both are available (per user requirement)
		if m.isDockerAvailable() || m.isPodmanAvailable() {
			// Container takes precedence over Kubernetes
			if m.isDockerAvailable() {
				return RuntimeDocker, nil
			}
			return RuntimePodman, nil
		}
		return RuntimeKubernetes, nil
	}

	// Check Docker first (preferred), then Podman
	if m.isDockerAvailable() {
		return RuntimeDocker, nil
	}

	if m.isPodmanAvailable() {
		return RuntimePodman, nil
	}

	return "", fmt.Errorf("no container runtime found; install Docker or Podman, or configure manual PostgreSQL (tried: Docker='docker info', Podman='podman info')")
}

// isKubernetes checks if running in Kubernetes.
func (m *Manager) isKubernetes() bool {
	// Check environment variable
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true
	}

	// Check if kubeconfig path is provided
	if m.cfg.KubeconfigPath != "" {
		if _, err := os.Stat(m.cfg.KubeconfigPath); err == nil {
			return true
		}
	}

	// Check default kubeconfig location
	home, _ := os.UserHomeDir()
	defaultKubeconfig := filepath.Join(home, ".kube", "config")
	if _, err := os.Stat(defaultKubeconfig); err == nil {
		// Has kubeconfig but might not be in cluster
		// Only return true if KUBERNETES_SERVICE_HOST is set
		return false
	}

	return false
}

// isDockerAvailable checks if Docker is available.
func (m *Manager) isDockerAvailable() bool {
	// Try to run docker command directly (works on all platforms including Windows)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "info")

	if cmd.Run() == nil {
		return true
	}

	// Fallback: check socket path (Unix/Linux only)
	socketPath := m.cfg.SocketPath
	if socketPath == "" {
		socketPath = "/var/run/docker.sock"
	}

	if _, err := os.Stat(socketPath); err == nil {
		// Socket exists, verify Docker is responsive
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "docker", "info")
		return cmd.Run() == nil
	}

	return false
}

// isPodmanAvailable checks if Podman is available.
func (m *Manager) isPodmanAvailable() bool {
	// First, try to run podman command directly (works on all platforms)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "podman", "info")
	if cmd.Run() == nil {
		return true
	}

	// Fallback: check socket path (Unix/Linux only)
	socketPath := m.cfg.SocketPath
	if socketPath == "" {
		// Check common Podman socket locations
		home, _ := os.UserHomeDir()
		candidates := []string{
			filepath.Join(home, ".local/share/containers/podman/machine/podman.sock"),
			"/run/podman/podman.sock",
			"/var/run/podman/podman.sock",
		}
		for _, path := range candidates {
			if _, err := os.Stat(path); err == nil {
				socketPath = path
				break
			}
		}
	}

	if socketPath != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "podman", "info")
		return cmd.Run() == nil
	}

	return false
}

// Start starts the PostgreSQL instance and waits for it to be ready.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create necessary directories
	if err := m.createDirectories(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Generate or load credentials
	if err := m.initCredentials(); err != nil {
		return fmt.Errorf("failed to initialize credentials: %w", err)
	}

	// Generate TLS certificates if needed
	if m.cfg.TLS.Enabled && m.cfg.TLS.AutoGenerate {
		if err := m.generateCertificates(); err != nil {
			return fmt.Errorf("failed to generate certificates: %w", err)
		}
	}

	// Generate PostgreSQL configuration
	// Temporarily disabled - using PostgreSQL defaults
	// if err := m.generatePostgresConfig(); err != nil {
	// 	return fmt.Errorf("failed to generate PostgreSQL config: %w", err)
	// }

	// Start based on runtime
	switch m.runtime {
	case RuntimeDocker:
		if err := m.startDocker(ctx); err != nil {
			return err
		}
	case RuntimePodman:
		if err := m.startPodman(ctx); err != nil {
			return err
		}
	case RuntimeKubernetes:
		if err := m.startKubernetes(ctx); err != nil {
			return err
		}
	case RuntimeManual:
		// Nothing to start, just verify connection
	default:
		return fmt.Errorf("unknown runtime: %s", m.runtime)
	}

	// Wait for PostgreSQL to be ready
	if err := m.waitForReady(ctx); err != nil {
		return fmt.Errorf("PostgreSQL failed to become ready: %w", err)
	}

	// Initialize database roles
	if err := m.initializeRoles(ctx); err != nil {
		return fmt.Errorf("failed to initialize database roles: %w", err)
	}

	m.ready = true

	// Start health check goroutine
	go m.healthCheckLoop()

	return nil
}

// createDirectories creates necessary directories.
func (m *Manager) createDirectories() error {
	dirs := []string{
		m.cfg.DataDir,
		filepath.Join(m.cfg.DataDir, "data"),
		filepath.Join(m.cfg.DataDir, "config"),
		m.cfg.TLS.CertDir,
		filepath.Dir(m.cfg.Credentials.EncryptedPath),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// initCredentials initializes or loads credentials.
func (m *Manager) initCredentials() error {
	// Check if credentials file exists
	if _, err := os.Stat(m.cfg.Credentials.EncryptedPath); err == nil {
		// TODO: Load and decrypt existing credentials
		// For now, generate new ones
	}

	// Generate new credentials
	creds := &Credentials{
		GeneratedAt: time.Now().UTC(),
	}

	var err error
	creds.SuperuserPassword, err = generatePassword(64)
	if err != nil {
		return err
	}
	creds.AdminPassword, err = generatePassword(64)
	if err != nil {
		return err
	}
	creds.ScrapePassword, err = generatePassword(64)
	if err != nil {
		return err
	}
	creds.QueryPassword, err = generatePassword(64)
	if err != nil {
		return err
	}
	creds.TransformPassword, err = generatePassword(64)
	if err != nil {
		return err
	}
	creds.AuditPassword, err = generatePassword(64)
	if err != nil {
		return err
	}
	creds.ReadOnlyPassword, err = generatePassword(64)
	if err != nil {
		return err
	}

	m.credentials = creds

	// TODO: Encrypt and save credentials
	return nil
}

// generatePassword generates a secure random password.
func generatePassword(length int) (string, error) {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// generateCertificates generates TLS certificates.
func (m *Manager) generateCertificates() error {
	// Check if certificates already exist and don't need rotation
	if certs.Exists(m.cfg.TLS.CertDir) {
		needsRotation, err := certs.NeedsRotation(m.cfg.TLS.CertDir, 30*24*time.Hour) // 30 days threshold
		if err == nil && !needsRotation {
			return nil // Certificates are still valid
		}
	}

	// Generate new certificates
	cfg := certs.DefaultGeneratorConfig(m.nodeID)
	bundle, err := certs.Generate(cfg)
	if err != nil {
		return fmt.Errorf("failed to generate certificates: %w", err)
	}

	if err := bundle.SaveToDir(m.cfg.TLS.CertDir); err != nil {
		return fmt.Errorf("failed to save certificates: %w", err)
	}

	return nil
}

// generatePostgresConfig generates postgresql.conf and pg_hba.conf.
func (m *Manager) generatePostgresConfig() error {
	configDir := filepath.Join(m.cfg.DataDir, "config")

	// Generate postgresql.conf
	postgresConf := m.buildPostgresConf()
	if err := os.WriteFile(filepath.Join(configDir, "postgresql.conf"), []byte(postgresConf), 0600); err != nil {
		return fmt.Errorf("failed to write postgresql.conf: %w", err)
	}

	// Generate pg_hba.conf
	pgHbaConf := m.buildPgHbaConf()
	if err := os.WriteFile(filepath.Join(configDir, "pg_hba.conf"), []byte(pgHbaConf), 0600); err != nil {
		return fmt.Errorf("failed to write pg_hba.conf: %w", err)
	}

	return nil
}

// buildPostgresConf builds the postgresql.conf content.
func (m *Manager) buildPostgresConf() string {
	var sb strings.Builder

	sb.WriteString("# Generated by bibd - do not edit manually\n\n")

	// Listen addresses
	if m.cfg.Network.UseUnixSocket {
		sb.WriteString("listen_addresses = ''\n")
		sb.WriteString(fmt.Sprintf("unix_socket_directories = '%s'\n", filepath.Join(m.cfg.DataDir, "run")))
	} else {
		// Inside Docker container, listen on all interfaces
		// Port forwarding will restrict access to 127.0.0.1 on the host
		sb.WriteString("listen_addresses = '*'\n")
	}
	sb.WriteString(fmt.Sprintf("port = %d\n", m.cfg.Port))

	// SSL
	if m.cfg.TLS.Enabled {
		sb.WriteString("\n# SSL Configuration\n")
		sb.WriteString("ssl = on\n")
		sb.WriteString(fmt.Sprintf("ssl_cert_file = '%s/server.crt'\n", m.cfg.TLS.CertDir))
		sb.WriteString(fmt.Sprintf("ssl_key_file = '%s/server.key'\n", m.cfg.TLS.CertDir))
		sb.WriteString(fmt.Sprintf("ssl_ca_file = '%s/ca.crt'\n", m.cfg.TLS.CertDir))
	}

	// Authentication
	sb.WriteString("\n# Authentication\n")
	sb.WriteString("password_encryption = scram-sha-256\n")

	// Logging for audit
	sb.WriteString("\n# Logging\n")
	sb.WriteString("log_statement = 'all'\n")
	sb.WriteString("log_connections = on\n")
	sb.WriteString("log_disconnections = on\n")
	sb.WriteString("log_duration = on\n")

	// Performance
	sb.WriteString("\n# Performance\n")
	sb.WriteString("shared_preload_libraries = 'pg_stat_statements'\n")

	return sb.String()
}

// buildPgHbaConf builds the pg_hba.conf content.
func (m *Manager) buildPgHbaConf() string {
	var sb strings.Builder

	sb.WriteString("# Generated by bibd - do not edit manually\n")
	sb.WriteString("# TYPE  DATABASE        USER            ADDRESS                 METHOD\n\n")

	// Local connections (Unix socket)
	sb.WriteString("local   all             all                                     scram-sha-256\n")

	// Host connections (if TCP enabled)
	if !m.cfg.Network.UseUnixSocket {
		if m.cfg.TLS.Enabled {
			sb.WriteString("hostssl all             all             127.0.0.1/32            scram-sha-256\n")
			sb.WriteString("hostssl all             all             ::1/128                 scram-sha-256\n")
		} else {
			sb.WriteString("host    all             all             127.0.0.1/32            scram-sha-256\n")
			sb.WriteString("host    all             all             ::1/128                 scram-sha-256\n")
		}
	}

	// Reject everything else
	sb.WriteString("\n# Reject all other connections\n")
	sb.WriteString("host    all             all             0.0.0.0/0               reject\n")
	sb.WriteString("host    all             all             ::/0                    reject\n")

	return sb.String()
}

// startDocker starts PostgreSQL using Docker.
func (m *Manager) startDocker(ctx context.Context) error {
	return m.startContainer(ctx, "docker")
}

// startPodman starts PostgreSQL using Podman.
func (m *Manager) startPodman(ctx context.Context) error {
	return m.startContainer(ctx, "podman")
}

// startContainer starts PostgreSQL using the specified container runtime.
func (m *Manager) startContainer(ctx context.Context, runtime string) error {
	// Check if container already exists
	checkCmd := exec.CommandContext(ctx, runtime, "container", "inspect", m.cfg.ContainerName)
	if err := checkCmd.Run(); err == nil {
		// Container exists, start it if not running
		startCmd := exec.CommandContext(ctx, runtime, "start", m.cfg.ContainerName)
		if err := startCmd.Run(); err != nil {
			return fmt.Errorf("failed to start existing container: %w", err)
		}
		return nil
	}

	// Create network if needed
	if m.cfg.Network.UseBridgeNetwork {
		networkCmd := exec.CommandContext(ctx, runtime, "network", "create", m.cfg.Network.BridgeNetworkName)
		networkCmd.Run() // Ignore error if network already exists
	}

	// Build run command
	args := []string{"run", "-d", "--name", m.cfg.ContainerName}

	// Network
	if m.cfg.Network.UseBridgeNetwork {
		args = append(args, "--network", m.cfg.Network.BridgeNetworkName)
	}

	// Don't expose ports externally - use Unix socket or localhost only
	if !m.cfg.Network.UseUnixSocket {
		args = append(args, "-p", fmt.Sprintf("127.0.0.1:%d:%d", m.cfg.Port, m.cfg.Port))
	}

	// Volumes - Convert to absolute paths for Docker/Podman compatibility
	dataPath, err := filepath.Abs(filepath.Join(m.cfg.DataDir, "data"))
	if err != nil {
		return fmt.Errorf("failed to get absolute path for data directory: %w", err)
	}

	args = append(args, "-v", fmt.Sprintf("%s:/var/lib/postgresql/data", dataPath))
	// Config volume mount disabled - using PostgreSQL defaults for now
	// Future: Re-enable after fixing postgresql.conf generation

	if m.cfg.TLS.Enabled {
		certPath, err := filepath.Abs(m.cfg.TLS.CertDir)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for cert directory: %w", err)
		}
		args = append(args, "-v", fmt.Sprintf("%s:/var/lib/postgresql/certs:ro", certPath))
	}

	if m.cfg.Network.UseUnixSocket {
		runDir := filepath.Join(m.cfg.DataDir, "run")
		os.MkdirAll(runDir, 0755)
		runPath, err := filepath.Abs(runDir)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for run directory: %w", err)
		}
		args = append(args, "-v", fmt.Sprintf("%s:/var/run/postgresql", runPath))
	}

	// Resource limits
	if m.cfg.Resources.MemoryMB > 0 {
		args = append(args, "--memory", fmt.Sprintf("%dm", m.cfg.Resources.MemoryMB))
	}
	if m.cfg.Resources.CPUCores > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%.2f", m.cfg.Resources.CPUCores))
	}

	// Environment
	args = append(args, "-e", fmt.Sprintf("POSTGRES_PASSWORD=%s", m.credentials.SuperuserPassword))
	args = append(args, "-e", "POSTGRES_DB=bibd")
	// Removed SCRAM auth forcing - using PostgreSQL defaults
	// args = append(args, "-e", "POSTGRES_INITDB_ARGS=--auth-host=scram-sha-256 --auth-local=scram-sha-256")

	// Health check
	args = append(args, "--health-cmd", "pg_isready -U postgres")
	args = append(args, "--health-interval", m.cfg.Health.Interval.String())
	args = append(args, "--health-timeout", m.cfg.Health.Timeout.String())

	// Restart policy
	args = append(args, "--restart", "unless-stopped")

	// Image
	args = append(args, m.cfg.Image)

	cmd := exec.CommandContext(ctx, runtime, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start container: %w\nOutput: %s", err, string(output))
	}

	m.containerID = strings.TrimSpace(string(output))
	return nil
}

// startKubernetes starts PostgreSQL in Kubernetes.
func (m *Manager) startKubernetes(ctx context.Context) error {
	// Create Kubernetes manager
	k8sMgr, err := NewKubernetesManager(m.cfg, m.nodeID, m.credentials)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes manager: %w", err)
	}

	// Validate prerequisites
	if err := k8sMgr.ValidatePrerequisites(ctx); err != nil {
		return fmt.Errorf("Kubernetes prerequisites validation failed: %w", err)
	}

	// Deploy PostgreSQL
	if err := k8sMgr.Deploy(ctx); err != nil {
		return fmt.Errorf("failed to deploy PostgreSQL to Kubernetes: %w", err)
	}

	// Store manager for later use
	m.k8sManager = k8sMgr

	return nil
}

// waitForReady waits for PostgreSQL to be ready.
func (m *Manager) waitForReady(ctx context.Context) error {
	deadline := time.Now().Add(m.cfg.Health.StartupTimeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if m.checkHealth(ctx) {
			// Add a small delay to ensure PostgreSQL is fully ready to accept connections
			// pg_isready can return true slightly before PostgreSQL is fully initialized
			time.Sleep(2 * time.Second)
			return nil
		}

		time.Sleep(time.Second)
	}

	return fmt.Errorf("PostgreSQL did not become ready within %s", m.cfg.Health.StartupTimeout)
}

// checkHealth checks if PostgreSQL is healthy.
func (m *Manager) checkHealth(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, m.cfg.Health.Timeout)
	defer cancel()

	var cmd *exec.Cmd
	switch m.runtime {
	case RuntimeDocker:
		cmd = exec.CommandContext(ctx, "docker", "exec", m.cfg.ContainerName, "pg_isready", "-U", "postgres")
	case RuntimePodman:
		cmd = exec.CommandContext(ctx, "podman", "exec", m.cfg.ContainerName, "pg_isready", "-U", "postgres")
	default:
		// For manual mode, try to connect directly
		// TODO: Implement direct connection check
		return true
	}

	return cmd.Run() == nil
}

// initializeRoles creates database roles.
func (m *Manager) initializeRoles(ctx context.Context) error {
	// TODO: Connect to PostgreSQL and create roles
	// This requires the store to be connected first
	return nil
}

// healthCheckLoop runs periodic health checks.
func (m *Manager) healthCheckLoop() {
	ticker := time.NewTicker(m.cfg.Health.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.shutdownCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), m.cfg.Health.Timeout)
			healthy := m.checkHealth(ctx)
			cancel()

			m.mu.Lock()
			if healthy {
				m.lastHealth = time.Now()
				m.healthErrors = 0
			} else {
				m.healthErrors++
				m.handleHealthFailure()
			}
			m.mu.Unlock()
		}
	}
}

// handleHealthFailure handles health check failures.
func (m *Manager) handleHealthFailure() {
	switch m.cfg.Health.Action {
	case HealthActionShutdown:
		// Signal shutdown
		// TODO: Implement proper shutdown signaling
		fmt.Printf("PostgreSQL health check failed %d times, shutting down\n", m.healthErrors)

	case HealthActionRetryLimit:
		if m.healthErrors >= m.cfg.Health.MaxRetries {
			fmt.Printf("PostgreSQL health check failed %d times (max: %d), giving up\n",
				m.healthErrors, m.cfg.Health.MaxRetries)
			// TODO: Signal shutdown
		} else {
			fmt.Printf("PostgreSQL health check failed, retry %d/%d\n",
				m.healthErrors, m.cfg.Health.MaxRetries)
			m.restartContainer()
		}

	case HealthActionRetryAlways:
		fmt.Printf("PostgreSQL health check failed, retrying (attempt %d)\n", m.healthErrors)
		m.restartContainer()
	}
}

// restartContainer attempts to restart the container.
func (m *Manager) restartContainer() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	switch m.runtime {
	case RuntimeDocker:
		cmd = exec.CommandContext(ctx, "docker", "restart", m.cfg.ContainerName)
	case RuntimePodman:
		cmd = exec.CommandContext(ctx, "podman", "restart", m.cfg.ContainerName)
	default:
		return
	}

	if err := cmd.Run(); err != nil {
		fmt.Printf("Failed to restart container: %v\n", err)
	}
}

// Stop stops the PostgreSQL instance.
func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	close(m.shutdownCh)
	m.ready = false

	switch m.runtime {
	case RuntimeDocker:
		cmd := exec.CommandContext(ctx, "docker", "stop", m.cfg.ContainerName)
		return cmd.Run()
	case RuntimePodman:
		cmd := exec.CommandContext(ctx, "podman", "stop", m.cfg.ContainerName)
		return cmd.Run()
	case RuntimeKubernetes:
		if m.k8sManager != nil {
			return m.k8sManager.Cleanup(ctx)
		}
		return nil
	case RuntimeManual:
		// Nothing to stop
		return nil
	}

	return nil
}

// IsReady returns whether PostgreSQL is ready.
func (m *Manager) IsReady() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ready
}

// Runtime returns the detected runtime type.
func (m *Manager) Runtime() RuntimeType {
	return m.runtime
}

// ConnectionString returns the connection string for the store.
func (m *Manager) ConnectionString() string {
	// For Kubernetes, get connection info from the manager
	if m.runtime == RuntimeKubernetes && m.k8sManager != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		host, port, err := m.k8sManager.GetConnectionInfo(ctx)
		if err != nil {
			// Fallback to service name
			host = fmt.Sprintf("bibd-postgres-%s.%s.svc.cluster.local", m.nodeID[:8], m.k8sManager.namespace)
			port = 5432
		}

		sslmode := "disable"
		if m.cfg.TLS.Enabled {
			sslmode = "require"
		}

		return fmt.Sprintf("host=%s port=%d user=postgres password=%s dbname=bibd sslmode=%s",
			host,
			port,
			m.credentials.SuperuserPassword,
			sslmode,
		)
	}

	// For Docker/Podman
	if m.cfg.Network.UseUnixSocket {
		return fmt.Sprintf("host=%s port=%d user=postgres password=%s dbname=bibd sslmode=disable",
			filepath.Join(m.cfg.DataDir, "run"),
			m.cfg.Port,
			m.credentials.SuperuserPassword,
		)
	}

	sslmode := "disable"
	if m.cfg.TLS.Enabled {
		sslmode = "require"
	}

	return fmt.Sprintf("host=%s port=%d user=postgres password=%s dbname=bibd sslmode=%s",
		m.cfg.Network.BindAddress,
		m.cfg.Port,
		m.credentials.SuperuserPassword,
		sslmode,
	)
}

// Credentials returns the current credentials.
func (m *Manager) Credentials() *Credentials {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.credentials
}
