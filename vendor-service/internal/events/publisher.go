package events

import (
	"context"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/Tesseract-Nexus/go-shared/events"
)

// Publisher wraps the shared events publisher for vendor-specific events
type Publisher struct {
	publisher *events.Publisher
	logger    *logrus.Entry
}

// NewPublisher creates a new vendor events publisher
func NewPublisher(logger *logrus.Logger) (*Publisher, error) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats.nats.svc.cluster.local:4222"
	}

	config := events.DefaultPublisherConfig(natsURL)
	config.Name = "vendor-service"

	publisher, err := events.NewPublisher(config, logger)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	if err := publisher.EnsureStream(ctx, events.StreamVendors, []string{"vendor.>"}); err != nil {
		logger.WithError(err).Warn("Failed to ensure VENDOR_EVENTS stream")
	}

	return &Publisher{
		publisher: publisher,
		logger:    logger.WithField("component", "events.publisher"),
	}, nil
}

// PublishVendorCreated publishes a vendor created event
func (p *Publisher) PublishVendorCreated(ctx context.Context, tenantID, vendorID, vendorName, vendorEmail, businessName string) error {
	event := events.NewVendorEvent(events.VendorCreated, tenantID)
	event.VendorID = vendorID
	event.VendorName = vendorName
	event.VendorEmail = vendorEmail
	event.BusinessName = businessName
	event.Status = "PENDING"

	return p.publisher.Publish(ctx, event)
}

// PublishVendorUpdated publishes a vendor updated event
func (p *Publisher) PublishVendorUpdated(ctx context.Context, tenantID, vendorID, vendorName, vendorEmail, businessName string) error {
	event := events.NewVendorEvent(events.VendorUpdated, tenantID)
	event.VendorID = vendorID
	event.VendorName = vendorName
	event.VendorEmail = vendorEmail
	event.BusinessName = businessName

	return p.publisher.Publish(ctx, event)
}

// PublishVendorApproved publishes a vendor approved event
func (p *Publisher) PublishVendorApproved(ctx context.Context, tenantID, vendorID, vendorName, vendorEmail, reviewedBy string) error {
	event := events.NewVendorEvent(events.VendorApproved, tenantID)
	event.VendorID = vendorID
	event.VendorName = vendorName
	event.VendorEmail = vendorEmail
	event.Status = "APPROVED"
	event.PreviousStatus = "PENDING"
	event.ReviewedBy = reviewedBy

	return p.publisher.Publish(ctx, event)
}

// PublishVendorRejected publishes a vendor rejected event
func (p *Publisher) PublishVendorRejected(ctx context.Context, tenantID, vendorID, vendorName, vendorEmail, reviewedBy, rejectReason string) error {
	event := events.NewVendorEvent(events.VendorRejected, tenantID)
	event.VendorID = vendorID
	event.VendorName = vendorName
	event.VendorEmail = vendorEmail
	event.Status = "REJECTED"
	event.PreviousStatus = "PENDING"
	event.ReviewedBy = reviewedBy
	event.RejectReason = rejectReason

	return p.publisher.Publish(ctx, event)
}

// PublishVendorSuspended publishes a vendor suspended event
func (p *Publisher) PublishVendorSuspended(ctx context.Context, tenantID, vendorID, vendorName, vendorEmail, reason, reviewedBy string) error {
	event := events.NewVendorEvent(events.VendorSuspended, tenantID)
	event.VendorID = vendorID
	event.VendorName = vendorName
	event.VendorEmail = vendorEmail
	event.Status = "SUSPENDED"
	event.StatusReason = reason
	event.ReviewedBy = reviewedBy

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
