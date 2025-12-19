package storage

import (
	"context"
	"io"
	"time"

	"bib/internal/domain"
)

// Store is the main storage interface that provides access to all repositories.
// It abstracts the underlying database implementation (SQLite or PostgreSQL).
type Store interface {
	io.Closer

	// Topics returns the topic repository.
	Topics() TopicRepository

	// Datasets returns the dataset repository.
	Datasets() DatasetRepository

	// Jobs returns the job repository.
	Jobs() JobRepository

	// Nodes returns the node repository.
	Nodes() NodeRepository

	// Users returns the user repository.
	Users() UserRepository

	// Sessions returns the session repository.
	Sessions() SessionRepository

	// Audit returns the audit repository.
	Audit() AuditRepository

	// UserPreferences returns the user preferences repository.
	UserPreferences() UserPreferencesRepository

	// TopicMembers returns the topic membership repository.
	TopicMembers() TopicMemberRepository

	// TopicInvitations returns the topic invitations repository.
	TopicInvitations() TopicInvitationRepository

	// BannedPeers returns the banned peers repository.
	BannedPeers() BannedPeerRepository

	// Ping checks database connectivity.
	Ping(ctx context.Context) error

	// IsAuthoritative returns true if this store can be an authoritative data source.
	// SQLite stores return false; PostgreSQL stores return true.
	IsAuthoritative() bool

	// Backend returns the storage backend type.
	Backend() BackendType

	// Migrate runs database migrations.
	Migrate(ctx context.Context) error

	// Stats returns storage statistics.
	Stats(ctx context.Context) (StorageStats, error)
}

// StorageStats contains storage usage statistics.
type StorageStats struct {
	// DatasetCount is the number of datasets in storage.
	DatasetCount int64

	// TopicCount is the number of topics in storage.
	TopicCount int64

	// BytesUsed is the total storage space used in bytes.
	BytesUsed int64

	// BytesAvailable is the available storage space in bytes.
	BytesAvailable int64

	// Healthy indicates if the storage is healthy.
	Healthy bool

	// Message provides additional information about storage status.
	Message string
}

// BackendType represents the storage backend.
type BackendType string

const (
	BackendSQLite   BackendType = "sqlite"
	BackendPostgres BackendType = "postgres"
)

// String returns the backend name.
func (b BackendType) String() string {
	return string(b)
}

// TopicRepository handles topic persistence.
type TopicRepository interface {
	// Create creates a new topic.
	Create(ctx context.Context, topic *domain.Topic) error

	// Get retrieves a topic by ID.
	Get(ctx context.Context, id domain.TopicID) (*domain.Topic, error)

	// GetByName retrieves a topic by name.
	GetByName(ctx context.Context, name string) (*domain.Topic, error)

	// List retrieves topics matching the filter.
	List(ctx context.Context, filter TopicFilter) ([]*domain.Topic, error)

	// Update updates an existing topic.
	Update(ctx context.Context, topic *domain.Topic) error

	// Delete deletes a topic (soft delete - sets status to deleted).
	Delete(ctx context.Context, id domain.TopicID) error

	// Count returns the number of topics matching the filter.
	Count(ctx context.Context, filter TopicFilter) (int64, error)
}

// TopicFilter defines filtering options for topic queries.
type TopicFilter struct {
	// Status filters by topic status (empty = all)
	Status domain.TopicStatus

	// ParentID filters by parent topic
	ParentID *domain.TopicID

	// OwnerID filters by owner
	OwnerID *domain.UserID

	// Tags filters by tags (AND logic)
	Tags []string

	// Search performs text search on name/description
	Search string

	// Limit is the maximum number of results
	Limit int

	// Offset is the number of results to skip
	Offset int

	// OrderBy is the field to order by
	OrderBy string

	// OrderDesc reverses the order
	OrderDesc bool
}

