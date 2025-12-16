// Package storage provides the database abstraction layer for bib.
// It defines repository interfaces and provides implementations for
// both SQLite (limited/cache mode) and PostgreSQL (full mode).
package storage

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// OperationContext carries audit and permission information for database operations.
// It is attached to context.Context and used for query tagging and role selection.
type OperationContext struct {
	// OperationID is a unique identifier for this operation (auto-generated if empty)
	OperationID string

	// JobID is the job this operation belongs to (optional)
	JobID string

	// Role determines which database role/permissions to use
	Role DBRole

	// Source identifies the component initiating the operation
	Source string

	// Actor is the user or node initiating the operation
	Actor string

	// StartTime is when the operation started
	StartTime time.Time

	// Metadata holds additional context for audit logging
	Metadata map[string]any
}

// DBRole represents a database role with specific permissions.
type DBRole string

const (
	// RoleAdmin has full access to all tables (migrations, maintenance)
	RoleAdmin DBRole = "bibd_admin"

	// RoleScrape can INSERT datasets/chunks, SELECT topics
	RoleScrape DBRole = "bibd_scrape"

	// RoleQuery can SELECT from datasets, chunks, topics
	RoleQuery DBRole = "bibd_query"

	// RoleTransform can SELECT, INSERT, UPDATE datasets/chunks
	RoleTransform DBRole = "bibd_transform"

	// RoleAudit can INSERT and SELECT audit_log
	RoleAudit DBRole = "bibd_audit"

	// RoleReadOnly is for cache/proxy operations (SELECT only)
	RoleReadOnly DBRole = "bibd_readonly"
)

// String returns the role name.
func (r DBRole) String() string {
	return string(r)
}

// IsValid checks if the role is valid.
func (r DBRole) IsValid() bool {
	switch r {
	case RoleAdmin, RoleScrape, RoleQuery, RoleTransform, RoleAudit, RoleReadOnly:
		return true
	default:
		return false
	}
}

// contextKey is a private type for context keys to avoid collisions.
type contextKey int

const (
	operationContextKey contextKey = iota
)

// NewOperationContext creates a new OperationContext with defaults.
func NewOperationContext(role DBRole, source string) *OperationContext {
	return &OperationContext{
		OperationID: uuid.New().String(),
		Role:        role,
		Source:      source,
		StartTime:   time.Now(),
		Metadata:    make(map[string]any),
	}
}

// WithJobID sets the job ID.
func (oc *OperationContext) WithJobID(jobID string) *OperationContext {
	oc.JobID = jobID
	return oc
}

// WithActor sets the actor.
func (oc *OperationContext) WithActor(actor string) *OperationContext {
	oc.Actor = actor
	return oc
}

// WithMetadata adds metadata.
func (oc *OperationContext) WithMetadata(key string, value any) *OperationContext {
	oc.Metadata[key] = value
	return oc
}

// WithOperationContext attaches an OperationContext to a context.Context.
func WithOperationContext(ctx context.Context, oc *OperationContext) context.Context {
	return context.WithValue(ctx, operationContextKey, oc)
}

// GetOperationContext retrieves the OperationContext from a context.Context.
// Returns nil if no OperationContext is present.
func GetOperationContext(ctx context.Context) *OperationContext {
	oc, _ := ctx.Value(operationContextKey).(*OperationContext)
	return oc
}

// MustGetOperationContext retrieves the OperationContext or creates a default one.
func MustGetOperationContext(ctx context.Context) *OperationContext {
	oc := GetOperationContext(ctx)
	if oc == nil {
		oc = NewOperationContext(RoleReadOnly, "unknown")
	}
	return oc
}

// QueryComment generates a SQL comment with operation context for query tagging.
func (oc *OperationContext) QueryComment() string {
	comment := "/* "
	comment += "op_id:" + oc.OperationID
	if oc.JobID != "" {
		comment += " job_id:" + oc.JobID
	}
	comment += " role:" + string(oc.Role)
	comment += " source:" + oc.Source
	if oc.Actor != "" {
		comment += " actor:" + oc.Actor
	}
	comment += " */"
	return comment
}
