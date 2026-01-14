package repository

import (
	"log"

	"shipping-service/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SeedCarrierTemplates seeds the default shipping carrier templates
// This is idempotent - it uses upsert to avoid duplicates
func SeedCarrierTemplates(db *gorm.DB) error {
	templates := []models.ShippingCarrierTemplate{
		// ==================== India Carriers ====================
		{
			CarrierType:      models.CarrierShiprocket,
			DisplayName:      "Shiprocket",
			Description:      "India's leading ecommerce shipping solution. Access 17+ courier partners with a single integration.",
			LogoURL:          "https://cdn.shiprocket.in/images/logo-light.svg",
			SupportsRates:    true,
			SupportsTracking: true,
			SupportsLabels:   true,
			SupportsReturns:  true,
			SupportsPickup:   true,
			SupportedCountries: models.StringArray{"IN"},
			SupportedServices:  models.StringArray{"standard", "express", "same_day", "next_day"},
			RequiredCredentials: models.StringArray{"email", "password"},
			DefaultConfig: models.JSONB{
				"channel_id":         "",
				"pickup_location_id": "",
				"default_weight":     0.5,
				"length_unit":        "cm",
				"weight_unit":        "kg",
			},
			SetupInstructions: `1. Sign up at https://app.shiprocket.in/register
2. Go to Settings > API > Generate API credentials
3. Copy your email and password
4. Optional: Set up a pickup location and copy the pickup_location_id`,
			DocumentationURL: "https://apidocs.shiprocket.in",
			Priority:         100,
			IsActive:         true,
		},
		{
			CarrierType:      models.CarrierDelhivery,
			DisplayName:      "Delhivery",
			Description:      "Pan-India express and reverse logistics provider with extensive reach. Supports Express and Surface shipping modes.",
			LogoURL:          "https://www.delhivery.com/favicon.ico",
			SupportsRates:    true,
			SupportsTracking: true,
			SupportsLabels:   true,
			SupportsReturns:  true,
			SupportsPickup:   true,
			SupportedCountries: models.StringArray{"IN"},
			SupportedServices:  models.StringArray{"EXPRESS", "SURFACE"},
			RequiredCredentials: models.StringArray{"api_token", "pickup_location"},
			DefaultConfig: models.JSONB{
				"pickup_location": "",
				"return_address":  "",
			},
			SetupInstructions: `1. Contact Delhivery sales for API access (https://www.delhivery.com/contact)
2. Get your API token from the Delhivery dashboard
3. Create a pickup location and note the pickup location code
4. For staging/testing, use staging-express.delhivery.com`,
			DocumentationURL: "https://track.delhivery.com/api/kinko/v1/invoice/charges/.json",
			Priority:         90,
			IsActive:         true,
		},
		{
			CarrierType:      models.CarrierBlueDart,
			DisplayName:      "BlueDart",
			Description:      "Premium express logistics in India. Part of DHL Group.",
			LogoURL:          "https://www.bluedart.com/images/logo.png",
			SupportsRates:    true,
			SupportsTracking: true,
			SupportsLabels:   true,
			SupportsReturns:  true,
			SupportsPickup:   true,
			SupportedCountries: models.StringArray{"IN"},
			SupportedServices:  models.StringArray{"domestic_priority", "ground_express", "dart_apex"},
			RequiredCredentials: models.StringArray{"api_key", "license_key", "login_id"},
			DefaultConfig: models.JSONB{
				"area":       "",
				"product_code": "D",
			},
			SetupInstructions: `1. Contact BlueDart for API access
2. Get your API Key, License Key, and Login ID
3. Configure your area code`,
			DocumentationURL: "https://www.bluedart.com/developers",
			Priority:         85,
			IsActive:         true,
		},
		{
			CarrierType:      models.CarrierDTDC,
			DisplayName:      "DTDC",
			Description:      "Domestic and international courier services in India.",
			LogoURL:          "https://www.dtdc.in/images/dtdc-logo.png",
			SupportsRates:    true,
			SupportsTracking: true,
			SupportsLabels:   true,
			SupportsReturns:  true,
			SupportsPickup:   true,
			SupportedCountries: models.StringArray{"IN"},
			SupportedServices:  models.StringArray{"lite", "priority", "plus"},
			RequiredCredentials: models.StringArray{"api_key", "customer_code"},
			DefaultConfig:       models.JSONB{},
			SetupInstructions: `1. Sign up for DTDC API access
2. Get your API Key and Customer Code`,
			DocumentationURL: "https://www.dtdc.in/developer",
			Priority:         80,
			IsActive:         true,
		},

		// ==================== Global Carriers ====================
		{
			CarrierType:      models.CarrierShippo,
			DisplayName:      "Shippo",
			Description:      "Multi-carrier shipping API. Connect to 50+ carriers including USPS, FedEx, UPS, DHL.",
			LogoURL:          "https://cdn.simpleicons.org/shippo/108CFF",
			SupportsRates:    true,
			SupportsTracking: true,
			SupportsLabels:   true,
			SupportsReturns:  true,
			SupportsPickup:   false,
			SupportedCountries: models.StringArray{"US", "CA", "GB", "AU", "DE", "FR", "NL", "ES", "IT"},
			SupportedServices:  models.StringArray{"usps_priority", "usps_first_class", "fedex_ground", "fedex_express", "ups_ground", "ups_3day"},
			RequiredCredentials: models.StringArray{"api_token"},
			DefaultConfig: models.JSONB{
				"default_carrier": "usps",
				"test_mode":       true,
			},
			SetupInstructions: `1. Sign up at https://goshippo.com
2. Go to API > Tokens and generate an API token
3. Use the Live token for production or Test token for sandbox`,
			DocumentationURL: "https://goshippo.com/docs/intro",
			Priority:         70,
			IsActive:         true,
		},
		{
			CarrierType:      models.CarrierShipEngine,
			DisplayName:      "ShipEngine",
			Description:      "Enterprise shipping API supporting 100+ carriers worldwide.",
			LogoURL:          "https://www.shipengine.com/favicon.ico",
			SupportsRates:    true,
			SupportsTracking: true,
			SupportsLabels:   true,
			SupportsReturns:  true,
			SupportsPickup:   true,
			SupportedCountries: models.StringArray{"US", "CA", "GB", "AU", "DE", "FR", "NL", "ES", "IT", "MX", "BR"},
			SupportedServices:  models.StringArray{"usps_priority_mail", "fedex_ground", "ups_ground", "dhl_express"},
			RequiredCredentials: models.StringArray{"api_key"},
			DefaultConfig: models.JSONB{
				"default_carrier_id": "",
			},
			SetupInstructions: `1. Sign up at https://shipengine.com
2. Go to Settings > API Keys and create a new key
3. Connect your carrier accounts in the dashboard`,
			DocumentationURL: "https://www.shipengine.com/docs",
			Priority:         65,
			IsActive:         true,
		},
		{
			CarrierType:      models.CarrierFedEx,
			DisplayName:      "FedEx",
			Description:      "Global express transportation and logistics services.",
			LogoURL:          "https://cdn.simpleicons.org/fedex/4D148C",
			SupportsRates:    true,
			SupportsTracking: true,
			SupportsLabels:   true,
			SupportsReturns:  true,
			SupportsPickup:   true,
			SupportedCountries: models.StringArray{"US", "CA", "GB", "AU", "DE", "FR", "NL", "ES", "IT", "JP", "CN", "IN", "MX", "BR"},
			SupportedServices:  models.StringArray{"FEDEX_GROUND", "FEDEX_EXPRESS_SAVER", "FEDEX_2_DAY", "PRIORITY_OVERNIGHT", "INTERNATIONAL_PRIORITY"},
			RequiredCredentials: models.StringArray{"api_key", "api_secret", "account_number"},
			DefaultConfig: models.JSONB{
				"meter_number": "",
			},
			SetupInstructions: `1. Sign up for FedEx Developer Portal at https://developer.fedex.com
2. Create an API project and get credentials
3. Add your FedEx account number`,
			DocumentationURL: "https://developer.fedex.com/api/en-us/home.html",
			Priority:         60,
			IsActive:         true,
		},
		{
			CarrierType:      models.CarrierUPS,
			DisplayName:      "UPS",
			Description:      "Package delivery and supply chain solutions worldwide.",
			LogoURL:          "https://cdn.simpleicons.org/ups/351C15",
			SupportsRates:    true,
			SupportsTracking: true,
			SupportsLabels:   true,
			SupportsReturns:  true,
			SupportsPickup:   true,
			SupportedCountries: models.StringArray{"US", "CA", "GB", "AU", "DE", "FR", "NL", "ES", "IT", "JP", "CN", "IN", "MX", "BR"},
			SupportedServices:  models.StringArray{"03", "02", "01", "14", "59", "65"}, // Ground, 2nd Day, Next Day Air, etc.
			RequiredCredentials: models.StringArray{"client_id", "client_secret", "account_number"},
			DefaultConfig: models.JSONB{
				"shipper_number": "",
			},
			SetupInstructions: `1. Sign up for UPS Developer Kit at https://developer.ups.com
2. Create an OAuth application and get credentials
3. Add your UPS account number`,
			DocumentationURL: "https://developer.ups.com/api/reference",
			Priority:         55,
			IsActive:         true,
		},
		{
			CarrierType:      models.CarrierDHL,
			DisplayName:      "DHL Express",
			Description:      "International express shipping and logistics services.",
			LogoURL:          "https://cdn.simpleicons.org/dhl/FFCC00",
			SupportsRates:    true,
			SupportsTracking: true,
			SupportsLabels:   true,
			SupportsReturns:  true,
			SupportsPickup:   true,
			SupportedCountries: models.StringArray{"US", "CA", "GB", "AU", "DE", "FR", "NL", "ES", "IT", "JP", "CN", "IN", "SG", "HK"},
			SupportedServices:  models.StringArray{"EXPRESS_WORLDWIDE", "EXPRESS_EASY", "EXPRESS_9_00", "EXPRESS_12_00"},
			RequiredCredentials: models.StringArray{"api_key", "api_secret", "account_number"},
			DefaultConfig: models.JSONB{
				"site_id":   "",
				"password":  "",
			},
			SetupInstructions: `1. Sign up for DHL Express API at https://developer.dhl.com
2. Get your API credentials
3. Add your DHL account number`,
			DocumentationURL: "https://developer.dhl.com/api-reference/dhl-express-mydhl-api",
			Priority:         50,
			IsActive:         true,
		},
	}

	// Use upsert (ON CONFLICT DO UPDATE) for idempotency
	result := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "carrier_type"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"display_name",
			"description",
			"logo_url",
			"supports_rates",
			"supports_tracking",
			"supports_labels",
			"supports_returns",
			"supports_pickup",
			"supported_countries",
			"supported_services",
			"required_credentials",
			"default_config",
			"setup_instructions",
			"documentation_url",
			"priority",
			"is_active",
			"updated_at",
		}),
	}).Create(&templates)

	if result.Error != nil {
		return result.Error
	}

	log.Printf("Seeded %d shipping carrier templates", len(templates))
	return nil
}
