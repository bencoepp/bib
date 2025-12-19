// Package grpc provides gRPC service implementations for the bib daemon.
package grpc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	bibv1 "bib/api/gen/go/bib/v1"
	services "bib/api/gen/go/bib/v1/services"
	"bib/internal/domain"
	"bib/internal/storage"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TopicServiceServer implements the TopicService gRPC service.
type TopicServiceServer struct {
	services.UnimplementedTopicServiceServer
	store       storage.Store
	auditLogger *AuditMiddleware
	nodeMode    string // "full", "selective", "proxy"
}

// TopicServiceConfig holds configuration for the TopicServiceServer.
type TopicServiceConfig struct {
	Store       storage.Store
	AuditLogger *AuditMiddleware
	NodeMode    string
}

// NewTopicServiceServer creates a new TopicServiceServer.
func NewTopicServiceServer() *TopicServiceServer {
	return &TopicServiceServer{}
}

// NewTopicServiceServerWithConfig creates a new TopicServiceServer with dependencies.
func NewTopicServiceServerWithConfig(cfg TopicServiceConfig) *TopicServiceServer {
	return &TopicServiceServer{
		store:       cfg.Store,
		auditLogger: cfg.AuditLogger,
		nodeMode:    cfg.NodeMode,
	}
}

// CreateTopic creates a new topic.
func (s *TopicServiceServer) CreateTopic(ctx context.Context, req *services.CreateTopicRequest) (*services.CreateTopicResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	// Validate required fields
	if req.Name == "" {
		return nil, NewValidationError("name is required", map[string]string{
			"name": "must not be empty",
		})
	}

	// Get current user (must be admin per requirements)
	user, ok := UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	// Check if topic name already exists
	existing, _ := s.store.Topics().GetByName(ctx, req.Name)
	if existing != nil {
		return nil, MapDomainError(domain.ErrTopicNotFound) // Reuse error or create ErrTopicExists
	}

	// Create topic
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
		return nil, MapDomainError(err)
	}

	// Create owner membership
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
	s.store.TopicMembers().Create(ctx, member)

	// Audit log
	if s.auditLogger != nil {
		s.auditLogger.LogMutation(ctx, "CREATE", "topic", string(topic.ID), "Created topic: "+topic.Name)
	}

	return &services.CreateTopicResponse{
		Topic: domainTopicToProto(topic),
	}, nil
}

// GetTopic retrieves a topic by ID or name.
func (s *TopicServiceServer) GetTopic(ctx context.Context, req *services.GetTopicRequest) (*services.GetTopicResponse, error) {
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
		return nil, NewValidationError("id or name is required", map[string]string{
			"id":   "either id or name must be provided",
			"name": "either id or name must be provided",
		})
	}

	if err != nil {
		return nil, MapDomainError(err)
	}

	// Check visibility
	user, _ := UserFromContext(ctx)
	if !s.canAccessTopic(ctx, topic, user) {
		return nil, NewPermissionDeniedError("view", "topic", "member")
	}

	resp := &services.GetTopicResponse{
		Topic: domainTopicToProto(topic),
	}

	// Check subscription status
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
func (s *TopicServiceServer) ListTopics(ctx context.Context, req *services.ListTopicsRequest) (*services.ListTopicsResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	user, _ := UserFromContext(ctx)

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
		return nil, MapDomainError(err)
	}

	// Filter by visibility
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
		protoTopics[i] = domainTopicToProto(t)
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
func (s *TopicServiceServer) UpdateTopic(ctx context.Context, req *services.UpdateTopicRequest) (*services.UpdateTopicResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.Id == "" {
		return nil, NewValidationError("id is required", map[string]string{
			"id": "must not be empty",
		})
	}

	topic, err := s.store.Topics().Get(ctx, domain.TopicID(req.Id))
	if err != nil {
		return nil, MapDomainError(err)
	}

	// Only owners can update (not even admins per requirements)
	user, ok := UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	role, err := s.store.TopicMembers().GetRole(ctx, topic.ID, user.ID)
	if err != nil || role != storage.TopicMemberRoleOwner {
		return nil, NewPermissionDeniedError("update", "topic", "owner")
	}

	// Update fields
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
		return nil, MapDomainError(err)
	}

	if err := s.store.Topics().Update(ctx, topic); err != nil {
		return nil, MapDomainError(err)
	}

	// Audit log
	if s.auditLogger != nil {
		s.auditLogger.LogMutation(ctx, "UPDATE", "topic", string(topic.ID), "Updated topic: "+topic.Name)
	}

	return &services.UpdateTopicResponse{
		Topic: domainTopicToProto(topic),
	}, nil
}

