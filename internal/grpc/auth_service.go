// Package grpc provides gRPC service implementations for the bib daemon.
package grpc

import (
	"context"

	services "bib/api/gen/go/bib/v1/services"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AuthServiceServer implements the AuthService gRPC service.
type AuthServiceServer struct {
	services.UnimplementedAuthServiceServer
}

// NewAuthServiceServer creates a new AuthServiceServer.
func NewAuthServiceServer() *AuthServiceServer {
	return &AuthServiceServer{}
}

// Challenge requests a challenge for signature-based authentication.
func (s *AuthServiceServer) Challenge(ctx context.Context, req *services.ChallengeRequest) (*services.ChallengeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Challenge not implemented")
}

// VerifyChallenge verifies a signed challenge and returns a session token.
func (s *AuthServiceServer) VerifyChallenge(ctx context.Context, req *services.VerifyChallengeRequest) (*services.VerifyChallengeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method VerifyChallenge not implemented")
}

// Logout ends the current session.
func (s *AuthServiceServer) Logout(ctx context.Context, req *services.LogoutRequest) (*services.LogoutResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Logout not implemented")
}

// RefreshSession extends the current session's expiry.
func (s *AuthServiceServer) RefreshSession(ctx context.Context, req *services.RefreshSessionRequest) (*services.RefreshSessionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RefreshSession not implemented")
}

// ValidateSession checks if a session token is still valid.
func (s *AuthServiceServer) ValidateSession(ctx context.Context, req *services.ValidateSessionRequest) (*services.ValidateSessionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ValidateSession not implemented")
}

// GetAuthConfig returns the authentication configuration.
func (s *AuthServiceServer) GetAuthConfig(ctx context.Context, req *services.GetAuthConfigRequest) (*services.GetAuthConfigResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetAuthConfig not implemented")
}

// GetPublicKeyInfo returns information about a public key.
func (s *AuthServiceServer) GetPublicKeyInfo(ctx context.Context, req *services.GetPublicKeyInfoRequest) (*services.GetPublicKeyInfoResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetPublicKeyInfo not implemented")
}

// ListMySessions lists all active sessions for the current user.
func (s *AuthServiceServer) ListMySessions(ctx context.Context, req *services.ListMySessionsRequest) (*services.ListMySessionsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListMySessions not implemented")
}

// RevokeSession revokes a specific session.
func (s *AuthServiceServer) RevokeSession(ctx context.Context, req *services.RevokeSessionRequest) (*services.RevokeSessionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RevokeSession not implemented")
}

// RevokeAllSessions revokes all sessions except the current one.
func (s *AuthServiceServer) RevokeAllSessions(ctx context.Context, req *services.RevokeAllSessionsRequest) (*services.RevokeAllSessionsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RevokeAllSessions not implemented")
}
