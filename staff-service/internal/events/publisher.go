package events

import (
	"context"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/Tesseract-Nexus/go-shared/events"
)

// Publisher wraps the shared events publisher for staff-specific events
type Publisher struct {
	publisher *events.Publisher
	logger    *logrus.Entry
}

// NewPublisher creates a new staff events publisher
func NewPublisher(logger *logrus.Logger) (*Publisher, error) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats.nats.svc.cluster.local:4222"
	}

	config := events.DefaultPublisherConfig(natsURL)
	config.Name = "staff-service"

	publisher, err := events.NewPublisher(config, logger)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	if err := publisher.EnsureStream(ctx, events.StreamStaff, []string{"staff.>"}); err != nil {
		logger.WithError(err).Warn("Failed to ensure STAFF_EVENTS stream")
	}

	return &Publisher{
		publisher: publisher,
		logger:    logger.WithField("component", "events.publisher"),
	}, nil
}

// PublishStaffCreated publishes a staff created event
func (p *Publisher) PublishStaffCreated(ctx context.Context, tenantID, staffID, staffEmail, staffName, department, position string, roles []string) error {
	event := events.NewStaffEvent(events.StaffCreated, tenantID)
	event.StaffID = staffID
	event.StaffEmail = staffEmail
	event.StaffName = staffName
	event.Department = department
	event.Position = position
	event.Roles = roles
	event.Status = "ACTIVE"

	return p.publisher.Publish(ctx, event)
}

// PublishStaffUpdated publishes a staff updated event
func (p *Publisher) PublishStaffUpdated(ctx context.Context, tenantID, staffID, staffEmail, staffName, department, position, changedBy string) error {
	event := events.NewStaffEvent(events.StaffUpdated, tenantID)
	event.StaffID = staffID
	event.StaffEmail = staffEmail
	event.StaffName = staffName
	event.Department = department
	event.Position = position
	event.ChangedBy = changedBy

	return p.publisher.Publish(ctx, event)
}

// PublishStaffDeactivated publishes a staff deactivated event
func (p *Publisher) PublishStaffDeactivated(ctx context.Context, tenantID, staffID, staffEmail, staffName, reason, changedBy string) error {
	event := events.NewStaffEvent(events.StaffDeactivated, tenantID)
	event.StaffID = staffID
	event.StaffEmail = staffEmail
	event.StaffName = staffName
	event.Status = "INACTIVE"
	event.PreviousStatus = "ACTIVE"
	event.StatusReason = reason
	event.ChangedBy = changedBy

	return p.publisher.Publish(ctx, event)
}

// PublishStaffRoleChanged publishes a staff role changed event
func (p *Publisher) PublishStaffRoleChanged(ctx context.Context, tenantID, staffID, staffEmail, staffName, oldRole, newRole, changedBy string) error {
	event := events.NewStaffEvent(events.StaffRoleChanged, tenantID)
	event.StaffID = staffID
	event.StaffEmail = staffEmail
	event.StaffName = staffName
	event.OldRole = oldRole
	event.NewRole = newRole
	event.ChangedBy = changedBy

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
