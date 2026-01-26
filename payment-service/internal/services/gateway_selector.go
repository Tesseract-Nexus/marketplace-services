package services

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"payment-service/internal/gateway"
	"payment-service/internal/models"
	"payment-service/internal/repository"
)

// GatewaySelectorService handles gateway selection based on country and preferences
type GatewaySelectorService struct {
	db                 *gorm.DB
	repo               *repository.PaymentRepository
	factory            *gateway.GatewayFactory
	credentialsService *PaymentCredentialsService
}

// NewGatewaySelectorService creates a new gateway selector service
func NewGatewaySelectorService(db *gorm.DB, repo *repository.PaymentRepository, factory *gateway.GatewayFactory) *GatewaySelectorService {
	return &GatewaySelectorService{
		db:      db,
		repo:    repo,
		factory: factory,
	}
}

// NewGatewaySelectorServiceWithCredentials creates a gateway selector service with credentials support
func NewGatewaySelectorServiceWithCredentials(db *gorm.DB, repo *repository.PaymentRepository, factory *gateway.GatewayFactory, credentialsService *PaymentCredentialsService) *GatewaySelectorService {
	return &GatewaySelectorService{
		db:                 db,
		repo:               repo,
		factory:            factory,
		credentialsService: credentialsService,
	}
}

// GatewayOption represents an available gateway option
type GatewayOption struct {
	GatewayType    models.GatewayType `json:"gatewayType"`
	DisplayName    string             `json:"displayName"`
	IsEnabled      bool               `json:"isEnabled"`
	IsTestMode     bool               `json:"isTestMode"`
	IsPrimary      bool               `json:"isPrimary"`
	Priority       int                `json:"priority"`
	PaymentMethods []PaymentMethodInfo `json:"paymentMethods"`
}

// PaymentMethodInfo represents a payment method with display info
type PaymentMethodInfo struct {
	Type        models.PaymentMethodType `json:"type"`
	DisplayName string                   `json:"displayName"`
	Icon        string                   `json:"icon"`
	GatewayType models.GatewayType       `json:"gatewayType"`
}

// GetAvailableGateways returns all configured and enabled gateways for a tenant and country
func (s *GatewaySelectorService) GetAvailableGateways(ctx context.Context, tenantID string, countryCode string) ([]GatewayOption, error) {
	// Get all gateway configs for tenant
	configs, err := s.repo.ListGatewayConfigs(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list gateway configs: %w", err)
	}

	// Get region mappings
	regionMappings, err := s.getGatewayRegions(ctx, tenantID, countryCode)
	if err != nil {
		// Log but don't fail - fallback to config-level country check
		regionMappings = make(map[uuid.UUID]*models.PaymentGatewayRegion)
	}

	var options []GatewayOption
	countryCode = strings.ToUpper(countryCode)

	for _, config := range configs {
		if !config.IsEnabled {
			continue
		}

		// Check if gateway supports this country
		supportsCountry := false
		isPrimary := false
		priority := config.Priority

		// First check region mapping (most specific)
		if region, ok := regionMappings[config.ID]; ok {
			supportsCountry = true
			isPrimary = region.IsPrimary
			priority = region.Priority
		} else {
			// Fall back to gateway's supported countries
			for _, c := range config.SupportedCountries {
				if strings.ToUpper(c) == countryCode {
					supportsCountry = true
					break
				}
			}
		}

		if !supportsCountry {
			continue
		}

		// Get payment methods for this gateway
		paymentMethods := s.getPaymentMethodsForGateway(config.GatewayType, config.SupportedPaymentMethods)

		options = append(options, GatewayOption{
			GatewayType:    config.GatewayType,
			DisplayName:    gateway.GetGatewayDisplayName(config.GatewayType),
			IsEnabled:      config.IsEnabled,
			IsTestMode:     config.IsTestMode,
			IsPrimary:      isPrimary,
			Priority:       priority,
			PaymentMethods: paymentMethods,
		})
	}

	// Sort by priority (lower is higher priority), then by primary status
	sort.Slice(options, func(i, j int) bool {
		if options[i].IsPrimary != options[j].IsPrimary {
			return options[i].IsPrimary
		}
		return options[i].Priority < options[j].Priority
	})

	return options, nil
}

