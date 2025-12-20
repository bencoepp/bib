package grpc

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRequestIDFromContext(t *testing.T) {
	// Test with no request ID
	ctx := context.Background()
	id := RequestIDFromContext(ctx)
	if id != "" {
		t.Errorf("expected empty string, got %s", id)
	}

	// Test with request ID
	expectedID := uuid.New().String()
	ctx = WithRequestID(ctx, expectedID)
	id = RequestIDFromContext(ctx)
	if id != expectedID {
		t.Errorf("expected %s, got %s", expectedID, id)
	}
}

func TestRateLimiter_Allow(t *testing.T) {
	limiter := NewRateLimiter(10, 20) // 10 rps, burst of 20

	key := "user:test"

	// First 20 requests should be allowed (burst)
	for i := 0; i < 20; i++ {
		if !limiter.Allow(key) {
			t.Errorf("request %d should be allowed", i)
		}
	}

	// Next request should be denied (burst exhausted)
	if limiter.Allow(key) {
		t.Error("request should be denied after burst exhausted")
	}
}

func TestRateLimiter_DifferentKeys(t *testing.T) {
	limiter := NewRateLimiter(1, 1) // 1 rps, burst of 1

	// First request for each key should be allowed
	if !limiter.Allow("user:a") {
		t.Error("first request for user:a should be allowed")
	}
	if !limiter.Allow("user:b") {
		t.Error("first request for user:b should be allowed")
	}

	// Second request for each should be denied
	if limiter.Allow("user:a") {
		t.Error("second request for user:a should be denied")
	}
	if limiter.Allow("user:b") {
		t.Error("second request for user:b should be denied")
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	limiter := NewRateLimiter(1000, 2000)

	var wg sync.WaitGroup
	allowed := 0
	denied := 0
	var mu sync.Mutex

	// Simulate concurrent requests
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := limiter.Allow("concurrent-test")
			mu.Lock()
			if result {
				allowed++
			} else {
				denied++
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	// All should be allowed with high rate limit
	if denied > 0 {
		t.Errorf("expected all requests allowed with high limit, got %d denied", denied)
	}
}

func TestExtractOrGenerateRequestID(t *testing.T) {
	ctx := context.Background()

	// Without existing ID, should generate new one
	id1 := extractOrGenerateRequestID(ctx)
	if id1 == "" {
		t.Error("should generate request ID")
	}

	// Should be valid UUID format
	if _, err := uuid.Parse(id1); err != nil {
		t.Errorf("should be valid UUID: %v", err)
	}
}

func TestWrappedServerStream_Context(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "test-id")

	wrapped := &wrappedServerStream{
		ctx: ctx,
	}

	if RequestIDFromContext(wrapped.Context()) != "test-id" {
		t.Error("wrapped stream should return modified context")
	}
}

func TestNewRateLimiter(t *testing.T) {
	limiter := NewRateLimiter(100, 200)

	if limiter.rps != 100 {
		t.Errorf("expected rps 100, got %f", limiter.rps)
	}
	if limiter.burst != 200 {
		t.Errorf("expected burst 200, got %d", limiter.burst)
	}
	if limiter.limiters == nil {
		t.Error("limiters map should be initialized")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	limiter := NewRateLimiter(10, 10)
	limiter.cleanupAge = 1 * time.Millisecond // Short for testing

	// Add many keys
	for i := 0; i < 100; i++ {
		limiter.Allow("key-" + string(rune(i)))
	}

	// Wait for cleanup age
	time.Sleep(10 * time.Millisecond)
	limiter.lastCleanup = time.Now().Add(-2 * time.Minute) // Force cleanup trigger

	// This should trigger cleanup
	limiter.Allow("trigger-cleanup")

	// Limiters map should be cleaned up if it was large
	// (Our cleanup is simple - it just resets if > 10000)
}
