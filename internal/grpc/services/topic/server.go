// Package topic implements the TopicService gRPC service.
package topic

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	bibv1 "bib/api/gen/go/bib/v1"
	services "bib/api/gen/go/bib/v1/services"
	"bib/internal/domain"
	grpcerrors "bib/internal/grpc/errors"
	"bib/internal/grpc/interfaces"
	"bib/internal/grpc/middleware"
	"bib/internal/storage"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Config holds configuration for the topic service server.
type Config struct {
	Store       storage.Store
	AuditLogger interfaces.AuditLogger
	NodeMode    string // "full", "selective", "proxy"
}

// Server implements the TopicService gRPC service.
type Server struct {
	services.UnimplementedTopicServiceServer
	store       storage.Store
	auditLogger interfaces.AuditLogger
	nodeMode    string
}

// NewServer creates a new topic service server.
func NewServer() *Server {
	return &Server{}
}

// NewServerWithConfig creates a new topic service server with dependencies.
func NewServerWithConfig(cfg Config) *Server {
	return &Server{
		store:       cfg.Store,
		auditLogger: cfg.AuditLogger,
		nodeMode:    cfg.NodeMode,
	}
}

// CreateTopic creates a new topic.
func (s *Server) CreateTopic(ctx context.Context, req *services.CreateTopicRequest) (*services.CreateTopicResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.Name == "" {
		return nil, grpcerrors.NewValidationError("name is required", map[string]string{
			"name": "must not be empty",
		})
	}

	user, ok := middleware.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	existing, _ := s.store.Topics().GetByName(ctx, req.Name)
	if existing != nil {
		return nil, grpcerrors.NewValidationError("topic already exists", map[string]string{
			"name": "a topic with this name already exists",
		})
	}

	topic := &domain.Topic{
		ID:          domain.TopicID(uuid.New().String()),
		Name:        req.Name,
		Description: req.Description,
		TableSchema: req.Schema,
		Status:      domain.TopicStatusActive,
		Owners:      []domain.UserID{user.ID},
		CreatedBy:   user.ID,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
		Tags:        req.Tags,
		Metadata:    req.Metadata,
	}

	if err := s.store.Topics().Create(ctx, topic); err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	member := &storage.TopicMember{
		ID:        uuid.New().String(),
		TopicID:   topic.ID,
		UserID:    user.ID,
		Role:      storage.TopicMemberRoleOwner,
		InvitedBy: user.ID,
		InvitedAt: time.Now().UTC(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	now := time.Now().UTC()
	member.AcceptedAt = &now
	_ = s.store.TopicMembers().Create(ctx, member)

	if s.auditLogger != nil {
		_ = s.auditLogger.LogServiceAction(ctx, "CREATE", "topic", string(topic.ID), map[string]interface{}{
			"name": topic.Name,
		})
	}

	return &services.CreateTopicResponse{
		Topic: topicToProto(topic),
	}, nil
}

// GetTopic retrieves a topic by ID or name.
func (s *Server) GetTopic(ctx context.Context, req *services.GetTopicRequest) (*services.GetTopicResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	var topic *domain.Topic
	var err error

	if req.Id != "" {
		topic, err = s.store.Topics().Get(ctx, domain.TopicID(req.Id))
	} else if req.Name != "" {
		topic, err = s.store.Topics().GetByName(ctx, req.Name)
	} else {
		return nil, grpcerrors.NewValidationError("id or name is required", map[string]string{
			"id":   "either id or name must be provided",
			"name": "either id or name must be provided",
		})
	}

	if err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	user, _ := middleware.UserFromContext(ctx)
	if !s.canAccessTopic(ctx, topic, user) {
		return nil, grpcerrors.NewPermissionDeniedError("view", "topic", "member")
	}

	resp := &services.GetTopicResponse{
		Topic: topicToProto(topic),
	}

	if user != nil {
		member, _ := s.store.TopicMembers().Get(ctx, topic.ID, user.ID)
		if member != nil {
			resp.Subscribed = true
			resp.Subscription = memberToSubscription(member, topic)
		}
	}

	return resp, nil
}

// ListTopics lists topics with filtering.
func (s *Server) ListTopics(ctx context.Context, req *services.ListTopicsRequest) (*services.ListTopicsResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	user, _ := middleware.UserFromContext(ctx)

	filter := storage.TopicFilter{
		Status: domain.TopicStatus(req.Status),
		Tags:   req.Tags,
	}

	if req.OwnerId != "" {
		ownerID := domain.UserID(req.OwnerId)
		filter.OwnerID = &ownerID
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

	topics, err := s.store.Topics().List(ctx, filter)
	if err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	visibleTopics := make([]*domain.Topic, 0, len(topics))
	for _, t := range topics {
		if req.PublicOnly && !s.isPublicTopic(t) {
			continue
		}
		if req.SubscribedOnly {
			if user == nil {
				continue
			}
			member, _ := s.store.TopicMembers().Get(ctx, t.ID, user.ID)
			if member == nil {
				continue
			}
		}
		if s.canAccessTopic(ctx, t, user) {
			visibleTopics = append(visibleTopics, t)
		}
	}

	total, _ := s.store.Topics().Count(ctx, filter)

	protoTopics := make([]*services.Topic, len(visibleTopics))
	for i, t := range visibleTopics {
		protoTopics[i] = topicToProto(t)
	}

	return &services.ListTopicsResponse{
		Topics: protoTopics,
		PageInfo: &bibv1.PageInfo{
			TotalCount: total,
			HasMore:    int64(filter.Offset+len(visibleTopics)) < total,
			PageSize:   int32(len(visibleTopics)),
		},
	}, nil
}

// UpdateTopic updates topic metadata.
func (s *Server) UpdateTopic(ctx context.Context, req *services.UpdateTopicRequest) (*services.UpdateTopicResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.Id == "" {
		return nil, grpcerrors.NewValidationError("id is required", map[string]string{
			"id": "must not be empty",
		})
	}

	topic, err := s.store.Topics().Get(ctx, domain.TopicID(req.Id))
	if err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	user, ok := middleware.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	role, err := s.store.TopicMembers().GetRole(ctx, topic.ID, user.ID)
	if err != nil || role != storage.TopicMemberRoleOwner {
		return nil, grpcerrors.NewPermissionDeniedError("update", "topic", "owner")
	}

	if req.Name != nil {
		topic.Name = *req.Name
	}
	if req.Description != nil {
		topic.Description = *req.Description
	}
	if req.Schema != nil {
		topic.TableSchema = *req.Schema
	}
	if req.UpdateTags {
		topic.Tags = req.Tags
	}
	if req.UpdateMetadata {
		topic.Metadata = req.Metadata
	}

	topic.UpdatedAt = time.Now().UTC()

	if err := topic.Validate(); err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	if err := s.store.Topics().Update(ctx, topic); err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogServiceAction(ctx, "UPDATE", "topic", string(topic.ID), nil)
	}

	return &services.UpdateTopicResponse{
		Topic: topicToProto(topic),
	}, nil
}

// DeleteTopic deletes a topic.
func (s *Server) DeleteTopic(ctx context.Context, req *services.DeleteTopicRequest) (*services.DeleteTopicResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.Id == "" {
		return nil, grpcerrors.NewValidationError("id is required", map[string]string{
			"id": "must not be empty",
		})
	}

	topic, err := s.store.Topics().Get(ctx, domain.TopicID(req.Id))
	if err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	user, ok := middleware.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	role, err := s.store.TopicMembers().GetRole(ctx, topic.ID, user.ID)
	if err != nil || role != storage.TopicMemberRoleOwner {
		return nil, grpcerrors.NewPermissionDeniedError("delete", "topic", "owner")
	}

	// Check if has datasets and force flag
	if topic.DatasetCount > 0 && !req.Force {
		return nil, grpcerrors.NewPreconditionError("topic has datasets", map[string]string{
			"dataset_count": "topic has datasets, use force=true to delete anyway",
		})
	}

	if err := s.store.Topics().Delete(ctx, topic.ID); err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	if s.auditLogger != nil {
		_ = s.auditLogger.LogServiceAction(ctx, "DELETE", "topic", string(topic.ID), nil)
	}

	return &services.DeleteTopicResponse{
		Success:          true,
		DatasetsAffected: int64(topic.DatasetCount),
	}, nil
}

// Subscribe subscribes to a topic.
func (s *Server) Subscribe(ctx context.Context, req *services.SubscribeRequest) (*services.SubscribeResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	user, ok := middleware.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	var topic *domain.Topic
	var err error

	if req.TopicId != "" {
		topic, err = s.store.Topics().Get(ctx, domain.TopicID(req.TopicId))
	} else if req.TopicName != "" {
		topic, err = s.store.Topics().GetByName(ctx, req.TopicName)
	} else {
		return nil, grpcerrors.NewValidationError("topic_id or topic_name is required", map[string]string{
			"topic_id":   "either topic_id or topic_name must be provided",
			"topic_name": "either topic_id or topic_name must be provided",
		})
	}

	if err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	if !s.canAccessTopic(ctx, topic, user) {
		return nil, grpcerrors.NewPermissionDeniedError("subscribe", "topic", "member")
	}

	existing, _ := s.store.TopicMembers().Get(ctx, topic.ID, user.ID)
	if existing != nil {
		return &services.SubscribeResponse{
			Subscription: memberToSubscription(existing, topic),
		}, nil
	}

	member := &storage.TopicMember{
		ID:        uuid.New().String(),
		TopicID:   topic.ID,
		UserID:    user.ID,
		Role:      storage.TopicMemberRoleViewer,
		InvitedBy: user.ID,
		InvitedAt: time.Now().UTC(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	now := time.Now().UTC()
	member.AcceptedAt = &now

	if err := s.store.TopicMembers().Create(ctx, member); err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	subscription := memberToSubscription(member, topic)
	if req.SyncNow {
		subscription.SyncStatus = "syncing"
	}

	return &services.SubscribeResponse{
		Subscription: subscription,
	}, nil
}

// Unsubscribe unsubscribes from a topic.
func (s *Server) Unsubscribe(ctx context.Context, req *services.UnsubscribeRequest) (*services.UnsubscribeResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	user, ok := middleware.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	if req.TopicId == "" {
		return nil, grpcerrors.NewValidationError("topic_id is required", map[string]string{
			"topic_id": "must not be empty",
		})
	}

	topicID := domain.TopicID(req.TopicId)

	role, _ := s.store.TopicMembers().GetRole(ctx, topicID, user.ID)
	if role == storage.TopicMemberRoleOwner {
		count, _ := s.store.TopicMembers().CountOwners(ctx, topicID)
		if count <= 1 {
			return nil, grpcerrors.NewPreconditionError("cannot unsubscribe as last owner", map[string]string{
				"owner": "you are the last owner, transfer ownership first",
			})
		}
	}

	if err := s.store.TopicMembers().Delete(ctx, topicID, user.ID); err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	return &services.UnsubscribeResponse{
		Success:    true,
		BytesFreed: 0,
	}, nil
}

// ListSubscriptions lists current subscriptions.
func (s *Server) ListSubscriptions(ctx context.Context, req *services.ListSubscriptionsRequest) (*services.ListSubscriptionsResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	user, ok := middleware.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	members, err := s.store.TopicMembers().ListByUser(ctx, user.ID)
	if err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	subscriptions := make([]*services.Subscription, 0, len(members))
	for _, m := range members {
		topic, err := s.store.Topics().Get(ctx, m.TopicID)
		if err != nil {
			continue
		}

		sub := memberToSubscription(m, topic)
		if req.SyncStatus != "" && sub.SyncStatus != req.SyncStatus {
			continue
		}
		subscriptions = append(subscriptions, sub)
	}

	return &services.ListSubscriptionsResponse{
		Subscriptions: subscriptions,
		PageInfo: &bibv1.PageInfo{
			TotalCount: int64(len(subscriptions)),
			PageSize:   int32(len(subscriptions)),
		},
	}, nil
}

// GetSubscription gets subscription details for a topic.
func (s *Server) GetSubscription(ctx context.Context, req *services.GetSubscriptionRequest) (*services.GetSubscriptionResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	user, ok := middleware.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	if req.TopicId == "" {
		return nil, grpcerrors.NewValidationError("topic_id is required", map[string]string{
			"topic_id": "must not be empty",
		})
	}

	topicID := domain.TopicID(req.TopicId)

	member, err := s.store.TopicMembers().Get(ctx, topicID, user.ID)
	if err != nil {
		return nil, grpcerrors.NewResourceNotFoundError("subscription", req.TopicId)
	}

	topic, err := s.store.Topics().Get(ctx, topicID)
	if err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	return &services.GetSubscriptionResponse{
		Subscription: memberToSubscription(member, topic),
	}, nil
}

// StreamTopicUpdates streams topic changes in real-time.
func (s *Server) StreamTopicUpdates(req *services.StreamTopicUpdatesRequest, stream services.TopicService_StreamTopicUpdatesServer) error {
	return status.Error(codes.Unimplemented, "streaming topic updates not yet implemented")
}

// GetTopicStats returns statistics for a topic.
func (s *Server) GetTopicStats(ctx context.Context, req *services.GetTopicStatsRequest) (*services.GetTopicStatsResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.TopicId == "" {
		return nil, grpcerrors.NewValidationError("topic_id is required", map[string]string{
			"topic_id": "must not be empty",
		})
	}

	topic, err := s.store.Topics().Get(ctx, domain.TopicID(req.TopicId))
	if err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	user, _ := middleware.UserFromContext(ctx)
	if !s.canAccessTopic(ctx, topic, user) {
		return nil, grpcerrors.NewPermissionDeniedError("view", "topic", "member")
	}

	members, _ := s.store.TopicMembers().ListByTopic(ctx, topic.ID, storage.TopicMemberFilter{})
	subscriberCount := int64(len(members))

	return &services.GetTopicStatsResponse{
		TopicId:         string(topic.ID),
		DatasetCount:    int64(topic.DatasetCount),
		SubscriberCount: subscriberCount,
		LastActivity:    timestamppb.New(topic.UpdatedAt),
	}, nil
}

// SearchTopics searches topics by text query.
func (s *Server) SearchTopics(ctx context.Context, req *services.SearchTopicsRequest) (*services.SearchTopicsResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if err := grpcerrors.ValidateSearchQuery(req.Query); err != nil {
		return nil, err
	}

	user, _ := middleware.UserFromContext(ctx)

	filter := storage.TopicFilter{
		Search: req.Query,
		Tags:   req.Tags,
	}

	if req.Page != nil {
		filter.Limit = int(req.Page.Limit)
		filter.Offset = int(req.Page.Offset)
	}
	if filter.Limit <= 0 {
		filter.Limit = 50
	}

	topics, err := s.store.Topics().List(ctx, filter)
	if err != nil {
		return nil, grpcerrors.MapDomainError(err)
	}

	visibleTopics := make([]*domain.Topic, 0, len(topics))
	for _, t := range topics {
		if req.PublicOnly && !s.isPublicTopic(t) {
			continue
		}
		if s.canAccessTopic(ctx, t, user) {
			visibleTopics = append(visibleTopics, t)
		}
	}

	total, _ := s.store.Topics().Count(ctx, filter)

	protoTopics := make([]*services.Topic, len(visibleTopics))
	for i, t := range visibleTopics {
		protoTopics[i] = topicToProto(t)
	}

	return &services.SearchTopicsResponse{
		Topics: protoTopics,
		PageInfo: &bibv1.PageInfo{
			TotalCount: total,
			HasMore:    int64(filter.Offset+len(visibleTopics)) < total,
			PageSize:   int32(len(visibleTopics)),
		},
	}, nil
}

// Helper methods

func (s *Server) canAccessTopic(ctx context.Context, topic *domain.Topic, user *domain.User) bool {
	if s.isPublicTopic(topic) {
		return true
	}
	if user == nil {
		return false
	}
	if user.Role == domain.UserRoleAdmin {
		return true
	}
	hasAccess, _ := s.store.TopicMembers().HasAccess(ctx, topic.ID, user.ID)
	return hasAccess
}

func (s *Server) isPublicTopic(topic *domain.Topic) bool {
	if topic.Metadata != nil {
		if val, ok := topic.Metadata["is_public"]; ok {
			return val == "true"
		}
		if val, ok := topic.Metadata["public"]; ok {
			return val == "true"
		}
	}
	return true // Default to public
}

// Conversion helpers

func topicToProto(t *domain.Topic) *services.Topic {
	if t == nil {
		return nil
	}

	ownerId := string(t.CreatedBy)
	if len(t.Owners) > 0 {
		ownerId = string(t.Owners[0])
	}

	isPublic := true
	if t.Metadata != nil {
		if val, ok := t.Metadata["is_public"]; ok {
			isPublic = val == "true"
		} else if val, ok := t.Metadata["public"]; ok {
			isPublic = val == "true"
		}
	}

	return &services.Topic{
		Id:           string(t.ID),
		Name:         t.Name,
		Description:  t.Description,
		Schema:       t.TableSchema,
		DatasetCount: int64(t.DatasetCount),
		Status:       string(t.Status),
		OwnerId:      ownerId,
		IsPublic:     isPublic,
		CreatedAt:    timestamppb.New(t.CreatedAt),
		UpdatedAt:    timestamppb.New(t.UpdatedAt),
		Tags:         t.Tags,
		Metadata:     t.Metadata,
	}
}

func memberToSubscription(m *storage.TopicMember, topic *domain.Topic) *services.Subscription {
	sub := &services.Subscription{
		TopicId:      string(m.TopicID),
		TopicName:    topic.Name,
		SubscribedAt: timestamppb.New(m.InvitedAt),
		SyncStatus:   "synced",
		AutoSync:     true,
	}

	if m.AcceptedAt != nil {
		sub.LastSyncAt = timestamppb.New(*m.AcceptedAt)
	}

	sub.TotalDatasets = int64(topic.DatasetCount)
	sub.SyncedDatasets = int64(topic.DatasetCount)

	return sub
}

func generateSecureToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
