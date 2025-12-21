package discovery

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	services "bib/api/gen/go/bib/v1/services"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// NetworkHealthStatus represents overall network health
type NetworkHealthStatus string

const (
	NetworkHealthGood     NetworkHealthStatus = "good"
	NetworkHealthDegraded NetworkHealthStatus = "degraded"
	NetworkHealthPoor     NetworkHealthStatus = "poor"
	NetworkHealthOffline  NetworkHealthStatus = "offline"
)

// NetworkHealthResult contains network health information for a node
type NetworkHealthResult struct {
	// Address is the node address
	Address string

	// Status is the overall network health status
	Status NetworkHealthStatus

	// NodeInfo contains basic node information
	NodeInfo *NodeInfo

	// Network contains network statistics
	Network *NetworkStats

	// Error contains any error message
	Error string

	// Duration is how long the health check took
	Duration time.Duration

	// TestedAt is when the check was performed
	TestedAt time.Time
}

// NetworkStats contains network statistics from a node
type NetworkStats struct {
	// ConnectedPeers is the number of connected P2P peers
	ConnectedPeers int32

	// KnownPeers is the number of known peers
	KnownPeers int32

	// BootstrapConnected indicates if connected to bootstrap nodes
	BootstrapConnected bool

	// DHTRoutingTableSize is the size of the DHT routing table
	DHTRoutingTableSize int32

	// ActiveStreams is the number of active P2P streams
	ActiveStreams int32

	// BytesSent is total bytes sent
	BytesSent int64

	// BytesReceived is total bytes received
	BytesReceived int64
}

// NetworkHealthSummary contains aggregated network health information
type NetworkHealthSummary struct {
	// TotalNodes is the number of nodes checked
	TotalNodes int

	// HealthyNodes is the number of healthy nodes
	HealthyNodes int

	// DegradedNodes is the number of degraded nodes
	DegradedNodes int

	// OfflineNodes is the number of offline nodes
	OfflineNodes int

	// TotalConnectedPeers is the sum of connected peers across all nodes
	TotalConnectedPeers int32

	// AverageConnectedPeers is the average number of connected peers
	AverageConnectedPeers float64

	// BootstrapConnected indicates if any node is connected to bootstrap
	BootstrapConnected bool

	// OverallStatus is the overall network health status
	OverallStatus NetworkHealthStatus
}

// NetworkHealthChecker checks network health of bibd nodes
type NetworkHealthChecker struct {
	// Timeout for health check requests
	Timeout time.Duration
}

// NewNetworkHealthChecker creates a new network health checker
func NewNetworkHealthChecker() *NetworkHealthChecker {
	return &NetworkHealthChecker{
		Timeout: 10 * time.Second,
	}
}

// WithTimeout sets the health check timeout
func (c *NetworkHealthChecker) WithTimeout(timeout time.Duration) *NetworkHealthChecker {
	c.Timeout = timeout
	return c
}

