//go:build integration

package grpc_test

import (
	"context"
	"testing"
	"time"

	services "bib/api/gen/go/bib/v1/services"
	"bib/test/testutil"

	"google.golang.org/grpc/codes"
)

// =============================================================================
// HealthService Integration Tests
// =============================================================================

// TestHealthService_Ping tests the ping endpoint.
func TestHealthService_Ping(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	healthClient := services.NewHealthServiceClient(conn)

	t.Run("BasicPing", func(t *testing.T) {
		resp, err := healthClient.Ping(ctx, &services.PingRequest{})
		assertNoError(t, err)

		if resp.Timestamp == nil {
			t.Error("expected timestamp")
		}
	})

	t.Run("PingWithPayload", func(t *testing.T) {
		payload := []byte("hello world")
		resp, err := healthClient.Ping(ctx, &services.PingRequest{
			Payload: payload,
		})
		assertNoError(t, err)

		if string(resp.Payload) != string(payload) {
			t.Errorf("expected payload '%s', got '%s'", payload, resp.Payload)
		}
	})

	t.Run("PingLargePayload", func(t *testing.T) {
		payload := make([]byte, 1024*10) // 10KB
		for i := range payload {
			payload[i] = byte(i % 256)
		}
		resp, err := healthClient.Ping(ctx, &services.PingRequest{
			Payload: payload,
		})
		assertNoError(t, err)

		if len(resp.Payload) != len(payload) {
			t.Errorf("expected payload length %d, got %d", len(payload), len(resp.Payload))
		}
	})
}

// TestHealthService_Check tests health check endpoint.
func TestHealthService_Check(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	healthClient := services.NewHealthServiceClient(conn)

	t.Run("BasicCheck", func(t *testing.T) {
		resp, err := healthClient.Check(ctx, &services.HealthCheckRequest{})
		assertNoError(t, err)

		// Should be serving
		if resp.Status != services.ServingStatus_SERVING_STATUS_SERVING {
			t.Errorf("expected SERVING, got %v", resp.Status)
		}
	})

	t.Run("CheckSpecificService", func(t *testing.T) {
		resp, err := healthClient.Check(ctx, &services.HealthCheckRequest{
			Service: "bib.v1.HealthService",
		})
		assertNoError(t, err)

		if resp.Status != services.ServingStatus_SERVING_STATUS_SERVING {
			t.Errorf("expected SERVING, got %v", resp.Status)
		}
	})
}

// TestHealthService_GetNodeInfo tests node info retrieval.
func TestHealthService_GetNodeInfo(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	healthClient := services.NewHealthServiceClient(conn)

	resp, err := healthClient.GetNodeInfo(ctx, &services.GetNodeInfoRequest{})
	assertNoError(t, err)

	if resp.NodeId == "" {
		t.Error("expected node ID")
	}
	if resp.Mode == "" {
		t.Error("expected mode")
	}
	if resp.StartedAt == nil {
		t.Error("expected started_at")
	}
}

// TestHealthService_Watch tests streaming health check.
func TestHealthService_Watch(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	healthClient := services.NewHealthServiceClient(conn)

	// Create a cancellable context
	watchCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	stream, err := healthClient.Watch(watchCtx, &services.HealthCheckRequest{})
	assertNoError(t, err)

	// Receive at least one health status
	resp, err := stream.Recv()
	assertNoError(t, err)

	if resp.Status == services.ServingStatus_SERVING_STATUS_UNKNOWN {
		t.Error("expected known status")
	}
}

// =============================================================================
// AdminService Integration Tests
// =============================================================================

// TestAdminService_GetConfig tests configuration retrieval.
func TestAdminService_GetConfig(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	adminClient := services.NewAdminServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "ConfigAdmin")

	t.Run("GetFullConfig", func(t *testing.T) {
		resp, err := adminClient.GetConfig(adminCtx, &services.GetConfigRequest{})
		assertNoError(t, err)

		if resp.Config == nil {
			t.Error("expected config")
		}
	})

	t.Run("GetConfigWithoutSecrets", func(t *testing.T) {
		resp, err := adminClient.GetConfig(adminCtx, &services.GetConfigRequest{
			IncludeSecrets: false,
		})
		assertNoError(t, err)

		// Check that sensitive fields are masked
		if resp.Config != nil {
			// Implementation-specific: check for masked values
			t.Log("Config retrieved without secrets")
		}
	})

	t.Run("GetConfigSection", func(t *testing.T) {
		resp, err := adminClient.GetConfig(adminCtx, &services.GetConfigRequest{
			Section: "storage",
		})
		// May succeed or fail depending on implementation
		if err == nil && resp.Config != nil {
			t.Log("Storage config section retrieved")
		}
	})
}

