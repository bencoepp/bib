package p2p

import (
	"context"
	"os"
	"testing"
	"time"

	"bib/internal/config"

	"github.com/libp2p/go-libp2p/core/peer"
)

func TestProxyHandler_Cache(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-proxy-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := config.P2PConfig{
		Proxy: config.ProxyConfig{
			CacheTTL:     100 * time.Millisecond,
			MaxCacheSize: 10,
		},
	}

	handler, err := NewProxyHandler(nil, nil, cfg, tmpDir)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	// Test cache put and get
	req := QueryRequest{ID: "test-1", TopicID: "topic-1"}
	result := &QueryResult{
		QueryID: "test-1",
		Entries: []CatalogEntry{{TopicID: "topic-1", DatasetID: "ds-1"}},
	}

	key := handler.cacheKey(req)
	handler.putInCache(key, result)

	// Should get from cache
	cached := handler.getFromCache(key)
	if cached == nil {
		t.Fatal("expected result from cache")
	}
	if !cached.FromCache {
		t.Error("expected FromCache to be true")
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Should not get from cache after expiry
	expired := handler.getFromCache(key)
	if expired != nil {
		t.Error("expected nil after cache expiry")
	}
}

func TestProxyHandler_CacheEviction(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-proxy-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := config.P2PConfig{
		Proxy: config.ProxyConfig{
			CacheTTL:     1 * time.Hour,
			MaxCacheSize: 3,
		},
	}

	handler, err := NewProxyHandler(nil, nil, cfg, tmpDir)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	// Fill cache
	for i := 0; i < 5; i++ {
		req := QueryRequest{ID: string(rune('a' + i))}
		result := &QueryResult{QueryID: req.ID}
		handler.putInCache(handler.cacheKey(req), result)
	}

	// Check cache size is limited
	size, max := handler.CacheStats()
	if size > max {
		t.Errorf("cache size %d exceeds max %d", size, max)
	}
}

func TestProxyHandler_Favorites(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-proxy-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := config.P2PConfig{}
	handler, err := NewProxyHandler(nil, nil, cfg, tmpDir)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	// Parse a peer ID
	peerIDStr := "12D3KooWDXHortdoEeRiLx7FAtQ2ebEvS25R4ZKmVzpiz2sa2JWG"
	peerID, err := peer.Decode(peerIDStr)
	if err != nil {
		t.Fatalf("failed to parse peer ID: %v", err)
	}

	// Add favorite
	handler.AddFavorite(peerID)
	favorites := handler.Favorites()
	if len(favorites) != 1 {
		t.Fatalf("expected 1 favorite, got %d", len(favorites))
	}

	// Add same favorite again (should not duplicate)
	handler.AddFavorite(peerID)
	favorites = handler.Favorites()
	if len(favorites) != 1 {
		t.Fatalf("expected 1 favorite after duplicate add, got %d", len(favorites))
	}

	// Remove favorite
	handler.RemoveFavorite(peerID)
	favorites = handler.Favorites()
	if len(favorites) != 0 {
		t.Fatalf("expected 0 favorites after removal, got %d", len(favorites))
	}
}

func TestProxyHandler_ClearCache(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-proxy-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := config.P2PConfig{
		Proxy: config.ProxyConfig{
			CacheTTL:     1 * time.Hour,
			MaxCacheSize: 100,
		},
	}

	handler, err := NewProxyHandler(nil, nil, cfg, tmpDir)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	// Add some entries
	for i := 0; i < 5; i++ {
		req := QueryRequest{ID: string(rune('a' + i))}
		result := &QueryResult{QueryID: req.ID}
		handler.putInCache(handler.cacheKey(req), result)
	}

	size, _ := handler.CacheStats()
	if size != 5 {
		t.Fatalf("expected 5 entries, got %d", size)
	}

	// Clear cache
	handler.ClearCache()

	size, _ = handler.CacheStats()
	if size != 0 {
		t.Fatalf("expected 0 entries after clear, got %d", size)
	}
}

func TestProxyHandler_Mode(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-proxy-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	handler, _ := NewProxyHandler(nil, nil, config.P2PConfig{}, tmpDir)
	if handler.Mode() != NodeModeProxy {
		t.Errorf("expected mode %s, got %s", NodeModeProxy, handler.Mode())
	}
}

func TestProxyHandler_StartStop(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bib-proxy-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	handler, _ := NewProxyHandler(nil, nil, config.P2PConfig{}, tmpDir)

	ctx := context.Background()
	if err := handler.Start(ctx); err != nil {
		t.Fatalf("failed to start handler: %v", err)
	}

	if err := handler.Stop(); err != nil {
		t.Fatalf("failed to stop handler: %v", err)
	}
}
