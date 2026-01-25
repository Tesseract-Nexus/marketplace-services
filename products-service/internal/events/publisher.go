package events

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/Tesseract-Nexus/go-shared/events"
	"products-service/internal/models"
)

// Publisher wraps the go-shared events publisher for product-specific events
type Publisher struct {
	publisher *events.Publisher
	logger    *logrus.Entry
}

// NewPublisher creates a new product events publisher
func NewPublisher(logger *logrus.Logger) (*Publisher, error) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		// Default to GKE internal NATS service URL
		natsURL = "nats://nats.nats.svc.cluster.local:4222"
	}

	config := events.DefaultPublisherConfig(natsURL)
	config.Name = "products-service"

	publisher, err := events.NewPublisher(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create events publisher: %w", err)
	}

	// Ensure the products stream exists
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := publisher.EnsureStream(ctx, events.StreamProducts, []string{"product.>"}); err != nil {
		logger.WithError(err).Warn("Failed to ensure products stream (may already exist)")
	}

	return &Publisher{
		publisher: publisher,
		logger:    logger.WithField("component", "products-events"),
	}, nil
}

// Close closes the NATS connection
func (p *Publisher) Close() {
	if p.publisher != nil {
		p.publisher.Close()
	}
}

// PublishProductCreated publishes a product.created event
func (p *Publisher) PublishProductCreated(ctx context.Context, product *models.Product, tenantID, actorID, actorName, actorEmail, clientIP, userAgent string) error {
	event := p.buildProductEvent(events.ProductCreated, product, tenantID)
	event.ActorID = actorID
	event.ActorName = actorName
	event.ActorEmail = actorEmail
	event.ClientIP = clientIP
	event.UserAgent = userAgent
	event.ChangeType = "created"
	return p.publish(ctx, event)
}

// PublishProductUpdated publishes a product.updated event
func (p *Publisher) PublishProductUpdated(ctx context.Context, product *models.Product, oldProduct *models.Product, changedFields []string, tenantID, actorID, actorName, actorEmail, clientIP, userAgent string) error {
	event := p.buildProductEvent(events.ProductUpdated, product, tenantID)
	event.ActorID = actorID
	event.ActorName = actorName
	event.ActorEmail = actorEmail
	event.ClientIP = clientIP
	event.UserAgent = userAgent
	event.ChangeType = "updated"
	event.ChangedFields = changedFields

	// Capture old values
	if oldProduct != nil {
		oldDesc := ""
		if oldProduct.Description != nil {
			oldDesc = *oldProduct.Description
		}
		event.OldValue = map[string]interface{}{
			"name":        oldProduct.Name,
			"description": oldDesc,
			"price":       oldProduct.Price,
			"status":      oldProduct.Status,
		}
	}

	// Capture new values
	newDesc := ""
	if product.Description != nil {
		newDesc = *product.Description
	}
	event.NewValue = map[string]interface{}{
		"name":        product.Name,
		"description": newDesc,
		"price":       product.Price,
		"status":      product.Status,
	}

	return p.publish(ctx, event)
}

// PublishProductDeleted publishes a product.deleted event
func (p *Publisher) PublishProductDeleted(ctx context.Context, product *models.Product, tenantID, actorID, actorName, actorEmail, clientIP, userAgent string) error {
	event := p.buildProductEvent(events.ProductDeleted, product, tenantID)
	event.ActorID = actorID
	event.ActorName = actorName
	event.ActorEmail = actorEmail
	event.ClientIP = clientIP
	event.UserAgent = userAgent
	event.ChangeType = "deleted"
	return p.publish(ctx, event)
}

// PublishProductPublished publishes a product.published event
func (p *Publisher) PublishProductPublished(ctx context.Context, product *models.Product, tenantID, actorID, actorName, actorEmail, clientIP, userAgent string) error {
	event := p.buildProductEvent(events.ProductPublished, product, tenantID)
	event.ActorID = actorID
	event.ActorName = actorName
	event.ActorEmail = actorEmail
	event.ClientIP = clientIP
	event.UserAgent = userAgent
	event.ChangeType = "published"
	return p.publish(ctx, event)
}

