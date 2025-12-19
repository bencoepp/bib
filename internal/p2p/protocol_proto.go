// Package p2p provides P2P networking functionality using libp2p.
package p2p

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"bib/internal/domain"

	bibv1 "bib/api/gen/go/bib/v1"
	p2ppb "bib/api/gen/go/bib/v1/p2p"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"google.golang.org/protobuf/proto"
)

// Protocol version 2 IDs - uses protobuf serialization
const (
	ProtocolDiscoveryV2 = "/bib/discovery/2.0.0"
	ProtocolDataV2      = "/bib/data/2.0.0"
	ProtocolJobsV2      = "/bib/jobs/2.0.0"
	ProtocolSyncV2      = "/bib/sync/2.0.0"
)

// SupportedProtocolsV2 returns all supported v2 protocol versions.
func SupportedProtocolsV2() []protocol.ID {
	return []protocol.ID{
		ProtocolDiscoveryV2,
		ProtocolDataV2,
		ProtocolJobsV2,
		ProtocolSyncV2,
	}
}

// ProtoProtocolHandler handles bib protocol messages using protobuf serialization.
type ProtoProtocolHandler struct {
	host     host.Host
	nodeMode NodeMode
	version  string

	mu sync.RWMutex

	// Callbacks for mode-specific handling
	onGetCatalog   func() *domain.Catalog
	onQueryCatalog func(topicID, datasetID, pattern string, limit, offset int) ([]domain.CatalogEntry, int, error)
	onGetDataset   func(id domain.DatasetID) (*domain.Dataset, error)
	onGetChunk     func(datasetID domain.DatasetID, index int) (*domain.Chunk, error)
	onGetChunks    func(datasetID domain.DatasetID, indices []int) ([]*domain.Chunk, error)
	onAnnounce     func(entries []domain.CatalogEntry, removedHashes []string) error
	onSyncState    func(catalogVersion uint64, hashes []string) (missing, wanted []string, version uint64, err error)
}

// NewProtoProtocolHandler creates a new protobuf-based protocol handler.
func NewProtoProtocolHandler(h host.Host, mode NodeMode, version string) *ProtoProtocolHandler {
	ph := &ProtoProtocolHandler{
		host:     h,
		nodeMode: mode,
		version:  version,
	}

	// Register stream handlers for each v2 protocol
	h.SetStreamHandler(ProtocolDiscoveryV2, ph.handleDiscoveryStream)
	h.SetStreamHandler(ProtocolDataV2, ph.handleDataStream)
	h.SetStreamHandler(ProtocolJobsV2, ph.handleJobsStream)
	h.SetStreamHandler(ProtocolSyncV2, ph.handleSyncStream)

	return ph
}

// SetCatalogProvider sets the callback for getting the local catalog.
func (ph *ProtoProtocolHandler) SetCatalogProvider(fn func() *domain.Catalog) {
	ph.mu.Lock()
	defer ph.mu.Unlock()
	ph.onGetCatalog = fn
}

// SetQueryHandler sets the callback for handling catalog queries.
func (ph *ProtoProtocolHandler) SetQueryHandler(fn func(topicID, datasetID, pattern string, limit, offset int) ([]domain.CatalogEntry, int, error)) {
	ph.mu.Lock()
	defer ph.mu.Unlock()
	ph.onQueryCatalog = fn
}

// SetDatasetProvider sets the callback for getting dataset info.
func (ph *ProtoProtocolHandler) SetDatasetProvider(fn func(id domain.DatasetID) (*domain.Dataset, error)) {
	ph.mu.Lock()
	defer ph.mu.Unlock()
	ph.onGetDataset = fn
}

// SetChunkProvider sets the callback for getting a single chunk.
func (ph *ProtoProtocolHandler) SetChunkProvider(fn func(datasetID domain.DatasetID, index int) (*domain.Chunk, error)) {
	ph.mu.Lock()
	defer ph.mu.Unlock()
	ph.onGetChunk = fn
}

