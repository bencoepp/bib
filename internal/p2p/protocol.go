package p2p

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"bib/internal/domain"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

// Protocol IDs
const (
	ProtocolDiscovery = "/bib/discovery/1.0.0"
	ProtocolData      = "/bib/data/1.0.0"
	ProtocolJobs      = "/bib/jobs/1.0.0"
	ProtocolSync      = "/bib/sync/1.0.0"
)

// SupportedProtocols returns all supported protocol versions.
func SupportedProtocols() []protocol.ID {
	return []protocol.ID{
		ProtocolDiscovery,
		ProtocolData,
		ProtocolJobs,
		ProtocolSync,
	}
}

// MessageType identifies the type of protocol message.
type MessageType string

const (
	// Discovery messages
	MsgGetCatalog   MessageType = "get_catalog"
	MsgCatalog      MessageType = "catalog"
	MsgQueryCatalog MessageType = "query_catalog"
	MsgQueryResult  MessageType = "query_result"
	MsgGetPeerInfo  MessageType = "get_peer_info"
	MsgPeerInfo     MessageType = "peer_info"
	MsgAnnounce     MessageType = "announce"
	MsgAnnounceAck  MessageType = "announce_ack"

	// Data messages
	MsgGetDatasetInfo MessageType = "get_dataset_info"
	MsgDatasetInfo    MessageType = "dataset_info"
	MsgGetChunk       MessageType = "get_chunk"
	MsgChunk          MessageType = "chunk"
	MsgGetChunks      MessageType = "get_chunks"
	MsgChunks         MessageType = "chunks"

	// Sync messages
	MsgGetSyncStatus MessageType = "get_sync_status"
	MsgSyncStatus    MessageType = "sync_status"
	MsgSyncState     MessageType = "sync_state"
	MsgSyncStateResp MessageType = "sync_state_response"

	// Jobs messages
	MsgSubmitJob    MessageType = "submit_job"
	MsgJobAccepted  MessageType = "job_accepted"
	MsgGetJobStatus MessageType = "get_job_status"
	MsgJobStatus    MessageType = "job_status"

	// Error
	MsgError MessageType = "error"
)

// Message is the base protocol message structure.
// We use JSON for now; will migrate to protobuf for production.
type Message struct {
	Type      MessageType     `json:"type"`
	RequestID string          `json:"request_id"`
	Payload   json.RawMessage `json:"payload"`
	Error     *ErrorPayload   `json:"error,omitempty"`
}

