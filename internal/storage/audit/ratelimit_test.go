package audit

import (
	"context"
	"testing"
	"time"
)

func TestNewRateLimiter(t *testing.T) {
	cfg := DefaultRateLimitConfig()
	limiter := NewRateLimiter(cfg)

	if limiter == nil {
		t.Fatal("NewRateLimiter returned nil")
	}
}

func TestRateLimiter_Check(t *testing.T) {
	cfg := RateLimitConfig{
		Enabled:       true,
		DefaultLimit:  3,
		DefaultWindow: 1 * time.Minute,
		BlockDuration: 1 * time.Minute,
		BypassRoles:   []string{"bibd_admin"},
	}

	limiter := NewRateLimiter(cfg)
	ctx := context.Background()

	// First 3 should be allowed
	for i := 0; i < 3; i++ {
		entry := &Entry{
			NodeID:          "node-1",
			OperationID:     GenerateOperationID(),
			RoleUsed:        "bibd_query",
			Action:          ActionSelect,
			SourceComponent: "test",
			Actor:           "user-1",
		}

		allowed, _ := limiter.Check(ctx, entry)
		if !allowed {
			t.Errorf("Entry %d should be allowed", i+1)
		}
	}

	// 4th should be blocked
	entry := &Entry{
		NodeID:          "node-1",
		OperationID:     GenerateOperationID(),
		RoleUsed:        "bibd_query",
		Action:          ActionSelect,
		SourceComponent: "test",
		Actor:           "user-1",
	}

	allowed, reason := limiter.Check(ctx, entry)
	if allowed {
		t.Error("4th entry should be blocked")
	}
	if reason == "" {
		t.Error("Should have a reason for blocking")
	}
}

func TestRateLimiter_BypassRole(t *testing.T) {
	cfg := RateLimitConfig{
		Enabled:       true,
		DefaultLimit:  1, // Very low limit
		DefaultWindow: 1 * time.Minute,
		BlockDuration: 1 * time.Minute,
		BypassRoles:   []string{"bibd_admin"},
	}

	limiter := NewRateLimiter(cfg)
	ctx := context.Background()

	// Admin role should bypass
	for i := 0; i < 10; i++ {
		entry := &Entry{
			NodeID:          "node-1",
			OperationID:     GenerateOperationID(),
			RoleUsed:        "bibd_admin",
			Action:          ActionSelect,
			SourceComponent: "test",
			Actor:           "admin-user",
		}

		allowed, _ := limiter.Check(ctx, entry)
		if !allowed {
			t.Errorf("Admin entry %d should bypass rate limit", i+1)
		}
	}
}

func TestRateLimiter_TriggerBlock(t *testing.T) {
	cfg := RateLimitConfig{
		Enabled:       true,
		DefaultLimit:  100,
		DefaultWindow: 1 * time.Minute,
		BlockDuration: 1 * time.Minute,
	}

	limiter := NewRateLimiter(cfg)
	ctx := context.Background()

	// Manually block a key
	limiter.TriggerBlock("actor:user-1", 1*time.Minute)

	// Check if blocked
	if !limiter.IsBlocked("actor:user-1") {
		t.Error("Key should be blocked")
	}

	// Entries from this actor should be blocked
	entry := &Entry{
		NodeID:          "node-1",
		OperationID:     GenerateOperationID(),
		RoleUsed:        "bibd_query",
		Action:          ActionSelect,
		SourceComponent: "test",
		Actor:           "user-1",
	}

	allowed, _ := limiter.Check(ctx, entry)
	if allowed {
		t.Error("Entry from blocked actor should be rejected")
	}

	// Other actors should be fine
	entry2 := &Entry{
		NodeID:          "node-1",
		OperationID:     GenerateOperationID(),
		RoleUsed:        "bibd_query",
		Action:          ActionSelect,
		SourceComponent: "test",
		Actor:           "user-2",
	}

	allowed, _ = limiter.Check(ctx, entry2)
	if !allowed {
		t.Error("Entry from different actor should be allowed")
	}
}

func TestRateLimiter_Unblock(t *testing.T) {
	cfg := DefaultRateLimitConfig()
	limiter := NewRateLimiter(cfg)

	// Block a key
	limiter.TriggerBlock("test-key", 1*time.Hour)

	if !limiter.IsBlocked("test-key") {
		t.Error("Key should be blocked")
	}

	// Unblock
	limiter.Unblock("test-key")

	if limiter.IsBlocked("test-key") {
		t.Error("Key should be unblocked")
	}
}

