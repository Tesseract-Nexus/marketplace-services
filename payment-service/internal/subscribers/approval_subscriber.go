package subscribers

import (
	"context"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	gosharedevents "github.com/Tesseract-Nexus/go-shared/events"
	"payment-service/internal/models"
)

// GatewayConfigExecutor defines the interface for executing gateway config operations
// This avoids import cycle with handlers package
type GatewayConfigExecutor interface {
	CreateGatewayConfig(ctx context.Context, config *models.PaymentGatewayConfig) error
	UpdateGatewayConfig(ctx context.Context, config *models.PaymentGatewayConfig) error
	DeleteGatewayConfig(ctx context.Context, id uuid.UUID) error
	GetGatewayConfig(ctx context.Context, id uuid.UUID) (*models.PaymentGatewayConfig, error)
}

// ApprovalSubscriber handles incoming approval events for gateway configs
type ApprovalSubscriber struct {
	subscriber        *gosharedevents.Subscriber
	gatewayExecutor   GatewayConfigExecutor
	logger            *logrus.Entry
	cancel            context.CancelFunc
}

// NewApprovalSubscriber creates a new approval event subscriber for payment-service
func NewApprovalSubscriber(
	gatewayExecutor GatewayConfigExecutor,
	logger *logrus.Logger,
) (*ApprovalSubscriber, error) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats.nats.svc.cluster.local:4222"
	}

	config := gosharedevents.DefaultSubscriberConfig(natsURL, "payment-service-approvals")
	config.Name = "payment-service-approval-subscriber"
	config.DeliverPolicy = "new"
	config.MaxDeliver = 3
	config.AckWait = 30 * time.Second

	subscriber, err := gosharedevents.NewSubscriber(config, logger)
	if err != nil {
		return nil, err
	}

	return &ApprovalSubscriber{
		subscriber:      subscriber,
		gatewayExecutor: gatewayExecutor,
		logger:          logger.WithField("component", "approval-subscriber"),
	}, nil
}

// Start starts listening for approval events
func (s *ApprovalSubscriber) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	// Subscribe to approval.granted events for gateway configs
	subjects := []string{gosharedevents.ApprovalGranted}

	s.logger.Info("Starting approval event subscription for gateway configs...")

	err := s.subscriber.SubscribeApprovalEvents(ctx, subjects, s.handleApprovalEvent)
	if err != nil {
		return err
	}

	s.logger.WithField("subjects", subjects).Info("Approval subscriber started successfully")
	return nil
}

// handleApprovalEvent processes approval events
func (s *ApprovalSubscriber) handleApprovalEvent(ctx context.Context, event *gosharedevents.ApprovalEvent) error {
	s.logger.WithFields(logrus.Fields{
		"event_type":    event.EventType,
		"approval_id":   event.ApprovalRequestID,
		"action_type":   event.ActionType,
		"resource_type": event.ResourceType,
		"status":        event.Status,
	}).Info("Received approval event")

	// Only process events for gateway_config resources
	if event.ResourceType != "gateway_config" {
		s.logger.WithField("resource_type", event.ResourceType).Debug("Ignoring non-gateway_config approval event")
		return nil
	}

	// Only process granted/approved events
	if event.Status != "approved" {
		s.logger.WithField("status", event.Status).Debug("Ignoring non-approved event")
		return nil
	}

	tenantID := event.TenantID

	// Execute the approved action based on action type
	switch event.ActionType {
	case "gateway_config_create":
		return s.executeCreate(ctx, event, tenantID)
	case "gateway_config_update":
		return s.executeUpdate(ctx, event, tenantID)
	case "gateway_config_delete":
		return s.executeDelete(ctx, event, tenantID)
	default:
		s.logger.WithField("action_type", event.ActionType).Warn("Unknown action type for gateway config")
		return nil
	}
}

