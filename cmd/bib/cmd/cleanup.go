package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"bib/internal/config"

	"github.com/spf13/cobra"
)

var (
	cleanupAll       bool
	cleanupPostgres  bool
	cleanupBackups   bool
	cleanupLogs      bool
	cleanupCache     bool
	cleanupForce     bool
	cleanupContainer string
)

// cleanupCmd represents the cleanup command
var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up bibd resources",
	Long: `Clean up resources created by bibd, including:

- Managed PostgreSQL containers
- Backup files
- Log files
- Cache data
- Configuration files (with --all)

This command is useful for:
- Completely removing bibd from a system
- Cleaning up after failed installations
- Freeing disk space`,
	Example: `  # Clean up everything (interactive confirmation)
  bib cleanup --all

  # Clean up only PostgreSQL containers
  bib cleanup --postgres

  # Clean up backups and logs
  bib cleanup --backups --logs

  # Force cleanup without confirmation
  bib cleanup --all --force`,
	RunE: runCleanup,
}

func init() {
	rootCmd.AddCommand(cleanupCmd)

	cleanupCmd.Flags().BoolVar(&cleanupAll, "all", false, "Clean up all resources (config, data, containers)")
	cleanupCmd.Flags().BoolVar(&cleanupPostgres, "postgres", false, "Clean up managed PostgreSQL containers")
	cleanupCmd.Flags().BoolVar(&cleanupBackups, "backups", false, "Clean up backup files")
	cleanupCmd.Flags().BoolVar(&cleanupLogs, "logs", false, "Clean up log files")
	cleanupCmd.Flags().BoolVar(&cleanupCache, "cache", false, "Clean up cache data")
	cleanupCmd.Flags().BoolVar(&cleanupForce, "force", false, "Force cleanup without confirmation")
	cleanupCmd.Flags().StringVar(&cleanupContainer, "container", "", "Specific container name to clean up")
}

