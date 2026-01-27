package services

import (
	"context"
	"fmt"
	"sync"

	"github.com/Tesseract-Nexus/go-shared/secrets"
	"github.com/sirupsen/logrus"
	"shipping-service/internal/clients"
)

// ShippingCredentialsService manages shipping carrier credentials.
// It uses:
// - ShippingSecretClient (from go-shared) for reading secret VALUES from GCP
// - SecretProvisionerClient for checking/provisioning secrets via the service API
type ShippingCredentialsService struct {
	secretClient      *secrets.ShippingSecretClient
	provisionerClient *clients.SecretProvisionerClient
	environment       string
	gcpProjectID      string
	logger            *logrus.Entry
	mu                sync.RWMutex
}

// ShippingCredentialsConfig configures the shipping credentials service
type ShippingCredentialsConfig struct {
	GCPProjectID         string
	Environment          string // "devtest" or "prod"
	SecretProvisionerURL string
	Logger               *logrus.Entry
}

// CarrierCredentials holds all credentials for a shipping carrier
type CarrierCredentials struct {
	Provider  string
	APIKey    string
	APISecret string
	Extra     map[string]string
}

// NewShippingCredentialsService creates a new shipping credentials service
func NewShippingCredentialsService(ctx context.Context, config ShippingCredentialsConfig) (*ShippingCredentialsService, error) {
	logger := config.Logger
	if logger == nil {
		logger = logrus.NewEntry(logrus.StandardLogger())
	}

	// Initialize ShippingSecretClient for reading secret values from GCP
	secretClient, err := secrets.NewShippingSecretClient(ctx, secrets.ShippingSecretClientConfig{
		ProjectID: config.GCPProjectID,
		Logger:    logger.WithField("component", "shipping-secret-client"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create shipping secret client: %w", err)
	}

	// Initialize SecretProvisionerClient for management operations
	provisionerClient := clients.NewSecretProvisionerClient(clients.SecretProvisionerConfig{
		BaseURL: config.SecretProvisionerURL,
		Logger:  logger.WithField("component", "secret-provisioner-client"),
	})

	return &ShippingCredentialsService{
		secretClient:      secretClient,
		provisionerClient: provisionerClient,
		environment:       config.Environment,
		gcpProjectID:      config.GCPProjectID,
		logger:            logger,
	}, nil
}

// GetShiprocketCredentials retrieves Shiprocket credentials for a tenant/vendor.
// Uses vendor-first precedence: if vendorID provided, tries vendor-level first,
// then falls back to tenant-level.
func (s *ShippingCredentialsService) GetShiprocketCredentials(
	ctx context.Context,
	tenantID, vendorID string,
) (*CarrierCredentials, error) {
	creds := &CarrierCredentials{
		Provider: "shiprocket",
		Extra:    make(map[string]string),
	}

	// Get api-email (required) -> APIKey
	apiEmail, err := s.secretClient.GetShippingSecretWithFallback(
		ctx, s.environment, tenantID, vendorID,
		secrets.ShippingProviderShiprocket, secrets.ShippingKeyAPIEmail,
	)
	if err != nil {
		return nil, fmt.Errorf("shiprocket api-email not configured: %w", err)
	}
	creds.APIKey = apiEmail

	// Get api-password (required) -> APISecret
	apiPassword, err := s.secretClient.GetShippingSecretWithFallback(
		ctx, s.environment, tenantID, vendorID,
		secrets.ShippingProviderShiprocket, secrets.ShippingKeyAPIPassword,
	)
	if err != nil {
		return nil, fmt.Errorf("shiprocket api-password not configured: %w", err)
	}
	creds.APISecret = apiPassword

	// Get webhook-secret (optional)
	webhookSecret, err := s.secretClient.GetShippingSecretWithFallback(
		ctx, s.environment, tenantID, vendorID,
		secrets.ShippingProviderShiprocket, secrets.ShippingKeyWebhookSecret,
	)
	if err == nil {
		creds.Extra["webhook_secret"] = webhookSecret
	}

	s.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"vendor_id": vendorID,
		"provider":  "shiprocket",
	}).Debug("retrieved shiprocket credentials")

	return creds, nil
}

// GetDelhiveryCredentials retrieves Delhivery credentials for a tenant/vendor.
func (s *ShippingCredentialsService) GetDelhiveryCredentials(
	ctx context.Context,
	tenantID, vendorID string,
) (*CarrierCredentials, error) {
	creds := &CarrierCredentials{
		Provider: "delhivery",
		Extra:    make(map[string]string),
	}

	// Get api-token (required) -> APIKey
	apiToken, err := s.secretClient.GetShippingSecretWithFallback(
		ctx, s.environment, tenantID, vendorID,
		secrets.ShippingProviderDelhivery, secrets.ShippingKeyAPIToken,
	)
	if err != nil {
		return nil, fmt.Errorf("delhivery api-token not configured: %w", err)
	}
	creds.APIKey = apiToken

	// Get pickup-location (optional) -> APISecret
	pickupLocation, err := s.secretClient.GetShippingSecretWithFallback(
		ctx, s.environment, tenantID, vendorID,
		secrets.ShippingProviderDelhivery, secrets.ShippingKeyPickupLocation,
	)
	if err == nil {
		creds.APISecret = pickupLocation
	}

	// Get webhook-secret (optional)
	webhookSecret, err := s.secretClient.GetShippingSecretWithFallback(
		ctx, s.environment, tenantID, vendorID,
		secrets.ShippingProviderDelhivery, secrets.ShippingKeyWebhookSecret,
	)
	if err == nil {
		creds.Extra["webhook_secret"] = webhookSecret
	}

	s.logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"vendor_id": vendorID,
		"provider":  "delhivery",
	}).Debug("retrieved delhivery credentials")

	return creds, nil
}

