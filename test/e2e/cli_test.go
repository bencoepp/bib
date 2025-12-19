//go:build e2e

package e2e

import (
	"strings"
	"testing"

	"bib/test/testutil"
	"bib/test/testutil/helpers"
)

// TestCLIHelp tests that all commands have proper help output.
func TestCLIHelp(t *testing.T) {
	ctx := testutil.TestContext(t)
	runner := helpers.NewBinaryRunner(t, bibBinary)

	tests := []struct {
		name     string
		args     []string
		contains []string
	}{
		{
			name:     "root help",
			args:     []string{"--help"},
			contains: []string{"bib", "command-line interface"},
		},
		{
			name:     "admin help",
			args:     []string{"admin", "--help"},
			contains: []string{"admin", "Administrative"},
		},
		{
			name:     "admin cleanup help",
			args:     []string{"admin", "cleanup", "--help"},
			contains: []string{"cleanup", "resources"},
		},
		{
			name:     "admin reset help",
			args:     []string{"admin", "reset", "--help"},
			contains: []string{"reset", "state"},
		},
		{
			name:     "admin backup help",
			args:     []string{"admin", "backup", "--help"},
			contains: []string{"backup", "database"},
		},
		{
			name:     "config help",
			args:     []string{"config", "--help"},
			contains: []string{"config", "configuration"},
		},
		{
			name:     "tui help",
			args:     []string{"tui", "--help"},
			contains: []string{"tui", "dashboard", "interactive"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := runner.Run(ctx, tt.args...)
			if err != nil {
				t.Fatalf("failed to run %v: %v", tt.args, err)
			}

			outputLower := strings.ToLower(output)
			for _, expected := range tt.contains {
				if !strings.Contains(outputLower, strings.ToLower(expected)) {
					t.Errorf("output missing %q:\n%s", expected, output)
				}
			}
		})
	}
}

// TestCLICommandGrouping tests that commands are properly grouped.
func TestCLICommandGrouping(t *testing.T) {
	ctx := testutil.TestContext(t)
	runner := helpers.NewBinaryRunner(t, bibBinary)

	// Test that admin contains cleanup and reset
	output, err := runner.Run(ctx, "admin", "--help")
	if err != nil {
		t.Fatalf("failed to run admin --help: %v", err)
	}

	adminSubcommands := []string{"cleanup", "reset", "backup", "blob", "breakglass"}
	for _, sub := range adminSubcommands {
		if !strings.Contains(output, sub) {
			t.Errorf("admin help missing subcommand %q:\n%s", sub, output)
		}
	}
}

// TestCLIOutputFormats tests different output format options.
func TestCLIOutputFormats(t *testing.T) {
	ctx := testutil.TestContext(t)
	runner := helpers.NewBinaryRunner(t, bibBinary)

	formats := []string{"json", "yaml", "table"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			// Version command should work with any format
			output, err := runner.Run(ctx, "--output", format, "version")
			if err != nil {
				t.Fatalf("failed to run version with -o %s: %v", format, err)
			}

			if output == "" {
				t.Errorf("expected output for format %s, got empty", format)
			}
		})
	}
}

// TestCLIErrorHandling tests that errors are properly reported.
func TestCLIErrorHandling(t *testing.T) {
	ctx := testutil.TestContext(t)
	runner := helpers.NewBinaryRunner(t, bibBinary)

	tests := []struct {
		name        string
		args        []string
		expectError bool
		contains    string
	}{
		{
			name:        "unknown command",
			args:        []string{"unknown-command"},
			expectError: true,
			contains:    "unknown",
		},
		{
			name:        "invalid flag",
			args:        []string{"--invalid-flag"},
			expectError: true,
			contains:    "unknown flag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runner.Run(ctx, tt.args...)
			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
