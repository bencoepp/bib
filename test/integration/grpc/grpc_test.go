//go:build integration

// Package grpc_test contains integration tests for gRPC services.
package grpc_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	services "bib/api/gen/go/bib/v1/services"
	"bib/internal/cluster"
	bibgrpc "bib/internal/grpc"
	"bib/internal/p2p"
	"bib/internal/storage"
	"bib/test/testutil"

	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// testServer holds a running test gRPC server.
type testServer struct {
	server   *grpc.Server
	listener net.Listener
	address  string
	services *bibgrpc.ServiceServers
}

// newTestServer creates and starts a test gRPC server.
func newTestServer(t *testing.T) *testServer {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	serviceServers := bibgrpc.NewServiceServers()

	// Configure basic services (health service works without store)
	serviceServers.Health.SetProvider(&testHealthProvider{running: true})

	// Create gRPC server
	server := grpc.NewServer()
	serviceServers.Register(server)

	// Start server in background
	go func() {
		if err := server.Serve(listener); err != nil {
			// Ignore "use of closed network connection" on shutdown
			if !isClosedConnError(err) {
				t.Logf("server error: %v", err)
			}
		}
	}()

	return &testServer{
		server:   server,
		listener: listener,
		address:  listener.Addr().String(),
		services: serviceServers,
	}
}

// close stops the test server.
func (ts *testServer) close() {
	ts.server.GracefulStop()
	ts.listener.Close()
}

// dial creates a client connection to the test server.
func (ts *testServer) dial(t *testing.T) *grpc.ClientConn {
	t.Helper()

	conn, err := grpc.Dial(
		ts.address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("failed to dial server: %v", err)
	}

	t.Cleanup(func() { conn.Close() })
	return conn
}

// =============================================================================
// Integration Tests
// =============================================================================

// TestGRPCIntegration_HealthService tests the health service end-to-end.
func TestGRPCIntegration_HealthService(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := newTestServer(t)
	defer ts.close()

	conn := ts.dial(t)
	client := services.NewHealthServiceClient(conn)

	t.Run("Ping", func(t *testing.T) {
		resp, err := client.Ping(ctx, &services.PingRequest{
			Payload: []byte("hello"),
		})
		if err != nil {
			t.Fatalf("Ping failed: %v", err)
		}
		if string(resp.Payload) != "hello" {
			t.Errorf("expected payload 'hello', got '%s'", string(resp.Payload))
		}
		if resp.Timestamp == nil {
			t.Error("expected timestamp")
		}
	})

	t.Run("Check", func(t *testing.T) {
		resp, err := client.Check(ctx, &services.HealthCheckRequest{})
		if err != nil {
			t.Fatalf("Check failed: %v", err)
		}
		t.Logf("Health status: %v", resp.Status)
	})
}

// TestGRPCIntegration_AuthChallenge tests the auth challenge endpoint.
func TestGRPCIntegration_AuthChallenge(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := newTestServer(t)
	defer ts.close()

	conn := ts.dial(t)
	authClient := services.NewAuthServiceClient(conn)

	// Generate test key pair
	pubKey, _ := generateTestKey(t)

	// Request challenge
	challengeResp, err := authClient.Challenge(ctx, &services.ChallengeRequest{
		PublicKey: pubKey,
		KeyType:   "ed25519",
	})
	if err != nil {
		t.Fatalf("Challenge failed: %v", err)
	}

	if challengeResp.ChallengeId == "" {
		t.Fatal("expected challenge ID")
	}
	if len(challengeResp.Challenge) == 0 {
		t.Fatal("expected challenge bytes")
	}
	if challengeResp.ExpiresAt == nil {
		t.Fatal("expected expiration time")
	}

	t.Logf("Received challenge: %s (expires: %v)",
		challengeResp.ChallengeId, challengeResp.ExpiresAt.AsTime())
}

// TestGRPCIntegration_AuthConfig tests getting auth configuration.
func TestGRPCIntegration_AuthConfig(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := newTestServer(t)
	defer ts.close()

	conn := ts.dial(t)
	authClient := services.NewAuthServiceClient(conn)

	// Get auth config
	configResp, err := authClient.GetAuthConfig(ctx, &services.GetAuthConfigRequest{})
	if err != nil {
		t.Fatalf("GetAuthConfig failed: %v", err)
	}

	t.Logf("Auth config: auto_reg=%v, key_types=%v",
		configResp.AllowAutoRegistration, configResp.SupportedKeyTypes)
}

