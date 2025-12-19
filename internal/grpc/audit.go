// Package grpc provides gRPC service implementations for the bib daemon.
package grpc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"bib/internal/storage"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// AuditConfig holds configuration for the audit middleware.
type AuditConfig struct {
	// Enabled determines if audit logging is active.
	Enabled bool

	// LogFailedOperations determines if failed operations should be logged.
	LogFailedOperations bool

	// NodeID is the ID of this node for audit entries.
	NodeID string
}

// DefaultAuditConfig returns the default audit configuration.
func DefaultAuditConfig() AuditConfig {
	return AuditConfig{
		Enabled:             true,
		LogFailedOperations: true,
		NodeID:              "",
	}
}

// AuditMiddleware provides audit logging for gRPC services.
type AuditMiddleware struct {
	auditRepo storage.AuditRepository
	cfg       AuditConfig
}

// NewAuditMiddleware creates a new audit middleware.
func NewAuditMiddleware(auditRepo storage.AuditRepository, cfg AuditConfig) *AuditMiddleware {
	return &AuditMiddleware{
		auditRepo: auditRepo,
		cfg:       cfg,
	}
}

// mutationMethods lists gRPC methods that are mutations (Create/Update/Delete).
var mutationMethods = map[string]string{
	// UserService mutations
	"/bib.v1.services.UserService/CreateUser":            "CREATE",
	"/bib.v1.services.UserService/UpdateUser":            "UPDATE",
	"/bib.v1.services.UserService/DeleteUser":            "DELETE",
	"/bib.v1.services.UserService/SuspendUser":           "UPDATE",
	"/bib.v1.services.UserService/ActivateUser":          "UPDATE",
	"/bib.v1.services.UserService/SetUserRole":           "UPDATE",
	"/bib.v1.services.UserService/UpdateCurrentUser":     "UPDATE",
	"/bib.v1.services.UserService/UpdateUserPreferences": "UPDATE",
	"/bib.v1.services.UserService/EndUserSession":        "DELETE",
	"/bib.v1.services.UserService/EndAllUserSessions":    "DELETE",

	// NodeService mutations
	"/bib.v1.services.NodeService/ConnectPeer":    "CREATE",
	"/bib.v1.services.NodeService/DisconnectPeer": "DELETE",
	"/bib.v1.services.NodeService/BanPeer":        "CREATE",
	"/bib.v1.services.NodeService/UnbanPeer":      "DELETE",

	// TopicService mutations
	"/bib.v1.services.TopicService/CreateTopic": "CREATE",
	"/bib.v1.services.TopicService/UpdateTopic": "UPDATE",
	"/bib.v1.services.TopicService/DeleteTopic": "DELETE",
	"/bib.v1.services.TopicService/Subscribe":   "CREATE",
	"/bib.v1.services.TopicService/Unsubscribe": "DELETE",

	// DatasetService mutations
	"/bib.v1.services.DatasetService/CreateDataset": "CREATE",
	"/bib.v1.services.DatasetService/UpdateDataset": "UPDATE",
	"/bib.v1.services.DatasetService/DeleteDataset": "DELETE",
	"/bib.v1.services.DatasetService/UploadDataset": "CREATE",

	// AdminService mutations
	"/bib.v1.services.AdminService/UpdateConfig":  "UPDATE",
	"/bib.v1.services.AdminService/TriggerBackup": "CREATE",
	"/bib.v1.services.AdminService/Shutdown":      "DDL",

	// JobService mutations
	"/bib.v1.services.JobService/CreateJob": "CREATE",
	"/bib.v1.services.JobService/CancelJob": "UPDATE",
	"/bib.v1.services.JobService/RetryJob":  "UPDATE",

	// AuthService mutations
	"/bib.v1.services.AuthService/Logout": "DELETE",

	// BreakGlassService mutations
	"/bib.v1.services.BreakGlassService/InitiateBreakGlass": "DDL",
	"/bib.v1.services.BreakGlassService/EndBreakGlass":      "DDL",
}

// AuditUnaryInterceptor creates a unary interceptor for audit logging.
func AuditUnaryInterceptor(am *AuditMiddleware) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if am == nil || !am.cfg.Enabled {
			return handler(ctx, req)
		}

		// Check if this is a mutation method
		action, isMutation := mutationMethods[info.FullMethod]
		if !isMutation {
			return handler(ctx, req)
		}

		startTime := time.Now()

		// Execute the handler
		resp, err := handler(ctx, req)

		// Log if successful or if configured to log failures
		if err == nil || am.cfg.LogFailedOperations {
			am.logAuditEntry(ctx, info.FullMethod, action, req, resp, err, startTime)
		}

		return resp, err
	}
}

// AuditStreamInterceptor creates a stream interceptor for audit logging.
// Note: Stream methods are typically not mutations, but we include this for completeness.
func AuditStreamInterceptor(am *AuditMiddleware) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if am == nil || !am.cfg.Enabled {
			return handler(srv, ss)
		}

		action, isMutation := mutationMethods[info.FullMethod]
		if !isMutation {
			return handler(srv, ss)
		}

		startTime := time.Now()

		err := handler(srv, ss)

		if err == nil || am.cfg.LogFailedOperations {
			am.logAuditEntry(ss.Context(), info.FullMethod, action, nil, nil, err, startTime)
		}

		return err
	}
}

