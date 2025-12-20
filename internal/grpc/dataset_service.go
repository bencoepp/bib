// Package grpc provides gRPC service implementations for the bib daemon.
package grpc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	services "bib/api/gen/go/bib/v1/services"
	"bib/internal/domain"
	"bib/internal/storage"
	"bib/internal/storage/blob"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// defaultChunkSize is the default chunk size for streaming uploads (256KB)
	defaultChunkSize = 256 * 1024
)

// DatasetServiceServer implements the DatasetService gRPC service.
type DatasetServiceServer struct {
	services.UnimplementedDatasetServiceServer
	store       storage.Store
	blobStore   blob.Store
	auditLogger *AuditMiddleware
	nodeMode    string
}

// DatasetServiceConfig holds configuration for the DatasetServiceServer.
type DatasetServiceConfig struct {
	Store       storage.Store
	BlobStore   blob.Store
	AuditLogger *AuditMiddleware
	NodeMode    string
}

// NewDatasetServiceServer creates a new DatasetServiceServer.
func NewDatasetServiceServer() *DatasetServiceServer {
	return &DatasetServiceServer{}
}

// NewDatasetServiceServerWithConfig creates a new DatasetServiceServer with dependencies.
func NewDatasetServiceServerWithConfig(cfg DatasetServiceConfig) *DatasetServiceServer {
	return &DatasetServiceServer{
		store:       cfg.Store,
		blobStore:   cfg.BlobStore,
		auditLogger: cfg.AuditLogger,
		nodeMode:    cfg.NodeMode,
	}
}

// SetStore sets the storage dependency.
func (s *DatasetServiceServer) SetStore(store storage.Store) {
	s.store = store
}

// SetBlobStore sets the blob storage dependency.
func (s *DatasetServiceServer) SetBlobStore(blobStore blob.Store) {
	s.blobStore = blobStore
}

// CreateDataset creates a new dataset.
func (s *DatasetServiceServer) CreateDataset(ctx context.Context, req *services.CreateDatasetRequest) (*services.CreateDatasetResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	// Validate required fields
	violations := make(map[string]string)
	if req.GetTopicId() == "" {
		violations["topic_id"] = "must not be empty"
	}
	if req.GetName() == "" {
		violations["name"] = "must not be empty"
	}
	if len(violations) > 0 {
		return nil, NewValidationError("invalid create dataset request", violations)
	}

	// Get current user
	user, ok := UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	// Check topic exists and user has access
	topic, err := s.store.Topics().Get(ctx, domain.TopicID(req.GetTopicId()))
	if err != nil {
		return nil, MapDomainError(err)
	}

	// Check user can create datasets in this topic
	role, err := s.store.TopicMembers().GetRole(ctx, topic.ID, user.ID)
	if err != nil || !role.CanEdit() {
		return nil, NewPermissionDeniedError("create dataset", "topic", "editor")
	}

	// Create dataset
	metadata := req.GetMetadata()
	if metadata == nil {
		metadata = make(map[string]string)
	}
	if req.GetContentType() != "" {
		metadata["content_type"] = req.GetContentType()
	}

	dataset := &domain.Dataset{
		ID:          domain.DatasetID(uuid.New().String()),
		TopicID:     topic.ID,
		Name:        req.GetName(),
		Description: req.GetDescription(),
		Status:      domain.DatasetStatusDraft,
		Owners:      []domain.UserID{user.ID},
		CreatedBy:   user.ID,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
		Tags:        req.GetTags(),
		Metadata:    metadata,
	}

	if err := s.store.Datasets().Create(ctx, dataset); err != nil {
		return nil, MapDomainError(err)
	}

	// Audit log
	if s.auditLogger != nil {
		_ = s.auditLogger.LogMutation(ctx, "CREATE", "dataset", string(dataset.ID), "Created dataset: "+dataset.Name)
	}

	return &services.CreateDatasetResponse{
		Dataset: domainDatasetToProto(dataset),
	}, nil
}

