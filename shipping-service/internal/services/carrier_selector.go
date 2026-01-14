package services

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"shipping-service/internal/carriers"
	"shipping-service/internal/config"
	"shipping-service/internal/models"
	"shipping-service/internal/repository"
)

// CarrierSelectorService handles carrier selection based on tenant configuration, country, and preferences
type CarrierSelectorService struct {
	db      *gorm.DB
	repo    *repository.CarrierConfigRepository
	factory *carriers.CarrierFactory
	envCfg  *config.Config // Fallback to env vars if no DB config

	// Legacy carrier service for backward compatibility
	legacyService *CarrierService
}

// NewCarrierSelectorService creates a new carrier selector service
func NewCarrierSelectorService(
	db *gorm.DB,
	repo *repository.CarrierConfigRepository,
	factory *carriers.CarrierFactory,
	envCfg *config.Config,
	legacyService *CarrierService,
) *CarrierSelectorService {
	return &CarrierSelectorService{
		db:            db,
		repo:          repo,
		factory:       factory,
		envCfg:        envCfg,
		legacyService: legacyService,
	}
}

// CarrierOption represents an available carrier option
type CarrierOption struct {
	CarrierType      models.CarrierType `json:"carrierType"`
	DisplayName      string             `json:"displayName"`
	IsEnabled        bool               `json:"isEnabled"`
	IsTestMode       bool               `json:"isTestMode"`
	IsPrimary        bool               `json:"isPrimary"`
	Priority         int                `json:"priority"`
	SupportsRates    bool               `json:"supportsRates"`
	SupportsTracking bool               `json:"supportsTracking"`
	SupportsLabels   bool               `json:"supportsLabels"`
	SupportsReturns  bool               `json:"supportsReturns"`
	SupportsPickup   bool               `json:"supportsPickup"`
	Services         []string           `json:"services,omitempty"`
}

