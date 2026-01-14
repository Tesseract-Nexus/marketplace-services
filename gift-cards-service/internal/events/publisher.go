package events

import (
	"context"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/Tesseract-Nexus/go-shared/events"
)

// Publisher wraps the shared events publisher for gift card-specific events
type Publisher struct {
	publisher *events.Publisher
	logger    *logrus.Entry
}

// NewPublisher creates a new gift card events publisher
func NewPublisher(logger *logrus.Logger) (*Publisher, error) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats.nats.svc.cluster.local:4222"
	}

	config := events.DefaultPublisherConfig(natsURL)
	config.Name = "gift-cards-service"

	publisher, err := events.NewPublisher(config, logger)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	if err := publisher.EnsureStream(ctx, events.StreamGiftCards, []string{"gift_card.>"}); err != nil {
		logger.WithError(err).Warn("Failed to ensure GIFT_CARD_EVENTS stream")
	}

	return &Publisher{
		publisher: publisher,
		logger:    logger.WithField("component", "events.publisher"),
	}, nil
}

// PublishGiftCardCreated publishes a gift card created event
func (p *Publisher) PublishGiftCardCreated(ctx context.Context, tenantID, giftCardID, giftCardCode, purchaserEmail, purchaserName, recipientEmail, recipientName string, initialBalance float64, currency string) error {
	event := events.NewGiftCardEvent(events.GiftCardCreated, tenantID)
	event.GiftCardID = giftCardID
	event.GiftCardCode = giftCardCode
	event.PurchaserEmail = purchaserEmail
	event.PurchaserName = purchaserName
	event.RecipientEmail = recipientEmail
	event.RecipientName = recipientName
	event.InitialBalance = initialBalance
	event.CurrentBalance = initialBalance
	event.Currency = currency
	event.Status = "PENDING"

	return p.publisher.Publish(ctx, event)
}

// PublishGiftCardActivated publishes a gift card activated event
func (p *Publisher) PublishGiftCardActivated(ctx context.Context, tenantID, giftCardID, giftCardCode, recipientEmail string, balance float64, currency, activatedAt, expiresAt string) error {
	event := events.NewGiftCardEvent(events.GiftCardActivated, tenantID)
	event.GiftCardID = giftCardID
	event.GiftCardCode = giftCardCode
	event.RecipientEmail = recipientEmail
	event.InitialBalance = balance
	event.CurrentBalance = balance
	event.Currency = currency
	event.Status = "ACTIVE"
	event.ActivatedAt = activatedAt
	event.ExpiresAt = expiresAt

	return p.publisher.Publish(ctx, event)
}

// PublishGiftCardApplied publishes a gift card applied event
func (p *Publisher) PublishGiftCardApplied(ctx context.Context, tenantID, giftCardID, giftCardCode, orderID, orderNumber string, amountUsed, currentBalance float64, currency string) error {
	event := events.NewGiftCardEvent(events.GiftCardApplied, tenantID)
	event.GiftCardID = giftCardID
	event.GiftCardCode = giftCardCode
	event.OrderID = orderID
	event.OrderNumber = orderNumber
	event.AmountUsed = amountUsed
	event.CurrentBalance = currentBalance
	event.Currency = currency

	return p.publisher.Publish(ctx, event)
}

// PublishGiftCardRefunded publishes a gift card refunded event
func (p *Publisher) PublishGiftCardRefunded(ctx context.Context, tenantID, giftCardID, giftCardCode, orderID string, refundAmount, currentBalance float64, currency string) error {
	event := events.NewGiftCardEvent(events.GiftCardRefunded, tenantID)
	event.GiftCardID = giftCardID
	event.GiftCardCode = giftCardCode
	event.OrderID = orderID
	event.RefundAmount = refundAmount
	event.CurrentBalance = currentBalance
	event.Currency = currency

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
