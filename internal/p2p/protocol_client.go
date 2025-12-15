package p2p

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"bib/internal/domain"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

// ProtocolClient makes protocol requests to other peers.
type ProtocolClient struct {
	host    host.Host
	timeout time.Duration
}

// NewProtocolClient creates a new protocol client.
func NewProtocolClient(h host.Host) *ProtocolClient {
	return &ProtocolClient{
		host:    h,
		timeout: 30 * time.Second,
	}
}

// SetTimeout sets the request timeout.
func (pc *ProtocolClient) SetTimeout(timeout time.Duration) {
	pc.timeout = timeout
}

// sendRequest sends a request to a peer and returns the response.
func (pc *ProtocolClient) sendRequest(ctx context.Context, peerID peer.ID, protocolID protocol.ID, msg *Message) (*Message, error) {
	ctx, cancel := context.WithTimeout(ctx, pc.timeout)
	defer cancel()

	// Open stream
	s, err := pc.host.NewStream(ctx, peerID, protocolID)
	if err != nil {
		return nil, fmt.Errorf("failed to open stream: %w", err)
	}
	defer s.Close()

	// Generate request ID if not set
	if msg.RequestID == "" {
		msg.RequestID = uuid.New().String()
	}

	// Write request
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	length := len(body)
	lenBuf := []byte{
		byte(length >> 24),
		byte(length >> 16),
		byte(length >> 8),
		byte(length),
	}

	if _, err := s.Write(lenBuf); err != nil {
		return nil, err
	}
	if _, err := s.Write(body); err != nil {
		return nil, err
	}

	// Read response
	respLenBuf := make([]byte, 4)
	if _, err := s.Read(respLenBuf); err != nil {
		return nil, err
	}

	respLength := int(respLenBuf[0])<<24 | int(respLenBuf[1])<<16 | int(respLenBuf[2])<<8 | int(respLenBuf[3])
	if respLength > 10*1024*1024 {
		return nil, fmt.Errorf("response too large: %d bytes", respLength)
	}

	respBody := make([]byte, respLength)
	if _, err := s.Read(respBody); err != nil {
		return nil, err
	}

	var resp Message
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("peer error: %s", resp.Error.Message)
	}

	return &resp, nil
}

// Discovery methods

// GetCatalog retrieves a peer's catalog.
func (pc *ProtocolClient) GetCatalog(ctx context.Context, peerID peer.ID) (*domain.Catalog, error) {
	msg := &Message{
		Type:    MsgGetCatalog,
		Payload: makePayload(map[string]interface{}{}),
	}

	resp, err := pc.sendRequest(ctx, peerID, ProtocolDiscovery, msg)
	if err != nil {
		return nil, err
	}

	var catalog domain.Catalog
	if err := json.Unmarshal(resp.Payload, &catalog); err != nil {
		return nil, err
	}

	return &catalog, nil
}

