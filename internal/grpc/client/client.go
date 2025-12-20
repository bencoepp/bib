// Package client provides a gRPC client library for connecting to bibd.
package client

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"sync"
	"time"

	services "bib/api/gen/go/bib/v1/services"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Client is the main gRPC client for connecting to bibd.
type Client struct {
	opts Options

	// Connection pool
	pool     []*grpc.ClientConn
	poolIdx  int
	poolLock sync.Mutex

	// Authentication
	auth         *Authenticator
	sessionToken string
	tokenLock    sync.RWMutex

	// Service clients (lazy initialized)
	healthOnce  sync.Once
	healthSvc   services.HealthServiceClient
	authOnce    sync.Once
	authSvc     services.AuthServiceClient
	userOnce    sync.Once
	userSvc     services.UserServiceClient
	nodeOnce    sync.Once
	nodeSvc     services.NodeServiceClient
	topicOnce   sync.Once
	topicSvc    services.TopicServiceClient
	datasetOnce sync.Once
	datasetSvc  services.DatasetServiceClient
	adminOnce   sync.Once
	adminSvc    services.AdminServiceClient
	queryOnce   sync.Once
	querySvc    services.QueryServiceClient
	jobOnce     sync.Once
	jobSvc      services.JobServiceClient

	// Connection state
	connected   bool
	connectedTo string
	connLock    sync.RWMutex
}

// New creates a new Client with the given options.
func New(opts Options) (*Client, error) {
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	c := &Client{
		opts: opts,
	}

	// Initialize authenticator
	c.auth = NewAuthenticator(opts.Auth)

	return c, nil
}

// Connect establishes connection to the bibd daemon.
func (c *Client) Connect(ctx context.Context) error {
	c.connLock.Lock()
	defer c.connLock.Unlock()

	if c.connected {
		return nil
	}

	var conn *grpc.ClientConn
	var target string
	var err error

	if c.opts.Mode == ConnectionModeParallel {
		conn, target, err = c.connectParallel(ctx)
	} else {
		conn, target, err = c.connectSequential(ctx)
	}

	if err != nil {
		return err
	}

	// Initialize pool
	poolSize := c.opts.PoolSize
	if poolSize <= 0 {
		poolSize = 1
	}

	c.pool = make([]*grpc.ClientConn, poolSize)
	c.pool[0] = conn

	// Create additional connections for pool
	for i := 1; i < poolSize; i++ {
		poolConn, err := c.dialTarget(ctx, target)
		if err != nil {
			// Log warning but continue with fewer connections
			break
		}
		c.pool[i] = poolConn
	}

	c.connected = true
	c.connectedTo = target

	return nil
}

// connectSequential tries targets in order.
func (c *Client) connectSequential(ctx context.Context) (*grpc.ClientConn, string, error) {
	targets := c.buildTargetList()
	var lastErr error

	for _, target := range targets {
		conn, err := c.dialWithRetry(ctx, target)
		if err == nil {
			return conn, target, nil
		}
		lastErr = err
	}

	return nil, "", fmt.Errorf("failed to connect to any target: %w", lastErr)
}

// connectParallel tries all targets in parallel, uses first success.
func (c *Client) connectParallel(ctx context.Context) (*grpc.ClientConn, string, error) {
	targets := c.buildTargetList()
	if len(targets) == 0 {
		return nil, "", fmt.Errorf("no connection targets configured")
	}

	type result struct {
		conn   *grpc.ClientConn
		target string
		err    error
	}

	results := make(chan result, len(targets))
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, target := range targets {
		go func(t string) {
			conn, err := c.dialWithRetry(ctx, t)
			results <- result{conn: conn, target: t, err: err}
		}(target)
	}

	var lastErr error
	for i := 0; i < len(targets); i++ {
		r := <-results
		if r.err == nil {
			// Cancel other attempts
			cancel()
			return r.conn, r.target, nil
		}
		lastErr = r.err
	}

	return nil, "", fmt.Errorf("failed to connect to any target: %w", lastErr)
}

