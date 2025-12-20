package grpc

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	services "bib/api/gen/go/bib/v1/services"
	"bib/internal/config"

	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// TestAuthServiceServer_Challenge tests the Challenge endpoint.
func TestAuthServiceServer_Challenge(t *testing.T) {
	server := NewAuthServiceServer()

	// Generate a test Ed25519 key
	pubKey, _, err := generateTestEd25519Key()
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}

	tests := []struct {
		name      string
		req       *services.ChallengeRequest
		wantCode  codes.Code
		wantError bool
	}{
		{
			name: "valid ed25519 key",
			req: &services.ChallengeRequest{
				PublicKey: pubKey,
				KeyType:   "ed25519",
			},
			wantCode:  codes.OK,
			wantError: false,
		},
		{
			name: "empty public key",
			req: &services.ChallengeRequest{
				PublicKey: nil,
				KeyType:   "ed25519",
			},
			wantCode:  codes.InvalidArgument,
			wantError: true,
		},
		{
			name: "invalid public key",
			req: &services.ChallengeRequest{
				PublicKey: []byte("not-a-valid-key"),
				KeyType:   "ed25519",
			},
			wantCode:  codes.InvalidArgument,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := server.Challenge(context.Background(), tt.req)

			if tt.wantError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				st, ok := status.FromError(err)
				if !ok {
					t.Fatalf("expected gRPC status error, got %v", err)
				}
				if st.Code() != tt.wantCode {
					t.Errorf("expected code %v, got %v", tt.wantCode, st.Code())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.ChallengeId == "" {
				t.Error("expected challenge ID")
			}
			if len(resp.Challenge) == 0 {
				t.Error("expected challenge bytes")
			}
			if resp.ExpiresAt == nil {
				t.Error("expected expires_at")
			}
			if resp.SignatureAlgorithm == "" {
				t.Error("expected signature algorithm")
			}
		})
	}
}

// TestAuthServiceServer_GetAuthConfig tests the GetAuthConfig endpoint.
func TestAuthServiceServer_GetAuthConfig(t *testing.T) {
	server := NewAuthServiceServerWithConfig(AuthServiceConfig{
		AuthConfig: config.AuthConfig{
			AllowAutoRegistration: true,
			RequireEmail:          true,
			DefaultRole:           "user",
			SessionTimeout:        2 * time.Hour,
		},
		NodeID:   "node-123",
		NodeMode: "full",
		Version:  "1.2.3",
	})

	resp, err := server.GetAuthConfig(context.Background(), &services.GetAuthConfigRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.AllowAutoRegistration {
		t.Error("expected AllowAutoRegistration=true")
	}
	if !resp.RequireEmail {
		t.Error("expected RequireEmail=true")
	}
	if resp.DefaultRole != "user" {
		t.Errorf("expected DefaultRole='user', got '%s'", resp.DefaultRole)
	}
	if resp.SessionTimeoutSeconds != 7200 {
		t.Errorf("expected SessionTimeoutSeconds=7200, got %d", resp.SessionTimeoutSeconds)
	}
	if resp.NodeId != "node-123" {
		t.Errorf("expected NodeId='node-123', got '%s'", resp.NodeId)
	}
	if resp.NodeMode != "full" {
		t.Errorf("expected NodeMode='full', got '%s'", resp.NodeMode)
	}
	if resp.ServerVersion != "1.2.3" {
		t.Errorf("expected ServerVersion='1.2.3', got '%s'", resp.ServerVersion)
	}
}

// TestAuthServiceServer_GetPublicKeyInfo tests the GetPublicKeyInfo endpoint.
func TestAuthServiceServer_GetPublicKeyInfo(t *testing.T) {
	server := NewAuthServiceServer()

	pubKey, _, err := generateTestEd25519Key()
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}

	tests := []struct {
		name      string
		req       *services.GetPublicKeyInfoRequest
		wantCode  codes.Code
		wantError bool
	}{
		{
			name: "valid key",
			req: &services.GetPublicKeyInfoRequest{
				PublicKey: pubKey,
			},
			wantCode:  codes.OK,
			wantError: false,
		},
		{
			name: "empty key",
			req: &services.GetPublicKeyInfoRequest{
				PublicKey: nil,
			},
			wantCode:  codes.InvalidArgument,
			wantError: true,
		},
		{
			name: "invalid key",
			req: &services.GetPublicKeyInfoRequest{
				PublicKey: []byte("invalid"),
			},
			wantCode:  codes.InvalidArgument,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := server.GetPublicKeyInfo(context.Background(), tt.req)

			if tt.wantError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				st, ok := status.FromError(err)
				if !ok {
					t.Fatalf("expected gRPC status error, got %v", err)
				}
				if st.Code() != tt.wantCode {
					t.Errorf("expected code %v, got %v", tt.wantCode, st.Code())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.KeyType != "ed25519" {
				t.Errorf("expected KeyType='ed25519', got '%s'", resp.KeyType)
			}
			if resp.FingerprintSha256 == "" {
				t.Error("expected fingerprint")
			}
		})
	}
}

