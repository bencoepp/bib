// Package grpc provides gRPC service implementations for the bib daemon.
package grpc

import (
	"context"
	"fmt"
	"sync"
	"time"

	services "bib/api/gen/go/bib/v1/services"
	"bib/internal/version"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// HealthServiceServer implements the HealthService gRPC service.
type HealthServiceServer struct {
	services.UnimplementedHealthServiceServer

	mu       sync.RWMutex
	provider HealthProvider
	started  time.Time
}

// NewHealthServiceServer creates a new HealthServiceServer.
func NewHealthServiceServer() *HealthServiceServer {
	return &HealthServiceServer{
		started: time.Now(),
	}
}

// SetProvider sets the health provider for the service.
// This must be called before the service is used.
func (s *HealthServiceServer) SetProvider(provider HealthProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.provider = provider
}

// Check performs a health check.
func (s *HealthServiceServer) Check(ctx context.Context, req *services.HealthCheckRequest) (*services.HealthCheckResponse, error) {
	s.mu.RLock()
	provider := s.provider
	s.mu.RUnlock()

	resp := &services.HealthCheckResponse{
		Status:     services.ServingStatus_SERVING_STATUS_SERVING,
		Components: make(map[string]*services.ComponentHealth),
		Timestamp:  timestamppb.Now(),
	}

	// If no provider is set, we're in a minimal state
	if provider == nil {
		resp.Status = services.ServingStatus_SERVING_STATUS_UNKNOWN
		return resp, nil
	}

	// Check if daemon is running
	if !provider.IsRunning() {
		resp.Status = services.ServingStatus_SERVING_STATUS_NOT_SERVING
		return resp, nil
	}

	// Check all components
	allHealthy := true

	// Check storage component
	storageHealth := s.checkStorageHealth(ctx, provider)
	resp.Components["storage"] = componentStatusToProto(storageHealth)
	if !storageHealth.Healthy {
		allHealthy = false
	}

	// Add sub-components for storage
	for name, subHealth := range storageHealth.SubComponents {
		resp.Components["storage."+name] = componentStatusToProto(subHealth)
	}

	// Check P2P component if enabled
	cfg := provider.HealthConfig()
	if cfg.P2PEnabled {
		p2pHealth := s.checkP2PHealth(ctx, provider)
		resp.Components["p2p"] = componentStatusToProto(p2pHealth)
		if !p2pHealth.Healthy {
			allHealthy = false
		}

		// Add sub-components for P2P
		for name, subHealth := range p2pHealth.SubComponents {
			resp.Components["p2p."+name] = componentStatusToProto(subHealth)
		}
	}

	// Check cluster component if enabled
	if cfg.ClusterEnabled {
		clusterHealth := s.checkClusterHealth(ctx, provider)
		resp.Components["cluster"] = componentStatusToProto(clusterHealth)
		if !clusterHealth.Healthy {
			allHealthy = false
		}

		// Add sub-components for cluster
		for name, subHealth := range clusterHealth.SubComponents {
			resp.Components["cluster."+name] = componentStatusToProto(subHealth)
		}
	}

	if !allHealthy {
		resp.Status = services.ServingStatus_SERVING_STATUS_NOT_SERVING
	}

	return resp, nil
}

// Watch streams health status changes.
func (s *HealthServiceServer) Watch(req *services.HealthCheckRequest, stream services.HealthService_WatchServer) error {
	// Send initial status
	resp, err := s.Check(stream.Context(), req)
	if err != nil {
		return err
	}
	if err := stream.Send(resp); err != nil {
		return err
	}

	// Poll for changes every 5 seconds
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	lastStatus := resp.Status

	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-ticker.C:
			resp, err := s.Check(stream.Context(), req)
			if err != nil {
				return err
			}

			// Only send if status changed
			if resp.Status != lastStatus {
				if err := stream.Send(resp); err != nil {
					return err
				}
				lastStatus = resp.Status
			}
		}
	}
}