// ErrorPayload contains error information.
type ErrorPayload struct {
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// ProtocolHandler handles bib protocol messages.
type ProtocolHandler struct {
	host     host.Host
	catalog  *domain.Catalog
	nodeMode NodeMode

	mu       sync.RWMutex
	handlers map[MessageType]MessageHandler

	// Callbacks for mode-specific handling
	onGetCatalog   func() *domain.Catalog
	onQueryCatalog func(req *domain.QueryRequest) (*domain.QueryResult, error)
	onGetDataset   func(id domain.DatasetID) (*domain.Dataset, error)
	onGetChunk     func(datasetID domain.DatasetID, index int) (*domain.Chunk, error)
}

// MessageHandler handles a specific message type.
type MessageHandler func(ctx context.Context, peerID peer.ID, msg *Message) (*Message, error)

// NewProtocolHandler creates a new protocol handler.
func NewProtocolHandler(h host.Host) *ProtocolHandler {
	ph := &ProtocolHandler{
		host:     h,
		handlers: make(map[MessageType]MessageHandler),
		catalog: &domain.Catalog{
			PeerID:      h.ID().String(),
			Entries:     []domain.CatalogEntry{},
			LastUpdated: time.Now(),
		},
	}

	// Register stream handlers for each protocol
	h.SetStreamHandler(ProtocolDiscovery, ph.handleDiscoveryStream)
	h.SetStreamHandler(ProtocolData, ph.handleDataStream)
	h.SetStreamHandler(ProtocolJobs, ph.handleJobsStream)
	h.SetStreamHandler(ProtocolSync, ph.handleSyncStream)

	// Register default message handlers
	ph.registerDefaultHandlers()

	return ph
}

// registerDefaultHandlers registers the default message handlers.
func (ph *ProtocolHandler) registerDefaultHandlers() {
	// Discovery handlers
	ph.handlers[MsgGetCatalog] = ph.handleGetCatalog
	ph.handlers[MsgQueryCatalog] = ph.handleQueryCatalog
	ph.handlers[MsgGetPeerInfo] = ph.handleGetPeerInfo
	ph.handlers[MsgAnnounce] = ph.handleAnnounce

	// Data handlers
	ph.handlers[MsgGetDatasetInfo] = ph.handleGetDatasetInfo
	ph.handlers[MsgGetChunk] = ph.handleGetChunk
	ph.handlers[MsgGetChunks] = ph.handleGetChunks

	// Sync handlers
	ph.handlers[MsgGetSyncStatus] = ph.handleGetSyncStatus
	ph.handlers[MsgSyncState] = ph.handleSyncState

	// Jobs handlers (placeholder)
	ph.handlers[MsgSubmitJob] = ph.handleSubmitJob
	ph.handlers[MsgGetJobStatus] = ph.handleGetJobStatus
}

// SetCatalogProvider sets the callback for getting the local catalog.
func (ph *ProtocolHandler) SetCatalogProvider(fn func() *domain.Catalog) {
	ph.mu.Lock()
	defer ph.mu.Unlock()
	ph.onGetCatalog = fn
}

// SetQueryHandler sets the callback for handling queries.
func (ph *ProtocolHandler) SetQueryHandler(fn func(req *domain.QueryRequest) (*domain.QueryResult, error)) {
	ph.mu.Lock()
	defer ph.mu.Unlock()
	ph.onQueryCatalog = fn
}

// SetDatasetProvider sets the callback for getting dataset info.
func (ph *ProtocolHandler) SetDatasetProvider(fn func(id domain.DatasetID) (*domain.Dataset, error)) {
	ph.mu.Lock()
	defer ph.mu.Unlock()
	ph.onGetDataset = fn
}

// SetChunkProvider sets the callback for getting chunks.
func (ph *ProtocolHandler) SetChunkProvider(fn func(datasetID domain.DatasetID, index int) (*domain.Chunk, error)) {
	ph.mu.Lock()
	defer ph.mu.Unlock()
	ph.onGetChunk = fn
}

// Close removes all stream handlers.
func (ph *ProtocolHandler) Close() {
	ph.host.RemoveStreamHandler(ProtocolDiscovery)
	ph.host.RemoveStreamHandler(ProtocolData)
	ph.host.RemoveStreamHandler(ProtocolJobs)
	ph.host.RemoveStreamHandler(ProtocolSync)
}

// handleStream is the generic stream handler.
func (ph *ProtocolHandler) handleStream(s network.Stream, protocolID string) {
	defer s.Close()

	// Read the message
	msg, err := ph.readMessage(s)
	if err != nil {
		ph.writeError(s, 400, "failed to read message: "+err.Error())
		return
	}

	// Find handler
	ph.mu.RLock()
	handler, ok := ph.handlers[msg.Type]
	ph.mu.RUnlock()

	if !ok {
		ph.writeError(s, 404, "unknown message type: "+string(msg.Type))
		return
	}

	// Handle message
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := handler(ctx, s.Conn().RemotePeer(), msg)
	if err != nil {
		ph.writeError(s, 500, err.Error())
		return
	}

	// Write response
	if resp != nil {
		resp.RequestID = msg.RequestID
		if err := ph.writeMessage(s, resp); err != nil {
			// Log error
		}
	}
}

func (ph *ProtocolHandler) handleDiscoveryStream(s network.Stream) {
	ph.handleStream(s, ProtocolDiscovery)
}

func (ph *ProtocolHandler) handleDataStream(s network.Stream) {
	ph.handleStream(s, ProtocolData)
}

func (ph *ProtocolHandler) handleJobsStream(s network.Stream) {
	ph.handleStream(s, ProtocolJobs)
}

func (ph *ProtocolHandler) handleSyncStream(s network.Stream) {
	ph.handleStream(s, ProtocolSync)
}

// readMessage reads a message from the stream.
func (ph *ProtocolHandler) readMessage(s network.Stream) (*Message, error) {
	// Read length prefix (4 bytes)
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(s, lenBuf); err != nil {
		return nil, err
	}

	length := int(lenBuf[0])<<24 | int(lenBuf[1])<<16 | int(lenBuf[2])<<8 | int(lenBuf[3])

	// Sanity check
	if length > 10*1024*1024 { // 10MB max
		return nil, fmt.Errorf("message too large: %d bytes", length)
	}

	// Read message body
	body := make([]byte, length)
	if _, err := io.ReadFull(s, body); err != nil {
		return nil, err
	}

	var msg Message
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, err
	}

	return &msg, nil
}

