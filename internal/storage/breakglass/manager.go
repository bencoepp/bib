package breakglass

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

// Manager handles break glass session lifecycle.
// All session state is kept in memory and does not survive restarts.
type Manager struct {
	config Config
	nodeID string

	// Current session (nil if no active session)
	session *Session

	// Pending challenges for authentication
	challenges map[string]*Challenge

	// Completed sessions pending acknowledgment
	pendingReports []*SessionReport

	// Callback for database operations
	dbCallback DatabaseCallback

	// Callback for notifications
	notifyCallback NotifyCallback

	// Callback for audit logging
	auditCallback AuditCallback

	mu sync.RWMutex
}

// DatabaseCallback handles database operations for break glass sessions.
type DatabaseCallback interface {
	// CreateBreakGlassUser creates a temporary PostgreSQL user for the session.
	CreateBreakGlassUser(ctx context.Context, username, password string, accessLevel AccessLevel) error

	// DropBreakGlassUser removes the temporary PostgreSQL user.
	DropBreakGlassUser(ctx context.Context, username string) error

	// GetConnectionString returns the connection string for the break glass user.
	GetConnectionString(username, password string) string
}

// NotifyCallback handles break glass notifications.
type NotifyCallback interface {
	// SendNotification sends a notification for a break glass event.
	SendNotification(ctx context.Context, notification *Notification) error
}

// AuditCallback handles audit logging for break glass operations.
type AuditCallback interface {
	// LogBreakGlassEvent logs a break glass event to the audit trail.
	LogBreakGlassEvent(ctx context.Context, eventType string, session *Session, metadata map[string]any) error

	// GetSessionQueryStats returns query statistics for a session.
	GetSessionQueryStats(ctx context.Context, sessionID string) (queryCount int64, tablesAccessed []string, operationCounts map[string]int64, err error)
}

// NewManager creates a new break glass manager.
func NewManager(config Config, nodeID string) *Manager {
	return &Manager{
		config:         config,
		nodeID:         nodeID,
		challenges:     make(map[string]*Challenge),
		pendingReports: make([]*SessionReport, 0),
	}
}

// SetDatabaseCallback sets the database callback.
func (m *Manager) SetDatabaseCallback(cb DatabaseCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dbCallback = cb
}

// SetNotifyCallback sets the notification callback.
func (m *Manager) SetNotifyCallback(cb NotifyCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifyCallback = cb
}

// SetAuditCallback sets the audit callback.
func (m *Manager) SetAuditCallback(cb AuditCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.auditCallback = cb
}

// IsEnabled returns whether break glass is enabled in configuration.
func (m *Manager) IsEnabled() bool {
	return m.config.Enabled
}

// HasActiveSession returns whether there is currently an active session.
func (m *Manager) HasActiveSession() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.session != nil && m.session.IsActive()
}

// GetSession returns the current session (may be nil).
func (m *Manager) GetSession() *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.session == nil {
		return nil
	}
	// Return a copy to prevent mutation
	sessionCopy := *m.session
	return &sessionCopy
}

// GetPendingReports returns sessions that are pending acknowledgment.
func (m *Manager) GetPendingReports() []*SessionReport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*SessionReport, len(m.pendingReports))
	copy(result, m.pendingReports)
	return result
}

// CreateChallenge creates an authentication challenge for a user.
func (m *Manager) CreateChallenge(username string) (*Challenge, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		return nil, fmt.Errorf("break glass is not enabled")
	}

	// Verify user exists
	user := m.findUser(username)
	if user == nil {
		return nil, fmt.Errorf("user not found: %s", username)
	}

	// Generate random nonce
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	challenge := &Challenge{
		ID:        uuid.New().String(),
		Nonce:     nonce,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(5 * time.Minute), // 5 minute expiry
		Username:  username,
	}

	m.challenges[challenge.ID] = challenge

	// Clean up expired challenges
	m.cleanupChallenges()

	return challenge, nil
}

// VerifyChallenge verifies a signed challenge response.
func (m *Manager) VerifyChallenge(challengeID string, signature []byte) (*User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	challenge, ok := m.challenges[challengeID]
	if !ok {
		return nil, fmt.Errorf("challenge not found")
	}

	if challenge.IsExpired() {
		delete(m.challenges, challengeID)
		return nil, fmt.Errorf("challenge expired")
	}

	user := m.findUser(challenge.Username)
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Verify the signature
	if !ed25519.Verify(user.PublicKey, challenge.Nonce, signature) {
		return nil, fmt.Errorf("invalid signature")
	}

	// Remove used challenge
	delete(m.challenges, challengeID)

	return user, nil
}