// GetDataset retrieves dataset metadata.
func (s *DatasetServiceServer) GetDataset(ctx context.Context, req *services.GetDatasetRequest) (*services.GetDatasetResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.GetId() == "" {
		return nil, NewValidationError("id is required", map[string]string{
			"id": "must not be empty",
		})
	}

	dataset, err := s.store.Datasets().Get(ctx, domain.DatasetID(req.GetId()))
	if err != nil {
		return nil, MapDomainError(err)
	}

	// Deleted datasets should appear as not found
	if dataset.Status == domain.DatasetStatusDeleted {
		return nil, status.Error(codes.NotFound, "dataset not found")
	}

	// Check user has access to the topic
	user, _ := UserFromContext(ctx)
	if !s.canAccessDataset(ctx, dataset, user) {
		return nil, NewPermissionDeniedError("view", "dataset", "member")
	}

	return &services.GetDatasetResponse{
		Dataset: domainDatasetToProto(dataset),
	}, nil
}

// ListDatasets lists datasets with filtering.
func (s *DatasetServiceServer) ListDatasets(ctx context.Context, req *services.ListDatasetsRequest) (*services.ListDatasetsResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	user, _ := UserFromContext(ctx)

	filter := storage.DatasetFilter{
		Tags: req.GetTags(),
	}

	if req.GetTopicId() != "" {
		topicID := domain.TopicID(req.GetTopicId())
		filter.TopicID = &topicID
	}

	if req.GetPage() != nil {
		filter.Limit = int(req.GetPage().GetLimit())
		filter.Offset = int(req.GetPage().GetOffset())
	}

	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 1000 {
		filter.Limit = 1000
	}

	datasets, err := s.store.Datasets().List(ctx, filter)
	if err != nil {
		return nil, MapDomainError(err)
	}

	// Filter by visibility
	visibleDatasets := make([]*domain.Dataset, 0, len(datasets))
	for _, d := range datasets {
		if s.canAccessDataset(ctx, d, user) {
			visibleDatasets = append(visibleDatasets, d)
		}
	}

	protoDatasets := make([]*services.Dataset, len(visibleDatasets))
	for i, d := range visibleDatasets {
		protoDatasets[i] = domainDatasetToProto(d)
	}

	return &services.ListDatasetsResponse{
		Datasets: protoDatasets,
	}, nil
}

// UpdateDataset updates dataset metadata.
func (s *DatasetServiceServer) UpdateDataset(ctx context.Context, req *services.UpdateDatasetRequest) (*services.UpdateDatasetResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.GetId() == "" {
		return nil, NewValidationError("id is required", map[string]string{
			"id": "must not be empty",
		})
	}

	dataset, err := s.store.Datasets().Get(ctx, domain.DatasetID(req.GetId()))
	if err != nil {
		return nil, MapDomainError(err)
	}

	// Only owners can update
	user, ok := UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	if !dataset.IsOwner(user.ID) {
		return nil, NewPermissionDeniedError("update", "dataset", "owner")
	}

	// Update fields from request
	if req.GetName() != "" {
		dataset.Name = req.GetName()
	}
	if req.Description != nil {
		dataset.Description = req.GetDescription()
	}
	if len(req.GetTags()) > 0 {
		dataset.Tags = req.GetTags()
	}
	if req.ContentType != nil {
		if dataset.Metadata == nil {
			dataset.Metadata = make(map[string]string)
		}
		dataset.Metadata["content_type"] = req.GetContentType()
	}

	dataset.UpdatedAt = time.Now().UTC()

	if err := dataset.Validate(); err != nil {
		return nil, MapDomainError(err)
	}

	if err := s.store.Datasets().Update(ctx, dataset); err != nil {
		return nil, MapDomainError(err)
	}

	// Audit log
	if s.auditLogger != nil {
		_ = s.auditLogger.LogMutation(ctx, "UPDATE", "dataset", string(dataset.ID), "Updated dataset: "+dataset.Name)
	}

	return &services.UpdateDatasetResponse{
		Dataset: domainDatasetToProto(dataset),
	}, nil
}

