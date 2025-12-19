// Package grpc provides gRPC service implementations for the bib daemon.
package grpc

import (
	"context"

	services "bib/api/gen/go/bib/v1/services"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// QueryServiceServer implements the QueryService gRPC service.
type QueryServiceServer struct {
	services.UnimplementedQueryServiceServer
}

// NewQueryServiceServer creates a new QueryServiceServer.
func NewQueryServiceServer() *QueryServiceServer {
	return &QueryServiceServer{}
}

// Execute runs a CEL query and returns results.
func (s *QueryServiceServer) Execute(ctx context.Context, req *services.ExecuteQueryRequest) (*services.ExecuteQueryResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Execute not implemented")
}

// ExecuteStream runs a query and streams results.
func (s *QueryServiceServer) ExecuteStream(req *services.ExecuteQueryRequest, stream services.QueryService_ExecuteStreamServer) error {
	return status.Errorf(codes.Unimplemented, "method ExecuteStream not implemented")
}

// ValidateQuery validates a CEL expression without executing.
func (s *QueryServiceServer) ValidateQuery(ctx context.Context, req *services.ValidateQueryRequest) (*services.ValidateQueryResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ValidateQuery not implemented")
}

// ExplainQuery explains query execution plan.
func (s *QueryServiceServer) ExplainQuery(ctx context.Context, req *services.ExplainQueryRequest) (*services.ExplainQueryResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ExplainQuery not implemented")
}

// ListFunctions lists available CEL functions.
func (s *QueryServiceServer) ListFunctions(ctx context.Context, req *services.ListFunctionsRequest) (*services.ListFunctionsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListFunctions not implemented")
}

// GetQueryHistory returns recent queries.
func (s *QueryServiceServer) GetQueryHistory(ctx context.Context, req *services.GetQueryHistoryRequest) (*services.GetQueryHistoryResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetQueryHistory not implemented")
}

// SaveQuery saves a named query.
func (s *QueryServiceServer) SaveQuery(ctx context.Context, req *services.SaveQueryRequest) (*services.SaveQueryResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SaveQuery not implemented")
}

// ListSavedQueries lists saved queries.
func (s *QueryServiceServer) ListSavedQueries(ctx context.Context, req *services.ListSavedQueriesRequest) (*services.ListSavedQueriesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListSavedQueries not implemented")
}

// DeleteSavedQuery deletes a saved query.
func (s *QueryServiceServer) DeleteSavedQuery(ctx context.Context, req *services.DeleteSavedQueryRequest) (*services.DeleteSavedQueryResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteSavedQuery not implemented")
}
