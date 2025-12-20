// Package user implements the UserService gRPC service.
package user

import (
	"context"

	bibv1 "bib/api/gen/go/bib/v1"
	services "bib/api/gen/go/bib/v1/services"
	"bib/internal/domain"
	grpcerrors "bib/internal/grpc/errors"
	"bib/internal/grpc/interfaces"
	"bib/internal/grpc/middleware"
	"bib/internal/storage"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Config holds configuration for the user service server.
type Config struct {
	Store       storage.Store
	AuditLogger interfaces.AuditLogger
}

// Server implements the UserService gRPC service.
type Server struct {
	services.UnimplementedUserServiceServer
	store       storage.Store
	auditLogger interfaces.AuditLogger
}

// NewServer creates a new user service server.
func NewServer() *Server {
	return &Server{}
}

// NewServerWithConfig creates a new user service server with dependencies.
func NewServerWithConfig(cfg Config) *Server {
	return &Server{
		store:       cfg.Store,
		auditLogger: cfg.AuditLogger,
	}
}

// GetUser retrieves a user by ID.
func (s *Server) GetUser(ctx context.Context, req *services.GetUserRequest) (*services.GetUserResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.UserId == "" {
		return nil, grpcerrors.NewValidationError("user_id is required", map[string]string{
			"user_id": "must not be empty",
		})
	}

	user, err := s.store.Users().Get(ctx, domain.UserID(req.UserId))
	if err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	return &services.GetUserResponse{
		User: userToProto(user),
	}, nil
}

// GetUserByPublicKey retrieves a user by their public key.
func (s *Server) GetUserByPublicKey(ctx context.Context, req *services.GetUserByPublicKeyRequest) (*services.GetUserByPublicKeyResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if len(req.PublicKey) == 0 {
		return nil, grpcerrors.NewValidationError("public_key is required", map[string]string{
			"public_key": "must not be empty",
		})
	}

	user, err := s.store.Users().GetByPublicKey(ctx, req.PublicKey)
	if err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	return &services.GetUserByPublicKeyResponse{
		User: userToProto(user),
	}, nil
}

// ListUsers lists users with filtering and pagination.
func (s *Server) ListUsers(ctx context.Context, req *services.ListUsersRequest) (*services.ListUsersResponse, error) {
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

	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 1000 {
		filter.Limit = 1000
	}

	users, err := s.store.Users().List(ctx, filter)
	if err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	total, err := s.store.Users().Count(ctx, filter)
	if err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	protoUsers := make([]*services.User, len(users))
	for i, u := range users {
		protoUsers[i] = userToProto(u)
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
func (s *Server) SearchUsers(ctx context.Context, req *services.SearchUsersRequest) (*services.SearchUsersResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if err := grpcerrors.ValidateSearchQuery(req.Query); err != nil {
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
		return nil, grpcerrors.MapDomainError(err)
	}

	total, err := s.store.Users().Count(ctx, filter)
	if err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	protoUsers := make([]*services.User, len(users))
	for i, u := range users {
		protoUsers[i] = userToProto(u)
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
func (s *Server) CreateUser(ctx context.Context, req *services.CreateUserRequest) (*services.CreateUserResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

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
		return nil, grpcerrors.NewValidationError("invalid create user request", violations)
	}

	exists, err := s.store.Users().Exists(ctx, req.PublicKey)
	if err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}
	if exists {
		return nil, grpcerrors.MapDomainError(domain.ErrUserExists)
	}

	keyType := domain.KeyType(req.KeyType)
	user := domain.NewUser(req.PublicKey, keyType, req.Name, req.Email, false)

	if req.Role != services.UserRole_USER_ROLE_UNSPECIFIED {
		user.Role = protoUserRoleToDomain(req.Role)
	}

	if err := s.store.Users().Create(ctx, user); err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogServiceAction(ctx, "CREATE", "user", string(user.ID), map[string]interface{}{
			"name": user.Name,
		})
	}

	return &services.CreateUserResponse{
		User: userToProto(user),
	}, nil
}

// UpdateUser updates an existing user.
func (s *Server) UpdateUser(ctx context.Context, req *services.UpdateUserRequest) (*services.UpdateUserResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.UserId == "" {
		return nil, grpcerrors.NewValidationError("user_id is required", map[string]string{
			"user_id": "must not be empty",
		})
	}

	user, err := s.store.Users().Get(ctx, domain.UserID(req.UserId))
	if err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

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
		return nil, grpcerrors.MapDomainError(err)
	}

	if err := s.store.Users().Update(ctx, user); err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogServiceAction(ctx, "UPDATE", "user", string(user.ID), nil)
	}

	return &services.UpdateUserResponse{
		User: userToProto(user),
	}, nil
}

