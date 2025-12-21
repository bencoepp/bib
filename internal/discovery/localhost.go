package discovery

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"sync"
	"time"
)

// discoverLocalhost discovers bibd nodes running on localhost
func (d *Discoverer) discoverLocalhost(ctx context.Context) ([]DiscoveredNode, error) {
	var nodes []DiscoveredNode
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Scan configured ports
	for _, port := range d.opts.LocalPorts {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()

			address := fmt.Sprintf("localhost:%d", p)
			node, err := d.checkLocalPort(ctx, address)
			if err == nil && node != nil {
				mu.Lock()
				nodes = append(nodes, *node)
				mu.Unlock()
			}
		}(port)
	}

	// Also check for Unix socket (Linux/macOS)
	if runtime.GOOS != "windows" {
		wg.Add(1)
		go func() {
			defer wg.Done()

			socketPaths := []string{
				"/var/run/bibd.sock",
				os.ExpandEnv("$HOME/.config/bibd/bibd.sock"),
			}

			for _, path := range socketPaths {
				if node := d.checkUnixSocket(ctx, path); node != nil {
					mu.Lock()
					nodes = append(nodes, *node)
					mu.Unlock()
					break // Only add one socket connection
				}
			}
		}()
	}

	wg.Wait()
	return nodes, nil
}

// checkLocalPort checks if a bibd node is running on the given address
func (d *Discoverer) checkLocalPort(ctx context.Context, address string) (*DiscoveredNode, error) {
	// Try to connect to the port
	timeout := d.opts.LatencyTimeout
	if timeout == 0 {
		timeout = 2 * time.Second
	}

	dialer := &net.Dialer{
		Timeout: timeout,
	}

	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	latency := time.Since(start)
	conn.Close()

	// Port is open, assume it's a bibd node
	// In a real implementation, we would verify by making a gRPC health check
	node := &DiscoveredNode{
		Address:      address,
		Method:       MethodLocal,
		Latency:      latency,
		DiscoveredAt: time.Now(),
	}

	// Try to get node info via gRPC (placeholder for now)
	// node.NodeInfo = d.getNodeInfo(ctx, address)

	return node, nil
}

// checkUnixSocket checks if a bibd node is accessible via Unix socket
func (d *Discoverer) checkUnixSocket(ctx context.Context, socketPath string) *DiscoveredNode {
	// Check if socket file exists
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		return nil
	}

	// Try to connect to the socket
	timeout := d.opts.LatencyTimeout
	if timeout == 0 {
		timeout = 2 * time.Second
	}

	dialer := &net.Dialer{
		Timeout: timeout,
	}

	start := time.Now()
	conn, err := dialer.DialContext(ctx, "unix", socketPath)
	if err != nil {
		return nil
	}
	latency := time.Since(start)
	conn.Close()

	return &DiscoveredNode{
		Address:      "unix://" + socketPath,
		Method:       MethodLocal,
		Latency:      latency,
		DiscoveredAt: time.Now(),
	}
}

// CheckAddress checks if a bibd node is accessible at the given address
// and returns node information if available
func (d *Discoverer) CheckAddress(ctx context.Context, address string) (*DiscoveredNode, error) {
	latency, err := measureLatency(ctx, address, d.opts.LatencyTimeout)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}

	return &DiscoveredNode{
		Address:      address,
		Method:       MethodManual,
		Latency:      latency,
		DiscoveredAt: time.Now(),
	}, nil
}

// ScanPorts scans a list of ports on a host and returns open ports
func ScanPorts(ctx context.Context, host string, ports []int, timeout time.Duration) []int {
	if timeout == 0 {
		timeout = 2 * time.Second
	}

	var openPorts []int
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, port := range ports {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()

			address := fmt.Sprintf("%s:%d", host, p)
			dialer := &net.Dialer{Timeout: timeout}

			conn, err := dialer.DialContext(ctx, "tcp", address)
			if err == nil {
				conn.Close()
				mu.Lock()
				openPorts = append(openPorts, p)
				mu.Unlock()
			}
		}(port)
	}

	wg.Wait()
	return openPorts
}
