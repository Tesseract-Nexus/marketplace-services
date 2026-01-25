package subscribers

import (
	"context"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	gosharedevents "github.com/Tesseract-Nexus/go-shared/events"
	"categories-service/internal/models"
	"categories-service/internal/repository"
)

// ApprovalSubscriber handles incoming approval events for categories
type ApprovalSubscriber struct {
	subscriber *gosharedevents.Subscriber
	repo       *repository.CategoryRepository
	logger     *logrus.Entry
	cancel     context.CancelFunc
}

// NewApprovalSubscriber creates a new approval event subscriber for categories
func NewApprovalSubscriber(
	repo *repository.CategoryRepository,
	logger *logrus.Logger,
) (*ApprovalSubscriber, error) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats.nats.svc.cluster.local:4222"
	}

	config := gosharedevents.DefaultSubscriberConfig(natsURL, "categories-service-approvals")
	config.Name = "categories-service-approval-subscriber"
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

	s.logger.Info("Starting category approval event subscription...")

	err := s.subscriber.SubscribeApprovalEvents(ctx, subjects, s.handleApprovalEvent)
	if err != nil {
		return err
	}

	s.logger.WithField("subjects", subjects).Info("Category approval subscriber started successfully")
	return nil
}

// handleApprovalEvent processes approval events for categories
func (s *ApprovalSubscriber) handleApprovalEvent(ctx context.Context, event *gosharedevents.ApprovalEvent) error {
	s.logger.WithFields(logrus.Fields{
		"event_type":    event.EventType,
		"approval_id":   event.ApprovalRequestID,
		"action_type":   event.ActionType,
		"resource_type": event.ResourceType,
		"status":        event.Status,
	}).Info("Received approval event")

	// Only process events for category resources
	if event.ResourceType != "category" {
		s.logger.WithField("resource_type", event.ResourceType).Debug("Ignoring non-category approval event")
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
	case "category_creation":
		return s.executeCategoryApprove(ctx, event, tenantID)
	case "category_update":
		return s.executeCategoryUpdateApprove(ctx, event, tenantID)
	default:
		s.logger.WithField("action_type", event.ActionType).Debug("Ignoring unhandled action type for categories")
		return nil
	}
}

// executeCategoryApprove approves a category (DRAFT -> APPROVED)
func (s *ApprovalSubscriber) executeCategoryApprove(ctx context.Context, event *gosharedevents.ApprovalEvent, tenantID string) error {
	s.logger.WithFields(logrus.Fields{
		"resource_id": event.ResourceID,
		"approver":    event.ApproverName,
	}).Info("Executing approved category publish")

	categoryID := event.ResourceID

	// Update category status to APPROVED
	if err := s.repo.UpdateStatus(tenantID, categoryID, models.StatusApproved); err != nil {
		s.logger.WithError(err).Error("Failed to update category status to APPROVED")
		return err
	}

	s.logger.WithFields(logrus.Fields{
		"category_id": categoryID,
		"new_status":  models.StatusApproved,
	}).Info("Category approved successfully after approval")
	return nil
}

// executeCategoryUpdateApprove applies an approved category update
func (s *ApprovalSubscriber) executeCategoryUpdateApprove(ctx context.Context, event *gosharedevents.ApprovalEvent, tenantID string) error {
	s.logger.WithFields(logrus.Fields{
		"resource_id": event.ResourceID,
		"approver":    event.ApproverName,
	}).Info("Executing approved category update")

	categoryID := event.ResourceID

	// For category updates, mark the category as APPROVED to confirm the update
	if err := s.repo.UpdateStatus(tenantID, categoryID, models.StatusApproved); err != nil {
		s.logger.WithError(err).Error("Failed to confirm category update")
		return err
	}

	s.logger.WithFields(logrus.Fields{
		"category_id": categoryID,
	}).Info("Category update confirmed after approval")
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
	s.logger.Info("Category approval subscriber stopped")
}
