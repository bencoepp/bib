// Package grpc provides gRPC service implementations for the bib daemon.
package grpc

import (
	"context"
	"time"

	services "bib/api/gen/go/bib/v1/services"
	"bib/internal/storage/breakglass"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// BreakGlassServiceServer implements the BreakGlassService gRPC service.
type BreakGlassServiceServer struct {
	services.UnimplementedBreakGlassServiceServer
	manager     *breakglass.Manager
	auditLogger *AuditMiddleware
	nodeID      string
}

// BreakGlassServiceConfig holds configuration for BreakGlassServiceServer.
type BreakGlassServiceConfig struct {
	Manager     *breakglass.Manager
	AuditLogger *AuditMiddleware
	NodeID      string
}

// NewBreakGlassServiceServer creates a new BreakGlassServiceServer.
func NewBreakGlassServiceServer() *BreakGlassServiceServer {
	return &BreakGlassServiceServer{}
}

// NewBreakGlassServiceServerWithConfig creates a new BreakGlassServiceServer with dependencies.
func NewBreakGlassServiceServerWithConfig(cfg BreakGlassServiceConfig) *BreakGlassServiceServer {
	return &BreakGlassServiceServer{
		manager:     cfg.Manager,
		auditLogger: cfg.AuditLogger,
		nodeID:      cfg.NodeID,
	}
}

// GetStatus returns the current break glass configuration and session status.
func (s *BreakGlassServiceServer) GetStatus(ctx context.Context, req *services.GetBreakGlassStatusRequest) (*services.GetBreakGlassStatusResponse, error) {
	if s.manager == nil {
		return nil, status.Error(codes.Unavailable, "break glass service not initialized")
	}

	resp := &services.GetBreakGlassStatusResponse{
		Config: &services.BreakGlassConfig{
			Enabled: s.manager.IsEnabled(),
		},
		ActiveSession: nil,
	}

	// Get current session if any
	session := s.manager.GetSession()
	if session != nil && session.IsActive() {
		resp.ActiveSession = sessionToProto(session)
	}

	// Get pending acknowledgments count
	pendingReports := s.manager.GetPendingReports()
	resp.PendingAcknowledgmentCount = int32(len(pendingReports))

	return resp, nil
}

// CreateChallenge creates an authentication challenge for a user.
func (s *BreakGlassServiceServer) CreateChallenge(ctx context.Context, req *services.CreateBreakGlassChallengeRequest) (*services.CreateBreakGlassChallengeResponse, error) {
	if s.manager == nil {
		return nil, status.Error(codes.Unavailable, "break glass service not initialized")
	}

	if req.Username == "" {
		return nil, NewValidationError("username is required", map[string]string{
			"username": "must not be empty",
		})
	}

	challenge, err := s.manager.CreateChallenge(req.Username)
	if err != nil {
		// Don't reveal whether user exists
		return nil, status.Errorf(codes.InvalidArgument, "failed to create challenge: %v", err)
	}

	return &services.CreateBreakGlassChallengeResponse{
		ChallengeId: challenge.ID,
		Nonce:       challenge.Nonce,
		ExpiresAt:   timestamppb.New(challenge.ExpiresAt),
	}, nil
}

// EnableSession enables a break glass session after successful authentication.
func (s *BreakGlassServiceServer) EnableSession(ctx context.Context, req *services.EnableBreakGlassSessionRequest) (*services.EnableBreakGlassSessionResponse, error) {
	if s.manager == nil {
		return nil, status.Error(codes.Unavailable, "break glass service not initialized")
	}

	// Validate required fields
	violations := make(map[string]string)
	if req.ChallengeId == "" {
		violations["challenge_id"] = "must not be empty"
	}
	if len(req.Signature) == 0 {
		violations["signature"] = "must not be empty"
	}
	if req.Reason == "" {
		violations["reason"] = "must not be empty"
	}
	if len(violations) > 0 {
		return nil, NewValidationError("invalid enable session request", violations)
	}

	// Verify the challenge
	user, err := s.manager.VerifyChallenge(req.ChallengeId, req.Signature)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "authentication failed: %v", err)
	}

	// Parse duration
	var duration time.Duration
	if req.Duration != nil {
		duration = req.Duration.AsDuration()
	}

	// Get requester info
	requestedBy := "unknown"
	if authUser, ok := UserFromContext(ctx); ok && authUser != nil {
		requestedBy = authUser.Name
	}

	// Enable the session
	session, err := s.manager.Enable(ctx, user, req.Reason, duration, requestedBy)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "failed to enable session: %v", err)
	}

	// Audit log
	if s.auditLogger != nil {
		_ = s.auditLogger.LogMutation(ctx, "DDL", "breakglass", session.ID, "Break glass session enabled: "+req.Reason)
	}

	return &services.EnableBreakGlassSessionResponse{
		Session:          sessionToProto(session),
		ConnectionString: session.ConnectionString,
	}, nil
}

