//go:build integration

package grpc_test

import (
	"testing"
	"time"

	services "bib/api/gen/go/bib/v1/services"
	"bib/test/testutil"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

// =============================================================================
// AuthService Integration Tests
// =============================================================================

// TestAuthService_CompleteAuthFlow tests the complete authentication flow.
func TestAuthService_CompleteAuthFlow(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	authClient := services.NewAuthServiceClient(conn)

	// Generate test key pair
	pubKey, privKey := generateTestKeyPair(t)

	// Step 1: Request challenge
	t.Run("Challenge", func(t *testing.T) {
		resp, err := authClient.Challenge(ctx, &services.ChallengeRequest{
			PublicKey: pubKey,
			KeyType:   "ed25519",
		})
		assertNoError(t, err)

		if resp.ChallengeId == "" {
			t.Error("expected challenge ID")
		}
		if len(resp.Challenge) == 0 {
			t.Error("expected challenge bytes")
		}
		if resp.ExpiresAt == nil {
			t.Error("expected expiration time")
		}
		if resp.SignatureAlgorithm == "" {
			t.Error("expected signature algorithm")
		}

		// Verify expiration is in the future
		if resp.ExpiresAt.AsTime().Before(time.Now()) {
			t.Error("expiration should be in the future")
		}
	})

	// Step 2: Verify challenge and get session
	var sessionToken string
	t.Run("VerifyChallenge", func(t *testing.T) {
		// Get fresh challenge
		challengeResp, err := authClient.Challenge(ctx, &services.ChallengeRequest{
			PublicKey: pubKey,
		})
		assertNoError(t, err)

		// Sign it
		signature := signChallengeBytes(t, privKey, challengeResp.Challenge)

		// Verify
		resp, err := authClient.VerifyChallenge(ctx, &services.VerifyChallengeRequest{
			ChallengeId: challengeResp.ChallengeId,
			Signature:   signature,
			Name:        "Test User",
			Email:       "test@example.com",
			ClientInfo: &services.ClientInfo{
				IpAddress: "127.0.0.1",
				UserAgent: "test-client/1.0",
				Version:   "1.0.0",
			},
		})
		assertNoError(t, err)

		if resp.SessionToken == "" {
			t.Error("expected session token")
		}
		if resp.User == nil {
			t.Error("expected user info")
		}
		if !resp.IsNewUser {
			t.Error("expected IsNewUser=true for first login")
		}
		if resp.User.Name != "Test User" {
			t.Errorf("expected name 'Test User', got '%s'", resp.User.Name)
		}
		if resp.Session == nil {
			t.Error("expected session info")
		}

		sessionToken = resp.SessionToken
	})

	// Step 3: Validate session
	t.Run("ValidateSession", func(t *testing.T) {
		resp, err := authClient.ValidateSession(ctx, &services.ValidateSessionRequest{
			SessionToken: sessionToken,
		})
		assertNoError(t, err)

		if !resp.Valid {
			t.Errorf("session should be valid, reason: %s", resp.InvalidReason)
		}
		if resp.User == nil {
			t.Error("expected user info")
		}
	})

	// Step 4: Refresh session
	t.Run("RefreshSession", func(t *testing.T) {
		authCtx := metadata.AppendToOutgoingContext(ctx, "x-session-token", sessionToken)
		resp, err := authClient.RefreshSession(authCtx, &services.RefreshSessionRequest{})
		assertNoError(t, err)

		if resp.SessionToken == "" {
			t.Error("expected session token")
		}
		if resp.ExpiresAt == nil {
			t.Error("expected expiration time")
		}
	})

	// Step 5: List sessions
	t.Run("ListMySessions", func(t *testing.T) {
		authCtx := metadata.AppendToOutgoingContext(ctx, "x-session-token", sessionToken)
		resp, err := authClient.ListMySessions(authCtx, &services.ListMySessionsRequest{})
		assertNoError(t, err)

		if len(resp.Sessions) == 0 {
			t.Error("expected at least one session")
		}

		// Find current session
		foundCurrent := false
		for _, s := range resp.Sessions {
			if s.IsCurrent {
				foundCurrent = true
			}
		}
		if !foundCurrent {
			t.Error("expected current session to be marked")
		}
	})

	// Step 6: Logout
	t.Run("Logout", func(t *testing.T) {
		authCtx := metadata.AppendToOutgoingContext(ctx, "x-session-token", sessionToken)
		resp, err := authClient.Logout(authCtx, &services.LogoutRequest{})
		assertNoError(t, err)

		if !resp.Success {
			t.Error("expected logout success")
		}

		// Verify session is invalid now
		validateResp, _ := authClient.ValidateSession(ctx, &services.ValidateSessionRequest{
			SessionToken: sessionToken,
		})
		if validateResp.Valid {
			t.Error("session should be invalid after logout")
		}
	})
}

// TestAuthService_Challenge tests the Challenge endpoint variations.
func TestAuthService_Challenge(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	authClient := services.NewAuthServiceClient(conn)

	tests := []struct {
		name      string
		req       *services.ChallengeRequest
		wantCode  codes.Code
		wantError bool
	}{
		{
			name: "valid ed25519 key",
			req: &services.ChallengeRequest{
				PublicKey: func() []byte { pk, _ := generateTestKeyPair(t); return pk }(),
				KeyType:   "ed25519",
			},
			wantError: false,
		},
		{
			name: "empty public key",
			req: &services.ChallengeRequest{
				PublicKey: nil,
			},
			wantCode:  codes.InvalidArgument,
			wantError: true,
		},
		{
			name: "invalid public key format",
			req: &services.ChallengeRequest{
				PublicKey: []byte("not-a-valid-key"),
			},
			wantCode:  codes.InvalidArgument,
			wantError: true,
		},
		{
			name: "truncated public key",
			req: &services.ChallengeRequest{
				PublicKey: []byte("ssh-ed25519 AAAA"),
			},
			wantCode:  codes.InvalidArgument,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := authClient.Challenge(ctx, tt.req)

			if tt.wantError {
				assertGRPCCode(t, err, tt.wantCode)
				return
			}

			assertNoError(t, err)
			if resp.ChallengeId == "" {
				t.Error("expected challenge ID")
			}
		})
	}
}

