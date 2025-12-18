package audit

import (
	"context"
	"sync"
	"time"
)

// RateLimiter provides rate limiting based on audit alerts.
type RateLimiter struct {
	config   RateLimitConfig
	limiters map[string]*limiterState
	mu       sync.RWMutex
}

// RateLimitConfig holds rate limiter configuration.
type RateLimitConfig struct {
	// Enabled controls whether rate limiting is active.
	Enabled bool `mapstructure:"enabled"`

	// DefaultLimit is the default rate limit per window.
	DefaultLimit int `mapstructure:"default_limit"`

	// DefaultWindow is the default time window.
	DefaultWindow time.Duration `mapstructure:"default_window"`

	// BlockDuration is how long to block after limit is reached.
	BlockDuration time.Duration `mapstructure:"block_duration"`

	// BypassRoles are roles that bypass rate limiting.
	BypassRoles []string `mapstructure:"bypass_roles"`

	// LimitsByAction defines per-action rate limits.
	LimitsByAction map[Action]int `mapstructure:"limits_by_action"`

	// LimitsByTable defines per-table rate limits.
	LimitsByTable map[string]int `mapstructure:"limits_by_table"`
}

// limiterState tracks rate limit state for a key.
type limiterState struct {
	key        string
	counts     []time.Time
	blockedAt  *time.Time
	blockUntil *time.Time
	limit      int
	window     time.Duration
	blocked    bool
}

// DefaultRateLimitConfig returns the default rate limit configuration.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Enabled:       true,
		DefaultLimit:  1000,
		DefaultWindow: 1 * time.Minute,
		BlockDuration: 5 * time.Minute,
		BypassRoles:   []string{"bibd_admin"},
		LimitsByAction: map[Action]int{
			ActionSelect: 500,
			ActionInsert: 200,
			ActionUpdate: 100,
			ActionDelete: 50,
			ActionDDL:    10,
		},
		LimitsByTable: map[string]int{
			"audit_log": 100,
		},
	}
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(cfg RateLimitConfig) *RateLimiter {
	return &RateLimiter{
		config:   cfg,
		limiters: make(map[string]*limiterState),
	}
}

// Check checks if an operation should be rate limited.
// Returns true if the operation is allowed, false if rate limited.
func (r *RateLimiter) Check(ctx context.Context, entry *Entry) (allowed bool, reason string) {
	if r == nil || !r.config.Enabled {
		return true, ""
	}

	// Check bypass roles
	for _, bypassRole := range r.config.BypassRoles {
		if entry.RoleUsed == bypassRole {
			return true, ""
		}
	}

	// Check various rate limit keys
	keys := r.generateKeys(entry)

	for _, key := range keys {
		allowed, reason := r.checkKey(key, entry)
		if !allowed {
			return false, reason
		}
	}

	return true, ""
}

// generateKeys generates rate limit keys for an entry.
func (r *RateLimiter) generateKeys(entry *Entry) []string {
	keys := make([]string, 0, 4)

	// Global key
	keys = append(keys, "global")

	// Actor key
	if entry.Actor != "" {
		keys = append(keys, "actor:"+entry.Actor)
	}

	// Action key
	keys = append(keys, "action:"+string(entry.Action))

	// Table key
	if entry.TableName != "" {
		keys = append(keys, "table:"+entry.TableName)
	}

	return keys
}

