// Package grpc provides gRPC service implementations for the bib daemon.
package grpc

import (
	"context"

	bibv1 "bib/api/gen/go/bib/v1"
	services "bib/api/gen/go/bib/v1/services"
	"bib/internal/domain"
	"bib/internal/storage"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// UserServiceServer implements the UserService gRPC service.
type UserServiceServer struct {
	services.UnimplementedUserServiceServer
	store       storage.Store
	auditLogger *AuditMiddleware
}

// UserServiceConfig holds configuration for the UserServiceServer.
type UserServiceConfig struct {
	Store       storage.Store
	AuditLogger *AuditMiddleware
}

// NewUserServiceServer creates a new UserServiceServer.
func NewUserServiceServer() *UserServiceServer {
	return &UserServiceServer{}
}

// NewUserServiceServerWithConfig creates a new UserServiceServer with dependencies.
func NewUserServiceServerWithConfig(cfg UserServiceConfig) *UserServiceServer {
	return &UserServiceServer{
		store:       cfg.Store,
		auditLogger: cfg.AuditLogger,
	}
}

// GetUser retrieves a user by ID.
func (s *UserServiceServer) GetUser(ctx context.Context, req *services.GetUserRequest) (*services.GetUserResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.UserId == "" {
		return nil, NewValidationError("user_id is required", map[string]string{
			"user_id": "must not be empty",
		})
	}

	user, err := s.store.Users().Get(ctx, domain.UserID(req.UserId))
	if err != nil {
		return nil, MapDomainError(err)
	}

	return &services.GetUserResponse{
		User: userToProtoUser(user),
	}, nil
}

// GetUserByPublicKey retrieves a user by their public key.
func (s *UserServiceServer) GetUserByPublicKey(ctx context.Context, req *services.GetUserByPublicKeyRequest) (*services.GetUserByPublicKeyResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if len(req.PublicKey) == 0 {
		return nil, NewValidationError("public_key is required", map[string]string{
			"public_key": "must not be empty",
		})
	}

	user, err := s.store.Users().GetByPublicKey(ctx, req.PublicKey)
	if err != nil {
		return nil, MapDomainError(err)
	}

	return &services.GetUserByPublicKeyResponse{
		User: userToProtoUser(user),
	}, nil
}

// ListUsers lists users with filtering and pagination.
func (s *UserServiceServer) ListUsers(ctx context.Context, req *services.ListUsersRequest) (*services.ListUsersResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	filter := storage.UserFilter{
		Status: protoUserStatusToDomain(req.Status),
		Role:   protoUserRoleToDomain(req.Role),
	}

	if req.Page != nil {
		filter.Limit = int(req.Page.Limit)
		filter.Offset = int(req.Page.Offset)
	}

	if req.Sort != nil {
		filter.OrderBy = req.Sort.Field
		filter.OrderDesc = req.Sort.Descending
	}

	// Apply defaults
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 1000 {
		filter.Limit = 1000
	}

	users, err := s.store.Users().List(ctx, filter)
	if err != nil {
		return nil, MapDomainError(err)
	}

	total, err := s.store.Users().Count(ctx, filter)
	if err != nil {
		return nil, MapDomainError(err)
	}

	protoUsers := make([]*services.User, len(users))
	for i, u := range users {
		protoUsers[i] = userToProtoUser(u)
	}

	return &services.ListUsersResponse{
		Users: protoUsers,
		PageInfo: &bibv1.PageInfo{
			TotalCount: total,
			HasMore:    int64(filter.Offset+len(users)) < total,
			PageSize:   int32(len(users)),
		},
	}, nil
}

// SearchUsers searches users by text query.
func (s *UserServiceServer) SearchUsers(ctx context.Context, req *services.SearchUsersRequest) (*services.SearchUsersResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	// Validate minimum query length
	if err := ValidateSearchQuery(req.Query); err != nil {
		return nil, err
	}

	filter := storage.UserFilter{
		Search: req.Query,
		Status: protoUserStatusToDomain(req.Status),
		Role:   protoUserRoleToDomain(req.Role),
	}

	if req.Page != nil {
		filter.Limit = int(req.Page.Limit)
		filter.Offset = int(req.Page.Offset)
	}

	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 1000 {
		filter.Limit = 1000
	}

	users, err := s.store.Users().List(ctx, filter)
	if err != nil {
		return nil, MapDomainError(err)
	}

	total, err := s.store.Users().Count(ctx, filter)
	if err != nil {
		return nil, MapDomainError(err)
	}

	protoUsers := make([]*services.User, len(users))
	for i, u := range users {
		protoUsers[i] = userToProtoUser(u)
	}

	return &services.SearchUsersResponse{
		Users: protoUsers,
		PageInfo: &bibv1.PageInfo{
			TotalCount: total,
			HasMore:    int64(filter.Offset+len(users)) < total,
			PageSize:   int32(len(users)),
		},
	}, nil
}

// CreateUser creates a new user.
func (s *UserServiceServer) CreateUser(ctx context.Context, req *services.CreateUserRequest) (*services.CreateUserResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	// Validate required fields
	violations := make(map[string]string)
	if len(req.PublicKey) == 0 {
		violations["public_key"] = "must not be empty"
	}
	if req.KeyType == "" {
		violations["key_type"] = "must not be empty"
	}
	if req.Name == "" {
		violations["name"] = "must not be empty"
	}
	if len(violations) > 0 {
		return nil, NewValidationError("invalid create user request", violations)
	}

	// Check if user already exists
	exists, err := s.store.Users().Exists(ctx, req.PublicKey)
	if err != nil {
		return nil, MapDomainError(err)
	}
	if exists {
		return nil, MapDomainError(domain.ErrUserExists)
	}

	// Create user
	keyType := domain.KeyType(req.KeyType)
	user := domain.NewUser(req.PublicKey, keyType, req.Name, req.Email, false)

	// Set role if specified
	if req.Role != services.UserRole_USER_ROLE_UNSPECIFIED {
		user.Role = protoUserRoleToDomain(req.Role)
	}

	if err := s.store.Users().Create(ctx, user); err != nil {
		return nil, MapDomainError(err)
	}

	// Audit log
	if s.auditLogger != nil {
		s.auditLogger.LogMutation(ctx, "CREATE", "user", string(user.ID), "Created user: "+user.Name)
	}

	return &services.CreateUserResponse{
		User: userToProtoUser(user),
	}, nil
}

// UpdateUser updates an existing user.
func (s *UserServiceServer) UpdateUser(ctx context.Context, req *services.UpdateUserRequest) (*services.UpdateUserResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.UserId == "" {
		return nil, NewValidationError("user_id is required", map[string]string{
			"user_id": "must not be empty",
		})
	}

	user, err := s.store.Users().Get(ctx, domain.UserID(req.UserId))
	if err != nil {
		return nil, MapDomainError(err)
	}

	// Update fields
	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.Locale != nil {
		user.Locale = *req.Locale
	}
	if req.UpdateMetadata && req.Metadata != nil {
		user.Metadata = req.Metadata
	}

	if err := user.Validate(); err != nil {
		return nil, MapDomainError(err)
	}

	if err := s.store.Users().Update(ctx, user); err != nil {
		return nil, MapDomainError(err)
	}

	// Audit log
	if s.auditLogger != nil {
		s.auditLogger.LogMutation(ctx, "UPDATE", "user", string(user.ID), "Updated user: "+user.Name)
	}

	return &services.UpdateUserResponse{
		User: userToProtoUser(user),
	}, nil
}

// DeleteUser soft-deletes a user.
func (s *UserServiceServer) DeleteUser(ctx context.Context, req *services.DeleteUserRequest) (*services.DeleteUserResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.UserId == "" {
		return nil, NewValidationError("user_id is required", map[string]string{
			"user_id": "must not be empty",
		})
	}

	if err := s.store.Users().Delete(ctx, domain.UserID(req.UserId)); err != nil {
		return nil, MapDomainError(err)
	}

	// End all sessions for the user
	if err := s.store.Sessions().EndAllForUser(ctx, domain.UserID(req.UserId)); err != nil {
		// Log but don't fail
	}

	// Audit log
	if s.auditLogger != nil {
		s.auditLogger.LogMutation(ctx, "DELETE", "user", req.UserId, "Deleted user")
	}

	return &services.DeleteUserResponse{
		Success: true,
	}, nil
}

// SuspendUser suspends a user account.
func (s *UserServiceServer) SuspendUser(ctx context.Context, req *services.SuspendUserRequest) (*services.SuspendUserResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.UserId == "" {
		return nil, NewValidationError("user_id is required", map[string]string{
			"user_id": "must not be empty",
		})
	}

	user, err := s.store.Users().Get(ctx, domain.UserID(req.UserId))
	if err != nil {
		return nil, MapDomainError(err)
	}

	user.Status = domain.UserStatusSuspended

	if err := s.store.Users().Update(ctx, user); err != nil {
		return nil, MapDomainError(err)
	}

	// End all sessions for the suspended user
	if err := s.store.Sessions().EndAllForUser(ctx, domain.UserID(req.UserId)); err != nil {
		// Log but don't fail
	}

	// Audit log
	if s.auditLogger != nil {
		s.auditLogger.LogMutation(ctx, "UPDATE", "user", req.UserId, "Suspended user. Reason: "+req.Reason)
	}

	return &services.SuspendUserResponse{
		User: userToProtoUser(user),
	}, nil
}

// ActivateUser activates a pending or suspended user.
func (s *UserServiceServer) ActivateUser(ctx context.Context, req *services.ActivateUserRequest) (*services.ActivateUserResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.UserId == "" {
		return nil, NewValidationError("user_id is required", map[string]string{
			"user_id": "must not be empty",
		})
	}

	user, err := s.store.Users().Get(ctx, domain.UserID(req.UserId))
	if err != nil {
		return nil, MapDomainError(err)
	}

	user.Status = domain.UserStatusActive

	if err := s.store.Users().Update(ctx, user); err != nil {
		return nil, MapDomainError(err)
	}

	// Audit log
	if s.auditLogger != nil {
		s.auditLogger.LogMutation(ctx, "UPDATE", "user", req.UserId, "Activated user")
	}

	return &services.ActivateUserResponse{
		User: userToProtoUser(user),
	}, nil
}

// SetUserRole changes a user's role.
func (s *UserServiceServer) SetUserRole(ctx context.Context, req *services.SetUserRoleRequest) (*services.SetUserRoleResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.UserId == "" {
		return nil, NewValidationError("user_id is required", map[string]string{
			"user_id": "must not be empty",
		})
	}

	if req.Role == services.UserRole_USER_ROLE_UNSPECIFIED {
		return nil, NewValidationError("role is required", map[string]string{
			"role": "must be specified",
		})
	}

	user, err := s.store.Users().Get(ctx, domain.UserID(req.UserId))
	if err != nil {
		return nil, MapDomainError(err)
	}

	oldRole := user.Role
	user.Role = protoUserRoleToDomain(req.Role)

	if err := s.store.Users().Update(ctx, user); err != nil {
		return nil, MapDomainError(err)
	}

	// Audit log
	if s.auditLogger != nil {
		s.auditLogger.LogMutation(ctx, "UPDATE", "user", req.UserId,
			"Changed role from "+string(oldRole)+" to "+string(user.Role))
	}

	return &services.SetUserRoleResponse{
		User: userToProtoUser(user),
	}, nil
}

// GetCurrentUser retrieves the currently authenticated user.
func (s *UserServiceServer) GetCurrentUser(ctx context.Context, _ *services.GetCurrentUserRequest) (*services.GetCurrentUserResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	user, ok := UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	return &services.GetCurrentUserResponse{
		User: userToProtoUser(user),
	}, nil
}

// UpdateCurrentUser updates the current user's profile.
func (s *UserServiceServer) UpdateCurrentUser(ctx context.Context, req *services.UpdateCurrentUserRequest) (*services.UpdateCurrentUserResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	user, ok := UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	// Fetch fresh copy from DB
	user, err := s.store.Users().Get(ctx, user.ID)
	if err != nil {
		return nil, MapDomainError(err)
	}

	// Update allowed fields
	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.Locale != nil {
		user.Locale = *req.Locale
	}

	if err := user.Validate(); err != nil {
		return nil, MapDomainError(err)
	}

	if err := s.store.Users().Update(ctx, user); err != nil {
		return nil, MapDomainError(err)
	}

	return &services.UpdateCurrentUserResponse{
		User: userToProtoUser(user),
	}, nil
}

// GetUserPreferences retrieves user preferences.
func (s *UserServiceServer) GetUserPreferences(ctx context.Context, req *services.GetUserPreferencesRequest) (*services.GetUserPreferencesResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	// Get user ID from context or request
	var userID domain.UserID
	if req.UserId != "" {
		// Admin accessing another user's preferences
		userID = domain.UserID(req.UserId)
	} else {
		// Current user
		user, ok := UserFromContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "not authenticated")
		}
		userID = user.ID
	}

	prefs, err := s.store.UserPreferences().Get(ctx, userID)
	if err != nil {
		return nil, MapDomainError(err)
	}

	return &services.GetUserPreferencesResponse{
		Preferences: storagePrefsToProto(prefs),
	}, nil
}