// TestAuthServiceServer_ValidateSession_EmptyToken tests validation with empty token.
func TestAuthServiceServer_ValidateSession_EmptyToken(t *testing.T) {
	server := NewAuthServiceServer()

	resp, err := server.ValidateSession(context.Background(), &services.ValidateSessionRequest{
		SessionToken: "",
	})

	// When authService is not initialized, may return Unavailable or handle gracefully
	if err != nil {
		// This is acceptable - service not initialized
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.Unavailable {
			t.Log("Service not initialized, returned Unavailable (expected)")
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Valid {
		t.Error("expected Valid=false for empty token")
	}
	if resp.InvalidReason == "" {
		t.Error("expected invalid reason")
	}
}

// TestAuthServiceServer_NotInitialized tests service when not properly initialized.
func TestAuthServiceServer_NotInitialized(t *testing.T) {
	server := NewAuthServiceServer()

	// VerifyChallenge should fail when authService is nil
	_, err := server.VerifyChallenge(context.Background(), &services.VerifyChallengeRequest{
		ChallengeId: "test",
		Signature:   []byte("test"),
	})
	if err == nil {
		t.Fatal("expected error for uninitialized service")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Unavailable {
		t.Errorf("expected Unavailable, got %v", st.Code())
	}

	// Logout should fail when authService is nil
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		sessionTokenHeader, "test-token",
	))
	_, err = server.Logout(ctx, &services.LogoutRequest{})
	if err == nil {
		t.Fatal("expected error for uninitialized service")
	}
	st, _ = status.FromError(err)
	if st.Code() != codes.Unavailable {
		t.Errorf("expected Unavailable, got %v", st.Code())
	}
}

// TestExtractSessionToken tests session token extraction from context.
func TestExtractSessionToken(t *testing.T) {
	tests := []struct {
		name      string
		metadata  metadata.MD
		wantToken string
		wantError bool
	}{
		{
			name:      "x-session-token header",
			metadata:  metadata.Pairs(sessionTokenHeader, "token-123"),
			wantToken: "token-123",
			wantError: false,
		},
		{
			name:      "authorization header with Bearer",
			metadata:  metadata.Pairs("authorization", "Bearer token-456"),
			wantToken: "token-456",
			wantError: false,
		},
		{
			name:      "authorization header without Bearer",
			metadata:  metadata.Pairs("authorization", "token-789"),
			wantToken: "token-789",
			wantError: false,
		},
		{
			name:      "missing token",
			metadata:  metadata.Pairs(),
			wantToken: "",
			wantError: true,
		},
		{
			name:      "no metadata",
			metadata:  nil,
			wantToken: "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ctx context.Context
			if tt.metadata != nil {
				ctx = metadata.NewIncomingContext(context.Background(), tt.metadata)
			} else {
				ctx = context.Background()
			}

			token, err := extractSessionToken(ctx)

			if tt.wantError {
				if err == nil {
					t.Error("expected error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if token != tt.wantToken {
				t.Errorf("expected token '%s', got '%s'", tt.wantToken, token)
			}
		})
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

// generateTestEd25519Key generates a test Ed25519 SSH key pair.
func generateTestEd25519Key() (pubKeyBytes []byte, privKey ed25519.PrivateKey, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil, nil, err
	}

	return ssh.MarshalAuthorizedKey(sshPub), priv, nil
}

// signChallenge signs a challenge with an Ed25519 private key.
func signTestChallenge(privKey ed25519.PrivateKey, challenge []byte) ([]byte, error) {
	signer, err := ssh.NewSignerFromKey(privKey)
	if err != nil {
		return nil, err
	}

	sig, err := signer.Sign(rand.Reader, challenge)
	if err != nil {
		return nil, err
	}

	return sig.Blob, nil
}
