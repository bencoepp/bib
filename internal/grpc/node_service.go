// Package grpc provides gRPC service implementations for the bib daemon.
package grpc

import (
	"context"
	"time"

	bibv1 "bib/api/gen/go/bib/v1"
	services "bib/api/gen/go/bib/v1/services"
	"bib/internal/domain"
	"bib/internal/p2p"
	"bib/internal/storage"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// NodeServiceServer implements the NodeService gRPC service.
type NodeServiceServer struct {
	services.UnimplementedNodeServiceServer
	nodeManager  p2p.NodeManager
	store        storage.Store
	auditLogger  *AuditMiddleware
	eventBufSize int
}

// NodeServiceConfig holds configuration for the NodeServiceServer.
type NodeServiceConfig struct {
	NodeManager     p2p.NodeManager
	Store           storage.Store
	AuditLogger     *AuditMiddleware
	EventBufferSize int
}

// NewNodeServiceServer creates a new NodeServiceServer.
func NewNodeServiceServer() *NodeServiceServer {
	return &NodeServiceServer{
		eventBufSize: 100,
	}
}

// NewNodeServiceServerWithConfig creates a new NodeServiceServer with dependencies.
func NewNodeServiceServerWithConfig(cfg NodeServiceConfig) *NodeServiceServer {
	bufSize := cfg.EventBufferSize
	if bufSize <= 0 {
		bufSize = 100
	}
	return &NodeServiceServer{
		nodeManager:  cfg.NodeManager,
		store:        cfg.Store,
		auditLogger:  cfg.AuditLogger,
		eventBufSize: bufSize,
	}
}

// GetNode returns information about a specific node by ID.
func (s *NodeServiceServer) GetNode(ctx context.Context, req *services.GetNodeRequest) (*services.GetNodeResponse, error) {
	if s.nodeManager == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.NodeId == "" {
		return nil, NewValidationError("node_id is required", map[string]string{
			"node_id": "must not be empty",
		})
	}

	peerID, err := peer.Decode(req.NodeId)
	if err != nil {
		return nil, NewValidationError("invalid node_id", map[string]string{
			"node_id": "must be a valid peer ID",
		})
	}

	info, err := s.nodeManager.GetPeerInfo(peerID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "node not found: %v", err)
	}

	// Also check database for additional info
	var dbNode *storage.NodeInfo
	if s.store != nil {
		dbNode, _ = s.store.Nodes().Get(ctx, req.NodeId)
	}

	return &services.GetNodeResponse{
		Node: nodeManagerInfoToProto(info, dbNode),
	}, nil
}

// ListNodes lists known nodes in the network.
func (s *NodeServiceServer) ListNodes(ctx context.Context, req *services.ListNodesRequest) (*services.ListNodesResponse, error) {
	if s.nodeManager == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	// Get nodes from P2P layer
	var p2pNodes []*p2p.NodeManagerInfo
	var err error

	if req.ConnectedOnly {
		p2pNodes, err = s.nodeManager.ListConnectedPeers()
	} else {
		p2pNodes, err = s.nodeManager.ListKnownPeers()
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list nodes: %v", err)
	}

	// Get nodes from database
	var dbNodes []*storage.NodeInfo
	if s.store != nil {
		filter := storage.NodeFilter{
			Mode:        req.Mode,
			TrustedOnly: req.AuthoritativeOnly,
		}
		if req.Page != nil {
			filter.Limit = int(req.Page.Limit)
			filter.Offset = int(req.Page.Offset)
		}
		if filter.Limit <= 0 {
			filter.Limit = 50
		}
		dbNodes, _ = s.store.Nodes().List(ctx, filter)
	}

	// Merge P2P and DB nodes
	nodeMap := make(map[string]*services.NodeInfo)

	// Add P2P nodes first
	for _, n := range p2pNodes {
		// Apply filters
		if req.Mode != "" && n.Mode != req.Mode {
			continue
		}
		if req.AuthoritativeOnly && !n.IsAuthoritative {
			continue
		}
		nodeMap[n.PeerID.String()] = nodeManagerInfoToProto(n, nil)
	}

	// Merge DB nodes (add missing ones, enhance existing)
	for _, dbNode := range dbNodes {
		if existing, ok := nodeMap[dbNode.PeerID]; ok {
			// Enhance with DB info
			existing.StorageType = dbNode.StorageType
			existing.IsAuthoritative = dbNode.TrustedStorage
		} else {
			// Add from DB only
			nodeMap[dbNode.PeerID] = dbNodeToProto(dbNode)
		}
	}

	// Convert to slice
	nodes := make([]*services.NodeInfo, 0, len(nodeMap))
	for _, n := range nodeMap {
		nodes = append(nodes, n)
	}

	// Apply pagination
	offset := 0
	limit := 50
	if req.Page != nil {
		offset = int(req.Page.Offset)
		limit = int(req.Page.Limit)
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}

	total := int64(len(nodes))
	if offset >= len(nodes) {
		nodes = []*services.NodeInfo{}
	} else {
		end := offset + limit
		if end > len(nodes) {
			end = len(nodes)
		}
		nodes = nodes[offset:end]
	}

	return &services.ListNodesResponse{
		Nodes: nodes,
		PageInfo: &bibv1.PageInfo{
			TotalCount: total,
			HasMore:    int64(offset+len(nodes)) < total,
			PageSize:   int32(len(nodes)),
		},
	}, nil
}

