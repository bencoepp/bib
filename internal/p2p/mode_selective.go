package p2p

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"bib/internal/config"
	"bib/internal/domain"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

// SelectiveHandler handles selective mode operations.
// In this mode, the node subscribes to specific topics on-demand.
type SelectiveHandler struct {
	host      host.Host
	discovery *Discovery
	cfg       config.P2PConfig
	configDir string
	client    *ProtocolClient

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// mu protects the fields below
	mu            sync.RWMutex
	subscriptions []domain.Subscription
	catalog       map[string][]domain.CatalogEntry // keyed by topic pattern
}

// NewSelectiveHandler creates a new selective handler.
func NewSelectiveHandler(h host.Host, discovery *Discovery, cfg config.P2PConfig, configDir string) (*SelectiveHandler, error) {
	sh := &SelectiveHandler{
		host:          h,
		discovery:     discovery,
		cfg:           cfg,
		configDir:     configDir,
		subscriptions: []domain.Subscription{},
		catalog:       make(map[string][]domain.CatalogEntry),
		client:        NewProtocolClient(h),
	}

	// Load persisted subscriptions
	if err := sh.loadSubscriptions(); err != nil {
		// Not fatal, just start with empty subscriptions
	}

	return sh, nil
}

// Mode returns the handler's mode.
func (h *SelectiveHandler) Mode() NodeMode {
	return NodeModeSelective
}

// Start begins selective mode operations.
func (h *SelectiveHandler) Start(ctx context.Context) error {
	h.ctx, h.cancel = context.WithCancel(ctx)
	return nil
}

// Stop stops the handler.
func (h *SelectiveHandler) Stop() error {
	if h.cancel != nil {
		h.cancel()
	}
	h.wg.Wait()

	// Persist subscriptions
	return h.saveSubscriptions()
}

// OnConfigUpdate handles configuration changes.
func (h *SelectiveHandler) OnConfigUpdate(cfg config.P2PConfig) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.cfg = cfg

	// Update subscriptions from config
	for _, pattern := range cfg.Selective.Subscriptions {
		h.addSubscriptionLocked(pattern)
	}

	return nil
}

// Subscribe adds a topic subscription.
func (h *SelectiveHandler) Subscribe(pattern string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.addSubscriptionLocked(pattern)

	// Persist immediately
	return h.saveSubscriptionsLocked()
}

// addSubscriptionLocked adds a subscription (caller must hold lock).
func (h *SelectiveHandler) addSubscriptionLocked(pattern string) {
	// Check if already subscribed
	for _, sub := range h.subscriptions {
		if sub.TopicPattern == pattern {
			return
		}
	}

	h.subscriptions = append(h.subscriptions, domain.Subscription{
		TopicPattern: pattern,
		CreatedAt:    time.Now(),
	})
}

// Unsubscribe removes a topic subscription.
func (h *SelectiveHandler) Unsubscribe(pattern string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	for i, sub := range h.subscriptions {
		if sub.TopicPattern == pattern {
			h.subscriptions = append(h.subscriptions[:i], h.subscriptions[i+1:]...)
			break
		}
	}

	// Remove cached entries for this pattern
	delete(h.catalog, pattern)

	return h.saveSubscriptionsLocked()
}

// Subscriptions returns all current subscriptions.
func (h *SelectiveHandler) Subscriptions() []domain.Subscription {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]domain.Subscription, len(h.subscriptions))
	copy(result, h.subscriptions)
	return result
}

// Query queries for data matching a request by asking all peers.
func (h *SelectiveHandler) Query(ctx context.Context, req domain.QueryRequest) (*domain.QueryResult, error) {
	h.mu.RLock()
	peers := h.host.Network().Peers()
	h.mu.RUnlock()

	var allEntries []domain.CatalogEntry
	seen := make(map[string]bool)

	for _, peerID := range peers {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		entries, err := h.queryPeer(ctx, peerID, req)
		if err != nil {
			// Continue with other peers
			continue
		}

		for _, entry := range entries {
			if !seen[entry.Hash] {
				seen[entry.Hash] = true
				allEntries = append(allEntries, entry)
			}
		}
	}

	// Apply limit/offset
	total := len(allEntries)
	if req.Offset > 0 && req.Offset < len(allEntries) {
		allEntries = allEntries[req.Offset:]
	}
	if req.Limit > 0 && req.Limit < len(allEntries) {
		allEntries = allEntries[:req.Limit]
	}

	return &domain.QueryResult{
		QueryID:    req.ID,
		Entries:    allEntries,
		TotalCount: total,
		FromCache:  false,
	}, nil
}

// queryPeer queries a specific peer for catalog entries using the discovery protocol.
func (h *SelectiveHandler) queryPeer(ctx context.Context, peerID peer.ID, req domain.QueryRequest) ([]domain.CatalogEntry, error) {
	result, err := h.client.QueryCatalog(ctx, peerID, &req)
	if err != nil {
		return nil, err
	}
	return result.Entries, nil
}

// SyncSubscription syncs data for a specific subscription pattern.
func (h *SelectiveHandler) SyncSubscription(ctx context.Context, pattern string) error {
	result, err := h.Query(ctx, domain.QueryRequest{
		ID: "sync-" + pattern,
	})
	if err != nil {
		return err
	}

	h.mu.Lock()
	h.catalog[pattern] = result.Entries

	// Update last sync time
	for i, sub := range h.subscriptions {
		if sub.TopicPattern == pattern {
			h.subscriptions[i].LastSync = time.Now()
			break
		}
	}
	h.mu.Unlock()

	return nil
}

// subscriptionStorePath returns the path to the subscription store file.
func (h *SelectiveHandler) subscriptionStorePath() string {
	if h.cfg.Selective.SubscriptionStorePath != "" {
		return h.cfg.Selective.SubscriptionStorePath
	}
	return filepath.Join(h.configDir, "subscriptions.json")
}

// loadSubscriptions loads subscriptions from disk.
func (h *SelectiveHandler) loadSubscriptions() error {
	path := h.subscriptionStorePath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	return json.Unmarshal(data, &h.subscriptions)
}

// saveSubscriptions saves subscriptions to disk.
func (h *SelectiveHandler) saveSubscriptions() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.saveSubscriptionsLocked()
}

// saveSubscriptionsLocked saves subscriptions (caller must hold lock).
func (h *SelectiveHandler) saveSubscriptionsLocked() error {
	path := h.subscriptionStorePath()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(h.subscriptions, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// GetCachedEntries returns cached entries for a subscription pattern.
func (h *SelectiveHandler) GetCachedEntries(pattern string) []domain.CatalogEntry {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if entries, ok := h.catalog[pattern]; ok {
		result := make([]domain.CatalogEntry, len(entries))
		copy(result, entries)
		return result
	}
	return nil
}

// MarshalJSON implements json.Marshaler for debugging.
func (h *SelectiveHandler) MarshalJSON() ([]byte, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return json.Marshal(map[string]interface{}{
		"mode":          h.Mode().String(),
		"subscriptions": h.subscriptions,
		"cached_topics": len(h.catalog),
	})
}