// SetChunksProvider sets the callback for getting multiple chunks.
func (ph *ProtoProtocolHandler) SetChunksProvider(fn func(datasetID domain.DatasetID, indices []int) ([]*domain.Chunk, error)) {
	ph.mu.Lock()
	defer ph.mu.Unlock()
	ph.onGetChunks = fn
}

// SetAnnounceHandler sets the callback for handling announcements.
func (ph *ProtoProtocolHandler) SetAnnounceHandler(fn func(entries []domain.CatalogEntry, removedHashes []string) error) {
	ph.mu.Lock()
	defer ph.mu.Unlock()
	ph.onAnnounce = fn
}

// SetSyncStateHandler sets the callback for handling sync state requests.
func (ph *ProtoProtocolHandler) SetSyncStateHandler(fn func(catalogVersion uint64, hashes []string) (missing, wanted []string, version uint64, err error)) {
	ph.mu.Lock()
	defer ph.mu.Unlock()
	ph.onSyncState = fn
}

// Close removes all stream handlers.
func (ph *ProtoProtocolHandler) Close() {
	ph.host.RemoveStreamHandler(ProtocolDiscoveryV2)
	ph.host.RemoveStreamHandler(ProtocolDataV2)
	ph.host.RemoveStreamHandler(ProtocolJobsV2)
	ph.host.RemoveStreamHandler(ProtocolSyncV2)
}

// =============================================================================
// Stream Handlers
// =============================================================================

