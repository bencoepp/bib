//go:build integration

package grpc_test

import (
	"context"
	"strings"
	"testing"
	"time"

	bibv1 "bib/api/gen/go/bib/v1"
	services "bib/api/gen/go/bib/v1/services"
	"bib/test/testutil"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

// =============================================================================
// Error Handling Tests
// =============================================================================

// TestErrors_InvalidInput tests various invalid input scenarios.
func TestErrors_InvalidInput(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	topicClient := services.NewTopicServiceClient(conn)
	datasetClient := services.NewDatasetServiceClient(conn)
	userClient := services.NewUserServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "InputErrorAdmin")

	tests := []struct {
		name     string
		testFunc func() error
		wantCode codes.Code
	}{
		{
			name: "Topic with empty name",
			testFunc: func() error {
				_, err := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{Name: ""})
				return err
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "Topic with very long name",
			testFunc: func() error {
				longName := strings.Repeat("a", 10000)
				_, err := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{Name: longName})
				return err
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "Get topic with empty ID",
			testFunc: func() error {
				_, err := topicClient.GetTopic(adminCtx, &services.GetTopicRequest{Id: ""})
				return err
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "Get dataset with empty ID",
			testFunc: func() error {
				_, err := datasetClient.GetDataset(adminCtx, &services.GetDatasetRequest{Id: ""})
				return err
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "Get user with empty ID",
			testFunc: func() error {
				_, err := userClient.GetUser(adminCtx, &services.GetUserRequest{UserId: ""})
				return err
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "Search with empty query",
			testFunc: func() error {
				_, err := userClient.SearchUsers(adminCtx, &services.SearchUsersRequest{Query: ""})
				return err
			},
			wantCode: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.testFunc()
			assertGRPCCode(t, err, tt.wantCode)
		})
	}
}

// TestErrors_NotFound tests not found error scenarios.
func TestErrors_NotFound(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	topicClient := services.NewTopicServiceClient(conn)
	datasetClient := services.NewDatasetServiceClient(conn)
	userClient := services.NewUserServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "NotFoundAdmin")

	tests := []struct {
		name     string
		testFunc func() error
	}{
		{
			name: "Get non-existent topic",
			testFunc: func() error {
				_, err := topicClient.GetTopic(adminCtx, &services.GetTopicRequest{Id: "non-existent-topic-xyz"})
				return err
			},
		},
		{
			name: "Get non-existent topic by name",
			testFunc: func() error {
				_, err := topicClient.GetTopic(adminCtx, &services.GetTopicRequest{Name: "non-existent-topic-name"})
				return err
			},
		},
		{
			name: "Update non-existent topic",
			testFunc: func() error {
				desc := "test"
				_, err := topicClient.UpdateTopic(adminCtx, &services.UpdateTopicRequest{
					Id:          "non-existent-id",
					Description: &desc,
				})
				return err
			},
		},
		{
			name: "Delete non-existent topic",
			testFunc: func() error {
				_, err := topicClient.DeleteTopic(adminCtx, &services.DeleteTopicRequest{Id: "non-existent-id"})
				return err
			},
		},
		{
			name: "Get non-existent dataset",
			testFunc: func() error {
				_, err := datasetClient.GetDataset(adminCtx, &services.GetDatasetRequest{Id: "non-existent-dataset"})
				return err
			},
		},
		{
			name: "Get non-existent user",
			testFunc: func() error {
				_, err := userClient.GetUser(adminCtx, &services.GetUserRequest{UserId: "non-existent-user"})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.testFunc()
			assertGRPCCode(t, err, codes.NotFound)
		})
	}
}

// TestErrors_Unauthenticated tests authentication errors.
func TestErrors_Unauthenticated(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	authClient := services.NewAuthServiceClient(conn)

	pubKey, privKey := generateTestKeyPair(t)

	// Get a challenge
	challengeResp, err := authClient.Challenge(ctx, &services.ChallengeRequest{
		PublicKey: pubKey,
	})
	assertNoError(t, err)

	t.Run("WrongSignature", func(t *testing.T) {
		// Use wrong key to sign
		_, wrongKey := generateTestKeyPair(t)
		wrongSig := signChallengeBytes(t, wrongKey, challengeResp.Challenge)

		_, err := authClient.VerifyChallenge(ctx, &services.VerifyChallengeRequest{
			ChallengeId: challengeResp.ChallengeId,
			Signature:   wrongSig,
		})
		assertGRPCCode(t, err, codes.Unauthenticated)
	})

	t.Run("TamperedChallenge", func(t *testing.T) {
		// Get fresh challenge
		freshChallenge, _ := authClient.Challenge(ctx, &services.ChallengeRequest{
			PublicKey: pubKey,
		})

		// Sign a different message
		wrongData := append(freshChallenge.Challenge, byte(0xFF))
		sig := signChallengeBytes(t, privKey, wrongData)

		_, err := authClient.VerifyChallenge(ctx, &services.VerifyChallengeRequest{
			ChallengeId: freshChallenge.ChallengeId,
			Signature:   sig,
		})
		assertGRPCCode(t, err, codes.Unauthenticated)
	})

	t.Run("InvalidSessionToken", func(t *testing.T) {
		resp, err := authClient.ValidateSession(ctx, &services.ValidateSessionRequest{
			SessionToken: "invalid-token-xyz",
		})
		// May return error or Invalid=true response
		if err == nil && resp.Valid {
			t.Error("invalid token should not be valid")
		}
	})
}

// TestErrors_PermissionDenied tests authorization errors.
func TestErrors_PermissionDenied(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	topicClient := services.NewTopicServiceClient(conn)
	userClient := services.NewUserServiceClient(conn)
	adminClient := services.NewAdminServiceClient(conn)

	// Create admin FIRST (so they become the first user, which gets admin role)
	adminCtx, _, _ := ts.CreateAdminUser(ctx, "AdminUser")

	// Create regular user AFTER admin (so they get regular user role)
	userCtx, _, _ := ts.AuthenticateUser(ctx, "RegularUser")
	topicResp, _ := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{
		Name: "admin-topic",
	})
	topicID := topicResp.Topic.Id

	tests := []struct {
		name     string
		testFunc func() error
	}{
		{
			name: "Regular user cannot create topic",
			testFunc: func() error {
				_, err := topicClient.CreateTopic(userCtx, &services.CreateTopicRequest{
					Name: "user-created-topic",
				})
				return err
			},
		},
		{
			name: "Regular user cannot delete topic",
			testFunc: func() error {
				_, err := topicClient.DeleteTopic(userCtx, &services.DeleteTopicRequest{
					Id: topicID,
				})
				return err
			},
		},
		{
			name: "Regular user cannot access admin config",
			testFunc: func() error {
				_, err := adminClient.GetConfig(userCtx, &services.GetConfigRequest{})
				return err
			},
		},
		{
			name: "Regular user cannot access system status",
			testFunc: func() error {
				_, err := adminClient.GetSystemInfo(userCtx, &services.GetSystemInfoRequest{})
				return err
			},
		},
		{
			name: "Regular user cannot update other user",
			testFunc: func() error {
				// Get another user
				_, otherUser, _ := ts.AuthenticateUser(ctx, "OtherUser")
				newName := "Hacked Name"
				_, err := userClient.UpdateUser(userCtx, &services.UpdateUserRequest{
					UserId: string(otherUser.ID),
					Name:   &newName,
				})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.testFunc()
			assertGRPCCode(t, err, codes.PermissionDenied)
		})
	}
}

