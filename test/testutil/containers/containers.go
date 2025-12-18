// Package containers provides Docker container management for testing.
package containers

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// Container represents a running Docker container.
type Container struct {
	ID       string
	Name     string
	Image    string
	Ports    map[int]int // container port -> host port
	Env      map[string]string
	Networks []string
	mu       sync.Mutex
	stopped  bool
}

// ContainerConfig holds configuration for starting a container.
type ContainerConfig struct {
	Image      string
	Name       string
	Env        map[string]string
	Ports      []int // Container ports to expose
	Cmd        []string
	Entrypoint []string
	Networks   []string
	Volumes    map[string]string // host path -> container path
	HealthCmd  []string          // Command to check container health
	WaitFor    time.Duration     // How long to wait for container to be ready
}

// Manager manages Docker containers for testing.
type Manager struct {
	containers []*Container
	mu         sync.Mutex
	t          testing.TB
	network    string
}

// NewManager creates a new container manager for testing.
func NewManager(t testing.TB) *Manager {
	m := &Manager{
		t:          t,
		containers: make([]*Container, 0),
	}
	t.Cleanup(m.Cleanup)
	return m
}

// checkDocker verifies Docker is available.
func checkDocker() error {
	cmd := exec.Command("docker", "info")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker not available: %w", err)
	}
	return nil
}

// Start starts a new container with the given configuration.
func (m *Manager) Start(ctx context.Context, cfg ContainerConfig) (*Container, error) {
	if err := checkDocker(); err != nil {
		m.t.Skip("Docker not available:", err)
		return nil, err
	}

	// Build docker run command
	args := []string{"run", "-d", "--rm"}

	// Add name
	containerName := cfg.Name
	if containerName == "" {
		containerName = fmt.Sprintf("bib-test-%d", time.Now().UnixNano())
	}
	args = append(args, "--name", containerName)

	// Add environment variables
	for k, v := range cfg.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	// Add port mappings
	portMap := make(map[int]int)
	for _, p := range cfg.Ports {
		hostPort := getFreePort()
		args = append(args, "-p", fmt.Sprintf("%d:%d", hostPort, p))
		portMap[p] = hostPort
	}

	// Add volumes
	for hostPath, containerPath := range cfg.Volumes {
		args = append(args, "-v", fmt.Sprintf("%s:%s", hostPath, containerPath))
	}

	// Add networks
	for _, n := range cfg.Networks {
		args = append(args, "--network", n)
	}

	// Add entrypoint
	if len(cfg.Entrypoint) > 0 {
		args = append(args, "--entrypoint", cfg.Entrypoint[0])
	}

	// Add image
	args = append(args, cfg.Image)

	// Add command
	if len(cfg.Cmd) > 0 {
		args = append(args, cfg.Cmd...)
	}
	if len(cfg.Entrypoint) > 1 {
		args = append(args, cfg.Entrypoint[1:]...)
	}

	// Pull image if needed
	pullCmd := exec.CommandContext(ctx, "docker", "pull", cfg.Image)
	pullCmd.Stdout = io.Discard
	pullCmd.Stderr = io.Discard
	_ = pullCmd.Run() // Ignore error, run will fail if image doesn't exist

	// Start container
	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %w, output: %s", err, output)
	}

	containerID := strings.TrimSpace(string(output))

	container := &Container{
		ID:       containerID,
		Name:     containerName,
		Image:    cfg.Image,
		Ports:    portMap,
		Env:      cfg.Env,
		Networks: cfg.Networks,
	}

	m.mu.Lock()
	m.containers = append(m.containers, container)
	m.mu.Unlock()

	// Wait for container to be ready
	waitTime := cfg.WaitFor
	if waitTime == 0 {
		waitTime = 30 * time.Second
	}

	if len(cfg.HealthCmd) > 0 {
		if err := m.waitForHealth(ctx, container, cfg.HealthCmd, waitTime); err != nil {
			m.Stop(container)
			return nil, fmt.Errorf("container health check failed: %w", err)
		}
	} else {
		// Default: wait for container to be running
		if err := m.waitForRunning(ctx, container, waitTime); err != nil {
			m.Stop(container)
			return nil, fmt.Errorf("container failed to start: %w", err)
		}
	}

	return container, nil
}

// waitForRunning waits for a container to be in running state.
func (m *Manager) waitForRunning(ctx context.Context, c *Container, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		cmd := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.State.Running}}", c.ID)
		output, err := cmd.Output()
		if err == nil && strings.TrimSpace(string(output)) == "true" {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return fmt.Errorf("timeout waiting for container to start")
}

// waitForHealth waits for a container to pass health check.
func (m *Manager) waitForHealth(ctx context.Context, c *Container, healthCmd []string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		args := append([]string{"exec", c.ID}, healthCmd...)
		cmd := exec.CommandContext(ctx, "docker", args...)
		if err := cmd.Run(); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return fmt.Errorf("timeout waiting for container health check")
}