// GetSelfNode returns this node's information.
func (s *NodeServiceServer) GetSelfNode(ctx context.Context, req *services.GetSelfNodeRequest) (*services.GetSelfNodeResponse, error) {
	if s.nodeManager == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	info, err := s.nodeManager.GetSelfInfo()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get self info: %v", err)
	}

	resp := &services.GetSelfNodeResponse{
		Node: nodeManagerInfoToProto(info, nil),
	}

	// Separate public and private addresses
	for _, addr := range info.Addresses {
		addrStr := addr.String()
		// Simple heuristic: private addresses typically start with /ip4/10., /ip4/192.168., /ip4/172., /ip6/fd, /ip6/fe80
		if isPrivateAddr(addrStr) {
			resp.PrivateAddresses = append(resp.PrivateAddresses, addrStr)
		} else {
			resp.PublicAddresses = append(resp.PublicAddresses, addrStr)
		}
	}

	return resp, nil
}

// ConnectPeer manually connects to a peer by multiaddr.
func (s *NodeServiceServer) ConnectPeer(ctx context.Context, req *services.ConnectPeerRequest) (*services.ConnectPeerResponse, error) {
	if s.nodeManager == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.Multiaddr == "" {
		return nil, NewValidationError("multiaddr is required", map[string]string{
			"multiaddr": "must not be empty",
		})
	}

	addr, err := multiaddr.NewMultiaddr(req.Multiaddr)
	if err != nil {
		return nil, NewValidationError("invalid multiaddr", map[string]string{
			"multiaddr": err.Error(),
		})
	}

	info, err := s.nodeManager.Connect(ctx, addr)
	if err != nil {
		if _, ok := err.(*p2p.PeerBannedError); ok {
			return &services.ConnectPeerResponse{
				Success: false,
				Error:   "peer is banned",
			}, nil
		}
		return &services.ConnectPeerResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Audit log
	if s.auditLogger != nil {
		s.auditLogger.LogMutation(ctx, "CREATE", "peer_connection", info.PeerID.String(), "Connected to peer: "+req.Multiaddr)
	}

	return &services.ConnectPeerResponse{
		Success: true,
		Node:    nodeManagerInfoToProto(info, nil),
	}, nil
}

// DisconnectPeer disconnects from a peer.
func (s *NodeServiceServer) DisconnectPeer(ctx context.Context, req *services.DisconnectPeerRequest) (*services.DisconnectPeerResponse, error) {
	if s.nodeManager == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.PeerId == "" {
		return nil, NewValidationError("peer_id is required", map[string]string{
			"peer_id": "must not be empty",
		})
	}

	peerID, err := peer.Decode(req.PeerId)
	if err != nil {
		return nil, NewValidationError("invalid peer_id", map[string]string{
			"peer_id": "must be a valid peer ID",
		})
	}

	if err := s.nodeManager.Disconnect(peerID); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to disconnect: %v", err)
	}

	// Audit log
	if s.auditLogger != nil {
		s.auditLogger.LogMutation(ctx, "DELETE", "peer_connection", req.PeerId, "Disconnected from peer")
	}

	return &services.DisconnectPeerResponse{
		Success: true,
	}, nil
}

