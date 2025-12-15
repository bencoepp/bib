package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// AuditAction represents the type of auditable action.
type AuditAction string

const (
	AuditActionConfigChange AuditAction = "config_change"
	AuditActionAuthAttempt  AuditAction = "auth_attempt"
	AuditActionAuthSuccess  AuditAction = "auth_success"
	AuditActionAuthFailure  AuditAction = "auth_failure"
	AuditActionCommand      AuditAction = "command"
	AuditActionAccess       AuditAction = "access"
	AuditActionCreate       AuditAction = "create"
	AuditActionUpdate       AuditAction = "update"
	AuditActionDelete       AuditAction = "delete"
	AuditActionPermission   AuditAction = "permission_change"
)

// AuditOutcome represents the result of an auditable action.
type AuditOutcome string

const (
	AuditOutcomeSuccess AuditOutcome = "success"
	AuditOutcomeFailure AuditOutcome = "failure"
	AuditOutcomeDenied  AuditOutcome = "denied"
	AuditOutcomePending AuditOutcome = "pending"
)

// AuditEvent represents an auditable event.
type AuditEvent struct {
	Action    AuditAction    `json:"action"`
	Actor     string         `json:"actor"`
	Resource  string         `json:"resource"`
	Outcome   AuditOutcome   `json:"outcome"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
	RequestID string         `json:"request_id,omitempty"`
}

// AuditLogger handles audit logging to a dedicated file.
type AuditLogger struct {
	logger *slog.Logger
	closer *lumberjack.Logger
}

// NewAuditLogger creates a new audit logger.
func NewAuditLogger(auditPath string, maxAgeDays int) (*AuditLogger, error) {
	if auditPath == "" {
		return nil, fmt.Errorf("audit path is required")
	}

	// Ensure directory exists
	if err := os.MkdirAll(auditPath[:len(auditPath)-len("/audit.log")], 0750); err != nil {
		// If we can't parse the directory, try creating parent of the full path
		// This handles cases where the path doesn't end with "/audit.log"
	}

	if maxAgeDays <= 0 {
		maxAgeDays = 365 // Default to 1 year retention for audit logs
	}

	lj := &lumberjack.Logger{
		Filename:   auditPath,
		MaxSize:    100, // 100 MB
		MaxBackups: 0,   // Keep all backups within MaxAge
		MaxAge:     maxAgeDays,
		Compress:   true,
	}

	// Always use JSON for audit logs
	handler := slog.NewJSONHandler(lj, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	return &AuditLogger{
		logger: slog.New(handler),
		closer: lj,
	}, nil
}

// Log records an audit event.
func (a *AuditLogger) Log(ctx context.Context, event AuditEvent) {
	if a == nil {
		return
	}

	// Set timestamp if not already set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Try to get request ID from context if not set
	if event.RequestID == "" {
		if cc := CommandContextFrom(ctx); cc != nil {
			event.RequestID = cc.RequestID
		}
	}

	attrs := []slog.Attr{
		slog.String("action", string(event.Action)),
		slog.String("actor", event.Actor),
		slog.String("resource", event.Resource),
		slog.String("outcome", string(event.Outcome)),
		slog.Time("timestamp", event.Timestamp),
	}

	if event.RequestID != "" {
		attrs = append(attrs, slog.String("request_id", event.RequestID))
	}

	if len(event.Metadata) > 0 {
		attrs = append(attrs, slog.Any("metadata", event.Metadata))
	}

	// Convert to []any for LogAttrs
	args := make([]any, len(attrs))
	for i, attr := range attrs {
		args[i] = attr
	}

	a.logger.LogAttrs(ctx, slog.LevelInfo, "audit", attrs...)
}

// LogCommand records a command execution audit event.
func (a *AuditLogger) LogCommand(ctx context.Context, command string, outcome AuditOutcome, metadata map[string]any) {
	actor := "unknown"
	if cc := CommandContextFrom(ctx); cc != nil {
		actor = cc.User
	}

	a.Log(ctx, AuditEvent{
		Action:   AuditActionCommand,
		Actor:    actor,
		Resource: command,
		Outcome:  outcome,
		Metadata: metadata,
	})
}

// LogConfigChange records a configuration change audit event.
func (a *AuditLogger) LogConfigChange(ctx context.Context, resource string, outcome AuditOutcome, before, after any) {
	actor := "unknown"
	if cc := CommandContextFrom(ctx); cc != nil {
		actor = cc.User
	}

	metadata := map[string]any{}
	if before != nil {
		metadata["before"] = before
	}
	if after != nil {
		metadata["after"] = after
	}

	a.Log(ctx, AuditEvent{
		Action:   AuditActionConfigChange,
		Actor:    actor,
		Resource: resource,
		Outcome:  outcome,
		Metadata: metadata,
	})
}

// LogAuth records an authentication audit event.
func (a *AuditLogger) LogAuth(ctx context.Context, actor string, success bool, metadata map[string]any) {
	action := AuditActionAuthSuccess
	outcome := AuditOutcomeSuccess
	if !success {
		action = AuditActionAuthFailure
		outcome = AuditOutcomeFailure
	}

	a.Log(ctx, AuditEvent{
		Action:   action,
		Actor:    actor,
		Resource: "auth",
		Outcome:  outcome,
		Metadata: metadata,
	})
}

// Close closes the audit logger.
func (a *AuditLogger) Close() error {
	if a != nil && a.closer != nil {
		return a.closer.Close()
	}
	return nil
}

// NopAuditLogger returns an audit logger that does nothing.
// Useful when audit logging is disabled.
func NopAuditLogger() *AuditLogger {
	return nil
}