// checkKey checks a specific rate limit key.
func (r *RateLimiter) checkKey(key string, entry *Entry) (allowed bool, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	state, ok := r.limiters[key]
	if !ok {
		limit := r.determineLimit(key, entry)
		state = &limiterState{
			key:    key,
			counts: make([]time.Time, 0),
			limit:  limit,
			window: r.config.DefaultWindow,
		}
		r.limiters[key] = state
	}

	// Check if currently blocked
	if state.blocked && state.blockUntil != nil && now.Before(*state.blockUntil) {
		return false, "rate limit exceeded, blocked until " + state.blockUntil.Format(time.RFC3339)
	}

	// Unblock if block period has passed
	if state.blocked && state.blockUntil != nil && now.After(*state.blockUntil) {
		state.blocked = false
		state.blockUntil = nil
		state.blockedAt = nil
		state.counts = make([]time.Time, 0)
	}

	// Remove expired counts
	cutoff := now.Add(-state.window)
	validCounts := make([]time.Time, 0, len(state.counts)+1)
	for _, t := range state.counts {
		if t.After(cutoff) {
			validCounts = append(validCounts, t)
		}
	}
	state.counts = validCounts

	// Check limit
	if len(state.counts) >= state.limit {
		// Block the key
		state.blocked = true
		blockedAt := now
		blockUntil := now.Add(r.config.BlockDuration)
		state.blockedAt = &blockedAt
		state.blockUntil = &blockUntil
		return false, "rate limit exceeded for " + key
	}

	// Allow and count
	state.counts = append(state.counts, now)
	return true, ""
}

// determineLimit determines the appropriate limit for a key.
func (r *RateLimiter) determineLimit(key string, entry *Entry) int {
	// Check action-specific limits
	if limit, ok := r.config.LimitsByAction[entry.Action]; ok {
		return limit
	}

	// Check table-specific limits
	if limit, ok := r.config.LimitsByTable[entry.TableName]; ok {
		return limit
	}

	return r.config.DefaultLimit
}

// TriggerBlock manually blocks a key (e.g., from alert callback).
func (r *RateLimiter) TriggerBlock(key string, duration time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	blockUntil := now.Add(duration)

	state, ok := r.limiters[key]
	if !ok {
		state = &limiterState{
			key:    key,
			counts: make([]time.Time, 0),
			limit:  r.config.DefaultLimit,
			window: r.config.DefaultWindow,
		}
		r.limiters[key] = state
	}

	state.blocked = true
	state.blockedAt = &now
	state.blockUntil = &blockUntil
}

// IsBlocked checks if a key is currently blocked.
func (r *RateLimiter) IsBlocked(key string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state, ok := r.limiters[key]
	if !ok {
		return false
	}

	if !state.blocked {
		return false
	}

	if state.blockUntil != nil && time.Now().After(*state.blockUntil) {
		return false
	}

	return true
}

// Unblock manually unblocks a key.
func (r *RateLimiter) Unblock(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if state, ok := r.limiters[key]; ok {
		state.blocked = false
		state.blockedAt = nil
		state.blockUntil = nil
	}
}

// GetStats returns rate limiter statistics.
func (r *RateLimiter) GetStats() RateLimitStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := RateLimitStats{
		ActiveLimiters: len(r.limiters),
		BlockedKeys:    make([]string, 0),
	}

	now := time.Now()
	for key, state := range r.limiters {
		if state.blocked && state.blockUntil != nil && now.Before(*state.blockUntil) {
			stats.BlockedKeys = append(stats.BlockedKeys, key)
		}
	}

	return stats
}

// RateLimitStats contains rate limiter statistics.
type RateLimitStats struct {
	ActiveLimiters int      `json:"active_limiters"`
	BlockedKeys    []string `json:"blocked_keys"`
}

// Cleanup removes expired limiters.
func (r *RateLimiter) Cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for key, state := range r.limiters {
		// Remove if no counts in window and not blocked
		if len(state.counts) == 0 && !state.blocked {
			delete(r.limiters, key)
			continue
		}

		// Remove if block has expired and no recent counts
		if state.blocked && state.blockUntil != nil && now.After(*state.blockUntil) {
			cutoff := now.Add(-state.window)
			hasValidCounts := false
			for _, t := range state.counts {
				if t.After(cutoff) {
					hasValidCounts = true
					break
				}
			}
			if !hasValidCounts {
				delete(r.limiters, key)
			}
		}
	}
}
