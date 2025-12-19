// Package grpc provides gRPC service implementations for the bib daemon.
package grpc

import (
	"context"
	"fmt"
	"time"

	services "bib/api/gen/go/bib/v1/services"
	"bib/internal/auth"
	"bib/internal/config"
	"bib/internal/domain"
	"bib/internal/storage"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// sessionTokenHeader is the metadata key for session tokens.
	sessionTokenHeader = "x-session-token"

	// challengeTTL is how long authentication challenges remain valid.
	challengeTTL = 30 * time.Second
)

// AuthServiceServer implements the AuthService gRPC service.
type AuthServiceServer struct {
	services.UnimplementedAuthServiceServer

	authService    *auth.Service
	challengeStore *auth.ChallengeStore
	cfg            config.AuthConfig
	nodeID         string
	nodeMode       string
	version        string
}

// AuthServiceConfig holds configuration for the AuthServiceServer.
type AuthServiceConfig struct {
	AuthService *auth.Service
	AuthConfig  config.AuthConfig
	NodeID      string
	NodeMode    string
	Version     string
}

// NewAuthServiceServer creates a new AuthServiceServer.
func NewAuthServiceServer() *AuthServiceServer {
	return &AuthServiceServer{
		challengeStore: auth.NewChallengeStore(challengeTTL),
	}
}

// NewAuthServiceServerWithConfig creates a new AuthServiceServer with dependencies.
func NewAuthServiceServerWithConfig(cfg AuthServiceConfig) *AuthServiceServer {
	return &AuthServiceServer{
		authService:    cfg.AuthService,
		challengeStore: auth.NewChallengeStore(challengeTTL),
		cfg:            cfg.AuthConfig,
		nodeID:         cfg.NodeID,
		nodeMode:       cfg.NodeMode,
		version:        cfg.Version,
	}
}

// Challenge requests a challenge for signature-based authentication.
func (s *AuthServiceServer) Challenge(_ context.Context, req *services.ChallengeRequest) (*services.ChallengeResponse, error) {
	if s.challengeStore == nil {
		return nil, status.Error(codes.Unavailable, "auth service not initialized")
	}

	if len(req.PublicKey) == 0 {
		return nil, status.Error(codes.InvalidArgument, "public_key is required")
	}

	// Parse the public key to validate it and get canonical form
	keyInfo, err := auth.GetPublicKeyInfo(req.PublicKey)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid public key: %v", err)
	}

	// Create challenge
	challenge, err := s.challengeStore.Create(keyInfo.KeyBytes, string(keyInfo.KeyType))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create challenge: %v", err)
	}

	return &services.ChallengeResponse{
		ChallengeId:        challenge.ID,
		Challenge:          challenge.Nonce,
		ExpiresAt:          timestamppb.New(challenge.ExpiresAt),
		SignatureAlgorithm: challenge.SignatureAlgorithm,
	}, nil
}

// VerifyChallenge verifies a signed challenge and returns a session token.
func (s *AuthServiceServer) VerifyChallenge(ctx context.Context, req *services.VerifyChallengeRequest) (*services.VerifyChallengeResponse, error) {
	if s.challengeStore == nil || s.authService == nil {
		return nil, status.Error(codes.Unavailable, "auth service not initialized")
	}

	if req.ChallengeId == "" {
		return nil, status.Error(codes.InvalidArgument, "challenge_id is required")
	}
	if len(req.Signature) == 0 {
		return nil, status.Error(codes.InvalidArgument, "signature is required")
	}

	// Get and consume the challenge (one-time use)
	challenge, ok := s.challengeStore.Get(req.ChallengeId)
	if !ok {
		return nil, status.Error(codes.NotFound, "challenge not found or expired")
	}

	// Verify signature
	keyType := domain.KeyType(challenge.KeyType)
	if err := auth.VerifySSHSignature(challenge.PublicKey, keyType, challenge.Nonce, req.Signature); err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "signature verification failed: %v", err)
	}

	// Extract client info
	clientIP := ""
	clientAgent := ""
	clientMetadata := make(map[string]string)

	if req.ClientInfo != nil {
		clientIP = req.ClientInfo.IpAddress
		clientAgent = req.ClientInfo.UserAgent
		if req.ClientInfo.Version != "" {
			clientMetadata["client_version"] = req.ClientInfo.Version
		}
		for k, v := range req.ClientInfo.Metadata {
			clientMetadata[k] = v
		}
	}

	// Authenticate (creates user if auto-registration is enabled)
	result, err := s.authService.Authenticate(ctx, auth.AuthenticateRequest{
		PublicKey:   challenge.PublicKey,
		KeyType:     keyType,
		Name:        req.Name,
		Email:       req.Email,
		SessionType: storage.SessionTypeGRPC,
		ClientIP:    clientIP,
		ClientAgent: clientAgent,
		Metadata:    clientMetadata,
	})
	if err != nil {
		return nil, authErrorToGRPC(err)
	}

	// Calculate session expiry
	expiresAt := time.Now().Add(s.cfg.SessionTimeout)
	if s.cfg.SessionTimeout == 0 {
		expiresAt = time.Now().Add(24 * time.Hour) // Default 24h
	}

	return &services.VerifyChallengeResponse{
		SessionToken: result.Session.ID, // Using session ID as opaque token
		ExpiresAt:    timestamppb.New(expiresAt),
		User:         domainUserToProto(result.User),
		IsNewUser:    result.IsNew,
		Session:      storageSessionToProto(result.Session, true),
	}, nil
}

