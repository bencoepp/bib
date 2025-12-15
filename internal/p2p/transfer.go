package p2p

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"bib/internal/config"
	"bib/internal/domain"

	"github.com/google/uuid"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

// TransferConfig holds data transfer configuration.
type TransferConfig struct {
	// ChunkSize is the size of each chunk in bytes.
	ChunkSize int64

	// MaxConcurrentChunks is the max concurrent chunk downloads per dataset.
	MaxConcurrentChunks int

	// ChunkTimeout is the timeout for downloading a single chunk.
	ChunkTimeout time.Duration

	// MaxRetries is the max retries per chunk.
	MaxRetries int

	// ParallelPeers enables downloading from multiple peers.
	ParallelPeers bool
}

// DefaultTransferConfig returns default transfer configuration.
func DefaultTransferConfig() TransferConfig {
	return TransferConfig{
		ChunkSize:           1024 * 1024, // 1MB default
		MaxConcurrentChunks: 4,
		ChunkTimeout:        30 * time.Second,
		MaxRetries:          3,
		ParallelPeers:       true,
	}
}

// TransferManager manages data transfers.
type TransferManager struct {
	host      host.Host
	client    *ProtocolClient
	peerStore *PeerStore
	cfg       TransferConfig

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu        sync.RWMutex
	downloads map[string]*activeDownload

	// Callbacks
	onChunkReceived func(download *domain.Download, chunk *domain.Chunk)
	onComplete      func(download *domain.Download)
	onError         func(download *domain.Download, err error)
}

// activeDownload tracks an in-progress download.
type activeDownload struct {
	download *domain.Download
	peers    []peer.ID
	cancel   context.CancelFunc
	wg       sync.WaitGroup

	mu          sync.Mutex
	chunkErrors map[int]error
}

// NewTransferManager creates a new transfer manager.
func NewTransferManager(h host.Host, client *ProtocolClient, peerStore *PeerStore, cfg TransferConfig) *TransferManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &TransferManager{
		host:      h,
		client:    client,
		peerStore: peerStore,
		cfg:       cfg,
		ctx:       ctx,
		cancel:    cancel,
		downloads: make(map[string]*activeDownload),
	}
}

// SetChunkCallback sets the callback for received chunks.
func (tm *TransferManager) SetChunkCallback(fn func(download *domain.Download, chunk *domain.Chunk)) {
	tm.onChunkReceived = fn
}

// SetCompleteCallback sets the callback for completed downloads.
func (tm *TransferManager) SetCompleteCallback(fn func(download *domain.Download)) {
	tm.onComplete = fn
}

// SetErrorCallback sets the callback for download errors.
func (tm *TransferManager) SetErrorCallback(fn func(download *domain.Download, err error)) {
	tm.onError = fn
}

// StartDownload starts a new download.
func (tm *TransferManager) StartDownload(ctx context.Context, datasetID domain.DatasetID, hash string, totalChunks int, peers []peer.ID) (*domain.Download, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Check if already downloading
	for _, ad := range tm.downloads {
		if ad.download.DatasetHash == hash {
			return ad.download, nil // Already downloading
		}
	}

	// Create download record
	download := &domain.Download{
		ID:          uuid.New().String(),
		DatasetID:   datasetID,
		DatasetHash: hash,
		TotalChunks: totalChunks,
		ChunkBitmap: make([]byte, (totalChunks+7)/8),
		Status:      domain.DownloadStatusActive,
		StartedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if len(peers) > 0 {
		download.PeerID = peers[0].String()
	}

	// Create active download
	dlCtx, cancel := context.WithCancel(tm.ctx)
	ad := &activeDownload{
		download:    download,
		peers:       peers,
		cancel:      cancel,
		chunkErrors: make(map[int]error),
	}

	tm.downloads[download.ID] = ad

	// Start download goroutine
	tm.wg.Add(1)
	go tm.runDownload(dlCtx, ad)

	return download, nil
}

// ResumeDownload resumes a paused or failed download.
func (tm *TransferManager) ResumeDownload(ctx context.Context, download *domain.Download, peers []peer.ID) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Check if already active
	if _, ok := tm.downloads[download.ID]; ok {
		return nil
	}

	download.Status = domain.DownloadStatusActive
	download.UpdatedAt = time.Now()
	download.Error = ""

	dlCtx, cancel := context.WithCancel(tm.ctx)
	ad := &activeDownload{
		download:    download,
		peers:       peers,
		cancel:      cancel,
		chunkErrors: make(map[int]error),
	}

	tm.downloads[download.ID] = ad

	tm.wg.Add(1)
	go tm.runDownload(dlCtx, ad)

	return nil
}

// PauseDownload pauses an active download.
func (tm *TransferManager) PauseDownload(downloadID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	ad, ok := tm.downloads[downloadID]
	if !ok {
		return domain.ErrDownloadNotFound
	}

	ad.cancel()
	ad.download.Status = domain.DownloadStatusPaused
	ad.download.UpdatedAt = time.Now()

	return nil
}

// CancelDownload cancels and removes a download.
func (tm *TransferManager) CancelDownload(downloadID string) error {
	tm.mu.Lock()
	ad, ok := tm.downloads[downloadID]
	if ok {
		ad.cancel()
		delete(tm.downloads, downloadID)
	}
	tm.mu.Unlock()

	if ok {
		ad.wg.Wait()
	}

	return nil
}

// GetDownload returns download status.
func (tm *TransferManager) GetDownload(downloadID string) *domain.Download {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if ad, ok := tm.downloads[downloadID]; ok {
		return ad.download
	}
	return nil
}

