package carriers

import (
	"fmt"
	"sync"

	"shipping-service/internal/models"
)

// CarrierFactory creates shipping carrier instances from database configuration
type CarrierFactory struct {
	mu       sync.RWMutex
	carriers map[string]Carrier // cache of carrier instances
}

// NewCarrierFactory creates a new carrier factory
func NewCarrierFactory() *CarrierFactory {
	return &CarrierFactory{
		carriers: make(map[string]Carrier),
	}
}

// CreateCarrier creates a new carrier instance from database configuration
func (f *CarrierFactory) CreateCarrier(config *models.ShippingCarrierConfig) (Carrier, error) {
	if config == nil {
		return nil, fmt.Errorf("carrier config is required")
	}

	// Generate cache key
	cacheKey := fmt.Sprintf("%s_%s_%t", config.TenantID, config.CarrierType, config.IsTestMode)

	// Check cache first
	f.mu.RLock()
	if carrier, exists := f.carriers[cacheKey]; exists {
		f.mu.RUnlock()
		return carrier, nil
	}
	f.mu.RUnlock()

	// Create new carrier instance
	var carrier Carrier
	var err error

	// Build carrier config from database config
	carrierConfig := buildCarrierConfig(config)

	switch config.CarrierType {
	case models.CarrierShiprocket:
		carrier = NewShiprocketCarrier(carrierConfig)
	case models.CarrierDelhivery:
		carrier, err = NewDelhiveryCarrier(carrierConfig)
	case models.CarrierBlueDart:
		carrier, err = NewBlueDartCarrier(carrierConfig)
	case models.CarrierDTDC:
		carrier, err = NewDTDCCarrier(carrierConfig)
	case models.CarrierShippo:
		carrier, err = NewShippoCarrier(carrierConfig)
	case models.CarrierShipEngine:
		carrier, err = NewShipEngineCarrier(carrierConfig)
	case models.CarrierFedEx:
		carrier, err = NewFedExCarrier(carrierConfig)
	case models.CarrierUPS:
		carrier, err = NewUPSCarrier(carrierConfig)
	case models.CarrierDHL:
		carrier, err = NewDHLCarrier(carrierConfig)
	default:
		return nil, fmt.Errorf("unsupported carrier type: %s", config.CarrierType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create %s carrier: %w", config.CarrierType, err)
	}

	// Cache the carrier instance
	f.mu.Lock()
	f.carriers[cacheKey] = carrier
	f.mu.Unlock()

	return carrier, nil
}

// CreateCarrierFromEnvConfig creates a carrier from environment configuration (legacy support)
func (f *CarrierFactory) CreateCarrierFromEnvConfig(carrierType models.CarrierType, config CarrierConfig) (Carrier, error) {
	cacheKey := fmt.Sprintf("env_%s_%t", carrierType, config.IsProduction)

	// Check cache first
	f.mu.RLock()
	if carrier, exists := f.carriers[cacheKey]; exists {
		f.mu.RUnlock()
		return carrier, nil
	}
	f.mu.RUnlock()

	var carrier Carrier
	var err error

	switch carrierType {
	case models.CarrierShiprocket:
		carrier = NewShiprocketCarrier(config)
	case models.CarrierDelhivery:
		carrier, err = NewDelhiveryCarrier(config)
	case models.CarrierBlueDart:
		carrier, err = NewBlueDartCarrier(config)
	case models.CarrierDTDC:
		carrier, err = NewDTDCCarrier(config)
	case models.CarrierShippo:
		carrier, err = NewShippoCarrier(config)
	case models.CarrierShipEngine:
		carrier, err = NewShipEngineCarrier(config)
	case models.CarrierFedEx:
		carrier, err = NewFedExCarrier(config)
	case models.CarrierUPS:
		carrier, err = NewUPSCarrier(config)
	case models.CarrierDHL:
		carrier, err = NewDHLCarrier(config)
	default:
		return nil, fmt.Errorf("unsupported carrier type: %s", carrierType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create %s carrier: %w", carrierType, err)
	}

	// Cache the carrier instance
	f.mu.Lock()
	f.carriers[cacheKey] = carrier
	f.mu.Unlock()

	return carrier, nil
}

// InvalidateCache removes a carrier from the cache
func (f *CarrierFactory) InvalidateCache(tenantID string, carrierType models.CarrierType, isTestMode bool) {
	cacheKey := fmt.Sprintf("%s_%s_%t", tenantID, carrierType, isTestMode)
	f.mu.Lock()
	delete(f.carriers, cacheKey)
	f.mu.Unlock()
}

// InvalidateTenantCache removes all carriers for a tenant from the cache
func (f *CarrierFactory) InvalidateTenantCache(tenantID string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	keysToDelete := []string{}
	for key := range f.carriers {
		if len(key) > len(tenantID) && key[:len(tenantID)] == tenantID {
			keysToDelete = append(keysToDelete, key)
		}
	}

	for _, key := range keysToDelete {
		delete(f.carriers, key)
	}
}

// ClearCache removes all carriers from the cache
func (f *CarrierFactory) ClearCache() {
	f.mu.Lock()
	f.carriers = make(map[string]Carrier)
	f.mu.Unlock()
}

// buildCarrierConfig builds a CarrierConfig from database ShippingCarrierConfig
func buildCarrierConfig(config *models.ShippingCarrierConfig) CarrierConfig {
	carrierCfg := CarrierConfig{
		APIKey:       config.APIKeyPublic,
		APISecret:    config.APIKeySecret,
		BaseURL:      config.BaseURL,
		Enabled:      config.IsEnabled,
		IsProduction: !config.IsTestMode,
	}

	// Handle carrier-specific credentials from JSONB
	// For Shiprocket, APIKey is email and APISecret is password
	if config.CarrierType == models.CarrierShiprocket {
		if email := config.GetCredential("email"); email != "" {
			carrierCfg.APIKey = email
		}
		if password := config.GetCredential("password"); password != "" {
			carrierCfg.APISecret = password
		}
	}

	// For Delhivery, APIKey is the token and APISecret is the pickup location code
	if config.CarrierType == models.CarrierDelhivery {
		if token := config.GetCredential("api_token"); token != "" {
			carrierCfg.APIKey = token
		}
		if pickupLocation := config.GetCredential("pickup_location"); pickupLocation != "" {
			carrierCfg.APISecret = pickupLocation
		}
	}

	// Set default base URLs based on carrier type and mode
	if carrierCfg.BaseURL == "" {
		carrierCfg.BaseURL = getDefaultBaseURL(config.CarrierType, config.IsTestMode)
	}

	return carrierCfg
}

// getDefaultBaseURL returns the default base URL for a carrier
func getDefaultBaseURL(carrierType models.CarrierType, isTestMode bool) string {
	urls := map[models.CarrierType]map[bool]string{
		models.CarrierShiprocket: {
			true:  "https://apiv2.shiprocket.in",
			false: "https://apiv2.shiprocket.in",
		},
		models.CarrierDelhivery: {
			true:  "https://staging-express.delhivery.com",
			false: "https://track.delhivery.com",
		},
		models.CarrierShippo: {
			true:  "https://api.goshippo.com",
			false: "https://api.goshippo.com",
		},
		models.CarrierShipEngine: {
			true:  "https://api.shipengine.com",
			false: "https://api.shipengine.com",
		},
		models.CarrierFedEx: {
			true:  "https://apis-sandbox.fedex.com",
			false: "https://apis.fedex.com",
		},
		models.CarrierUPS: {
			true:  "https://wwwcie.ups.com",
			false: "https://onlinetools.ups.com",
		},
		models.CarrierDHL: {
			true:  "https://api-sandbox.dhl.com/express/v1",
			false: "https://api.dhl.com/express/v1",
		},
	}

	if carrierURLs, ok := urls[carrierType]; ok {
		if url, ok := carrierURLs[isTestMode]; ok {
			return url
		}
	}
	return ""
}

// GetSupportedCarrierTypes returns all supported carrier types
func GetSupportedCarrierTypes() []models.CarrierType {
	return []models.CarrierType{
		models.CarrierShiprocket,
		models.CarrierDelhivery,
		models.CarrierBlueDart,
		models.CarrierDTDC,
		models.CarrierShippo,
		models.CarrierShipEngine,
		models.CarrierFedEx,
		models.CarrierUPS,
		models.CarrierDHL,
	}
}

// GetCarrierDisplayName returns the display name for a carrier type
func GetCarrierDisplayName(carrierType models.CarrierType) string {
	names := map[models.CarrierType]string{
		models.CarrierShiprocket:  "Shiprocket",
		models.CarrierDelhivery:   "Delhivery",
		models.CarrierBlueDart:    "BlueDart",
		models.CarrierDTDC:        "DTDC",
		models.CarrierShippo:      "Shippo",
		models.CarrierShipEngine:  "ShipEngine",
		models.CarrierFedEx:       "FedEx",
		models.CarrierUPS:         "UPS",
		models.CarrierDHL:         "DHL Express",
	}

	if name, ok := names[carrierType]; ok {
		return name
	}
	return string(carrierType)
}

// GetCarrierCountries returns the supported countries for a carrier type
func GetCarrierCountries(carrierType models.CarrierType) []string {
	countries := map[models.CarrierType][]string{
		models.CarrierShiprocket:  {"IN"},
		models.CarrierDelhivery:   {"IN"},
		models.CarrierBlueDart:    {"IN"},
		models.CarrierDTDC:        {"IN"},
		models.CarrierShippo:      {"US", "CA", "GB", "AU", "DE", "FR", "NL", "ES", "IT"},
		models.CarrierShipEngine:  {"US", "CA", "GB", "AU", "DE", "FR", "NL", "ES", "IT", "MX", "BR"},
		models.CarrierFedEx:       {"US", "CA", "GB", "AU", "DE", "FR", "NL", "ES", "IT", "JP", "CN", "IN", "MX", "BR"},
		models.CarrierUPS:         {"US", "CA", "GB", "AU", "DE", "FR", "NL", "ES", "IT", "JP", "CN", "IN", "MX", "BR"},
		models.CarrierDHL:         {"US", "CA", "GB", "AU", "DE", "FR", "NL", "ES", "IT", "JP", "CN", "IN", "SG", "HK"},
	}

	if c, ok := countries[carrierType]; ok {
		return c
	}
	return []string{}
}

// ==================== Placeholder Carrier Implementations ====================
// These are stubs that need to be implemented when the carriers are integrated

// NewDelhiveryCarrier creates a new Delhivery carrier instance (real implementation)
func NewDelhiveryCarrier(config CarrierConfig) (Carrier, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("Delhivery API token is required")
	}
	return NewDelhiveryCarrierImpl(config), nil
}

// NewBlueDartCarrier creates a new BlueDart carrier instance
func NewBlueDartCarrier(config CarrierConfig) (Carrier, error) {
	return &stubCarrier{
		name:              models.CarrierBlueDart,
		supportedCountries: []string{"IN"},
	}, nil
}

// NewDTDCCarrier creates a new DTDC carrier instance
func NewDTDCCarrier(config CarrierConfig) (Carrier, error) {
	return &stubCarrier{
		name:              models.CarrierDTDC,
		supportedCountries: []string{"IN"},
	}, nil
}

// NewShippoCarrier creates a new Shippo carrier instance
func NewShippoCarrier(config CarrierConfig) (Carrier, error) {
	return &stubCarrier{
		name:              models.CarrierShippo,
		supportedCountries: []string{"US", "CA", "GB", "AU", "DE", "FR", "NL", "ES", "IT"},
	}, nil
}

// NewShipEngineCarrier creates a new ShipEngine carrier instance
func NewShipEngineCarrier(config CarrierConfig) (Carrier, error) {
	return &stubCarrier{
		name:              models.CarrierShipEngine,
		supportedCountries: []string{"US", "CA", "GB", "AU", "DE", "FR", "NL", "ES", "IT", "MX", "BR"},
	}, nil
}

// NewFedExCarrier creates a new FedEx carrier instance
func NewFedExCarrier(config CarrierConfig) (Carrier, error) {
	return &stubCarrier{
		name:              models.CarrierFedEx,
		supportedCountries: []string{"US", "CA", "GB", "AU", "DE", "FR", "NL", "ES", "IT", "JP", "CN", "IN", "MX", "BR"},
	}, nil
}

// NewUPSCarrier creates a new UPS carrier instance
func NewUPSCarrier(config CarrierConfig) (Carrier, error) {
	return &stubCarrier{
		name:              models.CarrierUPS,
		supportedCountries: []string{"US", "CA", "GB", "AU", "DE", "FR", "NL", "ES", "IT", "JP", "CN", "IN", "MX", "BR"},
	}, nil
}

// NewDHLCarrier creates a new DHL carrier instance
func NewDHLCarrier(config CarrierConfig) (Carrier, error) {
	return &stubCarrier{
		name:              models.CarrierDHL,
		supportedCountries: []string{"US", "CA", "GB", "AU", "DE", "FR", "NL", "ES", "IT", "JP", "CN", "IN", "SG", "HK"},
	}, nil
}

// stubCarrier is a placeholder carrier implementation
type stubCarrier struct {
	name               models.CarrierType
	supportedCountries []string
}

func (s *stubCarrier) GetName() models.CarrierType {
	return s.name
}

func (s *stubCarrier) TestConnection() error {
	return fmt.Errorf("carrier %s is not yet implemented", s.name)
}

func (s *stubCarrier) GetRates(request models.GetRatesRequest) ([]models.ShippingRate, error) {
	return nil, fmt.Errorf("carrier %s is not yet implemented", s.name)
}

func (s *stubCarrier) CreateShipment(request models.CreateShipmentRequest) (*models.Shipment, error) {
	return nil, fmt.Errorf("carrier %s is not yet implemented", s.name)
}

func (s *stubCarrier) GetTracking(trackingNumber string) (*models.TrackShipmentResponse, error) {
	return nil, fmt.Errorf("carrier %s is not yet implemented", s.name)
}

func (s *stubCarrier) CancelShipment(shipmentID string) error {
	return fmt.Errorf("carrier %s is not yet implemented", s.name)
}

func (s *stubCarrier) IsAvailable(fromCountry, toCountry string) bool {
	for _, country := range s.supportedCountries {
		if country == fromCountry || country == toCountry {
			return true
		}
	}
	return false
}

func (s *stubCarrier) SupportsRegion(countryCode string) bool {
	for _, country := range s.supportedCountries {
		if country == countryCode {
			return true
		}
	}
	return false
}

func (s *stubCarrier) GenerateReturnLabel(request models.ReturnLabelRequest) (*models.ReturnLabelResponse, error) {
	return nil, fmt.Errorf("carrier %s is not yet implemented", s.name)
}
