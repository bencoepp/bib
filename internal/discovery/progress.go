package discovery

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ProgressCallback is called with progress updates during discovery
type ProgressCallback func(stage string, found int)

// DiscoverWithProgress runs discovery with progress callbacks
func (d *Discoverer) DiscoverWithProgress(ctx context.Context, callback ProgressCallback) *DiscoveryResult {
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

	nodeMap := make(map[string]DiscoveredNode)
	addNodes := func(nodes []DiscoveredNode) {
		for _, node := range nodes {
			if existing, ok := nodeMap[node.Address]; ok {
				if node.Latency > 0 && (existing.Latency == 0 || node.Latency < existing.Latency) {
					nodeMap[node.Address] = node
				}
			} else {
				nodeMap[node.Address] = node
			}
		}
	}

	// Localhost discovery
	if callback != nil {
		callback("Scanning localhost...", len(nodeMap))
	}
	nodes, err := d.discoverLocalhost(ctx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("localhost: %w", err))
	}
	addNodes(nodes)

	// mDNS discovery
	if d.opts.EnableMDNS {
		if callback != nil {
			callback("Scanning local network (mDNS)...", len(nodeMap))
		}
		nodes, err := d.discoverMDNS(ctx)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("mDNS: %w", err))
		}
		addNodes(nodes)
	}

	// P2P discovery
	if d.opts.EnableP2P {
		if callback != nil {
			callback("Discovering nearby peers (P2P)...", len(nodeMap))
		}
		nodes, err := d.discoverP2P(ctx)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("P2P: %w", err))
		}
		addNodes(nodes)
	}

	// Convert map to slice
	for _, node := range nodeMap {
		result.Nodes = append(result.Nodes, node)
	}

	sortNodesByLatency(result.Nodes)

	result.Duration = time.Since(start)

	if callback != nil {
		callback("Discovery complete", len(result.Nodes))
	}

	return result
}

// FormatDiscoveryResult formats the discovery result for display
func FormatDiscoveryResult(result *DiscoveryResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Discovered %d node(s) in %s\n\n", len(result.Nodes), result.Duration.Round(time.Millisecond)))

	if len(result.Nodes) == 0 {
		sb.WriteString("No bibd nodes found.\n")
		return sb.String()
	}

	// Group by method
	byMethod := make(map[DiscoveryMethod][]DiscoveredNode)
	for _, node := range result.Nodes {
		byMethod[node.Method] = append(byMethod[node.Method], node)
	}

	methodOrder := []DiscoveryMethod{MethodLocal, MethodMDNS, MethodP2P, MethodManual, MethodPublic}
	methodNames := map[DiscoveryMethod]string{
		MethodLocal:  "Local",
		MethodMDNS:   "Local Network (mDNS)",
		MethodP2P:    "Nearby Peers (P2P)",
		MethodManual: "Manual",
		MethodPublic: "Public Network",
	}

	for _, method := range methodOrder {
		nodes, ok := byMethod[method]
		if !ok || len(nodes) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("%s:\n", methodNames[method]))
		for _, node := range nodes {
			latencyStr := "N/A"
			if node.Latency > 0 {
				latencyStr = node.Latency.Round(time.Millisecond).String()
			}

			sb.WriteString(fmt.Sprintf("  • %s (%s)\n", node.Address, latencyStr))

			if node.NodeInfo != nil {
				if node.NodeInfo.Name != "" {
					sb.WriteString(fmt.Sprintf("    Name: %s\n", node.NodeInfo.Name))
				}
				if node.NodeInfo.Version != "" {
					sb.WriteString(fmt.Sprintf("    Version: %s\n", node.NodeInfo.Version))
				}
				if node.NodeInfo.Mode != "" {
					sb.WriteString(fmt.Sprintf("    Mode: %s\n", node.NodeInfo.Mode))
				}
			}
		}
		sb.WriteString("\n")
	}

	if len(result.Errors) > 0 {
		sb.WriteString("Warnings:\n")
		for _, err := range result.Errors {
			sb.WriteString(fmt.Sprintf("  ⚠ %v\n", err))
		}
	}

	return sb.String()
}

// DiscoverySummary returns a brief summary of discovered nodes
func DiscoverySummary(result *DiscoveryResult) string {
	if len(result.Nodes) == 0 {
		return "No nodes found"
	}

	// Count by method
	local := 0
	mdns := 0
	p2p := 0
	for _, node := range result.Nodes {
		switch node.Method {
		case MethodLocal:
			local++
		case MethodMDNS:
			mdns++
		case MethodP2P:
			p2p++
		}
	}

	parts := []string{}
	if local > 0 {
		parts = append(parts, fmt.Sprintf("%d local", local))
	}
	if mdns > 0 {
		parts = append(parts, fmt.Sprintf("%d mDNS", mdns))
	}
	if p2p > 0 {
		parts = append(parts, fmt.Sprintf("%d P2P", p2p))
	}

	return fmt.Sprintf("%d nodes (%s)", len(result.Nodes), strings.Join(parts, ", "))
}
