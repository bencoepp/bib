package admin

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	resetForce      bool
	resetDbOnly     bool
	resetCertsOnly  bool
	resetP2POnly    bool
	resetKeepConfig bool
	resetDataDir    string
)

// resetCmd represents the reset command
var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset bibd state and optionally delete data",
	Long: `Reset bibd by stopping the daemon and its managed services.

This command will by default reset EVERYTHING:
- Stop the bibd daemon if running
- Stop and remove the managed PostgreSQL container (if any)
- Delete all data (database, P2P state, certificates)

Use --force to stop in-flight jobs without waiting for completion.
Use granular flags to reset only specific components.

Examples:
  # Reset everything (prompts for confirmation)
  bib reset

  # Force reset even if jobs are in-flight
  bib reset --force

  # Reset only database
  bib reset --db-only

  # Reset only certificates
  bib reset --certs-only

  # Reset only P2P state
  bib reset --p2p-only

  # Reset everything but keep config
  bib reset --keep-config

  # Specify custom data directory
  bib reset --data-dir ~/.local/share/bibd`,
	RunE: runReset,
}

func init() {
	Cmd.AddCommand(resetCmd)

	resetCmd.Flags().BoolVarP(&resetForce, "force", "f", false, "Force stop even if jobs are in-flight")
	resetCmd.Flags().BoolVar(&resetDbOnly, "db-only", false, "Reset only database")
	resetCmd.Flags().BoolVar(&resetCertsOnly, "certs-only", false, "Reset only certificates")
	resetCmd.Flags().BoolVar(&resetP2POnly, "p2p-only", false, "Reset only P2P state")
	resetCmd.Flags().BoolVar(&resetKeepConfig, "keep-config", false, "Keep configuration files")
	resetCmd.Flags().StringVar(&resetDataDir, "data-dir", "", "Data directory (default: ~/.local/share/bibd)")
}

// resetScope determines what should be reset based on flags
type resetScope struct {
	database bool
	certs    bool
	p2p      bool
	config   bool
	daemon   bool
}

func determineResetScope() resetScope {
	// If any specific flag is set, only reset those components
	if resetDbOnly || resetCertsOnly || resetP2POnly {
		return resetScope{
			database: resetDbOnly,
			certs:    resetCertsOnly,
			p2p:      resetP2POnly,
			config:   false,
			daemon:   resetDbOnly, // Need to stop daemon to reset DB
		}
	}

	// Default: reset everything (except config if --keep-config)
	return resetScope{
		database: true,
		certs:    true,
		p2p:      true,
		config:   !resetKeepConfig,
		daemon:   true,
	}
}

