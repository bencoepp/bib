// Package breakglass provides controlled emergency access to the database
// for disaster recovery and debugging scenarios. It maintains a zero-trust
// security model with comprehensive audit logging.
package breakglass

import (
	"crypto/ed25519"
	"time"
)

// AccessLevel defines the permission level for break glass sessions.
type AccessLevel string

const (
	// AccessReadOnly allows SELECT operations only.
	AccessReadOnly AccessLevel = "readonly"

	// AccessReadWrite allows SELECT, INSERT, UPDATE, DELETE operations.
	// Note: audit_log table is always off-limits regardless of access level.
	AccessReadWrite AccessLevel = "readwrite"
)

// String returns the access level as a string.
func (a AccessLevel) String() string {
	return string(a)
}

// IsValid checks if the access level is valid.
func (a AccessLevel) IsValid() bool {
	switch a {
	case AccessReadOnly, AccessReadWrite:
		return true
	default:
		return false
	}
}

// SessionState represents the current state of a break glass session.
type SessionState string

const (
	// StateInactive means no break glass session is active.
	StateInactive SessionState = "inactive"

	// StateActive means a break glass session is currently active.
	StateActive SessionState = "active"

	// StateExpired means the session has expired and awaits acknowledgment.
	StateExpired SessionState = "expired"

	// StatePendingAck means session ended but requires admin acknowledgment.
	StatePendingAck SessionState = "pending_acknowledgment"
)

// User represents a pre-configured break glass user.
type User struct {
	// Name is the username for the emergency user.
	Name string `json:"name"`

	// PublicKey is the parsed Ed25519 public key.
	PublicKey ed25519.PublicKey `json:"-"`

	// PublicKeyString is the original SSH public key string.
	PublicKeyString string `json:"public_key"`

	// AccessLevel is the user's access level (empty = use default).
	AccessLevel AccessLevel `json:"access_level,omitempty"`
}

// Session represents an active break glass session.
type Session struct {
	// ID is the unique session identifier.
	ID string `json:"id"`

	// User is the authenticated user for this session.
	User *User `json:"user"`

	// Reason is the stated reason for the break glass session.
	Reason string `json:"reason"`

	// AccessLevel is the access level granted for this session.
	AccessLevel AccessLevel `json:"access_level"`

	// StartedAt is when the session started.
	StartedAt time.Time `json:"started_at"`

	// ExpiresAt is when the session will automatically expire.
	ExpiresAt time.Time `json:"expires_at"`

	// NodeID is the node where the session is active.
	NodeID string `json:"node_id"`

	// RequestedBy is who initiated the break glass request.
	RequestedBy string `json:"requested_by"`

	// ConnectionString is the PostgreSQL connection string for this session.
	// This is populated after the session is enabled.
	ConnectionString string `json:"connection_string,omitempty"`

	// DBUsername is the temporary database username created for this session.
	DBUsername string `json:"db_username,omitempty"`

	// State is the current session state.
	State SessionState `json:"state"`

	// RecordingPath is the path to the session recording file (if enabled).
	RecordingPath string `json:"recording_path,omitempty"`
}

// IsActive returns true if the session is currently active.
func (s *Session) IsActive() bool {
	return s.State == StateActive && time.Now().Before(s.ExpiresAt)
}

// RemainingDuration returns the remaining duration until expiry.
func (s *Session) RemainingDuration() time.Duration {
	if s.State != StateActive {
		return 0
	}
	remaining := time.Until(s.ExpiresAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Challenge represents an authentication challenge for break glass access.
type Challenge struct {
	// ID is the unique challenge identifier.
	ID string `json:"id"`

	// Nonce is the random bytes to be signed.
	Nonce []byte `json:"nonce"`

	// CreatedAt is when the challenge was created.
	CreatedAt time.Time `json:"created_at"`

	// ExpiresAt is when the challenge expires.
	ExpiresAt time.Time `json:"expires_at"`

	// Username is the user attempting authentication.
	Username string `json:"username"`
}

// IsExpired returns true if the challenge has expired.
func (c *Challenge) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// SessionReport contains the summary of a completed break glass session.
type SessionReport struct {
	// Session is the session that was completed.
	Session *Session `json:"session"`

	// EndedAt is when the session ended.
	EndedAt time.Time `json:"ended_at"`

	// Duration is the total session duration.
	Duration time.Duration `json:"duration"`

	// QueryCount is the number of queries executed during the session.
	QueryCount int64 `json:"query_count"`

	// TablesAccessed lists tables that were accessed.
	TablesAccessed []string `json:"tables_accessed"`

	// OperationCounts is a breakdown by operation type.
	OperationCounts map[string]int64 `json:"operation_counts"`

	// AcknowledgedAt is when the session was acknowledged (if required).
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`

	// AcknowledgedBy is who acknowledged the session.
	AcknowledgedBy string `json:"acknowledged_by,omitempty"`

	// RecordingPath is the path to the session recording.
	RecordingPath string `json:"recording_path,omitempty"`
}

// NotificationType represents the type of break glass notification.
type NotificationType string

const (
	// NotifySessionStarted is sent when a break glass session starts.
	NotifySessionStarted NotificationType = "session_started"

	// NotifySessionExpired is sent when a session expires.
	NotifySessionExpired NotificationType = "session_expired"

	// NotifySessionDisabled is sent when a session is manually disabled.
	NotifySessionDisabled NotificationType = "session_disabled"

	// NotifySessionAcknowledged is sent when a session is acknowledged.
	NotifySessionAcknowledged NotificationType = "session_acknowledged"
)

// Notification contains details for a break glass notification.
type Notification struct {
	// Type is the notification type.
	Type NotificationType `json:"type"`

	// Timestamp is when the notification was generated.
	Timestamp time.Time `json:"timestamp"`

	// Session contains session details.
	Session *Session `json:"session"`

	// NodeID is the node where this occurred.
	NodeID string `json:"node_id"`

	// IPAddress is the IP address of the requester (if available).
	IPAddress string `json:"ip_address,omitempty"`

	// AdditionalInfo contains any extra context.
	AdditionalInfo map[string]string `json:"additional_info,omitempty"`
}

// Config holds the break glass configuration (mirrors config.BreakGlassConfig).
type Config struct {
	Enabled               bool          `mapstructure:"enabled"`
	RequireRestart        bool          `mapstructure:"require_restart"`
	MaxDuration           time.Duration `mapstructure:"max_duration"`
	DefaultAccessLevel    AccessLevel   `mapstructure:"default_access_level"`
	AllowedUsers          []User        `mapstructure:"allowed_users"`
	AuditLevel            string        `mapstructure:"audit_level"`
	RequireAcknowledgment bool          `mapstructure:"require_acknowledgment"`
	SessionRecording      bool          `mapstructure:"session_recording"`
	RecordingPath         string        `mapstructure:"recording_path"`
	WebhookURL            string        `mapstructure:"webhook_url"`
	EmailAddress          string        `mapstructure:"email_address"`
}
