// Package auth provides user authentication and session management for bibd.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"bib/internal/config"
	"bib/internal/domain"
	"bib/internal/storage"
)

// Service handles user authentication and session management.
type Service struct {
	store  storage.Store
	cfg    config.AuthConfig
	nodeID string
}

// NewService creates a new authentication service.
func NewService(store storage.Store, cfg config.AuthConfig, nodeID string) *Service {
	return &Service{
		store:  store,
		cfg:    cfg,
		nodeID: nodeID,
	}
}

// AuthenticateResult contains the result of an authentication attempt.
type AuthenticateResult struct {
	User    *domain.User
	Session *storage.Session
	IsNew   bool // True if user was auto-registered
}

// Authenticate authenticates a user by their public key.
// If auto-registration is enabled and the user doesn't exist, creates a new user.
// Creates a new session for the authenticated user.
func (s *Service) Authenticate(ctx context.Context, req AuthenticateRequest) (*AuthenticateResult, error) {
	// Try to find existing user
	user, err := s.store.Users().GetByPublicKey(ctx, req.PublicKey)
	if err != nil && err != domain.ErrUserNotFound {
		return nil, fmt.Errorf("failed to lookup user: %w", err)
	}

	isNew := false

	if user == nil {
		// User not found - check if auto-registration is allowed
		if !s.cfg.AllowAutoRegistration {
			return nil, domain.ErrAutoRegDisabled
		}

		// Check if email is required
		if s.cfg.RequireEmail && req.Email == "" {
			return nil, fmt.Errorf("email is required for registration")
		}

		// Check if this is the first user (will be admin)
		isFirstUser, err := s.store.Users().IsFirstUser(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to check first user: %w", err)
		}

		// Create new user
		user = domain.NewUser(req.PublicKey, req.KeyType, req.Name, req.Email, isFirstUser)

		// Apply default role if not first user
		if !isFirstUser && s.cfg.DefaultRole != "" {
			role := domain.UserRole(s.cfg.DefaultRole)
			if role.IsValid() {
				user.Role = role
			}
		}

		// Set locale if provided
		if req.Locale != "" {
			user.Locale = req.Locale
		}

		if err := s.store.Users().Create(ctx, user); err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}

		isNew = true
	}

	// Check user status
	if err := s.checkUserStatus(user); err != nil {
		return nil, err
	}

	// Update last login
	now := time.Now().UTC()
	user.LastLoginAt = &now
	if err := s.store.Users().Update(ctx, user); err != nil {
		// Non-fatal, just log
	}

	// Check session limits
	if s.cfg.MaxSessionsPerUser > 0 {
		activeSessions, err := s.store.Sessions().GetByUser(ctx, user.ID)
		if err == nil && len(activeSessions) >= s.cfg.MaxSessionsPerUser {
			// End oldest session
			if len(activeSessions) > 0 {
				oldest := activeSessions[len(activeSessions)-1]
				_ = s.store.Sessions().End(ctx, oldest.ID)
			}
		}
	}

	// Create session
	session := &storage.Session{
		ID:                   generateSessionID(),
		UserID:               user.ID,
		Type:                 req.SessionType,
		ClientIP:             req.ClientIP,
		ClientAgent:          req.ClientAgent,
		PublicKeyFingerprint: user.PublicKeyFingerprint,
		NodeID:               s.nodeID,
		StartedAt:            now,
		LastActivityAt:       now,
		Metadata:             req.Metadata,
	}

	if err := s.store.Sessions().Create(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &AuthenticateResult{
		User:    user,
		Session: session,
		IsNew:   isNew,
	}, nil
}

// AuthenticateRequest contains the data needed to authenticate a user.
type AuthenticateRequest struct {
	PublicKey   []byte
	KeyType     domain.KeyType
	Name        string // Used for auto-registration
	Email       string // Used for auto-registration
	Locale      string // User's preferred locale
	SessionType storage.SessionType
	ClientIP    string
	ClientAgent string
	Metadata    map[string]string
}

// checkUserStatus verifies the user can authenticate.
func (s *Service) checkUserStatus(user *domain.User) error {
	switch user.Status {
	case domain.UserStatusActive:
		return nil
	case domain.UserStatusPending:
		return domain.ErrUserPending
	case domain.UserStatusSuspended:
		return domain.ErrUserSuspended
	case domain.UserStatusDeleted:
		return domain.ErrUserNotFound
	default:
		return domain.ErrUnauthorized
	}
}

// EndSession ends a user session.
func (s *Service) EndSession(ctx context.Context, sessionID string) error {
	return s.store.Sessions().End(ctx, sessionID)
}

// EndAllSessions ends all sessions for a user.
func (s *Service) EndAllSessions(ctx context.Context, userID domain.UserID) error {
	return s.store.Sessions().EndAllForUser(ctx, userID)
}