// UpdateUserPreferences updates user preferences.
func (s *UserServiceServer) UpdateUserPreferences(ctx context.Context, req *services.UpdateUserPreferencesRequest) (*services.UpdateUserPreferencesResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	// Current user only
	user, ok := UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	// Get existing preferences
	prefs, err := s.store.UserPreferences().Get(ctx, user.ID)
	if err != nil {
		// Create new if not found
		prefs = &storage.UserPreferences{
			UserID:               user.ID,
			Theme:                "system",
			Locale:               "en",
			Timezone:             "UTC",
			DateFormat:           "YYYY-MM-DD",
			NotificationsEnabled: true,
		}
	}

	// Update fields from request
	if req.Preferences != nil {
		if req.Preferences.Theme != "" {
			prefs.Theme = req.Preferences.Theme
		}
		if req.Preferences.Locale != "" {
			prefs.Locale = req.Preferences.Locale
		}
		if req.Preferences.Timezone != "" {
			prefs.Timezone = req.Preferences.Timezone
		}
		if req.Preferences.DateFormat != "" {
			prefs.DateFormat = req.Preferences.DateFormat
		}
		prefs.NotificationsEnabled = req.Preferences.NotificationsEnabled
		prefs.EmailNotifications = req.Preferences.EmailNotifications
		if req.Preferences.Custom != nil {
			prefs.Custom = req.Preferences.Custom
		}
	}

	if err := s.store.UserPreferences().Upsert(ctx, prefs); err != nil {
		return nil, MapDomainError(err)
	}

	return &services.UpdateUserPreferencesResponse{
		Preferences: storagePrefsToProto(prefs),
	}, nil
}

