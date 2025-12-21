package discovery

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	services "bib/api/gen/go/bib/v1/services"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// ConnectionStatus represents the status of a connection test
type ConnectionStatus string

const (
	StatusConnected    ConnectionStatus = "connected"
	StatusDisconnected ConnectionStatus = "disconnected"
	StatusTimeout      ConnectionStatus = "timeout"
	StatusRefused      ConnectionStatus = "refused"
	StatusUnreachable  ConnectionStatus = "unreachable"
	StatusAuthFailed   ConnectionStatus = "auth_failed"
	StatusTLSError     ConnectionStatus = "tls_error"
	StatusUnknown      ConnectionStatus = "unknown"
)

// ConnectionTestResult contains the result of testing a connection to a node
type ConnectionTestResult struct {
	// Address is the node address that was tested
	Address string

	// Status is the connection status
	Status ConnectionStatus

	// Latency is the measured connection latency (round-trip)
	Latency time.Duration

	// NodeInfo contains node information retrieved from the health endpoint
	NodeInfo *NodeInfo

	// TLSInfo contains TLS certificate information if available
	TLSInfo *TLSInfo

	// Error contains any error message
	Error string

	// TestedAt is when the test was performed
	TestedAt time.Time
}

// TLSInfo contains TLS certificate information
type TLSInfo struct {
	// Enabled indicates if TLS is enabled
	Enabled bool

	// Fingerprint is the certificate fingerprint (SHA256)
	Fingerprint string

	// Subject is the certificate subject
	Subject string

	// Issuer is the certificate issuer
	Issuer string

	// NotAfter is the certificate expiry time
	NotAfter time.Time

	// Trusted indicates if the certificate is in the trust store
	Trusted bool
}

// ConnectionTester tests connections to bibd nodes
type ConnectionTester struct {
	// Timeout for connection attempts
	Timeout time.Duration

	// UseTLS enables TLS for connections
	UseTLS bool

	// SkipTLSVerify disables TLS certificate verification
	SkipTLSVerify bool
}

// NewConnectionTester creates a new connection tester with default settings
func NewConnectionTester() *ConnectionTester {
	return &ConnectionTester{
		Timeout:       10 * time.Second,
		UseTLS:        false, // Auto-detect
		SkipTLSVerify: true,  // For testing, skip verification
	}
}

// WithTimeout sets the connection timeout
func (t *ConnectionTester) WithTimeout(timeout time.Duration) *ConnectionTester {
	t.Timeout = timeout
	return t
}

// WithTLS enables or disables TLS
func (t *ConnectionTester) WithTLS(useTLS bool) *ConnectionTester {
	t.UseTLS = useTLS
	return t
}

// WithSkipTLSVerify sets whether to skip TLS certificate verification
func (t *ConnectionTester) WithSkipTLSVerify(skip bool) *ConnectionTester {
	t.SkipTLSVerify = skip
	return t
}

// TestConnection tests a connection to a single node
func (t *ConnectionTester) TestConnection(ctx context.Context, address string) *ConnectionTestResult {
	result := &ConnectionTestResult{
		Address:  address,
		Status:   StatusUnknown,
		TestedAt: time.Now(),
	}

	// Create context with timeout
	if t.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t.Timeout)
		defer cancel()
	}

	// Measure latency
	start := time.Now()

	// First test TCP connectivity
	tcpConn, err := t.testTCPConnection(ctx, address)
	if err != nil {
		result.Status = t.classifyError(err)
		result.Error = err.Error()
		result.Latency = time.Since(start)
		return result
	}
	tcpConn.Close()

	tcpLatency := time.Since(start)

	// Now try gRPC connection
	grpcStart := time.Now()
	conn, tlsInfo, err := t.dialGRPC(ctx, address)
	if err != nil {
		result.Status = t.classifyError(err)
		result.Error = err.Error()
		result.Latency = tcpLatency
		result.TLSInfo = tlsInfo
		return result
	}
	defer conn.Close()

	result.TLSInfo = tlsInfo

	// Get health/node info
	nodeInfo, err := t.getNodeInfo(ctx, conn)
	if err != nil {
		// Connection works but couldn't get node info
		result.Status = StatusConnected
		result.Latency = time.Since(grpcStart)
		result.Error = fmt.Sprintf("connected but health check failed: %v", err)
		return result
	}

	result.Status = StatusConnected
	result.Latency = time.Since(grpcStart)
	result.NodeInfo = nodeInfo

	return result
}