// Logout ends the current session.
func (s *AuthServiceServer) Logout(ctx context.Context, req *services.LogoutRequest) (*services.LogoutResponse, error) {
	if s.authService == nil {
		return nil, status.Error(codes.Unavailable, "auth service not initialized")
	}

	sessionID := req.SessionId
	if sessionID == "" {
		// Use current session from metadata
		var err error
		sessionID, err = extractSessionToken(ctx)
		if err != nil {
			return nil, err
		}
	}

	if err := s.authService.EndSession(ctx, sessionID); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to end session: %v", err)
	}

	return &services.LogoutResponse{Success: true}, nil
}

// RefreshSession extends the current session's expiry.
func (s *AuthServiceServer) RefreshSession(ctx context.Context, _ *services.RefreshSessionRequest) (*services.RefreshSessionResponse, error) {
	if s.authService == nil {
		return nil, status.Error(codes.Unavailable, "auth service not initialized")
	}

	sessionID, err := extractSessionToken(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.authService.UpdateSessionActivity(ctx, sessionID); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to refresh session: %v", err)
	}

	// Calculate new expiry
	expiresAt := time.Now().Add(s.cfg.SessionTimeout)
	if s.cfg.SessionTimeout == 0 {
		expiresAt = time.Now().Add(24 * time.Hour)
	}

	return &services.RefreshSessionResponse{
		SessionToken: sessionID, // Token doesn't change for opaque tokens
		ExpiresAt:    timestamppb.New(expiresAt),
	}, nil
}

// ValidateSession checks if a session token is still valid.
func (s *AuthServiceServer) ValidateSession(ctx context.Context, req *services.ValidateSessionRequest) (*services.ValidateSessionResponse, error) {
	if s.authService == nil {
		return nil, status.Error(codes.Unavailable, "auth service not initialized")
	}

	sessionToken := req.SessionToken
	if sessionToken == "" {
		return &services.ValidateSessionResponse{
			Valid:         false,
			InvalidReason: "session_token is required",
		}, nil
	}

	session, err := s.authService.GetSession(ctx, sessionToken)
	if err != nil {
		reason := "session not found"
		if err == domain.ErrSessionExpired {
			reason = "session expired"
		}
		return &services.ValidateSessionResponse{
			Valid:         false,
			InvalidReason: reason,
		}, nil
	}

	// Get user
	user, err := s.authService.GetUser(ctx, session.UserID)
	if err != nil {
		return &services.ValidateSessionResponse{
			Valid:         false,
			InvalidReason: "user not found",
		}, nil
	}

	// Check user status
	if user.Status != domain.UserStatusActive {
		return &services.ValidateSessionResponse{
			Valid:         false,
			InvalidReason: fmt.Sprintf("user status: %s", user.Status),
		}, nil
	}

	// Calculate expiry
	expiresAt := session.LastActivityAt.Add(s.cfg.SessionTimeout)
	if s.cfg.SessionTimeout == 0 {
		expiresAt = session.LastActivityAt.Add(24 * time.Hour)
	}

	return &services.ValidateSessionResponse{
		Valid:     true,
		User:      domainUserToProto(user),
		Session:   storageSessionToProto(session, true),
		ExpiresAt: timestamppb.New(expiresAt),
	}, nil
}

