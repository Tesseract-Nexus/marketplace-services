package subscribers

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	gosharedevents "github.com/Tesseract-Nexus/go-shared/events"
	"github.com/Tesseract-Nexus/go-shared/security"
	"gorm.io/gorm"
	"payment-service/internal/models"
)

// PaymentConfigSubscriber handles incoming payment config events from orders-service
// and syncs them to payment-service's payment_gateway_configs table
type PaymentConfigSubscriber struct {
	subscriber *gosharedevents.Subscriber
	db         *gorm.DB
	logger     *logrus.Entry
	cancel     context.CancelFunc
}

// NewPaymentConfigSubscriber creates a new payment config event subscriber
func NewPaymentConfigSubscriber(
	db *gorm.DB,
	logger *logrus.Logger,
) (*PaymentConfigSubscriber, error) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://nats.nats.svc.cluster.local:4222"
	}

	config := gosharedevents.DefaultSubscriberConfig(natsURL, "payment-service-config-sync")
	config.Name = "payment-service-config-subscriber"
	config.DeliverPolicy = "new"
	config.MaxDeliver = 5 // Retry up to 5 times
	config.AckWait = 30 * time.Second

	subscriber, err := gosharedevents.NewSubscriber(config, logger)
	if err != nil {
		return nil, err
	}

	return &PaymentConfigSubscriber{
		subscriber: subscriber,
		db:         db,
		logger:     logger.WithField("component", "payment-config-subscriber"),
	}, nil
}

// Start starts listening for payment config events
func (s *PaymentConfigSubscriber) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	// Subscribe to all payment_config events
	subjects := []string{
		gosharedevents.PaymentConfigUpdated,
		gosharedevents.PaymentConfigEnabled,
		gosharedevents.PaymentConfigDisabled,
		gosharedevents.PaymentConfigTested,
	}

	s.logger.Info("Starting payment config event subscription...")

	// Subscribe to events
	// Note: The stream is created by orders-service publisher
	err := s.subscriber.Subscribe(ctx, gosharedevents.StreamPaymentConfigs, subjects, s.handlePaymentConfigMessage)
	if err != nil {
		return err
	}

	s.logger.WithField("subjects", subjects).Info("Payment config subscriber started successfully")
	return nil
}

// handlePaymentConfigMessage processes payment config messages from NATS
func (s *PaymentConfigSubscriber) handlePaymentConfigMessage(ctx context.Context, msg *gosharedevents.Message) error {
	// Parse the event from message data
	var event gosharedevents.PaymentConfigEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		s.logger.WithError(err).Error("Failed to unmarshal payment config event")
		return nil // Don't retry for invalid data
	}

	s.logger.WithFields(logrus.Fields{
		"event_type":          event.EventType,
		"tenant_id":           event.TenantID,
		"payment_method_code": event.PaymentMethodCode,
		"gateway_type":        event.GatewayType,
		"is_enabled":          event.IsEnabled,
	}).Info("Received payment config event")

	// Handle different event types
	switch event.EventType {
	case gosharedevents.PaymentConfigUpdated, gosharedevents.PaymentConfigEnabled:
		return s.syncPaymentConfig(ctx, &event)
	case gosharedevents.PaymentConfigDisabled:
		return s.disablePaymentConfig(ctx, &event)
	case gosharedevents.PaymentConfigTested:
		// Just log test results, no sync needed
		s.logger.WithFields(logrus.Fields{
			"success": event.TestSuccess,
			"message": event.TestMessage,
		}).Info("Payment config test result received")
		return nil
	default:
		s.logger.WithField("event_type", event.EventType).Warn("Unknown payment config event type")
		return nil
	}
}

// syncPaymentConfig syncs or creates a payment gateway config from the event
func (s *PaymentConfigSubscriber) syncPaymentConfig(ctx context.Context, event *gosharedevents.PaymentConfigEvent) error {
	tenantID := event.TenantID
	gatewayType := models.GatewayType(event.GatewayType)

	s.logger.WithFields(logrus.Fields{
		"tenant_id":    tenantID,
		"gateway_type": gatewayType,
	}).Info("Syncing payment config to gateway configs")

	// Find existing config or create new one
	var existingConfig models.PaymentGatewayConfig
	err := s.db.Where("tenant_id = ? AND gateway_type = ?", tenantID, gatewayType).First(&existingConfig).Error

	if err == gorm.ErrRecordNotFound {
		// Create new config
		return s.createGatewayConfig(ctx, event)
	} else if err != nil {
		s.logger.WithError(err).Error("Failed to check for existing gateway config")
		return err
	}

	// Update existing config
	return s.updateGatewayConfig(ctx, &existingConfig, event)
}

// createGatewayConfig creates a new gateway config from the event
func (s *PaymentConfigSubscriber) createGatewayConfig(ctx context.Context, event *gosharedevents.PaymentConfigEvent) error {
	config := models.PaymentGatewayConfig{
		ID:          uuid.New(),
		TenantID:    event.TenantID,
		GatewayType: models.GatewayType(event.GatewayType),
		DisplayName: event.PaymentMethodCode,
		IsEnabled:   event.IsEnabled,
		IsTestMode:  event.IsTestMode,
	}

	// Decrypt and extract credentials if present
	if event.CredentialsEncrypted != "" {
		if err := s.decryptAndApplyCredentials(&config, event.CredentialsEncrypted); err != nil {
			s.logger.WithError(err).Error("Failed to decrypt credentials for new config")
			// Continue without credentials - they may be added later
		}
	}

	if err := s.db.Create(&config).Error; err != nil {
		s.logger.WithError(err).Error("Failed to create gateway config")
		return err
	}

	s.logger.WithFields(logrus.Fields{
		"config_id":    config.ID,
		"tenant_id":    config.TenantID,
		"gateway_type": config.GatewayType,
	}).Info("Created new gateway config from payment config event")

	return nil
}

