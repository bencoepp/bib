// Package p2p provides peer-to-peer networking functionality for bib.
package p2p

import (
	"context"
	"testing"
	"time"

	"bib/internal/storage"

	"github.com/libp2p/go-libp2p/core/peer"
)

// mockAllowedPeerRepo is a mock implementation of storage.AllowedPeerRepository for testing.
type mockAllowedPeerRepo struct {
	peers map[string]*storage.AllowedPeer
}

func newMockAllowedPeerRepo() *mockAllowedPeerRepo {
	return &mockAllowedPeerRepo{
		peers: make(map[string]*storage.AllowedPeer),
	}
}

func (m *mockAllowedPeerRepo) Add(_ context.Context, peer *storage.AllowedPeer) error {
	m.peers[peer.PeerID] = peer
	return nil
}

func (m *mockAllowedPeerRepo) Remove(_ context.Context, peerID string) error {
	delete(m.peers, peerID)
	return nil
}

func (m *mockAllowedPeerRepo) Get(_ context.Context, peerID string) (*storage.AllowedPeer, error) {
	if peer, ok := m.peers[peerID]; ok {
		return peer, nil
	}
	return nil, storage.ErrNotFound
}

func (m *mockAllowedPeerRepo) List(_ context.Context) ([]*storage.AllowedPeer, error) {
	var result []*storage.AllowedPeer
	for _, peer := range m.peers {
		result = append(result, peer)
	}
	return result, nil
}

func (m *mockAllowedPeerRepo) IsAllowed(_ context.Context, peerID string) (bool, error) {
	peer, ok := m.peers[peerID]
	if !ok {
		return false, nil
	}
	// Check expiration
	if peer.ExpiresAt != nil && peer.ExpiresAt.Before(time.Now()) {
		return false, nil
	}
	return true, nil
}

func (m *mockAllowedPeerRepo) Cleanup(_ context.Context) error {
	now := time.Now()
	for id, peer := range m.peers {
		if peer.ExpiresAt != nil && peer.ExpiresAt.Before(now) {
			delete(m.peers, id)
		}
	}
	return nil
}

func (m *mockAllowedPeerRepo) Count(_ context.Context) (int64, error) {
	return int64(len(m.peers)), nil
}

func TestRateLimiter_Allow(t *testing.T) {
	cfg := RateLimitConfig{
		Enabled:           true,
		RequestsPerSecond: 10,
		BurstSize:         5,
		CleanupInterval:   time.Minute,
	}

	rl := NewRateLimiter(cfg)
	defer rl.Close()

	// Generate a fake peer ID
	peerID := peer.ID("test-peer-id")

	// Should allow burst requests
	for i := 0; i < 5; i++ {
		if !rl.Allow(peerID) {
			t.Errorf("Request %d should be allowed (within burst)", i)
		}
	}

	// Next request should be rate limited (burst exhausted)
	if rl.Allow(peerID) {
		t.Error("Request should be rate limited after burst exhausted")
	}
}

func TestRateLimiter_Disabled(t *testing.T) {
	cfg := RateLimitConfig{
		Enabled: false,
	}

	rl := NewRateLimiter(cfg)
	defer rl.Close()

	peerID := peer.ID("test-peer-id")

	// Should always allow when disabled
	for i := 0; i < 100; i++ {
		if !rl.Allow(peerID) {
			t.Error("Request should always be allowed when rate limiting is disabled")
		}
	}
}

