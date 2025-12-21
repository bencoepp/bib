// Package e2e provides end-to-end tests for bib setup.
package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// skipIfNoDocker skips the test if Docker is not available
func skipIfNoDocker(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "info")
	if err := cmd.Run(); err != nil {
		t.Skip("Docker not available, skipping E2E test")
	}
}

// skipIfNoBib skips the test if bib binary is not available
func skipIfNoBib(t *testing.T) string {
	t.Helper()

	// Try to find bib in common locations
	locations := []string{
		"bib",
		"./bib",
		"../../bib",
		"../../../bib",
		filepath.Join(os.Getenv("GOPATH"), "bin", "bib"),
	}

	for _, loc := range locations {
		if _, err := exec.LookPath(loc); err == nil {
			return loc
		}
	}

	t.Skip("bib binary not found, skipping E2E test")
	return ""
}

// TestBibVersion tests that bib --version works
func TestBibVersion(t *testing.T) {
	bibPath := skipIfNoBib(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bibPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("bib --version failed: %v", err)
	}

	if len(output) == 0 {
		t.Error("version output is empty")
	}
}

// TestBibHelp tests that bib --help works
func TestBibHelp(t *testing.T) {
	bibPath := skipIfNoBib(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bibPath, "--help")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("bib --help failed: %v", err)
	}

	if !strings.Contains(string(output), "setup") {
		t.Error("help should mention setup command")
	}
}

// TestBibSetupHelp tests that bib setup --help works
func TestBibSetupHelp(t *testing.T) {
	bibPath := skipIfNoBib(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bibPath, "setup", "--help")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("bib setup --help failed: %v", err)
	}

	helpText := string(output)

	// Check for expected flags
	expectedFlags := []string{
		"--daemon",
		"--quick",
		"--target",
		"--fresh",
	}

	for _, flag := range expectedFlags {
		if !strings.Contains(helpText, flag) {
			t.Errorf("help should mention %s flag", flag)
		}
	}
}

// TestBibConfigReset tests bib config reset command
func TestBibConfigReset(t *testing.T) {
	bibPath := skipIfNoBib(t)

	// Use a temporary config directory
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("HOME", tmpDir)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test help
	cmd := exec.CommandContext(ctx, bibPath, "config", "reset", "--help")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("bib config reset --help failed: %v", err)
	}

	if !strings.Contains(string(output), "Reset") {
		t.Error("help should mention Reset")
	}
}

// TestBibTrustHelp tests bib trust --help
func TestBibTrustHelp(t *testing.T) {
	bibPath := skipIfNoBib(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bibPath, "trust", "--help")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("bib trust --help failed: %v", err)
	}

	helpText := string(output)

	// Check for expected subcommands
	expectedSubcmds := []string{
		"add",
		"list",
		"remove",
		"pin",
	}

	for _, subcmd := range expectedSubcmds {
		if !strings.Contains(helpText, subcmd) {
			t.Errorf("help should mention %s subcommand", subcmd)
		}
	}
}

// TestBibConnectHelp tests bib connect --help
func TestBibConnectHelp(t *testing.T) {
	bibPath := skipIfNoBib(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bibPath, "connect", "--help")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("bib connect --help failed: %v", err)
	}

	helpText := string(output)

	// Check for TOFU flag
	if !strings.Contains(helpText, "trust-first-use") {
		t.Error("help should mention --trust-first-use flag")
	}
}

// TestDockerQuickSetupDryRun tests Docker quick setup in dry-run mode
func TestDockerQuickSetupDryRun(t *testing.T) {
	skipIfNoDocker(t)
	bibPath := skipIfNoBib(t)

	// Use a temporary directory for output
	tmpDir := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// This would run the setup but we can't fully test it without interaction
	// Instead, test that the command parses correctly
	cmd := exec.CommandContext(ctx, bibPath, "setup", "--daemon", "--target", "docker", "--help")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("bib setup --daemon --target docker --help failed: %v", err)
	}

	if !strings.Contains(string(output), "docker") {
		t.Error("help should mention docker target")
	}

	_ = tmpDir // Would use for actual deployment test
}

// TestKubernetesQuickSetupDryRun tests Kubernetes quick setup in dry-run mode
func TestKubernetesQuickSetupDryRun(t *testing.T) {
	bibPath := skipIfNoBib(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test that the command parses correctly
	cmd := exec.CommandContext(ctx, bibPath, "setup", "--daemon", "--target", "kubernetes", "--help")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("bib setup --daemon --target kubernetes --help failed: %v", err)
	}

	if !strings.Contains(string(output), "kubernetes") {
		t.Error("help should mention kubernetes target")
	}
}

// TestInvalidSetupFlags tests that invalid flags are rejected
func TestInvalidSetupFlags(t *testing.T) {
	bibPath := skipIfNoBib(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// --target without --daemon should fail
	cmd := exec.CommandContext(ctx, bibPath, "setup", "--target", "docker")
	output, _ := cmd.CombinedOutput()

	if !strings.Contains(string(output), "requires --daemon") && !strings.Contains(string(output), "error") {
		t.Error("should reject --target without --daemon")
	}
}

// TestSetupTargetValidation tests target validation
func TestSetupTargetValidation(t *testing.T) {
	bibPath := skipIfNoBib(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Invalid target should fail
	cmd := exec.CommandContext(ctx, bibPath, "setup", "--daemon", "--target", "invalid")
	output, _ := cmd.CombinedOutput()

	if !strings.Contains(string(output), "invalid") {
		t.Error("should reject invalid target")
	}
}