// CheckHealth checks network health of a single node
func (c *NetworkHealthChecker) CheckHealth(ctx context.Context, address string) *NetworkHealthResult {
	result := &NetworkHealthResult{
		Address:  address,
		Status:   NetworkHealthOffline,
		TestedAt: time.Now(),
	}

	// Create context with timeout
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}

	start := time.Now()

	// Connect to the node
	conn, err := grpc.DialContext(ctx, address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		result.Error = fmt.Sprintf("connection failed: %v", err)
		result.Duration = time.Since(start)
		return result
	}
	defer conn.Close()

	// Get node info with network stats
	client := services.NewHealthServiceClient(conn)
	resp, err := client.GetNodeInfo(ctx, &services.GetNodeInfoRequest{
		IncludeNetwork: true,
	})
	if err != nil {
		result.Error = fmt.Sprintf("failed to get node info: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	result.Duration = time.Since(start)

	// Extract node info
	result.NodeInfo = &NodeInfo{
		Version: resp.GetVersion(),
		Mode:    resp.GetMode(),
		PeerID:  resp.GetNodeId(),
	}

	// Extract network stats
	if network := resp.GetNetwork(); network != nil {
		result.Network = &NetworkStats{
			ConnectedPeers:      network.GetConnectedPeers(),
			KnownPeers:          network.GetKnownPeers(),
			BootstrapConnected:  network.GetBootstrapConnected(),
			DHTRoutingTableSize: network.GetDhtRoutingTableSize(),
			ActiveStreams:       network.GetActiveStreams(),
			BytesSent:           network.GetBytesSent(),
			BytesReceived:       network.GetBytesReceived(),
		}

		// Determine health status based on network stats
		result.Status = c.determineHealthStatus(result.Network)
	} else {
		// No network info, but connection succeeded
		result.Status = NetworkHealthGood
	}

	return result
}

// CheckHealthMultiple checks network health of multiple nodes in parallel
func (c *NetworkHealthChecker) CheckHealthMultiple(ctx context.Context, addresses []string) []*NetworkHealthResult {
	results := make([]*NetworkHealthResult, len(addresses))
	var wg sync.WaitGroup

	for i, addr := range addresses {
		wg.Add(1)
		go func(idx int, address string) {
			defer wg.Done()
			results[idx] = c.CheckHealth(ctx, address)
		}(i, addr)
	}

	wg.Wait()
	return results
}

// GetSummary creates a summary from multiple health results
func (c *NetworkHealthChecker) GetSummary(results []*NetworkHealthResult) *NetworkHealthSummary {
	summary := &NetworkHealthSummary{
		TotalNodes:    len(results),
		OverallStatus: NetworkHealthGood,
	}

	var totalPeers int32
	healthyCount := 0

	for _, r := range results {
		switch r.Status {
		case NetworkHealthGood:
			summary.HealthyNodes++
			healthyCount++
		case NetworkHealthDegraded:
			summary.DegradedNodes++
			healthyCount++
		case NetworkHealthPoor:
			summary.DegradedNodes++
		default:
			summary.OfflineNodes++
		}

		if r.Network != nil {
			totalPeers += r.Network.ConnectedPeers
			if r.Network.BootstrapConnected {
				summary.BootstrapConnected = true
			}
		}
	}

	summary.TotalConnectedPeers = totalPeers
	if healthyCount > 0 {
		summary.AverageConnectedPeers = float64(totalPeers) / float64(healthyCount)
	}

	// Determine overall status
	if summary.OfflineNodes == summary.TotalNodes {
		summary.OverallStatus = NetworkHealthOffline
	} else if summary.OfflineNodes > 0 || summary.DegradedNodes > summary.HealthyNodes {
		summary.OverallStatus = NetworkHealthDegraded
	} else if !summary.BootstrapConnected && summary.TotalConnectedPeers == 0 {
		summary.OverallStatus = NetworkHealthPoor
	}

	return summary
}

// determineHealthStatus determines health status from network stats
func (c *NetworkHealthChecker) determineHealthStatus(stats *NetworkStats) NetworkHealthStatus {
	if stats == nil {
		return NetworkHealthOffline
	}

	// Good: connected to peers and bootstrap
	if stats.ConnectedPeers > 0 && stats.BootstrapConnected {
		return NetworkHealthGood
	}

	// Degraded: connected to some peers but no bootstrap
	if stats.ConnectedPeers > 0 {
		return NetworkHealthDegraded
	}

	// Poor: no peers but bootstrap connected (might be starting up)
	if stats.BootstrapConnected {
		return NetworkHealthDegraded
	}

	// Poor: no connections at all
	return NetworkHealthPoor
}

// FormatNetworkHealthResult formats a single health result for display
func FormatNetworkHealthResult(result *NetworkHealthResult) string {
	var sb strings.Builder

	// Status icon
	var statusIcon string
	switch result.Status {
	case NetworkHealthGood:
		statusIcon = "✓"
	case NetworkHealthDegraded:
		statusIcon = "⚠"
	case NetworkHealthPoor:
		statusIcon = "✗"
	case NetworkHealthOffline:
		statusIcon = "⊘"
	default:
		statusIcon = "?"
	}

	sb.WriteString(fmt.Sprintf("%s %s", statusIcon, result.Address))

	if result.Status == NetworkHealthOffline {
		sb.WriteString(" - offline")
		if result.Error != "" {
			sb.WriteString(fmt.Sprintf(": %s", result.Error))
		}
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf(" (%s)", result.Duration.Round(time.Millisecond)))

	if result.Network != nil {
		// Peer count
		sb.WriteString(fmt.Sprintf(" - %d peers", result.Network.ConnectedPeers))

		// Bootstrap status
		if result.Network.BootstrapConnected {
			sb.WriteString(" 🔗bootstrap")
		} else {
			sb.WriteString(" ⚡no-bootstrap")
		}

		// DHT status
		if result.Network.DHTRoutingTableSize > 0 {
			sb.WriteString(fmt.Sprintf(" DHT:%d", result.Network.DHTRoutingTableSize))
		}
	}

	if result.NodeInfo != nil {
		if result.NodeInfo.Mode != "" {
			sb.WriteString(fmt.Sprintf(" [%s]", result.NodeInfo.Mode))
		}
	}

	return sb.String()
}

// FormatNetworkHealthResults formats multiple health results for display
func FormatNetworkHealthResults(results []*NetworkHealthResult) string {
	var sb strings.Builder

	// Count statuses
	good := 0
	degraded := 0
	poor := 0
	offline := 0

	for _, r := range results {
		switch r.Status {
		case NetworkHealthGood:
			good++
		case NetworkHealthDegraded:
			degraded++
		case NetworkHealthPoor:
			poor++
		case NetworkHealthOffline:
			offline++
		}
	}

	// Summary line
	sb.WriteString("Network Health Check Results: ")
	parts := []string{}
	if good > 0 {
		parts = append(parts, fmt.Sprintf("%d healthy", good))
	}
	if degraded > 0 {
		parts = append(parts, fmt.Sprintf("%d degraded", degraded))
	}
	if poor > 0 {
		parts = append(parts, fmt.Sprintf("%d poor", poor))
	}
	if offline > 0 {
		parts = append(parts, fmt.Sprintf("%d offline", offline))
	}
	sb.WriteString(strings.Join(parts, ", "))
	sb.WriteString("\n\n")

	// Individual results
	for _, r := range results {
		sb.WriteString(FormatNetworkHealthResult(r))
		sb.WriteString("\n")
	}

	return sb.String()
}

// FormatNetworkHealthSummary formats a health summary for display
func FormatNetworkHealthSummary(summary *NetworkHealthSummary) string {
	var sb strings.Builder

	// Overall status icon
	var statusIcon string
	switch summary.OverallStatus {
	case NetworkHealthGood:
		statusIcon = "✓"
	case NetworkHealthDegraded:
		statusIcon = "⚠"
	case NetworkHealthPoor:
		statusIcon = "✗"
	case NetworkHealthOffline:
		statusIcon = "⊘"
	}

	sb.WriteString(fmt.Sprintf("%s Network Status: %s\n", statusIcon, summary.OverallStatus))
	sb.WriteString(fmt.Sprintf("  Nodes: %d total (%d healthy, %d degraded, %d offline)\n",
		summary.TotalNodes, summary.HealthyNodes, summary.DegradedNodes, summary.OfflineNodes))
	sb.WriteString(fmt.Sprintf("  Peers: %d connected (avg %.1f per node)\n",
		summary.TotalConnectedPeers, summary.AverageConnectedPeers))

	if summary.BootstrapConnected {
		sb.WriteString("  Bootstrap: ✓ connected\n")
	} else {
		sb.WriteString("  Bootstrap: ✗ not connected\n")
	}

	return sb.String()
}

// NetworkHealthStatusIcon returns an icon for the status
func NetworkHealthStatusIcon(status NetworkHealthStatus) string {
	switch status {
	case NetworkHealthGood:
		return "✓"
	case NetworkHealthDegraded:
		return "⚠"
	case NetworkHealthPoor:
		return "✗"
	case NetworkHealthOffline:
		return "⊘"
	default:
		return "?"
	}
}

// NetworkHealthBrief returns a brief one-line summary
func NetworkHealthBrief(results []*NetworkHealthResult) string {
	good := 0
	total := len(results)
	var totalPeers int32

	for _, r := range results {
		if r.Status == NetworkHealthGood || r.Status == NetworkHealthDegraded {
			good++
		}
		if r.Network != nil {
			totalPeers += r.Network.ConnectedPeers
		}
	}

	return fmt.Sprintf("%d/%d nodes healthy, %d peers connected", good, total, totalPeers)
}