// updateGatewayConfig updates an existing gateway config from the event
func (s *PaymentConfigSubscriber) updateGatewayConfig(ctx context.Context, config *models.PaymentGatewayConfig, event *gosharedevents.PaymentConfigEvent) error {
	// Update fields
	config.IsEnabled = event.IsEnabled
	config.IsTestMode = event.IsTestMode

	// Decrypt and apply credentials if present
	if event.CredentialsEncrypted != "" {
		if err := s.decryptAndApplyCredentials(config, event.CredentialsEncrypted); err != nil {
			s.logger.WithError(err).Warn("Failed to decrypt credentials for update")
			// Continue with other updates
		}
	}

	if err := s.db.Save(config).Error; err != nil {
		s.logger.WithError(err).Error("Failed to update gateway config")
		return err
	}

	s.logger.WithFields(logrus.Fields{
		"config_id":    config.ID,
		"tenant_id":    config.TenantID,
		"gateway_type": config.GatewayType,
	}).Info("Updated gateway config from payment config event")

	return nil
}

// disablePaymentConfig disables a gateway config
func (s *PaymentConfigSubscriber) disablePaymentConfig(ctx context.Context, event *gosharedevents.PaymentConfigEvent) error {
	tenantID := event.TenantID
	gatewayType := models.GatewayType(event.GatewayType)

	result := s.db.Model(&models.PaymentGatewayConfig{}).
		Where("tenant_id = ? AND gateway_type = ?", tenantID, gatewayType).
		Update("is_enabled", false)

	if result.Error != nil {
		s.logger.WithError(result.Error).Error("Failed to disable gateway config")
		return result.Error
	}

	if result.RowsAffected == 0 {
		s.logger.WithFields(logrus.Fields{
			"tenant_id":    tenantID,
			"gateway_type": gatewayType,
		}).Debug("No gateway config found to disable")
	} else {
		s.logger.WithFields(logrus.Fields{
			"tenant_id":    tenantID,
			"gateway_type": gatewayType,
		}).Info("Disabled gateway config from payment config event")
	}

	return nil
}

// decryptAndApplyCredentials decrypts credentials and applies them to the config
func (s *PaymentConfigSubscriber) decryptAndApplyCredentials(config *models.PaymentGatewayConfig, encryptedCreds string) error {
	// Decrypt credentials using go-shared security package
	decrypted, err := security.DecryptPII(encryptedCreds)
	if err != nil {
		return err
	}

	// Parse credentials JSON
	// The format matches orders-service PaymentCredentials structure
	type Credentials struct {
		StripeSecretKey      string `json:"stripeSecretKey,omitempty"`
		StripePublishableKey string `json:"stripePublishableKey,omitempty"`
		StripeWebhookSecret  string `json:"stripeWebhookSecret,omitempty"`
		PayPalClientID       string `json:"paypalClientId,omitempty"`
		PayPalClientSecret   string `json:"paypalClientSecret,omitempty"`
		RazorpayKeyID        string `json:"razorpayKeyId,omitempty"`
		RazorpayKeySecret    string `json:"razorpayKeySecret,omitempty"`
		RazorpayWebhookSecret string `json:"razorpayWebhookSecret,omitempty"`
		AfterpayMerchantID   string `json:"afterpayMerchantId,omitempty"`
		AfterpaySecretKey    string `json:"afterpaySecretKey,omitempty"`
		ZipMerchantID        string `json:"zipMerchantId,omitempty"`
		ZipAPIKey            string `json:"zipApiKey,omitempty"`
	}

	var creds Credentials
	if err := json.Unmarshal([]byte(decrypted), &creds); err != nil {
		return err
	}

	// Apply credentials based on gateway type
	switch config.GatewayType {
	case models.GatewayStripe:
		config.APIKeyPublic = creds.StripePublishableKey
		config.APIKeySecret = creds.StripeSecretKey
		config.WebhookSecret = creds.StripeWebhookSecret
	case models.GatewayPayPal:
		config.APIKeyPublic = creds.PayPalClientID
		config.APIKeySecret = creds.PayPalClientSecret
	case models.GatewayRazorpay:
		config.APIKeyPublic = creds.RazorpayKeyID
		config.APIKeySecret = creds.RazorpayKeySecret
		config.WebhookSecret = creds.RazorpayWebhookSecret
	case models.GatewayAfterpay:
		config.APIKeyPublic = creds.AfterpayMerchantID
		config.APIKeySecret = creds.AfterpaySecretKey
	case models.GatewayZip:
		config.APIKeyPublic = creds.ZipMerchantID
		config.APIKeySecret = creds.ZipAPIKey
	}

	return nil
}

// Stop stops the payment config subscriber
func (s *PaymentConfigSubscriber) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.subscriber != nil {
		s.subscriber.Close()
	}
	s.logger.Info("Payment config subscriber stopped")
}