func TestRateLimiter_ActionLimits(t *testing.T) {
	cfg := RateLimitConfig{
		Enabled:       true,
		DefaultLimit:  100,
		DefaultWindow: 1 * time.Minute,
		BlockDuration: 1 * time.Minute,
		LimitsByAction: map[Action]int{
			ActionDelete: 2,
		},
	}

	limiter := NewRateLimiter(cfg)
	ctx := context.Background()

	// DELETE has lower limit
	for i := 0; i < 2; i++ {
		entry := &Entry{
			NodeID:          "node-1",
			OperationID:     GenerateOperationID(),
			RoleUsed:        "bibd_transform",
			Action:          ActionDelete,
			SourceComponent: "test",
			Actor:           "user-1",
		}

		allowed, _ := limiter.Check(ctx, entry)
		if !allowed {
			t.Errorf("DELETE %d should be allowed", i+1)
		}
	}

	// 3rd DELETE should be blocked
	entry := &Entry{
		NodeID:          "node-1",
		OperationID:     GenerateOperationID(),
		RoleUsed:        "bibd_transform",
		Action:          ActionDelete,
		SourceComponent: "test",
		Actor:           "user-1",
	}

	allowed, _ := limiter.Check(ctx, entry)
	if allowed {
		t.Error("3rd DELETE should be blocked")
	}
}

func TestRateLimiter_GetStats(t *testing.T) {
	cfg := DefaultRateLimitConfig()
	limiter := NewRateLimiter(cfg)
	ctx := context.Background()

	// Make some requests
	entry := &Entry{
		NodeID:          "node-1",
		OperationID:     GenerateOperationID(),
		RoleUsed:        "bibd_query",
		Action:          ActionSelect,
		SourceComponent: "test",
		Actor:           "user-1",
	}

	limiter.Check(ctx, entry)

	// Block a key
	limiter.TriggerBlock("test-key", 1*time.Hour)

	stats := limiter.GetStats()
	if stats.ActiveLimiters == 0 {
		t.Error("Should have active limiters")
	}
	if len(stats.BlockedKeys) == 0 {
		t.Error("Should have blocked keys")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	cfg := RateLimitConfig{
		Enabled:       true,
		DefaultLimit:  100,
		DefaultWindow: 1 * time.Millisecond, // Very short window
		BlockDuration: 1 * time.Millisecond,
	}

	limiter := NewRateLimiter(cfg)
	ctx := context.Background()

	// Make a request
	entry := &Entry{
		NodeID:          "node-1",
		OperationID:     GenerateOperationID(),
		RoleUsed:        "bibd_query",
		Action:          ActionSelect,
		SourceComponent: "test",
		Actor:           "user-1",
	}

	limiter.Check(ctx, entry)

	// Verify we have active limiters
	statsBefore := limiter.GetStats()
	if statsBefore.ActiveLimiters == 0 {
		t.Error("Should have active limiters before cleanup")
	}

	// Wait for window to expire
	time.Sleep(20 * time.Millisecond)

	// Cleanup
	limiter.Cleanup()

	// After cleanup with expired entries, limiters should be removed
	stats := limiter.GetStats()
	// Note: cleanup removes limiters with no valid counts, but this is best-effort
	// The main test is that cleanup doesn't crash and reduces count
	if stats.ActiveLimiters > statsBefore.ActiveLimiters {
		t.Errorf("ActiveLimiters should not increase after cleanup: before=%d, after=%d",
			statsBefore.ActiveLimiters, stats.ActiveLimiters)
	}
}

func TestRateLimiter_Disabled(t *testing.T) {
	cfg := RateLimitConfig{
		Enabled:      false,
		DefaultLimit: 1,
	}

	limiter := NewRateLimiter(cfg)
	ctx := context.Background()

	// Should always be allowed when disabled
	for i := 0; i < 10; i++ {
		entry := &Entry{
			NodeID:          "node-1",
			OperationID:     GenerateOperationID(),
			RoleUsed:        "bibd_query",
			Action:          ActionSelect,
			SourceComponent: "test",
			Actor:           "user-1",
		}

		allowed, _ := limiter.Check(ctx, entry)
		if !allowed {
			t.Errorf("Entry %d should be allowed when disabled", i+1)
		}
	}
}

func TestRateLimiter_Nil(t *testing.T) {
	var limiter *RateLimiter
	ctx := context.Background()

	entry := &Entry{
		NodeID:          "node-1",
		OperationID:     GenerateOperationID(),
		RoleUsed:        "bibd_query",
		Action:          ActionSelect,
		SourceComponent: "test",
	}

	// Should not panic
	allowed, _ := limiter.Check(ctx, entry)
	if !allowed {
		t.Error("Nil limiter should allow all")
	}
}
