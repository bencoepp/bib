//go:build integration

package grpc_test

import (
	"testing"

	bibv1 "bib/api/gen/go/bib/v1"
	services "bib/api/gen/go/bib/v1/services"
	"bib/test/testutil"

	"google.golang.org/grpc/codes"
)

// =============================================================================
// TopicService Integration Tests
// =============================================================================

// TestTopicService_CreateTopic tests topic creation.
func TestTopicService_CreateTopic(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	topicClient := services.NewTopicServiceClient(conn)

	// Only admins can create topics
	adminCtx, _, _ := ts.CreateAdminUser(ctx, "TopicAdmin")

	t.Run("CreateValidTopic", func(t *testing.T) {
		resp, err := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{
			Name:        "test-topic",
			Description: "A test topic for integration tests",
			Tags:        []string{"test", "integration"},
			Metadata: map[string]string{
				"owner":   "test-suite",
				"purpose": "testing",
			},
		})
		assertNoError(t, err)

		if resp.Topic == nil {
			t.Fatal("expected topic in response")
		}
		if resp.Topic.Id == "" {
			t.Error("expected topic ID")
		}
		if resp.Topic.Name != "test-topic" {
			t.Errorf("expected name 'test-topic', got '%s'", resp.Topic.Name)
		}
		if resp.Topic.Description != "A test topic for integration tests" {
			t.Errorf("unexpected description: %s", resp.Topic.Description)
		}
		if len(resp.Topic.Tags) != 2 {
			t.Errorf("expected 2 tags, got %d", len(resp.Topic.Tags))
		}
	})

	t.Run("CreateDuplicateName", func(t *testing.T) {
		// First create succeeds
		_, err := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{
			Name: "duplicate-topic",
		})
		assertNoError(t, err)

		// Second create should fail with some error (implementation may vary)
		_, err = topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{
			Name: "duplicate-topic",
		})
		if err == nil {
			t.Error("expected error for duplicate topic name")
		}
	})

	t.Run("CreateEmptyName", func(t *testing.T) {
		_, err := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{
			Name: "",
		})
		assertGRPCCode(t, err, codes.InvalidArgument)
	})

	t.Run("NonAdminCannotCreate", func(t *testing.T) {
		userCtx, _, _ := ts.AuthenticateUser(ctx, "RegularUser")
		_, err := topicClient.CreateTopic(userCtx, &services.CreateTopicRequest{
			Name: "user-topic",
		})
		assertGRPCCode(t, err, codes.PermissionDenied)
	})
}

// TestTopicService_GetTopic tests topic retrieval.
func TestTopicService_GetTopic(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	topicClient := services.NewTopicServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "GetTopicAdmin")

	// Create a topic
	createResp, err := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{
		Name:        "get-topic-test",
		Description: "Topic for get tests",
	})
	assertNoError(t, err)
	topicID := createResp.Topic.Id

	userCtx, _, _ := ts.AuthenticateUser(ctx, "GetTopicUser")

	t.Run("GetByID", func(t *testing.T) {
		resp, err := topicClient.GetTopic(userCtx, &services.GetTopicRequest{
			Id: topicID,
		})
		assertNoError(t, err)

		if resp.Topic.Id != topicID {
			t.Errorf("expected ID %s, got %s", topicID, resp.Topic.Id)
		}
	})

	t.Run("GetByName", func(t *testing.T) {
		resp, err := topicClient.GetTopic(userCtx, &services.GetTopicRequest{
			Name: "get-topic-test",
		})
		assertNoError(t, err)

		if resp.Topic.Id != topicID {
			t.Errorf("expected ID %s, got %s", topicID, resp.Topic.Id)
		}
	})

	t.Run("GetNonExistent", func(t *testing.T) {
		_, err := topicClient.GetTopic(userCtx, &services.GetTopicRequest{
			Id: "non-existent-topic-id",
		})
		assertGRPCCode(t, err, codes.NotFound)
	})

	t.Run("GetMissingIDAndName", func(t *testing.T) {
		_, err := topicClient.GetTopic(userCtx, &services.GetTopicRequest{})
		assertGRPCCode(t, err, codes.InvalidArgument)
	})
}

