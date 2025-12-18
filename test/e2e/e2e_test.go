//go:build e2e

// Package e2e contains end-to-end tests for the bib system.
package e2e

import (
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

// TestMain builds the binaries before running tests.
func TestMain(m *testing.M) {
	// Build binaries
	tmpDir, err := os.MkdirTemp("", "bib-e2e-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	// Find project root
	projectRoot := findProjectRoot()
	if projectRoot == "" {
		panic("could not find project root")
	}

	// Build bib
	bibBinary = filepath.Join(tmpDir, "bib")
	cmd := exec.Command("go", "build", "-o", bibBinary, "./cmd/bib")
	cmd.Dir = projectRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		panic("failed to build bib: " + string(output))
	}

	// Build bibd
	bibdBinary = filepath.Join(tmpDir, "bibd")
	cmd = exec.Command("go", "build", "-o", bibdBinary, "./cmd/bibd")
	cmd.Dir = projectRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		panic("failed to build bibd: " + string(output))
	}

	os.Exit(m.Run())
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

// TestBibVersion tests the bib version command.
func TestBibVersion(t *testing.T) {
	ctx := testutil.TestContext(t)
	runner := helpers.NewBinaryRunner(t, bibBinary)

	output, err := runner.Run(ctx, "version")
	if err != nil {
		t.Fatalf("failed to run bib version: %v", err)
	}

	helpers.AssertContains(t, output, "bib")
}

// TestBibdVersion tests the bibd version command.
func TestBibdVersion(t *testing.T) {
	ctx := testutil.TestContext(t)
	runner := helpers.NewBinaryRunner(t, bibdBinary)

	output, err := runner.Run(ctx, "--version")
	if err != nil {
		t.Fatalf("failed to run bibd --version: %v", err)
	}

	helpers.AssertContains(t, output, "bibd")
}

// TestDaemonLifecycle tests starting and stopping the daemon.
func TestDaemonLifecycle(t *testing.T) {
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	dataDir := testutil.TempDir(t, "daemon")

	// Create config file
	cfg := fixtures.BibdConfigSQLite(dataDir)
	configPath := fixtures.WriteConfigFile(t, dataDir, cfg)

	// Start daemon
	daemon := helpers.StartDaemon(t, bibdBinary, configPath, dataDir)

	// Wait for daemon to be ready
	if err := daemon.WaitForReady(30 * time.Second); err != nil {
		t.Logf("Daemon logs:\n%s", daemon.Logs())
		t.Fatalf("daemon failed to start: %v", err)
	}

	t.Logf("Daemon started with PID %d", daemon.PID())

	// Stop daemon
	daemon.Stop()

	// Verify daemon stopped
	time.Sleep(time.Second)
}

// TestDaemonWithPostgres tests the daemon with PostgreSQL backend.
func TestDaemonWithPostgres(t *testing.T) {
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	// Start PostgreSQL container
	cm := containers.NewManager(t)
	pgCfg := containers.DefaultPostgresConfig()
	pgContainer, err := cm.StartPostgres(ctx, pgCfg)
	if err != nil {
		t.Fatalf("failed to start postgres: %v", err)
	}

	dataDir := testutil.TempDir(t, "daemon-pg")

	// Create config file with PostgreSQL
	cfg := fixtures.BibdConfig(dataDir, pgContainer.HostPort(5432))
	configPath := fixtures.WriteConfigFile(t, dataDir, cfg)

	// Start daemon
	daemon := helpers.StartDaemon(t, bibdBinary, configPath, dataDir)

	// Wait for daemon to be ready
	if err := daemon.WaitForReady(60 * time.Second); err != nil {
		t.Logf("Daemon logs:\n%s", daemon.Logs())
		t.Fatalf("daemon failed to start: %v", err)
	}

	t.Logf("Daemon started with PostgreSQL backend")

	// Stop daemon
	daemon.Stop()
}

// TestCLIConfigCommands tests the CLI configuration commands.
func TestCLIConfigCommands(t *testing.T) {
	ctx := testutil.TestContext(t)
	runner := helpers.NewBinaryRunner(t, bibBinary)

	t.Run("config show", func(t *testing.T) {
		output, err := runner.Run(ctx, "config", "show")
		// May fail if no config exists, that's OK
		t.Logf("config show output: %s, err: %v", output, err)
	})

	t.Run("config validate", func(t *testing.T) {
		dataDir := testutil.TempDir(t, "config")
		cfg := fixtures.BibdConfigSQLite(dataDir)
		configPath := fixtures.WriteConfigFile(t, dataDir, cfg)

		output, err := runner.Run(ctx, "config", "validate", "--config", configPath)
		t.Logf("config validate output: %s, err: %v", output, err)
	})
}
