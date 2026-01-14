package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// JSONB custom type for PostgreSQL
type JSONB map[string]interface{}

func (j JSONB) Value() (driver.Value, error) {
	return json.Marshal(j)
}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = make(map[string]interface{})
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

func (j JSONB) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}(j))
}

func (j *JSONB) UnmarshalJSON(data []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*j = JSONB(m)
	return nil
}

// StringArray custom type for PostgreSQL text[]
type StringArray []string

func (s StringArray) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return "{" + stringArrayJoin(s) + "}", nil
}

func stringArrayJoin(arr []string) string {
	result := ""
	for i, v := range arr {
		if i > 0 {
			result += ","
		}
		result += "\"" + v + "\""
	}
	return result
}

func (s *StringArray) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return s.parsePostgresArray(string(v))
	case string:
		return s.parsePostgresArray(v)
	}
	return nil
}

func (s *StringArray) parsePostgresArray(str string) error {
	// Handle empty array
	if str == "{}" || str == "" {
		*s = []string{}
		return nil
	}

	// Remove outer braces
	str = str[1 : len(str)-1]

	// Parse elements
	var result []string
	var current string
	inQuotes := false

	for _, char := range str {
		switch char {
		case '"':
			inQuotes = !inQuotes
		case ',':
			if !inQuotes {
				result = append(result, current)
				current = ""
			} else {
				current += string(char)
			}
		default:
			current += string(char)
		}
	}

	if current != "" {
		result = append(result, current)
	}

	*s = result
	return nil
}