// logAuditEntry creates and stores an audit entry.
func (am *AuditMiddleware) logAuditEntry(ctx context.Context, method, action string, req, resp interface{}, err error, startTime time.Time) {
	duration := time.Since(startTime)

	// Extract user from context
	actor := "anonymous"
	if user, ok := UserFromContext(ctx); ok {
		actor = string(user.ID)
	}

	// Extract client IP
	clientIP := ""
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		clientIP = p.Addr.String()
	}

	// Generate operation ID
	operationID := generateOperationID()

	// Determine table/resource from method
	tableName := extractResourceFromMethod(method)

	// Create query hash for grouping
	queryHash := hashString(method)

	// Build metadata
	metadata := map[string]any{
		"method":    method,
		"client_ip": clientIP,
	}

	// Add request summary (redacted)
	if req != nil {
		if protoMsg, ok := req.(proto.Message); ok {
			metadata["request_type"] = string(protoMsg.ProtoReflect().Descriptor().FullName())
		}
	}

	// Determine if operation was suspicious (failed auth, etc.)
	suspicious := false
	if err != nil {
		st, ok := status.FromError(err)
		if ok && (st.Code() == codes.Unauthenticated || st.Code() == codes.PermissionDenied) {
			suspicious = true
		}
		metadata["error"] = err.Error()
		metadata["error_code"] = status.Code(err).String()
	}

	// Get last hash for chain
	lastHash, _ := am.auditRepo.GetLastHash(ctx)

	// Create entry
	entry := &storage.AuditEntry{
		Timestamp:       time.Now().UTC(),
		NodeID:          am.cfg.NodeID,
		OperationID:     operationID,
		RoleUsed:        "grpc",
		Action:          action,
		TableName:       tableName,
		Query:           method,
		QueryHash:       queryHash,
		RowsAffected:    1, // Approximate
		DurationMS:      int(duration.Milliseconds()),
		SourceComponent: "grpc",
		Actor:           actor,
		Metadata:        metadata,
		PrevHash:        lastHash,
		Flags: storage.AuditEntryFlags{
			Suspicious: suspicious,
		},
	}

	// Calculate entry hash
	entry.EntryHash = am.calculateEntryHash(entry)

	// Store the entry
	if err := am.auditRepo.Log(ctx, entry); err != nil {
		// Log error but don't fail the request
		// TODO: Use proper logging
	}
}

// calculateEntryHash calculates a hash for the audit entry.
func (am *AuditMiddleware) calculateEntryHash(entry *storage.AuditEntry) string {
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%d",
		entry.Timestamp.Format(time.RFC3339Nano),
		entry.NodeID,
		entry.OperationID,
		entry.Action,
		entry.TableName,
		entry.Actor,
		entry.PrevHash,
		entry.DurationMS,
	)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// LogMutation logs a mutation operation directly from a service.
// Use this when you need more control over the audit entry.
func (am *AuditMiddleware) LogMutation(ctx context.Context, action, resource, resourceID, description string) error {
	if am == nil || !am.cfg.Enabled {
		return nil
	}

	actor := "anonymous"
	if user, ok := UserFromContext(ctx); ok {
		actor = string(user.ID)
	}

	clientIP := ""
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		clientIP = p.Addr.String()
	}

	operationID := generateOperationID()
	lastHash, _ := am.auditRepo.GetLastHash(ctx)

	entry := &storage.AuditEntry{
		Timestamp:       time.Now().UTC(),
		NodeID:          am.cfg.NodeID,
		OperationID:     operationID,
		RoleUsed:        "grpc",
		Action:          action,
		TableName:       resource,
		Query:           description,
		QueryHash:       hashString(description),
		RowsAffected:    1,
		DurationMS:      0,
		SourceComponent: "grpc",
		Actor:           actor,
		Metadata: map[string]any{
			"resource_id": resourceID,
			"client_ip":   clientIP,
		},
		PrevHash: lastHash,
	}

	entry.EntryHash = am.calculateEntryHash(entry)

	return am.auditRepo.Log(ctx, entry)
}

// generateOperationID generates a unique operation ID.
func generateOperationID() string {
	timestamp := time.Now().UnixNano()
	hash := sha256.Sum256([]byte(fmt.Sprintf("%d", timestamp)))
	return hex.EncodeToString(hash[:8])
}

// hashString creates a SHA256 hash of a string.
func hashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}

// extractResourceFromMethod extracts the resource name from a gRPC method.
func extractResourceFromMethod(method string) string {
	// Method format: /package.Service/Method
	parts := strings.Split(method, "/")
	if len(parts) < 3 {
		return "unknown"
	}

	servicePart := parts[1]
	methodName := parts[2]

	// Extract service name (e.g., "bib.v1.services.UserService" -> "User")
	serviceParts := strings.Split(servicePart, ".")
	if len(serviceParts) > 0 {
		service := serviceParts[len(serviceParts)-1]
		// Remove "Service" suffix
		resource := strings.TrimSuffix(service, "Service")

		// Add method context for clarity
		if strings.HasPrefix(methodName, "Create") ||
			strings.HasPrefix(methodName, "Update") ||
			strings.HasPrefix(methodName, "Delete") {
			return strings.ToLower(resource)
		}
		return strings.ToLower(resource)
	}

	return "unknown"
}
