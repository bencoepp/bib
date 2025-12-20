// Package grpc provides gRPC service implementations for the bib daemon.
package grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"sync"
	"time"

	services "bib/api/gen/go/bib/v1/services"
	"bib/internal/config"
	"bib/internal/grpc/interfaces"
	"bib/internal/grpc/middleware"
	"bib/internal/version"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

// Server represents the gRPC server with all its listeners and lifecycle management.
type Server struct {
	cfg        config.GRPCConfig
	tlsConfig  *tls.Config
	serverHost string // Fallback host from ServerConfig

	grpcServer *grpc.Server
	services   *ServiceServers

	// Listeners
	tcpListener  net.Listener
	pipeListener net.Listener // Unix socket on Unix, named pipe on Windows

	// Metrics
	metricsServer   *http.Server
	metricsRegistry *prometheus.Registry
	grpcMetrics     *grpc_prometheus.ServerMetrics

	// Interceptor dependencies
	healthProvider  interfaces.HealthProvider
	auditMiddleware *middleware.AuditMiddleware
	rbacConfig      middleware.RBACConfig
	getUserFunc     func(ctx context.Context, token string) (*interface{}, error)

	// Lifecycle
	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// ServerConfig holds all dependencies needed to create a Server.
type ServerConfig struct {
	// GRPCConfig is the gRPC-specific configuration.
	GRPCConfig config.GRPCConfig

	// ServerHost is the fallback host if GRPCConfig.Host is empty.
	ServerHost string

	// TLSConfig is the TLS configuration for secure connections.
	// If nil, the server runs without TLS (not recommended for production).
	TLSConfig *tls.Config

	// HealthProvider provides health check information.
	HealthProvider interfaces.HealthProvider

	// AuditMiddleware provides audit logging (optional).
	AuditMiddleware *middleware.AuditMiddleware

	// RBACConfig holds RBAC settings.
	RBACConfig middleware.RBACConfig

	// GetUserFromToken extracts user from session token for RBAC.
	// Required if RBAC is enabled.
	GetUserFromToken func(ctx context.Context, token string) (*interface{}, error)
}

// NewServer creates a new gRPC server with all interceptors configured.
func NewServer(cfg ServerConfig) (*Server, error) {
	if !cfg.GRPCConfig.Enabled {
		return nil, fmt.Errorf("gRPC server is disabled in configuration")
	}

	s := &Server{
		cfg:             cfg.GRPCConfig,
		tlsConfig:       cfg.TLSConfig,
		serverHost:      cfg.ServerHost,
		services:        NewServiceServers(),
		healthProvider:  cfg.HealthProvider,
		auditMiddleware: cfg.AuditMiddleware,
		rbacConfig:      cfg.RBACConfig,
		stopCh:          make(chan struct{}),
	}

	// Set up Prometheus metrics if enabled
	if cfg.GRPCConfig.Metrics.Enabled {
		s.metricsRegistry = prometheus.NewRegistry()
		s.grpcMetrics = grpc_prometheus.NewServerMetrics()

		if cfg.GRPCConfig.Metrics.EnableLatencyHistograms {
			s.grpcMetrics.EnableHandlingTimeHistogram()
		}

		s.metricsRegistry.MustRegister(s.grpcMetrics)

		// Register standard Go metrics
		s.metricsRegistry.MustRegister(prometheus.NewGoCollector())
		s.metricsRegistry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	}

	// Build interceptor chains
	unaryInterceptors := s.buildUnaryInterceptors()
	streamInterceptors := s.buildStreamInterceptors()

	// Build server options
	opts := s.buildServerOptions(unaryInterceptors, streamInterceptors)

	// Create gRPC server
	s.grpcServer = grpc.NewServer(opts...)

	// Register all services
	s.registerServices()

	// Initialize metrics after service registration
	if s.grpcMetrics != nil {
		s.grpcMetrics.InitializeMetrics(s.grpcServer)
	}

	// Enable reflection only in development builds
	if cfg.GRPCConfig.Reflection && version.IsDev() {
		reflection.Register(s.grpcServer)
	} else if cfg.GRPCConfig.Reflection && !version.IsDev() {
		// Log warning that reflection was requested but denied
		fmt.Println("WARNING: gRPC reflection requested but disabled in release build")
	}

	return s, nil
}

// buildServerOptions creates the gRPC server options.
func (s *Server) buildServerOptions(unary []grpc.UnaryServerInterceptor, stream []grpc.StreamServerInterceptor) []grpc.ServerOption {
	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(s.cfg.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(s.cfg.MaxSendMsgSize),
		grpc.MaxConcurrentStreams(s.cfg.MaxConcurrentStreams),
	}

	// Keepalive settings
	opts = append(opts, grpc.KeepaliveParams(keepalive.ServerParameters{
		Time:    s.cfg.Keepalive.Time,
		Timeout: s.cfg.Keepalive.Timeout,
	}))

	opts = append(opts, grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
		MinTime:             s.cfg.Keepalive.MinTime,
		PermitWithoutStream: s.cfg.Keepalive.PermitWithoutStream,
	}))

	// TLS credentials
	if s.tlsConfig != nil {
		opts = append(opts, grpc.Creds(credentials.NewTLS(s.tlsConfig)))
	}

	// Interceptor chains
	opts = append(opts, grpc.ChainUnaryInterceptor(unary...))
	opts = append(opts, grpc.ChainStreamInterceptor(stream...))

	return opts
}

