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
// DatasetService Integration Tests
// =============================================================================

// TestDatasetService_CreateDataset tests dataset creation.
func TestDatasetService_CreateDataset(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	topicClient := services.NewTopicServiceClient(conn)
	datasetClient := services.NewDatasetServiceClient(conn)

	// Create admin and topic first
	adminCtx, _, _ := ts.CreateAdminUser(ctx, "DatasetAdmin")

	topicResp, err := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{
		Name: "dataset-topic",
	})
	assertNoError(t, err)
	topicID := topicResp.Topic.Id

	t.Run("CreateValidDataset", func(t *testing.T) {
		resp, err := datasetClient.CreateDataset(adminCtx, &services.CreateDatasetRequest{
			TopicId:     topicID,
			Name:        "test-dataset",
			Description: "A test dataset",
			ContentType: "application/json",
			Tags:        []string{"test", "integration"},
		})
		assertNoError(t, err)

		if resp.Dataset == nil {
			t.Fatal("expected dataset")
		}
		if resp.Dataset.Id == "" {
			t.Error("expected dataset ID")
		}
		if resp.Dataset.Name != "test-dataset" {
			t.Errorf("expected name 'test-dataset', got '%s'", resp.Dataset.Name)
		}
		if resp.Dataset.TopicId != topicID {
			t.Errorf("expected topic ID %s, got %s", topicID, resp.Dataset.TopicId)
		}
	})

	t.Run("CreateWithoutTopic", func(t *testing.T) {
		_, err := datasetClient.CreateDataset(adminCtx, &services.CreateDatasetRequest{
			TopicId: "",
			Name:    "orphan-dataset",
		})
		assertGRPCCode(t, err, codes.InvalidArgument)
	})

	t.Run("CreateWithInvalidTopic", func(t *testing.T) {
		_, err := datasetClient.CreateDataset(adminCtx, &services.CreateDatasetRequest{
			TopicId: "non-existent-topic",
			Name:    "invalid-topic-dataset",
		})
		assertGRPCCode(t, err, codes.NotFound)
	})

	t.Run("CreateWithMetadata", func(t *testing.T) {
		resp, err := datasetClient.CreateDataset(adminCtx, &services.CreateDatasetRequest{
			TopicId: topicID,
			Name:    "metadata-dataset",
			Metadata: map[string]string{
				"source":  "integration-test",
				"version": "1.0",
			},
		})
		assertNoError(t, err)

		if resp.Dataset.Metadata["source"] != "integration-test" {
			t.Error("metadata not set correctly")
		}
	})
}

// TestDatasetService_GetDataset tests dataset retrieval.
func TestDatasetService_GetDataset(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	topicClient := services.NewTopicServiceClient(conn)
	datasetClient := services.NewDatasetServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "GetDatasetAdmin")

	// Create topic and dataset
	topicResp, _ := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{Name: "get-dataset-topic"})
	datasetResp, err := datasetClient.CreateDataset(adminCtx, &services.CreateDatasetRequest{
		TopicId:     topicResp.Topic.Id,
		Name:        "get-dataset-test",
		Description: "For get tests",
	})
	assertNoError(t, err)
	datasetID := datasetResp.Dataset.Id

	userCtx, _, _ := ts.AuthenticateUser(ctx, "GetDatasetUser")

	t.Run("GetByID", func(t *testing.T) {
		resp, err := datasetClient.GetDataset(userCtx, &services.GetDatasetRequest{
			Id: datasetID,
		})
		assertNoError(t, err)

		if resp.Dataset.Id != datasetID {
			t.Errorf("expected ID %s, got %s", datasetID, resp.Dataset.Id)
		}
		if resp.Dataset.Name != "get-dataset-test" {
			t.Errorf("expected name 'get-dataset-test', got '%s'", resp.Dataset.Name)
		}
	})

	t.Run("GetNonExistent", func(t *testing.T) {
		_, err := datasetClient.GetDataset(userCtx, &services.GetDatasetRequest{
			Id: "non-existent-dataset",
		})
		assertGRPCCode(t, err, codes.NotFound)
	})

	t.Run("GetMissingID", func(t *testing.T) {
		_, err := datasetClient.GetDataset(userCtx, &services.GetDatasetRequest{})
		assertGRPCCode(t, err, codes.InvalidArgument)
	})
}