// ShippingCarrierConfig represents a per-tenant shipping carrier configuration
type ShippingCarrierConfig struct {
	ID          uuid.UUID   `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID    string      `gorm:"type:varchar(255);not null;index:idx_shipping_carriers_tenant" json:"tenantId"`
	CarrierType CarrierType `gorm:"type:varchar(50);not null" json:"carrierType"`
	DisplayName string      `gorm:"type:varchar(255);not null" json:"displayName"`
	IsEnabled   bool        `gorm:"default:true;index:idx_shipping_carriers_enabled" json:"isEnabled"`
	IsTestMode  bool        `gorm:"default:true" json:"isTestMode"`

	// API Credentials
	APIKeyPublic  string `gorm:"type:text" json:"apiKeyPublic"`
	APIKeySecret  string `gorm:"type:text" json:"-"` // Never expose in JSON
	WebhookSecret string `gorm:"type:text" json:"-"` // Never expose in JSON
	BaseURL       string `gorm:"type:text" json:"baseUrl,omitempty"`

	// Carrier-specific credentials (e.g., Shiprocket email/password, FedEx account number)
	Credentials JSONB `gorm:"type:jsonb" json:"-"` // Never expose credentials in JSON

	// Configuration
	Config JSONB `gorm:"type:jsonb" json:"config"`

	// Features
	SupportsRates    bool `gorm:"default:true" json:"supportsRates"`
	SupportsTracking bool `gorm:"default:true" json:"supportsTracking"`
	SupportsLabels   bool `gorm:"default:true" json:"supportsLabels"`
	SupportsReturns  bool `gorm:"default:false" json:"supportsReturns"`
	SupportsPickup   bool `gorm:"default:false" json:"supportsPickup"`

	// Geo-based Configuration
	SupportedCountries StringArray `gorm:"type:text[]" json:"supportedCountries"`
	SupportedServices  StringArray `gorm:"type:text[]" json:"supportedServices"`

	// Limits
	MaxWeight float64 `gorm:"type:decimal(10,2)" json:"maxWeight,omitempty"` // Max package weight in kg
	MaxLength float64 `gorm:"type:decimal(10,2)" json:"maxLength,omitempty"` // Max dimension in cm
	MaxVolume float64 `gorm:"type:decimal(10,2)" json:"maxVolume,omitempty"` // Max volumetric weight

	// Display
	Priority    int    `gorm:"default:0" json:"priority"`
	Description string `gorm:"type:text" json:"description"`
	LogoURL     string `gorm:"type:varchar(500)" json:"logoUrl,omitempty"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`

	// Relationships
	Regions []ShippingCarrierRegion `gorm:"foreignKey:CarrierConfigID" json:"regions,omitempty"`
}

// TableName specifies the table name for ShippingCarrierConfig
func (ShippingCarrierConfig) TableName() string {
	return "shipping_carrier_configs"
}

// HasCredentials returns true if the carrier has credentials configured
func (c *ShippingCarrierConfig) HasCredentials() bool {
	return c.APIKeyPublic != "" || c.APIKeySecret != "" || len(c.Credentials) > 0
}

// GetCredential retrieves a credential value from the Credentials JSONB field
func (c *ShippingCarrierConfig) GetCredential(key string) string {
	if c.Credentials == nil {
		return ""
	}
	if val, ok := c.Credentials[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// ShippingCarrierRegion represents country-specific configuration for shipping carriers
type ShippingCarrierRegion struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	CarrierConfigID uuid.UUID `gorm:"type:uuid;not null;index:idx_carrier_regions_carrier" json:"carrierConfigId"`
	CountryCode     string    `gorm:"type:varchar(2);not null;index:idx_carrier_regions_country" json:"countryCode"`
	IsPrimary       bool      `gorm:"default:false" json:"isPrimary"`
	Priority        int       `gorm:"default:0" json:"priority"`
	Enabled         bool      `gorm:"default:true;index:idx_carrier_regions_enabled" json:"enabled"`

	// Region-specific settings
	SupportedServices StringArray `gorm:"type:text[]" json:"supportedServices,omitempty"`
	DefaultService    string      `gorm:"type:varchar(100)" json:"defaultService,omitempty"`

	// Handling fees
	HandlingFee         float64 `gorm:"type:decimal(10,2);default:0" json:"handlingFee"`
	HandlingFeePercent  float64 `gorm:"type:decimal(5,4);default:0" json:"handlingFeePercent"`
	FreeShippingMinimum float64 `gorm:"type:decimal(10,2)" json:"freeShippingMinimum,omitempty"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`

	// Relationships
	CarrierConfig *ShippingCarrierConfig `gorm:"foreignKey:CarrierConfigID" json:"carrierConfig,omitempty"`
}

// TableName specifies the table name for ShippingCarrierRegion
func (ShippingCarrierRegion) TableName() string {
	return "shipping_carrier_regions"
}

// ShippingCarrierTemplate represents pre-configured templates for easy carrier setup
type ShippingCarrierTemplate struct {
	ID          uuid.UUID   `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	CarrierType CarrierType `gorm:"type:varchar(50);not null;unique" json:"carrierType"`
	DisplayName string      `gorm:"type:varchar(255);not null" json:"displayName"`
	Description string      `gorm:"type:text" json:"description,omitempty"`
	LogoURL     string      `gorm:"type:varchar(500)" json:"logoUrl,omitempty"`

	// Supported features
	SupportsRates    bool `gorm:"default:true" json:"supportsRates"`
	SupportsTracking bool `gorm:"default:true" json:"supportsTracking"`
	SupportsLabels   bool `gorm:"default:true" json:"supportsLabels"`
	SupportsReturns  bool `gorm:"default:false" json:"supportsReturns"`
	SupportsPickup   bool `gorm:"default:false" json:"supportsPickup"`

	// Regional support
	SupportedCountries StringArray `gorm:"type:text[];not null" json:"supportedCountries"`
	SupportedServices  StringArray `gorm:"type:text[]" json:"supportedServices,omitempty"`

	// Default configuration
	DefaultConfig JSONB `gorm:"type:jsonb" json:"defaultConfig,omitempty"`

	// Required credentials
	RequiredCredentials StringArray `gorm:"type:text[];default:'{\"api_key\"}'" json:"requiredCredentials"`

	// Documentation
	SetupInstructions string `gorm:"type:text" json:"setupInstructions,omitempty"`
	DocumentationURL  string `gorm:"type:varchar(500)" json:"documentationUrl,omitempty"`

	// Display
	Priority int  `gorm:"default:0" json:"priority"`
	IsActive bool `gorm:"default:true" json:"isActive"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
}

// TableName specifies the table name for ShippingCarrierTemplate
func (ShippingCarrierTemplate) TableName() string {
	return "shipping_carrier_templates"
}

// ShippingSettings represents global shipping settings per tenant
type ShippingSettings struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID string    `gorm:"type:varchar(255);not null;unique;index:idx_shipping_settings_tenant" json:"tenantId"`

	// Default Warehouse Address
	DefaultWarehouseID uuid.UUID `gorm:"type:uuid" json:"defaultWarehouseId,omitempty"`

	// Warehouse address fields (embedded, not a separate table)
	WarehouseName       string `gorm:"type:varchar(255)" json:"warehouseName,omitempty"`
	WarehouseCompany    string `gorm:"type:varchar(255)" json:"warehouseCompany,omitempty"`
	WarehousePhone      string `gorm:"type:varchar(50)" json:"warehousePhone,omitempty"`
	WarehouseEmail      string `gorm:"type:varchar(255)" json:"warehouseEmail,omitempty"`
	WarehouseStreet     string `gorm:"type:varchar(500)" json:"warehouseStreet,omitempty"`
	WarehouseStreet2    string `gorm:"type:varchar(500)" json:"warehouseStreet2,omitempty"`
	WarehouseCity       string `gorm:"type:varchar(255)" json:"warehouseCity,omitempty"`
	WarehouseState      string `gorm:"type:varchar(100)" json:"warehouseState,omitempty"`
	WarehousePostalCode string `gorm:"type:varchar(20)" json:"warehousePostalCode,omitempty"`
	WarehouseCountry    string `gorm:"type:varchar(100)" json:"warehouseCountry,omitempty"`

	// Auto-selection behavior
	AutoSelectCarrier     bool   `gorm:"default:true" json:"autoSelectCarrier"`
	PreferredCarrierType  string `gorm:"type:varchar(50)" json:"preferredCarrierType,omitempty"`
	FallbackCarrierType   string `gorm:"type:varchar(50)" json:"fallbackCarrierType,omitempty"`
	SelectionStrategy     string `gorm:"type:varchar(50);default:'priority'" json:"selectionStrategy"` // priority, cheapest, fastest

	// Shipping fees
	FreeShippingEnabled bool    `gorm:"default:false" json:"freeShippingEnabled"`
	FreeShippingMinimum float64 `gorm:"type:decimal(10,2)" json:"freeShippingMinimum,omitempty"`
	HandlingFee         float64 `gorm:"type:decimal(10,2);default:0" json:"handlingFee"`
	HandlingFeePercent  float64 `gorm:"type:decimal(5,4);default:1.5" json:"handlingFeePercent"` // Default 150% markup

	// Package defaults
	DefaultWeightUnit    string  `gorm:"type:varchar(10);default:'kg'" json:"defaultWeightUnit"`
	DefaultDimensionUnit string  `gorm:"type:varchar(10);default:'cm'" json:"defaultDimensionUnit"`
	DefaultPackageWeight float64 `gorm:"type:decimal(10,2);default:0.5" json:"defaultPackageWeight"`

	// Insurance
	InsuranceEnabled        bool    `gorm:"default:false" json:"insuranceEnabled"`
	InsuranceMinValue       float64 `gorm:"type:decimal(10,2)" json:"insuranceMinValue,omitempty"`
	InsurancePercentage     float64 `gorm:"type:decimal(5,4);default:0.01" json:"insurancePercentage"`
	AutoInsureAboveValue    float64 `gorm:"type:decimal(10,2)" json:"autoInsureAboveValue,omitempty"`

	// Notifications
	SendShipmentNotifications bool `gorm:"default:true" json:"sendShipmentNotifications"`
	SendDeliveryNotifications bool `gorm:"default:true" json:"sendDeliveryNotifications"`
	SendTrackingUpdates       bool `gorm:"default:true" json:"sendTrackingUpdates"`

	// Returns
	ReturnsEnabled     bool   `gorm:"default:true" json:"returnsEnabled"`
	ReturnWindowDays   int    `gorm:"default:30" json:"returnWindowDays"`
	FreeReturnsEnabled bool   `gorm:"default:false" json:"freeReturnsEnabled"`
	ReturnLabelMode    string `gorm:"type:varchar(50);default:'on_request'" json:"returnLabelMode"` // on_request, with_shipment

	// Rate caching
	CacheRates        bool `gorm:"default:true" json:"cacheRates"`
	RateCacheDuration int  `gorm:"default:3600" json:"rateCacheDuration"` // seconds

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
}

// TableName specifies the table name for ShippingSettings
func (ShippingSettings) TableName() string {
	return "shipping_settings"
}

// GetWarehouse returns the warehouse address as a nested object
func (s *ShippingSettings) GetWarehouse() *WarehouseAddress {
	return &WarehouseAddress{
		Name:       s.WarehouseName,
		Company:    s.WarehouseCompany,
		Phone:      s.WarehousePhone,
		Email:      s.WarehouseEmail,
		Street:     s.WarehouseStreet,
		Street2:    s.WarehouseStreet2,
		City:       s.WarehouseCity,
		State:      s.WarehouseState,
		PostalCode: s.WarehousePostalCode,
		Country:    s.WarehouseCountry,
	}
}

// ShippingSettingsResponse is used for API responses with nested warehouse object
type ShippingSettingsResponse struct {
	ID                        uuid.UUID         `json:"id"`
	TenantID                  string            `json:"tenantId"`
	Warehouse                 *WarehouseAddress `json:"warehouse,omitempty"`
	AutoSelectCarrier         bool              `json:"autoSelectCarrier"`
	PreferredCarrierType      string            `json:"preferredCarrierType,omitempty"`
	FallbackCarrierType       string            `json:"fallbackCarrierType,omitempty"`
	SelectionStrategy         string            `json:"selectionStrategy"`
	FreeShippingEnabled       bool              `json:"freeShippingEnabled"`
	FreeShippingMinimum       float64           `json:"freeShippingMinimum,omitempty"`
	HandlingFee               float64           `json:"handlingFee"`
	HandlingFeePercent        float64           `json:"handlingFeePercent"`
	DefaultWeightUnit         string            `json:"defaultWeightUnit"`
	DefaultDimensionUnit      string            `json:"defaultDimensionUnit"`
	DefaultPackageWeight      float64           `json:"defaultPackageWeight"`
	InsuranceEnabled          bool              `json:"insuranceEnabled"`
	InsuranceMinValue         float64           `json:"insuranceMinValue,omitempty"`
	InsurancePercentage       float64           `json:"insurancePercentage"`
	AutoInsureAboveValue      float64           `json:"autoInsureAboveValue,omitempty"`
	SendShipmentNotifications bool              `json:"sendShipmentNotifications"`
	SendDeliveryNotifications bool              `json:"sendDeliveryNotifications"`
	SendTrackingUpdates       bool              `json:"sendTrackingUpdates"`
	ReturnsEnabled            bool              `json:"returnsEnabled"`
	ReturnWindowDays          int               `json:"returnWindowDays"`
	FreeReturnsEnabled        bool              `json:"freeReturnsEnabled"`
	ReturnLabelMode           string            `json:"returnLabelMode"`
	CacheRates                bool              `json:"cacheRates"`
	RateCacheDuration         int               `json:"rateCacheDuration"`
	CreatedAt                 time.Time         `json:"createdAt"`
	UpdatedAt                 time.Time         `json:"updatedAt"`
}

// ToResponse converts ShippingSettings to ShippingSettingsResponse
func (s *ShippingSettings) ToResponse() *ShippingSettingsResponse {
	return &ShippingSettingsResponse{
		ID:                        s.ID,
		TenantID:                  s.TenantID,
		Warehouse:                 s.GetWarehouse(),
		AutoSelectCarrier:         s.AutoSelectCarrier,
		PreferredCarrierType:      s.PreferredCarrierType,
		FallbackCarrierType:       s.FallbackCarrierType,
		SelectionStrategy:         s.SelectionStrategy,
		FreeShippingEnabled:       s.FreeShippingEnabled,
		FreeShippingMinimum:       s.FreeShippingMinimum,
		HandlingFee:               s.HandlingFee,
		HandlingFeePercent:        s.HandlingFeePercent,
		DefaultWeightUnit:         s.DefaultWeightUnit,
		DefaultDimensionUnit:      s.DefaultDimensionUnit,
		DefaultPackageWeight:      s.DefaultPackageWeight,
		InsuranceEnabled:          s.InsuranceEnabled,
		InsuranceMinValue:         s.InsuranceMinValue,
		InsurancePercentage:       s.InsurancePercentage,
		AutoInsureAboveValue:      s.AutoInsureAboveValue,
		SendShipmentNotifications: s.SendShipmentNotifications,
		SendDeliveryNotifications: s.SendDeliveryNotifications,
		SendTrackingUpdates:       s.SendTrackingUpdates,
		ReturnsEnabled:            s.ReturnsEnabled,
		ReturnWindowDays:          s.ReturnWindowDays,
		FreeReturnsEnabled:        s.FreeReturnsEnabled,
		ReturnLabelMode:           s.ReturnLabelMode,
		CacheRates:                s.CacheRates,
		RateCacheDuration:         s.RateCacheDuration,
		CreatedAt:                 s.CreatedAt,
		UpdatedAt:                 s.UpdatedAt,
	}
}

// CarrierConfigResponse is used for API responses (excludes sensitive fields)
type CarrierConfigResponse struct {
	ID                 uuid.UUID              `json:"id"`
	TenantID           string                 `json:"tenantId"`
	CarrierType        CarrierType            `json:"carrierType"`
	DisplayName        string                 `json:"displayName"`
	IsEnabled          bool                   `json:"isEnabled"`
	IsTestMode         bool                   `json:"isTestMode"`
	HasCredentials     bool                   `json:"hasCredentials"`
	SupportsRates      bool                   `json:"supportsRates"`
	SupportsTracking   bool                   `json:"supportsTracking"`
	SupportsLabels     bool                   `json:"supportsLabels"`
	SupportsReturns    bool                   `json:"supportsReturns"`
	SupportsPickup     bool                   `json:"supportsPickup"`
	SupportedCountries StringArray            `json:"supportedCountries"`
	SupportedServices  StringArray            `json:"supportedServices"`
	Priority           int                    `json:"priority"`
	Description        string                 `json:"description"`
	LogoURL            string                 `json:"logoUrl,omitempty"`
	Config             JSONB                  `json:"config,omitempty"`
	Regions            []ShippingCarrierRegion `json:"regions,omitempty"`
	CreatedAt          time.Time              `json:"createdAt"`
	UpdatedAt          time.Time              `json:"updatedAt"`
}

// ToResponse converts ShippingCarrierConfig to CarrierConfigResponse
func (c *ShippingCarrierConfig) ToResponse() CarrierConfigResponse {
	return CarrierConfigResponse{
		ID:                 c.ID,
		TenantID:           c.TenantID,
		CarrierType:        c.CarrierType,
		DisplayName:        c.DisplayName,
		IsEnabled:          c.IsEnabled,
		IsTestMode:         c.IsTestMode,
		HasCredentials:     c.HasCredentials(),
		SupportsRates:      c.SupportsRates,
		SupportsTracking:   c.SupportsTracking,
		SupportsLabels:     c.SupportsLabels,
		SupportsReturns:    c.SupportsReturns,
		SupportsPickup:     c.SupportsPickup,
		SupportedCountries: c.SupportedCountries,
		SupportedServices:  c.SupportedServices,
		Priority:           c.Priority,
		Description:        c.Description,
		LogoURL:            c.LogoURL,
		Config:             c.Config,
		Regions:            c.Regions,
		CreatedAt:          c.CreatedAt,
		UpdatedAt:          c.UpdatedAt,
	}
}

// CreateCarrierConfigRequest represents a request to create a carrier configuration
type CreateCarrierConfigRequest struct {
	CarrierType        CarrierType `json:"carrierType" binding:"required"`
	DisplayName        string      `json:"displayName" binding:"required"`
	IsEnabled          bool        `json:"isEnabled"`
	IsTestMode         bool        `json:"isTestMode"`
	APIKeyPublic       string      `json:"apiKeyPublic"`
	APIKeySecret       string      `json:"apiKeySecret"`
	WebhookSecret      string      `json:"webhookSecret"`
	BaseURL            string      `json:"baseUrl"`
	Credentials        JSONB       `json:"credentials"` // Carrier-specific credentials
	Config             JSONB       `json:"config"`
	SupportedCountries []string    `json:"supportedCountries"`
	SupportedServices  []string    `json:"supportedServices"`
	Priority           int         `json:"priority"`
	Description        string      `json:"description"`
}

// UpdateCarrierConfigRequest represents a request to update a carrier configuration
type UpdateCarrierConfigRequest struct {
	DisplayName        *string  `json:"displayName"`
	IsEnabled          *bool    `json:"isEnabled"`
	IsTestMode         *bool    `json:"isTestMode"`
	APIKeyPublic       *string  `json:"apiKeyPublic"`
	APIKeySecret       *string  `json:"apiKeySecret"`
	WebhookSecret      *string  `json:"webhookSecret"`
	BaseURL            *string  `json:"baseUrl"`
	Credentials        JSONB    `json:"credentials"`
	Config             JSONB    `json:"config"`
	SupportedCountries []string `json:"supportedCountries"`
	SupportedServices  []string `json:"supportedServices"`
	Priority           *int     `json:"priority"`
	Description        *string  `json:"description"`
}

// CreateCarrierFromTemplateRequest represents a request to create a carrier from a template
// Note: CarrierType comes from URL param, not body
type CreateCarrierFromTemplateRequest struct {
	DisplayName   string `json:"displayName"`
	IsTestMode    bool   `json:"isTestMode"`
	APIKeyPublic  string `json:"apiKeyPublic"`
	APIKeySecret  string `json:"apiKeySecret"`
	WebhookSecret string `json:"webhookSecret"`
	BaseURL       string `json:"baseUrl"`
	Credentials   JSONB  `json:"credentials"` // Carrier-specific credentials (email, password, etc.)
}

// CreateCarrierRegionRequest represents a request to create a carrier region mapping
// Note: CarrierConfigID is obtained from the URL path parameter, not the request body
type CreateCarrierRegionRequest struct {
	CountryCode         string   `json:"countryCode" binding:"required,len=2"`
	IsPrimary           bool     `json:"isPrimary"`
	Priority            int      `json:"priority"`
	Enabled             bool     `json:"enabled"`
	SupportedServices   []string `json:"supportedServices"`
	DefaultService      string   `json:"defaultService"`
	HandlingFee         float64  `json:"handlingFee"`
	HandlingFeePercent  float64  `json:"handlingFeePercent"`
	FreeShippingMinimum float64  `json:"freeShippingMinimum"`
}

// UpdateCarrierRegionRequest represents a request to update a carrier region mapping
type UpdateCarrierRegionRequest struct {
	IsPrimary           *bool    `json:"isPrimary"`
	Priority            *int     `json:"priority"`
	Enabled             *bool    `json:"enabled"`
	SupportedServices   []string `json:"supportedServices"`
	DefaultService      *string  `json:"defaultService"`
	HandlingFee         *float64 `json:"handlingFee"`
	HandlingFeePercent  *float64 `json:"handlingFeePercent"`
	FreeShippingMinimum *float64 `json:"freeShippingMinimum"`
}

// WarehouseAddress represents a warehouse/ship-from address
type WarehouseAddress struct {
	Name       string `json:"name,omitempty"`
	Company    string `json:"company,omitempty"`
	Phone      string `json:"phone,omitempty"`
	Email      string `json:"email,omitempty"`
	Street     string `json:"street,omitempty"`
	Street2    string `json:"street2,omitempty"`
	City       string `json:"city,omitempty"`
	State      string `json:"state,omitempty"`
	PostalCode string `json:"postalCode,omitempty"`
	Country    string `json:"country,omitempty"`
}

// UpdateShippingSettingsRequest represents a request to update shipping settings
type UpdateShippingSettingsRequest struct {
	DefaultWarehouseID        *uuid.UUID        `json:"defaultWarehouseId"`
	Warehouse                 *WarehouseAddress `json:"warehouse"`
	AutoSelectCarrier         *bool             `json:"autoSelectCarrier"`
	PreferredCarrierType      *string           `json:"preferredCarrierType"`
	FallbackCarrierType       *string           `json:"fallbackCarrierType"`
	SelectionStrategy         *string           `json:"selectionStrategy"`
	FreeShippingEnabled       *bool             `json:"freeShippingEnabled"`
	FreeShippingMinimum       *float64          `json:"freeShippingMinimum"`
	HandlingFee               *float64          `json:"handlingFee"`
	HandlingFeePercent        *float64          `json:"handlingFeePercent"`
	DefaultWeightUnit         *string           `json:"defaultWeightUnit"`
	DefaultDimensionUnit      *string           `json:"defaultDimensionUnit"`
	DefaultPackageWeight      *float64          `json:"defaultPackageWeight"`
	InsuranceEnabled          *bool             `json:"insuranceEnabled"`
	InsuranceMinValue         *float64          `json:"insuranceMinValue"`
	InsurancePercentage       *float64          `json:"insurancePercentage"`
	AutoInsureAboveValue      *float64          `json:"autoInsureAboveValue"`
	SendShipmentNotifications *bool             `json:"sendShipmentNotifications"`
	SendDeliveryNotifications *bool             `json:"sendDeliveryNotifications"`
	SendTrackingUpdates       *bool             `json:"sendTrackingUpdates"`
	ReturnsEnabled            *bool             `json:"returnsEnabled"`
	ReturnWindowDays          *int              `json:"returnWindowDays"`
	FreeReturnsEnabled        *bool             `json:"freeReturnsEnabled"`
	ReturnLabelMode           *string           `json:"returnLabelMode"`
	CacheRates                *bool             `json:"cacheRates"`
	RateCacheDuration         *int              `json:"rateCacheDuration"`
}

// ValidateCredentialsRequest represents a request to validate carrier credentials
type ValidateCredentialsRequest struct {
	CarrierType  CarrierType `json:"carrierType" binding:"required"`
	IsTestMode   bool        `json:"isTestMode"`
	APIKeyPublic string      `json:"apiKeyPublic"`
	APIKeySecret string      `json:"apiKeySecret"`
	BaseURL      string      `json:"baseUrl"`
	Credentials  JSONB       `json:"credentials"`
}

// ValidateCredentialsResponse represents the response from credential validation
type ValidateCredentialsResponse struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message,omitempty"`
	Details JSONB  `json:"details,omitempty"` // Additional info from carrier (e.g., account name)
}