func runCleanup(cmd *cobra.Command, args []string) error {
	_ = cmd  // unused
	_ = args // unused
	ctx := context.Background()

	// Load configuration if it exists
	cfg, err := config.LoadBibd("")
	if err != nil {
		// Config might not exist, use defaults
		defaultCfg := config.DefaultBibdConfig()
		cfg = &defaultCfg
	}

	// Determine what to clean up
	if !cleanupAll && !cleanupPostgres && !cleanupBackups && !cleanupLogs && !cleanupCache && cleanupContainer == "" {
		return fmt.Errorf("please specify what to clean up (--all, --postgres, --backups, --logs, --cache, or --container)")
	}

	// Build cleanup plan
	plan := buildCleanupPlan(cfg)

	// Show cleanup plan
	fmt.Println("Cleanup Plan:")
	fmt.Println("=============")
	for _, item := range plan {
		fmt.Printf("  - %s\n", item)
	}
	fmt.Println()

	// Confirm unless --force is used
	if !cleanupForce {
		fmt.Print("Proceed with cleanup? (y/N): ")
		var response string
		_, _ = fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	// Execute cleanup
	fmt.Println("\nCleaning up...")

	var errors []error

	// Clean up PostgreSQL containers
	if cleanupAll || cleanupPostgres || cleanupContainer != "" {
		if err := cleanupPostgresContainers(ctx, cfg, cleanupContainer); err != nil {
			errors = append(errors, err)
		}
	}

	// Clean up backups
	if cleanupAll || cleanupBackups {
		if err := cleanupBackupFiles(cfg); err != nil {
			errors = append(errors, err)
		}
	}

	// Clean up logs
	if cleanupAll || cleanupLogs {
		if err := cleanupLogFiles(cfg); err != nil {
			errors = append(errors, err)
		}
	}

	// Clean up cache
	if cleanupAll || cleanupCache {
		if err := cleanupCacheFiles(cfg); err != nil {
			errors = append(errors, err)
		}
	}

	// Clean up data directory (only with --all)
	if cleanupAll {
		if err := cleanupDataDirectory(cfg); err != nil {
			errors = append(errors, err)
		}
	}

	// Report results
	if len(errors) > 0 {
		fmt.Println("\n⚠ Cleanup completed with errors:")
		for _, err := range errors {
			fmt.Printf("  - %v\n", err)
		}
		return fmt.Errorf("cleanup completed with %d error(s)", len(errors))
	}

	fmt.Println("\n✓ Cleanup completed successfully")
	return nil
}

func buildCleanupPlan(cfg *config.BibdConfig) []string {
	var plan []string

	if cleanupAll || cleanupPostgres || cleanupContainer != "" {
		if cleanupContainer != "" {
			plan = append(plan, fmt.Sprintf("Stop and remove PostgreSQL container: %s", cleanupContainer))
		} else {
			plan = append(plan, "Stop and remove all bibd PostgreSQL containers")
		}
		plan = append(plan, "Remove PostgreSQL data volumes")
	}

	if cleanupAll || cleanupBackups {
		backupPath := filepath.Join(cfg.Server.DataDir, "backups")
		plan = append(plan, fmt.Sprintf("Remove backup files from: %s", backupPath))
	}

	if cleanupAll || cleanupLogs {
		logPath := cfg.Log.FilePath
		if logPath == "" {
			logPath = filepath.Join(cfg.Server.DataDir, "bibd.log")
		}
		plan = append(plan, fmt.Sprintf("Remove log files from: %s", logPath))
		if cfg.Log.AuditPath != "" {
			plan = append(plan, fmt.Sprintf("Remove audit logs from: %s", cfg.Log.AuditPath))
		}
	}

	if cleanupAll || cleanupCache {
		if cfg.Database.Backend == "sqlite" {
			cachePath := cfg.Database.SQLite.Path
			if cachePath == "" {
				cachePath = filepath.Join(cfg.Server.DataDir, "cache.db")
			}
			plan = append(plan, fmt.Sprintf("Remove SQLite cache: %s", cachePath))
		}
	}

	if cleanupAll {
		plan = append(plan, fmt.Sprintf("Remove data directory: %s", cfg.Server.DataDir))
		configDir, _ := config.UserConfigDir(config.AppBibd)
		plan = append(plan, fmt.Sprintf("Remove config directory: %s", configDir))
	}

	return plan
}

func cleanupPostgresContainers(ctx context.Context, cfg *config.BibdConfig, specificContainer string) error {
	// Detect available container runtime
	containerRuntime := detectContainerRuntime()
	if containerRuntime == "" {
		fmt.Println("  ⚠ No container runtime detected, skipping PostgreSQL cleanup")
		return nil
	}

	fmt.Printf("  Using %s container runtime\n", containerRuntime)

	// List containers to clean up
	var containers []string
	if specificContainer != "" {
		containers = []string{specificContainer}
	} else {
		// Find all bibd-postgres containers
		listCmd := exec.CommandContext(ctx, containerRuntime, "ps", "-a", "--filter", "name=bibd-postgres", "--format", "{{.Names}}")
		output, err := listCmd.Output()
		if err != nil {
			return fmt.Errorf("failed to list containers: %w", err)
		}

		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			if line != "" {
				containers = append(containers, line)
			}
		}
	}

	if len(containers) == 0 {
		fmt.Println("  ✓ No PostgreSQL containers found")
		return nil
	}

	// Stop and remove each container
	for _, container := range containers {
		fmt.Printf("  Stopping container: %s\n", container)
		stopCmd := exec.CommandContext(ctx, containerRuntime, "stop", container)
		if err := stopCmd.Run(); err != nil {
			fmt.Printf("  ⚠ Failed to stop %s: %v\n", container, err)
		}

		fmt.Printf("  Removing container: %s\n", container)
		rmCmd := exec.CommandContext(ctx, containerRuntime, "rm", "-f", container)
		if err := rmCmd.Run(); err != nil {
			fmt.Printf("  ⚠ Failed to remove %s: %v\n", container, err)
		}
	}

	// Remove bridge network if it exists
	networkName := "bibd-network"
	fmt.Printf("  Checking for bridge network: %s\n", networkName)
	rmNetCmd := exec.CommandContext(ctx, containerRuntime, "network", "rm", networkName)
	if err := rmNetCmd.Run(); err != nil {
		// Non-fatal, network might not exist
		fmt.Printf("  ⚠ Could not remove network %s (may not exist)\n", networkName)
	} else {
		fmt.Printf("  ✓ Removed bridge network: %s\n", networkName)
	}

	// Remove PostgreSQL data directory
	pgDataDir := filepath.Join(cfg.Server.DataDir, "postgres")
	if _, err := os.Stat(pgDataDir); err == nil {
		fmt.Printf("  Removing PostgreSQL data: %s\n", pgDataDir)
		if err := os.RemoveAll(pgDataDir); err != nil {
			return fmt.Errorf("failed to remove PostgreSQL data: %w", err)
		}
		fmt.Println("  ✓ PostgreSQL data removed")
	}

	fmt.Printf("  ✓ Cleaned up %d PostgreSQL container(s)\n", len(containers))
	return nil
}