// buildUnaryInterceptors creates the chain of unary interceptors.
func (s *Server) buildUnaryInterceptors() []grpc.UnaryServerInterceptor {
	var interceptors []grpc.UnaryServerInterceptor

	// 1. Metrics (first, to capture everything)
	if s.grpcMetrics != nil {
		interceptors = append(interceptors, s.grpcMetrics.UnaryServerInterceptor())
	}

	// 2. Recovery (catch panics early)
	interceptors = append(interceptors, middleware.RecoveryUnaryInterceptor())

	// 3. Request ID
	interceptors = append(interceptors, middleware.RequestIDUnaryInterceptor())

	// 4. Logging
	interceptors = append(interceptors, middleware.LoggingUnaryInterceptor())

	// 5. Rate limiting (per-user, after we know the user)
	if s.cfg.RateLimit.Enabled {
		limiter := middleware.NewRateLimiter(s.cfg.RateLimit.RequestsPerSecond, s.cfg.RateLimit.Burst)
		interceptors = append(interceptors, middleware.RateLimitUnaryInterceptor(limiter, middleware.UserFromContext))
	}

	// 6. Audit (for mutations)
	if s.auditMiddleware != nil {
		interceptors = append(interceptors, middleware.AuditUnaryInterceptor(s.auditMiddleware))
	}

	return interceptors
}

// buildStreamInterceptors creates the chain of stream interceptors.
func (s *Server) buildStreamInterceptors() []grpc.StreamServerInterceptor {
	var interceptors []grpc.StreamServerInterceptor

	// 1. Metrics
	if s.grpcMetrics != nil {
		interceptors = append(interceptors, s.grpcMetrics.StreamServerInterceptor())
	}

	// 2. Recovery
	interceptors = append(interceptors, middleware.RecoveryStreamInterceptor())

	// 3. Request ID
	interceptors = append(interceptors, middleware.RequestIDStreamInterceptor())

	// 4. Logging
	interceptors = append(interceptors, middleware.LoggingStreamInterceptor())

	// 5. Rate limiting
	if s.cfg.RateLimit.Enabled {
		limiter := middleware.NewRateLimiter(s.cfg.RateLimit.RequestsPerSecond, s.cfg.RateLimit.Burst)
		interceptors = append(interceptors, middleware.RateLimitStreamInterceptor(limiter, middleware.UserFromContext))
	}

	// 6. Audit
	if s.auditMiddleware != nil {
		interceptors = append(interceptors, middleware.AuditStreamInterceptor(s.auditMiddleware))
	}

	return interceptors
}