func TestPeerAuthorizer_BootstrapPeers(t *testing.T) {
	repo := newMockAllowedPeerRepo()

	// Use actual peer.ID strings for bootstrap peers
	peerID1 := peer.ID("bootstrap-peer-1")
	peerID2 := peer.ID("bootstrap-peer-2")

	cfg := PeerAuthorizerConfig{
		AllowedPeerRepo: repo,
		RateLimitConfig: RateLimitConfig{Enabled: false},
		BootstrapPeers:  []string{peerID1.String(), peerID2.String()},
	}

	auth := NewPeerAuthorizer(cfg)
	defer auth.Close()

	ctx := context.Background()

	// Bootstrap peers should be allowed without being in the repo
	if err := auth.Authorize(ctx, peerID1); err != nil {
		t.Errorf("Bootstrap peer should be authorized: %v", err)
	}

	// Non-bootstrap peer not in repo should be denied
	unknownPeer := peer.ID("unknown-peer")
	if err := auth.Authorize(ctx, unknownPeer); err == nil {
		t.Error("Unknown peer should not be authorized")
	}
}

func TestPeerAuthorizer_AllowedList(t *testing.T) {
	repo := newMockAllowedPeerRepo()

	cfg := PeerAuthorizerConfig{
		AllowedPeerRepo: repo,
		RateLimitConfig: RateLimitConfig{Enabled: false},
		BootstrapPeers:  []string{},
	}

	auth := NewPeerAuthorizer(cfg)
	defer auth.Close()

	ctx := context.Background()

	// Add a peer to the allowed list using the peer.ID.String() representation
	allowedPeer := peer.ID("allowed-peer-1")
	allowedPeerIDStr := allowedPeer.String()
	err := repo.Add(ctx, &storage.AllowedPeer{
		PeerID:  allowedPeerIDStr,
		AddedAt: time.Now(),
		AddedBy: "test",
	})
	if err != nil {
		t.Fatalf("Failed to add peer: %v", err)
	}

	// Allowed peer should be authorized
	if err := auth.Authorize(ctx, allowedPeer); err != nil {
		t.Errorf("Allowed peer should be authorized: %v", err)
	}

	// Remove the peer
	err = repo.Remove(ctx, allowedPeerIDStr)
	if err != nil {
		t.Fatalf("Failed to remove peer: %v", err)
	}

	// Removed peer should no longer be authorized
	if err := auth.Authorize(ctx, allowedPeer); err == nil {
		t.Error("Removed peer should not be authorized")
	}
}

func TestPeerAuthorizer_ExpiredPeer(t *testing.T) {
	repo := newMockAllowedPeerRepo()

	cfg := PeerAuthorizerConfig{
		AllowedPeerRepo: repo,
		RateLimitConfig: RateLimitConfig{Enabled: false},
		BootstrapPeers:  []string{},
	}

	auth := NewPeerAuthorizer(cfg)
	defer auth.Close()

	ctx := context.Background()

	// Add a peer with past expiration using peer.ID.String() representation
	expiredPeer := peer.ID("expired-peer")
	expiredPeerIDStr := expiredPeer.String()
	pastTime := time.Now().Add(-time.Hour)
	err := repo.Add(ctx, &storage.AllowedPeer{
		PeerID:    expiredPeerIDStr,
		AddedAt:   time.Now().Add(-2 * time.Hour),
		AddedBy:   "test",
		ExpiresAt: &pastTime,
	})
	if err != nil {
		t.Fatalf("Failed to add peer: %v", err)
	}

	// Expired peer should not be authorized
	if err := auth.Authorize(ctx, expiredPeer); err == nil {
		t.Error("Expired peer should not be authorized")
	}
}

func TestIsRestrictedService(t *testing.T) {
	tests := []struct {
		method     string
		restricted bool
	}{
		{"/bib.v1.services.AdminService/GetConfig", true},
		{"/bib.v1.services.AdminService/UpdateConfig", true},
		{"/bib.v1.services.BreakGlassService/RequestAccess", true},
		{"/bib.v1.services.HealthService/Check", false},
		{"/bib.v1.services.DatasetService/GetDataset", false},
		{"/bib.v1.services.TopicService/ListTopics", false},
		{"/bib.v1.services.UserService/GetUser", false},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			got := isRestrictedService(tt.method)
			if got != tt.restricted {
				t.Errorf("isRestrictedService(%q) = %v, want %v", tt.method, got, tt.restricted)
			}
		})
	}
}
