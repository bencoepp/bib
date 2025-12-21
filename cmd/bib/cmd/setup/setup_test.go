package setup

import (
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