// DatasetRepository handles dataset persistence.
type DatasetRepository interface {
	// Create creates a new dataset.
	Create(ctx context.Context, dataset *domain.Dataset) error

	// Get retrieves a dataset by ID.
	Get(ctx context.Context, id domain.DatasetID) (*domain.Dataset, error)

	// List retrieves datasets matching the filter.
	List(ctx context.Context, filter DatasetFilter) ([]*domain.Dataset, error)

	// Update updates an existing dataset.
	Update(ctx context.Context, dataset *domain.Dataset) error

	// Delete deletes a dataset (soft delete).
	Delete(ctx context.Context, id domain.DatasetID) error

	// Count returns the number of datasets matching the filter.
	Count(ctx context.Context, filter DatasetFilter) (int64, error)

	// Versions

	// CreateVersion creates a new dataset version.
	CreateVersion(ctx context.Context, version *domain.DatasetVersion) error

	// GetVersion retrieves a specific version.
	GetVersion(ctx context.Context, datasetID domain.DatasetID, versionID domain.DatasetVersionID) (*domain.DatasetVersion, error)

	// GetLatestVersion retrieves the latest version of a dataset.
	GetLatestVersion(ctx context.Context, datasetID domain.DatasetID) (*domain.DatasetVersion, error)

	// ListVersions lists all versions of a dataset.
	ListVersions(ctx context.Context, datasetID domain.DatasetID) ([]*domain.DatasetVersion, error)

	// Chunks

	// CreateChunk creates a new chunk record.
	CreateChunk(ctx context.Context, chunk *domain.Chunk) error

	// GetChunk retrieves a chunk by dataset version and index.
	GetChunk(ctx context.Context, versionID domain.DatasetVersionID, index int) (*domain.Chunk, error)

	// ListChunks lists all chunks for a version.
	ListChunks(ctx context.Context, versionID domain.DatasetVersionID) ([]*domain.Chunk, error)

	// UpdateChunkStatus updates the status of a chunk (e.g., downloaded, verified).
	UpdateChunkStatus(ctx context.Context, chunkID domain.ChunkID, status domain.ChunkStatus) error
}

// DatasetFilter defines filtering options for dataset queries.
type DatasetFilter struct {
	// TopicID filters by topic
	TopicID *domain.TopicID

	// Status filters by dataset status
	Status domain.DatasetStatus

	// OwnerID filters by owner
	OwnerID *domain.UserID

	// HasContent filters by content presence
	HasContent *bool

	// HasInstructions filters by instruction presence
	HasInstructions *bool

	// Tags filters by tags
	Tags []string

	// Search performs text search
	Search string

	// Limit is the maximum number of results
	Limit int

	// Offset is the number of results to skip
	Offset int

	// OrderBy is the field to order by
	OrderBy string

	// OrderDesc reverses the order
	OrderDesc bool
}

// JobRepository handles job persistence.
type JobRepository interface {
	// Create creates a new job.
	Create(ctx context.Context, job *domain.Job) error

	// Get retrieves a job by ID.
	Get(ctx context.Context, id domain.JobID) (*domain.Job, error)

	// List retrieves jobs matching the filter.
	List(ctx context.Context, filter JobFilter) ([]*domain.Job, error)

	// Update updates an existing job.
	Update(ctx context.Context, job *domain.Job) error

	// UpdateStatus updates just the job status and related timestamps.
	UpdateStatus(ctx context.Context, id domain.JobID, status domain.JobStatus) error

	// Delete deletes a job.
	Delete(ctx context.Context, id domain.JobID) error

	// Count returns the number of jobs matching the filter.
	Count(ctx context.Context, filter JobFilter) (int64, error)

	// GetPending retrieves pending jobs ordered by priority.
	GetPending(ctx context.Context, limit int) ([]*domain.Job, error)

	// Results

	// CreateResult creates a job result.
	CreateResult(ctx context.Context, result *domain.JobResult) error

	// GetResult retrieves a job result by ID.
	GetResult(ctx context.Context, id string) (*domain.JobResult, error)

	// ListResults lists results for a job.
	ListResults(ctx context.Context, jobID domain.JobID) ([]*domain.JobResult, error)
}

