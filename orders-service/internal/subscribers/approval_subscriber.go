package subscribers

import (
	"context"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	gosharedevents "github.com/Tesseract-Nexus/go-shared/events"
	"orders-service/internal/clients"
	"orders-service/internal/models"
)

// OrderExecutor defines the interface for executing order operations
// This avoids the import cycle with the services package
type OrderExecutor interface {
	RefundOrder(orderID uuid.UUID, amount *float64, reason string, tenantID string) (*models.Order, error)
	CancelOrder(orderID uuid.UUID, reason string, tenantID string) (*models.Order, error)
}

// ApprovalSubscriber handles incoming approval events
type ApprovalSubscriber struct {
	subscriber     *gosharedevents.Subscriber
	orderExecutor  OrderExecutor
	approvalClient *clients.ApprovalClient
	logger         *logrus.Entry
	cancel         context.CancelFunc
}

// NewApprovalSubscriber creates a new approval event subscriber
func NewApprovalSubscriber(
	orderExecutor OrderExecutor,
	approvalClient *clients.ApprovalClient,
	logger *logrus.Logger,
) (*ApprovalSubscriber, error) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats.nats.svc.cluster.local:4222"
	}

	config := gosharedevents.DefaultSubscriberConfig(natsURL, "orders-service-approvals")
	config.Name = "orders-service-approval-subscriber"
	config.DeliverPolicy = "new"
	config.MaxDeliver = 3
	config.AckWait = 30 * time.Second

	subscriber, err := gosharedevents.NewSubscriber(config, logger)
	if err != nil {
		return nil, err
	}

	return &ApprovalSubscriber{
		subscriber:     subscriber,
		orderExecutor:  orderExecutor,
		approvalClient: approvalClient,
		logger:         logger.WithField("component", "approval-subscriber"),
	}, nil
}

// Start starts listening for approval events
func (s *ApprovalSubscriber) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	// Subscribe to approval.granted events for orders
	subjects := []string{gosharedevents.ApprovalGranted}

	s.logger.Info("Starting approval event subscription...")

	err := s.subscriber.SubscribeApprovalEvents(ctx, subjects, s.handleApprovalEvent)
	if err != nil {
		return err
	}

	s.logger.WithField("subjects", subjects).Info("Approval subscriber started successfully")
	return nil
}

// handleApprovalEvent processes approval events
func (s *ApprovalSubscriber) handleApprovalEvent(ctx context.Context, event *gosharedevents.ApprovalEvent) error {
	s.logger.WithFields(logrus.Fields{
		"event_type":    event.EventType,
		"approval_id":   event.ApprovalRequestID,
		"action_type":   event.ActionType,
		"resource_type": event.ResourceType,
		"status":        event.Status,
	}).Info("Received approval event")

	// Only process events for order resources
	if event.ResourceType != "order" {
		s.logger.WithField("resource_type", event.ResourceType).Debug("Ignoring non-order approval event")
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
	case "order_refund":
		return s.executeRefund(ctx, event, tenantID)
	case "order_cancel":
		return s.executeCancel(ctx, event, tenantID)
	default:
		s.logger.WithField("action_type", event.ActionType).Warn("Unknown action type")
		return nil
	}
}

// executeRefund executes an approved refund
func (s *ApprovalSubscriber) executeRefund(ctx context.Context, event *gosharedevents.ApprovalEvent, tenantID string) error {
	s.logger.WithFields(logrus.Fields{
		"resource_id": event.ResourceID,
		"approver":    event.ApproverName,
	}).Info("Executing approved refund")

	// Parse order ID
	orderID, err := uuid.Parse(event.ResourceID)
	if err != nil {
		s.logger.WithError(err).Error("Invalid order ID in approval event")
		return nil // Don't retry for invalid IDs
	}

	// Parse refund amount from action data
	var refundAmount *float64
	if event.ActionData != nil {
		if amount, ok := event.ActionData["amount"].(float64); ok {
			refundAmount = &amount
		}
	}

	// Get reason from action data
	reason := ""
	if event.ActionData != nil {
		if r, ok := event.ActionData["reason"].(string); ok {
			reason = r
		}
	}

	// Execute the refund
	_, err = s.orderExecutor.RefundOrder(orderID, refundAmount, reason, tenantID)
	if err != nil {
		s.logger.WithError(err).Error("Failed to execute approved refund")
		return err
	}

	s.logger.Info("Approved refund executed successfully")
	return nil
}

// executeCancel executes an approved cancellation
func (s *ApprovalSubscriber) executeCancel(ctx context.Context, event *gosharedevents.ApprovalEvent, tenantID string) error {
	s.logger.WithFields(logrus.Fields{
		"resource_id": event.ResourceID,
		"approver":    event.ApproverName,
	}).Info("Executing approved cancellation")

	// Parse order ID
	orderID, err := uuid.Parse(event.ResourceID)
	if err != nil {
		s.logger.WithError(err).Error("Invalid order ID in approval event")
		return nil // Don't retry for invalid IDs
	}

	// Get reason from action data
	reason := ""
	if event.ActionData != nil {
		if r, ok := event.ActionData["reason"].(string); ok {
			reason = r
		}
	}

	// Execute the cancellation
	_, err = s.orderExecutor.CancelOrder(orderID, reason, tenantID)
	if err != nil {
		s.logger.WithError(err).Error("Failed to execute approved cancellation")
		return err
	}

	s.logger.Info("Approved cancellation executed successfully")
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
	s.logger.Info("Approval subscriber stopped")
}