// buildTargetList returns targets in priority order.
func (c *Client) buildTargetList() []string {
	var targets []string

	// 1. Unix socket / named pipe (highest priority for local connections)
	if c.opts.UnixSocket != "" {
		if runtime.GOOS == "windows" {
			targets = append(targets, "pipe:"+c.opts.UnixSocket)
		} else {
			targets = append(targets, "unix:"+c.opts.UnixSocket)
		}
	}

	// 2. TCP address
	if c.opts.TCPAddress != "" {
		targets = append(targets, c.opts.TCPAddress)
	}

	// 3. P2P peer ID (lowest priority, requires P2P network)
	if c.opts.P2PPeerID != "" {
		targets = append(targets, "p2p:"+c.opts.P2PPeerID)
	}

	return targets
}

// dialWithRetry attempts to dial with retry logic.
func (c *Client) dialWithRetry(ctx context.Context, target string) (*grpc.ClientConn, error) {
	var lastErr error
	backoff := c.opts.RetryBackoff

	for attempt := 0; attempt <= c.opts.RetryAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				backoff *= 2 // Exponential backoff
			}
		}

		conn, err := c.dialTarget(ctx, target)
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}

	return nil, lastErr
}

// dialTarget establishes a connection to a specific target.
func (c *Client) dialTarget(ctx context.Context, target string) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		grpc.WithBlock(),
	}

	// Determine transport credentials
	if c.opts.TLS.Enabled && !isLocalTarget(target) {
		tlsConfig, err := c.opts.TLS.BuildTLSConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to build TLS config: %w", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Add client interceptors
	unaryInterceptors := c.buildUnaryInterceptors()
	streamInterceptors := c.buildStreamInterceptors()

	if len(unaryInterceptors) > 0 {
		opts = append(opts, grpc.WithChainUnaryInterceptor(unaryInterceptors...))
	}
	if len(streamInterceptors) > 0 {
		opts = append(opts, grpc.WithChainStreamInterceptor(streamInterceptors...))
	}

	// Handle different target types
	dialTarget := target
	if len(target) > 5 && target[:5] == "unix:" {
		dialTarget = target
		opts = append(opts, grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			return net.DialTimeout("unix", addr[5:], c.opts.Timeout)
		}))
	} else if len(target) > 5 && target[:5] == "pipe:" {
		// Windows named pipe
		dialTarget = target[5:]
		opts = append(opts, grpc.WithContextDialer(dialNamedPipe))
	} else if len(target) > 4 && target[:4] == "p2p:" {
		// P2P connection - would need libp2p integration
		return nil, fmt.Errorf("P2P connections not yet implemented")
	}

	dialCtx, cancel := context.WithTimeout(ctx, c.opts.Timeout)
	defer cancel()

	return grpc.DialContext(dialCtx, dialTarget, opts...)
}

// isLocalTarget returns true if the target is a local connection.
func isLocalTarget(target string) bool {
	if len(target) > 5 && (target[:5] == "unix:" || target[:5] == "pipe:") {
		return true
	}
	return false
}

// getConn returns a connection from the pool.
func (c *Client) getConn() (*grpc.ClientConn, error) {
	c.connLock.RLock()
	if !c.connected || len(c.pool) == 0 {
		c.connLock.RUnlock()
		return nil, fmt.Errorf("client not connected")
	}
	c.connLock.RUnlock()

	c.poolLock.Lock()
	defer c.poolLock.Unlock()

	// Round-robin through pool
	for i := 0; i < len(c.pool); i++ {
		idx := (c.poolIdx + i) % len(c.pool)
		conn := c.pool[idx]
		if conn != nil && conn.GetState() == connectivity.Ready {
			c.poolIdx = (idx + 1) % len(c.pool)
			return conn, nil
		}
	}

	// Return first non-nil connection
	for _, conn := range c.pool {
		if conn != nil {
			return conn, nil
		}
	}

	return nil, fmt.Errorf("no available connections in pool")
}

