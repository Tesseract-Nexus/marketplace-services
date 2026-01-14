package events

import (
	"context"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/Tesseract-Nexus/go-shared/events"
)

var (
	publisher     *Publisher
	publisherOnce sync.Once
	publisherMu   sync.RWMutex
)

// Publisher wraps the shared events publisher for tax-specific events
type Publisher struct {
	publisher *events.Publisher
	logger    *logrus.Entry
}

// InitPublisher initializes the singleton NATS publisher
func InitPublisher(logger *logrus.Logger) error {
	var initErr error
	publisherOnce.Do(func() {
		natsURL := os.Getenv("NATS_URL")
		if natsURL == "" {
			logger.Warn("NATS_URL not set, event publishing disabled")
			return
		}

		config := events.DefaultPublisherConfig(natsURL)
		config.Name = "tax-service"

		pub, err := events.NewPublisher(config, logger)
		if err != nil {
			initErr = err
			return
		}

		ctx := context.Background()
		if err := pub.EnsureStream(ctx, events.StreamTax, []string{"tax.>"}); err != nil {
			logger.WithError(err).Warn("Failed to ensure TAX_EVENTS stream")
		}

		publisherMu.Lock()
		publisher = &Publisher{
			publisher: pub,
			logger:    logger.WithField("component", "events.publisher"),
		}
		publisherMu.Unlock()

		logger.Info("NATS events publisher initialized for tax-service")
	})
	return initErr
}

// GetPublisher returns the singleton publisher instance
func GetPublisher() *Publisher {
	publisherMu.RLock()
	defer publisherMu.RUnlock()
	return publisher
}

// PublishTaxCalculated publishes a tax calculated event
func (p *Publisher) PublishTaxCalculated(ctx context.Context, tenantID, calculationID, orderID, customerID string, taxableAmount, taxAmount float64, currency string) error {
	event := events.NewTaxEvent(events.TaxCalculated, tenantID)
	event.CalculationID = calculationID
	event.OrderID = orderID
	event.CustomerID = customerID
	event.TaxableAmount = taxableAmount
	event.TaxAmount = taxAmount
	event.Currency = currency

	return p.publisher.Publish(ctx, event)
}

// PublishJurisdictionCreated publishes a jurisdiction created event
func (p *Publisher) PublishJurisdictionCreated(ctx context.Context, tenantID, jurisdictionID, name, jurisdictionType string) error {
	event := events.NewTaxEvent(events.TaxJurisdictionCreated, tenantID)
	event.JurisdictionID = jurisdictionID
	event.JurisdictionName = name
	event.JurisdictionType = jurisdictionType

	return p.publisher.Publish(ctx, event)
}

// PublishJurisdictionUpdated publishes a jurisdiction updated event
func (p *Publisher) PublishJurisdictionUpdated(ctx context.Context, tenantID, jurisdictionID, name string, actorID, actorName string) error {
	event := events.NewTaxEvent(events.TaxJurisdictionUpdated, tenantID)
	event.JurisdictionID = jurisdictionID
	event.JurisdictionName = name
	event.ActorID = actorID
	event.ActorName = actorName

	return p.publisher.Publish(ctx, event)
}

// PublishJurisdictionDeleted publishes a jurisdiction deleted event
func (p *Publisher) PublishJurisdictionDeleted(ctx context.Context, tenantID, jurisdictionID string, actorID, actorName string) error {
	event := events.NewTaxEvent(events.TaxJurisdictionDeleted, tenantID)
	event.JurisdictionID = jurisdictionID
	event.ActorID = actorID
	event.ActorName = actorName

	return p.publisher.Publish(ctx, event)
}

// PublishTaxRateCreated publishes a tax rate created event
func (p *Publisher) PublishTaxRateCreated(ctx context.Context, tenantID, taxRateID, jurisdictionID string, rate float64) error {
	event := events.NewTaxEvent(events.TaxRateCreated, tenantID)
	event.TaxRateID = taxRateID
	event.JurisdictionID = jurisdictionID
	event.TaxRate = rate

	return p.publisher.Publish(ctx, event)
}

// PublishTaxRateUpdated publishes a tax rate updated event
func (p *Publisher) PublishTaxRateUpdated(ctx context.Context, tenantID, taxRateID string, rate float64, actorID, actorName string) error {
	event := events.NewTaxEvent(events.TaxRateUpdated, tenantID)
	event.TaxRateID = taxRateID
	event.TaxRate = rate
	event.ActorID = actorID
	event.ActorName = actorName

	return p.publisher.Publish(ctx, event)
}

// PublishExemptionCreated publishes an exemption certificate created event
func (p *Publisher) PublishExemptionCreated(ctx context.Context, tenantID, exemptionID, exemptionType, exemptionNumber, customerID string) error {
	event := events.NewTaxEvent(events.TaxExemptionCreated, tenantID)
	event.ExemptionID = exemptionID
	event.ExemptionType = exemptionType
	event.ExemptionNumber = exemptionNumber
	event.CustomerID = customerID

	return p.publisher.Publish(ctx, event)
}

// IsConnected returns true if connected to NATS
func (p *Publisher) IsConnected() bool {
	return p.publisher != nil && p.publisher.IsConnected()
}

// Close closes the publisher connection
func (p *Publisher) Close() {
	if p.publisher != nil {
		p.publisher.Close()
	}
}
