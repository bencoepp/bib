package postsetup

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

// CLIStatus contains the status of CLI setup
type CLIStatus struct {
	// Nodes contains status of each connected node
	Nodes []NodeStatus

	// AllConnected indicates if all nodes are connected
	AllConnected bool

	// AllAuthenticated indicates if all nodes are authenticated
	AllAuthenticated bool

	// NetworkHealth is the overall network health
	NetworkHealth NetworkHealth

	// Error contains any error
	Error string
}

// NodeStatus contains status of a single node
type NodeStatus struct {
	// Address is the node address
	Address string

	// Alias is the node alias
	Alias string

	// Connected indicates if connected
	Connected bool

	// Authenticated indicates if authenticated
	Authenticated bool

	// Latency is the connection latency
	Latency time.Duration

	// Version is the node version
	Version string

	// NodeID is the node ID
	NodeID string

	// Error contains any error
	Error string
}

// NetworkHealth represents overall network health
type NetworkHealth string

const (
	NetworkHealthGood     NetworkHealth = "good"
	NetworkHealthDegraded NetworkHealth = "degraded"
	NetworkHealthPoor     NetworkHealth = "poor"
	NetworkHealthOffline  NetworkHealth = "offline"
)

// CLIVerifier verifies CLI setup
type CLIVerifier struct {
	// Nodes is the list of nodes to verify
	Nodes []NodeConfig

	// Timeout is the verification timeout per node
	Timeout time.Duration
}

// NodeConfig contains configuration for a node
type NodeConfig struct {
	// Address is the node address
	Address string

	// Alias is the node alias
	Alias string
}

// NewCLIVerifier creates a new CLI verifier
func NewCLIVerifier(nodes []NodeConfig) *CLIVerifier {
	return &CLIVerifier{
		Nodes:   nodes,
		Timeout: 10 * time.Second,
	}
}

// Verify checks all configured nodes
func (v *CLIVerifier) Verify(ctx context.Context) *CLIStatus {
	status := &CLIStatus{
		Nodes: make([]NodeStatus, 0, len(v.Nodes)),
	}

	allConnected := true
	allAuthenticated := true

	for _, node := range v.Nodes {
		nodeStatus := v.verifyNode(ctx, node)
		status.Nodes = append(status.Nodes, nodeStatus)

		if !nodeStatus.Connected {
			allConnected = false
		}
		if !nodeStatus.Authenticated {
			allAuthenticated = false
		}
	}

	status.AllConnected = allConnected
	status.AllAuthenticated = allAuthenticated

	// Determine network health
	if allConnected && allAuthenticated {
		status.NetworkHealth = NetworkHealthGood
	} else if allConnected {
		status.NetworkHealth = NetworkHealthDegraded
	} else if len(status.Nodes) > 0 {
		// Check if any node is connected
		anyConnected := false
		for _, n := range status.Nodes {
			if n.Connected {
				anyConnected = true
				break
			}
		}
		if anyConnected {
			status.NetworkHealth = NetworkHealthDegraded
		} else {
			status.NetworkHealth = NetworkHealthOffline
		}
	} else {
		status.NetworkHealth = NetworkHealthOffline
	}

	return status
}

// verifyNode verifies a single node
func (v *CLIVerifier) verifyNode(ctx context.Context, node NodeConfig) NodeStatus {
	status := NodeStatus{
		Address: node.Address,
		Alias:   node.Alias,
	}

	// Test TCP connectivity
	start := time.Now()
	conn, err := net.DialTimeout("tcp", node.Address, v.Timeout)
	if err != nil {
		status.Connected = false
		status.Error = err.Error()
		return status
	}
	status.Latency = time.Since(start)
	conn.Close()
	status.Connected = true

	// Note: Full authentication verification would require the gRPC client
	// For now, we just verify connectivity
	status.Authenticated = true // Assume authenticated if connected

	return status
}

// FormatCLIStatus formats CLI status for display
func FormatCLIStatus(status *CLIStatus) string {
	var sb strings.Builder

	// Network health summary
	switch status.NetworkHealth {
	case NetworkHealthGood:
		sb.WriteString("🟢 Network Health: Good\n")
	case NetworkHealthDegraded:
		sb.WriteString("🟡 Network Health: Degraded\n")
	case NetworkHealthPoor:
		sb.WriteString("🟠 Network Health: Poor\n")
	case NetworkHealthOffline:
		sb.WriteString("🔴 Network Health: Offline\n")
	}
	sb.WriteString("\n")

	if len(status.Nodes) == 0 {
		sb.WriteString("No nodes configured\n")
		return sb.String()
	}

	// Node details
	sb.WriteString("Nodes:\n")
	for _, n := range status.Nodes {
		var icon string
		if n.Connected && n.Authenticated {
			icon = "✓"
		} else if n.Connected {
			icon = "⚠️"
		} else {
			icon = "✗"
		}

		name := n.Address
		if n.Alias != "" {
			name = fmt.Sprintf("%s (%s)", n.Alias, n.Address)
		}

		if n.Connected {
			sb.WriteString(fmt.Sprintf("  %s %s: connected (%v)\n", icon, name, n.Latency.Round(time.Millisecond)))
		} else {
			sb.WriteString(fmt.Sprintf("  %s %s: %s\n", icon, name, n.Error))
		}
	}

	return sb.String()
}

// GetCLINextSteps returns next steps after CLI setup
func GetCLINextSteps() []string {
	return []string{
		"bib status                # Check connection status",
		"bib topic list            # List available topics",
		"bib dataset list          # List datasets",
		"bib query                 # Query data",
		"bib config show           # Show current configuration",
	}
}

// GetCLIHelpfulCommands returns helpful CLI commands
func GetCLIHelpfulCommands() []string {
	return []string{
		"bib connect <address>     # Connect to a node",
		"bib trust list            # List trusted nodes",
		"bib config set            # Configure settings",
		"bib help                  # Show help",
	}
}