// TestGRPCIntegration_ConcurrentRequests tests concurrent gRPC requests.
func TestGRPCIntegration_ConcurrentRequests(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := newTestServer(t)
	defer ts.close()

	conn := ts.dial(t)
	healthClient := services.NewHealthServiceClient(conn)

	const numRequests = 50
	errors := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			_, err := healthClient.Ping(ctx, &services.PingRequest{
				Payload: []byte(fmt.Sprintf("request-%d", id)),
			})
			errors <- err
		}(i)
	}

	successCount := 0
	for i := 0; i < numRequests; i++ {
		err := <-errors
		if err == nil {
			successCount++
		} else {
			t.Logf("Request failed: %v", err)
		}
	}

	if successCount < numRequests {
		t.Errorf("expected %d successful requests, got %d", numRequests, successCount)
	}
}

// TestGRPCIntegration_ErrorHandling tests error responses.
func TestGRPCIntegration_ErrorHandling(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := newTestServer(t)
	defer ts.close()

	conn := ts.dial(t)
	authClient := services.NewAuthServiceClient(conn)

	t.Run("InvalidPublicKey", func(t *testing.T) {
		_, err := authClient.Challenge(ctx, &services.ChallengeRequest{
			PublicKey: []byte("invalid-key"),
		})
		if err == nil {
			t.Fatal("expected error for invalid public key")
		}
		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("expected gRPC status, got %v", err)
		}
		if st.Code() != codes.InvalidArgument {
			t.Errorf("expected InvalidArgument, got %v", st.Code())
		}
	})

	t.Run("ExpiredChallenge", func(t *testing.T) {
		_, err := authClient.VerifyChallenge(ctx, &services.VerifyChallengeRequest{
			ChallengeId: "non-existent-challenge",
			Signature:   []byte("signature"),
		})
		if err == nil {
			t.Fatal("expected error for expired challenge")
		}
		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("expected gRPC status, got %v", err)
		}
		// May return Unavailable if auth service not fully initialized
		// or NotFound if challenge doesn't exist
		if st.Code() != codes.NotFound && st.Code() != codes.Unavailable {
			t.Errorf("expected NotFound or Unavailable, got %v", st.Code())
		}
	})
}

// TestGRPCIntegration_Streaming tests streaming endpoints.
func TestGRPCIntegration_Streaming(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := newTestServer(t)
	defer ts.close()

	conn := ts.dial(t)
	healthClient := services.NewHealthServiceClient(conn)

	// Create cancellable context
	streamCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	stream, err := healthClient.Watch(streamCtx, &services.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	// Receive at least one message
	resp, err := stream.Recv()
	if err != nil {
		t.Fatalf("Recv failed: %v", err)
	}

	t.Logf("Received health status: %v", resp.Status)
}

// =============================================================================
// Test Helpers
// =============================================================================

// testHealthProvider implements HealthProvider for tests.
type testHealthProvider struct {
	running bool
}

func (p *testHealthProvider) IsRunning() bool           { return p.running }
func (p *testHealthProvider) StartedAt() time.Time      { return time.Now().Add(-time.Hour) }
func (p *testHealthProvider) NodeMode() string          { return "full" }
func (p *testHealthProvider) NodeID() string            { return "test-node" }
func (p *testHealthProvider) ListenAddresses() []string { return []string{"127.0.0.1:0"} }
func (p *testHealthProvider) HealthConfig() bibgrpc.HealthProviderConfig {
	return bibgrpc.HealthProviderConfig{}
}
func (p *testHealthProvider) Store() storage.Store         { return nil }
func (p *testHealthProvider) P2PHost() *p2p.Host           { return nil }
func (p *testHealthProvider) P2PDiscovery() *p2p.Discovery { return nil }
func (p *testHealthProvider) Cluster() *cluster.Cluster    { return nil }

// generateTestKey generates a test Ed25519 SSH key pair.
func generateTestKey(t *testing.T) (pubKeyBytes []byte, privKey ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("failed to create SSH public key: %v", err)
	}
	return ssh.MarshalAuthorizedKey(sshPub), priv
}

// signChallenge signs a challenge with an Ed25519 private key.
func signChallenge(t *testing.T, privKey ed25519.PrivateKey, challenge []byte) []byte {
	t.Helper()
	signer, err := ssh.NewSignerFromKey(privKey)
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}
	sig, err := signer.Sign(rand.Reader, challenge)
	if err != nil {
		t.Fatalf("failed to sign: %v", err)
	}
	return sig.Blob
}

// isClosedConnError checks if the error is due to closed connection.
func isClosedConnError(err error) bool {
	if err == nil {
		return false
	}
	return os.IsTimeout(err) || err.Error() == "use of closed network connection"
}
