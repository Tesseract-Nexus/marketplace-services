package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"shipping-service/internal/models"
	"gorm.io/gorm"
)

// CarrierConfigRepository handles carrier configuration database operations
type CarrierConfigRepository struct {
	db *gorm.DB
}

// NewCarrierConfigRepository creates a new carrier config repository
func NewCarrierConfigRepository(db *gorm.DB) *CarrierConfigRepository {
	return &CarrierConfigRepository{db: db}
}

// ==================== Carrier Config Methods ====================

// GetCarrierConfig gets a carrier configuration by ID
func (r *CarrierConfigRepository) GetCarrierConfig(ctx context.Context, configID uuid.UUID) (*models.ShippingCarrierConfig, error) {
	var config models.ShippingCarrierConfig
	err := r.db.WithContext(ctx).
		Preload("Regions").
		First(&config, "id = ?", configID).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// GetCarrierConfigByType gets a carrier configuration by tenant and type
func (r *CarrierConfigRepository) GetCarrierConfigByType(ctx context.Context, tenantID string, carrierType models.CarrierType) (*models.ShippingCarrierConfig, error) {
	var config models.ShippingCarrierConfig
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND carrier_type = ? AND is_enabled = true", tenantID, carrierType).
		Preload("Regions").
		First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// ListCarrierConfigs lists all carrier configurations for a tenant
func (r *CarrierConfigRepository) ListCarrierConfigs(ctx context.Context, tenantID string) ([]models.ShippingCarrierConfig, error) {
	var configs []models.ShippingCarrierConfig
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Preload("Regions").
		Order("priority ASC").
		Find(&configs).Error
	if err != nil {
		return nil, err
	}
	return configs, nil
}

// ListEnabledCarrierConfigs lists all enabled carrier configurations for a tenant
func (r *CarrierConfigRepository) ListEnabledCarrierConfigs(ctx context.Context, tenantID string) ([]models.ShippingCarrierConfig, error) {
	var configs []models.ShippingCarrierConfig
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND is_enabled = true", tenantID).
		Preload("Regions").
		Order("priority ASC").
		Find(&configs).Error
	if err != nil {
		return nil, err
	}
	return configs, nil
}

// GetCarriersForCountry gets carriers that support a specific country
func (r *CarrierConfigRepository) GetCarriersForCountry(ctx context.Context, tenantID string, countryCode string) ([]models.ShippingCarrierConfig, error) {
	var configs []models.ShippingCarrierConfig

	// First try to get carriers with specific region mappings for this country
	err := r.db.WithContext(ctx).
		Joins("JOIN shipping_carrier_regions ON shipping_carrier_configs.id = shipping_carrier_regions.carrier_config_id").
		Where("shipping_carrier_configs.tenant_id = ?", tenantID).
		Where("shipping_carrier_configs.is_enabled = true").
		Where("shipping_carrier_regions.country_code = ?", countryCode).
		Where("shipping_carrier_regions.enabled = true").
		Preload("Regions", "country_code = ?", countryCode).
		Order("shipping_carrier_regions.priority ASC").
		Find(&configs).Error

	if err != nil {
		return nil, err
	}

	// If no specific regions, fall back to carriers that list country in supported_countries
	if len(configs) == 0 {
		err = r.db.WithContext(ctx).
			Where("tenant_id = ? AND is_enabled = true AND ? = ANY(supported_countries)", tenantID, countryCode).
			Order("priority ASC").
			Find(&configs).Error
		if err != nil {
			return nil, err
		}
	}

	return configs, nil
}

// GetPrimaryCarrierForCountry gets the primary carrier for a specific country
func (r *CarrierConfigRepository) GetPrimaryCarrierForCountry(ctx context.Context, tenantID string, countryCode string) (*models.ShippingCarrierConfig, error) {
	var config models.ShippingCarrierConfig

	// Look for a carrier with is_primary = true for this country
	err := r.db.WithContext(ctx).
		Joins("JOIN shipping_carrier_regions ON shipping_carrier_configs.id = shipping_carrier_regions.carrier_config_id").
		Where("shipping_carrier_configs.tenant_id = ?", tenantID).
		Where("shipping_carrier_configs.is_enabled = true").
		Where("shipping_carrier_regions.country_code = ?", countryCode).
		Where("shipping_carrier_regions.enabled = true").
		Where("shipping_carrier_regions.is_primary = true").
		Preload("Regions", "country_code = ?", countryCode).
		First(&config).Error

	if err != nil {
		return nil, err
	}

	return &config, nil
}

// CreateCarrierConfig creates a new carrier configuration
func (r *CarrierConfigRepository) CreateCarrierConfig(ctx context.Context, config *models.ShippingCarrierConfig) error {
	if config.ID == uuid.Nil {
		config.ID = uuid.New()
	}
	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Create(config).Error
}

// UpdateCarrierConfig updates a carrier configuration
func (r *CarrierConfigRepository) UpdateCarrierConfig(ctx context.Context, config *models.ShippingCarrierConfig) error {
	config.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(config).Error
}

// DeleteCarrierConfig deletes a carrier configuration and its regions
func (r *CarrierConfigRepository) DeleteCarrierConfig(ctx context.Context, configID uuid.UUID, tenantID string) error {
	// Verify ownership first
	var config models.ShippingCarrierConfig
	if err := r.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", configID, tenantID).First(&config).Error; err != nil {
		return fmt.Errorf("carrier config not found: %w", err)
	}

	// Delete regions first (cascading should handle this, but being explicit)
	if err := r.db.WithContext(ctx).Where("carrier_config_id = ?", configID).Delete(&models.ShippingCarrierRegion{}).Error; err != nil {
		return err
	}

	// Delete the config
	return r.db.WithContext(ctx).Delete(&models.ShippingCarrierConfig{}, "id = ?", configID).Error
}

// ==================== Carrier Region Methods ====================

// GetCarrierRegion gets a carrier region by ID
func (r *CarrierConfigRepository) GetCarrierRegion(ctx context.Context, regionID uuid.UUID) (*models.ShippingCarrierRegion, error) {
	var region models.ShippingCarrierRegion
	err := r.db.WithContext(ctx).
		Preload("CarrierConfig").
		First(&region, "id = ?", regionID).Error
	if err != nil {
		return nil, err
	}
	return &region, nil
}

// ListCarrierRegions lists all regions for a carrier config
func (r *CarrierConfigRepository) ListCarrierRegions(ctx context.Context, carrierConfigID uuid.UUID) ([]models.ShippingCarrierRegion, error) {
	var regions []models.ShippingCarrierRegion
	err := r.db.WithContext(ctx).
		Where("carrier_config_id = ?", carrierConfigID).
		Order("priority ASC").
		Find(&regions).Error
	if err != nil {
		return nil, err
	}
	return regions, nil
}

// CreateCarrierRegion creates a new carrier region mapping
func (r *CarrierConfigRepository) CreateCarrierRegion(ctx context.Context, region *models.ShippingCarrierRegion) error {
	if region.ID == uuid.Nil {
		region.ID = uuid.New()
	}
	region.CreatedAt = time.Now()
	region.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Create(region).Error
}

// UpdateCarrierRegion updates a carrier region mapping
func (r *CarrierConfigRepository) UpdateCarrierRegion(ctx context.Context, region *models.ShippingCarrierRegion) error {
	region.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(region).Error
}

// DeleteCarrierRegion deletes a carrier region mapping
func (r *CarrierConfigRepository) DeleteCarrierRegion(ctx context.Context, regionID uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.ShippingCarrierRegion{}, "id = ?", regionID).Error
}