// ListDownloads returns all active downloads.
func (tm *TransferManager) ListDownloads() []*domain.Download {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	downloads := make([]*domain.Download, 0, len(tm.downloads))
	for _, ad := range tm.downloads {
		downloads = append(downloads, ad.download)
	}
	return downloads
}

// Stop stops the transfer manager.
func (tm *TransferManager) Stop() {
	tm.cancel()
	tm.wg.Wait()
}

// runDownload executes the download.
func (tm *TransferManager) runDownload(ctx context.Context, ad *activeDownload) {
	defer tm.wg.Done()
	defer ad.wg.Wait()

	download := ad.download

	// Get missing chunks
	missingChunks := download.MissingChunks()
	if len(missingChunks) == 0 {
		tm.completeDownload(ad)
		return
	}

	// Create work channel
	work := make(chan int, len(missingChunks))
	for _, idx := range missingChunks {
		work <- idx
	}
	close(work)

	// Start workers
	workerCount := tm.cfg.MaxConcurrentChunks
	if workerCount > len(missingChunks) {
		workerCount = len(missingChunks)
	}

	for i := 0; i < workerCount; i++ {
		ad.wg.Add(1)
		go tm.chunkWorker(ctx, ad, work)
	}

	// Wait for workers to finish
	ad.wg.Wait()

	// Check result
	if download.IsComplete() {
		tm.completeDownload(ad)
	} else if ctx.Err() == nil {
		// Not cancelled but incomplete - mark as failed
		download.Status = domain.DownloadStatusFailed
		download.UpdatedAt = time.Now()

		// Collect errors
		ad.mu.Lock()
		for idx, err := range ad.chunkErrors {
			download.Error = fmt.Sprintf("chunk %d: %v", idx, err)
			break // Just take first error
		}
		ad.mu.Unlock()

		if tm.onError != nil {
			tm.onError(download, fmt.Errorf("%s", download.Error))
		}
	}

	// Remove from active downloads
	tm.mu.Lock()
	delete(tm.downloads, download.ID)
	tm.mu.Unlock()
}

// chunkWorker downloads chunks from the work channel.
func (tm *TransferManager) chunkWorker(ctx context.Context, ad *activeDownload, work <-chan int) {
	defer ad.wg.Done()

	for chunkIndex := range work {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := tm.downloadChunk(ctx, ad, chunkIndex); err != nil {
			ad.mu.Lock()
			ad.chunkErrors[chunkIndex] = err
			ad.mu.Unlock()
		}
	}
}

// downloadChunk downloads a single chunk with retries.
func (tm *TransferManager) downloadChunk(ctx context.Context, ad *activeDownload, chunkIndex int) error {
	var lastErr error

	for retry := 0; retry < tm.cfg.MaxRetries; retry++ {
		// Select a peer
		peerID := tm.selectPeer(ad, chunkIndex)
		if peerID == "" {
			return fmt.Errorf("no peers available")
		}

		// Download chunk
		chunk, err := tm.fetchChunk(ctx, peerID, ad.download.DatasetID, chunkIndex)
		if err != nil {
			lastErr = err
			continue
		}

		// Verify chunk hash
		if !tm.verifyChunk(chunk) {
			lastErr = domain.ErrHashMismatch
			continue
		}

		// Mark chunk as complete
		ad.download.SetChunkCompleted(chunkIndex)
		ad.download.UpdatedAt = time.Now()

		// Callback
		if tm.onChunkReceived != nil {
			tm.onChunkReceived(ad.download, chunk)
		}

		return nil
	}

	return lastErr
}

// selectPeer selects a peer to download from.
func (tm *TransferManager) selectPeer(ad *activeDownload, chunkIndex int) peer.ID {
	if len(ad.peers) == 0 {
		return ""
	}

	if tm.cfg.ParallelPeers && len(ad.peers) > 1 {
		// Round-robin across peers
		return ad.peers[chunkIndex%len(ad.peers)]
	}

	return ad.peers[0]
}

// fetchChunk fetches a chunk from a peer.
func (tm *TransferManager) fetchChunk(ctx context.Context, peerID peer.ID, datasetID domain.DatasetID, chunkIndex int) (*domain.Chunk, error) {
	ctx, cancel := context.WithTimeout(ctx, tm.cfg.ChunkTimeout)
	defer cancel()

	return tm.client.GetChunk(ctx, peerID, datasetID, chunkIndex)
}

// verifyChunk verifies chunk integrity.
func (tm *TransferManager) verifyChunk(chunk *domain.Chunk) bool {
	if chunk == nil || len(chunk.Data) == 0 {
		return false
	}

	// Compute hash
	hash := sha256.Sum256(chunk.Data)
	computed := hex.EncodeToString(hash[:])

	return computed == chunk.Hash
}

// completeDownload marks a download as complete.
func (tm *TransferManager) completeDownload(ad *activeDownload) {
	ad.download.Status = domain.DownloadStatusCompleted
	ad.download.UpdatedAt = time.Now()

	if tm.onComplete != nil {
		tm.onComplete(ad.download)
	}
}

// DownloadDataset is a high-level method to download a complete dataset.
func (tm *TransferManager) DownloadDataset(ctx context.Context, entry *domain.CatalogEntry, peers []peer.ID) (*domain.Download, error) {
	// Validate
	if entry == nil {
		return nil, fmt.Errorf("nil catalog entry")
	}
	if len(peers) == 0 {
		return nil, fmt.Errorf("no peers provided")
	}

	return tm.StartDownload(ctx, entry.DatasetID, entry.Hash, entry.ChunkCount, peers)
}

// TransferConfigFromP2PConfig creates transfer config from P2P config.
func TransferConfigFromP2PConfig(cfg config.P2PConfig) TransferConfig {
	tc := DefaultTransferConfig()
	// Could add P2P config options here in the future
	return tc
}