// PublishProductArchived publishes a product.archived event
func (p *Publisher) PublishProductArchived(ctx context.Context, product *models.Product, tenantID, actorID, actorName, actorEmail, clientIP, userAgent string) error {
	event := p.buildProductEvent(events.ProductArchived, product, tenantID)
	event.ActorID = actorID
	event.ActorName = actorName
	event.ActorEmail = actorEmail
	event.ClientIP = clientIP
	event.UserAgent = userAgent
	event.ChangeType = "archived"
	return p.publish(ctx, event)
}

// PublishProductStatusChanged publishes a product status change event
func (p *Publisher) PublishProductStatusChanged(ctx context.Context, product *models.Product, oldStatus, newStatus string, tenantID, actorID, actorName, actorEmail, clientIP, userAgent string) error {
	event := p.buildProductEvent("product.status_changed", product, tenantID)
	event.ActorID = actorID
	event.ActorName = actorName
	event.ActorEmail = actorEmail
	event.ClientIP = clientIP
	event.UserAgent = userAgent
	event.ChangeType = "status_changed"
	event.OldValue = map[string]interface{}{"status": oldStatus}
	event.NewValue = map[string]interface{}{"status": newStatus}
	event.ChangedFields = []string{"status"}
	return p.publish(ctx, event)
}

// PublishProductPriceChanged publishes a product price change event
func (p *Publisher) PublishProductPriceChanged(ctx context.Context, product *models.Product, oldPrice, newPrice float64, tenantID, actorID, actorName, actorEmail, clientIP, userAgent string) error {
	event := p.buildProductEvent("product.price_changed", product, tenantID)
	event.ActorID = actorID
	event.ActorName = actorName
	event.ActorEmail = actorEmail
	event.ClientIP = clientIP
	event.UserAgent = userAgent
	event.ChangeType = "price_changed"
	event.OldValue = map[string]interface{}{"price": oldPrice}
	event.NewValue = map[string]interface{}{"price": newPrice}
	event.ChangedFields = []string{"price"}
	return p.publish(ctx, event)
}

// buildProductEvent creates a ProductEvent from a product model
func (p *Publisher) buildProductEvent(eventType string, product *models.Product, tenantID string) *events.ProductEvent {
	event := events.NewProductEvent(eventType, tenantID)
	event.SourceID = uuid.New().String()
	event.ProductID = product.ID.String()
	event.ProductName = product.Name
	event.SKU = product.SKU
	event.Status = string(product.Status)

	// Parse price string to float64
	if price, err := parsePrice(product.Price); err == nil {
		event.Price = price
	}

	// Category info
	if product.CategoryID != "" {
		event.CategoryID = product.CategoryID
	}

	// Vendor info for multi-vendor scenarios
	if product.VendorID != "" {
		event.VendorID = product.VendorID
	}

	return event
}

// parsePrice converts a price string to float64
func parsePrice(priceStr string) (float64, error) {
	var price float64
	_, err := fmt.Sscanf(priceStr, "%f", &price)
	return price, err
}

// publish is a helper that logs and publishes events asynchronously
func (p *Publisher) publish(ctx context.Context, event *events.ProductEvent) error {
	// Publish asynchronously to not block the main flow
	go func() {
		pubCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := p.publisher.PublishProduct(pubCtx, event); err != nil {
			p.logger.WithFields(logrus.Fields{
				"eventType": event.EventType,
				"productID": event.ProductID,
				"tenantID":  event.TenantID,
			}).WithError(err).Error("Failed to publish product event")
		} else {
			p.logger.WithFields(logrus.Fields{
				"eventType":   event.EventType,
				"productID":   event.ProductID,
				"productName": event.ProductName,
				"tenantID":    event.TenantID,
			}).Info("Product event published successfully")
		}
	}()

	return nil
}
