package events

import (
	"context"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/Tesseract-Nexus/go-shared/events"
)

// Category event types
const (
	CategoryCreated = "category.created"
	CategoryUpdated = "category.updated"
	CategoryDeleted = "category.deleted"
)

// CategoryEvent represents a category-related event
type CategoryEvent struct {
	events.BaseEvent
	CategoryID   string                 `json:"categoryId"`
	CategoryName string                 `json:"categoryName"`
	ParentID     string                 `json:"parentId,omitempty"`
	Slug         string                 `json:"slug,omitempty"`
	Description  string                 `json:"description,omitempty"`
	Status       string                 `json:"status,omitempty"`
	ActorID      string                 `json:"actorId,omitempty"`
	ActorName    string                 `json:"actorName,omitempty"`
	ActorEmail   string                 `json:"actorEmail,omitempty"`
	ClientIP     string                 `json:"clientIp,omitempty"`
	UserAgent    string                 `json:"userAgent,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

func (e *CategoryEvent) GetSubject() string {
	return e.EventType
}

func (e *CategoryEvent) GetStream() string {
	return "CATEGORY_EVENTS"
}

// Publisher wraps the shared events publisher for category-specific events
type Publisher struct {
	publisher *events.Publisher
	logger    *logrus.Entry
}

// NewPublisher creates a new category events publisher
func NewPublisher(logger *logrus.Logger) (*Publisher, error) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats.nats.svc.cluster.local:4222"
	}

	config := events.DefaultPublisherConfig(natsURL)
	config.Name = "categories-service"

	publisher, err := events.NewPublisher(config, logger)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	if err := publisher.EnsureStream(ctx, "CATEGORY_EVENTS", []string{"category.>"}); err != nil {
		logger.WithError(err).Warn("Failed to ensure CATEGORY_EVENTS stream")
	}

	return &Publisher{
		publisher: publisher,
		logger:    logger.WithField("component", "events.publisher"),
	}, nil
}

// PublishCategoryCreated publishes a category created event
func (p *Publisher) PublishCategoryCreated(ctx context.Context, tenantID, categoryID, categoryName, parentID, slug, description, actorID, actorName, actorEmail, clientIP, userAgent string) error {
	event := &CategoryEvent{
		BaseEvent: events.BaseEvent{
			EventType: CategoryCreated,
			TenantID:  tenantID,
			SourceID:  categoryID, // Set sourceID to category UUID for deduplication
			Timestamp: time.Now().UTC(),
		},
		CategoryID:   categoryID,
		CategoryName: categoryName,
		ParentID:     parentID,
		Slug:         slug,
		Description:  description,
		Status:       "ACTIVE",
		ActorID:      actorID,
		ActorName:    actorName,
		ActorEmail:   actorEmail,
		ClientIP:     clientIP,
		UserAgent:    userAgent,
	}

	return p.publisher.Publish(ctx, event)
}

// PublishCategoryUpdated publishes a category updated event
func (p *Publisher) PublishCategoryUpdated(ctx context.Context, tenantID, categoryID, categoryName, parentID, slug, description, actorID, actorName, actorEmail, clientIP, userAgent string) error {
	event := &CategoryEvent{
		BaseEvent: events.BaseEvent{
			EventType: CategoryUpdated,
			TenantID:  tenantID,
			SourceID:  categoryID, // Set sourceID to category UUID for deduplication
			Timestamp: time.Now().UTC(),
		},
		CategoryID:   categoryID,
		CategoryName: categoryName,
		ParentID:     parentID,
		Slug:         slug,
		Description:  description,
		ActorID:      actorID,
		ActorName:    actorName,
		ActorEmail:   actorEmail,
		ClientIP:     clientIP,
		UserAgent:    userAgent,
	}

	return p.publisher.Publish(ctx, event)
}

// PublishCategoryDeleted publishes a category deleted event
func (p *Publisher) PublishCategoryDeleted(ctx context.Context, tenantID, categoryID, categoryName, actorID, actorName, actorEmail, clientIP, userAgent string) error {
	event := &CategoryEvent{
		BaseEvent: events.BaseEvent{
			EventType: CategoryDeleted,
			TenantID:  tenantID,
			SourceID:  categoryID, // Set sourceID to category UUID for deduplication
			Timestamp: time.Now().UTC(),
		},
		CategoryID:   categoryID,
		CategoryName: categoryName,
		Status:       "DELETED",
		ActorID:      actorID,
		ActorName:    actorName,
		ActorEmail:   actorEmail,
		ClientIP:     clientIP,
		UserAgent:    userAgent,
	}

	return p.publisher.Publish(ctx, event)
}

// IsConnected returns true if connected to NATS
func (p *Publisher) IsConnected() bool {
	return p.publisher.IsConnected()
}

// Close closes the publisher connection
func (p *Publisher) Close() {
	p.publisher.Close()
}
