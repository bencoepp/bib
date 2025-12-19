// Package grpc provides gRPC service implementations for the bib daemon.
package grpc

import (
	"context"

	services "bib/api/gen/go/bib/v1/services"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// BreakGlassServiceServer implements the BreakGlassService gRPC service.
type BreakGlassServiceServer struct {
	services.UnimplementedBreakGlassServiceServer
}

// NewBreakGlassServiceServer creates a new BreakGlassServiceServer.
func NewBreakGlassServiceServer() *BreakGlassServiceServer {
	return &BreakGlassServiceServer{}
}

// GetStatus returns the current break glass configuration and session status.
func (s *BreakGlassServiceServer) GetStatus(ctx context.Context, req *services.GetBreakGlassStatusRequest) (*services.GetBreakGlassStatusResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetStatus not implemented")
}

// CreateChallenge creates an authentication challenge for a user.
func (s *BreakGlassServiceServer) CreateChallenge(ctx context.Context, req *services.CreateBreakGlassChallengeRequest) (*services.CreateBreakGlassChallengeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateChallenge not implemented")
}

// EnableSession enables a break glass session after successful authentication.
func (s *BreakGlassServiceServer) EnableSession(ctx context.Context, req *services.EnableBreakGlassSessionRequest) (*services.EnableBreakGlassSessionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method EnableSession not implemented")
}

// DisableSession disables an active break glass session.
func (s *BreakGlassServiceServer) DisableSession(ctx context.Context, req *services.DisableBreakGlassSessionRequest) (*services.DisableBreakGlassSessionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DisableSession not implemented")
}

// GetPendingAcknowledgments returns sessions that need to be acknowledged.
func (s *BreakGlassServiceServer) GetPendingAcknowledgments(ctx context.Context, req *services.GetPendingAcknowledgmentsRequest) (*services.GetPendingAcknowledgmentsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetPendingAcknowledgments not implemented")
}

// AcknowledgeSession acknowledges a completed break glass session.
func (s *BreakGlassServiceServer) AcknowledgeSession(ctx context.Context, req *services.AcknowledgeBreakGlassSessionRequest) (*services.AcknowledgeBreakGlassSessionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AcknowledgeSession not implemented")
}

// GetSessionReport returns the detailed report for a break glass session.
func (s *BreakGlassServiceServer) GetSessionReport(ctx context.Context, req *services.GetBreakGlassSessionReportRequest) (*services.GetBreakGlassSessionReportResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetSessionReport not implemented")
}

// ListSessions lists all break glass sessions.
func (s *BreakGlassServiceServer) ListSessions(ctx context.Context, req *services.ListBreakGlassSessionsRequest) (*services.ListBreakGlassSessionsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListSessions not implemented")
}