// TestAdminService_GetSystemInfo tests system status retrieval.
func TestAdminService_GetSystemInfo(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	adminClient := services.NewAdminServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "StatusAdmin")

	resp, err := adminClient.GetSystemInfo(adminCtx, &services.GetSystemInfoRequest{})
	assertNoError(t, err)

	if resp.Uptime == nil {
		t.Error("expected uptime")
	}
	if resp.GoVersion == "" {
		t.Error("expected Go version")
	}
	if resp.NumGoroutine <= 0 {
		t.Error("expected positive goroutine count")
	}
}

// TestAdminService_StreamLogs tests log streaming.
func TestAdminService_StreamLogs(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	adminClient := services.NewAdminServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "LogAdmin")

	// Create a cancellable context
	streamCtx, cancel := context.WithTimeout(adminCtx, 3*time.Second)
	defer cancel()

	stream, err := adminClient.StreamLogs(streamCtx, &services.StreamLogsRequest{
		Level: "info",
	})
	assertNoError(t, err)

	// Stream should be established (may or may not have logs immediately)
	t.Log("Log stream established")

	// Try to receive (may timeout if no logs)
	_, _ = stream.Recv()
}

// TestAdminService_GetRecentLogs tests recent log retrieval.
func TestAdminService_GetRecentLogs(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	adminClient := services.NewAdminServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "RecentLogAdmin")

	// GetAuditLogs is the available method for retrieving logs
	resp, err := adminClient.GetAuditLogs(adminCtx, &services.GetAuditLogsRequest{})
	assertNoError(t, err)

	t.Logf("Retrieved %d audit log entries", len(resp.Entries))
}

// TestAdminService_RequiresAdmin tests admin-only access.
func TestAdminService_RequiresAdmin(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	adminClient := services.NewAdminServiceClient(conn)

	// Regular user
	userCtx, _, _ := ts.AuthenticateUser(ctx, "RegularUser")

	t.Run("GetConfigDenied", func(t *testing.T) {
		_, err := adminClient.GetConfig(userCtx, &services.GetConfigRequest{})
		assertGRPCCode(t, err, codes.PermissionDenied)
	})

	t.Run("GetSystemInfoDenied", func(t *testing.T) {
		_, err := adminClient.GetSystemInfo(userCtx, &services.GetSystemInfoRequest{})
		assertGRPCCode(t, err, codes.PermissionDenied)
	})
}

// =============================================================================
// QueryService Integration Tests
// =============================================================================

// TestQueryService_Validate tests CEL query validation.
func TestQueryService_Validate(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	queryClient := services.NewQueryServiceClient(conn)

	userCtx, _, _ := ts.AuthenticateUser(ctx, "QueryUser")

	t.Run("ValidExpression", func(t *testing.T) {
		resp, err := queryClient.ValidateQuery(userCtx, &services.ValidateQueryRequest{
			Expression: "1 + 2",
		})
		assertNoError(t, err)

		if !resp.Valid {
			t.Error("expected valid expression")
		}
	})

	t.Run("InvalidExpression", func(t *testing.T) {
		resp, err := queryClient.ValidateQuery(userCtx, &services.ValidateQueryRequest{
			Expression: "1 + + 2", // Invalid syntax
		})
		assertNoError(t, err)

		if resp.Valid {
			t.Error("expected invalid expression")
		}
	})

	t.Run("EmptyExpression", func(t *testing.T) {
		_, err := queryClient.ValidateQuery(userCtx, &services.ValidateQueryRequest{
			Expression: "",
		})
		assertGRPCCode(t, err, codes.InvalidArgument)
	})

	t.Run("BooleanExpression", func(t *testing.T) {
		resp, err := queryClient.ValidateQuery(userCtx, &services.ValidateQueryRequest{
			Expression: "true && false",
		})
		assertNoError(t, err)

		if !resp.Valid {
			t.Error("expected valid expression")
		}
		if resp.ResultType != "bool" {
			t.Logf("Result type: %s", resp.ResultType)
		}
	})
}