// TestTopicService_ListTopics tests topic listing with filters.
func TestTopicService_ListTopics(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	topicClient := services.NewTopicServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "ListTopicAdmin")

	// Create several topics with different tags
	for i := 0; i < 10; i++ {
		tags := []string{"batch"}
		if i%2 == 0 {
			tags = append(tags, "even")
		} else {
			tags = append(tags, "odd")
		}
		_, err := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{
			Name: "list-topic-" + string(rune('A'+i)),
			Tags: tags,
		})
		assertNoError(t, err)
	}

	userCtx, _, _ := ts.AuthenticateUser(ctx, "ListTopicUser")

	t.Run("ListAll", func(t *testing.T) {
		resp, err := topicClient.ListTopics(userCtx, &services.ListTopicsRequest{})
		assertNoError(t, err)

		if len(resp.Topics) == 0 {
			t.Error("expected topics")
		}
		if resp.PageInfo == nil {
			t.Error("expected page info")
		}
	})

	t.Run("ListWithPagination", func(t *testing.T) {
		resp1, err := topicClient.ListTopics(userCtx, &services.ListTopicsRequest{
			Page: &bibv1.PageRequest{Limit: 5, Offset: 0},
		})
		assertNoError(t, err)

		if len(resp1.Topics) > 5 {
			t.Errorf("expected at most 5 topics, got %d", len(resp1.Topics))
		}

		resp2, err := topicClient.ListTopics(userCtx, &services.ListTopicsRequest{
			Page: &bibv1.PageRequest{Limit: 5, Offset: 5},
		})
		assertNoError(t, err)

		// Check no overlap
		for _, t1 := range resp1.Topics {
			for _, t2 := range resp2.Topics {
				if t1.Id == t2.Id {
					t.Errorf("duplicate topic %s in paginated results", t1.Id)
				}
			}
		}
	})

	t.Run("ListByTag", func(t *testing.T) {
		resp, err := topicClient.ListTopics(userCtx, &services.ListTopicsRequest{
			Tags: []string{"even"},
		})
		assertNoError(t, err)

		for _, topic := range resp.Topics {
			hasEven := false
			for _, tag := range topic.Tags {
				if tag == "even" {
					hasEven = true
					break
				}
			}
			if !hasEven {
				t.Errorf("topic %s missing 'even' tag", topic.Name)
			}
		}
	})

	t.Run("ListSorted", func(t *testing.T) {
		resp, err := topicClient.ListTopics(userCtx, &services.ListTopicsRequest{
			Sort: &bibv1.SortOrder{
				Field:      "name",
				Descending: true,
			},
		})
		assertNoError(t, err)

		// Verify descending order
		for i := 1; i < len(resp.Topics); i++ {
			if resp.Topics[i-1].Name < resp.Topics[i].Name {
				t.Error("topics not sorted in descending order")
			}
		}
	})
}

// TestTopicService_UpdateTopic tests topic updates.
func TestTopicService_UpdateTopic(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	topicClient := services.NewTopicServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "UpdateTopicAdmin")

	// Create a topic
	createResp, err := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{
		Name:        "update-topic-test",
		Description: "Original description",
	})
	assertNoError(t, err)
	topicID := createResp.Topic.Id

	t.Run("UpdateDescription", func(t *testing.T) {
		newDesc := "Updated description"
		resp, err := topicClient.UpdateTopic(adminCtx, &services.UpdateTopicRequest{
			Id:          topicID,
			Description: &newDesc,
		})
		assertNoError(t, err)

		if resp.Topic.Description != newDesc {
			t.Errorf("expected description '%s', got '%s'", newDesc, resp.Topic.Description)
		}
	})

	t.Run("UpdateTags", func(t *testing.T) {
		resp, err := topicClient.UpdateTopic(adminCtx, &services.UpdateTopicRequest{
			Id:   topicID,
			Tags: []string{"new-tag-1", "new-tag-2"},
		})
		assertNoError(t, err)

		if len(resp.Topic.Tags) != 2 {
			t.Errorf("expected 2 tags, got %d", len(resp.Topic.Tags))
		}
	})

	t.Run("UpdateMetadata", func(t *testing.T) {
		resp, err := topicClient.UpdateTopic(adminCtx, &services.UpdateTopicRequest{
			Id: topicID,
			Metadata: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		})
		assertNoError(t, err)

		if resp.Topic.Metadata["key1"] != "value1" {
			t.Error("metadata not updated")
		}
	})

	t.Run("UpdateNonExistent", func(t *testing.T) {
		desc := "Test"
		_, err := topicClient.UpdateTopic(adminCtx, &services.UpdateTopicRequest{
			Id:          "non-existent-id",
			Description: &desc,
		})
		assertGRPCCode(t, err, codes.NotFound)
	})

	t.Run("UpdateMissingID", func(t *testing.T) {
		desc := "Test"
		_, err := topicClient.UpdateTopic(adminCtx, &services.UpdateTopicRequest{
			Description: &desc,
		})
		assertGRPCCode(t, err, codes.InvalidArgument)
	})
}

