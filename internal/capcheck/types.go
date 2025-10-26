package capcheck

import (
	"context"
	"time"
)

type CapabilityID string

type CheckResult struct {
	ID        CapabilityID   `json:"id"`
	Name      string         `json:"name"`
	Supported bool           `json:"supported"`
	Error     string         `json:"error,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
	CheckedAt time.Time      `json:"checked_at"`
	Duration  time.Duration  `json:"duration"`
}

type Checker interface {
	ID() CapabilityID
	Description() string
	// Check should respect context cancellation/deadlines and return quickly.
	Check(ctx context.Context) CheckResult
}

type Option func(*Runner)

type Runner struct {
	checkers        []Checker
	globalTimeout   time.Duration
	perCheckTimeout time.Duration
}

func WithGlobalTimeout(d time.Duration) Option {
	return func(r *Runner) { r.globalTimeout = d }
}

func WithPerCheckTimeout(d time.Duration) Option {
	return func(r *Runner) { r.perCheckTimeout = d }
}

func NewRunner(checkers []Checker, opts ...Option) *Runner {
	r := &Runner{
		checkers:        checkers,
		globalTimeout:   5 * time.Second,
		perCheckTimeout: 2 * time.Second,
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

func (r *Runner) Run(ctx context.Context) []CheckResult {
	if r.globalTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.globalTimeout)
		defer cancel()
	}

	results := make([]CheckResult, len(r.checkers))
	done := make(chan struct{})
	for i, c := range r.checkers {
		i, c := i, c
		go func() {
			checkCtx := ctx
			var cancel context.CancelFunc
			if r.perCheckTimeout > 0 {
				checkCtx, cancel = context.WithTimeout(ctx, r.perCheckTimeout)
				defer cancel()
			}
			start := time.Now()
			res := c.Check(checkCtx)
			if res.CheckedAt.IsZero() {
				res.CheckedAt = time.Now()
			}
			if res.Duration == 0 {
				res.Duration = time.Since(start)
			}
			results[i] = res
			done <- struct{}{}
		}()
	}
	for range r.checkers {
		select {
		case <-ctx.Done():
			// Context cancelled or timed out; remaining results might be zero values.
			return results
		case <-done:
		}
	}
	return results
}