// TestDatasetService_ListDatasets tests dataset listing.
func TestDatasetService_ListDatasets(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	topicClient := services.NewTopicServiceClient(conn)
	datasetClient := services.NewDatasetServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "ListDatasetAdmin")

	// Create topics
	topic1Resp, _ := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{Name: "list-dataset-topic-1"})
	topic2Resp, _ := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{Name: "list-dataset-topic-2"})

	// Create datasets in both topics
	for i := 0; i < 5; i++ {
		tags := []string{"batch"}
		if i%2 == 0 {
			tags = append(tags, "even")
		}
		_, err := datasetClient.CreateDataset(adminCtx, &services.CreateDatasetRequest{
			TopicId: topic1Resp.Topic.Id,
			Name:    "topic1-dataset-" + string(rune('A'+i)),
			Tags:    tags,
		})
		assertNoError(t, err)
	}
	for i := 0; i < 3; i++ {
		_, err := datasetClient.CreateDataset(adminCtx, &services.CreateDatasetRequest{
			TopicId: topic2Resp.Topic.Id,
			Name:    "topic2-dataset-" + string(rune('A'+i)),
		})
		assertNoError(t, err)
	}

	userCtx, _, _ := ts.AuthenticateUser(ctx, "ListDatasetUser")

	t.Run("ListAll", func(t *testing.T) {
		resp, err := datasetClient.ListDatasets(userCtx, &services.ListDatasetsRequest{})
		assertNoError(t, err)

		if len(resp.Datasets) < 8 {
			t.Errorf("expected at least 8 datasets, got %d", len(resp.Datasets))
		}
	})

	t.Run("ListByTopic", func(t *testing.T) {
		resp, err := datasetClient.ListDatasets(userCtx, &services.ListDatasetsRequest{
			TopicId: topic1Resp.Topic.Id,
		})
		assertNoError(t, err)

		if len(resp.Datasets) != 5 {
			t.Errorf("expected 5 datasets for topic1, got %d", len(resp.Datasets))
		}
		for _, ds := range resp.Datasets {
			if ds.TopicId != topic1Resp.Topic.Id {
				t.Errorf("expected topic %s, got %s", topic1Resp.Topic.Id, ds.TopicId)
			}
		}
	})

	t.Run("ListByTag", func(t *testing.T) {
		resp, err := datasetClient.ListDatasets(userCtx, &services.ListDatasetsRequest{
			Tags: []string{"even"},
		})
		assertNoError(t, err)

		for _, ds := range resp.Datasets {
			hasEven := false
			for _, tag := range ds.Tags {
				if tag == "even" {
					hasEven = true
					break
				}
			}
			if !hasEven {
				t.Errorf("dataset %s missing 'even' tag", ds.Name)
			}
		}
	})

	t.Run("ListWithPagination", func(t *testing.T) {
		resp1, err := datasetClient.ListDatasets(userCtx, &services.ListDatasetsRequest{
			Page: &bibv1.PageRequest{Limit: 3, Offset: 0},
		})
		assertNoError(t, err)

		if len(resp1.Datasets) > 3 {
			t.Errorf("expected at most 3 datasets, got %d", len(resp1.Datasets))
		}

		resp2, err := datasetClient.ListDatasets(userCtx, &services.ListDatasetsRequest{
			Page: &bibv1.PageRequest{Limit: 3, Offset: 3},
		})
		assertNoError(t, err)

		// Check no overlap
		for _, d1 := range resp1.Datasets {
			for _, d2 := range resp2.Datasets {
				if d1.Id == d2.Id {
					t.Errorf("duplicate dataset %s in paginated results", d1.Id)
				}
			}
		}
	})
}

