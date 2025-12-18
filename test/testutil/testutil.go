// Package testutil provides shared utilities for integration and e2e tests.
package testutil

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// DefaultTimeout is the default timeout for test operations.
var DefaultTimeout = 5 * time.Minute

func init() {
	if t := os.Getenv("TEST_TIMEOUT"); t != "" {
		if d, err := time.ParseDuration(t); err == nil {
			DefaultTimeout = d
		}
	}
}

// TestContext returns a context with the default test timeout.
// The context is cancelled when the test completes.
func TestContext(t testing.TB) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	t.Cleanup(cancel)
	return ctx
}

// TestContextWithTimeout returns a context with a custom timeout.
func TestContextWithTimeout(t testing.TB, timeout time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)
	return ctx
}

// TempDir creates a temporary directory that is cleaned up after the test.
func TempDir(t testing.TB, prefix string) string {
	t.Helper()
	dir, err := os.MkdirTemp("", fmt.Sprintf("bib-test-%s-*", prefix))
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Logf("failed to remove temp dir: %v", err)
		}
	})
	return dir
}

// SkipIfShort skips the test if running with -short flag.
func SkipIfShort(t testing.TB) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
}

// SkipIfCI skips the test if running in CI environment.
func SkipIfCI(t testing.TB) {
	t.Helper()
	if os.Getenv("CI") != "" {
		t.Skip("skipping in CI environment")
	}
}

// RequireEnv fails the test if the environment variable is not set.
func RequireEnv(t testing.TB, key string) string {
	t.Helper()
	value := os.Getenv(key)
	if value == "" {
		t.Fatalf("required environment variable %s not set", key)
	}
	return value
}

// GetEnvOrDefault returns the environment variable value or the default.
func GetEnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// IsVerbose returns true if TEST_VERBOSE is set.
func IsVerbose() bool {
	return os.Getenv("TEST_VERBOSE") == "true" || os.Getenv("TEST_VERBOSE") == "1"
}

// KeepContainers returns true if TEST_KEEP_CONTAINERS is set.
// When true, containers are not removed on test failure for debugging.
func KeepContainers() bool {
	return os.Getenv("TEST_KEEP_CONTAINERS") == "true" || os.Getenv("TEST_KEEP_CONTAINERS") == "1"
}