// SetPrimaryCarrierForCountry sets a carrier as primary for a country
// This will unset any existing primary carrier for that country within the same tenant
func (r *CarrierConfigRepository) SetPrimaryCarrierForCountry(ctx context.Context, tenantID string, carrierConfigID uuid.UUID, countryCode string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// First, unset any existing primary carriers for this country and tenant
		err := tx.Model(&models.ShippingCarrierRegion{}).
			Joins("JOIN shipping_carrier_configs ON shipping_carrier_regions.carrier_config_id = shipping_carrier_configs.id").
			Where("shipping_carrier_configs.tenant_id = ?", tenantID).
			Where("shipping_carrier_regions.country_code = ?", countryCode).
			Update("is_primary", false).Error
		if err != nil {
			return err
		}

		// Set the specified carrier as primary
		return tx.Model(&models.ShippingCarrierRegion{}).
			Where("carrier_config_id = ? AND country_code = ?", carrierConfigID, countryCode).
			Update("is_primary", true).Error
	})
}

// ==================== Template Methods ====================

// ListCarrierTemplates lists all active carrier templates
func (r *CarrierConfigRepository) ListCarrierTemplates(ctx context.Context) ([]models.ShippingCarrierTemplate, error) {
	var templates []models.ShippingCarrierTemplate
	err := r.db.WithContext(ctx).
		Where("is_active = true").
		Order("priority ASC").
		Find(&templates).Error
	if err != nil {
		return nil, err
	}
	return templates, nil
}

