package postgres

import (
	"strings"
	"testing"
)

func TestNewConfigGenerator_Defaults(t *testing.T) {
	gen := NewConfigGenerator(ConfigGeneratorOptions{
		NodeID: "test-node",
	})

	if gen.nodeID != "test-node" {
		t.Errorf("expected node ID 'test-node', got %q", gen.nodeID)
	}
	if gen.port != 5432 {
		t.Errorf("expected port 5432, got %d", gen.port)
	}
	if gen.maxConnections != 100 {
		t.Errorf("expected maxConnections 100, got %d", gen.maxConnections)
	}
	if gen.sharedBuffers != "256MB" {
		t.Errorf("expected sharedBuffers '256MB', got %q", gen.sharedBuffers)
	}
}

func TestNewConfigGenerator_CustomOptions(t *testing.T) {
	gen := NewConfigGenerator(ConfigGeneratorOptions{
		NodeID:            "custom-node",
		DataDir:           "/data/postgres",
		ConfigDir:         "/config",
		CertDir:           "/certs",
		Port:              5433,
		MaxConnections:    200,
		SharedBuffers:     "512MB",
		UseUnixSocket:     true,
		SocketDir:         "/var/run/postgresql",
		RequireClientCert: true,
		DockerSubnet:      "172.18.0.0/16",
		DockerNetwork:     "bib-network",
	})

	if gen.nodeID != "custom-node" {
		t.Errorf("expected node ID 'custom-node', got %q", gen.nodeID)
	}
	if gen.dataDir != "/data/postgres" {
		t.Errorf("expected dataDir '/data/postgres', got %q", gen.dataDir)
	}
	if gen.configDir != "/config" {
		t.Errorf("expected configDir '/config', got %q", gen.configDir)
	}
	if gen.certDir != "/certs" {
		t.Errorf("expected certDir '/certs', got %q", gen.certDir)
	}
	if gen.port != 5433 {
		t.Errorf("expected port 5433, got %d", gen.port)
	}
	if gen.maxConnections != 200 {
		t.Errorf("expected maxConnections 200, got %d", gen.maxConnections)
	}
	if gen.sharedBuffers != "512MB" {
		t.Errorf("expected sharedBuffers '512MB', got %q", gen.sharedBuffers)
	}
	if !gen.useUnixSocket {
		t.Error("expected useUnixSocket true")
	}
	if gen.socketDir != "/var/run/postgresql" {
		t.Errorf("expected socketDir '/var/run/postgresql', got %q", gen.socketDir)
	}
	if !gen.requireClientCert {
		t.Error("expected requireClientCert true")
	}
	if gen.dockerSubnet != "172.18.0.0/16" {
		t.Errorf("expected dockerSubnet '172.18.0.0/16', got %q", gen.dockerSubnet)
	}
	if gen.dockerNetwork != "bib-network" {
		t.Errorf("expected dockerNetwork 'bib-network', got %q", gen.dockerNetwork)
	}
}

func TestGeneratePostgresConf(t *testing.T) {
	gen := NewConfigGenerator(ConfigGeneratorOptions{
		NodeID:         "test-node",
		Port:           5432,
		MaxConnections: 100,
		SharedBuffers:  "256MB",
	})

	conf := gen.GeneratePostgresConf()

	// Check that essential configuration is present
	if !strings.Contains(conf, "# PostgreSQL Configuration") {
		t.Error("expected header comment")
	}
	if !strings.Contains(conf, "Node ID: test-node") {
		t.Error("expected node ID in comment")
	}
	if !strings.Contains(conf, "listen_addresses = '*'") {
		t.Error("expected listen_addresses")
	}
	if !strings.Contains(conf, "port = 5432") {
		t.Error("expected port setting")
	}
	if !strings.Contains(conf, "max_connections = 100") {
		t.Error("expected max_connections setting")
	}
	if !strings.Contains(conf, "ssl = on") {
		t.Error("expected SSL enabled")
	}
	if !strings.Contains(conf, "ssl_min_protocol_version = 'TLSv1.3'") {
		t.Error("expected TLS 1.3 minimum")
	}
}

func TestGeneratePostgresConf_UnixSocket(t *testing.T) {
	gen := NewConfigGenerator(ConfigGeneratorOptions{
		NodeID:        "test-node",
		UseUnixSocket: true,
		SocketDir:     "/var/run/postgresql",
	})

	conf := gen.GeneratePostgresConf()

	if !strings.Contains(conf, "listen_addresses = ''") {
		t.Error("expected empty listen_addresses for unix socket")
	}
	if !strings.Contains(conf, "unix_socket_directories = '/var/run/postgresql'") {
		t.Error("expected unix_socket_directories")
	}
}

func TestConfigGeneratorOptions_ZeroValues(t *testing.T) {
	opts := ConfigGeneratorOptions{}

	// Zero values should be defaults
	gen := NewConfigGenerator(opts)

	if gen.port != 5432 {
		t.Errorf("expected default port 5432, got %d", gen.port)
	}
	if gen.maxConnections != 100 {
		t.Errorf("expected default maxConnections 100, got %d", gen.maxConnections)
	}
	if gen.sharedBuffers != "256MB" {
		t.Errorf("expected default sharedBuffers '256MB', got %q", gen.sharedBuffers)
	}
}
