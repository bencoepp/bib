// Package dataset implements the DatasetService gRPC service.
package dataset

import (
	"context"
	"time"

	bibv1 "bib/api/gen/go/bib/v1"
	services "bib/api/gen/go/bib/v1/services"
	"bib/internal/domain"
	grpcerrors "bib/internal/grpc/errors"
	"bib/internal/grpc/interfaces"
	"bib/internal/grpc/middleware"
	"bib/internal/storage"
	"bib/internal/storage/blob"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultChunkSize = 256 * 1024 // 256KB
)

// Config holds configuration for the dataset service server.
type Config struct {
	Store       storage.Store
	BlobStore   blob.Store
	AuditLogger interfaces.AuditLogger
	NodeMode    string
}

// Server implements the DatasetService gRPC service.
type Server struct {
	services.UnimplementedDatasetServiceServer
	store       storage.Store
	blobStore   blob.Store
	auditLogger interfaces.AuditLogger
	nodeMode    string
}

// NewServer creates a new dataset service server.
func NewServer() *Server {
	return &Server{}
}

// NewServerWithConfig creates a new dataset service server with dependencies.
func NewServerWithConfig(cfg Config) *Server {
	return &Server{
		store:       cfg.Store,
		blobStore:   cfg.BlobStore,
		auditLogger: cfg.AuditLogger,
		nodeMode:    cfg.NodeMode,
	}
}

// SetStore sets the storage dependency.
func (s *Server) SetStore(store storage.Store) {
	s.store = store
}

// SetBlobStore sets the blob storage dependency.
func (s *Server) SetBlobStore(blobStore blob.Store) {
	s.blobStore = blobStore
}

// CreateDataset creates a new dataset.
func (s *Server) CreateDataset(ctx context.Context, req *services.CreateDatasetRequest) (*services.CreateDatasetResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	violations := make(map[string]string)
	if req.GetTopicId() == "" {
		violations["topic_id"] = "must not be empty"
	}
	if req.GetName() == "" {
		violations["name"] = "must not be empty"
	}
	if len(violations) > 0 {
		return nil, grpcerrors.NewValidationError("invalid create dataset request", violations)
	}

	user, ok := middleware.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	topic, err := s.store.Topics().Get(ctx, domain.TopicID(req.GetTopicId()))
	if err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	role, err := s.store.TopicMembers().GetRole(ctx, topic.ID, user.ID)
	if err != nil {
		return nil, grpcerrors.NewPermissionDeniedError("create", "dataset", "contributor")
	}
	if role != storage.TopicMemberRoleOwner && role != storage.TopicMemberRoleEditor {
		return nil, grpcerrors.NewPermissionDeniedError("create", "dataset", "contributor")
	}

	dataset := &domain.Dataset{
		ID:          domain.DatasetID(uuid.New().String()),
		TopicID:     topic.ID,
		Name:        req.GetName(),
		Description: req.GetDescription(),
		Status:      domain.DatasetStatusActive,
		Owners:      []domain.UserID{user.ID},
		CreatedBy:   user.ID,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
		Tags:        req.GetTags(),
		Metadata:    req.GetMetadata(),
	}

	if err := s.store.Datasets().Create(ctx, dataset); err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogServiceAction(ctx, "CREATE", "dataset", string(dataset.ID), map[string]interface{}{
			"name":     dataset.Name,
			"topic_id": string(topic.ID),
		})
	}

	return &services.CreateDatasetResponse{
		Dataset: datasetToProto(dataset),
	}, nil
}

// GetDataset retrieves a dataset by ID.
func (s *Server) GetDataset(ctx context.Context, req *services.GetDatasetRequest) (*services.GetDatasetResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.GetId() == "" {
		return nil, grpcerrors.NewValidationError("id is required", map[string]string{
			"id": "must not be empty",
		})
	}

	dataset, err := s.store.Datasets().Get(ctx, domain.DatasetID(req.GetId()))
	if err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	return &services.GetDatasetResponse{
		Dataset: datasetToProto(dataset),
	}, nil
}

