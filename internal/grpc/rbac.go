// Package grpc provides gRPC service implementations for the bib daemon.
package grpc

import (
	"context"
	"strings"

	"bib/internal/domain"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// RBACConfig holds configuration for the RBAC interceptor.
type RBACConfig struct {
	// Enabled determines if RBAC checks are enforced.
	Enabled bool

	// BootstrapMode allows unauthenticated access to certain endpoints.
	// Used during initial setup when no users exist yet.
	BootstrapMode bool
}

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	// userContextKey is the context key for the authenticated user.
	userContextKey contextKey = "user"

	// sessionContextKey is the context key for the current session.
	sessionContextKey contextKey = "session"
)

// UserFromContext extracts the authenticated user from the context.
func UserFromContext(ctx context.Context) (*domain.User, bool) {
	user, ok := ctx.Value(userContextKey).(*domain.User)
	return user, ok
}

// WithUser adds a user to the context.
func WithUser(ctx context.Context, user *domain.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// MethodPermission defines the permission requirements for a method.
type MethodPermission struct {
	// RequiresAuth indicates if the method requires authentication.
	RequiresAuth bool

	// RequiredRole is the minimum role required (empty = any authenticated user).
	RequiredRole domain.UserRole

	// AllowSelf allows access if operating on own resources.
	AllowSelf bool

	// AllowBootstrap allows access during bootstrap mode.
	AllowBootstrap bool
}

// methodPermissions maps gRPC method names to their permission requirements.
// Format: /package.Service/Method
var methodPermissions = map[string]MethodPermission{
	// HealthService - public endpoints
	"/bib.v1.services.HealthService/Check":     {RequiresAuth: false},
	"/bib.v1.services.HealthService/Watch":     {RequiresAuth: false},
	"/bib.v1.services.HealthService/GetStatus": {RequiresAuth: false},

	// AuthService - authentication endpoints (public for challenge, self for session management)
	"/bib.v1.services.AuthService/Challenge":       {RequiresAuth: false},
	"/bib.v1.services.AuthService/VerifyChallenge": {RequiresAuth: false},
	"/bib.v1.services.AuthService/Logout":          {RequiresAuth: true, AllowSelf: true},
	"/bib.v1.services.AuthService/GetSession":      {RequiresAuth: true, AllowSelf: true},
	"/bib.v1.services.AuthService/RefreshSession":  {RequiresAuth: true, AllowSelf: true},
	"/bib.v1.services.AuthService/ListMySessions":  {RequiresAuth: true, AllowSelf: true},

	// UserService - admin endpoints except for self-management
	"/bib.v1.services.UserService/GetUser":               {RequiresAuth: true, RequiredRole: domain.UserRoleAdmin},
	"/bib.v1.services.UserService/GetUserByPublicKey":    {RequiresAuth: true, RequiredRole: domain.UserRoleAdmin},
	"/bib.v1.services.UserService/ListUsers":             {RequiresAuth: true, RequiredRole: domain.UserRoleAdmin},
	"/bib.v1.services.UserService/SearchUsers":           {RequiresAuth: true, RequiredRole: domain.UserRoleAdmin},
	"/bib.v1.services.UserService/CreateUser":            {RequiresAuth: true, RequiredRole: domain.UserRoleAdmin},
	"/bib.v1.services.UserService/UpdateUser":            {RequiresAuth: true, RequiredRole: domain.UserRoleAdmin},
	"/bib.v1.services.UserService/DeleteUser":            {RequiresAuth: true, RequiredRole: domain.UserRoleAdmin},
	"/bib.v1.services.UserService/SuspendUser":           {RequiresAuth: true, RequiredRole: domain.UserRoleAdmin},
	"/bib.v1.services.UserService/ActivateUser":          {RequiresAuth: true, RequiredRole: domain.UserRoleAdmin},
	"/bib.v1.services.UserService/SetUserRole":           {RequiresAuth: true, RequiredRole: domain.UserRoleAdmin},
	"/bib.v1.services.UserService/GetCurrentUser":        {RequiresAuth: true, AllowSelf: true, AllowBootstrap: true},
	"/bib.v1.services.UserService/UpdateCurrentUser":     {RequiresAuth: true, AllowSelf: true, AllowBootstrap: true},
	"/bib.v1.services.UserService/GetUserPreferences":    {RequiresAuth: true, AllowSelf: true},
	"/bib.v1.services.UserService/UpdateUserPreferences": {RequiresAuth: true, AllowSelf: true},
	"/bib.v1.services.UserService/ListUserSessions":      {RequiresAuth: true, AllowSelf: true}, // Admin or self
	"/bib.v1.services.UserService/EndUserSession":        {RequiresAuth: true, AllowSelf: true}, // Admin or self
	"/bib.v1.services.UserService/EndAllUserSessions":    {RequiresAuth: true, RequiredRole: domain.UserRoleAdmin},

	// NodeService - mostly read-only, admin for management
	"/bib.v1.services.NodeService/GetNode":            {RequiresAuth: true},
	"/bib.v1.services.NodeService/ListNodes":          {RequiresAuth: true},
	"/bib.v1.services.NodeService/GetSelfNode":        {RequiresAuth: true},
	"/bib.v1.services.NodeService/ConnectPeer":        {RequiresAuth: true, RequiredRole: domain.UserRoleAdmin},
	"/bib.v1.services.NodeService/DisconnectPeer":     {RequiresAuth: true, RequiredRole: domain.UserRoleAdmin},
	"/bib.v1.services.NodeService/GetNetworkStats":    {RequiresAuth: true},
	"/bib.v1.services.NodeService/StreamNodeEvents":   {RequiresAuth: true},
	"/bib.v1.services.NodeService/GetPeerInfo":        {RequiresAuth: true},
	"/bib.v1.services.NodeService/ListConnectedPeers": {RequiresAuth: true},
	"/bib.v1.services.NodeService/BanPeer":            {RequiresAuth: true, RequiredRole: domain.UserRoleAdmin},
	"/bib.v1.services.NodeService/UnbanPeer":          {RequiresAuth: true, RequiredRole: domain.UserRoleAdmin},
	"/bib.v1.services.NodeService/ListBannedPeers":    {RequiresAuth: true, RequiredRole: domain.UserRoleAdmin},

	// TopicService - admin for create/delete, owner-based for updates
	"/bib.v1.services.TopicService/CreateTopic":        {RequiresAuth: true, RequiredRole: domain.UserRoleAdmin},
	"/bib.v1.services.TopicService/GetTopic":           {RequiresAuth: true}, // Visibility checked in handler
	"/bib.v1.services.TopicService/ListTopics":         {RequiresAuth: true}, // Visibility filtered in handler
	"/bib.v1.services.TopicService/UpdateTopic":        {RequiresAuth: true}, // Owner check in handler
	"/bib.v1.services.TopicService/DeleteTopic":        {RequiresAuth: true}, // Owner check in handler
	"/bib.v1.services.TopicService/Subscribe":          {RequiresAuth: true},
	"/bib.v1.services.TopicService/Unsubscribe":        {RequiresAuth: true},
	"/bib.v1.services.TopicService/ListSubscriptions":  {RequiresAuth: true},
	"/bib.v1.services.TopicService/GetSubscription":    {RequiresAuth: true},
	"/bib.v1.services.TopicService/StreamTopicUpdates": {RequiresAuth: true},
	"/bib.v1.services.TopicService/GetTopicStats":      {RequiresAuth: true},
	"/bib.v1.services.TopicService/SearchTopics":       {RequiresAuth: true},

	// AdminService - all admin-only
	"/bib.v1.services.AdminService/GetConfig":        {RequiresAuth: true, RequiredRole: domain.UserRoleAdmin},
	"/bib.v1.services.AdminService/UpdateConfig":     {RequiresAuth: true, RequiredRole: domain.UserRoleAdmin},
	"/bib.v1.services.AdminService/GetMetrics":       {RequiresAuth: true, RequiredRole: domain.UserRoleAdmin},
	"/bib.v1.services.AdminService/StreamLogs":       {RequiresAuth: true, RequiredRole: domain.UserRoleAdmin},
	"/bib.v1.services.AdminService/GetAuditLogs":     {RequiresAuth: true, RequiredRole: domain.UserRoleAdmin},
	"/bib.v1.services.AdminService/TriggerBackup":    {RequiresAuth: true, RequiredRole: domain.UserRoleAdmin},
	"/bib.v1.services.AdminService/ListBackups":      {RequiresAuth: true, RequiredRole: domain.UserRoleAdmin},
	"/bib.v1.services.AdminService/GetClusterStatus": {RequiresAuth: true, RequiredRole: domain.UserRoleAdmin},
	"/bib.v1.services.AdminService/Shutdown":         {RequiresAuth: true, RequiredRole: domain.UserRoleAdmin},
}

// RBACInterceptor creates a unary interceptor that enforces role-based access control.
func RBACInterceptor(cfg RBACConfig, getUserFromToken func(ctx context.Context, token string) (*domain.User, error)) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if !cfg.Enabled {
			return handler(ctx, req)
		}

		perm, ok := methodPermissions[info.FullMethod]
		if !ok {
			// Unknown method - deny by default in production
			return nil, status.Errorf(codes.PermissionDenied, "unknown method: %s", info.FullMethod)
		}

		// Public endpoints
		if !perm.RequiresAuth {
			return handler(ctx, req)
		}

		// Bootstrap mode allows certain endpoints without auth
		if cfg.BootstrapMode && perm.AllowBootstrap {
			return handler(ctx, req)
		}

		// Extract and validate token
		user, err := extractAndValidateUser(ctx, getUserFromToken)
		if err != nil {
			return nil, err
		}

		// Check user status
		if !user.IsActive() {
			return nil, status.Error(codes.PermissionDenied, "user account is not active")
		}

		// Check required role
		if perm.RequiredRole != "" && !hasMinimumRole(user.Role, perm.RequiredRole) {
			return nil, status.Errorf(codes.PermissionDenied, "insufficient permissions: requires %s role", perm.RequiredRole)
		}

		// Add user to context and continue
		ctx = WithUser(ctx, user)
		return handler(ctx, req)
	}
}