// GetAuthConfig returns the authentication configuration.
func (s *AuthServiceServer) GetAuthConfig(_ context.Context, _ *services.GetAuthConfigRequest) (*services.GetAuthConfigResponse, error) {
	sessionTimeout := int64(s.cfg.SessionTimeout.Seconds())
	if sessionTimeout == 0 {
		sessionTimeout = 86400 // 24 hours default
	}

	return &services.GetAuthConfigResponse{
		AllowAutoRegistration:     s.cfg.AllowAutoRegistration,
		RequireEmail:              s.cfg.RequireEmail,
		DefaultRole:               s.cfg.DefaultRole,
		SessionTimeoutSeconds:     sessionTimeout,
		MaxSessionLifetimeSeconds: sessionTimeout * 7, // 7x session timeout as max lifetime
		SupportedKeyTypes:         []string{"ed25519", "rsa"},
		ServerVersion:             s.version,
		NodeId:                    s.nodeID,
		NodeMode:                  s.nodeMode,
	}, nil
}

// GetPublicKeyInfo returns information about a public key.
func (s *AuthServiceServer) GetPublicKeyInfo(ctx context.Context, req *services.GetPublicKeyInfoRequest) (*services.GetPublicKeyInfoResponse, error) {
	if len(req.PublicKey) == 0 {
		return nil, status.Error(codes.InvalidArgument, "public_key is required")
	}

	keyInfo, err := auth.GetPublicKeyInfo(req.PublicKey)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to parse public key: %v", err)
	}

	resp := &services.GetPublicKeyInfoResponse{
		KeyType:           string(keyInfo.KeyType),
		FingerprintSha256: keyInfo.FingerprintSHA256,
		FingerprintMd5:    keyInfo.FingerprintMD5,
		KeySize:           int32(keyInfo.KeySize),
		OpensshFormat:     keyInfo.OpenSSHFormat,
	}

	// Check if user exists with this key
	if s.authService != nil {
		user, err := s.authService.GetUserByPublicKey(ctx, keyInfo.KeyBytes)
		if err == nil && user != nil {
			resp.HasUser = true
			resp.UserId = string(user.ID)
		}
	}

	return resp, nil
}

// ListMySessions lists all active sessions for the current user.
func (s *AuthServiceServer) ListMySessions(ctx context.Context, req *services.ListMySessionsRequest) (*services.ListMySessionsResponse, error) {
	if s.authService == nil {
		return nil, status.Error(codes.Unavailable, "auth service not initialized")
	}

	// Get current session to identify user
	sessionID, err := extractSessionToken(ctx)
	if err != nil {
		return nil, err
	}

	session, err := s.authService.GetSession(ctx, sessionID)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid session: %v", err)
	}

	// Get all sessions for this user
	sessions, err := s.authService.ListUserSessions(ctx, session.UserID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list sessions: %v", err)
	}

	// Convert to proto
	protoSessions := make([]*services.SessionInfo, 0, len(sessions))
	for _, sess := range sessions {
		// Skip ended sessions unless requested
		if sess.EndedAt != nil && !req.IncludeExpired {
			continue
		}
		isCurrent := sess.ID == sessionID
		protoSessions = append(protoSessions, storageSessionToProto(sess, isCurrent))
	}

	// Apply limit
	if req.Limit > 0 && int(req.Limit) < len(protoSessions) {
		protoSessions = protoSessions[:req.Limit]
	}

	return &services.ListMySessionsResponse{
		Sessions:   protoSessions,
		TotalCount: int32(len(protoSessions)),
	}, nil
}

// RevokeSession revokes a specific session.
func (s *AuthServiceServer) RevokeSession(ctx context.Context, req *services.RevokeSessionRequest) (*services.RevokeSessionResponse, error) {
	if s.authService == nil {
		return nil, status.Error(codes.Unavailable, "auth service not initialized")
	}

	if req.SessionId == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}

	// Get current session to verify ownership
	currentSessionID, err := extractSessionToken(ctx)
	if err != nil {
		return nil, err
	}

	currentSession, err := s.authService.GetSession(ctx, currentSessionID)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid session: %v", err)
	}

	// Get target session to verify ownership
	targetSession, err := s.authService.GetSession(ctx, req.SessionId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "session not found: %v", err)
	}

	// Verify same user
	if targetSession.UserID != currentSession.UserID {
		return nil, status.Error(codes.PermissionDenied, "cannot revoke another user's session")
	}

	if err := s.authService.EndSession(ctx, req.SessionId); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to revoke session: %v", err)
	}

	return &services.RevokeSessionResponse{Success: true}, nil
}