// ListDatasets lists datasets with filtering.
func (s *Server) ListDatasets(ctx context.Context, req *services.ListDatasetsRequest) (*services.ListDatasetsResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	filter := storage.DatasetFilter{
		Tags: req.GetTags(),
	}

	if req.GetTopicId() != "" {
		topicID := domain.TopicID(req.GetTopicId())
		filter.TopicID = &topicID
	}

	if req.Page != nil {
		filter.Limit = int(req.Page.Limit)
		filter.Offset = int(req.Page.Offset)
	}
	if req.Sort != nil {
		filter.OrderBy = req.Sort.Field
		filter.OrderDesc = req.Sort.Descending
	}

	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 1000 {
		filter.Limit = 1000
	}

	datasets, err := s.store.Datasets().List(ctx, filter)
	if err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	total, _ := s.store.Datasets().Count(ctx, filter)

	protoDatasets := make([]*services.Dataset, len(datasets))
	for i, d := range datasets {
		protoDatasets[i] = datasetToProto(d)
	}

	return &services.ListDatasetsResponse{
		Datasets: protoDatasets,
		PageInfo: &bibv1.PageInfo{
			TotalCount: total,
			HasMore:    int64(filter.Offset+len(datasets)) < total,
			PageSize:   int32(len(datasets)),
		},
	}, nil
}

// UpdateDataset updates dataset metadata.
func (s *Server) UpdateDataset(ctx context.Context, req *services.UpdateDatasetRequest) (*services.UpdateDatasetResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.GetId() == "" {
		return nil, grpcerrors.NewValidationError("id is required", map[string]string{
			"id": "must not be empty",
		})
	}

	dataset, err := s.store.Datasets().Get(ctx, domain.DatasetID(req.GetId()))
	if err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	user, ok := middleware.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	if !dataset.IsOwner(user.ID) && user.Role != domain.UserRoleAdmin {
		return nil, grpcerrors.NewPermissionDeniedError("update", "dataset", "owner")
	}

	if req.Name != nil {
		dataset.Name = *req.Name
	}
	if req.Description != nil {
		dataset.Description = *req.Description
	}
	if req.UpdateTags {
		dataset.Tags = req.Tags
	}
	if req.UpdateMetadata {
		dataset.Metadata = req.Metadata
	}

	dataset.UpdatedAt = time.Now().UTC()

	if err := s.store.Datasets().Update(ctx, dataset); err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogServiceAction(ctx, "UPDATE", "dataset", string(dataset.ID), nil)
	}

	return &services.UpdateDatasetResponse{
		Dataset: datasetToProto(dataset),
	}, nil
}

// DeleteDataset deletes a dataset.
func (s *Server) DeleteDataset(ctx context.Context, req *services.DeleteDatasetRequest) (*services.DeleteDatasetResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.GetId() == "" {
		return nil, grpcerrors.NewValidationError("id is required", map[string]string{
			"id": "must not be empty",
		})
	}

	dataset, err := s.store.Datasets().Get(ctx, domain.DatasetID(req.GetId()))
	if err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	user, ok := middleware.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	if !dataset.IsOwner(user.ID) && user.Role != domain.UserRoleAdmin {
		return nil, grpcerrors.NewPermissionDeniedError("delete", "dataset", "owner")
	}

	if err := s.store.Datasets().Delete(ctx, dataset.ID); err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogServiceAction(ctx, "DELETE", "dataset", string(dataset.ID), nil)
	}

	return &services.DeleteDatasetResponse{
		Success: true,
	}, nil
}

// Conversion helpers

func datasetToProto(d *domain.Dataset) *services.Dataset {
	if d == nil {
		return nil
	}

	ownerId := ""
	if len(d.Owners) > 0 {
		ownerId = string(d.Owners[0])
	}

	return &services.Dataset{
		Id:          string(d.ID),
		TopicId:     string(d.TopicID),
		Name:        d.Name,
		Description: d.Description,
		Status:      string(d.Status),
		OwnerId:     ownerId,
		CreatedAt:   timestamppb.New(d.CreatedAt),
		UpdatedAt:   timestamppb.New(d.UpdatedAt),
		Tags:        d.Tags,
		Metadata:    d.Metadata,
	}
}