// TestErrors_AlreadyExists tests duplicate resource errors.
func TestErrors_AlreadyExists(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	topicClient := services.NewTopicServiceClient(conn)
	datasetClient := services.NewDatasetServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "DupAdmin")

	t.Run("DuplicateTopicName", func(t *testing.T) {
		// Create first topic
		_, err := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{
			Name: "unique-topic-name",
		})
		assertNoError(t, err)

		// Try to create with same name
		_, err = topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{
			Name: "unique-topic-name",
		})
		assertGRPCCode(t, err, codes.AlreadyExists)
	})

	t.Run("DuplicateDatasetName", func(t *testing.T) {
		// Create topic first
		topicResp, _ := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{
			Name: "dataset-dup-topic",
		})

		// Create first dataset
		_, err := datasetClient.CreateDataset(adminCtx, &services.CreateDatasetRequest{
			TopicId: topicResp.Topic.Id,
			Name:    "unique-dataset-name",
		})
		assertNoError(t, err)

		// Try to create with same name in same topic
		_, err = datasetClient.CreateDataset(adminCtx, &services.CreateDatasetRequest{
			TopicId: topicResp.Topic.Id,
			Name:    "unique-dataset-name",
		})
		assertGRPCCode(t, err, codes.AlreadyExists)
	})
}

