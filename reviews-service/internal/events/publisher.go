package events

import (
	"context"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/Tesseract-Nexus/go-shared/events"
)

// Publisher wraps the shared events publisher for review-specific events
type Publisher struct {
	publisher *events.Publisher
	logger    *logrus.Entry
}

// NewPublisher creates a new review events publisher
func NewPublisher(logger *logrus.Logger) (*Publisher, error) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats.nats.svc.cluster.local:4222"
	}

	config := events.DefaultPublisherConfig(natsURL)
	config.Name = "reviews-service"

	publisher, err := events.NewPublisher(config, logger)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	if err := publisher.EnsureStream(ctx, events.StreamReviews, []string{"review.>"}); err != nil {
		logger.WithError(err).Warn("Failed to ensure REVIEW_EVENTS stream")
	}

	return &Publisher{
		publisher: publisher,
		logger:    logger.WithField("component", "events.publisher"),
	}, nil
}

// PublishReviewCreated publishes a review created event
func (p *Publisher) PublishReviewCreated(ctx context.Context, tenantID, reviewID, productID, productName, customerID, customerName string, rating int, content string, verified bool) error {
	event := events.NewReviewEvent(events.ReviewCreated, tenantID)
	event.ReviewID = reviewID
	event.ProductID = productID
	event.ProductName = productName
	event.CustomerID = customerID
	event.CustomerName = customerName
	event.Rating = rating
	event.Content = content
	event.Verified = verified
	event.Status = "pending"

	return p.publisher.PublishReview(ctx, event)
}

// PublishReviewApproved publishes a review approved event
func (p *Publisher) PublishReviewApproved(ctx context.Context, tenantID, reviewID, productID, productName, customerName, moderatedBy string, rating int) error {
	event := events.NewReviewEvent(events.ReviewApproved, tenantID)
	event.ReviewID = reviewID
	event.ProductID = productID
	event.ProductName = productName
	event.CustomerName = customerName
	event.Rating = rating
	event.ModeratedBy = moderatedBy
	event.Status = "approved"

	return p.publisher.PublishReview(ctx, event)
}

// PublishReviewRejected publishes a review rejected event
func (p *Publisher) PublishReviewRejected(ctx context.Context, tenantID, reviewID, productID, productName, customerName, moderatedBy, rejectReason string, rating int) error {
	event := events.NewReviewEvent(events.ReviewRejected, tenantID)
	event.ReviewID = reviewID
	event.ProductID = productID
	event.ProductName = productName
	event.CustomerName = customerName
	event.Rating = rating
	event.ModeratedBy = moderatedBy
	event.RejectReason = rejectReason
	event.Status = "rejected"

	return p.publisher.PublishReview(ctx, event)
}

// IsConnected returns true if connected to NATS
func (p *Publisher) IsConnected() bool {
	return p.publisher.IsConnected()
}

// Close closes the publisher connection
func (p *Publisher) Close() {
	p.publisher.Close()
}