// GetPaymentMethods returns all available payment methods for a tenant and country
func (s *GatewaySelectorService) GetPaymentMethods(ctx context.Context, tenantID string, countryCode string) ([]PaymentMethodInfo, error) {
	gateways, err := s.GetAvailableGateways(ctx, tenantID, countryCode)
	if err != nil {
		return nil, err
	}

	// Collect unique payment methods with gateway preference
	methodMap := make(map[models.PaymentMethodType]PaymentMethodInfo)
	methodPriority := make(map[models.PaymentMethodType]int)

	for _, gw := range gateways {
		for _, method := range gw.PaymentMethods {
			existingPriority, exists := methodPriority[method.Type]
			// Keep the method from the higher priority gateway
			if !exists || gw.Priority < existingPriority {
				methodMap[method.Type] = method
				methodPriority[method.Type] = gw.Priority
			}
		}
	}

	// Convert to slice
	var methods []PaymentMethodInfo
	for _, method := range methodMap {
		methods = append(methods, method)
	}

	// Sort by display name for consistent ordering
	sort.Slice(methods, func(i, j int) bool {
		return methods[i].DisplayName < methods[j].DisplayName
	})

	return methods, nil
}

// GetPrimaryGateway returns the primary gateway for a tenant and country
func (s *GatewaySelectorService) GetPrimaryGateway(ctx context.Context, tenantID string, countryCode string) (*models.PaymentGatewayConfig, error) {
	gateways, err := s.GetAvailableGateways(ctx, tenantID, countryCode)
	if err != nil {
		return nil, err
	}

	if len(gateways) == 0 {
		return nil, fmt.Errorf("no payment gateway available for country: %s", countryCode)
	}

	// First gateway in sorted list is the primary
	primaryType := gateways[0].GatewayType

	// Get the full config
	return s.repo.GetGatewayConfigByType(ctx, tenantID, primaryType)
}

// GetGatewayForPaymentMethod returns the best gateway for a specific payment method and country
func (s *GatewaySelectorService) GetGatewayForPaymentMethod(ctx context.Context, tenantID string, countryCode string, methodType models.PaymentMethodType) (*models.PaymentGatewayConfig, error) {
	gateways, err := s.GetAvailableGateways(ctx, tenantID, countryCode)
	if err != nil {
		return nil, err
	}

	// Find the highest priority gateway that supports this payment method
	for _, gw := range gateways {
		for _, method := range gw.PaymentMethods {
			if method.Type == methodType {
				return s.repo.GetGatewayConfigByType(ctx, tenantID, gw.GatewayType)
			}
		}
	}

	return nil, fmt.Errorf("no gateway found supporting payment method %s in country %s", methodType, countryCode)
}

// GetGatewayInstance returns an instantiated gateway for processing payments
func (s *GatewaySelectorService) GetGatewayInstance(ctx context.Context, config *models.PaymentGatewayConfig) (gateway.PaymentGateway, error) {
	return s.factory.CreateGateway(config)
}

// CreateGatewayRegion creates a region mapping for a gateway
func (s *GatewaySelectorService) CreateGatewayRegion(ctx context.Context, region *models.PaymentGatewayRegion) error {
	return s.db.WithContext(ctx).Create(region).Error
}

// UpdateGatewayRegion updates a region mapping
func (s *GatewaySelectorService) UpdateGatewayRegion(ctx context.Context, region *models.PaymentGatewayRegion) error {
	return s.db.WithContext(ctx).Save(region).Error
}

// DeleteGatewayRegion deletes a region mapping
func (s *GatewaySelectorService) DeleteGatewayRegion(ctx context.Context, regionID uuid.UUID) error {
	return s.db.WithContext(ctx).Delete(&models.PaymentGatewayRegion{}, "id = ?", regionID).Error
}

// GetGatewayRegions returns all region mappings for a gateway config
func (s *GatewaySelectorService) GetGatewayRegionsByConfig(ctx context.Context, configID uuid.UUID) ([]models.PaymentGatewayRegion, error) {
	var regions []models.PaymentGatewayRegion
	err := s.db.WithContext(ctx).Where("gateway_config_id = ?", configID).Order("priority ASC").Find(&regions).Error
	if err != nil {
		return nil, err
	}
	return regions, nil
}

