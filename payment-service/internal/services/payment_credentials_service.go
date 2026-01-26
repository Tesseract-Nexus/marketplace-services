package services

import (
	"context"
	"fmt"
	"sync"

	"github.com/Tesseract-Nexus/go-shared/secrets"
	"github.com/sirupsen/logrus"
	"payment-service/internal/clients"
)

// PaymentCredentialsService manages payment provider credentials.
// It uses:
// - PaymentSecretClient (from go-shared) for reading secret VALUES from GCP
// - SecretProvisionerClient for checking/provisioning secrets via the service API
type PaymentCredentialsService struct {
	secretClient      *secrets.PaymentSecretClient
	provisionerClient *clients.SecretProvisionerClient
	environment       string
	gcpProjectID      string
	logger            *logrus.Entry
	mu                sync.RWMutex
}

// PaymentCredentialsConfig configures the payment credentials service
type PaymentCredentialsConfig struct {
	GCPProjectID              string
	Environment               string // "devtest" or "prod"
	SecretProvisionerURL      string
	Logger                    *logrus.Entry
}

// ProviderCredentials holds all credentials for a payment provider
type ProviderCredentials struct {
	Provider      string
	APIKey        string
	APISecret     string
	WebhookSecret string
	// Provider-specific fields
	Extra map[string]string
}

// NewPaymentCredentialsService creates a new payment credentials service
func NewPaymentCredentialsService(ctx context.Context, config PaymentCredentialsConfig) (*PaymentCredentialsService, error) {
	logger := config.Logger
	if logger == nil {
		logger = logrus.NewEntry(logrus.StandardLogger())
	}

	// Initialize PaymentSecretClient for reading secret values from GCP
	secretClient, err := secrets.NewPaymentSecretClient(ctx, secrets.PaymentSecretClientConfig{
		ProjectID: config.GCPProjectID,
		Logger:    logger.WithField("component", "payment-secret-client"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create payment secret client: %w", err)
	}

	// Initialize SecretProvisionerClient for management operations
	provisionerClient := clients.NewSecretProvisionerClient(clients.SecretProvisionerConfig{
		BaseURL: config.SecretProvisionerURL,
		Logger:  logger.WithField("component", "secret-provisioner-client"),
	})

	return &PaymentCredentialsService{
		secretClient:      secretClient,
		provisionerClient: provisionerClient,
		environment:       config.Environment,
		gcpProjectID:      config.GCPProjectID,
		logger:            logger,
	}, nil
}

// GetStripeCredentials retrieves Stripe credentials for a tenant/vendor.
// Uses vendor-first precedence: if vendorID provided, tries vendor-level first,
// then falls back to tenant-level.
func (s *PaymentCredentialsService) GetStripeCredentials(
	ctx context.Context,
	tenantID, vendorID string,
) (*ProviderCredentials, error) {
	creds := &ProviderCredentials{
		Provider: "stripe",
		Extra:    make(map[string]string),
	}

	// Get API key (required)
	apiKey, err := s.secretClient.GetPaymentSecretWithFallback(
		ctx, s.environment, tenantID, vendorID,
		secrets.PaymentProviderStripe, secrets.KeyStripeAPIKey,
	)
	if err != nil {
		return nil, fmt.Errorf("stripe API key not configured: %w", err)
	}
	creds.APIKey = apiKey

	// Get webhook secret (optional)
	webhookSecret, err := s.secretClient.GetPaymentSecretWithFallback(
		ctx, s.environment, tenantID, vendorID,
		secrets.PaymentProviderStripe, secrets.KeyStripeWebhookSecret,
	)
	if err == nil {
		creds.WebhookSecret = webhookSecret
	}

	// Get connected account ID if vendor-level (optional)
	if vendorID != "" {
		connectedID, err := s.secretClient.GetPaymentSecretWithFallback(
			ctx, s.environment, tenantID, vendorID,
			secrets.PaymentProviderStripe, secrets.KeyStripeConnectedID,
		)
		if err == nil {
			creds.Extra["connected_account_id"] = connectedID
		}
	}

	s.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"vendor_id": vendorID,
		"provider":  "stripe",
	}).Debug("retrieved stripe credentials")

	return creds, nil
}

