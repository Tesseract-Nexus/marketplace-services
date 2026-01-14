package events

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/Tesseract-Nexus/go-shared/events"
	"orders-service/internal/models"
)

// Publisher wraps the go-shared events publisher for order-specific events
type Publisher struct {
	publisher *events.Publisher
	logger    *logrus.Entry
}

// NewPublisher creates a new order events publisher
func NewPublisher(logger *logrus.Logger) (*Publisher, error) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		// Default to GKE internal NATS service URL
		// For local development, set NATS_URL=nats://localhost:4222
		natsURL = "nats://nats.nats.svc.cluster.local:4222"
	}

	config := events.DefaultPublisherConfig(natsURL)
	config.Name = "orders-service"

	publisher, err := events.NewPublisher(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create events publisher: %w", err)
	}

	// Ensure the orders stream exists
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := publisher.EnsureStream(ctx, events.StreamOrders, []string{"order.>"}); err != nil {
		logger.WithError(err).Warn("Failed to ensure orders stream (may already exist)")
	}

	return &Publisher{
		publisher: publisher,
		logger:    logger.WithField("component", "orders-events"),
	}, nil
}

// Close closes the NATS connection
func (p *Publisher) Close() {
	if p.publisher != nil {
		p.publisher.Close()
	}
}

// PublishOrderCreated publishes an order.created event
func (p *Publisher) PublishOrderCreated(ctx context.Context, order *models.Order, tenantID string) error {
	event := p.buildOrderEvent(events.OrderCreated, order, tenantID)
	return p.publish(ctx, event)
}

// PublishOrderConfirmed publishes an order.confirmed event (after payment)
func (p *Publisher) PublishOrderConfirmed(ctx context.Context, order *models.Order, tenantID string) error {
	event := p.buildOrderEvent(events.OrderConfirmed, order, tenantID)
	return p.publish(ctx, event)
}

// PublishOrderStatusChanged publishes an order.status_changed event
func (p *Publisher) PublishOrderStatusChanged(ctx context.Context, order *models.Order, previousStatus, newStatus string, tenantID string) error {
	event := p.buildOrderEvent("order.status_changed", order, tenantID)
	event.Metadata = map[string]interface{}{
		"previousStatus": previousStatus,
		"newStatus":      newStatus,
	}
	return p.publish(ctx, event)
}

// PublishOrderShipped publishes an order.shipped event
func (p *Publisher) PublishOrderShipped(ctx context.Context, order *models.Order, tenantID string) error {
	event := p.buildOrderEvent(events.OrderShipped, order, tenantID)
	if order.Shipping != nil {
		event.CarrierName = order.Shipping.Carrier
		event.TrackingNumber = order.Shipping.TrackingNumber
		// TrackingURL is not stored in the model - it's typically constructed from carrier + tracking number
	}
	return p.publish(ctx, event)
}

// PublishOrderDelivered publishes an order.delivered event
func (p *Publisher) PublishOrderDelivered(ctx context.Context, order *models.Order, tenantID string) error {
	event := p.buildOrderEvent(events.OrderDelivered, order, tenantID)
	event.DeliveryDate = time.Now().UTC().Format(time.RFC3339)
	return p.publish(ctx, event)
}

// PublishOrderCancelled publishes an order.cancelled event
func (p *Publisher) PublishOrderCancelled(ctx context.Context, order *models.Order, reason string, tenantID string) error {
	event := p.buildOrderEvent(events.OrderCancelled, order, tenantID)
	event.CancellationReason = reason
	event.CancelledBy = "customer" // or "admin" based on context
	return p.publish(ctx, event)
}

// PublishOrderRefunded publishes an order.refunded event
func (p *Publisher) PublishOrderRefunded(ctx context.Context, order *models.Order, refundAmount float64, reason string, tenantID string) error {
	event := p.buildOrderEvent(events.OrderRefunded, order, tenantID)
	event.RefundAmount = refundAmount
	event.RefundReason = reason
	return p.publish(ctx, event)
}