// TestErrors_ContextCancellation tests behavior when context is cancelled.
func TestErrors_ContextCancellation(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	healthClient := services.NewHealthServiceClient(conn)

	t.Run("CancelledContext", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel() // Immediately cancel

		_, err := healthClient.Ping(cancelCtx, &services.PingRequest{})
		if err == nil {
			t.Error("expected error for cancelled context")
		}
	})

	t.Run("TimedOutContext", func(t *testing.T) {
		timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
		defer cancel()

		time.Sleep(1 * time.Millisecond) // Ensure timeout

		_, err := healthClient.Ping(timeoutCtx, &services.PingRequest{})
		if err == nil {
			t.Error("expected error for timed out context")
		}
	})
}

// TestErrors_InvalidMetadata tests behavior with invalid metadata.
func TestErrors_InvalidMetadata(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	userClient := services.NewUserServiceClient(conn)

	t.Run("InvalidSessionToken", func(t *testing.T) {
		// Add invalid session token to context
		invalidCtx := metadata.AppendToOutgoingContext(ctx, "x-session-token", "invalid-token")

		// GetUser requires valid session
		_, err := userClient.GetUser(invalidCtx, &services.GetUserRequest{
			UserId: "some-user-id",
		})
		// Should fail authentication
		if err == nil {
			t.Error("expected error for invalid session token")
		}
	})

	t.Run("MalformedAuthHeader", func(t *testing.T) {
		// Add malformed authorization header
		malformedCtx := metadata.AppendToOutgoingContext(ctx, "authorization", "NotBearer token")

		_, err := userClient.GetUser(malformedCtx, &services.GetUserRequest{
			UserId: "some-user-id",
		})
		if err == nil {
			t.Error("expected error for malformed auth header")
		}
	})
}

// =============================================================================
// Edge Cases
// =============================================================================

// TestEdgeCases_EmptyLists tests behavior with empty lists.
func TestEdgeCases_EmptyLists(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	topicClient := services.NewTopicServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "EmptyListAdmin")

	t.Run("ListWithNoResults", func(t *testing.T) {
		// Search for something that doesn't exist
		resp, err := topicClient.SearchTopics(adminCtx, &services.SearchTopicsRequest{
			Query: "nonexistentxyzabc123",
		})
		assertNoError(t, err)

		if len(resp.Topics) != 0 {
			t.Errorf("expected empty list, got %d items", len(resp.Topics))
		}
		if resp.PageInfo != nil && resp.PageInfo.TotalCount != 0 {
			t.Errorf("expected total count 0, got %d", resp.PageInfo.TotalCount)
		}
	})

	t.Run("ListWithLargeOffset", func(t *testing.T) {
		resp, err := topicClient.ListTopics(adminCtx, &services.ListTopicsRequest{
			Page: &bibv1.PageRequest{
				Limit:  10,
				Offset: 999999,
			},
		})
		assertNoError(t, err)

		if len(resp.Topics) != 0 {
			t.Errorf("expected empty list for large offset, got %d items", len(resp.Topics))
		}
	})
}

// TestEdgeCases_SpecialCharacters tests handling of special characters.
func TestEdgeCases_SpecialCharacters(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	topicClient := services.NewTopicServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "SpecialCharAdmin")

	specialNames := []struct {
		name  string
		input string
	}{
		{"unicode", "日本語トピック"},
		{"emoji", "Topic 🚀 with emoji"},
		{"spaces", "topic with spaces"},
		{"dashes", "topic-with-dashes"},
		{"underscores", "topic_with_underscores"},
		{"numbers", "topic123"},
		{"mixed", "Topic-123_test"},
	}

	for _, tc := range specialNames {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{
				Name:        tc.input,
				Description: "Test topic with " + tc.name,
			})
			if err != nil {
				t.Logf("Failed to create topic with %s: %v", tc.name, err)
				return
			}

			// Verify we can retrieve it
			getResp, err := topicClient.GetTopic(adminCtx, &services.GetTopicRequest{
				Id: resp.Topic.Id,
			})
			assertNoError(t, err)

			if getResp.Topic.Name != tc.input {
				t.Errorf("name mismatch: expected %q, got %q", tc.input, getResp.Topic.Name)
			}
		})
	}
}

