package events

import (
	"context"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/Tesseract-Nexus/go-shared/events"
)

// Publisher wraps the shared events publisher for payment-specific events
type Publisher struct {
	publisher *events.Publisher
	logger    *logrus.Entry
}

// NewPublisher creates a new payment events publisher
func NewPublisher(logger *logrus.Logger) (*Publisher, error) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats.nats.svc.cluster.local:4222"
	}

	config := events.DefaultPublisherConfig(natsURL)
	config.Name = "payment-service"

	publisher, err := events.NewPublisher(config, logger)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	if err := publisher.EnsureStream(ctx, events.StreamPayments, []string{"payment.>"}); err != nil {
		logger.WithError(err).Warn("Failed to ensure PAYMENT_EVENTS stream")
	}

	return &Publisher{
		publisher: publisher,
		logger:    logger.WithField("component", "events.publisher"),
	}, nil
}

// PublishPaymentSucceeded publishes a payment succeeded event
func (p *Publisher) PublishPaymentSucceeded(ctx context.Context, tenantID, paymentID, orderID, orderNumber, customerEmail, customerName string, amount float64, currency, provider, method string) error {
	event := events.NewPaymentEvent(events.PaymentSucceeded, tenantID)
	event.PaymentID = paymentID
	event.OrderID = orderID
	event.OrderNumber = orderNumber
	event.CustomerEmail = customerEmail
	event.CustomerName = customerName
	event.Amount = amount
	event.Currency = currency
	event.Provider = provider
	event.Method = method
	event.Status = "succeeded"

	return p.publisher.PublishPayment(ctx, event)
}

// PublishPaymentFailed publishes a payment failed event
func (p *Publisher) PublishPaymentFailed(ctx context.Context, tenantID, paymentID, orderID, orderNumber, customerEmail string, amount float64, currency, errorCode, errorMessage string) error {
	event := events.NewPaymentEvent(events.PaymentFailed, tenantID)
	event.PaymentID = paymentID
	event.OrderID = orderID
	event.OrderNumber = orderNumber
	event.CustomerEmail = customerEmail
	event.Amount = amount
	event.Currency = currency
	event.ErrorCode = errorCode
	event.ErrorMessage = errorMessage
	event.Status = "failed"

	return p.publisher.PublishPayment(ctx, event)
}

// PublishPaymentRefunded publishes a payment refunded event
func (p *Publisher) PublishPaymentRefunded(ctx context.Context, tenantID, paymentID, refundID, orderID, orderNumber, customerEmail string, refundAmount float64, currency, reason string) error {
	event := events.NewPaymentEvent(events.PaymentRefunded, tenantID)
	event.PaymentID = paymentID
	event.RefundID = refundID
	event.OrderID = orderID
	event.OrderNumber = orderNumber
	event.CustomerEmail = customerEmail
	event.RefundAmount = refundAmount
	event.Currency = currency
	event.RefundReason = reason
	event.Status = "refunded"

	return p.publisher.PublishPayment(ctx, event)
}

// IsConnected returns true if connected to NATS
func (p *Publisher) IsConnected() bool {
	return p.publisher.IsConnected()
}

// Close closes the publisher connection
func (p *Publisher) Close() {
	p.publisher.Close()
}
