// Package p2p provides peer-to-peer networking functionality for bib.
package p2p

import (
	"context"
	"sync"
	"time"

	"bib/internal/logger"
	"bib/internal/storage"

	"github.com/libp2p/go-libp2p/core/peer"
)

// PeerPermission represents the permission level for a peer.
type PeerPermission string

const (
	// PeerPermissionNone indicates the peer has no permissions (blocked).
	PeerPermissionNone PeerPermission = "none"

	// PeerPermissionAllowed indicates the peer is allowed to make gRPC calls.
	PeerPermissionAllowed PeerPermission = "allowed"
)

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	// Enabled controls whether rate limiting is active.
	Enabled bool

	// RequestsPerSecond is the maximum requests per second per peer.
	RequestsPerSecond float64

	// BurstSize is the maximum burst size.
	BurstSize int

	// CleanupInterval is how often to clean up stale rate limit entries.
	CleanupInterval time.Duration
}

// DefaultRateLimitConfig returns the default rate limit configuration.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 100,
		BurstSize:         200,
		CleanupInterval:   5 * time.Minute,
	}
}

// rateLimitEntry tracks rate limit state for a peer.
type rateLimitEntry struct {
	tokens     float64
	lastUpdate time.Time
	mu         sync.Mutex
}

// RateLimiter provides per-peer rate limiting.
type RateLimiter struct {
	cfg     RateLimitConfig
	entries sync.Map // peer.ID -> *rateLimitEntry
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(cfg RateLimitConfig) *RateLimiter {
	ctx, cancel := context.WithCancel(context.Background())
	rl := &RateLimiter{
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
	}

	if cfg.Enabled && cfg.CleanupInterval > 0 {
		go rl.cleanupLoop()
	}

	return rl
}

// Allow checks if a request from the peer should be allowed.
// Returns true if the request is allowed, false if rate limited.
func (rl *RateLimiter) Allow(peerID peer.ID) bool {
	if !rl.cfg.Enabled {
		return true
	}

	entry := rl.getOrCreateEntry(peerID)
	entry.mu.Lock()
	defer entry.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(entry.lastUpdate).Seconds()
	entry.lastUpdate = now

	// Refill tokens based on elapsed time
	entry.tokens += elapsed * rl.cfg.RequestsPerSecond
	if entry.tokens > float64(rl.cfg.BurstSize) {
		entry.tokens = float64(rl.cfg.BurstSize)
	}

	// Check if we have tokens available
	if entry.tokens >= 1 {
		entry.tokens--
		return true
	}

	return false
}

// getOrCreateEntry gets or creates a rate limit entry for a peer.
func (rl *RateLimiter) getOrCreateEntry(peerID peer.ID) *rateLimitEntry {
	if v, ok := rl.entries.Load(peerID); ok {
		return v.(*rateLimitEntry)
	}

	entry := &rateLimitEntry{
		tokens:     float64(rl.cfg.BurstSize),
		lastUpdate: time.Now(),
	}

	actual, _ := rl.entries.LoadOrStore(peerID, entry)
	return actual.(*rateLimitEntry)
}

// cleanupLoop periodically removes stale entries.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cfg.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.ctx.Done():
			return
		}
	}
}

// cleanup removes entries that haven't been used recently.
func (rl *RateLimiter) cleanup() {
	threshold := time.Now().Add(-rl.cfg.CleanupInterval * 2)

	rl.entries.Range(func(key, value any) bool {
		entry := value.(*rateLimitEntry)
		entry.mu.Lock()
		if entry.lastUpdate.Before(threshold) {
			rl.entries.Delete(key)
		}
		entry.mu.Unlock()
		return true
	})
}

// Close stops the rate limiter.
func (rl *RateLimiter) Close() {
	rl.cancel()
}

// PeerAuthorizer handles peer authorization for gRPC-over-P2P.
type PeerAuthorizer struct {
	allowedRepo storage.AllowedPeerRepository
	rateLimiter *RateLimiter
	log         *logger.Logger

	// bootstrapPeers are peers from config that are always allowed.
	bootstrapPeers map[string]struct{}
	mu             sync.RWMutex
}

