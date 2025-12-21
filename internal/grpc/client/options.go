// Package client provides a gRPC client library for connecting to bibd.
package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"
)

// ConnectionMode defines how connection targets are tried.
type ConnectionMode string

const (
	// ConnectionModeSequential tries targets in order: socket → TCP → P2P
	ConnectionModeSequential ConnectionMode = "sequential"

	// ConnectionModeParallel tries all targets in parallel, uses first success
	ConnectionModeParallel ConnectionMode = "parallel"
)

// Options configures the gRPC client connection.
type Options struct {
	// Connection targets (in priority order for sequential mode)
	UnixSocket string // Unix socket path (or named pipe on Windows)
	TCPAddress string // Direct TCP address (host:port)
	P2PPeerID  string // P2P peer ID for libp2p connection

	// Connection behavior
	Mode          ConnectionMode // How to try multiple targets
	Timeout       time.Duration  // Connection timeout
	RetryAttempts int            // Number of retry attempts
	RetryBackoff  time.Duration  // Initial backoff between retries

	// Connection pool
	PoolSize int // Maximum connections in pool (0 = single connection)

	// TLS configuration
	TLS TLSOptions

	// Authentication
	Auth AuthOptions

	// TOFU (Trust On First Use) callback
	// Called when connecting to a node for the first time
	// Returns (trusted, error) - if false, connection is rejected
	TOFUCallback TOFUCallbackFunc

	// Interceptors
	RequestIDEnabled bool // Add request ID to all calls
	LoggingEnabled   bool // Log all RPC calls
}

// TOFUCallbackFunc is called to verify a new server certificate
// nodeID is the server's node ID, certPEM is the server's certificate
// Returns (trusted, error) - return true to trust, false to reject
type TOFUCallbackFunc func(nodeID string, certPEM []byte) (bool, error)

// TLSOptions configures TLS for the connection.
type TLSOptions struct {
	// Enabled enables TLS (default true for TCP, false for Unix socket)
	Enabled bool

	// InsecureSkipVerify skips server certificate verification (DANGEROUS)
	InsecureSkipVerify bool

	// CAFile is the path to a custom CA certificate file
	CAFile string

	// CertFile is the path to the client certificate file (for mTLS)
	CertFile string

	// KeyFile is the path to the client key file (for mTLS)
	KeyFile string

	// ServerName overrides the server name for certificate verification
	ServerName string
}

// AuthOptions configures client authentication.
type AuthOptions struct {
	// SessionTokenPath is where to store/load the session token
	// Default: <config_dir>/session.token
	SessionTokenPath string

	// SSHKeyPath is the path to the SSH private key for authentication
	// If empty, uses SSH agent
	SSHKeyPath string

	// SSHKeyPassphrase is the passphrase for encrypted SSH keys
	// If empty and key is encrypted, will prompt or fail
	SSHKeyPassphrase string

	// UseSSHAgent enables SSH agent for key operations
	UseSSHAgent bool

	// AutoAuth automatically authenticates when token is missing/expired
	AutoAuth bool
}

// DefaultOptions returns sensible default options.
func DefaultOptions() Options {
	return Options{
		Mode:             ConnectionModeSequential,
		Timeout:          30 * time.Second,
		RetryAttempts:    3,
		RetryBackoff:     1 * time.Second,
		PoolSize:         5,
		RequestIDEnabled: true,
		LoggingEnabled:   false,
		TLS: TLSOptions{
			Enabled: true,
		},
		Auth: AuthOptions{
			UseSSHAgent: true,
			AutoAuth:    true,
		},
	}
}

// WithUnixSocket sets the Unix socket path.
func (o Options) WithUnixSocket(path string) Options {
	o.UnixSocket = path
	return o
}

// WithTCPAddress sets the TCP address.
func (o Options) WithTCPAddress(addr string) Options {
	o.TCPAddress = addr
	return o
}

// WithP2PPeerID sets the P2P peer ID.
func (o Options) WithP2PPeerID(peerID string) Options {
	o.P2PPeerID = peerID
	return o
}

// WithMode sets the connection mode.
func (o Options) WithMode(mode ConnectionMode) Options {
	o.Mode = mode
	return o
}

// WithTimeout sets the connection timeout.
func (o Options) WithTimeout(timeout time.Duration) Options {
	o.Timeout = timeout
	return o
}

// WithRetry sets retry configuration.
func (o Options) WithRetry(attempts int, backoff time.Duration) Options {
	o.RetryAttempts = attempts
	o.RetryBackoff = backoff
	return o
}

// WithPoolSize sets the connection pool size.
func (o Options) WithPoolSize(size int) Options {
	o.PoolSize = size
	return o
}

// WithTLS configures TLS options.
func (o Options) WithTLS(tls TLSOptions) Options {
	o.TLS = tls
	return o
}

// WithAuth configures authentication options.
func (o Options) WithAuth(auth AuthOptions) Options {
	o.Auth = auth
	return o
}

// WithInsecure disables TLS (DANGEROUS - only for local development).
func (o Options) WithInsecure() Options {
	o.TLS.Enabled = false
	return o
}

// WithInsecureSkipVerify skips TLS certificate verification (DANGEROUS).
func (o Options) WithInsecureSkipVerify(skip bool) Options {
	o.TLS.InsecureSkipVerify = skip
	return o
}

// WithTOFUCallback sets the TOFU callback for certificate verification.
func (o Options) WithTOFUCallback(callback TOFUCallbackFunc) Options {
	o.TOFUCallback = callback
	return o
}

// BuildTLSConfig builds a tls.Config from the options.
func (o *TLSOptions) BuildTLSConfig() (*tls.Config, error) {
	if !o.Enabled {
		return nil, nil
	}

	config := &tls.Config{
		InsecureSkipVerify: o.InsecureSkipVerify,
		ServerName:         o.ServerName,
	}

	// Load custom CA if specified
	if o.CAFile != "" {
		caCert, err := os.ReadFile(o.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		config.RootCAs = caCertPool
	}

	// Load client certificate if specified (mTLS)
	if o.CertFile != "" && o.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(o.CertFile, o.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		config.Certificates = []tls.Certificate{cert}
	}

	return config, nil
}

// Validate checks that the options are valid.
func (o *Options) Validate() error {
	if o.UnixSocket == "" && o.TCPAddress == "" && o.P2PPeerID == "" {
		return fmt.Errorf("at least one connection target must be specified")
	}

	if o.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	if o.RetryAttempts < 0 {
		return fmt.Errorf("retry attempts must be non-negative")
	}

	if o.PoolSize < 0 {
		return fmt.Errorf("pool size must be non-negative")
	}

	return nil
}