// SetPrimaryGateway sets a gateway as primary for a country
func (s *GatewaySelectorService) SetPrimaryGateway(ctx context.Context, tenantID string, configID uuid.UUID, countryCode string) error {
	countryCode = strings.ToUpper(countryCode)

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// First, unset any existing primary for this tenant and country
		if err := tx.Model(&models.PaymentGatewayRegion{}).
			Joins("JOIN payment_gateway_configs ON payment_gateway_regions.gateway_config_id = payment_gateway_configs.id").
			Where("payment_gateway_configs.tenant_id = ? AND payment_gateway_regions.country_code = ?", tenantID, countryCode).
			Update("is_primary", false).Error; err != nil {
			return err
		}

		// Set the new primary
		var region models.PaymentGatewayRegion
		result := tx.Where("gateway_config_id = ? AND country_code = ?", configID, countryCode).First(&region)

		if result.Error == gorm.ErrRecordNotFound {
			// Create new region mapping
			region = models.PaymentGatewayRegion{
				ID:              uuid.New(),
				GatewayConfigID: configID,
				CountryCode:     countryCode,
				IsPrimary:       true,
				Priority:        0,
			}
			return tx.Create(&region).Error
		} else if result.Error != nil {
			return result.Error
		}

		// Update existing
		region.IsPrimary = true
		return tx.Save(&region).Error
	})
}

// GetGatewayTemplates returns all available gateway templates
func (s *GatewaySelectorService) GetGatewayTemplates(ctx context.Context) ([]models.PaymentGatewayTemplate, error) {
	var templates []models.PaymentGatewayTemplate
	err := s.db.WithContext(ctx).Where("is_active = true").Order("display_name ASC").Find(&templates).Error
	if err != nil {
		return nil, err
	}
	return templates, nil
}

// GetGatewayTemplate returns a specific gateway template
func (s *GatewaySelectorService) GetGatewayTemplate(ctx context.Context, gatewayType models.GatewayType) (*models.PaymentGatewayTemplate, error) {
	var template models.PaymentGatewayTemplate
	err := s.db.WithContext(ctx).Where("gateway_type = ? AND is_active = true", gatewayType).First(&template).Error
	if err != nil {
		return nil, err
	}
	return &template, nil
}

// CreateGatewayFromTemplate creates a new gateway config from a template
// If credentialsService is configured, credentials are stored in GCP Secret Manager
// Otherwise, they fall back to database storage (legacy mode)
func (s *GatewaySelectorService) CreateGatewayFromTemplate(ctx context.Context, tenantID string, gatewayType models.GatewayType, credentials map[string]string, isTestMode bool) (*models.PaymentGatewayConfig, error) {
	template, err := s.GetGatewayTemplate(ctx, gatewayType)
	if err != nil {
		return nil, fmt.Errorf("gateway template not found: %w", err)
	}

	// Create config from template - NO credentials in database when using GCP Secret Manager
	config := &models.PaymentGatewayConfig{
		ID:                      uuid.New(),
		TenantID:                tenantID,
		GatewayType:             gatewayType,
		DisplayName:             template.DisplayName,
		IsEnabled:               true,
		IsTestMode:              isTestMode,
		Priority:                10, // Default priority
		SupportedCountries:      template.SupportedCountries,
		SupportedPaymentMethods: template.SupportedPaymentMethods,
		SupportsPlatformSplit:   template.SupportsPlatformSplit,
	}

	// If credentials service is available, provision secrets to GCP Secret Manager
	if s.credentialsService != nil && len(credentials) > 0 {
		// Map credentials to the secret-provisioner expected format
		secretsToProvision := s.mapCredentialsForProvider(string(gatewayType), credentials)

		if len(secretsToProvision) > 0 {
			// Provision credentials to GCP Secret Manager
			// Use tenantID as actorID for system-initiated provisioning
			_, err := s.credentialsService.ProvisionCredentials(
				ctx,
				tenantID,
				tenantID, // actorID - using tenantID for admin actions
				strings.ToLower(string(gatewayType)),
				"", // vendorID - empty for tenant-level credentials
				secretsToProvision,
				false, // Don't validate during creation (credentials might not be active yet)
			)
			if err != nil {
				return nil, fmt.Errorf("failed to provision credentials to GCP Secret Manager: %w", err)
			}
		}

		// Store only non-sensitive metadata in database (no actual credentials)
		// The credentials will be fetched from GCP at payment time
	} else {
		// Legacy mode: store credentials in database (less secure)
		// This is only used if credentialsService is not configured
		s.applyCredentialsToConfig(config, credentials)
	}

	if err := s.repo.CreateGatewayConfig(ctx, config); err != nil {
		return nil, fmt.Errorf("failed to create gateway config: %w", err)
	}

	return config, nil
}