// GetNetworkStats returns network statistics.
func (s *NodeServiceServer) GetNetworkStats(ctx context.Context, req *services.GetNetworkStatsRequest) (*services.GetNetworkStatsResponse, error) {
	if s.nodeManager == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	stats, err := s.nodeManager.GetNetworkStats()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get network stats: %v", err)
	}

	resp := &services.GetNetworkStatsResponse{
		ConnectedPeers:     stats.ConnectedPeers,
		KnownPeers:         stats.KnownPeers,
		TotalBytesSent:     stats.TotalBytesSent,
		TotalBytesReceived: stats.TotalBytesReceived,
		ActiveStreams:      stats.ActiveStreams,
		DhtSize:            stats.DHTSize,
		BootstrapConnected: stats.BootstrapConnected,
	}

	if stats.Bandwidth != nil {
		resp.Bandwidth = &services.BandwidthStats{
			RateIn:   stats.Bandwidth.RateIn,
			RateOut:  stats.Bandwidth.RateOut,
			TotalIn:  stats.Bandwidth.TotalIn,
			TotalOut: stats.Bandwidth.TotalOut,
		}
	}

	if req.IncludeProtocolStats && stats.ProtocolStats != nil {
		resp.ProtocolStats = make(map[string]*services.ProtocolStats)
		for id, ps := range stats.ProtocolStats {
			resp.ProtocolStats[id] = &services.ProtocolStats{
				ProtocolId:       ps.ProtocolID,
				StreamCount:      ps.StreamCount,
				MessagesSent:     ps.MessagesSent,
				MessagesReceived: ps.MessagesReceived,
				BytesSent:        ps.BytesSent,
				BytesReceived:    ps.BytesReceived,
			}
		}
	}

	return resp, nil
}

// StreamNodeEvents streams node join/leave events.
func (s *NodeServiceServer) StreamNodeEvents(req *services.StreamNodeEventsRequest, stream services.NodeService_StreamNodeEventsServer) error {
	if s.nodeManager == nil {
		return status.Error(codes.Unavailable, "service not initialized")
	}

	ctx := stream.Context()

	eventCh, err := s.nodeManager.SubscribeNodeEvents(ctx, s.eventBufSize)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to subscribe to events: %v", err)
	}

	// Filter event types if specified
	filterTypes := make(map[string]bool)
	for _, t := range req.EventTypes {
		filterTypes[t] = true
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-eventCh:
			if !ok {
				return nil // Channel closed
			}

			// Apply filter
			if len(filterTypes) > 0 && !filterTypes[string(event.Type)] {
				continue
			}

			protoEvent := &services.NodeEvent{
				Type:      string(event.Type),
				Timestamp: timestamppb.New(event.Timestamp),
				Details:   event.Details,
			}
			if event.Node != nil {
				protoEvent.Node = nodeManagerInfoToProto(event.Node, nil)
			}

			if err := stream.Send(protoEvent); err != nil {
				return err
			}
		}
	}
}

// GetPeerInfo returns detailed information about a connected peer.
func (s *NodeServiceServer) GetPeerInfo(ctx context.Context, req *services.GetPeerInfoRequest) (*services.GetPeerInfoResponse, error) {
	if s.nodeManager == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.PeerId == "" {
		return nil, NewValidationError("peer_id is required", map[string]string{
			"peer_id": "must not be empty",
		})
	}

	peerID, err := peer.Decode(req.PeerId)
	if err != nil {
		return nil, NewValidationError("invalid peer_id", map[string]string{
			"peer_id": "must be a valid peer ID",
		})
	}

	info, err := s.nodeManager.GetPeerInfo(peerID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "peer not found: %v", err)
	}

	return &services.GetPeerInfoResponse{
		Node: nodeManagerInfoToProto(info, nil),
		// Connection info would need additional P2P layer support
	}, nil
}

// ListConnectedPeers lists currently connected peers.
func (s *NodeServiceServer) ListConnectedPeers(ctx context.Context, req *services.ListConnectedPeersRequest) (*services.ListConnectedPeersResponse, error) {
	if s.nodeManager == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	peers, err := s.nodeManager.ListConnectedPeers()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list peers: %v", err)
	}

	// Apply pagination
	offset := 0
	limit := 50
	if req.Page != nil {
		offset = int(req.Page.Offset)
		limit = int(req.Page.Limit)
	}
	if limit <= 0 {
		limit = 50
	}

	total := int64(len(peers))
	if offset >= len(peers) {
		peers = []*p2p.NodeManagerInfo{}
	} else {
		end := offset + limit
		if end > len(peers) {
			end = len(peers)
		}
		peers = peers[offset:end]
	}

	protoPeers := make([]*services.NodeInfo, len(peers))
	for i, p := range peers {
		protoPeers[i] = nodeManagerInfoToProto(p, nil)
	}

	return &services.ListConnectedPeersResponse{
		Peers: protoPeers,
		PageInfo: &bibv1.PageInfo{
			TotalCount: total,
			HasMore:    int64(offset+len(peers)) < total,
			PageSize:   int32(len(peers)),
		},
	}, nil
}