// TestDatasetService_UpdateDataset tests dataset updates.
func TestDatasetService_UpdateDataset(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	topicClient := services.NewTopicServiceClient(conn)
	datasetClient := services.NewDatasetServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "UpdateDatasetAdmin")

	topicResp, _ := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{Name: "update-dataset-topic"})
	datasetResp, err := datasetClient.CreateDataset(adminCtx, &services.CreateDatasetRequest{
		TopicId:     topicResp.Topic.Id,
		Name:        "update-me-dataset",
		Description: "Original",
	})
	assertNoError(t, err)
	datasetID := datasetResp.Dataset.Id

	t.Run("UpdateDescription", func(t *testing.T) {
		newDesc := "Updated description"
		resp, err := datasetClient.UpdateDataset(adminCtx, &services.UpdateDatasetRequest{
			Id:          datasetID,
			Description: &newDesc,
		})
		assertNoError(t, err)

		if resp.Dataset.Description != newDesc {
			t.Errorf("expected '%s', got '%s'", newDesc, resp.Dataset.Description)
		}
	})

	t.Run("UpdateTags", func(t *testing.T) {
		resp, err := datasetClient.UpdateDataset(adminCtx, &services.UpdateDatasetRequest{
			Id:   datasetID,
			Tags: []string{"new-tag-1", "new-tag-2"},
		})
		assertNoError(t, err)

		if len(resp.Dataset.Tags) != 2 {
			t.Errorf("expected 2 tags, got %d", len(resp.Dataset.Tags))
		}
	})

	t.Run("UpdateContentType", func(t *testing.T) {
		newType := "text/plain"
		resp, err := datasetClient.UpdateDataset(adminCtx, &services.UpdateDatasetRequest{
			Id:          datasetID,
			ContentType: &newType,
		})
		assertNoError(t, err)

		if resp.Dataset.ContentType != newType {
			t.Errorf("content type not updated: expected %s, got %s", newType, resp.Dataset.ContentType)
		}
	})

	t.Run("UpdateNonExistent", func(t *testing.T) {
		desc := "Test"
		_, err := datasetClient.UpdateDataset(adminCtx, &services.UpdateDatasetRequest{
			Id:          "non-existent-id",
			Description: &desc,
		})
		assertGRPCCode(t, err, codes.NotFound)
	})
}

// TestDatasetService_DeleteDataset tests dataset deletion.
func TestDatasetService_DeleteDataset(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	topicClient := services.NewTopicServiceClient(conn)
	datasetClient := services.NewDatasetServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "DeleteDatasetAdmin")

	topicResp, _ := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{Name: "delete-dataset-topic"})

	t.Run("DeleteExisting", func(t *testing.T) {
		datasetResp, err := datasetClient.CreateDataset(adminCtx, &services.CreateDatasetRequest{
			TopicId: topicResp.Topic.Id,
			Name:    "delete-me-dataset",
		})
		assertNoError(t, err)
		datasetID := datasetResp.Dataset.Id

		resp, err := datasetClient.DeleteDataset(adminCtx, &services.DeleteDatasetRequest{
			Id: datasetID,
		})
		assertNoError(t, err)
		if !resp.Success {
			t.Error("expected success")
		}

		// Verify deleted
		_, err = datasetClient.GetDataset(adminCtx, &services.GetDatasetRequest{Id: datasetID})
		assertGRPCCode(t, err, codes.NotFound)
	})

	t.Run("DeleteNonExistent", func(t *testing.T) {
		_, err := datasetClient.DeleteDataset(adminCtx, &services.DeleteDatasetRequest{
			Id: "non-existent-id",
		})
		assertGRPCCode(t, err, codes.NotFound)
	})
}

// TestDatasetService_Versions tests dataset versioning.
func TestDatasetService_Versions(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	topicClient := services.NewTopicServiceClient(conn)
	datasetClient := services.NewDatasetServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "VersionAdmin")

	topicResp, _ := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{Name: "version-topic"})
	datasetResp, err := datasetClient.CreateDataset(adminCtx, &services.CreateDatasetRequest{
		TopicId: topicResp.Topic.Id,
		Name:    "versioned-dataset",
	})
	assertNoError(t, err)
	datasetID := datasetResp.Dataset.Id

	t.Run("GetDatasetVersions", func(t *testing.T) {
		resp, err := datasetClient.GetDatasetVersions(adminCtx, &services.GetDatasetVersionsRequest{
			DatasetId: datasetID,
		})
		assertNoError(t, err)

		// New datasets may have zero or one version
		t.Logf("Found %d versions for dataset", len(resp.Versions))
	})

	t.Run("GetVersionNotFound", func(t *testing.T) {
		_, err := datasetClient.GetVersion(adminCtx, &services.GetVersionRequest{
			DatasetId: datasetID,
			Version:   99999, // Non-existent version number
		})
		assertGRPCCode(t, err, codes.NotFound)
	})
}

