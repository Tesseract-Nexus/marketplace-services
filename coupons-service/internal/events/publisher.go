package events

import (
	"context"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/Tesseract-Nexus/go-shared/events"
)

// Publisher wraps the shared events publisher for coupon-specific events
type Publisher struct {
	publisher *events.Publisher
	logger    *logrus.Entry
}

// NewPublisher creates a new coupon events publisher
func NewPublisher(logger *logrus.Logger) (*Publisher, error) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats.nats.svc.cluster.local:4222"
	}

	config := events.DefaultPublisherConfig(natsURL)
	config.Name = "coupons-service"

	publisher, err := events.NewPublisher(config, logger)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	if err := publisher.EnsureStream(ctx, events.StreamCoupons, []string{"coupon.>"}); err != nil {
		logger.WithError(err).Warn("Failed to ensure COUPON_EVENTS stream")
	}

	return &Publisher{
		publisher: publisher,
		logger:    logger.WithField("component", "events.publisher"),
	}, nil
}

// PublishCouponCreated publishes a coupon created event
func (p *Publisher) PublishCouponCreated(ctx context.Context, tenantID, couponID, couponCode, discountType string, discountValue float64, validFrom, validUntil string) error {
	event := events.NewCouponEvent(events.CouponCreated, tenantID)
	event.CouponID = couponID
	event.CouponCode = couponCode
	event.DiscountType = discountType
	event.DiscountValue = discountValue
	event.ValidFrom = validFrom
	event.ValidUntil = validUntil
	event.Status = "ACTIVE"

	return p.publisher.Publish(ctx, event)
}

// PublishCouponApplied publishes a coupon applied event
func (p *Publisher) PublishCouponApplied(ctx context.Context, tenantID, couponID, couponCode, orderID, orderNumber, customerID, customerEmail string, discountAmount, orderValue float64, currency string) error {
	event := events.NewCouponEvent(events.CouponApplied, tenantID)
	event.CouponID = couponID
	event.CouponCode = couponCode
	event.OrderID = orderID
	event.OrderNumber = orderNumber
	event.CustomerID = customerID
	event.CustomerEmail = customerEmail
	event.DiscountAmount = discountAmount
	event.OrderValue = orderValue
	event.Currency = currency

	return p.publisher.Publish(ctx, event)
}

// PublishCouponUpdated publishes a coupon updated event
func (p *Publisher) PublishCouponUpdated(ctx context.Context, tenantID, couponID, couponCode, discountType string, discountValue float64, status string) error {
	event := events.NewCouponEvent(events.CouponUpdated, tenantID)
	event.CouponID = couponID
	event.CouponCode = couponCode
	event.DiscountType = discountType
	event.DiscountValue = discountValue
	event.Status = status

	return p.publisher.Publish(ctx, event)
}

// PublishCouponDeleted publishes a coupon deleted event
func (p *Publisher) PublishCouponDeleted(ctx context.Context, tenantID, couponID, couponCode string) error {
	event := events.NewCouponEvent(events.CouponDeleted, tenantID)
	event.CouponID = couponID
	event.CouponCode = couponCode
	event.Status = "DELETED"

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