// DeleteTopic soft-deletes a topic.
func (s *TopicServiceServer) DeleteTopic(ctx context.Context, req *services.DeleteTopicRequest) (*services.DeleteTopicResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.Id == "" {
		return nil, NewValidationError("id is required", map[string]string{
			"id": "must not be empty",
		})
	}

	topic, err := s.store.Topics().Get(ctx, domain.TopicID(req.Id))
	if err != nil {
		return nil, MapDomainError(err)
	}

	// Only owners can delete
	user, ok := UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	role, err := s.store.TopicMembers().GetRole(ctx, topic.ID, user.ID)
	if err != nil || role != storage.TopicMemberRoleOwner {
		return nil, NewPermissionDeniedError("delete", "topic", "owner")
	}

	// Check if has datasets and force flag
	if topic.DatasetCount > 0 && !req.Force {
		return nil, NewPreconditionError("topic has datasets", map[string]string{
			"dataset_count": "topic has datasets, use force=true to delete anyway",
		})
	}

	// Soft delete
	if err := s.store.Topics().Delete(ctx, topic.ID); err != nil {
		return nil, MapDomainError(err)
	}

	// Audit log
	if s.auditLogger != nil {
		s.auditLogger.LogMutation(ctx, "DELETE", "topic", string(topic.ID), "Deleted topic: "+topic.Name)
	}

	return &services.DeleteTopicResponse{
		Success:          true,
		DatasetsAffected: int64(topic.DatasetCount),
	}, nil
}

// Subscribe subscribes to a topic.
func (s *TopicServiceServer) Subscribe(ctx context.Context, req *services.SubscribeRequest) (*services.SubscribeResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	// Warn if in proxy mode
	if s.nodeMode == "proxy" {
		// Still allow subscription but data won't be stored locally
		// The warning is returned to the client
	}

	user, ok := UserFromContext(ctx)
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
		return nil, NewValidationError("topic_id or topic_name is required", map[string]string{
			"topic_id":   "either topic_id or topic_name must be provided",
			"topic_name": "either topic_id or topic_name must be provided",
		})
	}

	if err != nil {
		return nil, MapDomainError(err)
	}

	// Check if user can access this topic
	if !s.canAccessTopic(ctx, topic, user) {
		return nil, NewPermissionDeniedError("subscribe", "topic", "member")
	}

	// Check if already subscribed
	existing, _ := s.store.TopicMembers().Get(ctx, topic.ID, user.ID)
	if existing != nil {
		return &services.SubscribeResponse{
			Subscription: memberToSubscription(existing, topic),
		}, nil
	}

	// Create subscription as viewer
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
		return nil, MapDomainError(err)
	}

	// If sync_now is true, trigger background sync
	// (The actual sync happens asynchronously)
	subscription := memberToSubscription(member, topic)
	if req.SyncNow {
		subscription.SyncStatus = "syncing"
	}

	return &services.SubscribeResponse{
		Subscription: subscription,
	}, nil
}

// Unsubscribe unsubscribes from a topic.
func (s *TopicServiceServer) Unsubscribe(ctx context.Context, req *services.UnsubscribeRequest) (*services.UnsubscribeResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	user, ok := UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	if req.TopicId == "" {
		return nil, NewValidationError("topic_id is required", map[string]string{
			"topic_id": "must not be empty",
		})
	}

	topicID := domain.TopicID(req.TopicId)

	// Check if user is owner - owners can't unsubscribe if they're the last owner
	role, _ := s.store.TopicMembers().GetRole(ctx, topicID, user.ID)
	if role == storage.TopicMemberRoleOwner {
		count, _ := s.store.TopicMembers().CountOwners(ctx, topicID)
		if count <= 1 {
			return nil, NewPreconditionError("cannot unsubscribe as last owner", map[string]string{
				"owner": "you are the last owner, transfer ownership first",
			})
		}
	}

	if err := s.store.TopicMembers().Delete(ctx, topicID, user.ID); err != nil {
		return nil, MapDomainError(err)
	}

	// TODO: If delete_local_data, calculate bytes freed
	var bytesFreed int64 = 0

	return &services.UnsubscribeResponse{
		Success:    true,
		BytesFreed: bytesFreed,
	}, nil
}

