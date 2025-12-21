package discovery

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/hashicorp/mdns"
)

// MDNSServer manages mDNS service registration for a bibd node
type MDNSServer struct {
	// Config is the server configuration
	Config MDNSServerConfig

	// server is the underlying mDNS server
	server *mdns.Server

	// running indicates if the server is running
	running bool

	// mu protects the server state
	mu sync.RWMutex

	// shutdown channel
	shutdownCh chan struct{}
}

// MDNSServerConfig contains configuration for the mDNS server
type MDNSServerConfig struct {
	// InstanceName is the unique instance name for this node
	InstanceName string

	// Port is the bibd API port
	Port int

	// Host is the hostname to advertise
	Host string

	// NodeName is the display name of this node
	NodeName string

	// Version is the bibd version
	Version string

	// PeerID is the P2P peer ID
	PeerID string

	// Mode is the P2P mode (proxy, selective, full)
	Mode string

	// IPs are the IP addresses to advertise (optional, auto-detected if empty)
	IPs []net.IP
}

// DefaultMDNSServerConfig returns a default mDNS server configuration
func DefaultMDNSServerConfig() MDNSServerConfig {
	hostname, _ := os.Hostname()
	return MDNSServerConfig{
		InstanceName: hostname,
		Port:         4000,
		Host:         hostname,
		NodeName:     "",
		Version:      "unknown",
		Mode:         "proxy",
	}
}

// NewMDNSServer creates a new mDNS server
func NewMDNSServer(config MDNSServerConfig) *MDNSServer {
	return &MDNSServer{
		Config:     config,
		shutdownCh: make(chan struct{}),
	}
}

// Start starts the mDNS server and begins advertising the service
func (s *MDNSServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("mDNS server already running")
	}

	// Get IPs if not provided
	ips := s.Config.IPs
	if len(ips) == 0 {
		var err error
		ips, err = getLocalIPs()
		if err != nil {
			return fmt.Errorf("failed to get local IPs: %w", err)
		}
		if len(ips) == 0 {
			return fmt.Errorf("no local IP addresses found")
		}
	}

	// Build TXT records
	txtRecords := s.buildTXTRecords()

	// Determine hostname
	host := s.Config.Host
	if host == "" || host == "0.0.0.0" {
		host, _ = os.Hostname()
	}

	// Create the mDNS service
	service, err := mdns.NewMDNSService(
		s.Config.InstanceName,
		BibMDNSService,
		BibMDNSDomain,
		host,
		s.Config.Port,
		ips,
		txtRecords,
	)
	if err != nil {
		return fmt.Errorf("failed to create mDNS service: %w", err)
	}

	// Create the server
	server, err := mdns.NewServer(&mdns.Config{Zone: service})
	if err != nil {
		return fmt.Errorf("failed to create mDNS server: %w", err)
	}

	s.server = server
	s.running = true
	s.shutdownCh = make(chan struct{})

	return nil
}

// Stop stops the mDNS server
func (s *MDNSServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	if s.server != nil {
		s.server.Shutdown()
		s.server = nil
	}

	s.running = false
	close(s.shutdownCh)

	return nil
}

// IsRunning returns true if the server is running
func (s *MDNSServer) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// UpdateInfo updates the advertised node info
// This requires restarting the mDNS server
func (s *MDNSServer) UpdateInfo(config MDNSServerConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	wasRunning := s.running

	// Stop if running
	if s.running {
		if s.server != nil {
			s.server.Shutdown()
			s.server = nil
		}
		s.running = false
	}

	// Update config
	s.Config = config

	// Restart if was running
	if wasRunning {
		s.mu.Unlock()
		err := s.Start()
		s.mu.Lock()
		return err
	}

	return nil
}

// buildTXTRecords builds the TXT records for mDNS
func (s *MDNSServer) buildTXTRecords() []string {
	var records []string

	if s.Config.NodeName != "" {
		records = append(records, fmt.Sprintf("name=%s", s.Config.NodeName))
	}

	if s.Config.Version != "" {
		records = append(records, fmt.Sprintf("version=%s", s.Config.Version))
	}

	if s.Config.PeerID != "" {
		records = append(records, fmt.Sprintf("peer_id=%s", s.Config.PeerID))
	}

	if s.Config.Mode != "" {
		records = append(records, fmt.Sprintf("mode=%s", s.Config.Mode))
	}

	return records
}

// GetAdvertisedAddress returns the address being advertised
func (s *MDNSServer) GetAdvertisedAddress() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	host := s.Config.Host
	if host == "" || host == "0.0.0.0" {
		host, _ = os.Hostname()
	}

	return fmt.Sprintf("%s:%d", host, s.Config.Port)
}

// GetTXTRecords returns the TXT records being advertised
func (s *MDNSServer) GetTXTRecords() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.buildTXTRecords()
}

// WaitForShutdown blocks until the server is stopped
func (s *MDNSServer) WaitForShutdown() {
	<-s.shutdownCh
}

// MDNSServerManager manages the lifecycle of an mDNS server
type MDNSServerManager struct {
	server  *MDNSServer
	started bool
	mu      sync.Mutex
}

// NewMDNSServerManager creates a new mDNS server manager
func NewMDNSServerManager() *MDNSServerManager {
	return &MDNSServerManager{}
}

// StartServer starts the mDNS server with the given configuration
func (m *MDNSServerManager) StartServer(config MDNSServerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started && m.server != nil {
		// Stop existing server
		m.server.Stop()
	}

	m.server = NewMDNSServer(config)
	if err := m.server.Start(); err != nil {
		return err
	}

	m.started = true
	return nil
}

// StopServer stops the mDNS server
func (m *MDNSServerManager) StopServer() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started || m.server == nil {
		return nil
	}

	err := m.server.Stop()
	m.started = false
	return err
}

// IsRunning returns true if the server is running
func (m *MDNSServerManager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.started && m.server != nil && m.server.IsRunning()
}

// GetServer returns the underlying server
func (m *MDNSServerManager) GetServer() *MDNSServer {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.server
}

// RunWithContext runs the mDNS server until the context is cancelled
func (m *MDNSServerManager) RunWithContext(ctx context.Context, config MDNSServerConfig) error {
	if err := m.StartServer(config); err != nil {
		return err
	}

	<-ctx.Done()
	return m.StopServer()
}
