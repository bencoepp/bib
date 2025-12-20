// Package interfaces defines contracts for the gRPC layer.
package interfaces

import (
	"context"
	"time"

	"bib/internal/cluster"
	"bib/internal/p2p"
	"bib/internal/storage"
)

// HealthProvider provides health and status information for the daemon.
// This interface is implemented by the Daemon to provide health checks
// without creating a direct dependency on the daemon package.
type HealthProvider interface {
	// IsRunning returns whether the daemon is running.
	IsRunning() bool

	// StartedAt returns when the daemon started.
	StartedAt() time.Time

	// NodeMode returns the P2P mode: "proxy", "selective", or "full".
	NodeMode() string

	// NodeID returns the node identifier (peer ID or cluster node ID).
	NodeID() string

	// Store returns the storage instance (may be nil if not initialized).
	Store() storage.Store

	// P2PHost returns the P2P host (may be nil if not enabled).
	P2PHost() *p2p.Host

	// P2PDiscovery returns the P2P discovery manager (may be nil if not enabled).
	P2PDiscovery() *p2p.Discovery

	// Cluster returns the cluster instance (may be nil if not enabled).
	Cluster() *cluster.Cluster

	// ListenAddresses returns the gRPC server listen addresses.
	ListenAddresses() []string

	// Config returns relevant configuration for health reporting.
	HealthConfig() HealthProviderConfig
}

// HealthProviderConfig contains configuration relevant to health reporting.
type HealthProviderConfig struct {
	// P2PEnabled indicates if P2P networking is enabled.
	P2PEnabled bool

	// ClusterEnabled indicates if clustering is enabled.
	ClusterEnabled bool

	// BandwidthMetering indicates if bandwidth metering is enabled.
	BandwidthMetering bool
}

// ComponentHealthChecker checks the health of a specific component.
type ComponentHealthChecker interface {
	// CheckHealth performs a health check and returns status details.
	CheckHealth(ctx context.Context) ComponentHealthStatus
}

// ComponentHealthStatus represents the health status of a component.
type ComponentHealthStatus struct {
	// Name is the component name.
	Name string

	// Healthy indicates if the component is healthy.
	Healthy bool

	// Message provides additional details.
	Message string

	// LastCheck is when the check was performed.
	LastCheck time.Time

	// SubComponents contains health status of sub-components.
	SubComponents map[string]ComponentHealthStatus
}

// AuditLogger defines the interface for audit logging in services.
type AuditLogger interface {
	// LogServiceAction logs a service-level action for auditing.
	LogServiceAction(ctx context.Context, action, resourceType, resourceID string, details map[string]interface{}) error

	// LogMutation logs a mutation operation directly from a service.
	// Deprecated: Use LogServiceAction instead.
	LogMutation(ctx context.Context, action, resource, resourceID, description string) error
}
