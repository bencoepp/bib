// Package grpc provides gRPC service implementations for the bib daemon.
package grpc

import (
	"context"

	services "bib/api/gen/go/bib/v1/services"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NodeServiceServer implements the NodeService gRPC service.
type NodeServiceServer struct {
	services.UnimplementedNodeServiceServer
}

// NewNodeServiceServer creates a new NodeServiceServer.
func NewNodeServiceServer() *NodeServiceServer {
	return &NodeServiceServer{}
}

// GetNode returns information about a specific node by ID.
func (s *NodeServiceServer) GetNode(ctx context.Context, req *services.GetNodeRequest) (*services.GetNodeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetNode not implemented")
}

// ListNodes lists known nodes in the network.
func (s *NodeServiceServer) ListNodes(ctx context.Context, req *services.ListNodesRequest) (*services.ListNodesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListNodes not implemented")
}

// GetSelfNode returns this node's information.
func (s *NodeServiceServer) GetSelfNode(ctx context.Context, req *services.GetSelfNodeRequest) (*services.GetSelfNodeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetSelfNode not implemented")
}

// ConnectPeer manually connects to a peer by multiaddr.
func (s *NodeServiceServer) ConnectPeer(ctx context.Context, req *services.ConnectPeerRequest) (*services.ConnectPeerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ConnectPeer not implemented")
}

// DisconnectPeer disconnects from a peer.
func (s *NodeServiceServer) DisconnectPeer(ctx context.Context, req *services.DisconnectPeerRequest) (*services.DisconnectPeerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DisconnectPeer not implemented")
}

// GetNetworkStats returns network statistics.
func (s *NodeServiceServer) GetNetworkStats(ctx context.Context, req *services.GetNetworkStatsRequest) (*services.GetNetworkStatsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetNetworkStats not implemented")
}

// StreamNodeEvents streams node join/leave events.
func (s *NodeServiceServer) StreamNodeEvents(req *services.StreamNodeEventsRequest, stream services.NodeService_StreamNodeEventsServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamNodeEvents not implemented")
}

// GetPeerInfo returns detailed information about a connected peer.
func (s *NodeServiceServer) GetPeerInfo(ctx context.Context, req *services.GetPeerInfoRequest) (*services.GetPeerInfoResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetPeerInfo not implemented")
}

// ListConnectedPeers lists currently connected peers.
func (s *NodeServiceServer) ListConnectedPeers(ctx context.Context, req *services.ListConnectedPeersRequest) (*services.ListConnectedPeersResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListConnectedPeers not implemented")
}

// BanPeer bans a peer.
func (s *NodeServiceServer) BanPeer(ctx context.Context, req *services.BanPeerRequest) (*services.BanPeerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method BanPeer not implemented")
}

// UnbanPeer removes a peer ban.
func (s *NodeServiceServer) UnbanPeer(ctx context.Context, req *services.UnbanPeerRequest) (*services.UnbanPeerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UnbanPeer not implemented")
}

// ListBannedPeers lists banned peers.
func (s *NodeServiceServer) ListBannedPeers(ctx context.Context, req *services.ListBannedPeersRequest) (*services.ListBannedPeersResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListBannedPeers not implemented")
}