// BanPeer bans a peer.
func (s *NodeServiceServer) BanPeer(ctx context.Context, req *services.BanPeerRequest) (*services.BanPeerResponse, error) {
	if s.nodeManager == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.PeerId == "" {
		return nil, NewValidationError("peer_id is required", map[string]string{
			"peer_id": "must not be empty",
		})
	}

	peerID, err := peer.Decode(req.PeerId)
	if err != nil {
		return nil, NewValidationError("invalid peer_id", map[string]string{
			"peer_id": "must be a valid peer ID",
		})
	}

	// Duration: 0 = permanent
	duration := time.Duration(0)
	if req.DurationSeconds > 0 {
		duration = time.Duration(req.DurationSeconds) * time.Second
	}

	if err := s.nodeManager.BanPeer(peerID, req.Reason, duration); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to ban peer: %v", err)
	}

	// Persist to database
	if s.store != nil {
		var expiresAt *time.Time
		if duration > 0 {
			t := time.Now().Add(duration)
			expiresAt = &t
		}

		user, _ := UserFromContext(ctx)
		var bannedBy domain.UserID
		if user != nil {
			bannedBy = user.ID
		}

		ban := &storage.BannedPeer{
			PeerID:    req.PeerId,
			Reason:    req.Reason,
			BannedBy:  bannedBy,
			BannedAt:  time.Now(),
			ExpiresAt: expiresAt,
		}
		s.store.BannedPeers().Create(ctx, ban)
	}

	// Audit log
	if s.auditLogger != nil {
		s.auditLogger.LogMutation(ctx, "CREATE", "banned_peer", req.PeerId, "Banned peer. Reason: "+req.Reason)
	}

	return &services.BanPeerResponse{
		Success: true,
	}, nil
}

// UnbanPeer removes a peer ban.
func (s *NodeServiceServer) UnbanPeer(ctx context.Context, req *services.UnbanPeerRequest) (*services.UnbanPeerResponse, error) {
	if s.nodeManager == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.PeerId == "" {
		return nil, NewValidationError("peer_id is required", map[string]string{
			"peer_id": "must not be empty",
		})
	}

	peerID, err := peer.Decode(req.PeerId)
	if err != nil {
		return nil, NewValidationError("invalid peer_id", map[string]string{
			"peer_id": "must be a valid peer ID",
		})
	}

	if err := s.nodeManager.UnbanPeer(peerID); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unban peer: %v", err)
	}

	// Remove from database
	if s.store != nil {
		s.store.BannedPeers().Delete(ctx, req.PeerId)
	}

	// Audit log
	if s.auditLogger != nil {
		s.auditLogger.LogMutation(ctx, "DELETE", "banned_peer", req.PeerId, "Unbanned peer")
	}

	return &services.UnbanPeerResponse{
		Success: true,
	}, nil
}

// ListBannedPeers lists banned peers.
func (s *NodeServiceServer) ListBannedPeers(ctx context.Context, req *services.ListBannedPeersRequest) (*services.ListBannedPeersResponse, error) {
	if s.nodeManager == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	// Get from in-memory ban list
	bans, err := s.nodeManager.ListBannedPeers()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list banned peers: %v", err)
	}

	// Also get from database to include persisted bans
	if s.store != nil {
		filter := storage.BannedPeerFilter{
			IncludeExpired: false,
		}
		if req.Page != nil {
			filter.Limit = int(req.Page.Limit)
			filter.Offset = int(req.Page.Offset)
		}
		dbBans, _ := s.store.BannedPeers().List(ctx, filter)

		// Merge (database bans take precedence for details)
		banMap := make(map[string]*services.BannedPeer)
		for _, b := range bans {
			banMap[b.PeerID.String()] = &services.BannedPeer{
				PeerId:   b.PeerID.String(),
				Reason:   b.Reason,
				BannedAt: timestamppb.New(b.BannedAt),
			}
			if b.ExpiresAt != nil {
				banMap[b.PeerID.String()].ExpiresAt = timestamppb.New(*b.ExpiresAt)
			}
		}
		for _, db := range dbBans {
			if _, exists := banMap[db.PeerID]; !exists {
				bp := &services.BannedPeer{
					PeerId:   db.PeerID,
					Reason:   db.Reason,
					BannedAt: timestamppb.New(db.BannedAt),
				}
				if db.ExpiresAt != nil {
					bp.ExpiresAt = timestamppb.New(*db.ExpiresAt)
				}
				banMap[db.PeerID] = bp
			}
		}

		// Convert to slice
		result := make([]*services.BannedPeer, 0, len(banMap))
		for _, b := range banMap {
			result = append(result, b)
		}

		return &services.ListBannedPeersResponse{
			Peers: result,
			PageInfo: &bibv1.PageInfo{
				TotalCount: int64(len(result)),
				PageSize:   int32(len(result)),
			},
		}, nil
	}

	// No database, just return in-memory bans
	result := make([]*services.BannedPeer, len(bans))
	for i, b := range bans {
		result[i] = &services.BannedPeer{
			PeerId:   b.PeerID.String(),
			Reason:   b.Reason,
			BannedAt: timestamppb.New(b.BannedAt),
		}
		if b.ExpiresAt != nil {
			result[i].ExpiresAt = timestamppb.New(*b.ExpiresAt)
		}
	}

	return &services.ListBannedPeersResponse{
		Peers: result,
		PageInfo: &bibv1.PageInfo{
			TotalCount: int64(len(result)),
			PageSize:   int32(len(result)),
		},
	}, nil
}