// GetCredentials retrieves credentials for any supported shipping provider.
func (s *ShippingCredentialsService) GetCredentials(
	ctx context.Context,
	tenantID, vendorID string,
	provider secrets.ShippingProvider,
) (*CarrierCredentials, error) {
	switch provider {
	case secrets.ShippingProviderShiprocket:
		return s.GetShiprocketCredentials(ctx, tenantID, vendorID)
	case secrets.ShippingProviderDelhivery:
		return s.GetDelhiveryCredentials(ctx, tenantID, vendorID)
	default:
		// For unknown/other providers, use dynamic credential fetching
		return s.GetDynamicCarrierCredentials(ctx, tenantID, vendorID, string(provider), nil)
	}
}

// GetDynamicCarrierCredentials retrieves credentials for any carrier using dynamic key names.
// This method is fully adaptable - it works with any shipping carrier by accepting
// the key names from the carrier configuration's required_credentials field.
//
// If keyNames is nil or empty, it attempts to use the known keys for the provider
// from the go-shared secrets package.
//
// Usage:
//
//	keyNames := []string{"api-key", "account-number"} // for FedEx
//	creds, err := service.GetDynamicCarrierCredentials(ctx, tenantID, vendorID, "fedex", keyNames)
func (s *ShippingCredentialsService) GetDynamicCarrierCredentials(
	ctx context.Context,
	tenantID, vendorID string,
	provider string,
	keyNames []string,
) (*CarrierCredentials, error) {
	creds := &CarrierCredentials{
		Provider: provider,
		Extra:    make(map[string]string),
	}

	// If no key names provided, try to derive them from known provider keys
	if len(keyNames) == 0 {
		typedProvider := secrets.ShippingProvider(provider)
		allKeys := secrets.GetAllShippingProviderKeys(typedProvider)
		for _, k := range allKeys {
			keyNames = append(keyNames, string(k))
		}
	}

	if len(keyNames) == 0 {
		return nil, fmt.Errorf("no key names provided and provider %s has no known keys", provider)
	}

	// Fetch credentials dynamically using the provided key names
	dynamicCreds, err := s.secretClient.GetDynamicCredentials(
		ctx, s.environment, tenantID, vendorID,
		provider, keyNames,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get %s credentials: %w", provider, err)
	}

	// Store all retrieved credentials in the Extra map
	for k, v := range dynamicCreds {
		creds.Extra[k] = v
	}

	// For backwards compatibility, try to populate the standard fields
	// if the carrier uses common credential names
	if v, ok := dynamicCreds["api-key"]; ok {
		creds.APIKey = v
	} else if v, ok := dynamicCreds["api-token"]; ok {
		creds.APIKey = v
	} else if v, ok := dynamicCreds["api-email"]; ok {
		creds.APIKey = v
	} else if v, ok := dynamicCreds["client-id"]; ok {
		creds.APIKey = v
	}

	if v, ok := dynamicCreds["api-secret"]; ok {
		creds.APISecret = v
	} else if v, ok := dynamicCreds["api-password"]; ok {
		creds.APISecret = v
	} else if v, ok := dynamicCreds["client-secret"]; ok {
		creds.APISecret = v
	} else if v, ok := dynamicCreds["license-key"]; ok {
		creds.APISecret = v
	}

	s.logger.WithFields(logrus.Fields{
		"tenant_id":  tenantID,
		"vendor_id":  vendorID,
		"provider":   provider,
		"keys_found": len(dynamicCreds),
	}).Debug("retrieved dynamic carrier credentials")

	return creds, nil
}

// ProvisionCredentials provisions new shipping credentials via the secret-provisioner.
// This should be called from admin endpoints, not during shipping operations.
func (s *ShippingCredentialsService) ProvisionCredentials(
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
		Category: "shipping",
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
	s.invalidateCacheForProvider(tenantID, vendorID, secrets.ShippingProvider(provider))

	return resp, nil
}

// IsCarrierConfigured checks if a carrier is configured for a tenant/vendor.
func (s *ShippingCredentialsService) IsCarrierConfigured(
	ctx context.Context,
	tenantID, actorID string,
	provider string,
	vendorID string,
) (bool, error) {
	return s.provisionerClient.IsProviderConfigured(ctx, tenantID, actorID, provider, vendorID)
}

// ListConfiguredProviders returns a list of configured shipping providers for a tenant.
func (s *ShippingCredentialsService) ListConfiguredProviders(
	ctx context.Context,
	tenantID, actorID string,
) (*clients.ListProvidersResponse, error) {
	return s.provisionerClient.ListProviders(ctx, tenantID, actorID, "shipping")
}

// invalidateCacheForProvider invalidates cached secrets for a shipping provider
func (s *ShippingCredentialsService) invalidateCacheForProvider(
	tenantID, vendorID string,
	provider secrets.ShippingProvider,
) {
	allKeys := secrets.GetAllShippingProviderKeys(provider)
	for _, keyName := range allKeys {
		s.secretClient.InvalidateCache(s.environment, tenantID, vendorID, provider, keyName)
	}
}

// InvalidateCache invalidates all cached secrets
func (s *ShippingCredentialsService) InvalidateCache() {
	s.secretClient.InvalidateAllCache()
}

// Close closes the underlying clients
func (s *ShippingCredentialsService) Close() error {
	return s.secretClient.Close()
}

// GetEnvironment returns the configured environment
func (s *ShippingCredentialsService) GetEnvironment() string {
	return s.environment
}
