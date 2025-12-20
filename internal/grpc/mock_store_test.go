package grpc

import (
	"sync"
	"time"

	"bib/internal/domain"
	"bib/internal/storage"
)

// TestStore is a minimal in-memory store for testing gRPC services.
// It implements just enough of the storage.Store interface for unit tests.
// For full integration tests, use the actual storage implementations.
type TestStore struct {
	mu sync.RWMutex

	Topics           map[domain.TopicID]*domain.Topic
	TopicsByName     map[string]*domain.Topic
	Datasets         map[domain.DatasetID]*domain.Dataset
	Users            map[domain.UserID]*domain.User
	UsersByKey       map[string]*domain.User
	Sessions         map[string]*storage.Session
	TopicMemberships map[string]*storage.TopicMember // "topicID:userID" -> member

	// Control error injection for testing
	ForceError error
}

// NewTestStore creates a new test store.
func NewTestStore() *TestStore {
	return &TestStore{
		Topics:           make(map[domain.TopicID]*domain.Topic),
		TopicsByName:     make(map[string]*domain.Topic),
		Datasets:         make(map[domain.DatasetID]*domain.Dataset),
		Users:            make(map[domain.UserID]*domain.User),
		UsersByKey:       make(map[string]*domain.User),
		Sessions:         make(map[string]*storage.Session),
		TopicMemberships: make(map[string]*storage.TopicMember),
	}
}

// AddTopic adds a topic to the test store.
func (s *TestStore) AddTopic(topic *domain.Topic) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Topics[topic.ID] = topic
	s.TopicsByName[topic.Name] = topic
}

// AddDataset adds a dataset to the test store.
func (s *TestStore) AddDataset(dataset *domain.Dataset) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Datasets[dataset.ID] = dataset
}

// AddUser adds a user to the test store.
func (s *TestStore) AddUser(user *domain.User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Users[user.ID] = user
	if user.PublicKeyFingerprint != "" {
		s.UsersByKey[user.PublicKeyFingerprint] = user
	}
}

// AddSession adds a session to the test store.
func (s *TestStore) AddSession(session *storage.Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Sessions[session.ID] = session
}

// AddTopicMembership adds a topic membership to the test store.
func (s *TestStore) AddTopicMembership(member *storage.TopicMember) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := string(member.TopicID) + ":" + string(member.UserID)
	s.TopicMemberships[key] = member
}

// GetTopic retrieves a topic by ID.
func (s *TestStore) GetTopic(id domain.TopicID) (*domain.Topic, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.ForceError != nil {
		return nil, s.ForceError
	}
	topic, ok := s.Topics[id]
	if !ok {
		return nil, domain.ErrTopicNotFound
	}
	return topic, nil
}

// GetTopicByName retrieves a topic by name.
func (s *TestStore) GetTopicByName(name string) (*domain.Topic, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.ForceError != nil {
		return nil, s.ForceError
	}
	topic, ok := s.TopicsByName[name]
	if !ok {
		return nil, domain.ErrTopicNotFound
	}
	return topic, nil
}

// GetDataset retrieves a dataset by ID.
func (s *TestStore) GetDataset(id domain.DatasetID) (*domain.Dataset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.ForceError != nil {
		return nil, s.ForceError
	}
	dataset, ok := s.Datasets[id]
	if !ok {
		return nil, domain.ErrDatasetNotFound
	}
	return dataset, nil
}

// GetUser retrieves a user by ID.
func (s *TestStore) GetUser(id domain.UserID) (*domain.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.ForceError != nil {
		return nil, s.ForceError
	}
	user, ok := s.Users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return user, nil
}

// GetSession retrieves a session by ID.
func (s *TestStore) GetSession(id string) (*storage.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.ForceError != nil {
		return nil, s.ForceError
	}
	session, ok := s.Sessions[id]
	if !ok {
		return nil, domain.ErrSessionNotFound
	}
	return session, nil
}

// GetTopicMembership retrieves a topic membership.
func (s *TestStore) GetTopicMembership(topicID domain.TopicID, userID domain.UserID) (*storage.TopicMember, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := string(topicID) + ":" + string(userID)
	member, ok := s.TopicMemberships[key]
	if !ok {
		return nil, domain.ErrOwnerNotFound
	}
	return member, nil
}

// ListTopics returns all topics.
func (s *TestStore) ListTopics() []*domain.Topic {
	s.mu.RLock()
	defer s.mu.RUnlock()
	topics := make([]*domain.Topic, 0, len(s.Topics))
	for _, t := range s.Topics {
		topics = append(topics, t)
	}
	return topics
}

// ListDatasets returns all datasets.
func (s *TestStore) ListDatasets() []*domain.Dataset {
	s.mu.RLock()
	defer s.mu.RUnlock()
	datasets := make([]*domain.Dataset, 0, len(s.Datasets))
	for _, d := range s.Datasets {
		datasets = append(datasets, d)
	}
	return datasets
}

// ListSessions returns all sessions for a user.
func (s *TestStore) ListSessions(userID domain.UserID) []*storage.Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sessions := make([]*storage.Session, 0)
	for _, sess := range s.Sessions {
		if sess.UserID == userID {
			sessions = append(sessions, sess)
		}
	}
	return sessions
}

// EndSession marks a session as ended.
func (s *TestStore) EndSession(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.Sessions[id]
	if !ok {
		return domain.ErrSessionNotFound
	}
	now := time.Now()
	session.EndedAt = &now
	return nil
}
