// Package fixtures provides test data and configuration fixtures.
package fixtures

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bib/internal/config"
	"bib/internal/domain"

	"gopkg.in/yaml.v3"
)

// TestTopic returns a sample topic for testing.
func TestTopic(id string) *domain.Topic {
	now := time.Now()
	return &domain.Topic{
		ID:          domain.TopicID(id),
		Name:        "Test Topic " + id,
		Description: "A test topic for integration testing",
		Status:      domain.TopicStatusActive,
		Owners:      []domain.UserID{"test-user"},
		Tags:        []string{"test", "integration"},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// TestDataset returns a sample dataset for testing.
func TestDataset(id, topicID string) *domain.Dataset {
	now := time.Now()
	return &domain.Dataset{
		ID:          domain.DatasetID(id),
		TopicID:     domain.TopicID(topicID),
		Name:        "Test Dataset " + id,
		Description: "A test dataset for integration testing",
		Status:      domain.DatasetStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// TestJob returns a sample job for testing.
func TestJob(id, topicID string) *domain.Job {
	now := time.Now()
	return &domain.Job{
		ID:        domain.JobID(id),
		TopicID:   domain.TopicID(topicID),
		Type:      domain.JobTypeScrape,
		Status:    domain.JobStatusPending,
		CreatedBy: "test-user",
		CreatedAt: now,
	}
}

// TestUser returns a sample user for testing.
func TestUser(id string) *domain.User {
	now := time.Now()
	_, privKey, _ := ed25519.GenerateKey(rand.Reader)
	pubKey := privKey.Public().(ed25519.PublicKey)
	return &domain.User{
		ID:        domain.UserIDFromPublicKey(pubKey),
		PublicKey: pubKey,
		Name:      "Test User " + id,
		Email:     id + "@test.local",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// BibdConfig returns a test bibd configuration.
func BibdConfig(dataDir string, pgPort int) *config.BibdConfig {
	return &config.BibdConfig{
		Server: config.ServerConfig{
			DataDir: dataDir,
			PIDFile: filepath.Join(dataDir, "bibd.pid"),
		},
		P2P: config.P2PConfig{
			Enabled:         false, // Disable for most tests
			ListenAddresses: []string{"/ip4/127.0.0.1/tcp/0"},
		},
		Cluster: config.ClusterConfig{
			Enabled: false, // Disable for most tests
		},
		Database: config.DatabaseConfig{
			Backend: "postgres",
			Postgres: config.PostgresDatabaseConfig{
				Managed: false,
				Advanced: &config.PostgresAdvancedConfig{
					Host:     "localhost",
					Port:     pgPort,
					Database: "bib_test",
					User:     "bib_test",
					Password: "bib_test_password",
					SSLMode:  "disable",
				},
				MaxConnections: 10,
			},
		},
		Log: config.LogConfig{
			Level:  "debug",
			Format: "text",
		},
	}
}

// BibdConfigSQLite returns a test bibd configuration using SQLite.
func BibdConfigSQLite(dataDir string) *config.BibdConfig {
	return &config.BibdConfig{
		Server: config.ServerConfig{
			DataDir: dataDir,
			PIDFile: filepath.Join(dataDir, "bibd.pid"),
		},
		P2P: config.P2PConfig{
			Enabled:         false,
			ListenAddresses: []string{"/ip4/127.0.0.1/tcp/0"},
		},
		Cluster: config.ClusterConfig{
			Enabled: false,
		},
		Database: config.DatabaseConfig{
			Backend: "sqlite",
			SQLite: config.SQLiteDatabaseConfig{
				Path: filepath.Join(dataDir, "bib.db"),
			},
		},
		Log: config.LogConfig{
			Level:  "debug",
			Format: "text",
		},
	}
}

// BibdConfigP2P returns a test bibd configuration with P2P enabled.
func BibdConfigP2P(dataDir string, listenPort int) *config.BibdConfig {
	cfg := BibdConfigSQLite(dataDir)
	cfg.P2P = config.P2PConfig{
		Enabled:         true,
		ListenAddresses: []string{fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", listenPort)},
		Mode:            "full",
		MDNS: config.MDNSConfig{
			Enabled: false, // Disable for tests
		},
		Bootstrap: config.BootstrapConfig{
			Peers: []string{}, // Empty for tests - we'll inject peers manually
		},
	}
	return cfg
}

// BibdConfigCluster returns a test bibd configuration with clustering enabled.
func BibdConfigCluster(dataDir string, nodeID string, raftPort int) *config.BibdConfig {
	cfg := BibdConfigSQLite(dataDir)
	cfg.Cluster = config.ClusterConfig{
		Enabled:     true,
		ClusterName: "test-cluster",
		NodeID:      nodeID,
		ListenAddr:  fmt.Sprintf("127.0.0.1:%d", raftPort),
		Raft: config.RaftConfig{
			HeartbeatTimeout: 500 * time.Millisecond,
			ElectionTimeout:  2 * time.Second,
		},
		Snapshot: config.SnapshotConfig{
			Interval: 30 * time.Second,
		},
	}
	return cfg
}

// WriteConfigFile writes a config to a YAML file.
func WriteConfigFile(t testing.TB, dir string, cfg interface{}) string {
	t.Helper()
	path := filepath.Join(dir, "config.yaml")
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	return path
}

// SampleConfigYAML returns a sample configuration YAML for testing.
func SampleConfigYAML() string {
	return `server:
  data_dir: /tmp/bib-test
  pid_file: /tmp/bib-test/bibd.pid

database:
  backend: sqlite
  sqlite:
    path: /tmp/bib-test/bib.db

p2p:
  enabled: false

cluster:
  enabled: false

log:
  level: debug
  format: text
`
}
