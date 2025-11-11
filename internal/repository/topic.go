package repository

import "bib/internal/domain"

type TopicRepository interface {
	GetTopicByID(id string) (*domain.Topic, error)
	CreateTopic(topic *domain.Topic) error
	UpdateTopic(topic *domain.Topic) error
	DeleteTopic(id string) error
}