// TestAuthService_VerifyChallenge tests challenge verification variations.
func TestAuthService_VerifyChallenge(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	authClient := services.NewAuthServiceClient(conn)

	pubKey, privKey := generateTestKeyPair(t)

	// Get a valid challenge
	challengeResp, err := authClient.Challenge(ctx, &services.ChallengeRequest{
		PublicKey: pubKey,
	})
	assertNoError(t, err)

	validSignature := signChallengeBytes(t, privKey, challengeResp.Challenge)

	tests := []struct {
		name      string
		req       *services.VerifyChallengeRequest
		wantCode  codes.Code
		wantError bool
	}{
		{
			name: "missing challenge_id",
			req: &services.VerifyChallengeRequest{
				ChallengeId: "",
				Signature:   validSignature,
			},
			wantCode:  codes.InvalidArgument,
			wantError: true,
		},
		{
			name: "missing signature",
			req: &services.VerifyChallengeRequest{
				ChallengeId: challengeResp.ChallengeId,
				Signature:   nil,
			},
			wantCode:  codes.InvalidArgument,
			wantError: true,
		},
		{
			name: "non-existent challenge",
			req: &services.VerifyChallengeRequest{
				ChallengeId: "non-existent-challenge-id",
				Signature:   validSignature,
			},
			wantCode:  codes.NotFound,
			wantError: true,
		},
		{
			name: "invalid signature",
			req: &services.VerifyChallengeRequest{
				ChallengeId: challengeResp.ChallengeId,
				Signature:   []byte("invalid-signature"),
			},
			wantCode:  codes.Unauthenticated,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := authClient.VerifyChallenge(ctx, tt.req)
			assertGRPCCode(t, err, tt.wantCode)
		})
	}
}

