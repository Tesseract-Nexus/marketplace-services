package events

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/Tesseract-Nexus/go-shared/events"
	"customers-service/internal/models"
)

// Publisher wraps the go-shared events publisher for customer-specific events
type Publisher struct {
	publisher *events.Publisher
	logger    *logrus.Entry
}

// NewPublisher creates a new customer events publisher
func NewPublisher(logger *logrus.Logger) (*Publisher, error) {
	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(logrus.InfoLevel)
	}

	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats.nats.svc.cluster.local:4222"
	}

	config := events.DefaultPublisherConfig(natsURL)
	config.Name = "customers-service"

	publisher, err := events.NewPublisher(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create events publisher: %w", err)
	}

	// Ensure the customers stream exists
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := publisher.EnsureStream(ctx, events.StreamCustomers, []string{"customer.>"}); err != nil {
		logger.WithError(err).Warn("Failed to ensure customers stream (may already exist)")
	}

	return &Publisher{
		publisher: publisher,
		logger:    logger.WithField("component", "customers-events"),
	}, nil
}

// Close closes the NATS connection
func (p *Publisher) Close() {
	if p.publisher != nil {
		p.publisher.Close()
	}
}

// PublishCustomerCreated publishes a customer.created event
func (p *Publisher) PublishCustomerCreated(ctx context.Context, customer *models.Customer, tenantID string) error {
	event := p.buildCustomerEvent(events.CustomerCreated, customer, tenantID)
	return p.publish(ctx, event)
}

// PublishCustomerRegistered publishes a customer.registered event (for self-registration)
func (p *Publisher) PublishCustomerRegistered(ctx context.Context, customer *models.Customer, tenantID string) error {
	event := p.buildCustomerEvent(events.CustomerRegistered, customer, tenantID)
	return p.publish(ctx, event)
}

// PublishCustomerUpdated publishes a customer.updated event
func (p *Publisher) PublishCustomerUpdated(ctx context.Context, customer *models.Customer, tenantID string) error {
	event := p.buildCustomerEvent(events.CustomerUpdated, customer, tenantID)
	return p.publish(ctx, event)
}

// PublishCustomerDeleted publishes a customer.deleted event
func (p *Publisher) PublishCustomerDeleted(ctx context.Context, customer *models.Customer, tenantID string) error {
	event := p.buildCustomerEvent(events.CustomerDeleted, customer, tenantID)
	return p.publish(ctx, event)
}

// buildCustomerEvent creates a CustomerEvent from a customer model
func (p *Publisher) buildCustomerEvent(eventType string, customer *models.Customer, tenantID string) *events.CustomerEvent {
	event := events.NewCustomerEvent(eventType, tenantID)
	event.SourceID = uuid.New().String()
	event.CustomerID = customer.ID.String()
	event.CustomerEmail = customer.Email
	
	// Build customer name from first/last name
	customerName := ""
	if customer.FirstName != "" {
		customerName = customer.FirstName
		event.FirstName = customer.FirstName
	}
	if customer.LastName != "" {
		if customerName != "" {
			customerName += " "
		}
		customerName += customer.LastName
		event.LastName = customer.LastName
	}
	event.CustomerName = customerName

	if customer.Phone != "" {
		event.CustomerPhone = customer.Phone
	}

	return event
}

// publish is a helper that logs and publishes events asynchronously
func (p *Publisher) publish(ctx context.Context, event *events.CustomerEvent) error {
	go func() {
		pubCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := p.publisher.PublishCustomer(pubCtx, event); err != nil {
			p.logger.WithFields(logrus.Fields{
				"eventType":  event.EventType,
				"customerID": event.CustomerID,
				"tenantID":   event.TenantID,
			}).WithError(err).Error("Failed to publish customer event")
		} else {
			p.logger.WithFields(logrus.Fields{
				"eventType":     event.EventType,
				"customerID":    event.CustomerID,
				"customerEmail": event.CustomerEmail,
				"tenantID":      event.TenantID,
			}).Info("Customer event published successfully")
		}
	}()

	return nil
}