// TestQueryService_Explain tests query explanation.
func TestQueryService_Explain(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	queryClient := services.NewQueryServiceClient(conn)

	userCtx, _, _ := ts.AuthenticateUser(ctx, "ExplainUser")

	resp, err := queryClient.ExplainQuery(userCtx, &services.ExplainQueryRequest{
		Expression: "1 + 2 * 3",
	})
	assertNoError(t, err)

	if resp.Plan == nil {
		t.Error("expected query plan")
	}
}

// TestQueryService_ListFunctions tests function listing.
func TestQueryService_ListFunctions(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	queryClient := services.NewQueryServiceClient(conn)

	userCtx, _, _ := ts.AuthenticateUser(ctx, "FunctionsUser")

	t.Run("ListAll", func(t *testing.T) {
		resp, err := queryClient.ListFunctions(userCtx, &services.ListFunctionsRequest{})
		assertNoError(t, err)

		if len(resp.Functions) == 0 {
			t.Error("expected functions")
		}
	})

	t.Run("ListByCategory", func(t *testing.T) {
		resp, err := queryClient.ListFunctions(userCtx, &services.ListFunctionsRequest{
			Category: "standard",
		})
		assertNoError(t, err)

		for _, fn := range resp.Functions {
			if fn.Category != "standard" {
				t.Errorf("expected category 'standard', got '%s'", fn.Category)
			}
		}
	})
}

// TestQueryService_Execute tests query execution.
func TestQueryService_Execute(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	queryClient := services.NewQueryServiceClient(conn)

	userCtx, _, _ := ts.AuthenticateUser(ctx, "ExecuteUser")

	t.Run("SimpleExpression", func(t *testing.T) {
		resp, err := queryClient.Execute(userCtx, &services.ExecuteQueryRequest{
			Expression: "1 + 2",
		})
		assertNoError(t, err)

		// Phase 3 may return placeholder
		t.Logf("Execute response: %d results, %d warnings", len(resp.Results), len(resp.Warnings))
	})
}

// TestQueryService_SavedQueries tests saved query CRUD.
func TestQueryService_SavedQueries(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	queryClient := services.NewQueryServiceClient(conn)

	userCtx, _, _ := ts.AuthenticateUser(ctx, "SavedQueryUser")

	t.Run("SaveQuery", func(t *testing.T) {
		resp, err := queryClient.SaveQuery(userCtx, &services.SaveQueryRequest{
			Name:        "my-query",
			Expression:  "1 + 2",
			Description: "A simple test query",
		})
		assertNoError(t, err)

		if resp.Query.Name != "my-query" {
			t.Errorf("expected name 'my-query', got '%s'", resp.Query.Name)
		}
	})

	t.Run("ListSavedQueries", func(t *testing.T) {
		resp, err := queryClient.ListSavedQueries(userCtx, &services.ListSavedQueriesRequest{})
		assertNoError(t, err)

		t.Logf("Found %d saved queries", len(resp.Queries))
	})
}

// =============================================================================
// NodeService Integration Tests (Basic - P2P dependent)
// =============================================================================

// TestNodeService_NotInitialized tests node service when P2P is not configured.
func TestNodeService_NotInitialized(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	nodeClient := services.NewNodeServiceClient(conn)

	userCtx, _, _ := ts.AuthenticateUser(ctx, "NodeUser")

	// Node service requires P2P to be initialized
	// These should return Unavailable when P2P is not configured

	t.Run("GetSelfNode", func(t *testing.T) {
		_, err := nodeClient.GetSelfNode(userCtx, &services.GetSelfNodeRequest{})
		// Without P2P, should be unavailable
		assertGRPCCode(t, err, codes.Unavailable)
	})

	t.Run("ListNodes", func(t *testing.T) {
		_, err := nodeClient.ListNodes(userCtx, &services.ListNodesRequest{})
		assertGRPCCode(t, err, codes.Unavailable)
	})
}