// executeCreate executes an approved gateway config creation
func (s *ApprovalSubscriber) executeCreate(ctx context.Context, event *gosharedevents.ApprovalEvent, tenantID string) error {
	s.logger.WithFields(logrus.Fields{
		"resource_id": event.ResourceID,
		"approver":    event.ApproverName,
	}).Info("Executing approved gateway config creation")

	// Extract config from action data
	configData, ok := event.ActionData["config_json"].(map[string]interface{})
	if !ok {
		s.logger.Error("Invalid config_json in action data")
		return nil // Don't retry for invalid data
	}

	// Parse gateway type
	gatewayType, _ := configData["gateway_type"].(string)
	displayName, _ := configData["display_name"].(string)
	isEnabled, _ := configData["is_enabled"].(bool)
	isTestMode, _ := configData["is_test_mode"].(bool)

	config := &models.PaymentGatewayConfig{
		ID:          uuid.New(),
		TenantID:    tenantID,
		GatewayType: models.GatewayType(gatewayType),
		DisplayName: displayName,
		IsEnabled:   isEnabled,
		IsTestMode:  isTestMode,
	}

	// Extract API credentials if present
	if apiKeyPublic, ok := configData["api_key_public"].(string); ok {
		config.APIKeyPublic = apiKeyPublic
	}
	if apiKeySecret, ok := configData["api_key_secret"].(string); ok {
		config.APIKeySecret = apiKeySecret
	}
	if webhookSecret, ok := configData["webhook_secret"].(string); ok {
		config.WebhookSecret = webhookSecret
	}

	// Extract additional config if present
	if additionalConfig, ok := configData["config"].(map[string]interface{}); ok {
		config.Config = models.JSONB(additionalConfig)
	}

	// Execute the creation
	err := s.gatewayExecutor.CreateGatewayConfig(ctx, config)
	if err != nil {
		s.logger.WithError(err).Error("Failed to execute approved gateway config creation")
		return err
	}

	s.logger.WithField("config_id", config.ID).Info("Approved gateway config creation executed successfully")
	return nil
}

// executeUpdate executes an approved gateway config update
func (s *ApprovalSubscriber) executeUpdate(ctx context.Context, event *gosharedevents.ApprovalEvent, tenantID string) error {
	s.logger.WithFields(logrus.Fields{
		"resource_id": event.ResourceID,
		"approver":    event.ApproverName,
	}).Info("Executing approved gateway config update")

	// Parse config ID
	configID, err := uuid.Parse(event.ResourceID)
	if err != nil {
		s.logger.WithError(err).Error("Invalid config ID in approval event")
		return nil // Don't retry for invalid IDs
	}

	// Get existing config
	existingConfig, err := s.gatewayExecutor.GetGatewayConfig(ctx, configID)
	if err != nil {
		s.logger.WithError(err).Error("Failed to get existing config for update")
		return nil // Don't retry if config doesn't exist
	}

	// Extract updated values from action data
	configData, ok := event.ActionData["config_json"].(map[string]interface{})
	if !ok {
		s.logger.Error("Invalid config_json in action data")
		return nil
	}

	// Update fields that are present in the action data
	if displayName, ok := configData["display_name"].(string); ok {
		existingConfig.DisplayName = displayName
	}
	if isEnabled, ok := configData["is_enabled"].(bool); ok {
		existingConfig.IsEnabled = isEnabled
	}
	if isTestMode, ok := configData["is_test_mode"].(bool); ok {
		existingConfig.IsTestMode = isTestMode
	}

	// Update API credentials if present
	if apiKeyPublic, ok := configData["api_key_public"].(string); ok {
		existingConfig.APIKeyPublic = apiKeyPublic
	}
	if apiKeySecret, ok := configData["api_key_secret"].(string); ok {
		existingConfig.APIKeySecret = apiKeySecret
	}
	if webhookSecret, ok := configData["webhook_secret"].(string); ok {
		existingConfig.WebhookSecret = webhookSecret
	}

	// Update additional config if present
	if additionalConfig, ok := configData["config"].(map[string]interface{}); ok {
		existingConfig.Config = models.JSONB(additionalConfig)
	}

	// Execute the update
	err = s.gatewayExecutor.UpdateGatewayConfig(ctx, existingConfig)
	if err != nil {
		s.logger.WithError(err).Error("Failed to execute approved gateway config update")
		return err
	}

	s.logger.Info("Approved gateway config update executed successfully")
	return nil
}

// executeDelete executes an approved gateway config deletion
func (s *ApprovalSubscriber) executeDelete(ctx context.Context, event *gosharedevents.ApprovalEvent, tenantID string) error {
	s.logger.WithFields(logrus.Fields{
		"resource_id": event.ResourceID,
		"approver":    event.ApproverName,
	}).Info("Executing approved gateway config deletion")

	// Parse config ID
	configID, err := uuid.Parse(event.ResourceID)
	if err != nil {
		s.logger.WithError(err).Error("Invalid config ID in approval event")
		return nil // Don't retry for invalid IDs
	}

	// Execute the deletion
	err = s.gatewayExecutor.DeleteGatewayConfig(ctx, configID)
	if err != nil {
		s.logger.WithError(err).Error("Failed to execute approved gateway config deletion")
		return err
	}

	s.logger.Info("Approved gateway config deletion executed successfully")
	return nil
}

// Stop stops the approval subscriber
func (s *ApprovalSubscriber) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.subscriber != nil {
		s.subscriber.Close()
	}
	s.logger.Info("Approval subscriber stopped")
}
