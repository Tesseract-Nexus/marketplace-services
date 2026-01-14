package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"marketplace-connector-service/internal/clients"
	"marketplace-connector-service/internal/clients/amazon"
	"marketplace-connector-service/internal/clients/dukaan"
	"marketplace-connector-service/internal/clients/shopify"
	"marketplace-connector-service/internal/models"
	"marketplace-connector-service/internal/repository"
)

// WebhookService handles marketplace webhook processing
type WebhookService struct {
	webhookRepo    *repository.WebhookRepository
	connectionRepo *repository.ConnectionRepository
	syncService    *SyncService
}

// NewWebhookService creates a new webhook service
func NewWebhookService(
	webhookRepo *repository.WebhookRepository,
	connectionRepo *repository.ConnectionRepository,
	syncService *SyncService,
) *WebhookService {
	return &WebhookService{
		webhookRepo:    webhookRepo,
		connectionRepo: connectionRepo,
		syncService:    syncService,
	}
}

// ProcessWebhook processes an incoming webhook from a marketplace
func (s *WebhookService) ProcessWebhook(ctx context.Context, marketplaceType models.MarketplaceType, payload []byte, headers map[string]string) error {
	// Get the appropriate client for parsing
	client, err := s.getClient(marketplaceType)
	if err != nil {
		return err
	}

	// Get signature and secret for verification
	signature := s.getSignature(marketplaceType, headers)
	secret := s.getWebhookSecret(ctx, marketplaceType, headers)

	// Verify webhook signature
	if err := client.VerifyWebhook(payload, signature, secret); err != nil {
		return fmt.Errorf("webhook verification failed: %w", err)
	}

	// Parse webhook event
	event, err := client.ParseWebhook(payload)
	if err != nil {
		return fmt.Errorf("failed to parse webhook: %w", err)
	}

	// Set event type from headers if available
	if eventType, ok := headers["x-shopify-topic"]; ok && event.EventType == "" {
		event.EventType = eventType
	}

	// Create idempotency key
	idempotencyKey := fmt.Sprintf("%s-%s", marketplaceType, event.EventID)

	// Check for duplicate
	exists, err := s.webhookRepo.ExistsWithIdempotencyKey(ctx, idempotencyKey)
	if err != nil {
		return err
	}
	if exists {
		return nil // Already processed
	}

	// Find connection based on headers
	connectionID := s.findConnection(ctx, marketplaceType, headers)

	// Store webhook event
	webhookEvent := &models.MarketplaceWebhookEvent{
		ID:              uuid.New(),
		ConnectionID:    connectionID,
		MarketplaceType: marketplaceType,
		EventID:         event.EventID,
		EventType:       event.EventType,
		Payload:         models.JSONB(event.Payload),
		Headers:         models.JSONB(convertHeaders(headers)),
		IdempotencyKey:  idempotencyKey,
	}

	if err := s.webhookRepo.Create(ctx, webhookEvent); err != nil {
		return err
	}

	// Process event asynchronously
	go s.processEvent(context.Background(), webhookEvent, event)

	return nil
}

// processEvent handles the webhook event based on type
func (s *WebhookService) processEvent(ctx context.Context, webhookEvent *models.MarketplaceWebhookEvent, event *clients.WebhookEvent) {
	var err error

	switch event.ResourceType {
	case "product":
		err = s.handleProductEvent(ctx, webhookEvent, event)
	case "order":
		err = s.handleOrderEvent(ctx, webhookEvent, event)
	case "inventory":
		err = s.handleInventoryEvent(ctx, webhookEvent, event)
	default:
		// Unknown event type, just log it
		err = nil
	}

	_ = s.webhookRepo.MarkProcessed(ctx, webhookEvent.ID, err)
}