// TestTopicService_DeleteTopic tests topic deletion.
func TestTopicService_DeleteTopic(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	topicClient := services.NewTopicServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "DeleteTopicAdmin")

	t.Run("DeleteExisting", func(t *testing.T) {
		// Create a topic
		createResp, err := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{
			Name: "delete-me-topic",
		})
		assertNoError(t, err)
		topicID := createResp.Topic.Id

		// Delete it
		resp, err := topicClient.DeleteTopic(adminCtx, &services.DeleteTopicRequest{
			Id: topicID,
		})
		assertNoError(t, err)
		if !resp.Success {
			t.Error("expected success")
		}

		// Verify gone
		_, err = topicClient.GetTopic(adminCtx, &services.GetTopicRequest{Id: topicID})
		assertGRPCCode(t, err, codes.NotFound)
	})

	t.Run("DeleteNonExistent", func(t *testing.T) {
		_, err := topicClient.DeleteTopic(adminCtx, &services.DeleteTopicRequest{
			Id: "non-existent-id",
		})
		assertGRPCCode(t, err, codes.NotFound)
	})

	t.Run("DeleteMissingID", func(t *testing.T) {
		_, err := topicClient.DeleteTopic(adminCtx, &services.DeleteTopicRequest{})
		assertGRPCCode(t, err, codes.InvalidArgument)
	})
}

// TestTopicService_Subscription tests topic subscription.
func TestTopicService_Subscription(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	topicClient := services.NewTopicServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "SubAdmin")

	// Create a topic
	createResp, err := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{
		Name: "subscription-topic",
	})
	assertNoError(t, err)
	topicID := createResp.Topic.Id

	userCtx, _, _ := ts.AuthenticateUser(ctx, "Subscriber")

	t.Run("Subscribe", func(t *testing.T) {
		resp, err := topicClient.Subscribe(userCtx, &services.SubscribeRequest{
			TopicId: topicID,
		})
		assertNoError(t, err)

		if resp.Subscription == nil {
			t.Error("expected subscription")
		}
	})

	t.Run("ListSubscriptions", func(t *testing.T) {
		resp, err := topicClient.ListSubscriptions(userCtx, &services.ListSubscriptionsRequest{})
		assertNoError(t, err)

		found := false
		for _, sub := range resp.Subscriptions {
			if sub.TopicId == topicID {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find subscription")
		}
	})

	t.Run("Unsubscribe", func(t *testing.T) {
		resp, err := topicClient.Unsubscribe(userCtx, &services.UnsubscribeRequest{
			TopicId: topicID,
		})
		assertNoError(t, err)
		if !resp.Success {
			t.Error("expected success")
		}

		// Verify unsubscribed
		listResp, _ := topicClient.ListSubscriptions(userCtx, &services.ListSubscriptionsRequest{})
		for _, sub := range listResp.Subscriptions {
			if sub.TopicId == topicID {
				t.Error("should be unsubscribed")
			}
		}
	})

	t.Run("SubscribeNonExistent", func(t *testing.T) {
		_, err := topicClient.Subscribe(userCtx, &services.SubscribeRequest{
			TopicId: "non-existent-topic",
		})
		assertGRPCCode(t, err, codes.NotFound)
	})
}

// TestTopicService_Search tests topic search.
func TestTopicService_Search(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	topicClient := services.NewTopicServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "SearchAdmin")

	// Create topics with distinctive names
	_, _ = topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{
		Name:        "genomics-research",
		Description: "Human genomics research data",
	})
	_, _ = topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{
		Name:        "proteomics-analysis",
		Description: "Protein structure analysis",
	})
	_, _ = topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{
		Name:        "climate-data",
		Description: "Climate monitoring sensors",
	})

	userCtx, _, _ := ts.AuthenticateUser(ctx, "SearchUser")

	t.Run("SearchByName", func(t *testing.T) {
		resp, err := topicClient.SearchTopics(userCtx, &services.SearchTopicsRequest{
			Query: "genomics",
		})
		assertNoError(t, err)

		found := false
		for _, topic := range resp.Topics {
			if topic.Name == "genomics-research" {
				found = true
			}
		}
		if !found {
			t.Error("expected to find genomics-research topic")
		}
	})

	t.Run("SearchByDescription", func(t *testing.T) {
		resp, err := topicClient.SearchTopics(userCtx, &services.SearchTopicsRequest{
			Query: "protein",
		})
		assertNoError(t, err)

		found := false
		for _, topic := range resp.Topics {
			if topic.Name == "proteomics-analysis" {
				found = true
			}
		}
		if !found {
			t.Error("expected to find proteomics-analysis topic")
		}
	})

	t.Run("SearchNoResults", func(t *testing.T) {
		resp, err := topicClient.SearchTopics(userCtx, &services.SearchTopicsRequest{
			Query: "nonexistentxyz123",
		})
		assertNoError(t, err)

		if len(resp.Topics) != 0 {
			t.Error("expected no results")
		}
	})
}