// DeleteDataset soft-deletes a dataset.
func (s *DatasetServiceServer) DeleteDataset(ctx context.Context, req *services.DeleteDatasetRequest) (*services.DeleteDatasetResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.GetId() == "" {
		return nil, NewValidationError("id is required", map[string]string{
			"id": "must not be empty",
		})
	}

	dataset, err := s.store.Datasets().Get(ctx, domain.DatasetID(req.GetId()))
	if err != nil {
		return nil, MapDomainError(err)
	}

	// Only owners can delete
	user, ok := UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	if !dataset.IsOwner(user.ID) {
		return nil, NewPermissionDeniedError("delete", "dataset", "owner")
	}

	if err := s.store.Datasets().Delete(ctx, domain.DatasetID(req.GetId())); err != nil {
		return nil, MapDomainError(err)
	}

	// Audit log
	if s.auditLogger != nil {
		_ = s.auditLogger.LogMutation(ctx, "DELETE", "dataset", req.GetId(), "Deleted dataset: "+dataset.Name)
	}

	return &services.DeleteDatasetResponse{
		Success: true,
	}, nil
}

// UploadDataset uploads dataset content (streaming).
func (s *DatasetServiceServer) UploadDataset(_ services.DatasetService_UploadDatasetServer) error {
	// The proto uses a oneof pattern with Metadata and Chunk messages.
	// Full implementation requires proper handling of the streaming protocol.
	// TODO: Implement full streaming upload with chunked transfer
	return status.Errorf(codes.Unimplemented, "method UploadDataset not fully implemented - requires streaming protocol integration")
}

// DownloadDataset downloads dataset content (streaming).
func (s *DatasetServiceServer) DownloadDataset(req *services.DownloadDatasetRequest, stream services.DatasetService_DownloadDatasetServer) error {
	if s.store == nil || s.blobStore == nil {
		return status.Error(codes.Unavailable, "service not initialized")
	}

	ctx := stream.Context()

	if req.GetId() == "" {
		return NewValidationError("id is required", map[string]string{
			"id": "must not be empty",
		})
	}

	dataset, err := s.store.Datasets().Get(ctx, domain.DatasetID(req.GetId()))
	if err != nil {
		return MapDomainError(err)
	}

	// Check user access
	user, _ := UserFromContext(ctx)
	if !s.canAccessDataset(ctx, dataset, user) {
		return NewPermissionDeniedError("download", "dataset", "member")
	}

	// Get latest version
	version, err := s.store.Datasets().GetLatestVersion(ctx, dataset.ID)
	if err != nil {
		return MapDomainError(err)
	}

	if version == nil || version.Content == nil {
		return status.Error(codes.NotFound, "no content available for this version")
	}

	// Get chunks
	chunks, err := s.store.Datasets().ListChunks(ctx, version.ID)
	if err != nil {
		return MapDomainError(err)
	}

	// Stream chunks
	for i, chunk := range chunks {
		// Read from blob store
		reader, err := s.blobStore.Get(ctx, chunk.Hash)
		if err != nil {
			return status.Errorf(codes.Internal, "failed to read chunk %d: %v", i, err)
		}

		data, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			return status.Errorf(codes.Internal, "failed to read chunk data: %v", err)
		}

		if err := stream.Send(&services.DownloadDatasetResponse{
			Data: &services.DownloadDatasetResponse_Chunk{
				Chunk: &services.ChunkData{
					Data:  data,
					Index: int32(chunk.Index),
					Hash:  chunk.Hash,
				},
			},
		}); err != nil {
			return err
		}
	}

	return nil
}

// GetDatasetVersions lists versions of a dataset.
func (s *DatasetServiceServer) GetDatasetVersions(ctx context.Context, req *services.GetDatasetVersionsRequest) (*services.GetDatasetVersionsResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.GetDatasetId() == "" {
		return nil, NewValidationError("dataset_id is required", map[string]string{
			"dataset_id": "must not be empty",
		})
	}

	dataset, err := s.store.Datasets().Get(ctx, domain.DatasetID(req.GetDatasetId()))
	if err != nil {
		return nil, MapDomainError(err)
	}

	// Check user access
	user, _ := UserFromContext(ctx)
	if !s.canAccessDataset(ctx, dataset, user) {
		return nil, NewPermissionDeniedError("view versions", "dataset", "member")
	}

	versions, err := s.store.Datasets().ListVersions(ctx, dataset.ID)
	if err != nil {
		return nil, MapDomainError(err)
	}

	protoVersions := make([]*services.DatasetVersion, len(versions))
	for i, v := range versions {
		protoVersions[i] = domainVersionToProto(v)
	}

	return &services.GetDatasetVersionsResponse{
		Versions: protoVersions,
	}, nil
}