// GetCarrierTemplate gets a carrier template by type
func (r *CarrierConfigRepository) GetCarrierTemplate(ctx context.Context, carrierType models.CarrierType) (*models.ShippingCarrierTemplate, error) {
	var template models.ShippingCarrierTemplate
	err := r.db.WithContext(ctx).
		Where("carrier_type = ? AND is_active = true", carrierType).
		First(&template).Error
	if err != nil {
		return nil, err
	}
	return &template, nil
}

// ==================== Settings Methods ====================

// GetShippingSettings gets shipping settings for a tenant
func (r *CarrierConfigRepository) GetShippingSettings(ctx context.Context, tenantID string) (*models.ShippingSettings, error) {
	var settings models.ShippingSettings
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&settings).Error
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

// GetOrCreateShippingSettings gets or creates shipping settings for a tenant
func (r *CarrierConfigRepository) GetOrCreateShippingSettings(ctx context.Context, tenantID string) (*models.ShippingSettings, error) {
	settings, err := r.GetShippingSettings(ctx, tenantID)
	if err == nil {
		return settings, nil
	}

	// Create default settings
	settings = &models.ShippingSettings{
		ID:                        uuid.New(),
		TenantID:                  tenantID,
		AutoSelectCarrier:         true,
		SelectionStrategy:         "priority",
		DefaultWeightUnit:         "kg",
		DefaultDimensionUnit:      "cm",
		DefaultPackageWeight:      0.5,
		SendShipmentNotifications: true,
		SendDeliveryNotifications: true,
		SendTrackingUpdates:       true,
		ReturnsEnabled:            true,
		ReturnWindowDays:          30,
		ReturnLabelMode:           "on_request",
		CacheRates:                true,
		RateCacheDuration:         3600,
		CreatedAt:                 time.Now(),
		UpdatedAt:                 time.Now(),
	}

	if err := r.db.WithContext(ctx).Create(settings).Error; err != nil {
		return nil, err
	}
	return settings, nil
}

// UpdateShippingSettings updates shipping settings
func (r *CarrierConfigRepository) UpdateShippingSettings(ctx context.Context, settings *models.ShippingSettings) error {
	settings.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(settings).Error
}

// ==================== Utility Methods ====================

// CarrierExistsForTenant checks if a carrier type already exists for a tenant
func (r *CarrierConfigRepository) CarrierExistsForTenant(ctx context.Context, tenantID string, carrierType models.CarrierType) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.ShippingCarrierConfig{}).
		Where("tenant_id = ? AND carrier_type = ?", tenantID, carrierType).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetCountryCarrierMatrix returns a matrix of countries to available carriers for a tenant
func (r *CarrierConfigRepository) GetCountryCarrierMatrix(ctx context.Context, tenantID string) (map[string][]models.ShippingCarrierConfig, error) {
	configs, err := r.ListEnabledCarrierConfigs(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	matrix := make(map[string][]models.ShippingCarrierConfig)

	for _, config := range configs {
		// Add from region mappings
		for _, region := range config.Regions {
			if region.Enabled {
				matrix[region.CountryCode] = append(matrix[region.CountryCode], config)
			}
		}

		// Add from supported_countries if no specific regions
		if len(config.Regions) == 0 {
			for _, country := range config.SupportedCountries {
				matrix[country] = append(matrix[country], config)
			}
		}
	}

	return matrix, nil
}
