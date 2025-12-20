//go:build integration

// Package grpc_test contains comprehensive integration tests for all gRPC services.
package grpc_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"net"
	"testing"
	"time"

	services "bib/api/gen/go/bib/v1/services"
	"bib/internal/auth"
	"bib/internal/cluster"
	"bib/internal/config"
	"bib/internal/domain"
	bibgrpc "bib/internal/grpc"
	"bib/internal/p2p"
	"bib/internal/storage"
	"bib/internal/storage/postgres"
	"bib/test/testutil"
	"bib/test/testutil/containers"

	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// =============================================================================
// Test Infrastructure
// =============================================================================

// TestServer holds a complete test gRPC server with real dependencies.
type TestServer struct {
	Server      *grpc.Server
	Listener    net.Listener
	Address     string
	Store       storage.Store
	Services    *bibgrpc.ServiceServers
	AuthService *auth.Service
	Container   *containers.Container

	t  testing.TB
	cm *containers.Manager
}

// NewTestServer creates a fully initialized test server with PostgreSQL.
func NewTestServer(t testing.TB, ctx context.Context) *TestServer {
	t.Helper()

	// Start PostgreSQL container
	cm := containers.NewManager(t)
	pgCfg := containers.DefaultPostgresConfig()
	pgContainer, err := cm.StartPostgres(ctx, pgCfg)
	if err != nil {
		t.Fatalf("failed to start postgres: %v", err)
	}

	// Create store
	dataDir := testutil.TempDir(t, "grpc-integration")
	storeCfg := storage.PostgresConfig{
		Managed: false,
		Advanced: &storage.AdvancedPostgresConfig{
			Host:     "localhost",
			Port:     pgContainer.HostPort(5432),
			Database: pgCfg.Database,
			User:     pgCfg.User,
			Password: pgCfg.Password,
			SSLMode:  "disable",
		},
		MaxConnections: 10,
	}

	store, err := postgres.New(ctx, storeCfg, dataDir, "test-node")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Run migrations using the new migration framework
	migrateCfg := storage.MigrationsConfig{
		VerifyChecksums:    false, // Skip checksum verification for tests
		OnChecksumMismatch: "warn",
		LockTimeoutSeconds: 30,
	}
	if err := storage.RunMigrations(ctx, store, migrateCfg); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// Create auth service
	authCfg := config.AuthConfig{
		AllowAutoRegistration: true,
		RequireEmail:          false,
		DefaultRole:           "user",
		SessionTimeout:        24 * time.Hour,
	}
	authSvc := auth.NewService(store, authCfg, "test-node")

	// Create gRPC server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	// Create service servers with dependencies
	serviceServers := bibgrpc.NewServiceServers()

	// Configure all services with dependencies
	deps := bibgrpc.ServiceDependencies{
		Store:       store,
		AuthService: authSvc,
		AuthConfig:  authCfg,
		NodeID:      "test-node",
		NodeMode:    "full",
		Version:     "1.0.0-test",
		StartedAt:   time.Now(),
		Config:      map[string]interface{}{"test": true},
	}
	serviceServers.ConfigureServices(deps)

	// Configure health provider
	serviceServers.SetHealthProvider(&fullHealthProvider{
		store:   store,
		running: true,
	})

	// Create a function to get user from session token
	getUserFromToken := func(ctx context.Context, token string) (*domain.User, error) {
		session, err := store.Sessions().Get(ctx, token)
		if err != nil {
			return nil, err
		}
		// Check if session is still valid (not ended)
		if session.EndedAt != nil {
			return nil, fmt.Errorf("session ended")
		}
		user, err := store.Users().Get(ctx, session.UserID)
		if err != nil {
			return nil, err
		}
		return user, nil
	}

	// RBAC config - enable auth checks
	rbacCfg := bibgrpc.RBACConfig{
		Enabled:       true,
		BootstrapMode: false,
	}

	// Create gRPC server with interceptors
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			bibgrpc.RequestIDUnaryInterceptor(),
			bibgrpc.RBACInterceptor(rbacCfg, getUserFromToken),
			bibgrpc.RecoveryUnaryInterceptor(),
		),
		grpc.ChainStreamInterceptor(
			bibgrpc.RequestIDStreamInterceptor(),
			bibgrpc.RBACStreamInterceptor(rbacCfg, getUserFromToken),
			bibgrpc.RecoveryStreamInterceptor(),
		),
	)
	serviceServers.Register(server)

	// Start server
	go func() {
		if err := server.Serve(listener); err != nil {
			// Ignore closed listener error on shutdown
		}
	}()

	ts := &TestServer{
		Server:      server,
		Listener:    listener,
		Address:     listener.Addr().String(),
		Store:       store,
		Services:    serviceServers,
		AuthService: authSvc,
		Container:   pgContainer,
		t:           t,
		cm:          cm,
	}

	t.Cleanup(ts.Close)
	return ts
}