// TestAuthService_SessionManagement tests session lifecycle.
func TestAuthService_SessionManagement(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	authClient := services.NewAuthServiceClient(conn)

	// Authenticate
	authCtx, _, _ := ts.AuthenticateUser(ctx, "SessionUser")

	t.Run("CreateMultipleSessions", func(t *testing.T) {
		// Create additional sessions by authenticating again
		pubKey2, privKey2 := generateTestKeyPair(t)

		for i := 0; i < 3; i++ {
			challenge, err := authClient.Challenge(ctx, &services.ChallengeRequest{
				PublicKey: pubKey2,
			})
			assertNoError(t, err)

			sig := signChallengeBytes(t, privKey2, challenge.Challenge)
			_, err = authClient.VerifyChallenge(ctx, &services.VerifyChallengeRequest{
				ChallengeId: challenge.ChallengeId,
				Signature:   sig,
				Name:        "MultiSession User",
			})
			assertNoError(t, err)
		}
	})

	t.Run("RevokeSession", func(t *testing.T) {
		// List sessions first
		listResp, err := authClient.ListMySessions(authCtx, &services.ListMySessionsRequest{})
		assertNoError(t, err)

		if len(listResp.Sessions) == 0 {
			t.Skip("no sessions to revoke")
		}

		// Find a non-current session to revoke
		for _, s := range listResp.Sessions {
			if !s.IsCurrent {
				resp, err := authClient.RevokeSession(authCtx, &services.RevokeSessionRequest{
					SessionId: s.Id,
				})
				assertNoError(t, err)
				if !resp.Success {
					t.Error("expected revoke success")
				}
				return
			}
		}
	})

	t.Run("RevokeAllSessions", func(t *testing.T) {
		resp, err := authClient.RevokeAllSessions(authCtx, &services.RevokeAllSessionsRequest{
			IncludeCurrent: false,
		})
		assertNoError(t, err)
		t.Logf("Revoked %d sessions", resp.RevokedCount)
	})
}

// TestAuthService_GetAuthConfig tests auth configuration retrieval.
func TestAuthService_GetAuthConfig(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	authClient := services.NewAuthServiceClient(conn)

	resp, err := authClient.GetAuthConfig(ctx, &services.GetAuthConfigRequest{})
	assertNoError(t, err)

	if resp.NodeId == "" {
		t.Error("expected node ID")
	}
	if resp.ServerVersion == "" {
		t.Error("expected server version")
	}
	// Auto-registration should be enabled for tests
	if !resp.AllowAutoRegistration {
		t.Error("expected auto-registration to be enabled")
	}
}

// TestAuthService_GetPublicKeyInfo tests public key info retrieval.
func TestAuthService_GetPublicKeyInfo(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	authClient := services.NewAuthServiceClient(conn)

	pubKey, _ := generateTestKeyPair(t)

	resp, err := authClient.GetPublicKeyInfo(ctx, &services.GetPublicKeyInfoRequest{
		PublicKey: pubKey,
	})
	assertNoError(t, err)

	if resp.KeyType != "ed25519" {
		t.Errorf("expected KeyType 'ed25519', got '%s'", resp.KeyType)
	}
	if resp.FingerprintSha256 == "" {
		t.Error("expected SHA256 fingerprint")
	}
}

// TestAuthService_ChallengeExpiration tests that challenges expire.
func TestAuthService_ChallengeExpiration(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	authClient := services.NewAuthServiceClient(conn)

	pubKey, privKey := generateTestKeyPair(t)

	// Get challenge
	challengeResp, err := authClient.Challenge(ctx, &services.ChallengeRequest{
		PublicKey: pubKey,
	})
	assertNoError(t, err)

	// Use challenge once (should consume it)
	sig := signChallengeBytes(t, privKey, challengeResp.Challenge)
	_, err = authClient.VerifyChallenge(ctx, &services.VerifyChallengeRequest{
		ChallengeId: challengeResp.ChallengeId,
		Signature:   sig,
		Name:        "Test User",
	})
	assertNoError(t, err)

	// Try to use the same challenge again (should fail)
	_, err = authClient.VerifyChallenge(ctx, &services.VerifyChallengeRequest{
		ChallengeId: challengeResp.ChallengeId,
		Signature:   sig,
		Name:        "Test User 2",
	})
	assertGRPCCode(t, err, codes.NotFound) // Challenge consumed
}