// DisableSession disables an active break glass session.
func (s *BreakGlassServiceServer) DisableSession(ctx context.Context, req *services.DisableBreakGlassSessionRequest) (*services.DisableBreakGlassSessionResponse, error) {
	if s.manager == nil {
		return nil, status.Error(codes.Unavailable, "break glass service not initialized")
	}

	// Get the user who is disabling
	disabledBy := "unknown"
	if authUser, ok := UserFromContext(ctx); ok && authUser != nil {
		disabledBy = authUser.Name
	}

	report, err := s.manager.Disable(ctx, disabledBy)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "failed to disable session: %v", err)
	}

	// Audit log
	if s.auditLogger != nil {
		_ = s.auditLogger.LogMutation(ctx, "DDL", "breakglass", report.Session.ID, "Break glass session disabled")
	}

	return &services.DisableBreakGlassSessionResponse{
		Report: sessionReportToProto(report),
	}, nil
}

// GetPendingAcknowledgments returns sessions that need to be acknowledged.
func (s *BreakGlassServiceServer) GetPendingAcknowledgments(ctx context.Context, req *services.GetPendingAcknowledgmentsRequest) (*services.GetPendingAcknowledgmentsResponse, error) {
	if s.manager == nil {
		return nil, status.Error(codes.Unavailable, "break glass service not initialized")
	}

	reports := s.manager.GetPendingReports()

	protoReports := make([]*services.BreakGlassSessionReport, len(reports))
	for i, r := range reports {
		protoReports[i] = sessionReportToProto(r)
	}

	return &services.GetPendingAcknowledgmentsResponse{
		Reports: protoReports,
	}, nil
}

// AcknowledgeSession acknowledges a completed break glass session.
func (s *BreakGlassServiceServer) AcknowledgeSession(ctx context.Context, req *services.AcknowledgeBreakGlassSessionRequest) (*services.AcknowledgeBreakGlassSessionResponse, error) {
	if s.manager == nil {
		return nil, status.Error(codes.Unavailable, "break glass service not initialized")
	}

	if req.SessionId == "" {
		return nil, NewValidationError("session_id is required", map[string]string{
			"session_id": "must not be empty",
		})
	}

	// Get acknowledger info
	acknowledgedBy := "unknown"
	if authUser, ok := UserFromContext(ctx); ok && authUser != nil {
		acknowledgedBy = authUser.Name
	}

	if err := s.manager.Acknowledge(ctx, req.SessionId, acknowledgedBy); err != nil {
		return nil, status.Errorf(codes.NotFound, "failed to acknowledge session: %v", err)
	}

	// Audit log
	if s.auditLogger != nil {
		_ = s.auditLogger.LogMutation(ctx, "UPDATE", "breakglass", req.SessionId, "Break glass session acknowledged")
	}

	return &services.AcknowledgeBreakGlassSessionResponse{
		Success: true,
	}, nil
}

// GetSessionReport returns the detailed report for a break glass session.
func (s *BreakGlassServiceServer) GetSessionReport(ctx context.Context, req *services.GetBreakGlassSessionReportRequest) (*services.GetBreakGlassSessionReportResponse, error) {
	if s.manager == nil {
		return nil, status.Error(codes.Unavailable, "break glass service not initialized")
	}

	if req.SessionId == "" {
		return nil, NewValidationError("session_id is required", map[string]string{
			"session_id": "must not be empty",
		})
	}

	report, err := s.manager.GetReport(req.SessionId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "session report not found: %v", err)
	}

	return &services.GetBreakGlassSessionReportResponse{
		Report: sessionReportToProto(report),
	}, nil
}

