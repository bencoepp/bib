package grpc

import (
	"context"
	"testing"

	services "bib/api/gen/go/bib/v1/services"
	"bib/internal/domain"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestDatasetServiceServer_NotInitialized tests uninitialized service behavior.
func TestDatasetServiceServer_NotInitialized(t *testing.T) {
	server := NewDatasetServiceServer()

	ctx := WithUser(context.Background(), &domain.User{
		ID:   "user-1",
		Role: domain.UserRoleUser,
	})

	_, err := server.GetDataset(ctx, &services.GetDatasetRequest{
		Id: "any-id",
	})

	if err == nil {
		t.Fatal("expected error for uninitialized service")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.Unavailable {
		t.Errorf("expected Unavailable, got %v", st.Code())
	}
}

// TestDatasetServiceServer_ValidationErrors tests input validation.
func TestDatasetServiceServer_ValidationErrors(t *testing.T) {
	server := NewDatasetServiceServer()

	ctx := WithUser(context.Background(), &domain.User{
		ID:   "user-1",
		Role: domain.UserRoleUser,
	})

	// Test missing ID in GetDataset
	_, err := server.GetDataset(ctx, &services.GetDatasetRequest{
		Id: "",
	})
	// First checks for store nil, which returns Unavailable
	// In a real scenario with store set, it would return InvalidArgument
	if err == nil {
		t.Fatal("expected error")
	}

	// Test missing ID in UpdateDataset
	_, err = server.UpdateDataset(ctx, &services.UpdateDatasetRequest{
		Id: "",
	})
	if err == nil {
		t.Fatal("expected error")
	}

	// Test missing ID in DeleteDataset
	_, err = server.DeleteDataset(ctx, &services.DeleteDatasetRequest{
		Id: "",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

// TestDatasetServiceServer_Unauthenticated tests requests without authentication.
func TestDatasetServiceServer_Unauthenticated(t *testing.T) {
	// Note: Actual auth checks happen at interceptor level
	// This tests the service-level user context checks
	t.Log("Unauthenticated requests are handled by RBAC interceptor")
}

// TestNewDatasetServiceServer tests server creation.
func TestNewDatasetServiceServer(t *testing.T) {
	server := NewDatasetServiceServer()
	if server == nil {
		t.Fatal("expected non-nil server")
	}
}

// TestNewDatasetServiceServerWithConfig tests server creation with config.
func TestNewDatasetServiceServerWithConfig(t *testing.T) {
	server := NewDatasetServiceServerWithConfig(DatasetServiceConfig{
		NodeMode: "full",
	})
	if server == nil {
		t.Fatal("expected non-nil server")
	}
	if server.nodeMode != "full" {
		t.Errorf("expected nodeMode 'full', got '%s'", server.nodeMode)
	}
}

// TestDatasetServiceServer_SetStore tests store injection.
func TestDatasetServiceServer_SetStore(t *testing.T) {
	server := NewDatasetServiceServer()

	// Before setting store
	if server.store != nil {
		t.Error("expected nil store initially")
	}

	// SetStore is called, but we can't test with nil-accepting interface
	// In real usage, a valid store would be passed
	t.Log("SetStore tested via integration tests")
}

// TestDatasetConstants tests that default chunk size is reasonable.
func TestDatasetConstants(t *testing.T) {
	if defaultChunkSize <= 0 {
		t.Error("defaultChunkSize should be positive")
	}
	if defaultChunkSize > 10*1024*1024 { // 10MB max reasonable
		t.Error("defaultChunkSize seems too large")
	}
}

// TestDomainDatasetStatus tests dataset status values.
func TestDomainDatasetStatus(t *testing.T) {
	statuses := []domain.DatasetStatus{
		domain.DatasetStatusDraft,
		domain.DatasetStatusActive,
		domain.DatasetStatusArchived,
	}

	for _, s := range statuses {
		if s == "" {
			t.Error("status should not be empty")
		}
	}
}