// =============================================================================
// Helper functions
// =============================================================================

func nodeManagerInfoToProto(info *p2p.NodeManagerInfo, dbNode *storage.NodeInfo) *services.NodeInfo {
	if info == nil {
		return nil
	}

	addrs := make([]string, len(info.Addresses))
	for i, a := range info.Addresses {
		addrs[i] = a.String()
	}

	node := &services.NodeInfo{
		Id:              info.PeerID.String(),
		Mode:            info.Mode,
		Version:         info.Version,
		Addresses:       addrs,
		Connected:       info.Connected,
		LatencyMs:       info.LatencyMs,
		Protocols:       info.Protocols,
		AgentVersion:    info.AgentVersion,
		IsBootstrap:     info.IsBootstrap,
		StorageType:     info.StorageType,
		IsAuthoritative: info.IsAuthoritative,
		DatasetCount:    info.DatasetCount,
		Reputation:      info.Reputation,
		Metadata:        info.Metadata,
	}

	if !info.DiscoveredAt.IsZero() {
		node.DiscoveredAt = timestamppb.New(info.DiscoveredAt)
	}
	if !info.LastSeen.IsZero() {
		node.LastSeen = timestamppb.New(info.LastSeen)
	}

	// Enhance with DB info if available
	if dbNode != nil {
		if node.Mode == "" {
			node.Mode = dbNode.Mode
		}
		if node.StorageType == "" {
			node.StorageType = dbNode.StorageType
		}
		node.IsAuthoritative = dbNode.TrustedStorage
	}

	return node
}

func dbNodeToProto(node *storage.NodeInfo) *services.NodeInfo {
	if node == nil {
		return nil
	}

	return &services.NodeInfo{
		Id:              node.PeerID,
		Mode:            node.Mode,
		Addresses:       node.Addresses,
		Connected:       false, // DB only, not connected
		StorageType:     node.StorageType,
		IsAuthoritative: node.TrustedStorage,
		LastSeen:        timestamppb.New(node.LastSeen),
		DiscoveredAt:    timestamppb.New(node.CreatedAt),
	}
}

func isPrivateAddr(addr string) bool {
	// Simple heuristic for private addresses
	return contains(addr, "/ip4/10.") ||
		contains(addr, "/ip4/192.168.") ||
		contains(addr, "/ip4/172.16.") ||
		contains(addr, "/ip4/172.17.") ||
		contains(addr, "/ip4/172.18.") ||
		contains(addr, "/ip4/172.19.") ||
		contains(addr, "/ip4/172.20.") ||
		contains(addr, "/ip4/172.21.") ||
		contains(addr, "/ip4/172.22.") ||
		contains(addr, "/ip4/172.23.") ||
		contains(addr, "/ip4/172.24.") ||
		contains(addr, "/ip4/172.25.") ||
		contains(addr, "/ip4/172.26.") ||
		contains(addr, "/ip4/172.27.") ||
		contains(addr, "/ip4/172.28.") ||
		contains(addr, "/ip4/172.29.") ||
		contains(addr, "/ip4/172.30.") ||
		contains(addr, "/ip4/172.31.") ||
		contains(addr, "/ip4/127.") ||
		contains(addr, "/ip6/::1") ||
		contains(addr, "/ip6/fd") ||
		contains(addr, "/ip6/fe80")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