// ListSessions lists all break glass sessions.
func (s *BreakGlassServiceServer) ListSessions(ctx context.Context, req *services.ListBreakGlassSessionsRequest) (*services.ListBreakGlassSessionsResponse, error) {
	if s.manager == nil {
		return nil, status.Error(codes.Unavailable, "break glass service not initialized")
	}

	var sessions []*services.BreakGlassSession

	// Get current active session
	currentSession := s.manager.GetSession()
	if currentSession != nil {
		// Filter by state if requested (UNSPECIFIED means no filter)
		if req.State == services.BreakGlassSessionState_BREAK_GLASS_SESSION_STATE_UNSPECIFIED ||
			matchesSessionState(currentSession.State, req.State) {
			sessions = append(sessions, sessionToProto(currentSession))
		}
	}

	// Get sessions from pending reports
	reports := s.manager.GetPendingReports()
	for _, r := range reports {
		if req.State == services.BreakGlassSessionState_BREAK_GLASS_SESSION_STATE_UNSPECIFIED ||
			matchesSessionState(r.Session.State, req.State) {
			sessions = append(sessions, sessionToProto(r.Session))
		}
	}

	return &services.ListBreakGlassSessionsResponse{
		Sessions:   sessions,
		TotalCount: int32(len(sessions)),
	}, nil
}

// matchesSessionState checks if internal state matches proto state.
func matchesSessionState(internal breakglass.SessionState, proto services.BreakGlassSessionState) bool {
	switch proto {
	case services.BreakGlassSessionState_BREAK_GLASS_SESSION_STATE_ACTIVE:
		return internal == breakglass.StateActive
	case services.BreakGlassSessionState_BREAK_GLASS_SESSION_STATE_EXPIRED:
		return internal == breakglass.StateExpired
	case services.BreakGlassSessionState_BREAK_GLASS_SESSION_STATE_PENDING_ACK:
		return internal == breakglass.StatePendingAck
	default:
		return false
	}
}

// Helper functions for proto conversion

func sessionToProto(s *breakglass.Session) *services.BreakGlassSession {
	if s == nil {
		return nil
	}
	return &services.BreakGlassSession{
		Id:          s.ID,
		Username:    s.User.Name,
		Reason:      s.Reason,
		AccessLevel: accessLevelToProto(s.AccessLevel),
		State:       sessionStateToProto(s.State),
		StartedAt:   timestamppb.New(s.StartedAt),
		ExpiresAt:   timestamppb.New(s.ExpiresAt),
		NodeId:      s.NodeID,
		RequestedBy: s.RequestedBy,
	}
}

func sessionReportToProto(r *breakglass.SessionReport) *services.BreakGlassSessionReport {
	if r == nil {
		return nil
	}

	report := &services.BreakGlassSessionReport{
		Session:        sessionToProto(r.Session),
		EndedAt:        timestamppb.New(r.EndedAt),
		QueryCount:     r.QueryCount,
		TablesAccessed: r.TablesAccessed,
	}

	if r.Duration > 0 {
		report.Duration = durationpb.New(r.Duration)
	}

	if r.AcknowledgedAt != nil {
		report.AcknowledgedAt = timestamppb.New(*r.AcknowledgedAt)
		report.AcknowledgedBy = r.AcknowledgedBy
	}

	// Convert operation counts to proto map
	if len(r.OperationCounts) > 0 {
		report.OperationCounts = make(map[string]int64)
		for op, count := range r.OperationCounts {
			report.OperationCounts[op] = count
		}
	}

	return report
}

func accessLevelToProto(level breakglass.AccessLevel) services.BreakGlassAccessLevel {
	switch level {
	case breakglass.AccessReadOnly:
		return services.BreakGlassAccessLevel_BREAK_GLASS_ACCESS_LEVEL_READONLY
	case breakglass.AccessReadWrite:
		return services.BreakGlassAccessLevel_BREAK_GLASS_ACCESS_LEVEL_READWRITE
	default:
		return services.BreakGlassAccessLevel_BREAK_GLASS_ACCESS_LEVEL_UNSPECIFIED
	}
}

func sessionStateToProto(state breakglass.SessionState) services.BreakGlassSessionState {
	switch state {
	case breakglass.StateActive:
		return services.BreakGlassSessionState_BREAK_GLASS_SESSION_STATE_ACTIVE
	case breakglass.StateExpired:
		return services.BreakGlassSessionState_BREAK_GLASS_SESSION_STATE_EXPIRED
	case breakglass.StatePendingAck:
		return services.BreakGlassSessionState_BREAK_GLASS_SESSION_STATE_PENDING_ACK
	case breakglass.StateInactive:
		return services.BreakGlassSessionState_BREAK_GLASS_SESSION_STATE_DISABLED
	default:
		return services.BreakGlassSessionState_BREAK_GLASS_SESSION_STATE_UNSPECIFIED
	}
}