// ListUserSessions lists sessions for a user.
func (s *UserServiceServer) ListUserSessions(ctx context.Context, req *services.ListUserSessionsRequest) (*services.ListUserSessionsResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	var userID domain.UserID
	if req.UserId != "" {
		userID = domain.UserID(req.UserId)
	} else {
		user, ok := UserFromContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "not authenticated")
		}
		userID = user.ID
	}

	sessions, err := s.store.Sessions().GetByUser(ctx, userID)
	if err != nil {
		return nil, MapDomainError(err)
	}

	protoSessions := make([]*services.Session, len(sessions))
	for i, sess := range sessions {
		protoSessions[i] = sessionToProtoSession(sess)
	}

	return &services.ListUserSessionsResponse{
		Sessions: protoSessions,
	}, nil
}

// EndUserSession ends a specific session.
func (s *UserServiceServer) EndUserSession(ctx context.Context, req *services.EndUserSessionRequest) (*services.EndUserSessionResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.SessionId == "" {
		return nil, NewValidationError("session_id is required", map[string]string{
			"session_id": "must not be empty",
		})
	}

	if err := s.store.Sessions().End(ctx, req.SessionId); err != nil {
		return nil, MapDomainError(err)
	}

	// Audit log
	if s.auditLogger != nil {
		s.auditLogger.LogMutation(ctx, "DELETE", "session", req.SessionId, "Ended session")
	}

	return &services.EndUserSessionResponse{
		Success: true,
	}, nil
}

