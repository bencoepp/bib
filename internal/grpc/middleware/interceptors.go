// Package middleware provides gRPC interceptors for the bib daemon.
package middleware

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"bib/internal/domain"

	"github.com/google/uuid"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// Context keys for request metadata
type requestContextKey string

const (
	// RequestIDKey is the context key for request IDs.
	RequestIDKey requestContextKey = "request_id"

	// RequestIDHeader is the metadata key for request IDs.
	RequestIDHeader = "x-request-id"

	// StartTimeKey is the context key for request start time.
	StartTimeKey requestContextKey = "start_time"
)

// RequestIDFromContext extracts the request ID from context.
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

// WithRequestID adds a request ID to the context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, RequestIDKey, id)
}

// ============================================================================
// Request ID Interceptor
// ============================================================================

// RequestIDUnaryInterceptor adds or propagates request IDs for tracing.
func RequestIDUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		requestID := extractOrGenerateRequestID(ctx)
		ctx = WithRequestID(ctx, requestID)

		// Add request ID to outgoing headers
		_ = grpc.SetHeader(ctx, metadata.Pairs(RequestIDHeader, requestID))

		return handler(ctx, req)
	}
}

// RequestIDStreamInterceptor adds or propagates request IDs for streaming RPCs.
func RequestIDStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		requestID := extractOrGenerateRequestID(ss.Context())
		ctx := WithRequestID(ss.Context(), requestID)

		// Add request ID to outgoing headers
		_ = grpc.SetHeader(ctx, metadata.Pairs(RequestIDHeader, requestID))

		wrapped := &wrappedServerStream{ServerStream: ss, ctx: ctx}
		return handler(srv, wrapped)
	}
}

func extractOrGenerateRequestID(ctx context.Context) string {
	// Check if request ID is already in incoming metadata
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if ids := md.Get(RequestIDHeader); len(ids) > 0 && ids[0] != "" {
			return ids[0]
		}
	}
	// Generate new UUID
	return uuid.New().String()
}

// ============================================================================
// Logging Interceptor
// ============================================================================

// LoggingUnaryInterceptor logs all RPC calls with timing information.
func LoggingUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		ctx = context.WithValue(ctx, StartTimeKey, start)

		requestID := RequestIDFromContext(ctx)
		peerAddr := getPeerAddress(ctx)

		// Execute handler
		resp, err := handler(ctx, req)

		duration := time.Since(start)
		code := status.Code(err)

		// Log the request
		logRPCCall(requestID, info.FullMethod, peerAddr, code, duration, err)

		return resp, err
	}
}

// LoggingStreamInterceptor logs streaming RPC calls.
func LoggingStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		ctx := context.WithValue(ss.Context(), StartTimeKey, start)

		requestID := RequestIDFromContext(ctx)
		peerAddr := getPeerAddress(ctx)

		wrapped := &wrappedServerStream{ServerStream: ss, ctx: ctx}

		err := handler(srv, wrapped)

		duration := time.Since(start)
		code := status.Code(err)

		logRPCCall(requestID, info.FullMethod, peerAddr, code, duration, err)

		return err
	}
}

func getPeerAddress(ctx context.Context) string {
	if p, ok := peer.FromContext(ctx); ok {
		return p.Addr.String()
	}
	return "unknown"
}

func logRPCCall(requestID, method, peer string, code codes.Code, duration time.Duration, err error) {
	// TODO: Replace with proper structured logging via logger package
	if requestID == "" {
		requestID = "unknown"
	}
	prefix := requestID
	if len(requestID) > 8 {
		prefix = requestID[:8]
	}
	if err != nil {
		fmt.Printf("[gRPC] %s %s from %s - %s (%v) error: %v\n",
			prefix, method, peer, code, duration, err)
	} else {
		fmt.Printf("[gRPC] %s %s from %s - %s (%v)\n",
			prefix, method, peer, code, duration)
	}
}

// ============================================================================
// Recovery Interceptor
// ============================================================================