// mapCredentialsForProvider maps incoming credentials to GCP Secret Manager format
// This is a generic pass-through that preserves the original field names from the gateway template's required_credentials
// When adding new payment gateways:
//   1. Define required_credentials in the gateway template (e.g., ["api_key", "secret_key", "webhook_secret"])
//   2. The frontend will show input fields based on required_credentials
//   3. Credentials are stored in GCP Secret Manager as: {env}-tenant-{tenantId}-{provider}-{fieldName}
//   4. At payment time, fetch credentials using the same field names
//
// This approach is fully adaptable - no code changes needed when adding new payment gateways
func (s *GatewaySelectorService) mapCredentialsForProvider(provider string, credentials map[string]string) map[string]string {
	result := make(map[string]string)

	// Generic pass-through: preserve original field names for all providers
	// This works with any gateway as long as the frontend sends credentials
	// matching the required_credentials defined in the gateway template
	for key, value := range credentials {
		if value != "" {
			// Normalize key to lowercase with underscores for consistent GCP Secret naming
			normalizedKey := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
			result[normalizedKey] = value
		}
	}

	return result
}

// applyCredentialsToConfig applies credentials directly to config (legacy mode)
func (s *GatewaySelectorService) applyCredentialsToConfig(config *models.PaymentGatewayConfig, credentials map[string]string) {
	// api_key_public / api_key_secret (Stripe, Razorpay)
	if apiKey, ok := credentials["api_key_public"]; ok {
		config.APIKeyPublic = apiKey
	}
	if apiSecret, ok := credentials["api_key_secret"]; ok {
		config.APIKeySecret = apiSecret
	}
	// client_id / client_secret (PayPal)
	if clientID, ok := credentials["client_id"]; ok {
		config.APIKeyPublic = clientID
	}
	if clientSecret, ok := credentials["client_secret"]; ok {
		config.APIKeySecret = clientSecret
	}
	// merchant_id / secret_key (Afterpay, Zip)
	if merchantID, ok := credentials["merchant_id"]; ok {
		config.APIKeyPublic = merchantID
	}
	if secretKey, ok := credentials["secret_key"]; ok {
		config.APIKeySecret = secretKey
	}
	if apiKey, ok := credentials["api_key"]; ok {
		config.APIKeySecret = apiKey
	}
	// Other fields
	if webhookSecret, ok := credentials["webhook_secret"]; ok {
		config.WebhookSecret = webhookSecret
	}
	if merchantAccountID, ok := credentials["merchant_account_id"]; ok {
		config.MerchantAccountID = merchantAccountID
	}
	if platformAccountID, ok := credentials["platform_account_id"]; ok {
		config.PlatformAccountID = platformAccountID
	}
}

// CreateGatewayConfig creates a gateway config, provisioning credentials to GCP Secret Manager if available
// This should be used by handlers instead of directly calling repo.CreateGatewayConfig
func (s *GatewaySelectorService) CreateGatewayConfig(ctx context.Context, config *models.PaymentGatewayConfig) error {
	// If credentials service is available, provision credentials to GCP Secret Manager
	if s.credentialsService != nil {
		// Extract credentials from the config
		credentials := s.extractCredentialsFromConfig(config)

		if len(credentials) > 0 {
			// Map credentials to the secret-provisioner expected format
			secretsToProvision := s.mapCredentialsForProvider(string(config.GatewayType), credentials)

			if len(secretsToProvision) > 0 {
				// Provision credentials to GCP Secret Manager
				_, err := s.credentialsService.ProvisionCredentials(
					ctx,
					config.TenantID,
					config.TenantID, // actorID - using tenantID for admin actions
					strings.ToLower(string(config.GatewayType)),
					"", // vendorID - empty for tenant-level credentials
					secretsToProvision,
					false, // Don't validate during creation
				)
				if err != nil {
					return fmt.Errorf("failed to provision credentials to GCP Secret Manager: %w", err)
				}
			}

			// Clear credentials from config - they're now in GCP Secret Manager
			config.APIKeyPublic = ""
			config.APIKeySecret = ""
			config.WebhookSecret = ""
		}
	}
	// If no credentials service, credentials stay in config (legacy mode)

	return s.repo.CreateGatewayConfig(ctx, config)
}