// EndAllUserSessions ends all sessions for a user.
func (s *UserServiceServer) EndAllUserSessions(ctx context.Context, req *services.EndAllUserSessionsRequest) (*services.EndAllUserSessionsResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.UserId == "" {
		return nil, NewValidationError("user_id is required", map[string]string{
			"user_id": "must not be empty",
		})
	}

	if err := s.store.Sessions().EndAllForUser(ctx, domain.UserID(req.UserId)); err != nil {
		return nil, MapDomainError(err)
	}

	// Audit log
	if s.auditLogger != nil {
		s.auditLogger.LogMutation(ctx, "DELETE", "session", req.UserId, "Ended all sessions for user")
	}

	return &services.EndAllUserSessionsResponse{
		EndedCount: 0, // TODO: Get actual count from session repository
	}, nil
}

// =============================================================================
// Conversion helpers
// =============================================================================

func userToProtoUser(u *domain.User) *services.User {
	if u == nil {
		return nil
	}

	proto := &services.User{
		Id:                   string(u.ID),
		PublicKey:            u.PublicKey,
		KeyType:              string(u.KeyType),
		PublicKeyFingerprint: u.PublicKeyFingerprint,
		Name:                 u.Name,
		Email:                u.Email,
		Status:               domainUserStatusToProto(u.Status),
		Role:                 domainUserRoleToProto(u.Role),
		Locale:               u.Locale,
		CreatedAt:            timestamppb.New(u.CreatedAt),
		UpdatedAt:            timestamppb.New(u.UpdatedAt),
		Metadata:             u.Metadata,
	}

	if u.LastLoginAt != nil {
		proto.LastLoginAt = timestamppb.New(*u.LastLoginAt)
	}

	return proto
}