// PublishPaymentReceived publishes a payment.captured event
func (p *Publisher) PublishPaymentReceived(ctx context.Context, order *models.Order, transactionID string, tenantID string) error {
	event := events.NewPaymentEvent(events.PaymentCaptured, tenantID)
	event.SourceID = uuid.New().String()
	event.OrderID = order.ID.String()
	event.OrderNumber = order.OrderNumber
	event.Amount = order.Total
	event.Currency = order.Currency
	event.Status = "captured"
	event.PaymentID = transactionID

	if order.Customer != nil {
		event.CustomerEmail = order.Customer.Email
		event.CustomerName = fmt.Sprintf("%s %s", order.Customer.FirstName, order.Customer.LastName)
		event.CustomerID = order.CustomerID.String()
	}

	if order.Payment != nil {
		event.Method = order.Payment.Method
	}

	return p.publisher.PublishPayment(ctx, event)
}

// buildOrderEvent creates an OrderEvent from an order model
func (p *Publisher) buildOrderEvent(eventType string, order *models.Order, tenantID string) *events.OrderEvent {
	event := events.NewOrderEvent(eventType, tenantID)
	event.SourceID = uuid.New().String()
	event.OrderID = order.ID.String()
	event.OrderNumber = order.OrderNumber
	event.OrderDate = order.CreatedAt.Format(time.RFC3339)
	event.Status = string(order.Status)
	event.PaymentStatus = string(order.PaymentStatus)
	event.FulfillmentStatus = string(order.FulfillmentStatus)
	event.TotalAmount = order.Total
	event.Subtotal = order.Subtotal
	event.Tax = order.TaxAmount
	event.Discount = order.DiscountAmount
	event.ShippingCost = order.ShippingCost
	event.Currency = order.Currency
	event.CustomerID = order.CustomerID.String()

	// Customer info
	if order.Customer != nil {
		event.CustomerEmail = order.Customer.Email
		event.CustomerName = fmt.Sprintf("%s %s", order.Customer.FirstName, order.Customer.LastName)
		event.CustomerPhone = order.Customer.Phone
	}

	// Order items
	event.Items = make([]events.OrderItem, len(order.Items))
	for i, item := range order.Items {
		event.Items[i] = events.OrderItem{
			ProductID:  item.ProductID.String(),
			SKU:        item.SKU,
			Name:       item.ProductName,
			Quantity:   item.Quantity,
			UnitPrice:  item.UnitPrice,
			TotalPrice: item.TotalPrice,
		}
	}
	event.ItemCount = len(order.Items)

	// Shipping info
	if order.Shipping != nil {
		event.ShippingMethod = order.Shipping.Method
		event.ShippingAddress = &events.Address{
			Name:       event.CustomerName,
			Line1:      order.Shipping.Street,
			City:       order.Shipping.City,
			State:      order.Shipping.State,
			PostalCode: order.Shipping.PostalCode,
			Country:    order.Shipping.Country,
		}
		event.CarrierName = order.Shipping.Carrier
		event.TrackingNumber = order.Shipping.TrackingNumber
		// TrackingURL is typically constructed from carrier + tracking number on the frontend
		if order.Shipping.EstimatedDelivery != nil {
			event.EstimatedDelivery = order.Shipping.EstimatedDelivery.Format(time.RFC3339)
		}
	}

	// Payment method
	if order.Payment != nil {
		event.PaymentMethod = order.Payment.Method
	}

	return event
}

// publish is a helper that logs and publishes events asynchronously
func (p *Publisher) publish(ctx context.Context, event *events.OrderEvent) error {
	// Publish asynchronously to not block the main flow
	go func() {
		pubCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := p.publisher.PublishOrder(pubCtx, event); err != nil {
			p.logger.WithFields(logrus.Fields{
				"eventType":   event.EventType,
				"orderNumber": event.OrderNumber,
				"tenantID":    event.TenantID,
			}).WithError(err).Error("Failed to publish order event")
		} else {
			p.logger.WithFields(logrus.Fields{
				"eventType":   event.EventType,
				"orderNumber": event.OrderNumber,
				"tenantID":    event.TenantID,
			}).Info("Order event published successfully")
		}
	}()

	return nil
}