func runReset(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	out := NewOutputWriter()
	scope := determineResetScope()

	// Determine data directory
	dataDir := resetDataDir
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		dataDir = filepath.Join(home, ".local", "share", "bibd")
	}

	// Check if data directory exists
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		out.WriteSuccess("No bibd data directory found. Nothing to reset.")
		return nil
	}

	// Build description of what will be reset
	var resetItems []string
	if scope.database {
		resetItems = append(resetItems, "database")
	}
	if scope.certs {
		resetItems = append(resetItems, "certificates")
	}
	if scope.p2p {
		resetItems = append(resetItems, "P2P state")
	}
	if scope.config {
		resetItems = append(resetItems, "configuration")
	}

	// Confirm reset
	if !resetForce {
		fmt.Printf("WARNING: This will permanently delete the following in %s:\n", dataDir)
		for _, item := range resetItems {
			fmt.Printf("  - %s\n", item)
		}
		fmt.Print("Type 'yes' to confirm: ")

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input != "yes" {
			out.WriteSuccess("Aborted.")
			return nil
		}
	}

	// Check for in-flight jobs if not forcing
	if !resetForce && scope.daemon {
		hasJobs, err := checkInFlightJobs(ctx, dataDir)
		if err != nil {
			fmt.Printf("Warning: could not check for in-flight jobs: %v\n", err)
		} else if hasJobs {
			return fmt.Errorf("there are jobs in-flight; use --force to stop them")
		}
	}

	// Stop bibd daemon if needed
	if scope.daemon {
		fmt.Println("Stopping bibd daemon...")
		if err := stopBibdDaemon(ctx, dataDir); err != nil {
			fmt.Printf("Warning: could not stop daemon: %v\n", err)
		}
	}

	// Reset database
	if scope.database {
		fmt.Println("Resetting database...")
		// Stop and remove PostgreSQL container
		if err := stopPostgresContainer(ctx, dataDir); err != nil {
			fmt.Printf("Warning: could not stop PostgreSQL container: %v\n", err)
		}
		// Delete database files
		if err := deleteDatabase(dataDir); err != nil {
			fmt.Printf("Warning: could not delete database files: %v\n", err)
		}
	}

	// Reset certificates
	if scope.certs {
		fmt.Println("Resetting certificates...")
		if err := deleteCertificates(dataDir); err != nil {
			fmt.Printf("Warning: could not delete certificates: %v\n", err)
		}
	}

	// Reset P2P state
	if scope.p2p {
		fmt.Println("Resetting P2P state...")
		if err := deleteP2PState(dataDir); err != nil {
			fmt.Printf("Warning: could not delete P2P state: %v\n", err)
		}
	}

	// Reset config (only if explicitly requested)
	if scope.config {
		fmt.Println("Resetting configuration...")
		if err := deleteConfig(); err != nil {
			fmt.Printf("Warning: could not delete configuration: %v\n", err)
		}
	}

	out.WriteSuccess("Reset complete.")
	return nil
}

// checkInFlightJobs checks if there are any jobs currently running.
func checkInFlightJobs(ctx context.Context, dataDir string) (bool, error) {
	// Try to connect to bibd and check for running jobs
	// For now, check if there's a PID file and the process is running
	pidFile := filepath.Join(dataDir, "bibd.pid")
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		return false, nil // No daemon running
	}

	// TODO: Connect to bibd gRPC API to check for running jobs
	// For now, assume no jobs if daemon is running
	return false, nil
}

// stopBibdDaemon stops the bibd daemon.
func stopBibdDaemon(ctx context.Context, dataDir string) error {
	pidFile := filepath.Join(dataDir, "bibd.pid")

	// Check if PID file exists
	data, err := os.ReadFile(pidFile)
	if os.IsNotExist(err) {
		return nil // Daemon not running
	}
	if err != nil {
		return err
	}

	pid := strings.TrimSpace(string(data))

	// Send SIGTERM to the process
	cmd := exec.CommandContext(ctx, "kill", "-TERM", pid)
	if err := cmd.Run(); err != nil {
		// Process might already be dead
		os.Remove(pidFile)
		return nil
	}

	// Wait for process to stop
	for i := 0; i < 30; i++ {
		cmd := exec.Command("kill", "-0", pid)
		if cmd.Run() != nil {
			// Process no longer exists
			os.Remove(pidFile)
			return nil
		}
		time.Sleep(time.Second)
	}

	// Force kill if still running
	if resetForce {
		cmd := exec.CommandContext(ctx, "kill", "-KILL", pid)
		cmd.Run()
		os.Remove(pidFile)
		return nil
	}

	return fmt.Errorf("daemon did not stop within 30 seconds; use --force to kill")
}

// stopPostgresContainer stops and removes the PostgreSQL container.
func stopPostgresContainer(ctx context.Context, dataDir string) error {
	// Try to determine the container name from config or use default pattern
	// Look for container with bibd-postgres prefix

	// Try Docker first
	containerName := findPostgresContainer(ctx, "docker")
	if containerName != "" {
		return stopAndRemoveContainer(ctx, "docker", containerName)
	}

	// Try Podman
	containerName = findPostgresContainer(ctx, "podman")
	if containerName != "" {
		return stopAndRemoveContainer(ctx, "podman", containerName)
	}

	return nil // No container found
}