// writeMessage writes a message to the stream.
func (ph *ProtocolHandler) writeMessage(s network.Stream, msg *Message) error {
	body, err := json.Marshal(msg)
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

// writeError writes an error response.
func (ph *ProtocolHandler) writeError(s network.Stream, code int, message string) {
	msg := &Message{
		Type: MsgError,
		Error: &ErrorPayload{
			Code:    code,
			Message: message,
		},
	}
	ph.writeMessage(s, msg)
}

// makePayload creates a JSON payload.
func makePayload(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

// Discovery handlers

func (ph *ProtocolHandler) handleGetCatalog(ctx context.Context, peerID peer.ID, msg *Message) (*Message, error) {
	ph.mu.RLock()
	getCatalog := ph.onGetCatalog
	ph.mu.RUnlock()

	var catalog *domain.Catalog
	if getCatalog != nil {
		catalog = getCatalog()
	} else {
		catalog = ph.catalog
	}

	return &Message{
		Type:    MsgCatalog,
		Payload: makePayload(catalog),
	}, nil
}

func (ph *ProtocolHandler) handleQueryCatalog(ctx context.Context, peerID peer.ID, msg *Message) (*Message, error) {
	var req domain.QueryRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return nil, err
	}

	ph.mu.RLock()
	queryHandler := ph.onQueryCatalog
	ph.mu.RUnlock()

	var result *domain.QueryResult
	var err error

	if queryHandler != nil {
		result, err = queryHandler(&req)
		if err != nil {
			return nil, err
		}
	} else {
		// Default: search local catalog
		result = &domain.QueryResult{
			QueryID: req.ID,
			Entries: []domain.CatalogEntry{},
		}
	}

	return &Message{
		Type:    MsgQueryResult,
		Payload: makePayload(result),
	}, nil
}

func (ph *ProtocolHandler) handleGetPeerInfo(ctx context.Context, peerID peer.ID, msg *Message) (*Message, error) {
	info := map[string]interface{}{
		"peer_id":             ph.host.ID().String(),
		"addresses":           ph.host.Addrs(),
		"supported_protocols": SupportedProtocols(),
	}

	return &Message{
		Type:    MsgPeerInfo,
		Payload: makePayload(info),
	}, nil
}

func (ph *ProtocolHandler) handleAnnounce(ctx context.Context, peerID peer.ID, msg *Message) (*Message, error) {
	var announcement struct {
		NewEntries    []domain.CatalogEntry `json:"new_entries"`
		RemovedHashes []string              `json:"removed_hashes"`
	}
	if err := json.Unmarshal(msg.Payload, &announcement); err != nil {
		return nil, err
	}

	// TODO: Process announcement - add to local catalog, trigger sync

	return &Message{
		Type: MsgAnnounceAck,
		Payload: makePayload(map[string]int{
			"entries_received": len(announcement.NewEntries),
		}),
	}, nil
}

// Data handlers

func (ph *ProtocolHandler) handleGetDatasetInfo(ctx context.Context, peerID peer.ID, msg *Message) (*Message, error) {
	var req struct {
		DatasetID string `json:"dataset_id"`
		Hash      string `json:"hash"`
	}
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return nil, err
	}

	ph.mu.RLock()
	getDataset := ph.onGetDataset
	ph.mu.RUnlock()

	if getDataset == nil {
		return nil, domain.ErrDatasetNotFound
	}

	dataset, err := getDataset(domain.DatasetID(req.DatasetID))
	if err != nil {
		return nil, err
	}

	return &Message{
		Type:    MsgDatasetInfo,
		Payload: makePayload(dataset),
	}, nil
}

