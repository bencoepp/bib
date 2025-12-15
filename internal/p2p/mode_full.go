package p2p

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"bib/internal/config"
	"bib/internal/domain"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

// FullReplicaHandler handles full replica mode operations.
// In this mode, the node replicates all data from connected peers.
type FullReplicaHandler struct {
	host      host.Host
	discovery *Discovery
	cfg       config.P2PConfig
	configDir string
	client    *ProtocolClient

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// mu protects the fields below
	mu         sync.RWMutex
	catalogs   map[peer.ID]*domain.Catalog
	syncStatus domain.SyncStatus
}

// NewFullReplicaHandler creates a new full replica handler.
func NewFullReplicaHandler(h host.Host, discovery *Discovery, cfg config.P2PConfig, configDir string) (*FullReplicaHandler, error) {
	return &FullReplicaHandler{
		host:      h,
		discovery: discovery,
		cfg:       cfg,
		configDir: configDir,
		catalogs:  make(map[peer.ID]*domain.Catalog),
		client:    NewProtocolClient(h),
	}, nil
}

// Mode returns the handler's mode.
func (h *FullReplicaHandler) Mode() NodeMode {
	return NodeModeFull
}

// Start begins full replica operations.
func (h *FullReplicaHandler) Start(ctx context.Context) error {
	h.ctx, h.cancel = context.WithCancel(ctx)

	// Start periodic sync
	h.wg.Add(1)
	go h.syncLoop()

	// Do an initial sync
	go h.syncAll()

	return nil
}

// Stop stops the handler.
func (h *FullReplicaHandler) Stop() error {
	if h.cancel != nil {
		h.cancel()
	}
	h.wg.Wait()
	return nil
}

// OnConfigUpdate handles configuration changes.
func (h *FullReplicaHandler) OnConfigUpdate(cfg config.P2PConfig) error {
	h.mu.Lock()
	h.cfg = cfg
	h.mu.Unlock()
	return nil
}

// SyncStatus returns the current sync status.
func (h *FullReplicaHandler) SyncStatus() domain.SyncStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.syncStatus
}

// syncLoop runs periodic synchronization.
func (h *FullReplicaHandler) syncLoop() {
	defer h.wg.Done()

	h.mu.RLock()
	interval := h.cfg.FullReplica.SyncInterval
	h.mu.RUnlock()

	if interval == 0 {
		interval = 5 * time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			h.syncAll()
		}
	}
}

// syncAll synchronizes with all connected peers.
func (h *FullReplicaHandler) syncAll() {
	h.mu.Lock()
	h.syncStatus.InProgress = true
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		h.syncStatus.InProgress = false
		h.syncStatus.LastSyncTime = time.Now()
		h.mu.Unlock()
	}()

	// Get all connected peers
	peers := h.host.Network().Peers()

	for _, peerID := range peers {
		select {
		case <-h.ctx.Done():
			return
		default:
		}

		if err := h.syncFromPeer(peerID); err != nil {
			h.mu.Lock()
			h.syncStatus.LastSyncError = err.Error()
			h.mu.Unlock()
			// Continue with other peers
		}
	}
}

// syncFromPeer syncs the catalog from a specific peer.
func (h *FullReplicaHandler) syncFromPeer(peerID peer.ID) error {
	ctx, cancel := context.WithTimeout(h.ctx, 30*time.Second)
	defer cancel()

	// Request catalog from peer
	// TODO: This will use the /bib/discovery/1.0.0 protocol in Phase 1.4
	catalog, err := h.requestCatalog(ctx, peerID)
	if err != nil {
		return err
	}

	// Store the catalog
	h.mu.Lock()
	h.catalogs[peerID] = catalog
	h.mu.Unlock()

	// TODO: In Phase 2, this will trigger actual data replication
	// For now, we just track the catalog

	return nil
}

// requestCatalog requests a peer's catalog using the discovery protocol.
func (h *FullReplicaHandler) requestCatalog(ctx context.Context, peerID peer.ID) (*domain.Catalog, error) {
	return h.client.GetCatalog(ctx, peerID)
}

// GetCatalogs returns all known peer catalogs.
func (h *FullReplicaHandler) GetCatalogs() map[peer.ID]*domain.Catalog {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make(map[peer.ID]*domain.Catalog)
	for k, v := range h.catalogs {
		result[k] = v
	}
	return result
}

// GetAllEntries returns all catalog entries from all peers (deduplicated by hash).
func (h *FullReplicaHandler) GetAllEntries() []domain.CatalogEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	seen := make(map[string]bool)
	var entries []domain.CatalogEntry

	for _, catalog := range h.catalogs {
		for _, entry := range catalog.Entries {
			if !seen[entry.Hash] {
				seen[entry.Hash] = true
				entries = append(entries, entry)
			}
		}
	}

	return entries
}

// MarshalJSON implements json.Marshaler for debugging.
func (h *FullReplicaHandler) MarshalJSON() ([]byte, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return json.Marshal(map[string]interface{}{
		"mode":        h.Mode().String(),
		"sync_status": h.syncStatus,
		"peer_count":  len(h.catalogs),
	})
}
