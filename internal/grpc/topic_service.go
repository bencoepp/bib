// Package grpc provides gRPC service implementations for the bib daemon.
package grpc

import (
	"context"

	services "bib/api/gen/go/bib/v1/services"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TopicServiceServer implements the TopicService gRPC service.
type TopicServiceServer struct {
	services.UnimplementedTopicServiceServer
}

// NewTopicServiceServer creates a new TopicServiceServer.
func NewTopicServiceServer() *TopicServiceServer {
	return &TopicServiceServer{}
}

// CreateTopic creates a new topic.
func (s *TopicServiceServer) CreateTopic(ctx context.Context, req *services.CreateTopicRequest) (*services.CreateTopicResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateTopic not implemented")
}

// GetTopic retrieves a topic by ID or name.
func (s *TopicServiceServer) GetTopic(ctx context.Context, req *services.GetTopicRequest) (*services.GetTopicResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetTopic not implemented")
}

// ListTopics lists topics with filtering.
func (s *TopicServiceServer) ListTopics(ctx context.Context, req *services.ListTopicsRequest) (*services.ListTopicsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListTopics not implemented")
}

// UpdateTopic updates topic metadata.
func (s *TopicServiceServer) UpdateTopic(ctx context.Context, req *services.UpdateTopicRequest) (*services.UpdateTopicResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateTopic not implemented")
}

// DeleteTopic soft-deletes a topic.
func (s *TopicServiceServer) DeleteTopic(ctx context.Context, req *services.DeleteTopicRequest) (*services.DeleteTopicResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteTopic not implemented")
}

// Subscribe subscribes to a topic.
func (s *TopicServiceServer) Subscribe(ctx context.Context, req *services.SubscribeRequest) (*services.SubscribeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Subscribe not implemented")
}

// Unsubscribe unsubscribes from a topic.
func (s *TopicServiceServer) Unsubscribe(ctx context.Context, req *services.UnsubscribeRequest) (*services.UnsubscribeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Unsubscribe not implemented")
}

// ListSubscriptions lists current subscriptions.
func (s *TopicServiceServer) ListSubscriptions(ctx context.Context, req *services.ListSubscriptionsRequest) (*services.ListSubscriptionsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListSubscriptions not implemented")
}

// GetSubscription gets subscription details for a topic.
func (s *TopicServiceServer) GetSubscription(ctx context.Context, req *services.GetSubscriptionRequest) (*services.GetSubscriptionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetSubscription not implemented")
}

// StreamTopicUpdates streams topic changes in real-time.
func (s *TopicServiceServer) StreamTopicUpdates(req *services.StreamTopicUpdatesRequest, stream services.TopicService_StreamTopicUpdatesServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamTopicUpdates not implemented")
}

// GetTopicStats returns statistics for a topic.
func (s *TopicServiceServer) GetTopicStats(ctx context.Context, req *services.GetTopicStatsRequest) (*services.GetTopicStatsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetTopicStats not implemented")
}

// SearchTopics searches topics by text query.
func (s *TopicServiceServer) SearchTopics(ctx context.Context, req *services.SearchTopicsRequest) (*services.SearchTopicsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SearchTopics not implemented")
}