// GetNodeInfo returns detailed node information.
func (s *HealthServiceServer) GetNodeInfo(ctx context.Context, req *services.GetNodeInfoRequest) (*services.GetNodeInfoResponse, error) {
	s.mu.RLock()
	provider := s.provider
	started := s.started
	s.mu.RUnlock()

	info := version.Get()

	resp := &services.GetNodeInfoResponse{
		Version:   info.Version,
		Commit:    info.Commit,
		StartedAt: timestamppb.New(started),
		Uptime:    durationpb.New(time.Since(started)),
	}

	// Fill in build time if available
	if !info.BuildTime.IsZero() {
		resp.BuildTime = timestamppb.New(info.BuildTime)
	}

	// Get info from provider if available
	if provider != nil {
		resp.NodeId = provider.NodeID()
		resp.Mode = provider.NodeMode()
		resp.ListenAddresses = provider.ListenAddresses()
		resp.StartedAt = timestamppb.New(provider.StartedAt())
		resp.Uptime = durationpb.New(time.Since(provider.StartedAt()))

		// Get P2P addresses
		if host := provider.P2PHost(); host != nil {
			var p2pAddrs []string
			for _, addr := range host.FullAddrs() {
				p2pAddrs = append(p2pAddrs, addr.String())
			}
			resp.PublicAddresses = p2pAddrs
		}

		// Include components if requested
		if req.IncludeComponents {
			checkReq := &services.HealthCheckRequest{}
			checkResp, _ := s.Check(ctx, checkReq)
			if checkResp != nil {
				resp.Components = checkResp.Components
			}
		}

		// Include network info if requested
		if req.IncludeNetwork {
			resp.Network = s.getNetworkInfo(provider)
		}

		// Include storage info if requested
		if req.IncludeStorage {
			resp.Storage = s.getStorageInfo(ctx, provider)
		}

		// Include cluster info if applicable
		cfg := provider.HealthConfig()
		if cfg.ClusterEnabled {
			resp.Cluster = s.getClusterInfo(provider)
		}
	}

	return resp, nil
}

// Ping is a simple connectivity check.
func (s *HealthServiceServer) Ping(ctx context.Context, req *services.PingRequest) (*services.PingResponse, error) {
	return &services.PingResponse{
		Timestamp: timestamppb.Now(),
		Payload:   req.Payload, // Echo back the payload
	}, nil
}

// checkStorageHealth checks the storage component health.
func (s *HealthServiceServer) checkStorageHealth(ctx context.Context, provider HealthProvider) ComponentHealthStatus {
	status := ComponentHealthStatus{
		Name:          "storage",
		Healthy:       false,
		LastCheck:     time.Now(),
		SubComponents: make(map[string]ComponentHealthStatus),
	}

	store := provider.Store()
	if store == nil {
		status.Message = "storage not initialized"
		return status
	}

	// Check database connectivity
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := store.Ping(pingCtx); err != nil {
		status.Message = "database ping failed: " + err.Error()
		status.SubComponents["database"] = ComponentHealthStatus{
			Name:      "database",
			Healthy:   false,
			Message:   err.Error(),
			LastCheck: time.Now(),
		}
		return status
	}

	status.Healthy = true
	status.Message = "storage operational"
	status.SubComponents["database"] = ComponentHealthStatus{
		Name:      "database",
		Healthy:   true,
		Message:   "connected",
		LastCheck: time.Now(),
	}

	return status
}

// checkP2PHealth checks the P2P component health.
func (s *HealthServiceServer) checkP2PHealth(ctx context.Context, provider HealthProvider) ComponentHealthStatus {
	status := ComponentHealthStatus{
		Name:          "p2p",
		Healthy:       false,
		LastCheck:     time.Now(),
		SubComponents: make(map[string]ComponentHealthStatus),
	}

	host := provider.P2PHost()
	if host == nil {
		status.Message = "P2P host not initialized"
		return status
	}

	status.Healthy = true
	status.Message = "P2P operational"

	// Check host status
	status.SubComponents["host"] = ComponentHealthStatus{
		Name:      "host",
		Healthy:   true,
		Message:   "peer_id=" + host.PeerID().String(),
		LastCheck: time.Now(),
	}

	// Check discovery health
	discovery := provider.P2PDiscovery()
	if discovery != nil {
		discHealth := discovery.GetHealth()

		// Bootstrap health
		status.SubComponents["bootstrap"] = ComponentHealthStatus{
			Name:      "bootstrap",
			Healthy:   discHealth.BootstrapHealthy,
			Message:   boolToStatus(discHealth.BootstrapConnected, "connected", "disconnected"),
			LastCheck: time.Now(),
		}

		// mDNS health
		if discHealth.MDNSEnabled {
			status.SubComponents["mdns"] = ComponentHealthStatus{
				Name:      "mdns",
				Healthy:   discHealth.MDNSHealthy,
				Message:   boolToStatus(discHealth.MDNSHealthy, "running", "not running"),
				LastCheck: time.Now(),
			}
		}

		// DHT health
		if discHealth.DHTEnabled {
			status.SubComponents["dht"] = ComponentHealthStatus{
				Name:      "dht",
				Healthy:   discHealth.DHTHealthy,
				Message:   "routing_table_size=" + itoa(discHealth.DHTRoutingSize),
				LastCheck: time.Now(),
			}
		}
	}

	return status
}