// TestDatasetService_Search tests dataset search.
func TestDatasetService_Search(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	topicClient := services.NewTopicServiceClient(conn)
	datasetClient := services.NewDatasetServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "SearchDatasetAdmin")

	topicResp, _ := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{Name: "search-dataset-topic"})

	// Create datasets with distinct names
	_, _ = datasetClient.CreateDataset(adminCtx, &services.CreateDatasetRequest{
		TopicId:     topicResp.Topic.Id,
		Name:        "weather-observations",
		Description: "Daily weather data from sensors",
	})
	_, _ = datasetClient.CreateDataset(adminCtx, &services.CreateDatasetRequest{
		TopicId:     topicResp.Topic.Id,
		Name:        "traffic-counts",
		Description: "Vehicle counts at intersections",
	})
	_, _ = datasetClient.CreateDataset(adminCtx, &services.CreateDatasetRequest{
		TopicId:     topicResp.Topic.Id,
		Name:        "air-quality",
		Description: "Air quality measurements from weather stations",
	})

	userCtx, _, _ := ts.AuthenticateUser(ctx, "SearchDatasetUser")

	t.Run("SearchByName", func(t *testing.T) {
		resp, err := datasetClient.SearchDatasets(userCtx, &services.SearchDatasetsRequest{
			Query: "weather",
		})
		assertNoError(t, err)

		found := false
		for _, ds := range resp.Datasets {
			if ds.Name == "weather-observations" {
				found = true
			}
		}
		if !found {
			t.Error("expected to find weather-observations")
		}
	})

	t.Run("SearchByDescription", func(t *testing.T) {
		resp, err := datasetClient.SearchDatasets(userCtx, &services.SearchDatasetsRequest{
			Query: "vehicle",
		})
		assertNoError(t, err)

		found := false
		for _, ds := range resp.Datasets {
			if ds.Name == "traffic-counts" {
				found = true
			}
		}
		if !found {
			t.Error("expected to find traffic-counts")
		}
	})

	t.Run("SearchWithTopicFilter", func(t *testing.T) {
		resp, err := datasetClient.SearchDatasets(userCtx, &services.SearchDatasetsRequest{
			Query:   "quality",
			TopicId: topicResp.Topic.Id,
		})
		assertNoError(t, err)

		for _, ds := range resp.Datasets {
			if ds.TopicId != topicResp.Topic.Id {
				t.Errorf("expected topic %s, got %s", topicResp.Topic.Id, ds.TopicId)
			}
		}
	})
}

// TestDatasetService_Stats tests dataset statistics.
func TestDatasetService_Stats(t *testing.T) {
	t.Parallel()
	testutil.SkipIfShort(t)
	ctx := testutil.TestContext(t)

	ts := NewTestServer(t, ctx)
	conn := ts.Dial()
	topicClient := services.NewTopicServiceClient(conn)
	datasetClient := services.NewDatasetServiceClient(conn)

	adminCtx, _, _ := ts.CreateAdminUser(ctx, "StatsAdmin")

	topicResp, _ := topicClient.CreateTopic(adminCtx, &services.CreateTopicRequest{Name: "stats-topic"})
	datasetResp, err := datasetClient.CreateDataset(adminCtx, &services.CreateDatasetRequest{
		TopicId: topicResp.Topic.Id,
		Name:    "stats-dataset",
	})
	assertNoError(t, err)
	datasetID := datasetResp.Dataset.Id

	t.Run("GetStats", func(t *testing.T) {
		resp, err := datasetClient.GetDatasetStats(adminCtx, &services.GetDatasetStatsRequest{
			DatasetId: datasetID,
		})
		assertNoError(t, err)

		if resp.DatasetId != datasetID {
			t.Errorf("expected dataset ID %s, got %s", datasetID, resp.DatasetId)
		}
		t.Logf("Stats: versions=%d, totalSize=%d, downloads=%d",
			resp.VersionCount, resp.TotalSizeAllVersions, resp.DownloadCount)
	})
}