// Enable enables a break glass session after successful authentication.
func (m *Manager) Enable(ctx context.Context, user *User, reason string, duration time.Duration, requestedBy string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		return nil, fmt.Errorf("break glass is not enabled")
	}

	if m.session != nil && m.session.IsActive() {
		return nil, fmt.Errorf("a break glass session is already active")
	}

	// Validate duration
	if duration > m.config.MaxDuration {
		return nil, fmt.Errorf("requested duration %v exceeds maximum %v", duration, m.config.MaxDuration)
	}

	if duration <= 0 {
		duration = m.config.MaxDuration
	}

	// Determine access level
	accessLevel := m.config.DefaultAccessLevel
	if user.AccessLevel.IsValid() {
		accessLevel = user.AccessLevel
	}

	// Generate temporary credentials
	sessionID := uuid.New().String()
	dbUsername := fmt.Sprintf("breakglass_%s", strings.ReplaceAll(sessionID[:8], "-", ""))
	dbPassword := generatePassword(32)

	// Create the session
	session := &Session{
		ID:          sessionID,
		User:        user,
		Reason:      reason,
		AccessLevel: accessLevel,
		StartedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(duration),
		NodeID:      m.nodeID,
		RequestedBy: requestedBy,
		DBUsername:  dbUsername,
		State:       StateActive,
	}

	// Create database user if callback is set
	if m.dbCallback != nil {
		if err := m.dbCallback.CreateBreakGlassUser(ctx, dbUsername, dbPassword, accessLevel); err != nil {
			return nil, fmt.Errorf("failed to create break glass user: %w", err)
		}
		session.ConnectionString = m.dbCallback.GetConnectionString(dbUsername, dbPassword)
	}

	// Set up session recording path if enabled
	if m.config.SessionRecording {
		recordingPath := m.config.RecordingPath
		if recordingPath == "" {
			recordingPath = "."
		}
		session.RecordingPath = fmt.Sprintf("%s/breakglass_%s.rec", recordingPath, sessionID[:8])
	}

	m.session = session

	// Start expiry monitor
	go m.monitorExpiry(sessionID)

	// Log the event
	if m.auditCallback != nil {
		_ = m.auditCallback.LogBreakGlassEvent(ctx, "session_started", session, map[string]any{
			"reason":       reason,
			"duration":     duration.String(),
			"access_level": accessLevel.String(),
			"requested_by": requestedBy,
		})
	}

	// Send notification
	if m.notifyCallback != nil {
		notification := &Notification{
			Type:      NotifySessionStarted,
			Timestamp: time.Now(),
			Session:   session,
			NodeID:    m.nodeID,
			AdditionalInfo: map[string]string{
				"reason":       reason,
				"duration":     duration.String(),
				"access_level": accessLevel.String(),
			},
		}
		_ = m.notifyCallback.SendNotification(ctx, notification)
	}

	return session, nil
}

// Disable manually disables an active break glass session.
func (m *Manager) Disable(ctx context.Context, disabledBy string) (*SessionReport, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.session == nil {
		return nil, fmt.Errorf("no active break glass session")
	}

	return m.endSession(ctx, disabledBy, false)
}

// Acknowledge acknowledges a completed break glass session.
func (m *Manager) Acknowledge(ctx context.Context, sessionID string, acknowledgedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find the pending report
	var report *SessionReport
	var idx int
	for i, r := range m.pendingReports {
		if r.Session.ID == sessionID {
			report = r
			idx = i
			break
		}
	}

	if report == nil {
		return fmt.Errorf("session not found or not pending acknowledgment: %s", sessionID)
	}

	// Mark as acknowledged
	now := time.Now()
	report.AcknowledgedAt = &now
	report.AcknowledgedBy = acknowledgedBy

	// Remove from pending
	m.pendingReports = append(m.pendingReports[:idx], m.pendingReports[idx+1:]...)

	// Log the acknowledgment
	if m.auditCallback != nil {
		_ = m.auditCallback.LogBreakGlassEvent(ctx, "session_acknowledged", report.Session, map[string]any{
			"acknowledged_by": acknowledgedBy,
			"acknowledged_at": now.Format(time.RFC3339),
		})
	}

	// Send notification
	if m.notifyCallback != nil {
		notification := &Notification{
			Type:      NotifySessionAcknowledged,
			Timestamp: time.Now(),
			Session:   report.Session,
			NodeID:    m.nodeID,
			AdditionalInfo: map[string]string{
				"acknowledged_by": acknowledgedBy,
			},
		}
		_ = m.notifyCallback.SendNotification(ctx, notification)
	}

	return nil
}

// GetReport returns the report for a specific session.
func (m *Manager) GetReport(sessionID string) (*SessionReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, r := range m.pendingReports {
		if r.Session.ID == sessionID {
			return r, nil
		}
	}

	return nil, fmt.Errorf("session report not found: %s", sessionID)
}