// extractCredentialsFromConfig extracts credentials from a config into a map
func (s *GatewaySelectorService) extractCredentialsFromConfig(config *models.PaymentGatewayConfig) map[string]string {
	credentials := make(map[string]string)

	if config.APIKeyPublic != "" {
		credentials["api_key_public"] = config.APIKeyPublic
	}
	if config.APIKeySecret != "" {
		credentials["api_key_secret"] = config.APIKeySecret
	}
	if config.WebhookSecret != "" {
		credentials["webhook_secret"] = config.WebhookSecret
	}
	if config.MerchantAccountID != "" {
		credentials["merchant_account_id"] = config.MerchantAccountID
	}
	if config.PlatformAccountID != "" {
		credentials["platform_account_id"] = config.PlatformAccountID
	}

	return credentials
}

// GetCountryGatewayMatrix returns a matrix of countries to gateways for a tenant
func (s *GatewaySelectorService) GetCountryGatewayMatrix(ctx context.Context, tenantID string) (map[string][]GatewayOption, error) {
	// Get all gateway configs for tenant
	configs, err := s.repo.ListGatewayConfigs(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list gateway configs: %w", err)
	}

	// Get all unique countries
	countrySet := make(map[string]bool)
	for _, config := range configs {
		for _, country := range config.SupportedCountries {
			countrySet[strings.ToUpper(country)] = true
		}
	}

	// Get region mappings
	var regions []models.PaymentGatewayRegion
	if err := s.db.WithContext(ctx).
		Joins("JOIN payment_gateway_configs ON payment_gateway_regions.gateway_config_id = payment_gateway_configs.id").
		Where("payment_gateway_configs.tenant_id = ?", tenantID).
		Find(&regions).Error; err != nil {
		return nil, err
	}

	for _, region := range regions {
		countrySet[strings.ToUpper(region.CountryCode)] = true
	}

	// Build matrix
	matrix := make(map[string][]GatewayOption)
	for country := range countrySet {
		gateways, err := s.GetAvailableGateways(ctx, tenantID, country)
		if err != nil {
			continue
		}
		matrix[country] = gateways
	}

	return matrix, nil
}

// ValidateGatewayCredentials validates that a gateway can be created with the given credentials
func (s *GatewaySelectorService) ValidateGatewayCredentials(ctx context.Context, gatewayType models.GatewayType, credentials map[string]string, isTestMode bool) error {
	config := &models.PaymentGatewayConfig{
		GatewayType: gatewayType,
		IsTestMode:  isTestMode,
	}

	if apiKey, ok := credentials["api_key_public"]; ok {
		config.APIKeyPublic = apiKey
	}
	if apiSecret, ok := credentials["api_key_secret"]; ok {
		config.APIKeySecret = apiSecret
	}

	// Try to create the gateway instance
	_, err := s.factory.CreateGateway(config)
	if err != nil {
		return fmt.Errorf("invalid gateway credentials: %w", err)
	}

	return nil
}

// Internal helpers

func (s *GatewaySelectorService) getGatewayRegions(ctx context.Context, tenantID string, countryCode string) (map[uuid.UUID]*models.PaymentGatewayRegion, error) {
	var regions []models.PaymentGatewayRegion
	err := s.db.WithContext(ctx).
		Joins("JOIN payment_gateway_configs ON payment_gateway_regions.gateway_config_id = payment_gateway_configs.id").
		Where("payment_gateway_configs.tenant_id = ? AND payment_gateway_regions.country_code = ?", tenantID, strings.ToUpper(countryCode)).
		Find(&regions).Error
	if err != nil {
		return nil, err
	}

	result := make(map[uuid.UUID]*models.PaymentGatewayRegion)
	for i := range regions {
		result[regions[i].GatewayConfigID] = &regions[i]
	}
	return result, nil
}

func (s *GatewaySelectorService) getPaymentMethodsForGateway(gatewayType models.GatewayType, configuredMethods models.StringArray) []PaymentMethodInfo {
	// Get default methods for gateway type
	defaultMethods := gateway.GetGatewayPaymentMethods(gatewayType)

	// If specific methods are configured, filter to those
	methodSet := make(map[models.PaymentMethodType]bool)
	if len(configuredMethods) > 0 {
		for _, m := range configuredMethods {
			methodSet[models.PaymentMethodType(m)] = true
		}
	}

	var methods []PaymentMethodInfo
	for _, method := range defaultMethods {
		// If configured methods are specified, only include those
		if len(methodSet) > 0 && !methodSet[method] {
			continue
		}

		methods = append(methods, PaymentMethodInfo{
			Type:        method,
			DisplayName: gateway.GetPaymentMethodDisplayName(method),
			Icon:        gateway.GetPaymentMethodIcon(method),
			GatewayType: gatewayType,
		})
	}

	return methods
}