// JobFilter defines filtering options for job queries.
type JobFilter struct {
	// Type filters by job type
	Type domain.JobType

	// Status filters by job status
	Status domain.JobStatus

	// CreatedBy filters by creator
	CreatedBy string

	// TopicID filters by target topic
	TopicID *domain.TopicID

	// DatasetID filters by target dataset
	DatasetID *domain.DatasetID

	// MinPriority filters by minimum priority
	MinPriority *int

	// CreatedAfter filters by creation time
	CreatedAfter *time.Time

	// CreatedBefore filters by creation time
	CreatedBefore *time.Time

	// Limit is the maximum number of results
	Limit int

	// Offset is the number of results to skip
	Offset int

	// OrderBy is the field to order by
	OrderBy string

	// OrderDesc reverses the order
	OrderDesc bool
}

// NodeRepository handles peer node persistence.
type NodeRepository interface {
	// Upsert creates or updates a node.
	Upsert(ctx context.Context, node *NodeInfo) error

	// Get retrieves a node by peer ID.
	Get(ctx context.Context, peerID string) (*NodeInfo, error)

	// List retrieves nodes matching the filter.
	List(ctx context.Context, filter NodeFilter) ([]*NodeInfo, error)

	// Delete removes a node.
	Delete(ctx context.Context, peerID string) error

	// UpdateLastSeen updates the last seen timestamp.
	UpdateLastSeen(ctx context.Context, peerID string) error

	// Count returns the number of nodes matching the filter.
	Count(ctx context.Context, filter NodeFilter) (int64, error)
}