// QueryCatalog queries a peer's catalog.
func (pc *ProtocolClient) QueryCatalog(ctx context.Context, peerID peer.ID, req *domain.QueryRequest) (*domain.QueryResult, error) {
	msg := &Message{
		Type:    MsgQueryCatalog,
		Payload: makePayload(req),
	}

	resp, err := pc.sendRequest(ctx, peerID, ProtocolDiscovery, msg)
	if err != nil {
		return nil, err
	}

	var result domain.QueryResult
	if err := json.Unmarshal(resp.Payload, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetPeerInfo retrieves peer information.
func (pc *ProtocolClient) GetPeerInfo(ctx context.Context, peerID peer.ID) (map[string]interface{}, error) {
	msg := &Message{
		Type:    MsgGetPeerInfo,
		Payload: makePayload(map[string]interface{}{}),
	}

	resp, err := pc.sendRequest(ctx, peerID, ProtocolDiscovery, msg)
	if err != nil {
		return nil, err
	}

	var info map[string]interface{}
	if err := json.Unmarshal(resp.Payload, &info); err != nil {
		return nil, err
	}

	return info, nil
}

// Announce announces new data to a peer.
func (pc *ProtocolClient) Announce(ctx context.Context, peerID peer.ID, entries []domain.CatalogEntry, removedHashes []string) error {
	msg := &Message{
		Type: MsgAnnounce,
		Payload: makePayload(map[string]interface{}{
			"new_entries":    entries,
			"removed_hashes": removedHashes,
		}),
	}

	_, err := pc.sendRequest(ctx, peerID, ProtocolDiscovery, msg)
	return err
}

// Data methods

// GetDatasetInfo retrieves dataset metadata.
func (pc *ProtocolClient) GetDatasetInfo(ctx context.Context, peerID peer.ID, datasetID domain.DatasetID) (*domain.Dataset, error) {
	msg := &Message{
		Type: MsgGetDatasetInfo,
		Payload: makePayload(map[string]interface{}{
			"dataset_id": datasetID,
		}),
	}

	resp, err := pc.sendRequest(ctx, peerID, ProtocolData, msg)
	if err != nil {
		return nil, err
	}

	var dataset domain.Dataset
	if err := json.Unmarshal(resp.Payload, &dataset); err != nil {
		return nil, err
	}

	return &dataset, nil
}

// GetChunk retrieves a single chunk.
func (pc *ProtocolClient) GetChunk(ctx context.Context, peerID peer.ID, datasetID domain.DatasetID, chunkIndex int) (*domain.Chunk, error) {
	msg := &Message{
		Type: MsgGetChunk,
		Payload: makePayload(map[string]interface{}{
			"dataset_id":  datasetID,
			"chunk_index": chunkIndex,
		}),
	}

	resp, err := pc.sendRequest(ctx, peerID, ProtocolData, msg)
	if err != nil {
		return nil, err
	}

	var chunk domain.Chunk
	if err := json.Unmarshal(resp.Payload, &chunk); err != nil {
		return nil, err
	}

	return &chunk, nil
}

// GetChunks retrieves multiple chunks.
func (pc *ProtocolClient) GetChunks(ctx context.Context, peerID peer.ID, datasetID domain.DatasetID, chunkIndices []int) ([]*domain.Chunk, error) {
	msg := &Message{
		Type: MsgGetChunks,
		Payload: makePayload(map[string]interface{}{
			"dataset_id":    datasetID,
			"chunk_indices": chunkIndices,
		}),
	}

	resp, err := pc.sendRequest(ctx, peerID, ProtocolData, msg)
	if err != nil {
		return nil, err
	}

	var chunks []*domain.Chunk
	if err := json.Unmarshal(resp.Payload, &chunks); err != nil {
		return nil, err
	}

	return chunks, nil
}

// Sync methods

// GetSyncStatus retrieves a peer's sync status.
func (pc *ProtocolClient) GetSyncStatus(ctx context.Context, peerID peer.ID) (*domain.SyncStatus, error) {
	msg := &Message{
		Type:    MsgGetSyncStatus,
		Payload: makePayload(map[string]interface{}{}),
	}

	resp, err := pc.sendRequest(ctx, peerID, ProtocolSync, msg)
	if err != nil {
		return nil, err
	}

	var status domain.SyncStatus
	if err := json.Unmarshal(resp.Payload, &status); err != nil {
		return nil, err
	}

	return &status, nil
}

// SyncState exchanges sync state with a peer.
func (pc *ProtocolClient) SyncState(ctx context.Context, peerID peer.ID, catalogVersion uint64, hashes []string) (missingHashes, wantedHashes []string, peerVersion uint64, err error) {
	msg := &Message{
		Type: MsgSyncState,
		Payload: makePayload(map[string]interface{}{
			"catalog_version": catalogVersion,
			"hashes":          hashes,
		}),
	}

	resp, err := pc.sendRequest(ctx, peerID, ProtocolSync, msg)
	if err != nil {
		return nil, nil, 0, err
	}

	var result struct {
		MissingHashes  []string `json:"missing_hashes"`
		WantedHashes   []string `json:"wanted_hashes"`
		CatalogVersion uint64   `json:"catalog_version"`
	}
	if err := json.Unmarshal(resp.Payload, &result); err != nil {
		return nil, nil, 0, err
	}

	return result.MissingHashes, result.WantedHashes, result.CatalogVersion, nil
}

// Jobs methods (placeholder)

// SubmitJob submits a job to a peer.
func (pc *ProtocolClient) SubmitJob(ctx context.Context, peerID peer.ID, job *domain.Job) (bool, string, error) {
	msg := &Message{
		Type: MsgSubmitJob,
		Payload: makePayload(map[string]interface{}{
			"job": job,
		}),
	}

	resp, err := pc.sendRequest(ctx, peerID, ProtocolJobs, msg)
	if err != nil {
		return false, "", err
	}

	var result struct {
		Accepted bool   `json:"accepted"`
		JobID    string `json:"job_id"`
		Error    string `json:"error"`
	}
	if err := json.Unmarshal(resp.Payload, &result); err != nil {
		return false, "", err
	}

	if result.Error != "" {
		return false, "", fmt.Errorf("%s", result.Error)
	}

	return result.Accepted, result.JobID, nil
}
