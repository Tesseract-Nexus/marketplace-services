package repository

import (
	"context"

	"github.com/google/uuid"
	"marketplace-connector-service/internal/models"
	"gorm.io/gorm"
)

// WebhookRepository handles database operations for webhook events
type WebhookRepository struct {
	db *gorm.DB
}

// NewWebhookRepository creates a new webhook repository
func NewWebhookRepository(db *gorm.DB) *WebhookRepository {
	return &WebhookRepository{db: db}
}

// Create creates a new webhook event
func (r *WebhookRepository) Create(ctx context.Context, event *models.MarketplaceWebhookEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

// GetByID retrieves a webhook event by ID
func (r *WebhookRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.MarketplaceWebhookEvent, error) {
	var event models.MarketplaceWebhookEvent
	err := r.db.WithContext(ctx).First(&event, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

// GetByEventID retrieves a webhook event by marketplace event ID
func (r *WebhookRepository) GetByEventID(ctx context.Context, marketplaceType models.MarketplaceType, eventID string) (*models.MarketplaceWebhookEvent, error) {
	var event models.MarketplaceWebhookEvent
	err := r.db.WithContext(ctx).
		Where("marketplace_type = ? AND event_id = ?", marketplaceType, eventID).
		First(&event).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

// MarkProcessed marks a webhook event as processed
func (r *WebhookRepository) MarkProcessed(ctx context.Context, id uuid.UUID, err error) error {
	updates := map[string]interface{}{
		"processed":    true,
		"processed_at": gorm.Expr("CURRENT_TIMESTAMP"),
	}
	if err != nil {
		updates["processing_error"] = err.Error()
		updates["retry_count"] = gorm.Expr("retry_count + 1")
	}
	return r.db.WithContext(ctx).
		Model(&models.MarketplaceWebhookEvent{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// GetUnprocessedEvents retrieves unprocessed webhook events
func (r *WebhookRepository) GetUnprocessedEvents(ctx context.Context, limit int) ([]models.MarketplaceWebhookEvent, error) {
	var events []models.MarketplaceWebhookEvent
	err := r.db.WithContext(ctx).
		Where("processed = ? AND retry_count < ?", false, 3).
		Order("created_at ASC").
		Limit(limit).
		Find(&events).Error
	return events, err
}

// ExistsWithIdempotencyKey checks if an event with the given idempotency key exists
func (r *WebhookRepository) ExistsWithIdempotencyKey(ctx context.Context, key string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.MarketplaceWebhookEvent{}).
		Where("idempotency_key = ?", key).
		Count(&count).Error
	return count > 0, err
}