// GetVersion retrieves a specific version.
func (s *DatasetServiceServer) GetVersion(ctx context.Context, req *services.GetVersionRequest) (*services.GetVersionResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.GetDatasetId() == "" {
		return nil, NewValidationError("dataset_id is required", map[string]string{
			"dataset_id": "must not be empty",
		})
	}

	dataset, err := s.store.Datasets().Get(ctx, domain.DatasetID(req.GetDatasetId()))
	if err != nil {
		return nil, MapDomainError(err)
	}

	// Check user access
	user, _ := UserFromContext(ctx)
	if !s.canAccessDataset(ctx, dataset, user) {
		return nil, NewPermissionDeniedError("view version", "dataset", "member")
	}

	// Get version by version number
	versions, err := s.store.Datasets().ListVersions(ctx, dataset.ID)
	if err != nil {
		return nil, MapDomainError(err)
	}

	var version *domain.DatasetVersion
	for _, v := range versions {
		// Match by version number
		if v.Version == fmt.Sprintf("%d.0.0", req.GetVersion()) {
			version = v
			break
		}
	}

	if version == nil {
		return nil, status.Errorf(codes.NotFound, "version %d not found", req.GetVersion())
	}

	return &services.GetVersionResponse{
		Version: domainVersionToProto(version),
	}, nil
}

// GetChunk retrieves a specific chunk.
func (s *DatasetServiceServer) GetChunk(ctx context.Context, req *services.GetChunkRequest) (*services.GetChunkResponse, error) {
	if s.store == nil || s.blobStore == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.GetDatasetId() == "" {
		return nil, NewValidationError("dataset_id is required", map[string]string{
			"dataset_id": "must not be empty",
		})
	}

	dataset, err := s.store.Datasets().Get(ctx, domain.DatasetID(req.GetDatasetId()))
	if err != nil {
		return nil, MapDomainError(err)
	}

	// Check user access
	user, _ := UserFromContext(ctx)
	if !s.canAccessDataset(ctx, dataset, user) {
		return nil, NewPermissionDeniedError("get chunk", "dataset", "member")
	}

	// Get latest version
	version, err := s.store.Datasets().GetLatestVersion(ctx, dataset.ID)
	if err != nil {
		return nil, MapDomainError(err)
	}

	if version == nil {
		return nil, status.Error(codes.NotFound, "no version available")
	}

	chunk, err := s.store.Datasets().GetChunk(ctx, version.ID, int(req.GetChunkIndex()))
	if err != nil {
		return nil, MapDomainError(err)
	}

	// Read data from blob store
	reader, err := s.blobStore.Get(ctx, chunk.Hash)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read chunk: %v", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read chunk data: %v", err)
	}

	return &services.GetChunkResponse{
		Chunk: &services.ChunkData{
			Data:  data,
			Index: int32(chunk.Index),
			Hash:  chunk.Hash,
		},
	}, nil
}

// VerifyDataset verifies dataset integrity.
func (s *DatasetServiceServer) VerifyDataset(ctx context.Context, req *services.VerifyDatasetRequest) (*services.VerifyDatasetResponse, error) {
	if s.store == nil || s.blobStore == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.GetDatasetId() == "" {
		return nil, NewValidationError("dataset_id is required", map[string]string{
			"dataset_id": "must not be empty",
		})
	}

	dataset, err := s.store.Datasets().Get(ctx, domain.DatasetID(req.GetDatasetId()))
	if err != nil {
		return nil, MapDomainError(err)
	}

	// Check user access
	user, _ := UserFromContext(ctx)
	if !s.canAccessDataset(ctx, dataset, user) {
		return nil, NewPermissionDeniedError("verify", "dataset", "member")
	}

	// Get version to verify
	version, err := s.store.Datasets().GetLatestVersion(ctx, dataset.ID)
	if err != nil {
		return nil, MapDomainError(err)
	}

	if version == nil || version.Content == nil {
		return &services.VerifyDatasetResponse{
			Valid: false,
		}, nil
	}

	// Get and verify chunks
	chunks, err := s.store.Datasets().ListChunks(ctx, version.ID)
	if err != nil {
		return nil, MapDomainError(err)
	}

	valid := true
	for _, chunk := range chunks {
		reader, err := s.blobStore.Get(ctx, chunk.Hash)
		if err != nil {
			valid = false
			break
		}

		data, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			valid = false
			break
		}

		// Verify chunk hash
		chunkHasher := sha256.New()
		chunkHasher.Write(data)
		if hex.EncodeToString(chunkHasher.Sum(nil)) != chunk.Hash {
			valid = false
			break
		}
	}

	return &services.VerifyDatasetResponse{
		Valid: valid,
	}, nil
}

