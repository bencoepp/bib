// Package p2p provides peer-to-peer networking functionality for bib.
package p2p

import (
	"context"
	"net"
	"strings"
	"time"

	"bib/internal/logger"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"google.golang.org/grpc"
	grpcpeer "google.golang.org/grpc/peer"
)

// authTimeout is the timeout for authorization checks.
const authTimeout = 5 * time.Second

// GRPCServerConfig holds configuration for the P2P gRPC server.
type GRPCServerConfig struct {
	// Host is the libp2p host.
	Host host.Host

	// Authorizer handles peer authorization.
	Authorizer *PeerAuthorizer

	// RegisterServices is called to register gRPC services on the server.
	// This allows the caller to register their services without importing them here.
	RegisterServices func(*grpc.Server)

	// ServerOptions are additional gRPC server options.
	ServerOptions []grpc.ServerOption
}

// GRPCServer wraps a gRPC server that listens on libp2p streams.
type GRPCServer struct {
	host       host.Host
	server     *grpc.Server
	listener   *p2pListener
	authorizer *PeerAuthorizer
	log        *logger.Logger
}

// NewGRPCServer creates a new gRPC server that listens on libp2p streams.
func NewGRPCServer(cfg GRPCServerConfig) (*GRPCServer, error) {
	log := getLogger("grpc_server")

	// Build server options with authorization interceptor
	opts := cfg.ServerOptions
	if cfg.Authorizer != nil {
		opts = append(opts,
			grpc.ChainUnaryInterceptor(newAuthUnaryInterceptor(cfg.Authorizer, log)),
			grpc.ChainStreamInterceptor(newAuthStreamInterceptor(cfg.Authorizer, log)),
		)
	}

	// Create gRPC server
	server := grpc.NewServer(opts...)

	// Register services
	if cfg.RegisterServices != nil {
		cfg.RegisterServices(server)
	}

	return &GRPCServer{
		host:       cfg.Host,
		server:     server,
		authorizer: cfg.Authorizer,
		log:        log,
	}, nil
}

// Start starts the gRPC server.
func (s *GRPCServer) Start(ctx context.Context) error {
	s.listener = newP2PListener(ctx, s.host)

	s.log.Info("starting gRPC-over-P2P server",
		"peer_id", s.host.ID().String(),
		"protocol", ProtocolGRPC,
	)

	// Create a wrapper listener that handles authorization
	authListener := &authorizingListener{
		inner:      s.listener,
		authorizer: s.authorizer,
		log:        s.log,
	}

	// Serve in a goroutine
	go func() {
		if err := s.server.Serve(authListener); err != nil {
			// Ignore "use of closed network connection" on shutdown
			if !isClosedConnError(err) {
				s.log.Error("gRPC-over-P2P server error", "error", err)
			}
		}
	}()

	return nil
}

// Stop gracefully stops the gRPC server.
func (s *GRPCServer) Stop() {
	s.log.Info("stopping gRPC-over-P2P server")

	// Graceful stop with timeout
	stopped := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-context.Background().Done():
		s.server.Stop()
	}

	if s.listener != nil {
		_ = s.listener.Close()
	}
}

// Server returns the underlying gRPC server.
func (s *GRPCServer) Server() *grpc.Server {
	return s.server
}

// authorizingListener wraps a listener to authorize connections before accepting.
type authorizingListener struct {
	inner      net.Listener
	authorizer *PeerAuthorizer
	log        *logger.Logger
}

// Accept accepts a connection and authorizes the peer.
func (l *authorizingListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.inner.Accept()
		if err != nil {
			return nil, err
		}

		// Get peer ID from connection
		sc, ok := conn.(*streamConn)
		if !ok {
			// Not a stream connection, accept anyway
			return conn, nil
		}

		peerID := sc.PeerID()

		// Authorize the peer
		if l.authorizer != nil {
			ctx, cancel := context.WithTimeout(context.Background(), authTimeout)
			err := l.authorizer.Authorize(ctx, peerID)
			cancel()

			if err != nil {
				// Silently close the connection - don't reveal why
				l.log.Debug("rejecting unauthorized peer connection", "peer_id", peerID.String())
				_ = conn.Close()
				continue
			}
		}

		return conn, nil
	}
}

// Close closes the listener.
func (l *authorizingListener) Close() error {
	return l.inner.Close()
}

// Addr returns the listener's address.
func (l *authorizingListener) Addr() net.Addr {
	return l.inner.Addr()
}

// peerIDContextKey is used to store the peer ID in the gRPC context.
type peerIDContextKey struct{}

// PeerIDFromContext extracts the peer ID from a gRPC context.
func PeerIDFromContext(ctx context.Context) (peer.ID, bool) {
	v := ctx.Value(peerIDContextKey{})
	if v == nil {
		return "", false
	}
	return v.(peer.ID), true
}

// withPeerID adds a peer ID to the context.
func withPeerID(ctx context.Context, peerID peer.ID) context.Context {
	return context.WithValue(ctx, peerIDContextKey{}, peerID)
}

// newAuthUnaryInterceptor creates a unary interceptor that adds peer ID to context.
func newAuthUnaryInterceptor(_ *PeerAuthorizer, log *logger.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Extract peer ID from the connection (set during Accept)
		peerID := extractPeerIDFromContext(ctx)
		if peerID != "" {
			ctx = withPeerID(ctx, peerID)
		}

		// Check if this is an admin/breakglass service - reject over P2P
		if isRestrictedService(info.FullMethod) {
			log.Debug("rejecting restricted service call over P2P",
				"method", info.FullMethod,
				"peer_id", peerID.String(),
			)
			// Silently fail - don't reveal why
			return nil, nil
		}

		return handler(ctx, req)
	}
}

// newAuthStreamInterceptor creates a stream interceptor that adds peer ID to context.
func newAuthStreamInterceptor(_ *PeerAuthorizer, log *logger.Logger) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// Check if this is an admin/breakglass service - reject over P2P
		if isRestrictedService(info.FullMethod) {
			log.Debug("rejecting restricted service call over P2P",
				"method", info.FullMethod,
			)
			// Silently fail
			return nil
		}

		return handler(srv, ss)
	}
}

// extractPeerIDFromContext extracts the peer ID from the gRPC peer info.
func extractPeerIDFromContext(ctx context.Context) peer.ID {
	// The peer ID should be available from the connection's RemoteAddr
	p, ok := grpcpeer.FromContext(ctx)
	if !ok {
		return ""
	}

	// Our peerAddr implements net.Addr with the peer ID as the string
	if pa, ok := p.Addr.(*peerAddr); ok {
		return pa.id
	}

	// Try to decode from the address string
	peerID, err := peer.Decode(p.Addr.String())
	if err != nil {
		return ""
	}
	return peerID
}

// isRestrictedService returns true if the service should not be accessible over P2P.
func isRestrictedService(method string) bool {
	// Block admin and breakglass services over P2P
	restrictedPrefixes := []string{
		"/bib.v1.services.AdminService/",
		"/bib.v1.services.BreakGlassService/",
	}

	for _, prefix := range restrictedPrefixes {
		if strings.HasPrefix(method, prefix) {
			return true
		}
	}

	return false
}

// isClosedConnError returns true if the error indicates a closed connection.
func isClosedConnError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "use of closed network connection") ||
		strings.Contains(errStr, "listener closed")
}
