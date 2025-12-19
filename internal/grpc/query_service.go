// Package grpc provides gRPC service implementations for the bib daemon.
package grpc

import (
	"context"

	services "bib/api/gen/go/bib/v1/services"
	"bib/internal/storage"

	"github.com/google/cel-go/cel"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// QueryServiceServer implements the QueryService gRPC service.
type QueryServiceServer struct {
	services.UnimplementedQueryServiceServer
	store       storage.Store
	auditLogger *AuditMiddleware
	celEnv      *cel.Env
}

// QueryServiceConfig holds configuration for the QueryServiceServer.
type QueryServiceConfig struct {
	Store       storage.Store
	AuditLogger *AuditMiddleware
}

// NewQueryServiceServer creates a new QueryServiceServer.
func NewQueryServiceServer() *QueryServiceServer {
	s := &QueryServiceServer{}
	s.initCELEnv()
	return s
}

// NewQueryServiceServerWithConfig creates a new QueryServiceServer with dependencies.
func NewQueryServiceServerWithConfig(cfg QueryServiceConfig) *QueryServiceServer {
	s := &QueryServiceServer{
		store:       cfg.Store,
		auditLogger: cfg.AuditLogger,
	}
	s.initCELEnv()
	return s
}

// initCELEnv initializes the CEL environment with bib-specific functions
func (s *QueryServiceServer) initCELEnv() {
	// Create a basic CEL environment with standard functions
	// This will be expanded in Phase 3 with bib-specific functions
	env, err := cel.NewEnv()
	if err == nil {
		s.celEnv = env
	}
}

// Execute runs a CEL query and returns results.
func (s *QueryServiceServer) Execute(ctx context.Context, req *services.ExecuteQueryRequest) (*services.ExecuteQueryResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.GetExpression() == "" {
		return nil, NewValidationError("expression is required", map[string]string{
			"expression": "must not be empty",
		})
	}

	// Validate the query first
	if s.celEnv == nil {
		return nil, status.Error(codes.Unavailable, "CEL environment not initialized")
	}

	_, issues := s.celEnv.Compile(req.GetExpression())
	if issues != nil && issues.Err() != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid CEL expression: %v", issues.Err())
	}

	// Audit log
	if s.auditLogger != nil {
		_ = s.auditLogger.LogMutation(ctx, "EXECUTE", "query", "", "Executed CEL query")
	}

	// For now, return a placeholder response
	// Full execution will be implemented in Phase 3
	return &services.ExecuteQueryResponse{
		Results:  []*services.QueryResult{},
		Warnings: []string{"Full query execution will be available in Phase 3"},
	}, nil
}

// ExecuteStream runs a query and streams results.
func (s *QueryServiceServer) ExecuteStream(req *services.ExecuteQueryRequest, stream services.QueryService_ExecuteStreamServer) error {
	if s.store == nil {
		return status.Error(codes.Unavailable, "service not initialized")
	}

	if req.GetExpression() == "" {
		return NewValidationError("expression is required", map[string]string{
			"expression": "must not be empty",
		})
	}

	// Validate the query first
	if s.celEnv == nil {
		return status.Error(codes.Unavailable, "CEL environment not initialized")
	}

	_, issues := s.celEnv.Compile(req.GetExpression())
	if issues != nil && issues.Err() != nil {
		return status.Errorf(codes.InvalidArgument, "invalid CEL expression: %v", issues.Err())
	}

	// For now, send a single result indicating the feature is pending
	err := stream.Send(&services.QueryResult{})
	if err != nil {
		return err
	}

	return nil
}

// Validate validates a CEL expression without executing.
func (s *QueryServiceServer) Validate(_ context.Context, req *services.ValidateQueryRequest) (*services.ValidateQueryResponse, error) {
	if req.GetExpression() == "" {
		return nil, NewValidationError("expression is required", map[string]string{
			"expression": "must not be empty",
		})
	}

	if s.celEnv == nil {
		return nil, status.Error(codes.Unavailable, "CEL environment not initialized")
	}

	ast, issues := s.celEnv.Compile(req.GetExpression())
	if issues != nil && issues.Err() != nil {
		return &services.ValidateQueryResponse{
			Valid:    false,
			Warnings: []string{issues.Err().Error()},
		}, nil
	}

	// Get output type
	resultType := ""
	if ast.OutputType() != nil {
		resultType = ast.OutputType().String()
	}

	return &services.ValidateQueryResponse{
		Valid:      true,
		ResultType: resultType,
	}, nil
}