// GetAvailableCarriers returns all configured and enabled carriers for a tenant and country
func (s *CarrierSelectorService) GetAvailableCarriers(ctx context.Context, tenantID string, countryCode string) ([]CarrierOption, error) {
	// Get all carrier configs for tenant
	configs, err := s.repo.ListEnabledCarrierConfigs(ctx, tenantID)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to list carrier configs: %w", err)
	}

	// If no database config, fall back to legacy behavior
	if len(configs) == 0 {
		log.Printf("No carrier configs found for tenant %s, using legacy selection", tenantID)
		return s.getLegacyCarrierOptions(countryCode), nil
	}

	countryCode = strings.ToUpper(countryCode)
	var options []CarrierOption

	for _, cfg := range configs {
		if !cfg.IsEnabled {
			continue
		}

		// Check if carrier supports this country
		supportsCountry := false
		isPrimary := false
		priority := cfg.Priority
		var services []string

		// First check region mapping (most specific)
		for _, region := range cfg.Regions {
			if strings.ToUpper(region.CountryCode) == countryCode && region.Enabled {
				supportsCountry = true
				isPrimary = region.IsPrimary
				priority = region.Priority
				services = region.SupportedServices
				break
			}
		}

		// Fall back to carrier's supported countries
		if !supportsCountry {
			for _, c := range cfg.SupportedCountries {
				if strings.ToUpper(c) == countryCode {
					supportsCountry = true
					break
				}
			}
		}

		if !supportsCountry {
			continue
		}

		if len(services) == 0 {
			services = cfg.SupportedServices
		}

		options = append(options, CarrierOption{
			CarrierType:      cfg.CarrierType,
			DisplayName:      cfg.DisplayName,
			IsEnabled:        cfg.IsEnabled,
			IsTestMode:       cfg.IsTestMode,
			IsPrimary:        isPrimary,
			Priority:         priority,
			SupportsRates:    cfg.SupportsRates,
			SupportsTracking: cfg.SupportsTracking,
			SupportsLabels:   cfg.SupportsLabels,
			SupportsReturns:  cfg.SupportsReturns,
			SupportsPickup:   cfg.SupportsPickup,
			Services:         services,
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

// GetPrimaryCarrier returns the primary carrier for a tenant and country
func (s *CarrierSelectorService) GetPrimaryCarrier(ctx context.Context, tenantID string, countryCode string) (*models.ShippingCarrierConfig, error) {
	// First try to get from region mapping (most specific)
	primaryConfig, err := s.repo.GetPrimaryCarrierForCountry(ctx, tenantID, strings.ToUpper(countryCode))
	if err == nil {
		return primaryConfig, nil
	}

	// Fall back to available carriers
	carriers, err := s.GetAvailableCarriers(ctx, tenantID, countryCode)
	if err != nil {
		return nil, err
	}

	if len(carriers) == 0 {
		return nil, fmt.Errorf("no carrier available for country: %s", countryCode)
	}

	// First carrier in sorted list is the primary
	primaryType := carriers[0].CarrierType
	return s.repo.GetCarrierConfigByType(ctx, tenantID, primaryType)
}

// SelectCarrierForRoute selects the best carrier for a shipping route
// Uses a fallback strategy: preferred -> fallback -> priority-ordered list
func (s *CarrierSelectorService) SelectCarrierForRoute(ctx context.Context, tenantID string, fromCountry, toCountry string) (carriers.Carrier, error) {
	fromCountry = strings.ToUpper(fromCountry)
	toCountry = strings.ToUpper(toCountry)

	// Get shipping settings for tenant
	settings, _ := s.repo.GetShippingSettings(ctx, tenantID)

	// Try to get carrier config from database
	configs, err := s.repo.ListEnabledCarrierConfigs(ctx, tenantID)
	if err != nil || len(configs) == 0 {
		// Fall back to legacy carrier service
		log.Printf("No carrier configs for tenant %s, using legacy carrier selection", tenantID)
		return s.legacyService.SelectCarrier(fromCountry, toCountry)
	}

	// Determine selection strategy
	strategy := "priority"
	if settings != nil && settings.SelectionStrategy != "" {
		strategy = settings.SelectionStrategy
	}

	// Find carriers that support this route
	var matchingConfigs []models.ShippingCarrierConfig
	for _, cfg := range configs {
		if s.carrierSupportsRoute(cfg, fromCountry, toCountry) {
			matchingConfigs = append(matchingConfigs, cfg)
		}
	}

	if len(matchingConfigs) == 0 {
		// Fall back to legacy if no matching carriers in database
		log.Printf("No matching carriers for route %s -> %s, using legacy selection", fromCountry, toCountry)
		return s.legacyService.SelectCarrier(fromCountry, toCountry)
	}

	// Sort by strategy
	switch strategy {
	case "priority":
		sort.Slice(matchingConfigs, func(i, j int) bool {
			return matchingConfigs[i].Priority < matchingConfigs[j].Priority
		})
	case "cheapest", "fastest":
		// These would require rate comparison - for now, just use priority
		sort.Slice(matchingConfigs, func(i, j int) bool {
			return matchingConfigs[i].Priority < matchingConfigs[j].Priority
		})
	}

	// Try preferred carrier first (if configured)
	if settings != nil && settings.PreferredCarrierType != "" {
		for _, cfg := range matchingConfigs {
			if string(cfg.CarrierType) == settings.PreferredCarrierType {
				carrier, err := s.factory.CreateCarrier(&cfg)
				if err != nil {
					log.Printf("Failed to create preferred carrier %s: %v, trying fallback", cfg.CarrierType, err)
					break // Try fallback
				}
				if carrier.IsAvailable(fromCountry, toCountry) {
					log.Printf("Selected preferred carrier: %s", cfg.CarrierType)
					return carrier, nil
				}
				log.Printf("Preferred carrier %s not available for route %s->%s, trying fallback", cfg.CarrierType, fromCountry, toCountry)
				break
			}
		}
	}

	// Try fallback carrier (if configured and different from preferred)
	if settings != nil && settings.FallbackCarrierType != "" && settings.FallbackCarrierType != settings.PreferredCarrierType {
		for _, cfg := range matchingConfigs {
			if string(cfg.CarrierType) == settings.FallbackCarrierType {
				carrier, err := s.factory.CreateCarrier(&cfg)
				if err != nil {
					log.Printf("Failed to create fallback carrier %s: %v, trying remaining carriers", cfg.CarrierType, err)
					break
				}
				if carrier.IsAvailable(fromCountry, toCountry) {
					log.Printf("Selected fallback carrier: %s", cfg.CarrierType)
					return carrier, nil
				}
				log.Printf("Fallback carrier %s not available for route %s->%s", cfg.CarrierType, fromCountry, toCountry)
				break
			}
		}
	}

	// Try remaining carriers in priority order (excluding already tried preferred/fallback)
	for _, cfg := range matchingConfigs {
		// Skip if already tried as preferred or fallback
		if settings != nil {
			if string(cfg.CarrierType) == settings.PreferredCarrierType {
				continue
			}
			if string(cfg.CarrierType) == settings.FallbackCarrierType {
				continue
			}
		}

		carrier, err := s.factory.CreateCarrier(&cfg)
		if err != nil {
			log.Printf("Failed to create carrier %s: %v", cfg.CarrierType, err)
			continue
		}
		if carrier.IsAvailable(fromCountry, toCountry) {
			log.Printf("Selected carrier from priority list: %s", cfg.CarrierType)
			return carrier, nil
		}
	}

	return nil, fmt.Errorf("no available carrier for route %s -> %s", fromCountry, toCountry)
}

// GetCarrierInstance returns an instantiated carrier for the given config
func (s *CarrierSelectorService) GetCarrierInstance(ctx context.Context, config *models.ShippingCarrierConfig) (carriers.Carrier, error) {
	return s.factory.CreateCarrier(config)
}

// GetCarrierByType returns a carrier instance by type for a tenant
func (s *CarrierSelectorService) GetCarrierByType(ctx context.Context, tenantID string, carrierType models.CarrierType) (carriers.Carrier, error) {
	config, err := s.repo.GetCarrierConfigByType(ctx, tenantID, carrierType)
	if err != nil {
		return nil, fmt.Errorf("carrier config not found: %w", err)
	}
	return s.factory.CreateCarrier(config)
}

// GetRatesFromAllCarriers gets rates from all available carriers for a route
func (s *CarrierSelectorService) GetRatesFromAllCarriers(ctx context.Context, tenantID string, request models.GetRatesRequest) ([]models.ShippingRate, error) {
	fromCountry := strings.ToUpper(request.FromAddress.Country)
	toCountry := strings.ToUpper(request.ToAddress.Country)

	log.Printf("GetRatesFromAllCarriers: tenant=%s, route=%s->%s", tenantID, fromCountry, toCountry)

	// Get shipping settings for markup configuration
	settings, _ := s.repo.GetShippingSettings(ctx, tenantID)
	var markupPercent float64
	var handlingFee float64
	if settings != nil {
		markupPercent = settings.HandlingFeePercent
		handlingFee = settings.HandlingFee
		log.Printf("GetRatesFromAllCarriers: markup settings - percent=%.2f%%, fixed=%.2f", markupPercent*100, handlingFee)
	}

	// Get available carriers
	configs, err := s.repo.ListEnabledCarrierConfigs(ctx, tenantID)
	if err != nil {
		log.Printf("GetRatesFromAllCarriers: failed to list configs: %v", err)
		rates, err := s.legacyService.GetRatesWithFallback(request)
		if err != nil {
			return nil, err
		}
		return s.applyMarkupToRates(rates, markupPercent, handlingFee), nil
	}
	if len(configs) == 0 {
		log.Printf("GetRatesFromAllCarriers: no configs found for tenant %s", tenantID)
		rates, err := s.legacyService.GetRatesWithFallback(request)
		if err != nil {
			return nil, err
		}
		return s.applyMarkupToRates(rates, markupPercent, handlingFee), nil
	}

	log.Printf("GetRatesFromAllCarriers: found %d carrier configs", len(configs))

	// Sort configs to prioritize preferred carrier, then fallback, then by priority
	preferredType := ""
	fallbackType := ""
	if settings != nil {
		preferredType = settings.PreferredCarrierType
		fallbackType = settings.FallbackCarrierType
	}

	sort.Slice(configs, func(i, j int) bool {
		iType := string(configs[i].CarrierType)
		jType := string(configs[j].CarrierType)

		// Preferred carrier comes first
		if iType == preferredType && jType != preferredType {
			return true
		}
		if jType == preferredType && iType != preferredType {
			return false
		}
		// Fallback carrier comes second
		if iType == fallbackType && jType != fallbackType && jType != preferredType {
			return true
		}
		if jType == fallbackType && iType != fallbackType && iType != preferredType {
			return false
		}
		// Then by priority
		return configs[i].Priority < configs[j].Priority
	})

	var allRates []models.ShippingRate
	var preferredRates []models.ShippingRate
	var fallbackRates []models.ShippingRate

	// Try to get rates from preferred and fallback carriers only
	for _, cfg := range configs {
		carrierTypeStr := string(cfg.CarrierType)
		isPreferred := carrierTypeStr == preferredType
		isFallback := carrierTypeStr == fallbackType

		// Only process preferred or fallback carriers (skip others)
		if !isPreferred && !isFallback {
			log.Printf("GetRatesFromAllCarriers: skipping carrier %s (not preferred or fallback)", cfg.CarrierType)
			continue
		}

		log.Printf("GetRatesFromAllCarriers: checking carrier %s (preferred=%v, fallback=%v), supported_countries=%v",
			cfg.CarrierType, isPreferred, isFallback, cfg.SupportedCountries)

		if !s.carrierSupportsRoute(cfg, fromCountry, toCountry) {
			log.Printf("GetRatesFromAllCarriers: carrier %s does not support route %s->%s", cfg.CarrierType, fromCountry, toCountry)
			continue
		}

		carrier, err := s.factory.CreateCarrier(&cfg)
		if err != nil {
			log.Printf("Failed to create carrier %s: %v", cfg.CarrierType, err)
			continue
		}

		rates, err := carrier.GetRates(request)
		if err != nil {
			log.Printf("Failed to get rates from %s: %v", cfg.CarrierType, err)
			continue
		}

		if len(rates) > 0 {
			log.Printf("Got %d rates from %s", len(rates), cfg.CarrierType)
			if isPreferred {
				preferredRates = append(preferredRates, rates...)
			} else if isFallback {
				fallbackRates = append(fallbackRates, rates...)
			}
		}
	}

	// Use preferred carrier rates if available, otherwise use fallback
	if len(preferredRates) > 0 {
		log.Printf("GetRatesFromAllCarriers: using %d rates from preferred carrier %s", len(preferredRates), preferredType)
		allRates = preferredRates
	} else if len(fallbackRates) > 0 {
		log.Printf("GetRatesFromAllCarriers: preferred carrier %s returned no rates, using %d rates from fallback %s", preferredType, len(fallbackRates), fallbackType)
		allRates = fallbackRates
	} else {
		log.Printf("GetRatesFromAllCarriers: no rates from preferred or fallback carriers")
	}

	// Apply markup to all rates
	allRates = s.applyMarkupToRates(allRates, markupPercent, handlingFee)

	// Sort by rate (final rate including markup)
	sort.Slice(allRates, func(i, j int) bool {
		return allRates[i].Rate < allRates[j].Rate
	})

	return allRates, nil
}

// applyMarkupToRates applies markup (percentage and/or fixed fee) to all rates
func (s *CarrierSelectorService) applyMarkupToRates(rates []models.ShippingRate, markupPercent, handlingFee float64) []models.ShippingRate {
	if markupPercent == 0 && handlingFee == 0 {
		// No markup configured, just set BaseRate = Rate
		for i := range rates {
			rates[i].BaseRate = rates[i].Rate
			rates[i].MarkupAmount = 0
			rates[i].MarkupPercent = 0
		}
		return rates
	}

	for i := range rates {
		baseRate := rates[i].Rate

		// Calculate markup: percentage of base rate + fixed handling fee
		percentMarkup := baseRate * markupPercent
		totalMarkup := percentMarkup + handlingFee
		finalRate := baseRate + totalMarkup

		// Round to 2 decimal places
		finalRate = float64(int(finalRate*100+0.5)) / 100
		totalMarkup = float64(int(totalMarkup*100+0.5)) / 100

		rates[i].BaseRate = baseRate
		rates[i].MarkupAmount = totalMarkup
		rates[i].MarkupPercent = markupPercent * 100 // Store as percentage (e.g., 10 for 10%)
		rates[i].Rate = finalRate

		log.Printf("Applied markup to %s %s: base=%.2f + markup=%.2f (%.1f%% + %.2f fixed) = %.2f",
			rates[i].Carrier, rates[i].ServiceName, baseRate, totalMarkup, markupPercent*100, handlingFee, finalRate)
	}

	return rates
}

// CreateCarrierFromTemplate creates a new carrier config from a template
func (s *CarrierSelectorService) CreateCarrierFromTemplate(ctx context.Context, tenantID string, carrierType models.CarrierType, credentials map[string]string, isTestMode bool) (*models.ShippingCarrierConfig, error) {
	// Check if carrier already exists
	exists, err := s.repo.CarrierExistsForTenant(ctx, tenantID, carrierType)
	if err != nil {
		return nil, fmt.Errorf("failed to check carrier existence: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("carrier %s already exists for this tenant", carrierType)
	}

	// Get template
	template, err := s.repo.GetCarrierTemplate(ctx, carrierType)
	if err != nil {
		return nil, fmt.Errorf("carrier template not found: %w", err)
	}

	// Create config from template
	config := &models.ShippingCarrierConfig{
		ID:                 uuid.New(),
		TenantID:           tenantID,
		CarrierType:        carrierType,
		DisplayName:        template.DisplayName,
		IsEnabled:          true,
		IsTestMode:         isTestMode,
		SupportsRates:      template.SupportsRates,
		SupportsTracking:   template.SupportsTracking,
		SupportsLabels:     template.SupportsLabels,
		SupportsReturns:    template.SupportsReturns,
		SupportsPickup:     template.SupportsPickup,
		SupportedCountries: template.SupportedCountries,
		SupportedServices:  template.SupportedServices,
		Priority:           10,
		Description:        template.Description,
		LogoURL:            template.LogoURL,
		Config:             template.DefaultConfig,
		Credentials:        models.JSONB{},
	}

	// Apply credentials
	for key, value := range credentials {
		switch key {
		case "email":
			config.Credentials["email"] = value
		case "password":
			config.Credentials["password"] = value
		case "api_key", "api_token":
			config.APIKeyPublic = value
		case "api_secret", "api_key_secret":
			config.APIKeySecret = value
		case "webhook_secret":
			config.WebhookSecret = value
		case "base_url":
			config.BaseURL = value
		default:
			// Store in credentials JSONB
			config.Credentials[key] = value
		}
	}

	if err := s.repo.CreateCarrierConfig(ctx, config); err != nil {
		return nil, fmt.Errorf("failed to create carrier config: %w", err)
	}

	// Invalidate factory cache
	s.factory.InvalidateTenantCache(tenantID)

	return config, nil
}

// TestCarrierConnection tests connection for an existing carrier config
func (s *CarrierSelectorService) TestCarrierConnection(ctx context.Context, configID uuid.UUID) (*models.ValidateCredentialsResponse, error) {
	// Get the carrier config
	config, err := s.repo.GetCarrierConfig(ctx, configID)
	if err != nil {
		return &models.ValidateCredentialsResponse{
			Valid:   false,
			Message: fmt.Sprintf("Carrier config not found: %v", err),
		}, nil
	}

	// Create carrier instance (bypass cache to ensure fresh connection)
	s.factory.InvalidateCache(config.TenantID, config.CarrierType, config.IsTestMode)
	carrier, err := s.factory.CreateCarrier(config)
	if err != nil {
		return &models.ValidateCredentialsResponse{
			Valid:   false,
			Message: fmt.Sprintf("Failed to create carrier: %v", err),
		}, nil
	}

	// Test the connection
	if err := carrier.TestConnection(); err != nil {
		return &models.ValidateCredentialsResponse{
			Valid:   false,
			Message: fmt.Sprintf("Connection test failed: %v", err),
			Details: models.JSONB{
				"carrier_type": string(config.CarrierType),
				"is_test_mode": config.IsTestMode,
				"error":        err.Error(),
			},
		}, nil
	}

	return &models.ValidateCredentialsResponse{
		Valid:   true,
		Message: fmt.Sprintf("Successfully connected to %s", carrier.GetName()),
		Details: models.JSONB{
			"carrier_type": string(config.CarrierType),
			"is_test_mode": config.IsTestMode,
		},
	}, nil
}

// ValidateCarrierCredentials validates that carrier credentials are valid by making a test API call
func (s *CarrierSelectorService) ValidateCarrierCredentials(ctx context.Context, carrierType models.CarrierType, credentials map[string]string, isTestMode bool) (*models.ValidateCredentialsResponse, error) {
	// Build a temporary config
	config := &models.ShippingCarrierConfig{
		CarrierType: carrierType,
		IsTestMode:  isTestMode,
		Credentials: models.JSONB{},
	}

	for key, value := range credentials {
		switch key {
		case "email":
			config.Credentials["email"] = value
		case "password":
			config.Credentials["password"] = value
		case "api_key", "api_token":
			config.APIKeyPublic = value
		case "api_secret", "api_key_secret":
			config.APIKeySecret = value
		default:
			config.Credentials[key] = value
		}
	}

	// Try to create the carrier
	carrier, err := s.factory.CreateCarrier(config)
	if err != nil {
		return &models.ValidateCredentialsResponse{
			Valid:   false,
			Message: fmt.Sprintf("Failed to create carrier: %v", err),
		}, nil
	}

	// Test the connection by making a real API call
	if err := carrier.TestConnection(); err != nil {
		return &models.ValidateCredentialsResponse{
			Valid:   false,
			Message: fmt.Sprintf("Connection test failed: %v", err),
			Details: models.JSONB{
				"carrier_type": string(carrierType),
				"is_test_mode": isTestMode,
				"error":        err.Error(),
			},
		}, nil
	}

	return &models.ValidateCredentialsResponse{
		Valid:   true,
		Message: fmt.Sprintf("Successfully connected to %s", carrier.GetName()),
		Details: models.JSONB{
			"carrier_type": string(carrierType),
			"is_test_mode": isTestMode,
		},
	}, nil
}

// GetCountryCarrierMatrix returns a matrix of countries to carriers for a tenant
func (s *CarrierSelectorService) GetCountryCarrierMatrix(ctx context.Context, tenantID string) (map[string][]CarrierOption, error) {
	// Get all carrier configs for tenant
	configs, err := s.repo.ListCarrierConfigs(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list carrier configs: %w", err)
	}

	// Get all unique countries
	countrySet := make(map[string]bool)
	for _, config := range configs {
		for _, country := range config.SupportedCountries {
			countrySet[strings.ToUpper(country)] = true
		}
		for _, region := range config.Regions {
			countrySet[strings.ToUpper(region.CountryCode)] = true
		}
	}

	// Build matrix
	matrix := make(map[string][]CarrierOption)
	for country := range countrySet {
		carriers, err := s.GetAvailableCarriers(ctx, tenantID, country)
		if err != nil {
			continue
		}
		if len(carriers) > 0 {
			matrix[country] = carriers
		}
	}

	return matrix, nil
}

// GetCarrierTemplates returns all available carrier templates
func (s *CarrierSelectorService) GetCarrierTemplates(ctx context.Context) ([]models.ShippingCarrierTemplate, error) {
	return s.repo.ListCarrierTemplates(ctx)
}

// GetShippingSettings returns shipping settings for a tenant
func (s *CarrierSelectorService) GetShippingSettings(ctx context.Context, tenantID string) (*models.ShippingSettings, error) {
	return s.repo.GetOrCreateShippingSettings(ctx, tenantID)
}

// UpdateShippingSettings updates shipping settings for a tenant
func (s *CarrierSelectorService) UpdateShippingSettings(ctx context.Context, tenantID string, req *models.UpdateShippingSettingsRequest) (*models.ShippingSettings, error) {
	settings, err := s.repo.GetOrCreateShippingSettings(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	// Apply updates
	if req.AutoSelectCarrier != nil {
		settings.AutoSelectCarrier = *req.AutoSelectCarrier
	}
	if req.PreferredCarrierType != nil {
		settings.PreferredCarrierType = *req.PreferredCarrierType
	}
	if req.FallbackCarrierType != nil {
		settings.FallbackCarrierType = *req.FallbackCarrierType
	}
	if req.SelectionStrategy != nil {
		settings.SelectionStrategy = *req.SelectionStrategy
	}
	if req.FreeShippingEnabled != nil {
		settings.FreeShippingEnabled = *req.FreeShippingEnabled
	}
	if req.FreeShippingMinimum != nil {
		settings.FreeShippingMinimum = *req.FreeShippingMinimum
	}
	if req.HandlingFee != nil {
		settings.HandlingFee = *req.HandlingFee
	}
	if req.HandlingFeePercent != nil {
		settings.HandlingFeePercent = *req.HandlingFeePercent
	}
	if req.DefaultWeightUnit != nil {
		settings.DefaultWeightUnit = *req.DefaultWeightUnit
	}
	if req.DefaultDimensionUnit != nil {
		settings.DefaultDimensionUnit = *req.DefaultDimensionUnit
	}
	if req.DefaultPackageWeight != nil {
		settings.DefaultPackageWeight = *req.DefaultPackageWeight
	}
	if req.InsuranceEnabled != nil {
		settings.InsuranceEnabled = *req.InsuranceEnabled
	}
	if req.InsuranceMinValue != nil {
		settings.InsuranceMinValue = *req.InsuranceMinValue
	}
	if req.InsurancePercentage != nil {
		settings.InsurancePercentage = *req.InsurancePercentage
	}
	if req.AutoInsureAboveValue != nil {
		settings.AutoInsureAboveValue = *req.AutoInsureAboveValue
	}
	if req.SendShipmentNotifications != nil {
		settings.SendShipmentNotifications = *req.SendShipmentNotifications
	}
	if req.SendDeliveryNotifications != nil {
		settings.SendDeliveryNotifications = *req.SendDeliveryNotifications
	}
	if req.SendTrackingUpdates != nil {
		settings.SendTrackingUpdates = *req.SendTrackingUpdates
	}
	if req.ReturnsEnabled != nil {
		settings.ReturnsEnabled = *req.ReturnsEnabled
	}
	if req.ReturnWindowDays != nil {
		settings.ReturnWindowDays = *req.ReturnWindowDays
	}
	if req.FreeReturnsEnabled != nil {
		settings.FreeReturnsEnabled = *req.FreeReturnsEnabled
	}
	if req.ReturnLabelMode != nil {
		settings.ReturnLabelMode = *req.ReturnLabelMode
	}
	if req.CacheRates != nil {
		settings.CacheRates = *req.CacheRates
	}
	if req.RateCacheDuration != nil {
		settings.RateCacheDuration = *req.RateCacheDuration
	}

	// Apply warehouse address updates
	if req.Warehouse != nil {
		settings.WarehouseName = req.Warehouse.Name
		settings.WarehouseCompany = req.Warehouse.Company
		settings.WarehousePhone = req.Warehouse.Phone
		settings.WarehouseEmail = req.Warehouse.Email
		settings.WarehouseStreet = req.Warehouse.Street
		settings.WarehouseStreet2 = req.Warehouse.Street2
		settings.WarehouseCity = req.Warehouse.City
		settings.WarehouseState = req.Warehouse.State
		settings.WarehousePostalCode = req.Warehouse.PostalCode
		settings.WarehouseCountry = req.Warehouse.Country
	}

	if err := s.repo.UpdateShippingSettings(ctx, settings); err != nil {
		return nil, err
	}

	return settings, nil
}

// Internal helpers

func (s *CarrierSelectorService) carrierSupportsRoute(cfg models.ShippingCarrierConfig, fromCountry, toCountry string) bool {
	// Check region mappings first
	for _, region := range cfg.Regions {
		if region.Enabled {
			cc := strings.ToUpper(region.CountryCode)
			if cc == fromCountry || cc == toCountry {
				return true
			}
		}
	}

	// Check supported countries
	for _, country := range cfg.SupportedCountries {
		c := strings.ToUpper(country)
		if c == fromCountry || c == toCountry {
			return true
		}
	}

	return false
}

func (s *CarrierSelectorService) getLegacyCarrierOptions(countryCode string) []CarrierOption {
	// Return legacy carrier options based on country
	countryCode = strings.ToUpper(countryCode)
	var options []CarrierOption

	if countryCode == "IN" {
		// India carriers: Delhivery (primary), Shiprocket (fallback)
		options = append(options, CarrierOption{
			CarrierType:      models.CarrierDelhivery,
			DisplayName:      "Delhivery",
			IsEnabled:        true,
			IsTestMode:       false,
			IsPrimary:        true, // Delhivery is primary for India
			Priority:         0,
			SupportsRates:    true,
			SupportsTracking: true,
			SupportsLabels:   true,
			SupportsReturns:  true,
			SupportsPickup:   true,
		})
		options = append(options, CarrierOption{
			CarrierType:      models.CarrierShiprocket,
			DisplayName:      "Shiprocket",
			IsEnabled:        true,
			IsTestMode:       false,
			IsPrimary:        false, // Shiprocket is fallback
			Priority:         10,
			SupportsRates:    true,
			SupportsTracking: true,
			SupportsLabels:   true,
			SupportsReturns:  true,
			SupportsPickup:   true,
		})
	} else {
		// Global carrier options
		options = append(options, CarrierOption{
			CarrierType:      models.CarrierShippo,
			DisplayName:      "Shippo",
			IsEnabled:        true,
			IsTestMode:       false,
			IsPrimary:        true,
			Priority:         0,
			SupportsRates:    true,
			SupportsTracking: true,
			SupportsLabels:   true,
			SupportsReturns:  true,
			SupportsPickup:   false,
		})
	}

	return options
}

// SyncWarehouseToCarriers syncs the warehouse address to all configured carriers that support pickup locations
// Currently supports: Shiprocket
func (s *CarrierSelectorService) SyncWarehouseToCarriers(ctx context.Context, tenantID string, warehouse *models.WarehouseAddress) error {
	if warehouse == nil {
		return nil
	}

	// Get Shiprocket config for this tenant (if exists)
	shiprocketConfig, err := s.repo.GetCarrierConfigByType(ctx, tenantID, models.CarrierShiprocket)
	if err != nil {
		// No Shiprocket configured - that's fine, skip sync
		log.Printf("No Shiprocket config for tenant %s, skipping pickup location sync", tenantID)
		return nil
	}

	if !shiprocketConfig.IsEnabled {
		log.Printf("Shiprocket is disabled for tenant %s, skipping pickup location sync", tenantID)
		return nil
	}

	// Create Shiprocket carrier instance
	shiprocketCarrier, err := s.factory.CreateCarrier(shiprocketConfig)
	if err != nil {
		return fmt.Errorf("failed to create Shiprocket carrier: %w", err)
	}

	// Type assert to get Shiprocket-specific methods
	sr, ok := shiprocketCarrier.(*carriers.ShiprocketCarrier)
	if !ok {
		return fmt.Errorf("carrier is not a Shiprocket carrier")
	}

	// Convert country and state codes to full names (Shiprocket requires full names)
	countryName := convertCountryCodeToName(warehouse.Country)
	stateName := convertStateCodeToName(warehouse.State, warehouse.Country)

	// Create pickup location with code "Primary" (used in shipment creation)
	pickupLocation := carriers.PickupLocation{
		PickupCode: "Primary",
		Name:       warehouse.Name,
		Email:      warehouse.Email,
		Phone:      warehouse.Phone,
		Address:    warehouse.Street,
		Address2:   warehouse.Street2,
		City:       warehouse.City,
		State:      stateName,
		Country:    countryName,
		PinCode:    warehouse.PostalCode,
	}

	// Add/update pickup location in Shiprocket
	if err := sr.AddPickupLocation(pickupLocation); err != nil {
		log.Printf("Failed to sync warehouse to Shiprocket for tenant %s: %v", tenantID, err)
		return fmt.Errorf("failed to sync warehouse to Shiprocket: %w", err)
	}

	log.Printf("Successfully synced warehouse to Shiprocket as 'Primary' pickup location for tenant %s (test_mode: %v)", tenantID, shiprocketConfig.IsTestMode)
	return nil
}

// convertCountryCodeToName converts ISO country code to full name
func convertCountryCodeToName(code string) string {
	countryMap := map[string]string{
		"IN": "India",
		"US": "United States",
		"GB": "United Kingdom",
		"CA": "Canada",
		"AU": "Australia",
		"DE": "Germany",
		"FR": "France",
		"JP": "Japan",
		"CN": "China",
		"SG": "Singapore",
		"AE": "United Arab Emirates",
	}
	if name, ok := countryMap[code]; ok {
		return name
	}
	return code // Return as-is if not found (might already be full name)
}

// convertStateCodeToName converts state code to full name (for India)
func convertStateCodeToName(stateCode, countryCode string) string {
	if countryCode != "IN" && countryCode != "India" {
		return stateCode // Only convert Indian states for now
	}

	indianStates := map[string]string{
		"AN": "Andaman and Nicobar Islands",
		"AP": "Andhra Pradesh",
		"AR": "Arunachal Pradesh",
		"AS": "Assam",
		"BR": "Bihar",
		"CH": "Chandigarh",
		"CT": "Chhattisgarh",
		"DN": "Dadra and Nagar Haveli",
		"DD": "Daman and Diu",
		"DL": "Delhi",
		"GA": "Goa",
		"GJ": "Gujarat",
		"HR": "Haryana",
		"HP": "Himachal Pradesh",
		"JK": "Jammu and Kashmir",
		"JH": "Jharkhand",
		"KA": "Karnataka",
		"KL": "Kerala",
		"LA": "Ladakh",
		"LD": "Lakshadweep",
		"MP": "Madhya Pradesh",
		"MH": "Maharashtra",
		"MN": "Manipur",
		"ML": "Meghalaya",
		"MZ": "Mizoram",
		"NL": "Nagaland",
		"OR": "Odisha",
		"PY": "Puducherry",
		"PB": "Punjab",
		"RJ": "Rajasthan",
		"SK": "Sikkim",
		"TN": "Tamil Nadu",
		"TG": "Telangana",
		"TR": "Tripura",
		"UP": "Uttar Pradesh",
		"UK": "Uttarakhand",
		"WB": "West Bengal",
	}
	if name, ok := indianStates[stateCode]; ok {
		return name
	}
	return stateCode // Return as-is if not found (might already be full name)
}