// registerServices registers all gRPC services with the server.
func (s *Server) registerServices() {
	// Set health provider if available
	if s.healthProvider != nil {
		s.services.Health.SetProvider(s.healthProvider)
	}

	// Register all services
	services.RegisterHealthServiceServer(s.grpcServer, s.services.Health)
	services.RegisterAuthServiceServer(s.grpcServer, s.services.Auth)
	services.RegisterUserServiceServer(s.grpcServer, s.services.User)
	services.RegisterNodeServiceServer(s.grpcServer, s.services.Node)
	services.RegisterTopicServiceServer(s.grpcServer, s.services.Topic)
	services.RegisterDatasetServiceServer(s.grpcServer, s.services.Dataset)
	services.RegisterAdminServiceServer(s.grpcServer, s.services.Admin)
	services.RegisterQueryServiceServer(s.grpcServer, s.services.Query)
	services.RegisterJobServiceServer(s.grpcServer, s.services.Job)
	services.RegisterBreakGlassServiceServer(s.grpcServer, s.services.BreakGlass)
}

// Start begins listening on all configured endpoints.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server already running")
	}
	s.running = true
	s.mu.Unlock()

	// Start TCP listener
	if err := s.startTCPListener(); err != nil {
		return fmt.Errorf("failed to start TCP listener: %w", err)
	}

	// Start Unix socket / named pipe listener if configured
	if s.cfg.UnixSocket != "" {
		if err := s.startPipeListener(); err != nil {
			s.stopTCPListener()
			return fmt.Errorf("failed to start pipe listener: %w", err)
		}
	}

	// Start metrics HTTP server if enabled
	if s.cfg.Metrics.Enabled && s.cfg.Metrics.HTTPPort > 0 {
		if err := s.startMetricsServer(); err != nil {
			s.stopPipeListener()
			s.stopTCPListener()
			return fmt.Errorf("failed to start metrics server: %w", err)
		}
	}

	return nil
}

// startTCPListener starts the TCP listener for gRPC.
func (s *Server) startTCPListener() error {
	host := s.cfg.Host
	if host == "" {
		host = s.serverHost
	}
	addr := fmt.Sprintf("%s:%d", host, s.cfg.Port)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	s.tcpListener = lis

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.grpcServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			fmt.Printf("gRPC TCP server error: %v\n", err)
		}
	}()

	fmt.Printf("gRPC server listening on %s\n", addr)
	return nil
}

// startPipeListener starts the Unix socket or named pipe listener.
func (s *Server) startPipeListener() error {
	if runtime.GOOS == "windows" {
		return s.startNamedPipe()
	}
	return s.startUnixSocket()
}

// startUnixSocket starts a Unix socket listener (Unix systems).
func (s *Server) startUnixSocket() error {
	// Remove existing socket file if it exists
	// (Unix sockets leave files behind)
	_ = removeSocketFile(s.cfg.UnixSocket)

	lis, err := net.Listen("unix", s.cfg.UnixSocket)
	if err != nil {
		return fmt.Errorf("failed to listen on unix socket %s: %w", s.cfg.UnixSocket, err)
	}
	s.pipeListener = lis

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		// Create a separate server for Unix socket (no TLS needed for local)
		unixServer := s.createLocalServer()
		if err := unixServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			fmt.Printf("gRPC Unix socket server error: %v\n", err)
		}
	}()

	fmt.Printf("gRPC server listening on unix://%s\n", s.cfg.UnixSocket)
	return nil
}

// startNamedPipe starts a named pipe listener (Windows).
func (s *Server) startNamedPipe() error {
	// On Windows, use named pipes via npipe or winio
	// The socket path is treated as a pipe name
	pipeName := s.cfg.UnixSocket
	if pipeName == "" {
		pipeName = `\\.\pipe\bibd-grpc`
	}

	lis, err := listenNamedPipe(pipeName)
	if err != nil {
		return fmt.Errorf("failed to listen on named pipe %s: %w", pipeName, err)
	}
	s.pipeListener = lis

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		// Create a separate server for named pipe (no TLS needed for local)
		pipeServer := s.createLocalServer()
		if err := pipeServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			fmt.Printf("gRPC named pipe server error: %v\n", err)
		}
	}()

	fmt.Printf("gRPC server listening on pipe://%s\n", pipeName)
	return nil
}