// DeleteUser soft-deletes a user.
func (s *Server) DeleteUser(ctx context.Context, req *services.DeleteUserRequest) (*services.DeleteUserResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.UserId == "" {
		return nil, grpcerrors.NewValidationError("user_id is required", map[string]string{
			"user_id": "must not be empty",
		})
	}

	if err := s.store.Users().Delete(ctx, domain.UserID(req.UserId)); err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	_ = s.store.Sessions().EndAllForUser(ctx, domain.UserID(req.UserId))

	if s.auditLogger != nil {
		_ = s.auditLogger.LogServiceAction(ctx, "DELETE", "user", req.UserId, nil)
	}

	return &services.DeleteUserResponse{
		Success: true,
	}, nil
}

// SuspendUser suspends a user account.
func (s *Server) SuspendUser(ctx context.Context, req *services.SuspendUserRequest) (*services.SuspendUserResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.UserId == "" {
		return nil, grpcerrors.NewValidationError("user_id is required", map[string]string{
			"user_id": "must not be empty",
		})
	}

	user, err := s.store.Users().Get(ctx, domain.UserID(req.UserId))
	if err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	user.Status = domain.UserStatusSuspended

	if err := s.store.Users().Update(ctx, user); err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	_ = s.store.Sessions().EndAllForUser(ctx, domain.UserID(req.UserId))

	if s.auditLogger != nil {
		_ = s.auditLogger.LogServiceAction(ctx, "UPDATE", "user", req.UserId, map[string]interface{}{
			"action": "suspend",
			"reason": req.Reason,
		})
	}

	return &services.SuspendUserResponse{
		User: userToProto(user),
	}, nil
}

// ActivateUser activates a pending or suspended user.
func (s *Server) ActivateUser(ctx context.Context, req *services.ActivateUserRequest) (*services.ActivateUserResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.UserId == "" {
		return nil, grpcerrors.NewValidationError("user_id is required", map[string]string{
			"user_id": "must not be empty",
		})
	}

	user, err := s.store.Users().Get(ctx, domain.UserID(req.UserId))
	if err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	user.Status = domain.UserStatusActive

	if err := s.store.Users().Update(ctx, user); err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogServiceAction(ctx, "UPDATE", "user", req.UserId, map[string]interface{}{
			"action": "activate",
		})
	}

	return &services.ActivateUserResponse{
		User: userToProto(user),
	}, nil
}

// SetUserRole changes a user's role.
func (s *Server) SetUserRole(ctx context.Context, req *services.SetUserRoleRequest) (*services.SetUserRoleResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.UserId == "" {
		return nil, grpcerrors.NewValidationError("user_id is required", map[string]string{
			"user_id": "must not be empty",
		})
	}

	if req.Role == services.UserRole_USER_ROLE_UNSPECIFIED {
		return nil, grpcerrors.NewValidationError("role is required", map[string]string{
			"role": "must be specified",
		})
	}

	user, err := s.store.Users().Get(ctx, domain.UserID(req.UserId))
	if err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	oldRole := user.Role
	user.Role = protoUserRoleToDomain(req.Role)

	if err := s.store.Users().Update(ctx, user); err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogServiceAction(ctx, "UPDATE", "user", req.UserId, map[string]interface{}{
			"action":   "set_role",
			"old_role": string(oldRole),
			"new_role": string(user.Role),
		})
	}

	return &services.SetUserRoleResponse{
		User: userToProto(user),
	}, nil
}