// TestConnections tests connections to multiple nodes in parallel
func (t *ConnectionTester) TestConnections(ctx context.Context, addresses []string) []*ConnectionTestResult {
	results := make([]*ConnectionTestResult, len(addresses))
	var wg sync.WaitGroup

	for i, addr := range addresses {
		wg.Add(1)
		go func(idx int, address string) {
			defer wg.Done()
			results[idx] = t.TestConnection(ctx, address)
		}(i, addr)
	}

	wg.Wait()
	return results
}

// TestNodes tests connections to discovered nodes
func (t *ConnectionTester) TestNodes(ctx context.Context, nodes []DiscoveredNode) []*ConnectionTestResult {
	addresses := make([]string, len(nodes))
	for i, node := range nodes {
		addresses[i] = node.Address
	}
	return t.TestConnections(ctx, addresses)
}

// testTCPConnection tests basic TCP connectivity
func (t *ConnectionTester) testTCPConnection(ctx context.Context, address string) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout: t.Timeout,
	}
	return dialer.DialContext(ctx, "tcp", address)
}

// dialGRPC creates a gRPC connection to the address
func (t *ConnectionTester) dialGRPC(ctx context.Context, address string) (*grpc.ClientConn, *TLSInfo, error) {
	var opts []grpc.DialOption
	var tlsInfo *TLSInfo

	// Try TLS first, fallback to insecure
	if t.UseTLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: t.SkipTLSVerify,
		}
		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.WithTransportCredentials(creds))
		tlsInfo = &TLSInfo{Enabled: true}
	} else {
		// Try to detect TLS
		tlsInfo = t.probeTLS(ctx, address)
		if tlsInfo != nil && tlsInfo.Enabled {
			tlsConfig := &tls.Config{
				InsecureSkipVerify: t.SkipTLSVerify,
			}
			creds := credentials.NewTLS(tlsConfig)
			opts = append(opts, grpc.WithTransportCredentials(creds))
		} else {
			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		}
	}

	opts = append(opts, grpc.WithBlock())

	conn, err := grpc.DialContext(ctx, address, opts...)
	return conn, tlsInfo, err
}

// probeTLS attempts to detect if the server uses TLS
func (t *ConnectionTester) probeTLS(ctx context.Context, address string) *TLSInfo {
	// Try TLS connection with skip verify
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
	}

	dialer := &net.Dialer{
		Timeout: 2 * time.Second,
	}

	conn, err := tls.DialWithDialer(dialer, "tcp", address, tlsConfig)
	if err != nil {
		return &TLSInfo{Enabled: false}
	}
	defer conn.Close()

	// Get certificate info
	state := conn.ConnectionState()
	if len(state.PeerCertificates) > 0 {
		cert := state.PeerCertificates[0]
		return &TLSInfo{
			Enabled:     true,
			Subject:     cert.Subject.String(),
			Issuer:      cert.Issuer.String(),
			NotAfter:    cert.NotAfter,
			Fingerprint: fingerprintCert(cert.Raw),
		}
	}

	return &TLSInfo{Enabled: true}
}

