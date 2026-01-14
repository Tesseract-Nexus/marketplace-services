package events

import (
	"context"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/Tesseract-Nexus/go-shared/events"
)

// Shipping event types
const (
	ShipmentCreated   = "shipping.shipment_created"
	ShipmentUpdated   = "shipping.shipment_updated"
	ShipmentShipped   = "shipping.shipped"
	ShipmentDelivered = "shipping.delivered"
	ShipmentFailed    = "shipping.failed"
	RateCreated       = "shipping.rate_created"
	RateUpdated       = "shipping.rate_updated"
	RateDeleted       = "shipping.rate_deleted"
)

// ShippingEvent represents a shipping-related event
type ShippingEvent struct {
	events.BaseEvent
	ShipmentID     string                 `json:"shipmentId,omitempty"`
	OrderID        string                 `json:"orderId,omitempty"`
	OrderNumber    string                 `json:"orderNumber,omitempty"`
	TrackingNumber string                 `json:"trackingNumber,omitempty"`
	Carrier        string                 `json:"carrier,omitempty"`
	CarrierCode    string                 `json:"carrierCode,omitempty"`
	Status         string                 `json:"status,omitempty"`
	RateID         string                 `json:"rateId,omitempty"`
	RateName       string                 `json:"rateName,omitempty"`
	Price          float64                `json:"price,omitempty"`
	Currency       string                 `json:"currency,omitempty"`
	CustomerEmail  string                 `json:"customerEmail,omitempty"`
	CustomerName   string                 `json:"customerName,omitempty"`
	ActorID        string                 `json:"actorId,omitempty"`
	ActorName      string                 `json:"actorName,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

func (e *ShippingEvent) GetSubject() string {
	return e.EventType
}

func (e *ShippingEvent) GetStream() string {
	return "SHIPPING_EVENTS"
}

// Publisher wraps the shared events publisher for shipping-specific events
type Publisher struct {
	publisher *events.Publisher
	logger    *logrus.Entry
}

// NewPublisher creates a new shipping events publisher
func NewPublisher(logger *logrus.Logger) (*Publisher, error) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats.nats.svc.cluster.local:4222"
	}

	config := events.DefaultPublisherConfig(natsURL)
	config.Name = "shipping-service"

	publisher, err := events.NewPublisher(config, logger)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	if err := publisher.EnsureStream(ctx, "SHIPPING_EVENTS", []string{"shipping.>"}); err != nil {
		logger.WithError(err).Warn("Failed to ensure SHIPPING_EVENTS stream")
	}

	return &Publisher{
		publisher: publisher,
		logger:    logger.WithField("component", "events.publisher"),
	}, nil
}

// PublishShipmentCreated publishes a shipment created event
func (p *Publisher) PublishShipmentCreated(ctx context.Context, tenantID, shipmentID, orderID, orderNumber, carrier, customerEmail, customerName string) error {
	event := &ShippingEvent{
		BaseEvent: events.BaseEvent{
			EventType: ShipmentCreated,
			TenantID:  tenantID,
			Timestamp: time.Now().UTC(),
		},
		ShipmentID:    shipmentID,
		OrderID:       orderID,
		OrderNumber:   orderNumber,
		Carrier:       carrier,
		CustomerEmail: customerEmail,
		CustomerName:  customerName,
		Status:        "CREATED",
	}

	return p.publisher.Publish(ctx, event)
}

// PublishShipmentShipped publishes a shipment shipped event
func (p *Publisher) PublishShipmentShipped(ctx context.Context, tenantID, shipmentID, orderID, orderNumber, trackingNumber, carrier, carrierCode, customerEmail, customerName string) error {
	event := &ShippingEvent{
		BaseEvent: events.BaseEvent{
			EventType: ShipmentShipped,
			TenantID:  tenantID,
			Timestamp: time.Now().UTC(),
		},
		ShipmentID:     shipmentID,
		OrderID:        orderID,
		OrderNumber:    orderNumber,
		TrackingNumber: trackingNumber,
		Carrier:        carrier,
		CarrierCode:    carrierCode,
		CustomerEmail:  customerEmail,
		CustomerName:   customerName,
		Status:         "SHIPPED",
	}

	return p.publisher.Publish(ctx, event)
}

// PublishShipmentDelivered publishes a shipment delivered event
func (p *Publisher) PublishShipmentDelivered(ctx context.Context, tenantID, shipmentID, orderID, orderNumber, trackingNumber, customerEmail, customerName string) error {
	event := &ShippingEvent{
		BaseEvent: events.BaseEvent{
			EventType: ShipmentDelivered,
			TenantID:  tenantID,
			Timestamp: time.Now().UTC(),
		},
		ShipmentID:     shipmentID,
		OrderID:        orderID,
		OrderNumber:    orderNumber,
		TrackingNumber: trackingNumber,
		CustomerEmail:  customerEmail,
		CustomerName:   customerName,
		Status:         "DELIVERED",
	}

	return p.publisher.Publish(ctx, event)
}

// PublishRateCreated publishes a shipping rate created event
func (p *Publisher) PublishRateCreated(ctx context.Context, tenantID, rateID, rateName, carrier string, price float64, currency, actorID, actorName string) error {
	event := &ShippingEvent{
		BaseEvent: events.BaseEvent{
			EventType: RateCreated,
			TenantID:  tenantID,
			Timestamp: time.Now().UTC(),
		},
		RateID:    rateID,
		RateName:  rateName,
		Carrier:   carrier,
		Price:     price,
		Currency:  currency,
		ActorID:   actorID,
		ActorName: actorName,
		Status:    "ACTIVE",
	}

	return p.publisher.Publish(ctx, event)
}

// PublishRateUpdated publishes a shipping rate updated event
func (p *Publisher) PublishRateUpdated(ctx context.Context, tenantID, rateID, rateName, carrier string, price float64, currency, actorID, actorName string) error {
	event := &ShippingEvent{
		BaseEvent: events.BaseEvent{
			EventType: RateUpdated,
			TenantID:  tenantID,
			Timestamp: time.Now().UTC(),
		},
		RateID:    rateID,
		RateName:  rateName,
		Carrier:   carrier,
		Price:     price,
		Currency:  currency,
		ActorID:   actorID,
		ActorName: actorName,
	}

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
