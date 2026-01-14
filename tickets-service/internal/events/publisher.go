package events

import (
	"context"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/Tesseract-Nexus/go-shared/events"
)

// Publisher wraps the shared events publisher for ticket-specific events
type Publisher struct {
	publisher *events.Publisher
	logger    *logrus.Entry
}

// NewPublisher creates a new ticket events publisher
func NewPublisher(logger *logrus.Logger) (*Publisher, error) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats.nats.svc.cluster.local:4222"
	}

	config := events.DefaultPublisherConfig(natsURL)
	config.Name = "tickets-service"

	publisher, err := events.NewPublisher(config, logger)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	if err := publisher.EnsureStream(ctx, events.StreamTickets, []string{"ticket.>"}); err != nil {
		logger.WithError(err).Warn("Failed to ensure TICKET_EVENTS stream")
	}

	return &Publisher{
		publisher: publisher,
		logger:    logger.WithField("component", "events.publisher"),
	}, nil
}

// PublishTicketCreated publishes a ticket created event
func (p *Publisher) PublishTicketCreated(ctx context.Context, tenantID, ticketID, ticketNumber, customerEmail, customerName, subject, category, priority string) error {
	event := events.NewTicketEvent(events.TicketCreated, tenantID)
	event.TicketID = ticketID
	event.TicketNumber = ticketNumber
	event.CustomerEmail = customerEmail
	event.CustomerName = customerName
	event.Subject = subject
	event.Category = category
	event.Priority = priority
	event.Status = "OPEN"

	return p.publisher.Publish(ctx, event)
}

// PublishTicketUpdated publishes a ticket updated event
func (p *Publisher) PublishTicketUpdated(ctx context.Context, tenantID, ticketID, ticketNumber, customerEmail, subject, status string) error {
	event := events.NewTicketEvent(events.TicketUpdated, tenantID)
	event.TicketID = ticketID
	event.TicketNumber = ticketNumber
	event.CustomerEmail = customerEmail
	event.Subject = subject
	event.Status = status

	return p.publisher.Publish(ctx, event)
}

// PublishTicketAssigned publishes a ticket assigned event
func (p *Publisher) PublishTicketAssigned(ctx context.Context, tenantID, ticketID, ticketNumber, customerEmail, subject, assignedTo, assignedToName, team string) error {
	event := events.NewTicketEvent(events.TicketAssigned, tenantID)
	event.TicketID = ticketID
	event.TicketNumber = ticketNumber
	event.CustomerEmail = customerEmail
	event.Subject = subject
	event.AssignedTo = assignedTo
	event.AssignedToName = assignedToName
	event.Team = team
	event.Status = "IN_PROGRESS"

	return p.publisher.Publish(ctx, event)
}

// PublishTicketResolved publishes a ticket resolved event
func (p *Publisher) PublishTicketResolved(ctx context.Context, tenantID, ticketID, ticketNumber, customerEmail, subject, resolution, resolvedBy string) error {
	event := events.NewTicketEvent(events.TicketResolved, tenantID)
	event.TicketID = ticketID
	event.TicketNumber = ticketNumber
	event.CustomerEmail = customerEmail
	event.Subject = subject
	event.Resolution = resolution
	event.ResolvedBy = resolvedBy
	event.Status = "RESOLVED"

	return p.publisher.Publish(ctx, event)
}

// PublishTicketClosed publishes a ticket closed event
func (p *Publisher) PublishTicketClosed(ctx context.Context, tenantID, ticketID, ticketNumber, customerEmail, subject, closedBy string) error {
	event := events.NewTicketEvent(events.TicketClosed, tenantID)
	event.TicketID = ticketID
	event.TicketNumber = ticketNumber
	event.CustomerEmail = customerEmail
	event.Subject = subject
	event.ClosedBy = closedBy
	event.Status = "CLOSED"

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