// GetRazorpayCredentials retrieves Razorpay credentials for a tenant/vendor.
func (s *PaymentCredentialsService) GetRazorpayCredentials(
	ctx context.Context,
	tenantID, vendorID string,
) (*ProviderCredentials, error) {
	creds := &ProviderCredentials{
		Provider: "razorpay",
		Extra:    make(map[string]string),
	}

	// Get key ID (required)
	keyID, err := s.secretClient.GetPaymentSecretWithFallback(
		ctx, s.environment, tenantID, vendorID,
		secrets.PaymentProviderRazorpay, secrets.KeyRazorpayKeyID,
	)
	if err != nil {
		return nil, fmt.Errorf("razorpay key ID not configured: %w", err)
	}
	creds.APIKey = keyID

	// Get key secret (required)
	keySecret, err := s.secretClient.GetPaymentSecretWithFallback(
		ctx, s.environment, tenantID, vendorID,
		secrets.PaymentProviderRazorpay, secrets.KeyRazorpayKeySecret,
	)
	if err != nil {
		return nil, fmt.Errorf("razorpay key secret not configured: %w", err)
	}
	creds.APISecret = keySecret

	// Get webhook secret (optional)
	webhookSecret, err := s.secretClient.GetPaymentSecretWithFallback(
		ctx, s.environment, tenantID, vendorID,
		secrets.PaymentProviderRazorpay, secrets.KeyRazorpayWebhook,
	)
	if err == nil {
		creds.WebhookSecret = webhookSecret
	}

	s.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"vendor_id": vendorID,
		"provider":  "razorpay",
	}).Debug("retrieved razorpay credentials")

	return creds, nil
}

// GetCredentials retrieves credentials for any supported provider.
func (s *PaymentCredentialsService) GetCredentials(
	ctx context.Context,
	tenantID, vendorID string,
	provider secrets.PaymentProvider,
) (*ProviderCredentials, error) {
	switch provider {
	case secrets.PaymentProviderStripe:
		return s.GetStripeCredentials(ctx, tenantID, vendorID)
	case secrets.PaymentProviderRazorpay:
		return s.GetRazorpayCredentials(ctx, tenantID, vendorID)
	default:
		return nil, fmt.Errorf("unsupported payment provider: %s", provider)
	}
}

// IsProviderConfigured checks if a provider is configured for a tenant/vendor.
func (s *PaymentCredentialsService) IsProviderConfigured(
	ctx context.Context,
	tenantID, actorID string,
	provider string,
	vendorID string,
) (bool, error) {
	return s.provisionerClient.IsProviderConfigured(ctx, tenantID, actorID, provider, vendorID)
}

// ListConfiguredProviders returns a list of configured payment providers for a tenant.
func (s *PaymentCredentialsService) ListConfiguredProviders(
	ctx context.Context,
	tenantID, actorID string,
) (*clients.ListProvidersResponse, error) {
	return s.provisionerClient.ListProviders(ctx, tenantID, actorID, "payment")
}

// ProvisionCredentials provisions new payment credentials via the secret-provisioner.
// This should be called from admin endpoints, not during payment processing.
func (s *PaymentCredentialsService) ProvisionCredentials(
	ctx context.Context,
	tenantID, actorID string,
	provider string,
	vendorID string,
	credentials map[string]string,
	validate bool,
) (*clients.ProvisionSecretsResponse, error) {
	scope := "tenant"
	if vendorID != "" {
		scope = "vendor"
	}

	req := clients.ProvisionSecretsRequest{
		TenantID: tenantID,
		Category: "payment",
		Scope:    scope,
		ScopeID:  vendorID,
		Provider: provider,
		Secrets:  credentials,
		Validate: validate,
	}

	resp, err := s.provisionerClient.ProvisionSecrets(ctx, tenantID, actorID, req)
	if err != nil {
		return nil, err
	}

	// Invalidate cache for the provisioned secrets
	s.invalidateCacheForProvider(tenantID, vendorID, secrets.PaymentProvider(provider))

	return resp, nil
}

// invalidateCacheForProvider invalidates cached secrets for a provider
func (s *PaymentCredentialsService) invalidateCacheForProvider(
	tenantID, vendorID string,
	provider secrets.PaymentProvider,
) {
	allKeys := secrets.GetAllPaymentProviderKeys(provider)
	for _, keyName := range allKeys {
		s.secretClient.InvalidateCache(s.environment, tenantID, vendorID, provider, keyName)
	}
}

// InvalidateCache invalidates all cached secrets
func (s *PaymentCredentialsService) InvalidateCache() {
	s.secretClient.InvalidateAllCache()
}

// Close closes the underlying clients
func (s *PaymentCredentialsService) Close() error {
	return s.secretClient.Close()
}

// GetEnvironment returns the configured environment
func (s *PaymentCredentialsService) GetEnvironment() string {
	return s.environment
}