// createLocalServer creates a gRPC server for local connections (no TLS).
func (s *Server) createLocalServer() *grpc.Server {
	// Build interceptors (same as main server)
	unaryInterceptors := s.buildUnaryInterceptors()
	streamInterceptors := s.buildStreamInterceptors()

	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(s.cfg.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(s.cfg.MaxSendMsgSize),
		grpc.MaxConcurrentStreams(s.cfg.MaxConcurrentStreams),
		grpc.ChainUnaryInterceptor(unaryInterceptors...),
		grpc.ChainStreamInterceptor(streamInterceptors...),
	}

	localServer := grpc.NewServer(opts...)

	// Register same services
	services.RegisterHealthServiceServer(localServer, s.services.Health)
	services.RegisterAuthServiceServer(localServer, s.services.Auth)
	services.RegisterUserServiceServer(localServer, s.services.User)
	services.RegisterNodeServiceServer(localServer, s.services.Node)
	services.RegisterTopicServiceServer(localServer, s.services.Topic)
	services.RegisterDatasetServiceServer(localServer, s.services.Dataset)
	services.RegisterAdminServiceServer(localServer, s.services.Admin)
	services.RegisterQueryServiceServer(localServer, s.services.Query)
	services.RegisterJobServiceServer(localServer, s.services.Job)
	services.RegisterBreakGlassServiceServer(localServer, s.services.BreakGlass)

	if s.cfg.Reflection && version.IsDev() {
		reflection.Register(localServer)
	}

	return localServer
}

// startMetricsServer starts the Prometheus metrics HTTP server.
func (s *Server) startMetricsServer() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Metrics.HTTPHost, s.cfg.Metrics.HTTPPort)

	mux := http.NewServeMux()
	mux.Handle(s.cfg.Metrics.Path, promhttp.HandlerFor(s.metricsRegistry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}))

	s.metricsServer = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Metrics server error: %v\n", err)
		}
	}()

	fmt.Printf("Prometheus metrics available at http://%s%s\n", addr, s.cfg.Metrics.Path)
	return nil
}

// Stop gracefully stops the gRPC server with connection draining.
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	close(s.stopCh)
	s.mu.Unlock()

	// Create a context with timeout for graceful shutdown
	gracePeriod := s.cfg.ShutdownGracePeriod
	if gracePeriod == 0 {
		gracePeriod = 30 * time.Second
	}
	shutdownCtx, cancel := context.WithTimeout(ctx, gracePeriod)
	defer cancel()

	var errs []error

	// Stop accepting new connections and drain existing ones
	stopped := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		// Graceful shutdown completed
	case <-shutdownCtx.Done():
		// Force stop if grace period exceeded
		s.grpcServer.Stop()
		errs = append(errs, fmt.Errorf("graceful shutdown timed out, forced stop"))
	}

	// Stop metrics server
	if s.metricsServer != nil {
		if err := s.metricsServer.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("metrics server shutdown: %w", err))
		}
	}

	// Clean up listeners
	s.stopPipeListener()
	s.stopTCPListener()

	// Wait for all goroutines
	s.wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	return nil
}

func (s *Server) stopTCPListener() {
	if s.tcpListener != nil {
		_ = s.tcpListener.Close()
		s.tcpListener = nil
	}
}

func (s *Server) stopPipeListener() {
	if s.pipeListener != nil {
		_ = s.pipeListener.Close()
		s.pipeListener = nil
	}
	// Clean up Unix socket file
	if s.cfg.UnixSocket != "" && runtime.GOOS != "windows" {
		_ = removeSocketFile(s.cfg.UnixSocket)
	}
}

// Services returns the service servers for dependency injection.
func (s *Server) Services() *ServiceServers {
	return s.services
}

// GRPCServer returns the underlying gRPC server.
func (s *Server) GRPCServer() *grpc.Server {
	return s.grpcServer
}

// Address returns the TCP listener address.
func (s *Server) Address() string {
	if s.tcpListener != nil {
		return s.tcpListener.Addr().String()
	}
	return ""
}

// IsRunning returns whether the server is currently running.
func (s *Server) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}