// Explain explains query execution plan.
func (s *QueryServiceServer) Explain(_ context.Context, req *services.ExplainQueryRequest) (*services.ExplainQueryResponse, error) {
	if req.GetExpression() == "" {
		return nil, NewValidationError("expression is required", map[string]string{
			"expression": "must not be empty",
		})
	}

	if s.celEnv == nil {
		return nil, status.Error(codes.Unavailable, "CEL environment not initialized")
	}

	_, issues := s.celEnv.Compile(req.GetExpression())
	if issues != nil && issues.Err() != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid CEL expression: %v", issues.Err())
	}

	return &services.ExplainQueryResponse{
		Plan: &services.QueryPlan{
			Description: "CEL expression parsed successfully. Full execution plan will be available in Phase 3.",
		},
	}, nil
}

// ListFunctions lists available CEL functions.
func (s *QueryServiceServer) ListFunctions(_ context.Context, req *services.ListFunctionsRequest) (*services.ListFunctionsResponse, error) {
	functions := []*services.FunctionInfo{
		{
			Name:        "size",
			Description: "Returns the size of a string, bytes, list, or map",
			ReturnType:  "int",
			Category:    "standard",
		},
		{
			Name:        "contains",
			Description: "Checks if a string contains a substring or a list contains an element",
			ReturnType:  "bool",
			Category:    "standard",
		},
		{
			Name:        "startsWith",
			Description: "Checks if a string starts with a prefix",
			ReturnType:  "bool",
			Category:    "standard",
		},
		{
			Name:        "endsWith",
			Description: "Checks if a string ends with a suffix",
			ReturnType:  "bool",
			Category:    "standard",
		},
		{
			Name:        "matches",
			Description: "Checks if a string matches a regular expression",
			ReturnType:  "bool",
			Category:    "standard",
		},
		{
			Name:        "dataset",
			Description: "Accesses dataset data by name or ID (Phase 3)",
			ReturnType:  "Dataset",
			Category:    "bib",
		},
		{
			Name:        "topic",
			Description: "Accesses topic data by name or ID (Phase 3)",
			ReturnType:  "Topic",
			Category:    "bib",
		},
	}

	// Filter by category if specified
	if req.GetCategory() != "" {
		filtered := make([]*services.FunctionInfo, 0)
		for _, f := range functions {
			if f.Category == req.GetCategory() {
				filtered = append(filtered, f)
			}
		}
		functions = filtered
	}

	return &services.ListFunctionsResponse{
		Functions: functions,
	}, nil
}

// GetHistory returns recent queries.
func (s *QueryServiceServer) GetHistory(_ context.Context, _ *services.GetQueryHistoryRequest) (*services.GetQueryHistoryResponse, error) {
	// Query history will be implemented with storage layer support
	return &services.GetQueryHistoryResponse{
		Entries: []*services.QueryHistoryEntry{},
	}, nil
}

// SaveQuery saves a named query.
func (s *QueryServiceServer) SaveQuery(ctx context.Context, req *services.SaveQueryRequest) (*services.SaveQueryResponse, error) {
	violations := make(map[string]string)
	if req.GetName() == "" {
		violations["name"] = "must not be empty"
	}
	if req.GetExpression() == "" {
		violations["expression"] = "must not be empty"
	}
	if len(violations) > 0 {
		return nil, NewValidationError("invalid save query request", violations)
	}

	// Validate query syntax
	if s.celEnv != nil {
		_, issues := s.celEnv.Compile(req.GetExpression())
		if issues != nil && issues.Err() != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid CEL expression: %v", issues.Err())
		}
	}

	// Audit log
	if s.auditLogger != nil {
		_ = s.auditLogger.LogMutation(ctx, "CREATE", "saved_query", "", "Saved query: "+req.GetName())
	}

	return &services.SaveQueryResponse{
		Query: &services.SavedQuery{
			Name:       req.GetName(),
			Expression: req.GetExpression(),
		},
	}, nil
}

// ListSavedQueries lists saved queries.
func (s *QueryServiceServer) ListSavedQueries(_ context.Context, _ *services.ListSavedQueriesRequest) (*services.ListSavedQueriesResponse, error) {
	// Saved queries will be implemented with storage layer support
	return &services.ListSavedQueriesResponse{
		Queries: []*services.SavedQuery{},
	}, nil
}

// DeleteSavedQuery deletes a saved query.
func (s *QueryServiceServer) DeleteSavedQuery(ctx context.Context, req *services.DeleteSavedQueryRequest) (*services.DeleteSavedQueryResponse, error) {
	if req.GetId() == "" {
		return nil, NewValidationError("id is required", map[string]string{
			"id": "must not be empty",
		})
	}

	// Audit log
	if s.auditLogger != nil {
		_ = s.auditLogger.LogMutation(ctx, "DELETE", "saved_query", req.GetId(), "Deleted saved query")
	}

	return &services.DeleteSavedQueryResponse{
		Success: true,
	}, nil
}