// SearchDatasets searches datasets by text query.
func (s *DatasetServiceServer) SearchDatasets(ctx context.Context, req *services.SearchDatasetsRequest) (*services.SearchDatasetsResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	user, _ := UserFromContext(ctx)

	filter := storage.DatasetFilter{
		Search: req.GetQuery(),
	}

	if req.GetPage() != nil {
		filter.Limit = int(req.GetPage().GetLimit())
		filter.Offset = int(req.GetPage().GetOffset())
	}

	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 1000 {
		filter.Limit = 1000
	}

	datasets, err := s.store.Datasets().List(ctx, filter)
	if err != nil {
		return nil, MapDomainError(err)
	}

	// Filter by visibility
	visibleDatasets := make([]*domain.Dataset, 0, len(datasets))
	for _, d := range datasets {
		if s.canAccessDataset(ctx, d, user) {
			visibleDatasets = append(visibleDatasets, d)
		}
	}

	protoDatasets := make([]*services.Dataset, len(visibleDatasets))
	for i, d := range visibleDatasets {
		protoDatasets[i] = domainDatasetToProto(d)
	}

	return &services.SearchDatasetsResponse{
		Datasets: protoDatasets,
	}, nil
}

// GetDatasetStats returns statistics for a dataset.
func (s *DatasetServiceServer) GetDatasetStats(ctx context.Context, req *services.GetDatasetStatsRequest) (*services.GetDatasetStatsResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.GetDatasetId() == "" {
		return nil, NewValidationError("dataset_id is required", map[string]string{
			"dataset_id": "must not be empty",
		})
	}

	dataset, err := s.store.Datasets().Get(ctx, domain.DatasetID(req.GetDatasetId()))
	if err != nil {
		return nil, MapDomainError(err)
	}

	// Check user access
	user, _ := UserFromContext(ctx)
	if !s.canAccessDataset(ctx, dataset, user) {
		return nil, NewPermissionDeniedError("view stats", "dataset", "member")
	}

	// Get latest version for content stats
	var totalSize int64
	if version, err := s.store.Datasets().GetLatestVersion(ctx, dataset.ID); err == nil && version != nil && version.Content != nil {
		totalSize = version.Content.Size
	}

	return &services.GetDatasetStatsResponse{
		DatasetId:            req.GetDatasetId(),
		VersionCount:         int32(dataset.VersionCount),
		TotalSizeAllVersions: totalSize,
	}, nil
}

// CopyDataset copies a dataset to another topic.
func (s *DatasetServiceServer) CopyDataset(ctx context.Context, req *services.CopyDatasetRequest) (*services.CopyDatasetResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.GetSourceDatasetId() == "" || req.GetTargetTopicId() == "" {
		return nil, NewValidationError("source_dataset_id and target_topic_id are required", map[string]string{
			"source_dataset_id": "must not be empty",
			"target_topic_id":   "must not be empty",
		})
	}

	user, ok := UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	// Get source dataset
	srcDataset, err := s.store.Datasets().Get(ctx, domain.DatasetID(req.GetSourceDatasetId()))
	if err != nil {
		return nil, MapDomainError(err)
	}

	// Check user can read source
	if !s.canAccessDataset(ctx, srcDataset, user) {
		return nil, NewPermissionDeniedError("copy", "source dataset", "member")
	}

	// Check target topic exists and user can write
	targetTopic, err := s.store.Topics().Get(ctx, domain.TopicID(req.GetTargetTopicId()))
	if err != nil {
		return nil, MapDomainError(err)
	}

	role, err := s.store.TopicMembers().GetRole(ctx, targetTopic.ID, user.ID)
	if err != nil || !role.CanEdit() {
		return nil, NewPermissionDeniedError("copy to", "target topic", "editor")
	}

	// Create new dataset in target topic
	newName := srcDataset.Name
	if req.GetNewName() != "" {
		newName = req.GetNewName()
	}

	newDataset := &domain.Dataset{
		ID:          domain.DatasetID(uuid.New().String()),
		TopicID:     targetTopic.ID,
		Name:        newName,
		Description: srcDataset.Description,
		Status:      domain.DatasetStatusActive,
		HasContent:  srcDataset.HasContent,
		Owners:      []domain.UserID{user.ID},
		CreatedBy:   user.ID,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
		Tags:        srcDataset.Tags,
		Metadata:    srcDataset.Metadata,
	}

	if err := s.store.Datasets().Create(ctx, newDataset); err != nil {
		return nil, MapDomainError(err)
	}

	// Audit log
	if s.auditLogger != nil {
		_ = s.auditLogger.LogMutation(ctx, "CREATE", "dataset", string(newDataset.ID),
			fmt.Sprintf("Copied dataset from %s to topic %s", srcDataset.ID, targetTopic.ID))
	}

	return &services.CopyDatasetResponse{
		Dataset: domainDatasetToProto(newDataset),
	}, nil
}