// Stop stops a running container.
func (m *Manager) Stop(c *Container) error {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return nil
	}
	c.stopped = true
	c.mu.Unlock()

	cmd := exec.Command("docker", "stop", "-t", "5", c.ID)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

// Logs returns the logs from a container.
func (m *Manager) Logs(c *Container) (string, error) {
	cmd := exec.Command("docker", "logs", c.ID)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// Exec executes a command in a running container.
func (m *Manager) Exec(ctx context.Context, c *Container, cmd []string) (string, error) {
	args := append([]string{"exec", c.ID}, cmd...)
	execCmd := exec.CommandContext(ctx, "docker", args...)
	output, err := execCmd.CombinedOutput()
	return string(output), err
}

// Cleanup stops and removes all containers managed by this manager.
func (m *Manager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	keepContainers := os.Getenv("TEST_KEEP_CONTAINERS") == "true"

	for _, c := range m.containers {
		if keepContainers && m.t.Failed() {
			m.t.Logf("keeping container %s for debugging", c.Name)
			continue
		}
		if err := m.Stop(c); err != nil {
			m.t.Logf("failed to stop container %s: %v", c.Name, err)
		}
	}
	m.containers = nil

	if m.network != "" && !keepContainers {
		cmd := exec.Command("docker", "network", "rm", m.network)
		_ = cmd.Run()
	}
}

// CreateNetwork creates a Docker network for the test.
func (m *Manager) CreateNetwork(ctx context.Context, name string) (string, error) {
	if err := checkDocker(); err != nil {
		return "", err
	}

	networkName := fmt.Sprintf("bib-test-%s-%d", name, time.Now().UnixNano())
	cmd := exec.CommandContext(ctx, "docker", "network", "create", networkName)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to create network: %w", err)
	}

	m.network = networkName
	return networkName, nil
}

// HostPort returns the host port mapped to a container port.
func (c *Container) HostPort(containerPort int) int {
	return c.Ports[containerPort]
}

// Address returns the address to connect to a container port.
func (c *Container) Address(containerPort int) string {
	return fmt.Sprintf("localhost:%d", c.Ports[containerPort])
}

// getFreePort finds an available port on the host.
func getFreePort() int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0
	}
	defer l.Close()
	addr := l.Addr().(*net.TCPAddr)
	return addr.Port
}

// GetFreePort exports getFreePort for testing.
func GetFreePort() int {
	return getFreePort()
}

// PostgresConfig holds configuration for a PostgreSQL container.
type PostgresConfig struct {
	Image    string
	Database string
	User     string
	Password string
}

// DefaultPostgresConfig returns the default PostgreSQL configuration.
func DefaultPostgresConfig() PostgresConfig {
	return PostgresConfig{
		Image:    GetEnvOrDefault("TEST_POSTGRES_IMAGE", "postgres:16-alpine"),
		Database: "bib_test",
		User:     "bib_test",
		Password: "bib_test_password",
	}
}

// GetEnvOrDefault returns the environment variable value or default.
func GetEnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// StartPostgres starts a PostgreSQL container with the given configuration.
func (m *Manager) StartPostgres(ctx context.Context, cfg PostgresConfig) (*Container, error) {
	return m.Start(ctx, ContainerConfig{
		Image: cfg.Image,
		Env: map[string]string{
			"POSTGRES_DB":       cfg.Database,
			"POSTGRES_USER":     cfg.User,
			"POSTGRES_PASSWORD": cfg.Password,
		},
		Ports: []int{5432},
		HealthCmd: []string{
			"pg_isready", "-U", cfg.User, "-d", cfg.Database,
		},
		WaitFor: 60 * time.Second,
	})
}

// PostgresConnectionString returns a connection string for a PostgreSQL container.
func PostgresConnectionString(c *Container, cfg PostgresConfig) string {
	return fmt.Sprintf(
		"host=localhost port=%d dbname=%s user=%s password=%s sslmode=disable",
		c.HostPort(5432),
		cfg.Database,
		cfg.User,
		cfg.Password,
	)
}

// PostgresDSN returns a DSN for a PostgreSQL container.
func PostgresDSN(c *Container, cfg PostgresConfig) string {
	return fmt.Sprintf(
		"postgres://%s:%s@localhost:%d/%s?sslmode=disable",
		cfg.User,
		cfg.Password,
		c.HostPort(5432),
		cfg.Database,
	)
}

// WaitForPort waits for a port to be available.
func WaitForPort(address string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for port %s", address)
}

// ParsePort parses a port from an address string.
func ParsePort(address string) (int, error) {
	_, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(portStr)
}
