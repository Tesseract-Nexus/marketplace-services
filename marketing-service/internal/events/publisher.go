package events

import (
	"context"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/Tesseract-Nexus/go-shared/events"
)

// Publisher wraps the shared events publisher for marketing-service events
type Publisher struct {
	publisher *events.Publisher
	logger    *logrus.Entry
}

// NewPublisher creates a new marketing events publisher
func NewPublisher(logger *logrus.Logger) (*Publisher, error) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats.nats.svc.cluster.local:4222"
	}

	config := events.DefaultPublisherConfig(natsURL)
	config.Name = "marketing-service"

	publisher, err := events.NewPublisher(config, logger)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	if err := publisher.EnsureStream(ctx, events.StreamCampaigns, []string{"campaign.>"}); err != nil {
		logger.WithError(err).Warn("Failed to ensure CAMPAIGN_EVENTS stream")
	}
	if err := publisher.EnsureStream(ctx, events.StreamLoyalty, []string{"loyalty.>"}); err != nil {
		logger.WithError(err).Warn("Failed to ensure LOYALTY_EVENTS stream")
	}
	if err := publisher.EnsureStream(ctx, events.StreamCoupons, []string{"coupon.>"}); err != nil {
		logger.WithError(err).Warn("Failed to ensure COUPON_EVENTS stream")
	}

	return &Publisher{
		publisher: publisher,
		logger:    logger.WithField("component", "events.publisher"),
	}, nil
}

// ===== CAMPAIGN EVENTS =====

// PublishCampaignCreated publishes a campaign created event
func (p *Publisher) PublishCampaignCreated(ctx context.Context, tenantID, campaignID, campaignName, campaignType, channel, actorID string) error {
	event := events.NewCampaignEvent(events.CampaignCreated, tenantID)
	event.CampaignID = campaignID
	event.CampaignName = campaignName
	event.CampaignType = campaignType
	event.Channel = channel
	event.Status = "DRAFT"
	event.ActorID = actorID
	return p.publisher.Publish(ctx, event)
}

// PublishCampaignUpdated publishes a campaign updated event
func (p *Publisher) PublishCampaignUpdated(ctx context.Context, tenantID, campaignID, campaignName, status, actorID string) error {
	event := events.NewCampaignEvent(events.CampaignUpdated, tenantID)
	event.CampaignID = campaignID
	event.CampaignName = campaignName
	event.Status = status
	event.ActorID = actorID
	return p.publisher.Publish(ctx, event)
}

// PublishCampaignSent publishes a campaign sent event
func (p *Publisher) PublishCampaignSent(ctx context.Context, tenantID, campaignID, campaignName, campaignType, channel string, totalRecipients int, actorID string) error {
	event := events.NewCampaignEvent(events.CampaignSent, tenantID)
	event.CampaignID = campaignID
	event.CampaignName = campaignName
	event.CampaignType = campaignType
	event.Channel = channel
	event.Status = "SENT"
	event.TotalRecipients = totalRecipients
	event.ActorID = actorID
	return p.publisher.Publish(ctx, event)
}

// PublishCampaignScheduled publishes a campaign scheduled event
func (p *Publisher) PublishCampaignScheduled(ctx context.Context, tenantID, campaignID, campaignName, scheduledAt, actorID string) error {
	event := events.NewCampaignEvent(events.CampaignScheduled, tenantID)
	event.CampaignID = campaignID
	event.CampaignName = campaignName
	event.Status = "SCHEDULED"
	event.ScheduledAt = scheduledAt
	event.ActorID = actorID
	return p.publisher.Publish(ctx, event)
}

// PublishCampaignDeleted publishes a campaign deleted event
func (p *Publisher) PublishCampaignDeleted(ctx context.Context, tenantID, campaignID, campaignName, actorID string) error {
	event := events.NewCampaignEvent(events.CampaignDeleted, tenantID)
	event.CampaignID = campaignID
	event.CampaignName = campaignName
	event.Status = "DELETED"
	event.ActorID = actorID
	return p.publisher.Publish(ctx, event)
}

// ===== LOYALTY EVENTS =====

// PublishLoyaltyProgramCreated publishes a loyalty program created event
func (p *Publisher) PublishLoyaltyProgramCreated(ctx context.Context, tenantID, programID, programName string) error {
	event := events.NewLoyaltyEvent(events.LoyaltyProgramCreated, tenantID)
	event.ProgramID = programID
	event.ProgramName = programName
	return p.publisher.Publish(ctx, event)
}

// PublishLoyaltyProgramUpdated publishes a loyalty program updated event
func (p *Publisher) PublishLoyaltyProgramUpdated(ctx context.Context, tenantID, programID, programName string) error {
	event := events.NewLoyaltyEvent(events.LoyaltyProgramUpdated, tenantID)
	event.ProgramID = programID
	event.ProgramName = programName
	return p.publisher.Publish(ctx, event)
}

// PublishCustomerEnrolled publishes a customer enrolled in loyalty event
func (p *Publisher) PublishCustomerEnrolled(ctx context.Context, tenantID, programID, programName, customerID string) error {
	event := events.NewLoyaltyEvent(events.LoyaltyCustomerEnrolled, tenantID)
	event.ProgramID = programID
	event.ProgramName = programName
	event.CustomerID = customerID
	return p.publisher.Publish(ctx, event)
}

// PublishPointsRedeemed publishes a points redeemed event
func (p *Publisher) PublishPointsRedeemed(ctx context.Context, tenantID, customerID string, points int, reason string) error {
	event := events.NewLoyaltyEvent(events.LoyaltyPointsRedeemed, tenantID)
	event.CustomerID = customerID
	event.Points = points
	event.Reason = reason
	return p.publisher.Publish(ctx, event)
}

// ===== COUPON EVENTS =====

// PublishCouponCreated publishes a coupon created event
func (p *Publisher) PublishCouponCreated(ctx context.Context, tenantID, couponID, couponCode, discountType string, discountValue float64, actorID string) error {
	event := events.NewCouponEvent(events.CouponCreated, tenantID)
	event.CouponID = couponID
	event.CouponCode = couponCode
	event.DiscountType = discountType
	event.DiscountValue = discountValue
	return p.publisher.Publish(ctx, event)
}

// PublishCouponUpdated publishes a coupon updated event
func (p *Publisher) PublishCouponUpdated(ctx context.Context, tenantID, couponID, couponCode, status string) error {
	event := events.NewCouponEvent(events.CouponUpdated, tenantID)
	event.CouponID = couponID
	event.CouponCode = couponCode
	event.Status = status
	return p.publisher.Publish(ctx, event)
}

// PublishCouponDeleted publishes a coupon deleted event
func (p *Publisher) PublishCouponDeleted(ctx context.Context, tenantID, couponID, couponCode string) error {
	event := events.NewCouponEvent(events.CouponDeleted, tenantID)
	event.CouponID = couponID
	event.CouponCode = couponCode
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