// RBACStreamInterceptor creates a stream interceptor that enforces role-based access control.
func RBACStreamInterceptor(cfg RBACConfig, getUserFromToken func(ctx context.Context, token string) (*domain.User, error)) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if !cfg.Enabled {
			return handler(srv, ss)
		}

		perm, ok := methodPermissions[info.FullMethod]
		if !ok {
			return status.Errorf(codes.PermissionDenied, "unknown method: %s", info.FullMethod)
		}

		if !perm.RequiresAuth {
			return handler(srv, ss)
		}

		if cfg.BootstrapMode && perm.AllowBootstrap {
			return handler(srv, ss)
		}

		user, err := extractAndValidateUser(ss.Context(), getUserFromToken)
		if err != nil {
			return err
		}

		if !user.IsActive() {
			return status.Error(codes.PermissionDenied, "user account is not active")
		}

		if perm.RequiredRole != "" && !hasMinimumRole(user.Role, perm.RequiredRole) {
			return status.Errorf(codes.PermissionDenied, "insufficient permissions: requires %s role", perm.RequiredRole)
		}

		// Wrap the stream to include user in context
		wrapped := &wrappedServerStream{
			ServerStream: ss,
			ctx:          WithUser(ss.Context(), user),
		}
		return handler(srv, wrapped)
	}
}

