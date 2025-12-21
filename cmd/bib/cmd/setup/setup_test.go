package setup

import (
	"strings"
	"testing"
)

func TestDeploymentTarget_String(t *testing.T) {
	tests := []struct {
		target   DeploymentTarget
		expected string
	}{
		{TargetLocal, "local"},
		{TargetDocker, "docker"},
		{TargetPodman, "podman"},
		{TargetKubernetes, "kubernetes"},
	}

	for _, tt := range tests {
		if string(tt.target) != tt.expected {
			t.Errorf("expected %q, got %q", tt.expected, string(tt.target))
		}
	}
}

func TestDeploymentTarget_IsValid(t *testing.T) {
	tests := []struct {
		target string
		valid  bool
	}{
		{"local", true},
		{"docker", true},
		{"podman", true},
		{"kubernetes", true},
		{"invalid", false},
		{"", false},
		{"Local", false}, // case sensitive
	}

	for _, tt := range tests {
		target := DeploymentTarget(tt.target)
		valid := target.IsValid()
		if valid != tt.valid {
			t.Errorf("DeploymentTarget(%q).IsValid() = %v, expected %v", tt.target, valid, tt.valid)
		}
	}
}

func TestValidReconfigureSections(t *testing.T) {
	t.Run("bib sections", func(t *testing.T) {
		sections := ValidReconfigureSections(false)
		expected := []string{"identity", "output", "connection", "logging"}

		if len(sections) != len(expected) {
			t.Errorf("expected %d sections, got %d", len(expected), len(sections))
		}

		for i, s := range expected {
			if i >= len(sections) || sections[i] != s {
				t.Errorf("expected section %d to be %q, got %q", i, s, sections[i])
			}
		}
	})

	t.Run("bibd sections", func(t *testing.T) {
		sections := ValidReconfigureSections(true)
		expectedContains := []string{"identity", "server", "tls", "storage", "p2p", "cluster"}

		for _, exp := range expectedContains {
			found := false
			for _, s := range sections {
				if s == exp {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected bibd sections to contain %q", exp)
			}
		}
	})
}

func TestSetupFlags_Defaults(t *testing.T) {
	// Note: Package-level flags are set during init(), so we can only
	// test that they have expected values after cobra flag parsing.
	// The target defaults to "local" if --target is not provided but --daemon is used.
	// For now, just verify the variables exist and can be read.
	_ = setupDaemon
	_ = setupQuick
	_ = setupFresh
	_ = setupTarget
}

func TestValidatePort(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"", false},      // Empty is OK (uses default)
		{"5432", false},  // Valid port
		{"1", false},     // Minimum valid
		{"65535", false}, // Maximum valid
		{"0", true},      // Too low
		{"65536", true},  // Too high
		{"-1", true},     // Negative
		{"abc", true},    // Non-numeric
		{"12.34", true},  // Decimal
		{"5432a", true},  // Mixed
	}

	for _, tt := range tests {
		err := validatePort(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("validatePort(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
		}
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},         // Short string unchanged
		{"hello", 5, "hello"},          // Exact length unchanged
		{"hello world", 8, "hello..."}, // Truncated with ellipsis
		{"hello", 3, "hel"},            // Very short maxLen
		{"hello", 2, "he"},             // Even shorter
		{"", 5, ""},                    // Empty string
	}

	for _, tt := range tests {
		result := truncateString(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncateString(%q, %d) = %q, expected %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestPostgresTestResult_Fields(t *testing.T) {
	result := PostgresTestResult{
		Success:       true,
		ServerVersion: "15.0",
		Database:      "testdb",
		User:          "testuser",
		Duration:      100,
		Error:         "",
	}

	if !result.Success {
		t.Error("expected success to be true")
	}
	if result.ServerVersion != "15.0" {
		t.Error("server version mismatch")
	}
	if result.Database != "testdb" {
		t.Error("database mismatch")
	}
	if result.User != "testuser" {
		t.Error("user mismatch")
	}
}

func TestTestPostgresConnection_InvalidHost(t *testing.T) {
	// Test with an invalid connection string
	result := testPostgresConnection("postgres://user:pass@localhost:59999/db")

	// Should fail since no postgres is running on that port
	if result.Success {
		t.Skip("Skipping test - connection unexpectedly succeeded (postgres running on port 59999?)")
	}

	if result.Error == "" {
		t.Error("expected error message for failed connection")
	}
}

func TestCustomBootstrapPeerValidation(t *testing.T) {
	tests := []struct {
		input   string
		isValid bool
		desc    string
	}{
		{"/ip4/1.2.3.4/tcp/4001/p2p/Qm123", true, "valid multiaddr"},
		{"/ip6/::1/tcp/4001/p2p/Qm123", true, "valid ipv6 multiaddr"},
		{"/dns4/bootstrap.bib.dev/tcp/4001/p2p/Qm123", true, "valid dns multiaddr"},
		{"", false, "empty string"},
		{"not-a-multiaddr", false, "no leading slash"},
		{"Qm123456", false, "peer ID only"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// Basic validation: should start with /
			valid := strings.HasPrefix(tt.input, "/")
			if valid != tt.isValid {
				t.Errorf("validation for %q: got %v, expected %v", tt.input, valid, tt.isValid)
			}
		})
	}
}