// RecoveryUnaryInterceptor catches panics and converts them to gRPC errors.
func RecoveryUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				requestID := RequestIDFromContext(ctx)
				stack := string(debug.Stack())

				prefix := requestID
				if len(requestID) > 8 {
					prefix = requestID[:8]
				}

				// Log the panic with full stack trace
				fmt.Printf("[gRPC] PANIC %s %s: %v\n%s\n",
					prefix, info.FullMethod, r, stack)

				// Return internal error (don't leak panic details to client)
				err = status.Errorf(codes.Internal, "internal server error (request_id: %s)", requestID)
			}
		}()

		return handler(ctx, req)
	}
}

// RecoveryStreamInterceptor catches panics in streaming RPCs.
func RecoveryStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				requestID := RequestIDFromContext(ss.Context())
				stack := string(debug.Stack())

				prefix := requestID
				if len(requestID) > 8 {
					prefix = requestID[:8]
				}

				fmt.Printf("[gRPC] PANIC %s %s: %v\n%s\n",
					prefix, info.FullMethod, r, stack)

				err = status.Errorf(codes.Internal, "internal server error (request_id: %s)", requestID)
			}
		}()

		return handler(srv, ss)
	}
}

// ============================================================================
// Rate Limiting Interceptor
// ============================================================================

// RateLimiter provides per-user rate limiting.
type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	rps      float64
	burst    int

	// Cleanup old limiters periodically
	lastCleanup time.Time
	cleanupAge  time.Duration
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(requestsPerSecond float64, burst int) *RateLimiter {
	return &RateLimiter{
		limiters:    make(map[string]*rate.Limiter),
		rps:         requestsPerSecond,
		burst:       burst,
		lastCleanup: time.Now(),
		cleanupAge:  10 * time.Minute,
	}
}

// Allow checks if the request should be allowed for the given key (user ID or IP).
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Periodic cleanup
	if time.Since(rl.lastCleanup) > rl.cleanupAge {
		rl.cleanup()
	}

	limiter, exists := rl.limiters[key]
	if !exists {
		limiter = rate.NewLimiter(rate.Limit(rl.rps), rl.burst)
		rl.limiters[key] = limiter
	}

	return limiter.Allow()
}

func (rl *RateLimiter) cleanup() {
	// Simple cleanup: just reset the map
	// A more sophisticated implementation would track last access times
	if len(rl.limiters) > 10000 {
		rl.limiters = make(map[string]*rate.Limiter)
	}
	rl.lastCleanup = time.Now()
}

// RateLimitUnaryInterceptor applies rate limiting to unary RPCs.
func RateLimitUnaryInterceptor(limiter *RateLimiter, getUserFromCtx func(ctx context.Context) (*domain.User, bool)) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		key := getRateLimitKey(ctx, getUserFromCtx)

		if !limiter.Allow(key) {
			return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded")
		}

		return handler(ctx, req)
	}
}

// RateLimitStreamInterceptor applies rate limiting to streaming RPCs.
func RateLimitStreamInterceptor(limiter *RateLimiter, getUserFromCtx func(ctx context.Context) (*domain.User, bool)) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		key := getRateLimitKey(ss.Context(), getUserFromCtx)

		if !limiter.Allow(key) {
			return status.Error(codes.ResourceExhausted, "rate limit exceeded")
		}

		return handler(srv, ss)
	}
}

func getRateLimitKey(ctx context.Context, getUserFromCtx func(ctx context.Context) (*domain.User, bool)) string {
	// First, try to get user ID from context (set by auth interceptor)
	if getUserFromCtx != nil {
		if user, ok := getUserFromCtx(ctx); ok && user != nil {
			return "user:" + user.ID.String()
		}
	}

	// Fall back to peer address
	if p, ok := peer.FromContext(ctx); ok {
		return "ip:" + p.Addr.String()
	}

	return "unknown"
}

// ============================================================================
// Wrapped Server Stream
// ============================================================================

// wrappedServerStream wraps a grpc.ServerStream to override the context.
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}
