//go:build e2e

// Package scenarios contains end-to-end test scenarios.
package scenarios

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"bib/test/testutil"
	"bib/test/testutil/containers"
	"bib/test/testutil/fixtures"
	"bib/test/testutil/helpers"
)

var (
	bibBinary  string
	bibdBinary string
)

func init() {
	// Binaries should be built by the parent e2e package TestMain
	// or by the Makefile
	bibBinary = os.Getenv("BIB_BINARY")
	bibdBinary = os.Getenv("BIBD_BINARY")

	if bibBinary == "" || bibdBinary == "" {
		// Try to find built binaries
		projectRoot := findProjectRoot()
		if projectRoot != "" {
			bibBinary = filepath.Join(projectRoot, "bin", "bib")
			bibdBinary = filepath.Join(projectRoot, "bin", "bibd")
		}
	}
}

func findProjectRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func ensureBinariesBuilt(t *testing.T) {
	t.Helper()

	if bibBinary == "" || bibdBinary == "" {
		projectRoot := findProjectRoot()
		if projectRoot == "" {
			t.Fatal("could not find project root")
		}

		tmpDir := t.TempDir()

		// Build bib
		bibBinary = filepath.Join(tmpDir, "bib")
		cmd := exec.Command("go", "build", "-o", bibBinary, "./cmd/bib")
		cmd.Dir = projectRoot
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to build bib: %s", output)
		}

		// Build bibd
		bibdBinary = filepath.Join(tmpDir, "bibd")
		cmd = exec.Command("go", "build", "-o", bibdBinary, "./cmd/bibd")
		cmd.Dir = projectRoot
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to build bibd: %s", output)
		}
	}
}

// TestScenario_SingleNodeSetup tests a complete single-node setup scenario.
func TestScenario_SingleNodeSetup(t *testing.T) {
	testutil.SkipIfShort(t)
	ensureBinariesBuilt(t)

	ctx := testutil.TestContext(t)
	dataDir := testutil.TempDir(t, "single-node")

	// Create config
	cfg := fixtures.BibdConfigSQLite(dataDir)
	configPath := fixtures.WriteConfigFile(t, dataDir, cfg)

	// Start daemon
	daemon := helpers.StartDaemon(t, bibdBinary, configPath, dataDir)
	if err := daemon.WaitForReady(30 * time.Second); err != nil {
		t.Fatalf("daemon failed to start: %v\nLogs:\n%s", err, daemon.Logs())
	}

	t.Log("Single-node daemon started successfully")

	// Run some CLI commands against the daemon
	runner := helpers.NewBinaryRunner(t, bibBinary)

	// Check version
	output, err := runner.Run(ctx, "version")
	if err != nil {
		t.Errorf("version command failed: %v", err)
	}
	t.Logf("Version: %s", output)

	daemon.Stop()
	t.Log("Single-node scenario completed")
}

// TestScenario_PostgresBackend tests a complete setup with PostgreSQL backend.
func TestScenario_PostgresBackend(t *testing.T) {
	testutil.SkipIfShort(t)
	ensureBinariesBuilt(t)

	ctx := testutil.TestContext(t)

	// Start PostgreSQL
	cm := containers.NewManager(t)
	pgCfg := containers.DefaultPostgresConfig()
	pgContainer, err := cm.StartPostgres(ctx, pgCfg)
	if err != nil {
		t.Fatalf("failed to start postgres: %v", err)
	}

	dataDir := testutil.TempDir(t, "postgres-backend")

	// Create config with PostgreSQL
	cfg := fixtures.BibdConfig(dataDir, pgContainer.HostPort(5432))
	configPath := fixtures.WriteConfigFile(t, dataDir, cfg)

	// Start daemon
	daemon := helpers.StartDaemon(t, bibdBinary, configPath, dataDir)
	if err := daemon.WaitForReady(60 * time.Second); err != nil {
		t.Fatalf("daemon failed to start: %v\nLogs:\n%s", err, daemon.Logs())
	}

	t.Log("Daemon with PostgreSQL backend started successfully")

	daemon.Stop()
	t.Log("PostgreSQL backend scenario completed")
}

// TestScenario_MultiNodeCluster tests a multi-node cluster setup.
func TestScenario_MultiNodeCluster(t *testing.T) {
	testutil.SkipIfShort(t)
	ensureBinariesBuilt(t)

	ctx := testutil.TestContext(t)
	const numNodes = 3

	dataDirs := make([]string, numNodes)
	daemons := make([]*helpers.DaemonProcess, numNodes)
	ports := helpers.GetFreePorts(t, numNodes*2) // P2P and Raft ports

	// Create and start nodes
	for i := 0; i < numNodes; i++ {
		dataDir := testutil.TempDir(t, fmt.Sprintf("cluster-node%d", i))
		dataDirs[i] = dataDir

		// Create config with clustering enabled
		cfg := fixtures.BibdConfigCluster(dataDir, fmt.Sprintf("node-%d", i), ports[i*2+1])
		if i > 0 {
			// Join to first node
			cfg.Cluster.JoinAddrs = []string{fmt.Sprintf("127.0.0.1:%d", ports[1])}
		}
		configPath := fixtures.WriteConfigFile(t, dataDir, cfg)

		daemon := helpers.StartDaemon(t, bibdBinary, configPath, dataDir)
		daemons[i] = daemon
	}

	// Wait for all nodes to be ready
	for i, daemon := range daemons {
		if err := daemon.WaitForReady(60 * time.Second); err != nil {
			t.Fatalf("node %d failed to start: %v\nLogs:\n%s", i, err, daemon.Logs())
		}
		t.Logf("Node %d started", i)
	}

	// Wait for cluster to stabilize
	time.Sleep(10 * time.Second)

	t.Log("Multi-node cluster started successfully")

	// Stop all nodes
	for i, daemon := range daemons {
		daemon.Stop()
		t.Logf("Node %d stopped", i)
	}

	t.Log("Multi-node cluster scenario completed")
}

// TestScenario_DataPipeline tests a complete data pipeline workflow.
func TestScenario_DataPipeline(t *testing.T) {
	testutil.SkipIfShort(t)
	ensureBinariesBuilt(t)

	ctx := testutil.TestContext(t)

	// Start PostgreSQL for the authoritative store
	cm := containers.NewManager(t)
	pgCfg := containers.DefaultPostgresConfig()
	pgContainer, err := cm.StartPostgres(ctx, pgCfg)
	if err != nil {
		t.Fatalf("failed to start postgres: %v", err)
	}

	dataDir := testutil.TempDir(t, "data-pipeline")

	// Create config
	cfg := fixtures.BibdConfig(dataDir, pgContainer.HostPort(5432))
	configPath := fixtures.WriteConfigFile(t, dataDir, cfg)

	// Start daemon
	daemon := helpers.StartDaemon(t, bibdBinary, configPath, dataDir)
	if err := daemon.WaitForReady(60 * time.Second); err != nil {
		t.Fatalf("daemon failed to start: %v\nLogs:\n%s", err, daemon.Logs())
	}

	t.Log("Data pipeline daemon started")

	// This is where you would add CLI commands to:
	// 1. Create a topic
	// 2. Create a dataset
	// 3. Submit a job
	// 4. Query data
	// For now, we just verify the daemon is running

	daemon.Stop()
	t.Log("Data pipeline scenario completed")
}