func cleanupBackupFiles(cfg *config.BibdConfig) error {
	backupPath := filepath.Join(cfg.Server.DataDir, "backups")

	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		fmt.Println("  ✓ No backup directory found")
		return nil
	}

	fmt.Printf("  Removing backups: %s\n", backupPath)
	if err := os.RemoveAll(backupPath); err != nil {
		return fmt.Errorf("failed to remove backups: %w", err)
	}

	fmt.Println("  ✓ Backups removed")
	return nil
}

func cleanupLogFiles(cfg *config.BibdConfig) error {
	// Remove main log
	logPath := cfg.Log.FilePath
	if logPath == "" {
		logPath = filepath.Join(cfg.Server.DataDir, "bibd.log")
	}
	if logPath != "" {
		if _, err := os.Stat(logPath); err == nil {
			fmt.Printf("  Removing log file: %s\n", logPath)
			if err := os.Remove(logPath); err != nil {
				return fmt.Errorf("failed to remove log file: %w", err)
			}
		}
	}

	// Remove audit log
	if cfg.Log.AuditPath != "" {
		if _, err := os.Stat(cfg.Log.AuditPath); err == nil {
			fmt.Printf("  Removing audit log: %s\n", cfg.Log.AuditPath)
			if err := os.Remove(cfg.Log.AuditPath); err != nil {
				return fmt.Errorf("failed to remove audit log: %w", err)
			}
		}
	}

	fmt.Println("  ✓ Log files removed")
	return nil
}

func cleanupCacheFiles(cfg *config.BibdConfig) error {
	if cfg.Database.Backend != "sqlite" {
		fmt.Println("  ✓ No cache files to remove (not using SQLite)")
		return nil
	}

	cachePath := cfg.Database.SQLite.Path
	if cachePath == "" {
		cachePath = filepath.Join(cfg.Server.DataDir, "cache.db")
	}

	if _, err := os.Stat(cachePath); err == nil {
		fmt.Printf("  Removing SQLite cache: %s\n", cachePath)
		if err := os.Remove(cachePath); err != nil {
			return fmt.Errorf("failed to remove cache: %w", err)
		}

		// Also remove SQLite journal files (ignore errors)
		_ = os.Remove(cachePath + "-wal")
		_ = os.Remove(cachePath + "-shm")
	}

	fmt.Println("  ✓ Cache files removed")
	return nil
}

func cleanupDataDirectory(cfg *config.BibdConfig) error {
	fmt.Printf("  Removing data directory: %s\n", cfg.Server.DataDir)
	if err := os.RemoveAll(cfg.Server.DataDir); err != nil {
		return fmt.Errorf("failed to remove data directory: %w", err)
	}

	// Remove config directory
	configDir, err := config.UserConfigDir(config.AppBibd)
	if err == nil {
		fmt.Printf("  Removing config directory: %s\n", configDir)
		if err := os.RemoveAll(configDir); err != nil {
			return fmt.Errorf("failed to remove config directory: %w", err)
		}
	}

	fmt.Println("  ✓ All data removed")
	return nil
}

func detectContainerRuntime() string {
	// Check for Docker
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("docker.exe"); err == nil {
			return "docker"
		}
		if _, err := exec.LookPath("podman.exe"); err == nil {
			return "podman"
		}
	} else {
		if _, err := exec.LookPath("docker"); err == nil {
			return "docker"
		}
		if _, err := exec.LookPath("podman"); err == nil {
			return "podman"
		}
	}

	return ""
}