// PeerAuthorizerConfig holds configuration for the peer authorizer.
type PeerAuthorizerConfig struct {
	// AllowedPeerRepo is the repository for allowed peers.
	AllowedPeerRepo storage.AllowedPeerRepository

	// RateLimitConfig is the rate limiting configuration.
	RateLimitConfig RateLimitConfig

	// BootstrapPeers are peer IDs that are always allowed (from config).
	BootstrapPeers []string
}

// NewPeerAuthorizer creates a new peer authorizer.
func NewPeerAuthorizer(cfg PeerAuthorizerConfig) *PeerAuthorizer {
	bootstrapPeers := make(map[string]struct{})
	for _, p := range cfg.BootstrapPeers {
		bootstrapPeers[p] = struct{}{}
	}

	return &PeerAuthorizer{
		allowedRepo:    cfg.AllowedPeerRepo,
		rateLimiter:    NewRateLimiter(cfg.RateLimitConfig),
		log:            getLogger("grpc_auth"),
		bootstrapPeers: bootstrapPeers,
	}
}

// Authorize checks if a peer is authorized to make a gRPC call.
// Returns nil if authorized, otherwise silently drops the connection.
// This method does not return detailed errors to avoid information leakage.
func (pa *PeerAuthorizer) Authorize(ctx context.Context, peerID peer.ID) error {
	peerIDStr := peerID.String()

	// Check bootstrap peers first (always allowed)
	pa.mu.RLock()
	_, isBootstrap := pa.bootstrapPeers[peerIDStr]
	pa.mu.RUnlock()

	if !isBootstrap {
		// Check allowed list in database
		allowed, err := pa.allowedRepo.IsAllowed(ctx, peerIDStr)
		if err != nil {
			pa.log.Debug("error checking peer authorization", "peer_id", peerIDStr, "error", err)
			return ErrUnauthorizedPeer
		}
		if !allowed {
			pa.log.Debug("peer not in allowed list", "peer_id", peerIDStr)
			return ErrUnauthorizedPeer
		}
	}

	// Check rate limit
	if !pa.rateLimiter.Allow(peerID) {
		pa.log.Debug("peer rate limited", "peer_id", peerIDStr)
		return ErrRateLimited
	}

	return nil
}

// AddAllowedPeer adds a peer to the allowed list.
func (pa *PeerAuthorizer) AddAllowedPeer(ctx context.Context, peer *storage.AllowedPeer) error {
	return pa.allowedRepo.Add(ctx, peer)
}

// RemoveAllowedPeer removes a peer from the allowed list.
func (pa *PeerAuthorizer) RemoveAllowedPeer(ctx context.Context, peerID string) error {
	return pa.allowedRepo.Remove(ctx, peerID)
}

// ListAllowedPeers lists all allowed peers.
func (pa *PeerAuthorizer) ListAllowedPeers(ctx context.Context) ([]*storage.AllowedPeer, error) {
	return pa.allowedRepo.List(ctx)
}

// IsAllowed checks if a peer is allowed (including bootstrap peers).
func (pa *PeerAuthorizer) IsAllowed(ctx context.Context, peerID string) (bool, error) {
	// Check bootstrap peers
	pa.mu.RLock()
	_, isBootstrap := pa.bootstrapPeers[peerID]
	pa.mu.RUnlock()

	if isBootstrap {
		return true, nil
	}

	return pa.allowedRepo.IsAllowed(ctx, peerID)
}

// UpdateBootstrapPeers updates the list of bootstrap peers.
func (pa *PeerAuthorizer) UpdateBootstrapPeers(peers []string) {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	pa.bootstrapPeers = make(map[string]struct{})
	for _, p := range peers {
		pa.bootstrapPeers[p] = struct{}{}
	}
}

// Close stops the authorizer.
func (pa *PeerAuthorizer) Close() {
	if pa.rateLimiter != nil {
		pa.rateLimiter.Close()
	}
}

// Authorization errors - these are intentionally vague for security.
var (
	ErrUnauthorizedPeer = &authError{msg: "unauthorized"}
	ErrRateLimited      = &authError{msg: "rate limited"}
)

type authError struct {
	msg string
}

func (e *authError) Error() string {
	return e.msg
}

// IsAuthError returns true if the error is an authorization error.
func IsAuthError(err error) bool {
	_, ok := err.(*authError)
	return ok
}