// NodeInfo represents a peer node in the network.
type NodeInfo struct {
	// PeerID is the libp2p peer ID.
	PeerID string `json:"peer_id"`

	// Addresses are the multiaddrs for this peer.
	Addresses []string `json:"addresses"`

	// Mode is the node's operation mode.
	Mode string `json:"mode"`

	// StorageType is the storage backend (sqlite/postgres).
	StorageType string `json:"storage_type"`

	// TrustedStorage indicates if the node can be an authoritative source.
	TrustedStorage bool `json:"trusted_storage"`

	// LastSeen is when the node was last seen.
	LastSeen time.Time `json:"last_seen"`

	// Metadata holds additional node information.
	Metadata map[string]any `json:"metadata,omitempty"`

	// CreatedAt is when the node was first seen.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the node info was last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// NodeFilter defines filtering options for node queries.
type NodeFilter struct {
	// Mode filters by node mode
	Mode string

	// TrustedOnly filters to only trusted storage nodes
	TrustedOnly bool

	// SeenAfter filters by last seen time
	SeenAfter *time.Time

	// Limit is the maximum number of results
	Limit int

	// Offset is the number of results to skip
	Offset int
}

// AuditRepository handles audit log persistence.
type AuditRepository interface {
	// Log records an audit entry.
	Log(ctx context.Context, entry *AuditEntry) error

	// Query retrieves audit entries matching the filter.
	Query(ctx context.Context, filter AuditFilter) ([]*AuditEntry, error)

	// Count returns the number of entries matching the filter.
	Count(ctx context.Context, filter AuditFilter) (int64, error)

	// GetByOperationID retrieves all entries for an operation.
	GetByOperationID(ctx context.Context, operationID string) ([]*AuditEntry, error)

	// GetByJobID retrieves all entries for a job.
	GetByJobID(ctx context.Context, jobID string) ([]*AuditEntry, error)

	// Purge removes entries older than the retention period.
	Purge(ctx context.Context, before time.Time) (int64, error)

	// VerifyChain verifies the hash chain integrity.
	VerifyChain(ctx context.Context, from, to int64) (bool, error)

	// GetLastHash returns the hash of the last entry.
	GetLastHash(ctx context.Context) (string, error)
}

// AuditEntry represents a single audit log entry.
type AuditEntry struct {
	// ID is the unique entry ID.
	ID int64 `json:"id"`

	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`

	// NodeID is the node that generated the entry.
	NodeID string `json:"node_id"`

	// JobID is the associated job (optional).
	JobID string `json:"job_id,omitempty"`

	// OperationID is the operation identifier.
	OperationID string `json:"operation_id"`

	// RoleUsed is the database role used.
	RoleUsed string `json:"role_used"`

	// Action is the type of action (SELECT, INSERT, UPDATE, DELETE, DDL).
	Action string `json:"action"`

	// TableName is the affected table.
	TableName string `json:"table_name,omitempty"`

	// Query is the SQL query (with sensitive values redacted).
	Query string `json:"query,omitempty"`

	// QueryHash is a hash of the query for grouping.
	QueryHash string `json:"query_hash,omitempty"`

	// RowsAffected is the number of rows affected.
	RowsAffected int `json:"rows_affected"`

	// DurationMS is the execution time in milliseconds.
	DurationMS int `json:"duration_ms"`

	// SourceComponent is the component that initiated the operation.
	SourceComponent string `json:"source_component"`

	// Actor is the user/node that initiated the operation.
	Actor string `json:"actor,omitempty"`

	// Metadata holds additional context.
	Metadata map[string]any `json:"metadata,omitempty"`

	// PrevHash is the hash of the previous entry (for tamper detection).
	PrevHash string `json:"prev_hash,omitempty"`

	// EntryHash is the hash of this entry.
	EntryHash string `json:"entry_hash"`

	// Flags contains additional flags for this entry.
	Flags AuditEntryFlags `json:"flags,omitempty"`
}

// AuditEntryFlags contains additional flags for audit entries.
type AuditEntryFlags struct {
	// BreakGlass indicates this was a break-glass session operation.
	BreakGlass bool `json:"break_glass,omitempty"`

	// RateLimited indicates this operation triggered rate limiting.
	RateLimited bool `json:"rate_limited,omitempty"`

	// Suspicious indicates this operation matched a suspicious pattern.
	Suspicious bool `json:"suspicious,omitempty"`

	// AlertTriggered indicates an alert was triggered for this operation.
	AlertTriggered bool `json:"alert_triggered,omitempty"`
}

// AuditFilter defines filtering options for audit queries.
type AuditFilter struct {
	// NodeID filters by node
	NodeID string

	// JobID filters by job
	JobID string

	// OperationID filters by operation
	OperationID string

	// Action filters by action type
	Action string

	// TableName filters by table
	TableName string

	// RoleUsed filters by role
	RoleUsed string

	// Actor filters by actor
	Actor string

	// After filters by timestamp
	After *time.Time

	// Before filters by timestamp
	Before *time.Time

	// Suspicious filters for suspicious entries only
	Suspicious *bool

	// Limit is the maximum number of results
	Limit int

	// Offset is the number of results to skip
	Offset int
}

// UserRepository handles user persistence.
type UserRepository interface {
	// Create creates a new user.
	Create(ctx context.Context, user *domain.User) error

	// Get retrieves a user by ID.
	Get(ctx context.Context, id domain.UserID) (*domain.User, error)

	// GetByPublicKey retrieves a user by their public key.
	GetByPublicKey(ctx context.Context, publicKey []byte) (*domain.User, error)

	// GetByEmail retrieves a user by email.
	GetByEmail(ctx context.Context, email string) (*domain.User, error)

	// List retrieves users matching the filter.
	List(ctx context.Context, filter UserFilter) ([]*domain.User, error)

	// Update updates an existing user.
	Update(ctx context.Context, user *domain.User) error

	// Delete deletes a user (soft delete - sets status to deleted).
	Delete(ctx context.Context, id domain.UserID) error

	// Count returns the number of users matching the filter.
	Count(ctx context.Context, filter UserFilter) (int64, error)

	// Exists checks if a user with the given public key exists.
	Exists(ctx context.Context, publicKey []byte) (bool, error)

	// IsFirstUser returns true if no users exist yet (for auto-admin).
	IsFirstUser(ctx context.Context) (bool, error)
}

// UserFilter defines filtering options for user queries.
type UserFilter struct {
	// Status filters by user status
	Status domain.UserStatus

	// Role filters by user role
	Role domain.UserRole

	// Search performs text search on name/email
	Search string

	// Limit is the maximum number of results
	Limit int

	// Offset is the number of results to skip
	Offset int

	// OrderBy is the field to order by
	OrderBy string

	// OrderDesc reverses the order
	OrderDesc bool
}

// SessionRepository handles user session persistence for security auditing.
type SessionRepository interface {
	// Create creates a new session.
	Create(ctx context.Context, session *Session) error

	// Get retrieves a session by ID.
	Get(ctx context.Context, id string) (*Session, error)

	// GetByUser retrieves all active sessions for a user.
	GetByUser(ctx context.Context, userID domain.UserID) ([]*Session, error)

	// Update updates an existing session.
	Update(ctx context.Context, session *Session) error

	// End marks a session as ended.
	End(ctx context.Context, id string) error

	// EndAllForUser ends all sessions for a user.
	EndAllForUser(ctx context.Context, userID domain.UserID) error

	// List retrieves sessions matching the filter.
	List(ctx context.Context, filter SessionFilter) ([]*Session, error)

	// Cleanup removes expired sessions older than the given time.
	Cleanup(ctx context.Context, before time.Time) (int64, error)
}

// Session represents an authenticated user session.
type Session struct {
	// ID is the unique session identifier.
	ID string `json:"id"`

	// UserID is the user who owns this session.
	UserID domain.UserID `json:"user_id"`

	// Type is the session type (ssh, grpc, api).
	Type SessionType `json:"type"`

	// ClientIP is the client's IP address.
	ClientIP string `json:"client_ip"`

	// ClientAgent is the client's user agent or SSH client string.
	ClientAgent string `json:"client_agent,omitempty"`

	// PublicKeyFingerprint is the fingerprint of the key used to authenticate.
	PublicKeyFingerprint string `json:"public_key_fingerprint"`

	// NodeID is the node that the session is connected to.
	NodeID string `json:"node_id"`

	// StartedAt is when the session started.
	StartedAt time.Time `json:"started_at"`

	// EndedAt is when the session ended (null if still active).
	EndedAt *time.Time `json:"ended_at,omitempty"`

	// LastActivityAt is when the session was last active.
	LastActivityAt time.Time `json:"last_activity_at"`

	// Metadata holds additional session data.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// SessionType represents the type of session.
type SessionType string

const (
	SessionTypeSSH  SessionType = "ssh"
	SessionTypeGRPC SessionType = "grpc"
	SessionTypeAPI  SessionType = "api"
)

// SessionFilter defines filtering options for session queries.
type SessionFilter struct {
	// UserID filters by user
	UserID *domain.UserID

	// Type filters by session type
	Type SessionType

	// Active filters for active sessions only (EndedAt is null)
	Active *bool

	// ClientIP filters by client IP
	ClientIP string

	// NodeID filters by node
	NodeID string

	// After filters by start time
	After *time.Time

	// Before filters by start time
	Before *time.Time

	// Limit is the maximum number of results
	Limit int

	// Offset is the number of results to skip
	Offset int
}

// =============================================================================
// User Preferences
// =============================================================================

// UserPreferences represents user preference settings.
type UserPreferences struct {
	// UserID is the user ID.
	UserID domain.UserID `json:"user_id"`

	// Theme is the UI theme preference: "light", "dark", "system".
	Theme string `json:"theme"`

	// Locale is the preferred locale.
	Locale string `json:"locale"`

	// Timezone is the IANA timezone name.
	Timezone string `json:"timezone"`

	// DateFormat is the preferred date format.
	DateFormat string `json:"date_format"`

	// NotificationsEnabled indicates if notifications are enabled.
	NotificationsEnabled bool `json:"notifications_enabled"`

	// EmailNotifications indicates if email notifications are enabled.
	EmailNotifications bool `json:"email_notifications"`

	// Custom holds additional preferences.
	Custom map[string]string `json:"custom,omitempty"`

	// CreatedAt is when the preferences were created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the preferences were last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// UserPreferencesRepository handles user preferences persistence.
type UserPreferencesRepository interface {
	// Get retrieves preferences for a user.
	Get(ctx context.Context, userID domain.UserID) (*UserPreferences, error)

	// Upsert creates or updates preferences for a user.
	Upsert(ctx context.Context, prefs *UserPreferences) error

	// Delete removes preferences for a user.
	Delete(ctx context.Context, userID domain.UserID) error
}

// =============================================================================
// Topic Membership
// =============================================================================

// TopicMemberRole represents the role of a topic member.
type TopicMemberRole string

const (
	TopicMemberRoleOwner  TopicMemberRole = "owner"
	TopicMemberRoleEditor TopicMemberRole = "editor"
	TopicMemberRoleViewer TopicMemberRole = "viewer"
)

// IsValid checks if the role is valid.
func (r TopicMemberRole) IsValid() bool {
	switch r {
	case TopicMemberRoleOwner, TopicMemberRoleEditor, TopicMemberRoleViewer:
		return true
	default:
		return false
	}
}

// CanEdit returns true if the role allows editing.
func (r TopicMemberRole) CanEdit() bool {
	return r == TopicMemberRoleOwner || r == TopicMemberRoleEditor
}

// TopicMember represents a user's membership in a topic.
type TopicMember struct {
	// ID is the unique membership ID.
	ID string `json:"id"`

	// TopicID is the topic ID.
	TopicID domain.TopicID `json:"topic_id"`

	// UserID is the user ID.
	UserID domain.UserID `json:"user_id"`

	// Role is the member's role in the topic.
	Role TopicMemberRole `json:"role"`

	// InvitedBy is the user who invited this member.
	InvitedBy domain.UserID `json:"invited_by,omitempty"`

	// InvitedAt is when the user was invited.
	InvitedAt time.Time `json:"invited_at"`

	// AcceptedAt is when the user accepted the invitation.
	AcceptedAt *time.Time `json:"accepted_at,omitempty"`

	// CreatedAt is when the membership was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the membership was last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// TopicMemberRepository handles topic membership persistence.
type TopicMemberRepository interface {
	// Create creates a new membership.
	Create(ctx context.Context, member *TopicMember) error

	// Get retrieves a membership by topic and user.
	Get(ctx context.Context, topicID domain.TopicID, userID domain.UserID) (*TopicMember, error)

	// GetByID retrieves a membership by ID.
	GetByID(ctx context.Context, id string) (*TopicMember, error)

	// ListByTopic lists all members of a topic.
	ListByTopic(ctx context.Context, topicID domain.TopicID, filter TopicMemberFilter) ([]*TopicMember, error)

	// ListByUser lists all topic memberships for a user.
	ListByUser(ctx context.Context, userID domain.UserID) ([]*TopicMember, error)

	// Update updates a membership.
	Update(ctx context.Context, member *TopicMember) error

	// Delete removes a membership.
	Delete(ctx context.Context, topicID domain.TopicID, userID domain.UserID) error

	// CountOwners counts the number of owners for a topic.
	CountOwners(ctx context.Context, topicID domain.TopicID) (int, error)

	// HasAccess checks if a user has access to a topic.
	HasAccess(ctx context.Context, topicID domain.TopicID, userID domain.UserID) (bool, error)

	// GetRole gets the role of a user in a topic.
	GetRole(ctx context.Context, topicID domain.TopicID, userID domain.UserID) (TopicMemberRole, error)
}

// TopicMemberFilter defines filtering options for topic member queries.
type TopicMemberFilter struct {
	// Role filters by role
	Role TopicMemberRole

	// Limit is the maximum number of results
	Limit int

	// Offset is the number of results to skip
	Offset int
}

// =============================================================================
// Topic Invitations
// =============================================================================

// InvitationStatus represents the status of an invitation.
type InvitationStatus string

const (
	InvitationStatusPending   InvitationStatus = "pending"
	InvitationStatusAccepted  InvitationStatus = "accepted"
	InvitationStatusDeclined  InvitationStatus = "declined"
	InvitationStatusExpired   InvitationStatus = "expired"
	InvitationStatusCancelled InvitationStatus = "cancelled"
)

// TopicInvitation represents an invitation to join a topic.
type TopicInvitation struct {
	// ID is the unique invitation ID.
	ID string `json:"id"`

	// TopicID is the topic being invited to.
	TopicID domain.TopicID `json:"topic_id"`

	// InviterID is the user who sent the invitation.
	InviterID domain.UserID `json:"inviter_id"`

	// InviteeEmail is the email of the invitee (optional).
	InviteeEmail string `json:"invitee_email,omitempty"`

	// InviteeUserID is the user ID of the invitee (optional).
	InviteeUserID domain.UserID `json:"invitee_user_id,omitempty"`

	// Role is the role being offered.
	Role TopicMemberRole `json:"role"`

	// Token is the unique token for accepting the invitation.
	Token string `json:"token"`

	// Message is an optional message from the inviter.
	Message string `json:"message,omitempty"`

	// Status is the invitation status.
	Status InvitationStatus `json:"status"`

	// ExpiresAt is when the invitation expires.
	ExpiresAt time.Time `json:"expires_at"`

	// CreatedAt is when the invitation was created.
	CreatedAt time.Time `json:"created_at"`

	// RespondedAt is when the invitation was responded to.
	RespondedAt *time.Time `json:"responded_at,omitempty"`
}

// TopicInvitationRepository handles topic invitation persistence.
type TopicInvitationRepository interface {
	// Create creates a new invitation.
	Create(ctx context.Context, invitation *TopicInvitation) error

	// Get retrieves an invitation by ID.
	Get(ctx context.Context, id string) (*TopicInvitation, error)

	// GetByToken retrieves an invitation by token.
	GetByToken(ctx context.Context, token string) (*TopicInvitation, error)

	// ListByTopic lists invitations for a topic.
	ListByTopic(ctx context.Context, topicID domain.TopicID, filter InvitationFilter) ([]*TopicInvitation, error)

	// ListByUser lists invitations for a user (as invitee).
	ListByUser(ctx context.Context, userID domain.UserID) ([]*TopicInvitation, error)

	// ListByEmail lists invitations for an email address.
	ListByEmail(ctx context.Context, email string) ([]*TopicInvitation, error)

	// Update updates an invitation.
	Update(ctx context.Context, invitation *TopicInvitation) error

	// Delete removes an invitation.
	Delete(ctx context.Context, id string) error

	// ExpirePending expires all pending invitations that have passed their expiration.
	ExpirePending(ctx context.Context) (int64, error)
}

// InvitationFilter defines filtering options for invitation queries.
type InvitationFilter struct {
	// Status filters by status
	Status InvitationStatus

	// Limit is the maximum number of results
	Limit int

	// Offset is the number of results to skip
	Offset int
}

// =============================================================================
// Banned Peers
// =============================================================================

// BannedPeer represents a banned peer.
type BannedPeer struct {
	// PeerID is the libp2p peer ID.
	PeerID string `json:"peer_id"`

	// Reason is the reason for the ban.
	Reason string `json:"reason"`

	// BannedBy is the user who banned the peer.
	BannedBy domain.UserID `json:"banned_by,omitempty"`

	// BannedAt is when the peer was banned.
	BannedAt time.Time `json:"banned_at"`

	// ExpiresAt is when the ban expires (nil = permanent).
	ExpiresAt *time.Time `json:"expires_at,omitempty"`

	// Metadata holds additional ban information.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// IsPermanent returns true if the ban is permanent.
func (bp *BannedPeer) IsPermanent() bool {
	return bp.ExpiresAt == nil
}

// IsExpired returns true if the ban has expired.
func (bp *BannedPeer) IsExpired() bool {
	if bp.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*bp.ExpiresAt)
}

// BannedPeerRepository handles banned peer persistence.
type BannedPeerRepository interface {
	// Create creates a new ban.
	Create(ctx context.Context, ban *BannedPeer) error

	// Get retrieves a ban by peer ID.
	Get(ctx context.Context, peerID string) (*BannedPeer, error)

	// List lists all bans.
	List(ctx context.Context, filter BannedPeerFilter) ([]*BannedPeer, error)

	// Delete removes a ban.
	Delete(ctx context.Context, peerID string) error

	// IsBanned checks if a peer is currently banned.
	IsBanned(ctx context.Context, peerID string) (bool, error)

	// CleanupExpired removes expired bans.
	CleanupExpired(ctx context.Context) (int64, error)
}

// BannedPeerFilter defines filtering options for banned peer queries.
type BannedPeerFilter struct {
	// IncludeExpired includes expired bans
	IncludeExpired bool

	// Limit is the maximum number of results
	Limit int

	// Offset is the number of results to skip
	Offset int
}