// RevokeAllSessions revokes all sessions except the current one.
func (s *AuthServiceServer) RevokeAllSessions(ctx context.Context, req *services.RevokeAllSessionsRequest) (*services.RevokeAllSessionsResponse, error) {
	if s.authService == nil {
		return nil, status.Error(codes.Unavailable, "auth service not initialized")
	}

	// Get current session
	currentSessionID, err := extractSessionToken(ctx)
	if err != nil {
		return nil, err
	}

	currentSession, err := s.authService.GetSession(ctx, currentSessionID)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid session: %v", err)
	}

	// Get all sessions for user
	sessions, err := s.authService.ListUserSessions(ctx, currentSession.UserID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list sessions: %v", err)
	}

	// Revoke all except current (unless IncludeCurrent is true)
	revokedCount := int32(0)
	for _, sess := range sessions {
		if sess.EndedAt != nil {
			continue // Already ended
		}
		if sess.ID == currentSessionID && !req.IncludeCurrent {
			continue // Skip current
		}
		if err := s.authService.EndSession(ctx, sess.ID); err == nil {
			revokedCount++
		}
	}

	return &services.RevokeAllSessionsResponse{RevokedCount: revokedCount}, nil
}

// SetDependencies sets the service dependencies after creation.
// This is useful when dependencies aren't available at construction time.
func (s *AuthServiceServer) SetDependencies(authSvc *auth.Service, cfg config.AuthConfig, nodeID, nodeMode, version string) {
	s.authService = authSvc
	s.cfg = cfg
	s.nodeID = nodeID
	s.nodeMode = nodeMode
	s.version = version
}

// Stop stops the auth service (cleanup resources).
func (s *AuthServiceServer) Stop() {
	if s.challengeStore != nil {
		s.challengeStore.Stop()
	}
}

// extractSessionToken extracts the session token from gRPC metadata.
func extractSessionToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing metadata")
	}

	tokens := md.Get(sessionTokenHeader)
	if len(tokens) == 0 {
		// Also check authorization header
		tokens = md.Get("authorization")
		if len(tokens) == 0 {
			return "", status.Error(codes.Unauthenticated, "missing session token")
		}
		// Strip "Bearer " prefix if present
		token := tokens[0]
		if len(token) > 7 && token[:7] == "Bearer " {
			return token[7:], nil
		}
		return token, nil
	}

	return tokens[0], nil
}

// authErrorToGRPC converts auth errors to gRPC status errors.
func authErrorToGRPC(err error) error {
	switch err {
	case domain.ErrUserNotFound:
		return status.Error(codes.NotFound, "user not found")
	case domain.ErrUserPending:
		return status.Error(codes.PermissionDenied, "user account is pending approval")
	case domain.ErrUserSuspended:
		return status.Error(codes.PermissionDenied, "user account is suspended")
	case domain.ErrAutoRegDisabled:
		return status.Error(codes.PermissionDenied, "auto-registration is disabled")
	case domain.ErrUnauthorized:
		return status.Error(codes.Unauthenticated, "unauthorized")
	case domain.ErrSessionExpired:
		return status.Error(codes.Unauthenticated, "session expired")
	default:
		return status.Errorf(codes.Internal, "authentication failed: %v", err)
	}
}

// domainUserToProto converts a domain user to proto UserInfo.
func domainUserToProto(user *domain.User) *services.UserInfo {
	return &services.UserInfo{
		Id:                   string(user.ID),
		Name:                 user.Name,
		Email:                user.Email,
		Role:                 string(user.Role),
		Status:               string(user.Status),
		PublicKeyFingerprint: user.PublicKeyFingerprint,
		Locale:               user.Locale,
	}
}

// storageSessionToProto converts a storage session to proto SessionInfo.
func storageSessionToProto(session *storage.Session, isCurrent bool) *services.SessionInfo {
	info := &services.SessionInfo{
		Id:          session.ID,
		Type:        string(session.Type),
		ClientIp:    session.ClientIP,
		ClientAgent: session.ClientAgent,
		NodeId:      session.NodeID,
		StartedAt:   timestamppb.New(session.StartedAt),
		IsCurrent:   isCurrent,
	}

	if !session.LastActivityAt.IsZero() {
		info.LastActivityAt = timestamppb.New(session.LastActivityAt)
	}

	if session.EndedAt != nil {
		info.ExpiresAt = timestamppb.New(*session.EndedAt)
	}

	return info
}