func (ph *ProtocolHandler) handleGetChunk(ctx context.Context, peerID peer.ID, msg *Message) (*Message, error) {
	var req struct {
		DatasetID  string `json:"dataset_id"`
		ChunkIndex int    `json:"chunk_index"`
	}
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return nil, err
	}

	ph.mu.RLock()
	getChunk := ph.onGetChunk
	ph.mu.RUnlock()

	if getChunk == nil {
		return nil, domain.ErrChunkNotFound
	}

	chunk, err := getChunk(domain.DatasetID(req.DatasetID), req.ChunkIndex)
	if err != nil {
		return nil, err
	}

	return &Message{
		Type:    MsgChunk,
		Payload: makePayload(chunk),
	}, nil
}

func (ph *ProtocolHandler) handleGetChunks(ctx context.Context, peerID peer.ID, msg *Message) (*Message, error) {
	var req struct {
		DatasetID    string `json:"dataset_id"`
		ChunkIndices []int  `json:"chunk_indices"`
	}
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return nil, err
	}

	ph.mu.RLock()
	getChunk := ph.onGetChunk
	ph.mu.RUnlock()

	if getChunk == nil {
		return nil, domain.ErrChunkNotFound
	}

	chunks := make([]*domain.Chunk, 0, len(req.ChunkIndices))
	for _, idx := range req.ChunkIndices {
		chunk, err := getChunk(domain.DatasetID(req.DatasetID), idx)
		if err != nil {
			continue // Skip missing chunks
		}
		chunks = append(chunks, chunk)
	}

	return &Message{
		Type:    MsgChunks,
		Payload: makePayload(chunks),
	}, nil
}

// Sync handlers

func (ph *ProtocolHandler) handleGetSyncStatus(ctx context.Context, peerID peer.ID, msg *Message) (*Message, error) {
	status := domain.SyncStatus{
		InProgress:   false,
		LastSyncTime: time.Now(),
	}

	return &Message{
		Type:    MsgSyncStatus,
		Payload: makePayload(status),
	}, nil
}

func (ph *ProtocolHandler) handleSyncState(ctx context.Context, peerID peer.ID, msg *Message) (*Message, error) {
	var req struct {
		CatalogVersion uint64   `json:"catalog_version"`
		Hashes         []string `json:"hashes"`
	}
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return nil, err
	}

	// Compare with our catalog
	ph.mu.RLock()
	getCatalog := ph.onGetCatalog
	ph.mu.RUnlock()

	var catalog *domain.Catalog
	if getCatalog != nil {
		catalog = getCatalog()
	} else {
		catalog = ph.catalog
	}

	// Find what we have that they don't
	theirHashes := make(map[string]bool)
	for _, h := range req.Hashes {
		theirHashes[h] = true
	}

	var missingHashes []string
	for _, entry := range catalog.Entries {
		if !theirHashes[entry.Hash] {
			missingHashes = append(missingHashes, entry.Hash)
		}
	}

	// Find what they have that we want
	ourHashes := make(map[string]bool)
	for _, entry := range catalog.Entries {
		ourHashes[entry.Hash] = true
	}

	var wantedHashes []string
	for _, h := range req.Hashes {
		if !ourHashes[h] {
			wantedHashes = append(wantedHashes, h)
		}
	}

	return &Message{
		Type: MsgSyncStateResp,
		Payload: makePayload(map[string]interface{}{
			"missing_hashes":  missingHashes,
			"wanted_hashes":   wantedHashes,
			"catalog_version": catalog.Version,
		}),
	}, nil
}

// Jobs handlers (placeholder for Phase 3)

func (ph *ProtocolHandler) handleSubmitJob(ctx context.Context, peerID peer.ID, msg *Message) (*Message, error) {
	// TODO: Implement in Phase 3
	return &Message{
		Type: MsgJobAccepted,
		Payload: makePayload(map[string]interface{}{
			"accepted": false,
			"error":    "job submission not yet implemented",
		}),
	}, nil
}

func (ph *ProtocolHandler) handleGetJobStatus(ctx context.Context, peerID peer.ID, msg *Message) (*Message, error) {
	// TODO: Implement in Phase 3
	return nil, domain.ErrJobNotFound
}
