// Package node implements the NodeService gRPC service.
package node

import (
	"context"

	bibv1 "bib/api/gen/go/bib/v1"
	services "bib/api/gen/go/bib/v1/services"
	grpcerrors "bib/internal/grpc/errors"
	"bib/internal/grpc/interfaces"
	"bib/internal/p2p"
	"bib/internal/storage"

	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Config holds configuration for the node service server.
type Config struct {
	NodeManager     p2p.NodeManager
	Store           storage.Store
	AuditLogger     interfaces.AuditLogger
	EventBufferSize int
}

// Server implements the NodeService gRPC service.
type Server struct {
	services.UnimplementedNodeServiceServer
	nodeManager  p2p.NodeManager
	store        storage.Store
	auditLogger  interfaces.AuditLogger
	eventBufSize int
}

// NewServer creates a new node service server.
func NewServer() *Server {
	return &Server{
		eventBufSize: 100,
	}
}

// NewServerWithConfig creates a new node service server with dependencies.
func NewServerWithConfig(cfg Config) *Server {
	bufSize := cfg.EventBufferSize
	if bufSize <= 0 {
		bufSize = 100
	}
	return &Server{
		nodeManager:  cfg.NodeManager,
		store:        cfg.Store,
		auditLogger:  cfg.AuditLogger,
		eventBufSize: bufSize,
	}
}

// GetNode returns information about a specific node by ID.
func (s *Server) GetNode(ctx context.Context, req *services.GetNodeRequest) (*services.GetNodeResponse, error) {
	if s.nodeManager == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.NodeId == "" {
		return nil, grpcerrors.NewValidationError("node_id is required", map[string]string{
			"node_id": "must not be empty",
		})
	}

	peerID, err := peer.Decode(req.NodeId)
	if err != nil {
		return nil, grpcerrors.NewValidationError("invalid node_id", map[string]string{
			"node_id": "must be a valid peer ID",
		})
	}

	info, err := s.nodeManager.GetPeerInfo(peerID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "node not found: %v", err)
	}

	var dbNode *storage.NodeInfo
	if s.store != nil {
		dbNode, _ = s.store.Nodes().Get(ctx, req.NodeId)
	}

	return &services.GetNodeResponse{
		Node: nodeInfoToProto(info, dbNode),
	}, nil
}

// ListNodes lists known nodes in the network.
func (s *Server) ListNodes(ctx context.Context, req *services.ListNodesRequest) (*services.ListNodesResponse, error) {
	if s.nodeManager == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

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

	nodeMap := make(map[string]*services.NodeInfo)

	for _, n := range p2pNodes {
		if req.Mode != "" && n.Mode != req.Mode {
			continue
		}
		if req.AuthoritativeOnly && !n.IsAuthoritative {
			continue
		}
		nodeMap[n.PeerID.String()] = nodeInfoToProto(n, nil)
	}

	for _, dbNode := range dbNodes {
		if existing, ok := nodeMap[dbNode.PeerID]; ok {
			existing.StorageType = dbNode.StorageType
			existing.IsAuthoritative = dbNode.TrustedStorage
		} else {
			nodeMap[dbNode.PeerID] = dbNodeToProto(dbNode)
		}
	}

	nodes := make([]*services.NodeInfo, 0, len(nodeMap))
	for _, n := range nodeMap {
		nodes = append(nodes, n)
	}

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
func (s *Server) GetSelfNode(ctx context.Context, req *services.GetSelfNodeRequest) (*services.GetSelfNodeResponse, error) {
	if s.nodeManager == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	selfInfo, err := s.nodeManager.GetSelfInfo()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get self info: %v", err)
	}

	return &services.GetSelfNodeResponse{
		Node: nodeInfoToProto(selfInfo, nil),
	}, nil
}

// Conversion helpers

func nodeInfoToProto(info *p2p.NodeManagerInfo, dbNode *storage.NodeInfo) *services.NodeInfo {
	if info == nil {
		return nil
	}

	node := &services.NodeInfo{
		Id:              info.PeerID.String(),
		Mode:            info.Mode,
		IsAuthoritative: info.IsAuthoritative,
		Version:         info.Version,
	}

	for _, addr := range info.Addresses {
		node.Addresses = append(node.Addresses, addr.String())
	}

	if dbNode != nil {
		node.StorageType = dbNode.StorageType
		node.IsAuthoritative = dbNode.TrustedStorage
	}

	return node
}

func dbNodeToProto(n *storage.NodeInfo) *services.NodeInfo {
	if n == nil {
		return nil
	}
	return &services.NodeInfo{
		Id:              n.PeerID,
		Mode:            n.Mode,
		StorageType:     n.StorageType,
		IsAuthoritative: n.TrustedStorage,
		Addresses:       n.Addresses,
	}
}