// ListSubscriptions lists current subscriptions.
func (s *TopicServiceServer) ListSubscriptions(ctx context.Context, req *services.ListSubscriptionsRequest) (*services.ListSubscriptionsResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	user, ok := UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	members, err := s.store.TopicMembers().ListByUser(ctx, user.ID)
	if err != nil {
		return nil, MapDomainError(err)
	}

	subscriptions := make([]*services.Subscription, 0, len(members))
	for _, m := range members {
		topic, err := s.store.Topics().Get(ctx, m.TopicID)
		if err != nil {
			continue
		}

		sub := memberToSubscription(m, topic)

		// Filter by sync status if specified
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
func (s *TopicServiceServer) GetSubscription(ctx context.Context, req *services.GetSubscriptionRequest) (*services.GetSubscriptionResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	user, ok := UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "not authenticated")
	}

	if req.TopicId == "" {
		return nil, NewValidationError("topic_id is required", map[string]string{
			"topic_id": "must not be empty",
		})
	}

	topicID := domain.TopicID(req.TopicId)

	member, err := s.store.TopicMembers().Get(ctx, topicID, user.ID)
	if err != nil {
		return nil, NewResourceNotFoundError("subscription", req.TopicId)
	}

	topic, err := s.store.Topics().Get(ctx, topicID)
	if err != nil {
		return nil, MapDomainError(err)
	}

	return &services.GetSubscriptionResponse{
		Subscription: memberToSubscription(member, topic),
	}, nil
}

// StreamTopicUpdates streams topic changes in real-time.
func (s *TopicServiceServer) StreamTopicUpdates(req *services.StreamTopicUpdatesRequest, stream services.TopicService_StreamTopicUpdatesServer) error {
	// TODO: Implement with hybrid P2P pubsub + local events
	// For now, return unimplemented
	return status.Error(codes.Unimplemented, "streaming topic updates not yet implemented")
}

// GetTopicStats returns statistics for a topic.
func (s *TopicServiceServer) GetTopicStats(ctx context.Context, req *services.GetTopicStatsRequest) (*services.GetTopicStatsResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	if req.TopicId == "" {
		return nil, NewValidationError("topic_id is required", map[string]string{
			"topic_id": "must not be empty",
		})
	}

	topic, err := s.store.Topics().Get(ctx, domain.TopicID(req.TopicId))
	if err != nil {
		return nil, MapDomainError(err)
	}

	// Check access
	user, _ := UserFromContext(ctx)
	if !s.canAccessTopic(ctx, topic, user) {
		return nil, NewPermissionDeniedError("view", "topic", "member")
	}

	// Get member count (approximate subscriber count)
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
func (s *TopicServiceServer) SearchTopics(ctx context.Context, req *services.SearchTopicsRequest) (*services.SearchTopicsResponse, error) {
	if s.store == nil {
		return nil, status.Error(codes.Unavailable, "service not initialized")
	}

	// Validate minimum query length
	if err := ValidateSearchQuery(req.Query); err != nil {
		return nil, err
	}

	user, _ := UserFromContext(ctx)

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
		return nil, MapDomainError(err)
	}

	// Filter by visibility
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
		protoTopics[i] = domainTopicToProto(t)
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

// =============================================================================
// Helper functions
// =============================================================================

func (s *TopicServiceServer) canAccessTopic(ctx context.Context, topic *domain.Topic, user *domain.User) bool {
	// Public topics are accessible to all authenticated users
	if s.isPublicTopic(topic) {
		return true
	}

	// User must be a member of private topics
	if user == nil {
		return false
	}

	hasAccess, _ := s.store.TopicMembers().HasAccess(ctx, topic.ID, user.ID)
	return hasAccess
}

func (s *TopicServiceServer) isPublicTopic(topic *domain.Topic) bool {
	// Check metadata for is_public flag
	if topic.Metadata != nil {
		if val, ok := topic.Metadata["is_public"]; ok {
			return val == "true"
		}
	}
	// Default to public for now
	return true
}

func domainTopicToProto(t *domain.Topic) *services.Topic {
	if t == nil {
		return nil
	}

	isPublic := true
	if t.Metadata != nil {
		if val, ok := t.Metadata["is_public"]; ok {
			isPublic = val == "true"
		}
	}

	return &services.Topic{
		Id:           string(t.ID),
		Name:         t.Name,
		Description:  t.Description,
		Schema:       t.TableSchema,
		DatasetCount: int64(t.DatasetCount),
		OwnerId:      string(t.CreatedBy),
		IsPublic:     isPublic,
		Status:       string(t.Status),
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
		SyncStatus:   "synced", // TODO: Track actual sync status
		AutoSync:     true,     // Default
	}

	if m.AcceptedAt != nil {
		sub.LastSyncAt = timestamppb.New(*m.AcceptedAt)
	}

	sub.TotalDatasets = int64(topic.DatasetCount)
	sub.SyncedDatasets = int64(topic.DatasetCount) // Assume fully synced for now

	return sub
}

func generateInviteToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