func (ph *ProtoProtocolHandler) handleDiscoveryStream(s network.Stream) {
	defer s.Close()

	req := &p2ppb.DiscoveryRequest{}
	if err := ph.readProto(s, req); err != nil {
		ph.writeDiscoveryError(s, req.RequestId, 400, "failed to read request: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp := ph.handleDiscoveryRequest(ctx, s.Conn().RemotePeer(), req)
	if err := ph.writeProto(s, resp); err != nil {
		// Log error
	}
}

func (ph *ProtoProtocolHandler) handleDataStream(s network.Stream) {
	defer s.Close()

	req := &p2ppb.DataRequest{}
	if err := ph.readProto(s, req); err != nil {
		ph.writeDataError(s, req.RequestId, 400, "failed to read request: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp := ph.handleDataRequest(ctx, s.Conn().RemotePeer(), req)
	if err := ph.writeProto(s, resp); err != nil {
		// Log error
	}
}

func (ph *ProtoProtocolHandler) handleJobsStream(s network.Stream) {
	defer s.Close()

	req := &p2ppb.JobRequest{}
	if err := ph.readProto(s, req); err != nil {
		ph.writeJobError(s, req.RequestId, 400, "failed to read request: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp := ph.handleJobRequest(ctx, s.Conn().RemotePeer(), req)
	if err := ph.writeProto(s, resp); err != nil {
		// Log error
	}
}

func (ph *ProtoProtocolHandler) handleSyncStream(s network.Stream) {
	defer s.Close()

	req := &p2ppb.SyncRequest{}
	if err := ph.readProto(s, req); err != nil {
		ph.writeSyncError(s, req.RequestId, 400, "failed to read request: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp := ph.handleSyncRequest(ctx, s.Conn().RemotePeer(), req)
	if err := ph.writeProto(s, resp); err != nil {
		// Log error
	}
}

// =============================================================================
// Request Handlers
// =============================================================================

func (ph *ProtoProtocolHandler) handleDiscoveryRequest(ctx context.Context, peerID peer.ID, req *p2ppb.DiscoveryRequest) *p2ppb.DiscoveryResponse {
	switch r := req.Request.(type) {
	case *p2ppb.DiscoveryRequest_GetCatalog:
		return ph.handleGetCatalogProto(ctx, peerID, req.RequestId, r.GetCatalog)
	case *p2ppb.DiscoveryRequest_QueryCatalog:
		return ph.handleQueryCatalogProto(ctx, peerID, req.RequestId, r.QueryCatalog)
	case *p2ppb.DiscoveryRequest_GetPeerInfo:
		return ph.handleGetPeerInfoProto(ctx, peerID, req.RequestId, r.GetPeerInfo)
	case *p2ppb.DiscoveryRequest_Announce:
		return ph.handleAnnounceProto(ctx, peerID, req.RequestId, r.Announce)
	default:
		return &p2ppb.DiscoveryResponse{
			RequestId: req.RequestId,
			Success:   false,
			Error:     ErrorToProto(400, "unknown request type", nil),
		}
	}
}

func (ph *ProtoProtocolHandler) handleDataRequest(ctx context.Context, peerID peer.ID, req *p2ppb.DataRequest) *p2ppb.DataResponse {
	switch r := req.Request.(type) {
	case *p2ppb.DataRequest_GetDatasetInfo:
		return ph.handleGetDatasetInfoProto(ctx, peerID, req.RequestId, r.GetDatasetInfo)
	case *p2ppb.DataRequest_GetChunk:
		return ph.handleGetChunkProto(ctx, peerID, req.RequestId, r.GetChunk)
	case *p2ppb.DataRequest_GetChunks:
		return ph.handleGetChunksProto(ctx, peerID, req.RequestId, r.GetChunks)
	default:
		return &p2ppb.DataResponse{
			RequestId: req.RequestId,
			Success:   false,
			Error:     ErrorToProto(400, "unknown request type", nil),
		}
	}
}

func (ph *ProtoProtocolHandler) handleJobRequest(ctx context.Context, peerID peer.ID, req *p2ppb.JobRequest) *p2ppb.JobResponse {
	// Jobs are placeholder - not implemented yet
	return &p2ppb.JobResponse{
		RequestId: req.RequestId,
		Success:   false,
		Error:     ErrorToProto(501, "jobs not implemented", nil),
	}
}

func (ph *ProtoProtocolHandler) handleSyncRequest(ctx context.Context, peerID peer.ID, req *p2ppb.SyncRequest) *p2ppb.SyncResponse {
	switch r := req.Request.(type) {
	case *p2ppb.SyncRequest_GetSyncStatus:
		return ph.handleGetSyncStatusProto(ctx, peerID, req.RequestId, r.GetSyncStatus)
	case *p2ppb.SyncRequest_SyncState:
		return ph.handleSyncStateProto(ctx, peerID, req.RequestId, r.SyncState)
	default:
		return &p2ppb.SyncResponse{
			RequestId: req.RequestId,
			Success:   false,
			Error:     ErrorToProto(400, "unknown request type", nil),
		}
	}
}

// =============================================================================
// Discovery Handlers
// =============================================================================

func (ph *ProtoProtocolHandler) handleGetCatalogProto(ctx context.Context, peerID peer.ID, reqID string, req *p2ppb.GetCatalogRequest) *p2ppb.DiscoveryResponse {
	ph.mu.RLock()
	getCatalog := ph.onGetCatalog
	ph.mu.RUnlock()

	var catalog *domain.Catalog
	if getCatalog != nil {
		catalog = getCatalog()
	}

	if catalog == nil {
		catalog = &domain.Catalog{
			PeerID:      ph.host.ID().String(),
			Entries:     []domain.CatalogEntry{},
			LastUpdated: time.Now(),
		}
	}

	// Filter by version if requested
	if req.SinceVersion > 0 && catalog.Version <= req.SinceVersion {
		// Return empty catalog if no updates
		return &p2ppb.DiscoveryResponse{
			RequestId: reqID,
			Success:   true,
			Response: &p2ppb.DiscoveryResponse_GetCatalog{
				GetCatalog: &p2ppb.GetCatalogResponse{
					Catalog: &bibv1.Catalog{
						PeerId:  catalog.PeerID,
						Version: catalog.Version,
					},
				},
			},
		}
	}

	return &p2ppb.DiscoveryResponse{
		RequestId: reqID,
		Success:   true,
		Response: &p2ppb.DiscoveryResponse_GetCatalog{
			GetCatalog: &p2ppb.GetCatalogResponse{
				Catalog: CatalogToProto(catalog),
			},
		},
	}
}

func (ph *ProtoProtocolHandler) handleQueryCatalogProto(ctx context.Context, peerID peer.ID, reqID string, req *p2ppb.QueryCatalogRequest) *p2ppb.DiscoveryResponse {
	ph.mu.RLock()
	queryHandler := ph.onQueryCatalog
	ph.mu.RUnlock()

	if queryHandler == nil {
		return &p2ppb.DiscoveryResponse{
			RequestId: reqID,
			Success:   false,
			Error:     ErrorToProto(501, "query not supported", nil),
		}
	}

	entries, total, err := queryHandler(req.TopicId, req.DatasetId, req.NamePattern, int(req.Limit), int(req.Offset))
	if err != nil {
		return &p2ppb.DiscoveryResponse{
			RequestId: reqID,
			Success:   false,
			Error:     ErrorToProto(500, err.Error(), nil),
		}
	}

	protoEntries := make([]*bibv1.CatalogEntry, len(entries))
	for i, e := range entries {
		protoEntries[i] = CatalogEntryToProto(&e)
	}

	return &p2ppb.DiscoveryResponse{
		RequestId: reqID,
		Success:   true,
		Response: &p2ppb.DiscoveryResponse_QueryCatalog{
			QueryCatalog: &p2ppb.QueryCatalogResponse{
				Entries:    protoEntries,
				TotalCount: int32(total),
			},
		},
	}
}

func (ph *ProtoProtocolHandler) handleGetPeerInfoProto(ctx context.Context, peerID peer.ID, reqID string, req *p2ppb.GetPeerInfoRequest) *p2ppb.DiscoveryResponse {
	addrs := make([]string, 0)
	for _, addr := range ph.host.Addrs() {
		addrs = append(addrs, addr.String())
	}

	peerInfo := PeerInfoToProto(ph.host.ID().String(), addrs, ph.nodeMode, ph.version, time.Now())

	return &p2ppb.DiscoveryResponse{
		RequestId: reqID,
		Success:   true,
		Response: &p2ppb.DiscoveryResponse_GetPeerInfo{
			GetPeerInfo: &p2ppb.GetPeerInfoResponse{
				PeerInfo:           peerInfo,
				SupportedProtocols: []string{ProtocolDiscoveryV2, ProtocolDataV2, ProtocolSyncV2, ProtocolJobsV2},
			},
		},
	}
}

func (ph *ProtoProtocolHandler) handleAnnounceProto(ctx context.Context, peerID peer.ID, reqID string, req *p2ppb.AnnounceRequest) *p2ppb.DiscoveryResponse {
	ph.mu.RLock()
	announceHandler := ph.onAnnounce
	ph.mu.RUnlock()

	if announceHandler == nil {
		return &p2ppb.DiscoveryResponse{
			RequestId: reqID,
			Success:   true,
			Response: &p2ppb.DiscoveryResponse_Announce{
				Announce: &p2ppb.AnnounceResponse{
					EntriesReceived: int32(len(req.NewEntries)),
				},
			},
		}
	}

	entries := make([]domain.CatalogEntry, len(req.NewEntries))
	for i, e := range req.NewEntries {
		entries[i] = *ProtoToCatalogEntry(e)
	}

	if err := announceHandler(entries, req.RemovedHashes); err != nil {
		return &p2ppb.DiscoveryResponse{
			RequestId: reqID,
			Success:   false,
			Error:     ErrorToProto(500, err.Error(), nil),
		}
	}

	return &p2ppb.DiscoveryResponse{
		RequestId: reqID,
		Success:   true,
		Response: &p2ppb.DiscoveryResponse_Announce{
			Announce: &p2ppb.AnnounceResponse{
				EntriesReceived: int32(len(req.NewEntries)),
			},
		},
	}
}

// =============================================================================
// Data Handlers
// =============================================================================

func (ph *ProtoProtocolHandler) handleGetDatasetInfoProto(ctx context.Context, peerID peer.ID, reqID string, req *p2ppb.GetDatasetInfoRequest) *p2ppb.DataResponse {
	ph.mu.RLock()
	getDataset := ph.onGetDataset
	ph.mu.RUnlock()

	if getDataset == nil {
		return &p2ppb.DataResponse{
			RequestId: reqID,
			Success:   false,
			Error:     ErrorToProto(501, "dataset info not supported", nil),
		}
	}

	datasetID := req.DatasetId
	if datasetID == "" {
		datasetID = req.Hash // Fallback to hash lookup
	}

	dataset, err := getDataset(domain.DatasetID(datasetID))
	if err != nil {
		return &p2ppb.DataResponse{
			RequestId: reqID,
			Success:   false,
			Error:     ErrorToProto(404, err.Error(), nil),
		}
	}

	return &p2ppb.DataResponse{
		RequestId: reqID,
		Success:   true,
		Response: &p2ppb.DataResponse_GetDatasetInfo{
			GetDatasetInfo: &p2ppb.GetDatasetInfoResponse{
				Dataset: DatasetInfoToProto(dataset),
			},
		},
	}
}

func (ph *ProtoProtocolHandler) handleGetChunkProto(ctx context.Context, peerID peer.ID, reqID string, req *p2ppb.GetChunkRequest) *p2ppb.DataResponse {
	ph.mu.RLock()
	getChunk := ph.onGetChunk
	ph.mu.RUnlock()

	if getChunk == nil {
		return &p2ppb.DataResponse{
			RequestId: reqID,
			Success:   false,
			Error:     ErrorToProto(501, "chunk retrieval not supported", nil),
		}
	}

	datasetID := req.DatasetId
	if datasetID == "" {
		datasetID = req.DatasetHash
	}

	chunk, err := getChunk(domain.DatasetID(datasetID), int(req.ChunkIndex))
	if err != nil {
		return &p2ppb.DataResponse{
			RequestId: reqID,
			Success:   false,
			Error:     ErrorToProto(404, err.Error(), nil),
		}
	}

	return &p2ppb.DataResponse{
		RequestId: reqID,
		Success:   true,
		Response: &p2ppb.DataResponse_GetChunk{
			GetChunk: &p2ppb.GetChunkResponse{
				Chunk: ChunkDataToProto(chunk),
			},
		},
	}
}

func (ph *ProtoProtocolHandler) handleGetChunksProto(ctx context.Context, peerID peer.ID, reqID string, req *p2ppb.GetChunksRequest) *p2ppb.DataResponse {
	ph.mu.RLock()
	getChunks := ph.onGetChunks
	getChunk := ph.onGetChunk
	ph.mu.RUnlock()

	datasetID := req.DatasetId
	if datasetID == "" {
		datasetID = req.DatasetHash
	}

	indices := make([]int, len(req.ChunkIndices))
	for i, idx := range req.ChunkIndices {
		indices[i] = int(idx)
	}

	var chunks []*domain.Chunk

	if getChunks != nil {
		var err error
		chunks, err = getChunks(domain.DatasetID(datasetID), indices)
		if err != nil {
			return &p2ppb.DataResponse{
				RequestId: reqID,
				Success:   false,
				Error:     ErrorToProto(404, err.Error(), nil),
			}
		}
	} else if getChunk != nil {
		// Fallback to getting chunks one by one
		chunks = make([]*domain.Chunk, 0, len(indices))
		for _, idx := range indices {
			chunk, err := getChunk(domain.DatasetID(datasetID), idx)
			if err != nil {
				continue // Skip missing chunks
			}
			chunks = append(chunks, chunk)
		}
	} else {
		return &p2ppb.DataResponse{
			RequestId: reqID,
			Success:   false,
			Error:     ErrorToProto(501, "chunk retrieval not supported", nil),
		}
	}

	protoChunks := make([]*p2ppb.ChunkData, len(chunks))
	for i, c := range chunks {
		protoChunks[i] = ChunkDataToProto(c)
	}

	return &p2ppb.DataResponse{
		RequestId: reqID,
		Success:   true,
		Response: &p2ppb.DataResponse_GetChunks{
			GetChunks: &p2ppb.GetChunksResponse{
				Chunks: protoChunks,
			},
		},
	}
}

// =============================================================================
// Sync Handlers
// =============================================================================

func (ph *ProtoProtocolHandler) handleGetSyncStatusProto(ctx context.Context, peerID peer.ID, reqID string, req *p2ppb.GetSyncStatusRequest) *p2ppb.SyncResponse {
	// Return basic sync status
	return &p2ppb.SyncResponse{
		RequestId: reqID,
		Success:   true,
		Response: &p2ppb.SyncResponse_GetSyncStatus{
			GetSyncStatus: &p2ppb.GetSyncStatusResponse{
				InProgress:     false,
				PendingEntries: 0,
				SyncedEntries:  0,
			},
		},
	}
}

func (ph *ProtoProtocolHandler) handleSyncStateProto(ctx context.Context, peerID peer.ID, reqID string, req *p2ppb.SyncStateRequest) *p2ppb.SyncResponse {
	ph.mu.RLock()
	syncHandler := ph.onSyncState
	ph.mu.RUnlock()

	if syncHandler == nil {
		return &p2ppb.SyncResponse{
			RequestId: reqID,
			Success:   true,
			Response: &p2ppb.SyncResponse_SyncState{
				SyncState: &p2ppb.SyncStateResponse{
					MissingHashes: []string{},
					WantedHashes:  []string{},
				},
			},
		}
	}

	missing, wanted, version, err := syncHandler(req.CatalogVersion, req.Hashes)
	if err != nil {
		return &p2ppb.SyncResponse{
			RequestId: reqID,
			Success:   false,
			Error:     ErrorToProto(500, err.Error(), nil),
		}
	}

	return &p2ppb.SyncResponse{
		RequestId: reqID,
		Success:   true,
		Response: &p2ppb.SyncResponse_SyncState{
			SyncState: &p2ppb.SyncStateResponse{
				MissingHashes:  missing,
				WantedHashes:   wanted,
				CatalogVersion: version,
			},
		},
	}
}

// =============================================================================
// Serialization Helpers
// =============================================================================

func (ph *ProtoProtocolHandler) readProto(s network.Stream, msg proto.Message) error {
	// Read length prefix (4 bytes, big-endian)
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(s, lenBuf); err != nil {
		return err
	}

	length := int(lenBuf[0])<<24 | int(lenBuf[1])<<16 | int(lenBuf[2])<<8 | int(lenBuf[3])

	// Sanity check
	if length > 10*1024*1024 { // 10MB max
		return fmt.Errorf("message too large: %d bytes", length)
	}

	// Read message body
	body := make([]byte, length)
	if _, err := io.ReadFull(s, body); err != nil {
		return err
	}

	return proto.Unmarshal(body, msg)
}

func (ph *ProtoProtocolHandler) writeProto(s network.Stream, msg proto.Message) error {
	body, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	// Write length prefix
	length := len(body)
	lenBuf := []byte{
		byte(length >> 24),
		byte(length >> 16),
		byte(length >> 8),
		byte(length),
	}

	if _, err := s.Write(lenBuf); err != nil {
		return err
	}

	_, err = s.Write(body)
	return err
}

func (ph *ProtoProtocolHandler) writeDiscoveryError(s network.Stream, reqID string, code int, message string) {
	resp := &p2ppb.DiscoveryResponse{
		RequestId: reqID,
		Success:   false,
		Error:     ErrorToProto(code, message, nil),
	}
	ph.writeProto(s, resp)
}

func (ph *ProtoProtocolHandler) writeDataError(s network.Stream, reqID string, code int, message string) {
	resp := &p2ppb.DataResponse{
		RequestId: reqID,
		Success:   false,
		Error:     ErrorToProto(code, message, nil),
	}
	ph.writeProto(s, resp)
}

func (ph *ProtoProtocolHandler) writeJobError(s network.Stream, reqID string, code int, message string) {
	resp := &p2ppb.JobResponse{
		RequestId: reqID,
		Success:   false,
		Error:     ErrorToProto(code, message, nil),
	}
	ph.writeProto(s, resp)
}

func (ph *ProtoProtocolHandler) writeSyncError(s network.Stream, reqID string, code int, message string) {
	resp := &p2ppb.SyncResponse{
		RequestId: reqID,
		Success:   false,
		Error:     ErrorToProto(code, message, nil),
	}
	ph.writeProto(s, resp)
}

// =============================================================================
// Client Methods (for making requests to peers)
// =============================================================================

// GetCatalog requests the catalog from a peer.
func (ph *ProtoProtocolHandler) GetCatalog(ctx context.Context, peerID peer.ID, sinceVersion uint64) (*domain.Catalog, error) {
	s, err := ph.host.NewStream(ctx, peerID, ProtocolDiscoveryV2)
	if err != nil {
		return nil, fmt.Errorf("failed to open stream: %w", err)
	}
	defer s.Close()

	req := &p2ppb.DiscoveryRequest{
		RequestId: uuid.New().String(),
		Request: &p2ppb.DiscoveryRequest_GetCatalog{
			GetCatalog: &p2ppb.GetCatalogRequest{
				SinceVersion: sinceVersion,
			},
		},
	}

	if err := ph.writeProto(s, req); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	resp := &p2ppb.DiscoveryResponse{}
	if err := ph.readProto(s, resp); err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if !resp.Success {
		code, msg, _ := ProtoToError(resp.Error)
		return nil, fmt.Errorf("request failed (%d): %s", code, msg)
	}

	getCatalogResp := resp.GetGetCatalog()
	if getCatalogResp == nil {
		return nil, fmt.Errorf("unexpected response type")
	}

	return ProtoToCatalog(getCatalogResp.Catalog), nil
}

// GetPeerInfo requests peer info from a peer.
func (ph *ProtoProtocolHandler) GetPeerInfo(ctx context.Context, peerID peer.ID) (*bibv1.PeerInfo, []string, error) {
	s, err := ph.host.NewStream(ctx, peerID, ProtocolDiscoveryV2)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open stream: %w", err)
	}
	defer s.Close()

	req := &p2ppb.DiscoveryRequest{
		RequestId: uuid.New().String(),
		Request: &p2ppb.DiscoveryRequest_GetPeerInfo{
			GetPeerInfo: &p2ppb.GetPeerInfoRequest{},
		},
	}

	if err := ph.writeProto(s, req); err != nil {
		return nil, nil, fmt.Errorf("failed to write request: %w", err)
	}

	resp := &p2ppb.DiscoveryResponse{}
	if err := ph.readProto(s, resp); err != nil {
		return nil, nil, fmt.Errorf("failed to read response: %w", err)
	}

	if !resp.Success {
		code, msg, _ := ProtoToError(resp.Error)
		return nil, nil, fmt.Errorf("request failed (%d): %s", code, msg)
	}

	peerInfoResp := resp.GetGetPeerInfo()
	if peerInfoResp == nil {
		return nil, nil, fmt.Errorf("unexpected response type")
	}

	return peerInfoResp.PeerInfo, peerInfoResp.SupportedProtocols, nil
}

// GetDatasetInfo requests dataset info from a peer.
func (ph *ProtoProtocolHandler) GetDatasetInfo(ctx context.Context, peerID peer.ID, datasetID string) (*domain.Dataset, error) {
	s, err := ph.host.NewStream(ctx, peerID, ProtocolDataV2)
	if err != nil {
		return nil, fmt.Errorf("failed to open stream: %w", err)
	}
	defer s.Close()

	req := &p2ppb.DataRequest{
		RequestId: uuid.New().String(),
		Request: &p2ppb.DataRequest_GetDatasetInfo{
			GetDatasetInfo: &p2ppb.GetDatasetInfoRequest{
				DatasetId: datasetID,
			},
		},
	}

	if err := ph.writeProto(s, req); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	resp := &p2ppb.DataResponse{}
	if err := ph.readProto(s, resp); err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if !resp.Success {
		code, msg, _ := ProtoToError(resp.Error)
		return nil, fmt.Errorf("request failed (%d): %s", code, msg)
	}

	datasetResp := resp.GetGetDatasetInfo()
	if datasetResp == nil {
		return nil, fmt.Errorf("unexpected response type")
	}

	return ProtoToDatasetInfo(datasetResp.Dataset), nil
}

// GetChunk requests a single chunk from a peer.
func (ph *ProtoProtocolHandler) GetChunk(ctx context.Context, peerID peer.ID, datasetID string, chunkIndex int) (*domain.Chunk, error) {
	s, err := ph.host.NewStream(ctx, peerID, ProtocolDataV2)
	if err != nil {
		return nil, fmt.Errorf("failed to open stream: %w", err)
	}
	defer s.Close()

	req := &p2ppb.DataRequest{
		RequestId: uuid.New().String(),
		Request: &p2ppb.DataRequest_GetChunk{
			GetChunk: &p2ppb.GetChunkRequest{
				DatasetId:  datasetID,
				ChunkIndex: int32(chunkIndex),
			},
		},
	}

	if err := ph.writeProto(s, req); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	resp := &p2ppb.DataResponse{}
	if err := ph.readProto(s, resp); err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if !resp.Success {
		code, msg, _ := ProtoToError(resp.Error)
		return nil, fmt.Errorf("request failed (%d): %s", code, msg)
	}

	chunkResp := resp.GetGetChunk()
	if chunkResp == nil {
		return nil, fmt.Errorf("unexpected response type")
	}

	return ProtoToChunkData(chunkResp.Chunk), nil
}

// GetChunks requests multiple chunks from a peer.
func (ph *ProtoProtocolHandler) GetChunks(ctx context.Context, peerID peer.ID, datasetID string, chunkIndices []int) ([]*domain.Chunk, error) {
	s, err := ph.host.NewStream(ctx, peerID, ProtocolDataV2)
	if err != nil {
		return nil, fmt.Errorf("failed to open stream: %w", err)
	}
	defer s.Close()

	indices := make([]int32, len(chunkIndices))
	for i, idx := range chunkIndices {
		indices[i] = int32(idx)
	}

	req := &p2ppb.DataRequest{
		RequestId: uuid.New().String(),
		Request: &p2ppb.DataRequest_GetChunks{
			GetChunks: &p2ppb.GetChunksRequest{
				DatasetId:    datasetID,
				ChunkIndices: indices,
			},
		},
	}

	if err := ph.writeProto(s, req); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	resp := &p2ppb.DataResponse{}
	if err := ph.readProto(s, resp); err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if !resp.Success {
		code, msg, _ := ProtoToError(resp.Error)
		return nil, fmt.Errorf("request failed (%d): %s", code, msg)
	}

	chunksResp := resp.GetGetChunks()
	if chunksResp == nil {
		return nil, fmt.Errorf("unexpected response type")
	}

	chunks := make([]*domain.Chunk, len(chunksResp.Chunks))
	for i, c := range chunksResp.Chunks {
		chunks[i] = ProtoToChunkData(c)
	}

	return chunks, nil
}
