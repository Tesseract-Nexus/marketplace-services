package subscribers

import (
	"context"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	gosharedevents "github.com/Tesseract-Nexus/go-shared/events"
	"products-service/internal/models"
	"products-service/internal/repository"
)

// ApprovalSubscriber handles incoming approval events for products
type ApprovalSubscriber struct {
	subscriber *gosharedevents.Subscriber
	repo       *repository.ProductsRepository
	logger     *logrus.Entry
	cancel     context.CancelFunc
}

// NewApprovalSubscriber creates a new approval event subscriber for products
func NewApprovalSubscriber(
	repo *repository.ProductsRepository,
	logger *logrus.Logger,
) (*ApprovalSubscriber, error) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats.nats.svc.cluster.local:4222"
	}

	config := gosharedevents.DefaultSubscriberConfig(natsURL, "products-service-approvals")
	config.Name = "products-service-approval-subscriber"
	config.DeliverPolicy = "new"
	config.MaxDeliver = 3
	config.AckWait = 30 * time.Second

	subscriber, err := gosharedevents.NewSubscriber(config, logger)
	if err != nil {
		return nil, err
	}

	return &ApprovalSubscriber{
		subscriber: subscriber,
		repo:       repo,
		logger:     logger.WithField("component", "approval-subscriber"),
	}, nil
}

// Start starts listening for approval events
func (s *ApprovalSubscriber) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	// Subscribe to approval.granted events
	subjects := []string{gosharedevents.ApprovalGranted}

	s.logger.Info("Starting product approval event subscription...")

	err := s.subscriber.SubscribeApprovalEvents(ctx, subjects, s.handleApprovalEvent)
	if err != nil {
		return err
	}

	s.logger.WithField("subjects", subjects).Info("Product approval subscriber started successfully")
	return nil
}

// handleApprovalEvent processes approval events for products
func (s *ApprovalSubscriber) handleApprovalEvent(ctx context.Context, event *gosharedevents.ApprovalEvent) error {
	s.logger.WithFields(logrus.Fields{
		"event_type":    event.EventType,
		"approval_id":   event.ApprovalRequestID,
		"action_type":   event.ActionType,
		"resource_type": event.ResourceType,
		"status":        event.Status,
	}).Info("Received approval event")

	// Only process events for product resources
	if event.ResourceType != "product" {
		s.logger.WithField("resource_type", event.ResourceType).Debug("Ignoring non-product approval event")
		return nil
	}

	// Only process granted events
	if event.Status != "approved" {
		s.logger.WithField("status", event.Status).Debug("Ignoring non-approved event")
		return nil
	}

	tenantID := event.TenantID

	// Execute the approved action based on action type
	switch event.ActionType {
	case "product_creation":
		return s.executeProductPublish(ctx, event, tenantID)
	case "product_update":
		return s.executeProductUpdate(ctx, event, tenantID)
	default:
		s.logger.WithField("action_type", event.ActionType).Debug("Ignoring unhandled action type for products")
		return nil
	}
}

// executeProductPublish publishes an approved product (DRAFT -> ACTIVE)
func (s *ApprovalSubscriber) executeProductPublish(ctx context.Context, event *gosharedevents.ApprovalEvent, tenantID string) error {
	s.logger.WithFields(logrus.Fields{
		"resource_id": event.ResourceID,
		"approver":    event.ApproverName,
	}).Info("Executing approved product publish")

	// Parse product ID
	productID, err := uuid.Parse(event.ResourceID)
	if err != nil {
		s.logger.WithError(err).Error("Invalid product ID in approval event")
		return nil // Don't retry for invalid IDs
	}

	// Update product status to ACTIVE
	if err := s.repo.UpdateProductStatus(tenantID, productID, models.ProductStatusActive, nil); err != nil {
		s.logger.WithError(err).Error("Failed to update product status to ACTIVE")
		return err
	}

	s.logger.WithFields(logrus.Fields{
		"product_id": productID,
		"new_status": models.ProductStatusActive,
	}).Info("Product published successfully after approval")
	return nil
}

// executeProductUpdate applies an approved product update
func (s *ApprovalSubscriber) executeProductUpdate(ctx context.Context, event *gosharedevents.ApprovalEvent, tenantID string) error {
	s.logger.WithFields(logrus.Fields{
		"resource_id": event.ResourceID,
		"approver":    event.ApproverName,
	}).Info("Executing approved product update")

	// Parse product ID
	productID, err := uuid.Parse(event.ResourceID)
	if err != nil {
		s.logger.WithError(err).Error("Invalid product ID in approval event")
		return nil // Don't retry for invalid IDs
	}

	// For product updates, the changes are already applied but in a pending state
	// Mark the product as ACTIVE to confirm the update
	if err := s.repo.UpdateProductStatus(tenantID, productID, models.ProductStatusActive, nil); err != nil {
		s.logger.WithError(err).Error("Failed to confirm product update")
		return err
	}

	s.logger.WithFields(logrus.Fields{
		"product_id": productID,
	}).Info("Product update confirmed after approval")
	return nil
}

// Stop stops the approval subscriber
func (s *ApprovalSubscriber) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.subscriber != nil {
		s.subscriber.Close()
	}
	s.logger.Info("Product approval subscriber stopped")
}