// ListUserSessions lists all sessions for a user.
func (s *Service) ListUserSessions(ctx context.Context, userID domain.UserID) ([]*storage.Session, error) {
	return s.store.Sessions().GetByUser(ctx, userID)
}

// GetSession retrieves a session by ID.
func (s *Service) GetSession(ctx context.Context, sessionID string) (*storage.Session, error) {
	session, err := s.store.Sessions().Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Check if session is expired
	if s.cfg.SessionTimeout > 0 {
		if time.Since(session.LastActivityAt) > s.cfg.SessionTimeout {
			// End expired session
			_ = s.store.Sessions().End(ctx, sessionID)
			return nil, domain.ErrSessionExpired
		}
	}

	return session, nil
}

// UpdateSessionActivity updates the last activity time for a session.
func (s *Service) UpdateSessionActivity(ctx context.Context, sessionID string) error {
	session, err := s.store.Sessions().Get(ctx, sessionID)
	if err != nil {
		return err
	}

	session.LastActivityAt = time.Now().UTC()
	return s.store.Sessions().Update(ctx, session)
}

// GetUser retrieves a user by ID.
func (s *Service) GetUser(ctx context.Context, userID domain.UserID) (*domain.User, error) {
	return s.store.Users().Get(ctx, userID)
}

// GetUserByPublicKey retrieves a user by their public key.
func (s *Service) GetUserByPublicKey(ctx context.Context, publicKey []byte) (*domain.User, error) {
	return s.store.Users().GetByPublicKey(ctx, publicKey)
}

// ListUsers lists users with optional filtering.
func (s *Service) ListUsers(ctx context.Context, filter storage.UserFilter) ([]*domain.User, error) {
	return s.store.Users().List(ctx, filter)
}

// UpdateUser updates a user's profile.
func (s *Service) UpdateUser(ctx context.Context, user *domain.User) error {
	return s.store.Users().Update(ctx, user)
}

// SuspendUser suspends a user account.
func (s *Service) SuspendUser(ctx context.Context, userID domain.UserID) error {
	user, err := s.store.Users().Get(ctx, userID)
	if err != nil {
		return err
	}

	user.Status = domain.UserStatusSuspended
	if err := s.store.Users().Update(ctx, user); err != nil {
		return err
	}

	// End all sessions
	return s.store.Sessions().EndAllForUser(ctx, userID)
}

// ActivateUser activates a pending or suspended user.
func (s *Service) ActivateUser(ctx context.Context, userID domain.UserID) error {
	user, err := s.store.Users().Get(ctx, userID)
	if err != nil {
		return err
	}

	user.Status = domain.UserStatusActive
	return s.store.Users().Update(ctx, user)
}

// DeleteUser soft-deletes a user.
func (s *Service) DeleteUser(ctx context.Context, userID domain.UserID) error {
	// End all sessions first
	if err := s.store.Sessions().EndAllForUser(ctx, userID); err != nil {
		return err
	}

	return s.store.Users().Delete(ctx, userID)
}

// CreateUser creates a new user (admin function).
func (s *Service) CreateUser(ctx context.Context, user *domain.User) error {
	// Check if this would be the first user
	isFirstUser, err := s.store.Users().IsFirstUser(ctx)
	if err != nil {
		return fmt.Errorf("failed to check first user: %w", err)
	}

	// First user is always admin
	if isFirstUser {
		user.Role = domain.UserRoleAdmin
	}

	// Ensure required fields
	if user.ID == "" {
		user.ID = domain.UserIDFromPublicKey(user.PublicKey)
	}
	if user.PublicKeyFingerprint == "" {
		user.PublicKeyFingerprint = domain.PublicKeyFingerprint(user.PublicKey)
	}
	if user.Status == "" {
		user.Status = domain.UserStatusActive
	}
	if user.Role == "" {
		user.Role = domain.UserRoleUser
	}

	now := time.Now().UTC()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	user.UpdatedAt = now

	return s.store.Users().Create(ctx, user)
}

// SetUserRole changes a user's role.
func (s *Service) SetUserRole(ctx context.Context, userID domain.UserID, role domain.UserRole) error {
	if !role.IsValid() {
		return domain.ErrInvalidUserRole
	}

	user, err := s.store.Users().Get(ctx, userID)
	if err != nil {
		return err
	}

	user.Role = role
	return s.store.Users().Update(ctx, user)
}

// CleanupExpiredSessions removes sessions that haven't been active recently.
func (s *Service) CleanupExpiredSessions(ctx context.Context) (int64, error) {
	if s.cfg.SessionTimeout == 0 {
		return 0, nil
	}

	cutoff := time.Now().Add(-s.cfg.SessionTimeout * 2) // Keep some buffer
	return s.store.Sessions().Cleanup(ctx, cutoff)
}

// generateSessionID generates a random session ID.
func generateSessionID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID
		return fmt.Sprintf("session-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