// endSession ends the current session and creates a report.
// Must be called with lock held.
func (m *Manager) endSession(ctx context.Context, endedBy string, expired bool) (*SessionReport, error) {
	session := m.session
	if session == nil {
		return nil, fmt.Errorf("no active session")
	}

	now := time.Now()

	// Update session state
	if expired {
		session.State = StateExpired
	} else {
		session.State = StatePendingAck
	}

	// Drop the database user
	if m.dbCallback != nil && session.DBUsername != "" {
		if err := m.dbCallback.DropBreakGlassUser(ctx, session.DBUsername); err != nil {
			// Log but don't fail - the session is ending either way
			// The credential will be invalid anyway
			_ = err
		}
	}

	// Get query stats if audit callback is available
	var queryCount int64
	var tablesAccessed []string
	var operationCounts map[string]int64
	if m.auditCallback != nil {
		var err error
		queryCount, tablesAccessed, operationCounts, err = m.auditCallback.GetSessionQueryStats(ctx, session.ID)
		if err != nil {
			// Non-fatal, continue with zeros
			queryCount = 0
			tablesAccessed = []string{}
			operationCounts = map[string]int64{}
		}
	}

	// Create the report
	report := &SessionReport{
		Session:         session,
		EndedAt:         now,
		Duration:        now.Sub(session.StartedAt),
		QueryCount:      queryCount,
		TablesAccessed:  tablesAccessed,
		OperationCounts: operationCounts,
		RecordingPath:   session.RecordingPath,
	}

	// Add to pending reports if acknowledgment is required
	if m.config.RequireAcknowledgment {
		session.State = StatePendingAck
		m.pendingReports = append(m.pendingReports, report)
	}

	// Clear the active session
	m.session = nil

	// Log the event
	eventType := "session_disabled"
	if expired {
		eventType = "session_expired"
	}
	if m.auditCallback != nil {
		_ = m.auditCallback.LogBreakGlassEvent(ctx, eventType, session, map[string]any{
			"ended_by":         endedBy,
			"duration":         report.Duration.String(),
			"query_count":      queryCount,
			"tables_accessed":  tablesAccessed,
			"operation_counts": operationCounts,
		})
	}

	// Send notification
	if m.notifyCallback != nil {
		notifyType := NotifySessionDisabled
		if expired {
			notifyType = NotifySessionExpired
		}
		notification := &Notification{
			Type:      notifyType,
			Timestamp: now,
			Session:   session,
			NodeID:    m.nodeID,
			AdditionalInfo: map[string]string{
				"ended_by":    endedBy,
				"duration":    report.Duration.String(),
				"query_count": fmt.Sprintf("%d", queryCount),
			},
		}
		_ = m.notifyCallback.SendNotification(ctx, notification)
	}

	return report, nil
}

// monitorExpiry monitors the session for expiry.
func (m *Manager) monitorExpiry(sessionID string) {
	m.mu.RLock()
	session := m.session
	m.mu.RUnlock()

	if session == nil || session.ID != sessionID {
		return
	}

	// Wait until expiry
	timer := time.NewTimer(time.Until(session.ExpiresAt))
	defer timer.Stop()

	<-timer.C

	// Check if this session is still active
	m.mu.Lock()
	if m.session != nil && m.session.ID == sessionID && m.session.State == StateActive {
		_, _ = m.endSession(context.Background(), "system:expiry", true)
	}
	m.mu.Unlock()
}

// cleanupChallenges removes expired challenges.
// Must be called with lock held.
func (m *Manager) cleanupChallenges() {
	for id, challenge := range m.challenges {
		if challenge.IsExpired() {
			delete(m.challenges, id)
		}
	}
}

// findUser finds a user by username.
// Must be called with lock held.
func (m *Manager) findUser(username string) *User {
	for i := range m.config.AllowedUsers {
		if m.config.AllowedUsers[i].Name == username {
			return &m.config.AllowedUsers[i]
		}
	}
	return nil
}

// ParseSSHPublicKey parses an SSH public key string into an ed25519.PublicKey.
func ParseSSHPublicKey(keyStr string) (ed25519.PublicKey, error) {
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(keyStr))
	if err != nil {
		return nil, fmt.Errorf("failed to parse SSH public key: %w", err)
	}

	// Convert to crypto public key
	cryptoPubKey := pubKey.(ssh.CryptoPublicKey).CryptoPublicKey()

	ed25519Key, ok := cryptoPubKey.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not Ed25519")
	}

	return ed25519Key, nil
}

// generatePassword generates a random password.
func generatePassword(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to a less secure but still random string
		return uuid.New().String() + uuid.New().String()
	}
	return base64.RawURLEncoding.EncodeToString(bytes)[:length]
}
