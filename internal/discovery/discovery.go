// Package discovery provides mechanisms for discovering bibd nodes.
package discovery

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// DiscoveryMethod indicates how a node was discovered
type DiscoveryMethod string

const (
	MethodLocal  DiscoveryMethod = "local"  // Localhost scan
	MethodMDNS   DiscoveryMethod = "mdns"   // mDNS discovery
	MethodP2P    DiscoveryMethod = "p2p"    // P2P DHT discovery
	MethodManual DiscoveryMethod = "manual" // Manually entered
	MethodPublic DiscoveryMethod = "public" // Public network (bib.dev)
)

// DiscoveredNode represents a discovered bibd node
type DiscoveredNode struct {
	// Address is the node address (host:port)
	Address string

	// Method is how the node was discovered
	Method DiscoveryMethod

	// Latency is the measured connection latency
	Latency time.Duration

	// NodeInfo contains additional node information if available
	NodeInfo *NodeInfo

	// DiscoveredAt is when the node was discovered
	DiscoveredAt time.Time
}

// NodeInfo contains information about a discovered node
type NodeInfo struct {
	// Name is the node's display name
	Name string

	// Version is the bibd version
	Version string

	// PeerID is the P2P peer ID (if available)
	PeerID string

	// Mode is the node's P2P mode (proxy, selective, full)
	Mode string
}

// DiscoveryResult contains the results of a discovery operation
type DiscoveryResult struct {
	// Nodes is the list of discovered nodes
	Nodes []DiscoveredNode

	// Errors contains any errors that occurred during discovery
	Errors []error

	// Duration is how long the discovery took
	Duration time.Duration
}

// DiscoveryOptions configures the discovery process
type DiscoveryOptions struct {
	// Timeout is the maximum time to spend on discovery
	Timeout time.Duration

	// LocalPorts are the ports to scan on localhost
	LocalPorts []int

	// EnableMDNS enables mDNS discovery
	EnableMDNS bool

	// MDNSTimeout is the timeout for mDNS discovery
	MDNSTimeout time.Duration

	// EnableP2P enables P2P discovery
	EnableP2P bool

	// P2PTimeout is the timeout for P2P discovery
	P2PTimeout time.Duration

	// MeasureLatency enables latency measurement
	MeasureLatency bool

	// LatencyTimeout is the timeout for latency measurement
	LatencyTimeout time.Duration
}

// DefaultOptions returns sensible default discovery options
func DefaultOptions() DiscoveryOptions {
	return DiscoveryOptions{
		Timeout:        10 * time.Second,
		LocalPorts:     []int{4000, 8080},
		EnableMDNS:     true,
		MDNSTimeout:    3 * time.Second,
		EnableP2P:      false, // Disabled by default as it requires P2P infrastructure
		P2PTimeout:     5 * time.Second,
		MeasureLatency: true,
		LatencyTimeout: 2 * time.Second,
	}
}

// Discoverer discovers bibd nodes using multiple methods
type Discoverer struct {
	opts DiscoveryOptions
	mu   sync.Mutex
}

// New creates a new Discoverer with the given options
func New(opts DiscoveryOptions) *Discoverer {
	return &Discoverer{
		opts: opts,
	}
}

// NewWithDefaults creates a new Discoverer with default options
func NewWithDefaults() *Discoverer {
	return New(DefaultOptions())
}

// Discover runs all enabled discovery methods and returns combined results
func (d *Discoverer) Discover(ctx context.Context) *DiscoveryResult {
	start := time.Now()

	// Create a context with timeout
	if d.opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d.opts.Timeout)
		defer cancel()
	}

	result := &DiscoveryResult{
		Nodes:  []DiscoveredNode{},
		Errors: []error{},
	}

	// Channel for collecting results from parallel discovery methods
	nodesChan := make(chan []DiscoveredNode, 3)
	errsChan := make(chan error, 3)
	var wg sync.WaitGroup

	// Run localhost discovery
	wg.Add(1)
	go func() {
		defer wg.Done()
		nodes, err := d.discoverLocalhost(ctx)
		if err != nil {
			errsChan <- fmt.Errorf("localhost discovery: %w", err)
		}
		nodesChan <- nodes
	}()

	// Run mDNS discovery if enabled
	if d.opts.EnableMDNS {
		wg.Add(1)
		go func() {
			defer wg.Done()
			nodes, err := d.discoverMDNS(ctx)
			if err != nil {
				errsChan <- fmt.Errorf("mDNS discovery: %w", err)
			}
			nodesChan <- nodes
		}()
	}

	// Run P2P discovery if enabled
	if d.opts.EnableP2P {
		wg.Add(1)
		go func() {
			defer wg.Done()
			nodes, err := d.discoverP2P(ctx)
			if err != nil {
				errsChan <- fmt.Errorf("P2P discovery: %w", err)
			}
			nodesChan <- nodes
		}()
	}

	// Wait for all discovery methods to complete
	go func() {
		wg.Wait()
		close(nodesChan)
		close(errsChan)
	}()

	// Collect results
	nodeMap := make(map[string]DiscoveredNode)
	for nodes := range nodesChan {
		for _, node := range nodes {
			// Deduplicate by address, keep the one with lower latency
			if existing, ok := nodeMap[node.Address]; ok {
				if node.Latency > 0 && (existing.Latency == 0 || node.Latency < existing.Latency) {
					nodeMap[node.Address] = node
				}
			} else {
				nodeMap[node.Address] = node
			}
		}
	}

	for err := range errsChan {
		if err != nil {
			result.Errors = append(result.Errors, err)
		}
	}

	// Convert map to slice
	for _, node := range nodeMap {
		result.Nodes = append(result.Nodes, node)
	}

	// Sort by latency (nodes with measured latency first, then by latency value)
	sortNodesByLatency(result.Nodes)

	result.Duration = time.Since(start)
	return result
}

// DiscoverLocalhost discovers bibd nodes running on localhost
func (d *Discoverer) DiscoverLocalhost(ctx context.Context) ([]DiscoveredNode, error) {
	return d.discoverLocalhost(ctx)
}

// DiscoverMDNS discovers bibd nodes using mDNS
func (d *Discoverer) DiscoverMDNS(ctx context.Context) ([]DiscoveredNode, error) {
	return d.discoverMDNS(ctx)
}

// MeasureLatency measures the latency to a node
func (d *Discoverer) MeasureLatency(ctx context.Context, address string) (time.Duration, error) {
	return measureLatency(ctx, address, d.opts.LatencyTimeout)
}

// sortNodesByLatency sorts nodes by latency (lowest first)
func sortNodesByLatency(nodes []DiscoveredNode) {
	// Simple bubble sort (small list expected)
	for i := 0; i < len(nodes)-1; i++ {
		for j := 0; j < len(nodes)-i-1; j++ {
			// Nodes without latency go last
			if nodes[j].Latency == 0 && nodes[j+1].Latency > 0 {
				nodes[j], nodes[j+1] = nodes[j+1], nodes[j]
			} else if nodes[j].Latency > 0 && nodes[j+1].Latency > 0 && nodes[j].Latency > nodes[j+1].Latency {
				nodes[j], nodes[j+1] = nodes[j+1], nodes[j]
			}
		}
	}
}

// measureLatency measures TCP connection latency to an address
func measureLatency(ctx context.Context, address string, timeout time.Duration) (time.Duration, error) {
	if timeout == 0 {
		timeout = 2 * time.Second
	}

	dialer := &net.Dialer{
		Timeout: timeout,
	}

	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return 0, err
	}
	latency := time.Since(start)
	conn.Close()

	return latency, nil
}
