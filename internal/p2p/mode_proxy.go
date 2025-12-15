package p2p

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"bib/internal/config"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

// cacheEntry represents a cached query result.
type cacheEntry struct {
	result    *QueryResult
	expiresAt time.Time
}

// ProxyHandler handles proxy mode operations.
// In this mode, the node forwards requests to peers and caches results.
type ProxyHandler struct {
	host      host.Host
	discovery *Discovery
	cfg       config.P2PConfig
	configDir string

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// mu protects the cache
	mu    sync.RWMutex
	cache map[string]*cacheEntry

	// favorites are preferred peers for forwarding
	favorites []peer.ID
}

// NewProxyHandler creates a new proxy handler.
func NewProxyHandler(h host.Host, discovery *Discovery, cfg config.P2PConfig, configDir string) (*ProxyHandler, error) {
	ph := &ProxyHandler{
		host:      h,
		discovery: discovery,
		cfg:       cfg,
		configDir: configDir,
		cache:     make(map[string]*cacheEntry),
	}

	// Parse favorite peers
	if err := ph.parseFavorites(cfg.Proxy.FavoritePeers); err != nil {
		// Log but don't fail
	}

	return ph, nil
}

// Mode returns the handler's mode.
func (h *ProxyHandler) Mode() NodeMode {
	return NodeModeProxy
}

// Start begins proxy mode operations.
func (h *ProxyHandler) Start(ctx context.Context) error {
	h.ctx, h.cancel = context.WithCancel(ctx)

	// Start cache cleanup goroutine
	h.wg.Add(1)
	go h.cleanupLoop()

	return nil
}

// Stop stops the handler.
func (h *ProxyHandler) Stop() error {
	if h.cancel != nil {
		h.cancel()
	}
	h.wg.Wait()
	return nil
}

// OnConfigUpdate handles configuration changes.
func (h *ProxyHandler) OnConfigUpdate(cfg config.P2PConfig) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.cfg = cfg

	// Update favorites
	if err := h.parseFavorites(cfg.Proxy.FavoritePeers); err != nil {
		// Log but don't fail
	}

	return nil
}

// parseFavorites parses favorite peer IDs from config.
func (h *ProxyHandler) parseFavorites(addrs []string) error {
	h.favorites = nil

	for _, addr := range addrs {
		peerID, err := peer.Decode(addr)
		if err != nil {
			// Try to extract from multiaddr
			// For now, skip invalid entries
			continue
		}
		h.favorites = append(h.favorites, peerID)
	}

	return nil
}

// Query forwards a query to peers and caches the result.
func (h *ProxyHandler) Query(ctx context.Context, req QueryRequest) (*QueryResult, error) {
	// Generate cache key
	cacheKey := h.cacheKey(req)

	// Check cache first
	if result := h.getFromCache(cacheKey); result != nil {
		return result, nil
	}

	// Forward to peers
	result, err := h.forwardQuery(ctx, req)
	if err != nil {
		return nil, err
	}

	// Cache the result
	h.putInCache(cacheKey, result)

	return result, nil
}

// cacheKey generates a cache key for a query request.
func (h *ProxyHandler) cacheKey(req QueryRequest) string {
	// Simple key based on query parameters
	data, _ := json.Marshal(req)
	return string(data)
}

// getFromCache retrieves a result from cache if not expired.
func (h *ProxyHandler) getFromCache(key string) *QueryResult {
	h.mu.RLock()
	defer h.mu.RUnlock()

	entry, ok := h.cache[key]
	if !ok {
		return nil
	}

	if time.Now().After(entry.expiresAt) {
		return nil
	}

	// Mark as from cache
	result := *entry.result
	result.FromCache = true
	return &result
}

// putInCache stores a result in cache.
func (h *ProxyHandler) putInCache(key string, result *QueryResult) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Check cache size limit
	maxSize := h.cfg.Proxy.MaxCacheSize
	if maxSize == 0 {
		maxSize = 1000
	}

	if len(h.cache) >= maxSize {
		// Evict oldest entry
		h.evictOldest()
	}

	ttl := h.cfg.Proxy.CacheTTL
	if ttl == 0 {
		ttl = 2 * time.Minute
	}

	h.cache[key] = &cacheEntry{
		result:    result,
		expiresAt: time.Now().Add(ttl),
	}
}