// GetCurrentUser retrieves the currently authenticated user.
func (s *Server) GetCurrentUser(ctx context.Context, _ *services.GetCurrentUserRequest) (*services.GetCurrentUserResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	user, ok := middleware.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	return &services.GetCurrentUserResponse{
		User: userToProto(user),
	}, nil
}

// UpdateCurrentUser updates the current user's profile.
func (s *Server) UpdateCurrentUser(ctx context.Context, req *services.UpdateCurrentUserRequest) (*services.UpdateCurrentUserResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	user, ok := middleware.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	user, err := s.store.Users().Get(ctx, user.ID)
	if err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

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
		return nil, grpcerrors.MapDomainError(err)
	}

	if err := s.store.Users().Update(ctx, user); err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	return &services.UpdateCurrentUserResponse{
		User: userToProto(user),
	}, nil
}

// GetUserPreferences retrieves user preferences.
func (s *Server) GetUserPreferences(ctx context.Context, req *services.GetUserPreferencesRequest) (*services.GetUserPreferencesResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	var userID domain.UserID
	if req.UserId != "" {
		userID = domain.UserID(req.UserId)
	} else {
		user, ok := middleware.UserFromContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "not authenticated")
		}
		userID = user.ID
	}

	prefs, err := s.store.UserPreferences().Get(ctx, userID)
	if err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	return &services.GetUserPreferencesResponse{
		Preferences: prefsToProto(prefs),
	}, nil
}

// UpdateUserPreferences updates user preferences.
func (s *Server) UpdateUserPreferences(ctx context.Context, req *services.UpdateUserPreferencesRequest) (*services.UpdateUserPreferencesResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	user, ok := middleware.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	prefs, err := s.store.UserPreferences().Get(ctx, user.ID)
	if err != nil {
		prefs = &storage.UserPreferences{
			UserID:               user.ID,
			Theme:                "system",
			Locale:               "en",
			Timezone:             "UTC",
			DateFormat:           "YYYY-MM-DD",
			NotificationsEnabled: true,
		}
	}

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
		return nil, grpcerrors.MapDomainError(err)
	}

	return &services.UpdateUserPreferencesResponse{
		Preferences: prefsToProto(prefs),
	}, nil
}

// ListUserSessions lists sessions for a user.
func (s *Server) ListUserSessions(ctx context.Context, req *services.ListUserSessionsRequest) (*services.ListUserSessionsResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	var userID domain.UserID
	if req.UserId != "" {
		userID = domain.UserID(req.UserId)
	} else {
		user, ok := middleware.UserFromContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "not authenticated")
		}
		userID = user.ID
	}

	sessions, err := s.store.Sessions().GetByUser(ctx, userID)
	if err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	protoSessions := make([]*services.Session, len(sessions))
	for i, sess := range sessions {
		protoSessions[i] = sessionToProto(sess)
	}

	return &services.ListUserSessionsResponse{
		Sessions: protoSessions,
	}, nil
}

// EndUserSession ends a specific session.
func (s *Server) EndUserSession(ctx context.Context, req *services.EndUserSessionRequest) (*services.EndUserSessionResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.SessionId == "" {
		return nil, grpcerrors.NewValidationError("session_id is required", map[string]string{
			"session_id": "must not be empty",
		})
	}

	if err := s.store.Sessions().End(ctx, req.SessionId); err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogServiceAction(ctx, "DELETE", "session", req.SessionId, nil)
	}

	return &services.EndUserSessionResponse{
		Success: true,
	}, nil
}

// EndAllUserSessions ends all sessions for a user.
func (s *Server) EndAllUserSessions(ctx context.Context, req *services.EndAllUserSessionsRequest) (*services.EndAllUserSessionsResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.UserId == "" {
		return nil, grpcerrors.NewValidationError("user_id is required", map[string]string{
			"user_id": "must not be empty",
		})
	}

	if err := s.store.Sessions().EndAllForUser(ctx, domain.UserID(req.UserId)); err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogServiceAction(ctx, "DELETE", "session", req.UserId, map[string]interface{}{
			"action": "end_all",
		})
	}

	return &services.EndAllUserSessionsResponse{
		EndedCount: 0,
	}, nil
}