// StreamDatasetEvents streams dataset events.
func (s *DatasetServiceServer) StreamDatasetEvents(_ *services.StreamDatasetEventsRequest, _ services.DatasetService_StreamDatasetEventsServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamDatasetEvents not implemented - requires P2P event streaming")
}

// =============================================================================
// Helper methods
// =============================================================================

func (s *DatasetServiceServer) canAccessDataset(ctx context.Context, dataset *domain.Dataset, user *domain.User) bool {
	if user == nil {
		return false
	}

	// Owners always have access
	if dataset.IsOwner(user.ID) {
		return true
	}

	// Admins have access
	if user.Role == domain.UserRoleAdmin {
		return true
	}

	// Check if the topic is public
	if s.store != nil {
		topic, err := s.store.Topics().Get(ctx, dataset.TopicID)
		if err == nil && s.isPublicTopic(topic) {
			return true
		}

		// Check topic membership for private topics
		hasAccess, _ := s.store.TopicMembers().HasAccess(ctx, dataset.TopicID, user.ID)
		return hasAccess
	}

	return false
}

// isPublicTopic checks if a topic is public based on its metadata.
func (s *DatasetServiceServer) isPublicTopic(topic *domain.Topic) bool {
	if topic.Metadata != nil {
		if val, ok := topic.Metadata["is_public"]; ok {
			return val == "true"
		}
	}
	// Default to public
	return true
}

// =============================================================================
// Conversion helpers
// =============================================================================

func domainDatasetToProto(d *domain.Dataset) *services.Dataset {
	if d == nil {
		return nil
	}

	var ownerID string
	if len(d.Owners) > 0 {
		ownerID = string(d.Owners[0])
	}

	// Extract content_type from metadata if present
	contentType := ""
	if d.Metadata != nil {
		contentType = d.Metadata["content_type"]
	}

	return &services.Dataset{
		Id:          string(d.ID),
		TopicId:     string(d.TopicID),
		Name:        d.Name,
		Description: d.Description,
		ContentType: contentType,
		Status:      string(d.Status),
		OwnerId:     ownerID,
		Version:     int32(d.VersionCount),
		CreatedAt:   timestamppb.New(d.CreatedAt),
		UpdatedAt:   timestamppb.New(d.UpdatedAt),
		Tags:        d.Tags,
		Metadata:    d.Metadata,
	}
}

func domainVersionToProto(v *domain.DatasetVersion) *services.DatasetVersion {
	if v == nil {
		return nil
	}

	// Extract version number from string format like "1.0.0"
	var versionNum int32
	if v.Version != "" {
		_, _ = fmt.Sscanf(v.Version, "%d", &versionNum)
	}

	proto := &services.DatasetVersion{
		Version:   versionNum,
		DatasetId: string(v.DatasetID),
		CreatedBy: string(v.CreatedBy),
		CreatedAt: timestamppb.New(v.CreatedAt),
	}

	if v.Content != nil {
		proto.Hash = v.Content.Hash
		proto.Size = v.Content.Size
		proto.ChunkCount = int32(v.Content.ChunkCount)
	}

	return proto
}