// TestEdgeCases_LargePayloads tests handling of large payloads.
func TestEdgeCases_LargePayloads(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	topicClient := services.NewTopicServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "LargePayloadAdmin")

	t.Run("LargeDescription", func(t *testing.T) {
		largeDesc := strings.Repeat("This is a test. ", 1000) // ~16KB

		resp, err := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{
			Name:        "large-desc-topic",
			Description: largeDesc,
		})
		assertNoError(t, err)

		if resp.Topic.Description != largeDesc {
			t.Error("description truncated or modified")
		}
	})

	t.Run("ManyTags", func(t *testing.T) {
		tags := make([]string, 100)
		for i := range tags {
			tags[i] = "tag-" + strings.Repeat("a", 10)
		}

		resp, err := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{
			Name: "many-tags-topic",
			Tags: tags,
		})
		if err != nil {
			t.Logf("Many tags may be rejected: %v", err)
			return
		}

		t.Logf("Created topic with %d tags", len(resp.Topic.Tags))
	})

	t.Run("LargeMetadata", func(t *testing.T) {
		metadata := make(map[string]string)
		for i := 0; i < 50; i++ {
			metadata["key-"+string(rune('A'+i))] = strings.Repeat("value", 100)
		}

		resp, err := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{
			Name:     "large-metadata-topic",
			Metadata: metadata,
		})
		if err != nil {
			t.Logf("Large metadata may be rejected: %v", err)
			return
		}

		t.Logf("Created topic with %d metadata entries", len(resp.Topic.Metadata))
	})
}

// TestEdgeCases_Pagination tests pagination edge cases.
func TestEdgeCases_Pagination(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	topicClient := services.NewTopicServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "PaginationAdmin")

	// Create some topics
	for i := 0; i < 15; i++ {
		topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{
			Name: "pagination-topic-" + string(rune('A'+i)),
		})
	}

	t.Run("ZeroLimit", func(t *testing.T) {
		resp, err := topicClient.ListTopics(adminCtx, &services.ListTopicsRequest{
			Page: &bibv1.PageRequest{Limit: 0},
		})
		assertNoError(t, err)

		// Should use default limit
		t.Logf("Got %d results with limit=0", len(resp.Topics))
	})

	t.Run("NegativeOffset", func(t *testing.T) {
		// Negative offset might be treated as 0 or rejected
		resp, err := topicClient.ListTopics(adminCtx, &services.ListTopicsRequest{
			Page: &bibv1.PageRequest{Offset: -1},
		})
		if err != nil {
			t.Logf("Negative offset rejected: %v", err)
			return
		}
		t.Logf("Got %d results with offset=-1", len(resp.Topics))
	})

	t.Run("ExcessiveLimit", func(t *testing.T) {
		resp, err := topicClient.ListTopics(adminCtx, &services.ListTopicsRequest{
			Page: &bibv1.PageRequest{Limit: 100000},
		})
		assertNoError(t, err)

		// Should be capped at max limit
		t.Logf("Got %d results with limit=100000 (should be capped)", len(resp.Topics))
	})

	t.Run("ExactPageBoundary", func(t *testing.T) {
		// Get exact count
		resp1, _ := topicClient.ListTopics(adminCtx, &services.ListTopicsRequest{})
		total := resp1.PageInfo.TotalCount

		// Request exactly at boundary
		resp2, err := topicClient.ListTopics(adminCtx, &services.ListTopicsRequest{
			Page: &bibv1.PageRequest{
				Limit:  10,
				Offset: int32(total),
			},
		})
		assertNoError(t, err)

		if len(resp2.Topics) != 0 {
			t.Errorf("expected 0 topics at exact boundary, got %d", len(resp2.Topics))
		}
	})
}
