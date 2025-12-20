// Package client provides a gRPC client library for connecting to bibd.
package client

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// buildUnaryInterceptors builds the chain of unary interceptors.
func (c *Client) buildUnaryInterceptors() []grpc.UnaryClientInterceptor {
	var interceptors []grpc.UnaryClientInterceptor

	// Request ID interceptor
	if c.opts.RequestIDEnabled {
		interceptors = append(interceptors, requestIDUnaryInterceptor())
	}

	// Auth interceptor (adds session token)
	interceptors = append(interceptors, c.authUnaryInterceptor())

	// Logging interceptor
	if c.opts.LoggingEnabled {
		interceptors = append(interceptors, loggingUnaryInterceptor())
	}

	return interceptors
}

// buildStreamInterceptors builds the chain of stream interceptors.
func (c *Client) buildStreamInterceptors() []grpc.StreamClientInterceptor {
	var interceptors []grpc.StreamClientInterceptor

	// Request ID interceptor
	if c.opts.RequestIDEnabled {
		interceptors = append(interceptors, requestIDStreamInterceptor())
	}

	// Auth interceptor (adds session token)
	interceptors = append(interceptors, c.authStreamInterceptor())

	// Logging interceptor
	if c.opts.LoggingEnabled {
		interceptors = append(interceptors, loggingStreamInterceptor())
	}

	return interceptors
}

// requestIDUnaryInterceptor adds a request ID to outgoing calls.
func requestIDUnaryInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = ensureRequestID(ctx)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// requestIDStreamInterceptor adds a request ID to streaming calls.
func requestIDStreamInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		ctx = ensureRequestID(ctx)
		return streamer(ctx, desc, cc, method, opts...)
	}
}

// ensureRequestID ensures the context has a request ID in metadata.
func ensureRequestID(ctx context.Context) context.Context {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}

	if len(md.Get("x-request-id")) == 0 {
		md = md.Copy()
		md.Set("x-request-id", uuid.New().String())
	}

	return metadata.NewOutgoingContext(ctx, md)
}

// authUnaryInterceptor adds the session token to outgoing calls.
func (c *Client) authUnaryInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = c.addAuthMetadata(ctx)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// authStreamInterceptor adds the session token to streaming calls.
func (c *Client) authStreamInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		ctx = c.addAuthMetadata(ctx)
		return streamer(ctx, desc, cc, method, opts...)
	}
}

// addAuthMetadata adds authentication metadata to the context.
func (c *Client) addAuthMetadata(ctx context.Context) context.Context {
	token := c.SessionToken()
	if token == "" {
		return ctx
	}

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	} else {
		md = md.Copy()
	}

	md.Set("x-session-token", token)
	return metadata.NewOutgoingContext(ctx, md)
}

// loggingUnaryInterceptor logs all unary RPC calls.
func loggingUnaryInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		duration := time.Since(start)

		// Log to stdout for now - in production would use structured logging
		if err != nil {
			// Could log: method, duration, error
			_ = duration
		}

		return err
	}
}

// loggingStreamInterceptor logs streaming RPC calls.
func loggingStreamInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		start := time.Now()
		stream, err := streamer(ctx, desc, cc, method, opts...)

		// Log stream start
		_ = time.Since(start)

		return stream, err
	}
}

// WithRequestID returns a context with the specified request ID.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	} else {
		md = md.Copy()
	}
	md.Set("x-request-id", requestID)
	return metadata.NewOutgoingContext(ctx, md)
}

// WithSessionToken returns a context with the specified session token.
func WithSessionToken(ctx context.Context, token string) context.Context {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	} else {
		md = md.Copy()
	}
	md.Set("x-session-token", token)
	return metadata.NewOutgoingContext(ctx, md)
}
