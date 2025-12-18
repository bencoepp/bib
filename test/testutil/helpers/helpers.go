// Package helpers provides common test helper functions.
package helpers

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Retry retries a function until it succeeds or the timeout is reached.
func Retry(ctx context.Context, interval time.Duration, fn func() error) error {
	var lastErr error
	for {
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("timeout: last error: %w", lastErr)
			}
			return ctx.Err()
		default:
			if err := fn(); err != nil {
				lastErr = err
				time.Sleep(interval)
				continue
			}
			return nil
		}
	}
}

// Eventually waits for a condition to be true.
func Eventually(t testing.TB, timeout time.Duration, condition func() bool, msgAndArgs ...interface{}) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	msg := "condition never became true"
	if len(msgAndArgs) > 0 {
		msg = fmt.Sprint(msgAndArgs...)
	}
	t.Fatal(msg)
}

// Never waits and fails if a condition becomes true.
func Never(t testing.TB, timeout time.Duration, condition func() bool, msgAndArgs ...interface{}) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			msg := "condition unexpectedly became true"
			if len(msgAndArgs) > 0 {
				msg = fmt.Sprint(msgAndArgs...)
			}
			t.Fatal(msg)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// WaitForPort waits for a port to be available.
func WaitForPort(t testing.TB, address string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, time.Second)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for port %s", address)
}

// GetFreePort finds an available port.
func GetFreePort(t testing.TB) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to get free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// GetFreePorts finds multiple available ports.
func GetFreePorts(t testing.TB, count int) []int {
	t.Helper()
	ports := make([]int, count)
	listeners := make([]net.Listener, count)

	for i := 0; i < count; i++ {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			for j := 0; j < i; j++ {
				listeners[j].Close()
			}
			t.Fatalf("failed to get free port: %v", err)
		}
		listeners[i] = l
		ports[i] = l.Addr().(*net.TCPAddr).Port
	}

	for _, l := range listeners {
		l.Close()
	}

	return ports
}

// BinaryRunner helps run compiled binaries in tests.
type BinaryRunner struct {
	t          testing.TB
	binaryPath string
	processes  []*os.Process
	mu         sync.Mutex
}

// NewBinaryRunner creates a new binary runner.
func NewBinaryRunner(t testing.TB, binaryPath string) *BinaryRunner {
	t.Helper()
	r := &BinaryRunner{
		t:          t,
		binaryPath: binaryPath,
		processes:  make([]*os.Process, 0),
	}
	t.Cleanup(r.Cleanup)
	return r
}

// Run runs the binary with the given arguments and waits for it to complete.
func (r *BinaryRunner) Run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, r.binaryPath, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// Start starts the binary in the background.
func (r *BinaryRunner) Start(ctx context.Context, args ...string) (*os.Process, error) {
	cmd := exec.CommandContext(ctx, r.binaryPath, args...)
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.processes = append(r.processes, cmd.Process)
	r.mu.Unlock()

	return cmd.Process, nil
}

// Cleanup kills all started processes.
func (r *BinaryRunner) Cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, p := range r.processes {
		_ = p.Kill()
		_, _ = p.Wait()
	}
	r.processes = nil
}

// BuildBinary builds a Go binary and returns the path.
func BuildBinary(t testing.TB, pkg, outputDir string) string {
	t.Helper()
	name := filepath.Base(pkg)
	outputPath := filepath.Join(outputDir, name)

	cmd := exec.Command("go", "build", "-o", outputPath, pkg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build binary %s: %v\n%s", pkg, err, output)
	}

	return outputPath
}

// DaemonProcess represents a running daemon process.
type DaemonProcess struct {
	t       testing.TB
	cmd     *exec.Cmd
	pid     int
	dataDir string
	logFile *os.File
	stopped bool
	mu      sync.Mutex
}

// StartDaemon starts a bibd daemon process.
func StartDaemon(t testing.TB, binaryPath, configPath, dataDir string) *DaemonProcess {
	t.Helper()

	// Create log file
	logPath := filepath.Join(dataDir, "daemon.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("failed to create log file: %v", err)
	}

	cmd := exec.Command(binaryPath, "--config", configPath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Dir = dataDir

	if err := cmd.Start(); err != nil {
		logFile.Close()
		t.Fatalf("failed to start daemon: %v", err)
	}

	dp := &DaemonProcess{
		t:       t,
		cmd:     cmd,
		pid:     cmd.Process.Pid,
		dataDir: dataDir,
		logFile: logFile,
	}

	t.Cleanup(dp.Stop)

	return dp
}

// Stop stops the daemon process.
func (dp *DaemonProcess) Stop() {
	dp.mu.Lock()
	defer dp.mu.Unlock()

	if dp.stopped {
		return
	}
	dp.stopped = true

	if dp.cmd.Process != nil {
		_ = dp.cmd.Process.Signal(os.Interrupt)
		done := make(chan error, 1)
		go func() {
			done <- dp.cmd.Wait()
		}()

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = dp.cmd.Process.Kill()
			<-done
		}
	}

	if dp.logFile != nil {
		dp.logFile.Close()
	}
}

// PID returns the process ID.
func (dp *DaemonProcess) PID() int {
	return dp.pid
}

// Logs returns the daemon logs.
func (dp *DaemonProcess) Logs() string {
	logPath := filepath.Join(dp.dataDir, "daemon.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		return fmt.Sprintf("error reading logs: %v", err)
	}
	return string(data)
}

// WaitForReady waits for the daemon to be ready.
func (dp *DaemonProcess) WaitForReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		// Check if process is still running
		if dp.cmd.ProcessState != nil {
			return fmt.Errorf("daemon exited unexpectedly")
		}

		// Check for ready message in logs
		logs := dp.Logs()
		if strings.Contains(logs, "daemon started successfully") {
			return nil
		}

		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for daemon to be ready")
}

// AssertContains checks if a string contains a substring.
func AssertContains(t testing.TB, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected %q to contain %q", s, substr)
	}
}

// AssertNotContains checks if a string does not contain a substring.
func AssertNotContains(t testing.TB, s, substr string) {
	t.Helper()
	if strings.Contains(s, substr) {
		t.Errorf("expected %q to not contain %q", s, substr)
	}
}

// AssertEqual checks if two values are equal.
func AssertEqual(t testing.TB, expected, actual interface{}) {
	t.Helper()
	if expected != actual {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}

// AssertNil checks if a value is nil.
func AssertNil(t testing.TB, v interface{}) {
	t.Helper()
	if v != nil {
		t.Errorf("expected nil, got %v", v)
	}
}

// AssertNotNil checks if a value is not nil.
func AssertNotNil(t testing.TB, v interface{}) {
	t.Helper()
	if v == nil {
		t.Error("expected non-nil value")
	}
}

// AssertNoError checks if there is no error.
func AssertNoError(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// AssertError checks if there is an error.
func AssertError(t testing.TB, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