// checkClusterHealth checks the cluster component health.
func (s *HealthServiceServer) checkClusterHealth(ctx context.Context, provider HealthProvider) ComponentHealthStatus {
	status := ComponentHealthStatus{
		Name:          "cluster",
		Healthy:       false,
		LastCheck:     time.Now(),
		SubComponents: make(map[string]ComponentHealthStatus),
	}

	cluster := provider.Cluster()
	if cluster == nil {
		status.Message = "cluster not initialized"
		return status
	}

	clusterStatus := cluster.Status()

	status.Healthy = clusterStatus.HasQuorum
	if clusterStatus.HasQuorum {
		status.Message = "cluster healthy"
	} else {
		status.Message = "no quorum"
	}

	// Raft status
	status.SubComponents["raft"] = ComponentHealthStatus{
		Name:      "raft",
		Healthy:   clusterStatus.HasQuorum,
		Message:   "state=" + string(clusterStatus.State) + " term=" + uitoa(clusterStatus.Term),
		LastCheck: time.Now(),
	}

	// Leadership status
	status.SubComponents["leadership"] = ComponentHealthStatus{
		Name:      "leadership",
		Healthy:   clusterStatus.Leader != "",
		Message:   "leader=" + clusterStatus.Leader,
		LastCheck: time.Now(),
	}

	// Membership status
	healthyMembers := 0
	for _, m := range clusterStatus.Members {
		if m.IsHealthy {
			healthyMembers++
		}
	}
	status.SubComponents["membership"] = ComponentHealthStatus{
		Name:      "membership",
		Healthy:   healthyMembers >= (len(clusterStatus.Members)/2 + 1),
		Message:   itoa(healthyMembers) + "/" + itoa(len(clusterStatus.Members)) + " healthy",
		LastCheck: time.Now(),
	}

	return status
}

// getNetworkInfo builds network information.
func (s *HealthServiceServer) getNetworkInfo(provider HealthProvider) *services.NetworkInfo {
	info := &services.NetworkInfo{}

	host := provider.P2PHost()
	if host == nil {
		return info
	}

	info.ConnectedPeers = int32(host.ConnectedPeersCount())
	info.ActiveStreams = int32(host.ActiveStreamsCount())

	// Get bandwidth stats if available
	bytesSent, bytesRecv, enabled := host.BandwidthStats()
	if enabled {
		info.BytesSent = bytesSent
		info.BytesReceived = bytesRecv
	}

	// Get discovery info
	discovery := provider.P2PDiscovery()
	if discovery != nil {
		discHealth := discovery.GetHealth()
		info.KnownPeers = int32(discHealth.KnownPeers)
		info.BootstrapConnected = discHealth.BootstrapConnected
		info.DhtRoutingTableSize = int32(discHealth.DHTRoutingSize)
	}

	return info
}

// getStorageInfo builds storage information.
func (s *HealthServiceServer) getStorageInfo(ctx context.Context, provider HealthProvider) *services.StorageInfo {
	info := &services.StorageInfo{}

	store := provider.Store()
	if store == nil {
		return info
	}

	info.Backend = store.Backend().String()
	info.IsAuthoritative = store.IsAuthoritative()

	// Get storage stats
	statsCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	stats, err := store.Stats(statsCtx)
	if err != nil {
		info.Healthy = false
		return info
	}

	info.Healthy = stats.Healthy
	info.DatasetCount = stats.DatasetCount
	info.TopicCount = stats.TopicCount
	info.BytesUsed = stats.BytesUsed
	info.BytesAvailable = stats.BytesAvailable

	return info
}

// getClusterInfo builds cluster information.
func (s *HealthServiceServer) getClusterInfo(provider HealthProvider) *services.ClusterInfo {
	info := &services.ClusterInfo{
		Enabled: true,
	}

	cluster := provider.Cluster()
	if cluster == nil {
		info.Enabled = false
		return info
	}

	status := cluster.Status()

	info.Role = string(status.State)
	info.LeaderId = status.Leader
	info.Term = status.Term
	info.MemberCount = int32(len(status.Members))

	healthyMembers := 0
	for _, m := range status.Members {
		if m.IsHealthy {
			healthyMembers++
		}
	}
	info.HealthyMembers = int32(healthyMembers)

	if status.HasQuorum {
		info.State = "healthy"
	} else {
		info.State = "no_quorum"
	}

	return info
}

// componentStatusToProto converts ComponentHealthStatus to proto.
func componentStatusToProto(status ComponentHealthStatus) *services.ComponentHealth {
	protoStatus := services.ServingStatus_SERVING_STATUS_SERVING
	if !status.Healthy {
		protoStatus = services.ServingStatus_SERVING_STATUS_NOT_SERVING
	}

	return &services.ComponentHealth{
		Name:      status.Name,
		Status:    protoStatus,
		Message:   status.Message,
		LastCheck: timestamppb.New(status.LastCheck),
	}
}

// Helper functions
func boolToStatus(b bool, trueVal, falseVal string) string {
	if b {
		return trueVal
	}
	return falseVal
}

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}

func uitoa(i uint64) string {
	return fmt.Sprintf("%d", i)
}