// Close stops the test server and cleans up resources.
func (ts *TestServer) Close() {
	ts.Server.GracefulStop()
	ts.Listener.Close()
	ts.Store.Close()
}

// Dial creates a client connection to the test server.
func (ts *TestServer) Dial() *grpc.ClientConn {
	conn, err := grpc.Dial(
		ts.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		ts.t.Fatalf("failed to dial server: %v", err)
	}
	ts.t.Cleanup(func() { conn.Close() })
	return conn
}

// AuthenticateUser creates a user and returns an authenticated context.
func (ts *TestServer) AuthenticateUser(ctx context.Context, name string) (context.Context, *domain.User, string) {
	conn := ts.Dial()
	authClient := services.NewAuthServiceClient(conn)

	// Generate key pair
	pubKey, privKey := generateTestKeyPair(ts.t)

	// Get challenge
	challengeResp, err := authClient.Challenge(ctx, &services.ChallengeRequest{
		PublicKey: pubKey,
	})
	if err != nil {
		ts.t.Fatalf("Challenge failed: %v", err)
	}

	// Sign challenge
	signature := signChallengeBytes(ts.t, privKey, challengeResp.Challenge)

	// Verify challenge
	verifyResp, err := authClient.VerifyChallenge(ctx, &services.VerifyChallengeRequest{
		ChallengeId: challengeResp.ChallengeId,
		Signature:   signature,
		Name:        name,
		Email:       name + "@test.com",
	})
	if err != nil {
		ts.t.Fatalf("VerifyChallenge failed: %v", err)
	}

	// Create authenticated context
	authCtx := metadata.AppendToOutgoingContext(ctx, "x-session-token", verifyResp.SessionToken)

	user := &domain.User{
		ID:   domain.UserID(verifyResp.User.Id),
		Name: verifyResp.User.Name,
	}

	return authCtx, user, verifyResp.SessionToken
}

// CreateAdminUser creates an admin user in the database.
func (ts *TestServer) CreateAdminUser(ctx context.Context, name string) (context.Context, *domain.User, string) {
	authCtx, user, token := ts.AuthenticateUser(ctx, name)

	// Get the full user from the database first
	fullUser, err := ts.Store.Users().Get(ctx, user.ID)
	if err != nil {
		ts.t.Fatalf("failed to get user: %v", err)
	}

	// Upgrade to admin
	fullUser.Role = domain.UserRoleAdmin
	fullUser.Status = domain.UserStatusActive
	if err := ts.Store.Users().Update(ctx, fullUser); err != nil {
		ts.t.Fatalf("failed to upgrade user to admin: %v", err)
	}

	user.Role = domain.UserRoleAdmin
	return authCtx, user, token
}

// =============================================================================
// Helper Types
// =============================================================================

// fullHealthProvider implements HealthProvider with store.
type fullHealthProvider struct {
	store   storage.Store
	running bool
}

func (p *fullHealthProvider) IsRunning() bool           { return p.running }
func (p *fullHealthProvider) StartedAt() time.Time      { return time.Now().Add(-time.Hour) }
func (p *fullHealthProvider) NodeMode() string          { return "full" }
func (p *fullHealthProvider) NodeID() string            { return "test-node" }
func (p *fullHealthProvider) ListenAddresses() []string { return []string{"127.0.0.1:0"} }
func (p *fullHealthProvider) HealthConfig() bibgrpc.HealthProviderConfig {
	return bibgrpc.HealthProviderConfig{}
}
func (p *fullHealthProvider) Store() storage.Store         { return p.store }
func (p *fullHealthProvider) P2PHost() *p2p.Host           { return nil }
func (p *fullHealthProvider) P2PDiscovery() *p2p.Discovery { return nil }
func (p *fullHealthProvider) Cluster() *cluster.Cluster    { return nil }

// =============================================================================
// Key Generation Helpers
// =============================================================================

func generateTestKeyPair(t testing.TB) (pubKeyBytes []byte, privKey ed25519.PrivateKey) {
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

func signChallengeBytes(t testing.TB, privKey ed25519.PrivateKey, challenge []byte) []byte {
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

// =============================================================================
// Assertion Helpers
// =============================================================================

func assertGRPCCode(t testing.TB, err error, expected codes.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %v, got nil", expected)
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != expected {
		t.Errorf("expected code %v, got %v: %s", expected, st.Code(), st.Message())
	}
}

func assertNoError(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