// findPostgresContainer finds the bibd PostgreSQL container.
func findPostgresContainer(ctx context.Context, runtime string) string {
	cmd := exec.CommandContext(ctx, runtime, "ps", "-a", "--format", "{{.Names}}", "--filter", "name=bibd-postgres")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	name := strings.TrimSpace(string(output))
	if name == "" {
		return ""
	}

	// Return first matching container
	lines := strings.Split(name, "\n")
	if len(lines) > 0 {
		return lines[0]
	}

	return ""
}

// stopAndRemoveContainer stops and removes a container.
func stopAndRemoveContainer(ctx context.Context, runtime, name string) error {
	// Stop the container
	stopCmd := exec.CommandContext(ctx, runtime, "stop", name)
	stopCmd.Run() // Ignore error if already stopped

	// Remove the container
	rmCmd := exec.CommandContext(ctx, runtime, "rm", name)
	return rmCmd.Run()
}

// deleteDataDirectory deletes the data directory.
func deleteDataDirectory(dataDir string) error {
	// Safety check - make sure it looks like a bibd data directory
	markers := []string{"postgres", "cache.db", "secrets", "config"}
	found := false
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(dataDir, marker)); err == nil {
			found = true
			break
		}
	}

	if !found {
		// Also check if directory is empty or doesn't exist
		entries, err := os.ReadDir(dataDir)
		if err != nil || len(entries) == 0 {
			return nil // Nothing to delete
		}

		return fmt.Errorf("directory %s does not appear to be a bibd data directory; refusing to delete", dataDir)
	}

	return os.RemoveAll(dataDir)
}

// deleteDatabase deletes database-related files
func deleteDatabase(dataDir string) error {
	// Delete SQLite database
	sqlitePath := filepath.Join(dataDir, "cache.db")
	if err := os.Remove(sqlitePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete SQLite database: %w", err)
	}
	// Also remove WAL and SHM files
	os.Remove(sqlitePath + "-wal")
	os.Remove(sqlitePath + "-shm")

	// Delete Postgres data directory
	postgresPath := filepath.Join(dataDir, "postgres")
	if err := os.RemoveAll(postgresPath); err != nil {
		return fmt.Errorf("failed to delete Postgres data: %w", err)
	}

	return nil
}

// deleteCertificates deletes certificate files
func deleteCertificates(dataDir string) error {
	certsPath := filepath.Join(dataDir, "certs")
	if err := os.RemoveAll(certsPath); err != nil {
		return fmt.Errorf("failed to delete certificates: %w", err)
	}

	// Also delete secrets directory
	secretsPath := filepath.Join(dataDir, "secrets")
	if err := os.RemoveAll(secretsPath); err != nil {
		return fmt.Errorf("failed to delete secrets: %w", err)
	}

	return nil
}

// deleteP2PState deletes P2P-related state files
func deleteP2PState(dataDir string) error {
	// Delete identity key
	identityPath := filepath.Join(dataDir, "identity.pem")
	if err := os.Remove(identityPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete identity key: %w", err)
	}

	// Delete peer store
	peersPath := filepath.Join(dataDir, "peers.db")
	if err := os.Remove(peersPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete peer store: %w", err)
	}

	// Delete subscriptions
	subsPath := filepath.Join(dataDir, "subscriptions.json")
	if err := os.Remove(subsPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete subscriptions: %w", err)
	}

	// Delete Raft state
	raftPath := filepath.Join(dataDir, "raft")
	if err := os.RemoveAll(raftPath); err != nil {
		return fmt.Errorf("failed to delete Raft state: %w", err)
	}

	return nil
}

// deleteConfig deletes configuration files
func deleteConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Delete bib CLI config
	bibConfigDir := filepath.Join(home, ".config", "bib")
	if err := os.RemoveAll(bibConfigDir); err != nil {
		return fmt.Errorf("failed to delete bib config: %w", err)
	}

	// Delete bibd config
	bibdConfigDir := filepath.Join(home, ".config", "bibd")
	if err := os.RemoveAll(bibdConfigDir); err != nil {
		return fmt.Errorf("failed to delete bibd config: %w", err)
	}

	return nil
}