func domainUserStatusToProto(s domain.UserStatus) services.UserStatus {
	switch s {
	case domain.UserStatusActive:
		return services.UserStatus_USER_STATUS_ACTIVE
	case domain.UserStatusPending:
		return services.UserStatus_USER_STATUS_PENDING
	case domain.UserStatusSuspended:
		return services.UserStatus_USER_STATUS_SUSPENDED
	case domain.UserStatusDeleted:
		return services.UserStatus_USER_STATUS_DELETED
	default:
		return services.UserStatus_USER_STATUS_UNSPECIFIED
	}
}

func protoUserStatusToDomain(s services.UserStatus) domain.UserStatus {
	switch s {
	case services.UserStatus_USER_STATUS_ACTIVE:
		return domain.UserStatusActive
	case services.UserStatus_USER_STATUS_PENDING:
		return domain.UserStatusPending
	case services.UserStatus_USER_STATUS_SUSPENDED:
		return domain.UserStatusSuspended
	case services.UserStatus_USER_STATUS_DELETED:
		return domain.UserStatusDeleted
	default:
		return ""
	}
}

func domainUserRoleToProto(r domain.UserRole) services.UserRole {
	switch r {
	case domain.UserRoleAdmin:
		return services.UserRole_USER_ROLE_ADMIN
	case domain.UserRoleUser:
		return services.UserRole_USER_ROLE_USER
	case domain.UserRoleReadonly:
		return services.UserRole_USER_ROLE_READONLY
	default:
		return services.UserRole_USER_ROLE_UNSPECIFIED
	}
}

func protoUserRoleToDomain(r services.UserRole) domain.UserRole {
	switch r {
	case services.UserRole_USER_ROLE_ADMIN:
		return domain.UserRoleAdmin
	case services.UserRole_USER_ROLE_USER:
		return domain.UserRoleUser
	case services.UserRole_USER_ROLE_READONLY:
		return domain.UserRoleReadonly
	default:
		return ""
	}
}

func storagePrefsToProto(p *storage.UserPreferences) *services.UserPreferences {
	if p == nil {
		return nil
	}

	return &services.UserPreferences{
		UserId:               string(p.UserID),
		Theme:                p.Theme,
		Locale:               p.Locale,
		Timezone:             p.Timezone,
		DateFormat:           p.DateFormat,
		NotificationsEnabled: p.NotificationsEnabled,
		EmailNotifications:   p.EmailNotifications,
		Custom:               p.Custom,
	}
}

func sessionToProtoSession(s *storage.Session) *services.Session {
	if s == nil {
		return nil
	}

	proto := &services.Session{
		Id:                   s.ID,
		UserId:               string(s.UserID),
		Type:                 storageSessionTypeToProto(s.Type),
		ClientIp:             s.ClientIP,
		ClientAgent:          s.ClientAgent,
		PublicKeyFingerprint: s.PublicKeyFingerprint,
		NodeId:               s.NodeID,
		StartedAt:            timestamppb.New(s.StartedAt),
		LastActivityAt:       timestamppb.New(s.LastActivityAt),
		Metadata:             s.Metadata,
	}

	if s.EndedAt != nil {
		proto.EndedAt = timestamppb.New(*s.EndedAt)
	}

	return proto
}

func storageSessionTypeToProto(t storage.SessionType) services.SessionType {
	switch t {
	case storage.SessionTypeSSH:
		return services.SessionType_SESSION_TYPE_SSH
	case storage.SessionTypeGRPC:
		return services.SessionType_SESSION_TYPE_GRPC
	case storage.SessionTypeAPI:
		return services.SessionType_SESSION_TYPE_API
	default:
		return services.SessionType_SESSION_TYPE_UNSPECIFIED
	}
}