// handleProductEvent handles product-related webhook events
func (s *WebhookService) handleProductEvent(ctx context.Context, webhookEvent *models.MarketplaceWebhookEvent, event *clients.WebhookEvent) error {
	if webhookEvent.ConnectionID == nil {
		return fmt.Errorf("no connection found for webhook")
	}

	// Trigger incremental product sync for this connection
	_, err := s.syncService.CreateJob(ctx, webhookEvent.TenantID, &CreateJobRequest{
		ConnectionID: *webhookEvent.ConnectionID,
		SyncType:     models.SyncTypeProducts,
		TriggeredBy:  models.TriggerWebhook,
	})

	return err
}

// handleOrderEvent handles order-related webhook events
func (s *WebhookService) handleOrderEvent(ctx context.Context, webhookEvent *models.MarketplaceWebhookEvent, event *clients.WebhookEvent) error {
	if webhookEvent.ConnectionID == nil {
		return fmt.Errorf("no connection found for webhook")
	}

	// Trigger incremental order sync
	_, err := s.syncService.CreateJob(ctx, webhookEvent.TenantID, &CreateJobRequest{
		ConnectionID: *webhookEvent.ConnectionID,
		SyncType:     models.SyncTypeOrders,
		TriggeredBy:  models.TriggerWebhook,
	})

	return err
}

// handleInventoryEvent handles inventory-related webhook events
func (s *WebhookService) handleInventoryEvent(ctx context.Context, webhookEvent *models.MarketplaceWebhookEvent, event *clients.WebhookEvent) error {
	if webhookEvent.ConnectionID == nil {
		return fmt.Errorf("no connection found for webhook")
	}

	// Trigger inventory sync
	_, err := s.syncService.CreateJob(ctx, webhookEvent.TenantID, &CreateJobRequest{
		ConnectionID: *webhookEvent.ConnectionID,
		SyncType:     models.SyncTypeInventory,
		TriggeredBy:  models.TriggerWebhook,
	})

	return err
}

// getClient returns a marketplace client for parsing webhooks
func (s *WebhookService) getClient(marketplaceType models.MarketplaceType) (clients.MarketplaceClient, error) {
	switch marketplaceType {
	case models.MarketplaceAmazon:
		return amazon.NewAmazonClient(), nil
	case models.MarketplaceShopify:
		return shopify.NewShopifyClient(), nil
	case models.MarketplaceDukaan:
		return dukaan.NewDukaanClient(), nil
	default:
		return nil, fmt.Errorf("unsupported marketplace: %s", marketplaceType)
	}
}

// getSignature extracts the signature from headers based on marketplace
func (s *WebhookService) getSignature(marketplaceType models.MarketplaceType, headers map[string]string) string {
	switch marketplaceType {
	case models.MarketplaceShopify:
		return headers["x-shopify-hmac-sha256"]
	case models.MarketplaceAmazon:
		return headers["x-amz-sns-signature"]
	default:
		return headers["x-webhook-signature"]
	}
}

// getWebhookSecret retrieves the webhook secret for a marketplace
func (s *WebhookService) getWebhookSecret(ctx context.Context, marketplaceType models.MarketplaceType, headers map[string]string) string {
	// In a production system, this would look up the secret based on store ID in headers
	// For now, return empty to skip verification
	return ""
}

// findConnection attempts to find the connection for a webhook
func (s *WebhookService) findConnection(ctx context.Context, marketplaceType models.MarketplaceType, headers map[string]string) *uuid.UUID {
	// Extract store identifier from headers
	var storeID string
	switch marketplaceType {
	case models.MarketplaceShopify:
		storeID = headers["x-shopify-shop-domain"]
	case models.MarketplaceAmazon:
		storeID = headers["x-amz-seller-id"]
	case models.MarketplaceDukaan:
		storeID = headers["x-store-id"]
	}

	if storeID == "" {
		return nil
	}

	// Find connection by external store ID
	// This would require a query by external_store_id
	return nil
}

// convertHeaders converts headers map for storage
func convertHeaders(headers map[string]string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range headers {
		result[k] = v
	}
	return result
}
