// Package audit provides comprehensive database audit logging, streaming,
// and monitoring capabilities for bib storage operations.
package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Entry represents a single audit log entry with comprehensive metadata.
type Entry struct {
	// ID is the unique entry ID (set after persistence).
	ID int64 `json:"id"`

	// Timestamp is when the event occurred (UTC, microsecond precision).
	Timestamp time.Time `json:"timestamp"`

	// NodeID is the node that generated the entry.
	NodeID string `json:"node_id"`

	// JobID is the associated job (optional).
	JobID string `json:"job_id,omitempty"`

	// OperationID is the unique operation identifier.
	OperationID string `json:"operation_id"`

	// RoleUsed is the database role used for this operation.
	RoleUsed string `json:"role_used"`

	// Action is the type of action (SELECT, INSERT, UPDATE, DELETE, DDL).
	Action Action `json:"action"`

	// TableName is the affected table.
	TableName string `json:"table_name,omitempty"`

	// Query is the SQL query (with sensitive values redacted).
	Query string `json:"query,omitempty"`

	// QueryHash is a hash of the query for grouping similar queries.
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
	Flags EntryFlags `json:"flags,omitempty"`
}

// EntryFlags contains additional flags for audit entries.
type EntryFlags struct {
	// BreakGlass indicates this was a break-glass session operation.
	BreakGlass bool `json:"break_glass,omitempty"`

	// RateLimited indicates this operation triggered rate limiting.
	RateLimited bool `json:"rate_limited,omitempty"`

	// Suspicious indicates this operation matched a suspicious pattern.
	Suspicious bool `json:"suspicious,omitempty"`

	// AlertTriggered indicates an alert was triggered for this operation.
	AlertTriggered bool `json:"alert_triggered,omitempty"`
}

// Action represents the type of database operation.
type Action string

const (
	ActionSelect Action = "SELECT"
	ActionInsert Action = "INSERT"
	ActionUpdate Action = "UPDATE"
	ActionDelete Action = "DELETE"
	ActionDDL    Action = "DDL"
	ActionOther  Action = "OTHER"
)

// ParseAction determines the action type from a SQL query.
func ParseAction(query string) Action {
	normalized := strings.ToUpper(strings.TrimSpace(query))
	switch {
	case strings.HasPrefix(normalized, "SELECT"):
		return ActionSelect
	case strings.HasPrefix(normalized, "INSERT"):
		return ActionInsert
	case strings.HasPrefix(normalized, "UPDATE"):
		return ActionUpdate
	case strings.HasPrefix(normalized, "DELETE"):
		return ActionDelete
	case strings.HasPrefix(normalized, "CREATE"),
		strings.HasPrefix(normalized, "ALTER"),
		strings.HasPrefix(normalized, "DROP"),
		strings.HasPrefix(normalized, "TRUNCATE"):
		return ActionDDL
	default:
		return ActionOther
	}
}

// ExtractTableName attempts to extract the table name from a SQL query.
func ExtractTableName(query string) string {
	normalized := strings.ToUpper(strings.TrimSpace(query))

	patterns := []struct {
		prefix  string
		pattern *regexp.Regexp
	}{
		{"SELECT", regexp.MustCompile(`(?i)FROM\s+([^\s,()]+)`)},
		{"INSERT", regexp.MustCompile(`(?i)INTO\s+([^\s(]+)`)},
		{"UPDATE", regexp.MustCompile(`(?i)UPDATE\s+([^\s]+)`)},
		{"DELETE", regexp.MustCompile(`(?i)FROM\s+([^\s]+)`)},
		{"CREATE", regexp.MustCompile(`(?i)TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([^\s(]+)`)},
		{"ALTER", regexp.MustCompile(`(?i)TABLE\s+([^\s]+)`)},
		{"DROP", regexp.MustCompile(`(?i)TABLE\s+(?:IF\s+EXISTS\s+)?([^\s]+)`)},
		{"TRUNCATE", regexp.MustCompile(`(?i)TABLE\s+([^\s]+)`)},
	}

	for _, p := range patterns {
		if strings.HasPrefix(normalized, p.prefix) {
			if matches := p.pattern.FindStringSubmatch(query); len(matches) > 1 {
				return strings.Trim(matches[1], `"'`)
			}
		}
	}

	return ""
}

// NewEntry creates a new audit entry with required fields.
func NewEntry(nodeID, operationID, role, source string, action Action) *Entry {
	return &Entry{
		Timestamp:       time.Now().UTC(),
		NodeID:          nodeID,
		OperationID:     operationID,
		RoleUsed:        role,
		Action:          action,
		SourceComponent: source,
		Metadata:        make(map[string]any),
	}
}

// CalculateHash computes the SHA-256 hash for this entry.
func (e *Entry) CalculateHash() string {
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%d|%d|%s|%s|%s",
		e.Timestamp.UTC().Format(time.RFC3339Nano),
		e.NodeID,
		e.OperationID,
		e.RoleUsed,
		e.Action,
		e.TableName,
		e.SourceComponent,
		e.RowsAffected,
		e.DurationMS,
		e.PrevHash,
		e.JobID,
		e.QueryHash,
	)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// SetHashChain sets the previous hash and calculates the entry hash.
func (e *Entry) SetHashChain(prevHash string) {
	e.PrevHash = prevHash
	e.EntryHash = e.CalculateHash()
}

// ToJSON returns the entry as JSON.
func (e *Entry) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// Filter defines filtering options for audit queries.
type Filter struct {
	// NodeID filters by node
	NodeID string

	// JobID filters by job
	JobID string

	// OperationID filters by operation
	OperationID string

	// Action filters by action type
	Action Action

	// TableName filters by table
	TableName string

	// RoleUsed filters by role
	RoleUsed string

	// Actor filters by actor
	Actor string

	// After filters by timestamp (inclusive)
	After *time.Time

	// Before filters by timestamp (inclusive)
	Before *time.Time

	// Suspicious filters for suspicious entries only
	Suspicious *bool

	// Limit is the maximum number of results
	Limit int

	// Offset is the number of results to skip
	Offset int
}

// QueryInfo holds information about a query for logging.
type QueryInfo struct {
	// Query is the original SQL query.
	Query string

	// Args are the query arguments.
	Args []any

	// StartTime is when the query started.
	StartTime time.Time

	// Duration is how long the query took.
	Duration time.Duration

	// RowsAffected is the number of rows affected.
	RowsAffected int64

	// Error is any error that occurred.
	Error error
}

// GenerateOperationID generates a new unique operation ID.
func GenerateOperationID() string {
	return uuid.New().String()
}
