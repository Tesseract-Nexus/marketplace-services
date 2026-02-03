package events

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/Tesseract-Nexus/go-shared/events"
	"approval-service/internal/models"
)

// Publisher wraps the go-shared events publisher for approval-specific events
type Publisher struct {
	publisher *events.Publisher
	logger    *logrus.Entry
}

// NewPublisher creates a new approval events publisher from an existing go-shared publisher
func NewPublisher(publisher *events.Publisher, logger *logrus.Logger) *Publisher {
	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(logrus.InfoLevel)
	}

	return &Publisher{
		publisher: publisher,
		logger:    logger.WithField("component", "approval-events"),
	}
}

// Close closes the NATS connection
func (p *Publisher) Close() {
	if p.publisher != nil {
		p.publisher.Close()
	}
}

// PublishApprovalRequested publishes an approval.requested event
func (p *Publisher) PublishApprovalRequested(ctx context.Context, request *models.ApprovalRequest, tenantID string) error {
	event := p.buildApprovalEvent(events.ApprovalRequested, request, tenantID)
	event.Status = models.StatusPending
	event.RequestedAt = request.CreatedAt.Format(time.RFC3339)
	return p.publish(ctx, event)
}

// PublishApprovalGranted publishes an approval.granted event
func (p *Publisher) PublishApprovalGranted(ctx context.Context, request *models.ApprovalRequest, decision *models.ApprovalDecision, tenantID string) error {
	event := p.buildApprovalEvent(events.ApprovalGranted, request, tenantID)
	event.Status = models.StatusApproved
	event.PreviousStatus = models.StatusPending
	event.Decision = "approve"
	
	// Add decision details
	if decision != nil {
		event.ApproverID = decision.ApproverID.String()
		event.ApproverRole = decision.ApproverRole
		event.DecisionReason = decision.Comment
		event.DecisionAt = decision.DecidedAt.Format(time.RFC3339)
	}
	
	return p.publish(ctx, event)
}

// PublishApprovalRejected publishes an approval.rejected event
func (p *Publisher) PublishApprovalRejected(ctx context.Context, request *models.ApprovalRequest, decision *models.ApprovalDecision, tenantID string) error {
	event := p.buildApprovalEvent(events.ApprovalRejected, request, tenantID)
	event.Status = models.StatusRejected
	event.PreviousStatus = models.StatusPending
	event.Decision = "reject"
	
	// Add decision details
	if decision != nil {
		event.ApproverID = decision.ApproverID.String()
		event.ApproverRole = decision.ApproverRole
		event.DecisionReason = decision.Comment
		event.DecisionAt = decision.DecidedAt.Format(time.RFC3339)
	}
	
	return p.publish(ctx, event)
}

// PublishApprovalCancelled publishes an approval.cancelled event
func (p *Publisher) PublishApprovalCancelled(ctx context.Context, request *models.ApprovalRequest, tenantID string) error {
	event := p.buildApprovalEvent(events.ApprovalCancelled, request, tenantID)
	event.Status = models.StatusCancelled
	event.PreviousStatus = models.StatusPending
	return p.publish(ctx, event)
}

// PublishApprovalExpired publishes an approval.expired event
func (p *Publisher) PublishApprovalExpired(ctx context.Context, request *models.ApprovalRequest, tenantID string) error {
	event := p.buildApprovalEvent(events.ApprovalExpired, request, tenantID)
	event.Status = models.StatusExpired
	event.PreviousStatus = models.StatusPending
	return p.publish(ctx, event)
}

// PublishApprovalEscalated publishes an approval.escalated event
func (p *Publisher) PublishApprovalEscalated(ctx context.Context, request *models.ApprovalRequest, fromRole, toRole, reason string, tenantID string) error {
	event := p.buildApprovalEvent(events.ApprovalEscalated, request, tenantID)
	event.EscalatedFrom = fromRole
	event.EscalatedTo = toRole
	event.EscalationReason = reason
	event.EscalationLevel = request.EscalationLevel
	return p.publish(ctx, event)
}

// buildApprovalEvent creates an ApprovalEvent from an approval request model
func (p *Publisher) buildApprovalEvent(eventType string, request *models.ApprovalRequest, tenantID string) *events.ApprovalEvent {
	event := events.NewApprovalEvent(eventType, tenantID)
	event.SourceID = uuid.New().String()
	event.ApprovalRequestID = request.ID.String()
	event.WorkflowID = request.WorkflowID.String()
	
	// Get workflow name if available
	if request.Workflow != nil {
		event.WorkflowName = request.Workflow.Name
	}

	// Requester info
	event.RequesterID = request.RequesterID.String()
	event.RequesterName = request.RequesterName

	// Action details
	event.ActionType = request.ActionType
	event.ResourceType = request.ResourceType
	if request.ResourceID != nil {
		event.ResourceID = request.ResourceID.String()
	}

	// Priority and timing
	event.Priority = request.Priority
	event.ExpiresAt = request.ExpiresAt.Format(time.RFC3339)

	return event
}

// publish is a helper that logs and publishes events asynchronously
func (p *Publisher) publish(ctx context.Context, event *events.ApprovalEvent) error {
	go func() {
		pubCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := p.publisher.PublishApproval(pubCtx, event); err != nil {
			p.logger.WithFields(logrus.Fields{
				"eventType":         event.EventType,
				"approvalRequestID": event.ApprovalRequestID,
				"tenantID":          event.TenantID,
			}).WithError(err).Error("Failed to publish approval event")
		} else {
			p.logger.WithFields(logrus.Fields{
				"eventType":         event.EventType,
				"approvalRequestID": event.ApprovalRequestID,
				"tenantID":          event.TenantID,
				"actionType":        event.ActionType,
			}).Info("Approval event published successfully")
		}
	}()

	return nil
}