// Close closes all connections.
func (c *Client) Close() error {
	c.connLock.Lock()
	defer c.connLock.Unlock()

	var errs []error
	for _, conn := range c.pool {
		if conn != nil {
			if err := conn.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}

	c.pool = nil
	c.connected = false
	c.connectedTo = ""

	// Reset service clients
	c.healthOnce = sync.Once{}
	c.authOnce = sync.Once{}
	c.userOnce = sync.Once{}
	c.nodeOnce = sync.Once{}
	c.topicOnce = sync.Once{}
	c.datasetOnce = sync.Once{}
	c.adminOnce = sync.Once{}
	c.queryOnce = sync.Once{}
	c.jobOnce = sync.Once{}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing connections: %v", errs)
	}
	return nil
}

// IsConnected returns whether the client is connected.
func (c *Client) IsConnected() bool {
	c.connLock.RLock()
	defer c.connLock.RUnlock()
	return c.connected
}

// ConnectedTo returns the address of the connected target.
func (c *Client) ConnectedTo() string {
	c.connLock.RLock()
	defer c.connLock.RUnlock()
	return c.connectedTo
}

// ============================================================================
// Service Accessors
// ============================================================================

// Health returns the HealthService client.
func (c *Client) Health() (services.HealthServiceClient, error) {
	conn, err := c.getConn()
	if err != nil {
		return nil, err
	}
	c.healthOnce.Do(func() {
		c.healthSvc = services.NewHealthServiceClient(conn)
	})
	return c.healthSvc, nil
}

// Auth returns the AuthService client.
func (c *Client) Auth() (services.AuthServiceClient, error) {
	conn, err := c.getConn()
	if err != nil {
		return nil, err
	}
	c.authOnce.Do(func() {
		c.authSvc = services.NewAuthServiceClient(conn)
	})
	return c.authSvc, nil
}

// User returns the UserService client.
func (c *Client) User() (services.UserServiceClient, error) {
	conn, err := c.getConn()
	if err != nil {
		return nil, err
	}
	c.userOnce.Do(func() {
		c.userSvc = services.NewUserServiceClient(conn)
	})
	return c.userSvc, nil
}

// Node returns the NodeService client.
func (c *Client) Node() (services.NodeServiceClient, error) {
	conn, err := c.getConn()
	if err != nil {
		return nil, err
	}
	c.nodeOnce.Do(func() {
		c.nodeSvc = services.NewNodeServiceClient(conn)
	})
	return c.nodeSvc, nil
}

// Topic returns the TopicService client.
func (c *Client) Topic() (services.TopicServiceClient, error) {
	conn, err := c.getConn()
	if err != nil {
		return nil, err
	}
	c.topicOnce.Do(func() {
		c.topicSvc = services.NewTopicServiceClient(conn)
	})
	return c.topicSvc, nil
}

// Dataset returns the DatasetService client.
func (c *Client) Dataset() (services.DatasetServiceClient, error) {
	conn, err := c.getConn()
	if err != nil {
		return nil, err
	}
	c.datasetOnce.Do(func() {
		c.datasetSvc = services.NewDatasetServiceClient(conn)
	})
	return c.datasetSvc, nil
}

// Admin returns the AdminService client.
func (c *Client) Admin() (services.AdminServiceClient, error) {
	conn, err := c.getConn()
	if err != nil {
		return nil, err
	}
	c.adminOnce.Do(func() {
		c.adminSvc = services.NewAdminServiceClient(conn)
	})
	return c.adminSvc, nil
}

// Query returns the QueryService client.
func (c *Client) Query() (services.QueryServiceClient, error) {
	conn, err := c.getConn()
	if err != nil {
		return nil, err
	}
	c.queryOnce.Do(func() {
		c.querySvc = services.NewQueryServiceClient(conn)
	})
	return c.querySvc, nil
}

// Job returns the JobService client.
func (c *Client) Job() (services.JobServiceClient, error) {
	conn, err := c.getConn()
	if err != nil {
		return nil, err
	}
	c.jobOnce.Do(func() {
		c.jobSvc = services.NewJobServiceClient(conn)
	})
	return c.jobSvc, nil
}

// GetPublicKey returns the public key used for authentication.
func (c *Client) GetPublicKey() (string, error) {
	return c.auth.GetPublicKey()
}