// wrappedServerStream wraps a grpc.ServerStream to override the context.
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

// extractAndValidateUser extracts the session token from metadata and validates the user.
func extractAndValidateUser(ctx context.Context, getUserFromToken func(ctx context.Context, token string) (*domain.User, error)) (*domain.User, error) {
	token := extractToken(ctx)
	if token == "" {
		return nil, status.Error(codes.Unauthenticated, "missing authentication token")
	}

	user, err := getUserFromToken(ctx, token)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
	}

	return user, nil
}

// extractToken extracts the session token from gRPC metadata.
func extractToken(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	// Try x-session-token header first
	if tokens := md.Get("x-session-token"); len(tokens) > 0 {
		return tokens[0]
	}

	// Try authorization header
	if auth := md.Get("authorization"); len(auth) > 0 {
		token := auth[0]
		// Handle "Bearer <token>" format
		if strings.HasPrefix(strings.ToLower(token), "bearer ") {
			return token[7:]
		}
		return token
	}

	return ""
}

// hasMinimumRole checks if the user's role meets the minimum required role.
func hasMinimumRole(userRole, requiredRole domain.UserRole) bool {
	roleHierarchy := map[domain.UserRole]int{
		domain.UserRoleAdmin:    3,
		domain.UserRoleUser:     2,
		domain.UserRoleReadonly: 1,
	}

	userLevel, ok := roleHierarchy[userRole]
	if !ok {
		return false
	}

	requiredLevel, ok := roleHierarchy[requiredRole]
	if !ok {
		return false
	}

	return userLevel >= requiredLevel
}