// getNodeInfo retrieves node information via gRPC health service
func (t *ConnectionTester) getNodeInfo(ctx context.Context, conn *grpc.ClientConn) (*NodeInfo, error) {
	client := services.NewHealthServiceClient(conn)

	// First try GetNodeInfo for detailed information
	nodeResp, err := client.GetNodeInfo(ctx, &services.GetNodeInfoRequest{})
	if err == nil {
		return &NodeInfo{
			Version: nodeResp.GetVersion(),
			Mode:    nodeResp.GetMode(),
			PeerID:  nodeResp.GetNodeId(),
		}, nil
	}

	// Fall back to basic health check
	resp, err := client.Check(ctx, &services.HealthCheckRequest{})
	if err != nil {
		return nil, err
	}

	info := &NodeInfo{}

	// If serving, connection is healthy
	if resp.GetStatus() == services.ServingStatus_SERVING_STATUS_SERVING {
		// Basic connection confirmed
	}

	return info, nil
}

// classifyError classifies a connection error into a status
func (t *ConnectionTester) classifyError(err error) ConnectionStatus {
	if err == nil {
		return StatusConnected
	}

	errStr := err.Error()

	// Check for common error patterns
	if netErr, ok := err.(net.Error); ok {
		if netErr.Timeout() {
			return StatusTimeout
		}
	}

	// Connection refused
	if contains(errStr, "connection refused") {
		return StatusRefused
	}

	// Network unreachable
	if contains(errStr, "network is unreachable") || contains(errStr, "no route to host") {
		return StatusUnreachable
	}

	// TLS errors
	if contains(errStr, "tls") || contains(errStr, "certificate") || contains(errStr, "x509") {
		return StatusTLSError
	}

	// Authentication errors
	if contains(errStr, "authentication") || contains(errStr, "unauthorized") || contains(errStr, "unauthenticated") {
		return StatusAuthFailed
	}

	return StatusDisconnected
}

// fingerprintCert returns the SHA256 fingerprint of a certificate
func fingerprintCert(der []byte) string {
	hash := sha256.Sum256(der)
	return fmt.Sprintf("SHA256:%x", hash)
}

// contains checks if s contains substr (case-insensitive)
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// FormatConnectionResult formats a connection result for display
func FormatConnectionResult(result *ConnectionTestResult) string {
	var sb strings.Builder

	// Status icon
	var statusIcon string
	switch result.Status {
	case StatusConnected:
		statusIcon = "✓"
	case StatusDisconnected, StatusRefused, StatusUnreachable:
		statusIcon = "✗"
	case StatusTimeout:
		statusIcon = "⏱"
	case StatusTLSError:
		statusIcon = "🔒"
	case StatusAuthFailed:
		statusIcon = "🔑"
	default:
		statusIcon = "?"
	}

	sb.WriteString(fmt.Sprintf("%s %s", statusIcon, result.Address))

	if result.Status == StatusConnected {
		sb.WriteString(fmt.Sprintf(" (%s)", result.Latency.Round(time.Millisecond)))
		if result.NodeInfo != nil {
			if result.NodeInfo.Version != "" {
				sb.WriteString(fmt.Sprintf(" v%s", result.NodeInfo.Version))
			}
			if result.NodeInfo.Mode != "" {
				sb.WriteString(fmt.Sprintf(" [%s]", result.NodeInfo.Mode))
			}
		}
		if result.TLSInfo != nil && result.TLSInfo.Enabled {
			sb.WriteString(" 🔒")
		}
	} else {
		sb.WriteString(fmt.Sprintf(" - %s", result.Status))
		if result.Error != "" {
			sb.WriteString(fmt.Sprintf(": %s", result.Error))
		}
	}

	return sb.String()
}

// FormatConnectionResults formats multiple connection results for display
func FormatConnectionResults(results []*ConnectionTestResult) string {
	var sb strings.Builder

	connected := 0
	failed := 0

	for _, r := range results {
		if r.Status == StatusConnected {
			connected++
		} else {
			failed++
		}
	}

	sb.WriteString(fmt.Sprintf("Connection Test Results: %d connected, %d failed\n\n", connected, failed))

	for _, r := range results {
		sb.WriteString(FormatConnectionResult(r))
		sb.WriteString("\n")
	}

	return sb.String()
}