// evictOldest removes the oldest cache entry.
func (h *ProxyHandler) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range h.cache {
		if oldestKey == "" || entry.expiresAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.expiresAt
		}
	}

	if oldestKey != "" {
		delete(h.cache, oldestKey)
	}
}

// forwardQuery forwards a query to available peers.
func (h *ProxyHandler) forwardQuery(ctx context.Context, req QueryRequest) (*QueryResult, error) {
	// Get peers to query
	peers := h.getPeersForForwarding()

	if len(peers) == 0 {
		// No peers available, return empty result
		return &QueryResult{
			QueryID:    req.ID,
			Entries:    []CatalogEntry{},
			TotalCount: 0,
		}, nil
	}

	// Query each peer and aggregate results
	var allEntries []CatalogEntry
	seen := make(map[string]bool)
	var sourcePeer string

	for _, peerID := range peers {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		entries, err := h.queryPeer(ctx, peerID, req)
		if err != nil {
			continue
		}

		if sourcePeer == "" && len(entries) > 0 {
			sourcePeer = peerID.String()
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

	return &QueryResult{
		QueryID:    req.ID,
		Entries:    allEntries,
		TotalCount: total,
		SourcePeer: sourcePeer,
	}, nil
}

// getPeersForForwarding returns peers to forward requests to.
// Favorites are tried first, then discovered peers.
func (h *ProxyHandler) getPeersForForwarding() []peer.ID {
	h.mu.RLock()
	favorites := h.favorites
	h.mu.RUnlock()

	// Start with connected favorites
	var peers []peer.ID
	for _, fav := range favorites {
		if h.host.Network().Connectedness(fav) == 1 { // Connected
			peers = append(peers, fav)
		}
	}

	// Add other connected peers
	for _, p := range h.host.Network().Peers() {
		// Skip if already in favorites
		isFavorite := false
		for _, fav := range favorites {
			if p == fav {
				isFavorite = true
				break
			}
		}
		if !isFavorite {
			peers = append(peers, p)
		}
	}

	return peers
}

// queryPeer queries a specific peer.
// TODO: This will use the /bib/discovery/1.0.0 protocol in Phase 1.4.
func (h *ProxyHandler) queryPeer(ctx context.Context, peerID peer.ID, req QueryRequest) ([]CatalogEntry, error) {
	// Placeholder: return empty for now
	// This will be implemented with proper protocols in Phase 1.4
	return []CatalogEntry{}, nil
}

// cleanupLoop periodically cleans expired cache entries.
func (h *ProxyHandler) cleanupLoop() {
	defer h.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			h.cleanupExpired()
		}
	}
}

// cleanupExpired removes expired cache entries.
func (h *ProxyHandler) cleanupExpired() {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	for key, entry := range h.cache {
		if now.After(entry.expiresAt) {
			delete(h.cache, key)
		}
	}
}

// CacheStats returns cache statistics.
func (h *ProxyHandler) CacheStats() (size int, maxSize int) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	max := h.cfg.Proxy.MaxCacheSize
	if max == 0 {
		max = 1000
	}

	return len(h.cache), max
}

// ClearCache clears all cached entries.
func (h *ProxyHandler) ClearCache() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cache = make(map[string]*cacheEntry)
}

// AddFavorite adds a peer to the favorites list.
func (h *ProxyHandler) AddFavorite(peerID peer.ID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if already a favorite
	for _, fav := range h.favorites {
		if fav == peerID {
			return
		}
	}

	h.favorites = append(h.favorites, peerID)
}

// RemoveFavorite removes a peer from the favorites list.
func (h *ProxyHandler) RemoveFavorite(peerID peer.ID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for i, fav := range h.favorites {
		if fav == peerID {
			h.favorites = append(h.favorites[:i], h.favorites[i+1:]...)
			return
		}
	}
}

// Favorites returns the list of favorite peers.
func (h *ProxyHandler) Favorites() []peer.ID {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]peer.ID, len(h.favorites))
	copy(result, h.favorites)
	return result
}

// MarshalJSON implements json.Marshaler for debugging.
func (h *ProxyHandler) MarshalJSON() ([]byte, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	favStrings := make([]string, len(h.favorites))
	for i, fav := range h.favorites {
		favStrings[i] = fav.String()
	}

	return json.Marshal(map[string]interface{}{
		"mode":       h.Mode().String(),
		"cache_size": len(h.cache),
		"favorites":  favStrings,
	})
}
