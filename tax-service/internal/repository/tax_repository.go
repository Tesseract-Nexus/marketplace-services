package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"tax-service/internal/models"
	"gorm.io/gorm"
)

// TaxRepository handles tax data operations
type TaxRepository struct {
	db *gorm.DB
}

// NewTaxRepository creates a new tax repository
func NewTaxRepository(db *gorm.DB) *TaxRepository {
	return &TaxRepository{db: db}
}

// GetJurisdictionByLocation finds jurisdictions matching the given address (including global data)
func (r *TaxRepository) GetJurisdictionByLocation(ctx context.Context, tenantID, country, state, city, zip string) ([]models.TaxJurisdiction, error) {
	var jurisdictions []models.TaxJurisdiction

	// Query hierarchically: ZIP > City > State > Country (includes global tenant data)
	query := r.db.WithContext(ctx).Where("tenant_id IN ? AND is_active = true", []string{tenantID, GlobalTenantID})

	// Try ZIP first if provided - always filter country to avoid matching all countries
	if zip != "" {
		query = query.Where("(type = ? AND code = ?) OR (type = ? AND code = ?) OR (type = ? AND code = ?) OR (type = ? AND code = ?)",
			models.JurisdictionTypeZIP, zip,
			models.JurisdictionTypeCity, city,
			models.JurisdictionTypeState, state,
			models.JurisdictionTypeCountry, country)
	} else if city != "" {
		query = query.Where("(type = ? AND code = ?) OR (type = ? AND code = ?) OR (type = ? AND code = ?)",
			models.JurisdictionTypeCity, city,
			models.JurisdictionTypeState, state,
			models.JurisdictionTypeCountry, country)
	} else if state != "" {
		query = query.Where("(type = ? AND code = ?) OR (type = ? AND code = ?)",
			models.JurisdictionTypeState, state,
			models.JurisdictionTypeCountry, country)
	} else {
		query = query.Where("type = ? AND code = ?", models.JurisdictionTypeCountry, country)
	}

	err := query.Order("type DESC").Find(&jurisdictions).Error
	return jurisdictions, err
}

// GetActiveTaxRates gets all active tax rates for given jurisdictions
func (r *TaxRepository) GetActiveTaxRates(ctx context.Context, jurisdictionIDs []uuid.UUID) ([]models.TaxRate, error) {
	var rates []models.TaxRate

	now := time.Now()
	err := r.db.WithContext(ctx).
		Where("jurisdiction_id IN ? AND is_active = true", jurisdictionIDs).
		Where("effective_from <= ?", now).
		Where("effective_to IS NULL OR effective_to >= ?", now).
		Order("priority ASC").
		Find(&rates).Error

	return rates, err
}

// GetTaxRatesForJurisdictionAndCategory gets tax rates with category overrides
func (r *TaxRepository) GetTaxRatesForJurisdictionAndCategory(ctx context.Context, jurisdictionID, categoryID uuid.UUID) ([]models.TaxRate, []models.TaxRateCategoryOverride, error) {
	var rates []models.TaxRate
	var overrides []models.TaxRateCategoryOverride

	now := time.Now()

	// Get base rates
	err := r.db.WithContext(ctx).
		Where("jurisdiction_id = ? AND is_active = true", jurisdictionID).
		Where("effective_from <= ?", now).
		Where("effective_to IS NULL OR effective_to >= ?", now).
		Order("priority ASC").
		Find(&rates).Error

	if err != nil {
		return nil, nil, err
	}

	// Get category overrides
	if categoryID != uuid.Nil {
		rateIDs := make([]uuid.UUID, len(rates))
		for i, rate := range rates {
			rateIDs[i] = rate.ID
		}

		err = r.db.WithContext(ctx).
			Where("tax_rate_id IN ? AND category_id = ?", rateIDs, categoryID).
			Find(&overrides).Error
	}

	return rates, overrides, err
}

// GetCustomerExemption checks if a customer has a valid tax exemption
func (r *TaxRepository) GetCustomerExemption(ctx context.Context, tenantID string, customerID uuid.UUID) (*models.TaxExemptionCertificate, error) {
	var exemption models.TaxExemptionCertificate

	now := time.Now()
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND customer_id = ?", tenantID, customerID).
		Where("status = ?", models.CertificateStatusActive).
		Where("issued_date <= ?", now).
		Where("expiry_date IS NULL OR expiry_date >= ?", now).
		First(&exemption).Error

	if err != nil {
		return nil, err
	}

	return &exemption, nil
}

// GetProductCategory gets a product tax category by ID
func (r *TaxRepository) GetProductCategory(ctx context.Context, categoryID uuid.UUID) (*models.ProductTaxCategory, error) {
	var category models.ProductTaxCategory
	err := r.db.WithContext(ctx).First(&category, "id = ?", categoryID).Error
	if err != nil {
		return nil, err
	}
	return &category, nil
}

// GetCachedTaxCalculation retrieves a cached tax calculation
func (r *TaxRepository) GetCachedTaxCalculation(ctx context.Context, cacheKey string) (*models.TaxCalculationCache, error) {
	var cache models.TaxCalculationCache

	err := r.db.WithContext(ctx).
		Where("cache_key = ? AND expires_at > ?", cacheKey, time.Now()).
		First(&cache).Error

	if err != nil {
		return nil, err
	}

	return &cache, nil
}

// CacheTaxCalculation stores a tax calculation in cache
func (r *TaxRepository) CacheTaxCalculation(ctx context.Context, cache *models.TaxCalculationCache) error {
	return r.db.WithContext(ctx).Create(cache).Error
}

// CreateTaxRate creates a new tax rate
func (r *TaxRepository) CreateTaxRate(ctx context.Context, rate *models.TaxRate) error {
	return r.db.WithContext(ctx).Create(rate).Error
}

// UpdateTaxRate updates a tax rate
func (r *TaxRepository) UpdateTaxRate(ctx context.Context, rate *models.TaxRate) error {
	rate.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(rate).Error
}

// DeleteTaxRate soft deletes a tax rate (marks as inactive)
func (r *TaxRepository) DeleteTaxRate(ctx context.Context, rateID uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&models.TaxRate{}).
		Where("id = ?", rateID).
		Update("is_active", false).Error
}

// ListTaxRates lists all tax rates for a jurisdiction
func (r *TaxRepository) ListTaxRates(ctx context.Context, jurisdictionID uuid.UUID) ([]models.TaxRate, error) {
	var rates []models.TaxRate
	err := r.db.WithContext(ctx).
		Where("jurisdiction_id = ?", jurisdictionID).
		Order("priority ASC").
		Find(&rates).Error
	return rates, err
}

// CreateJurisdiction creates a new jurisdiction
func (r *TaxRepository) CreateJurisdiction(ctx context.Context, jurisdiction *models.TaxJurisdiction) error {
	return r.db.WithContext(ctx).Create(jurisdiction).Error
}

// UpdateJurisdiction updates a jurisdiction
func (r *TaxRepository) UpdateJurisdiction(ctx context.Context, jurisdiction *models.TaxJurisdiction) error {
	jurisdiction.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(jurisdiction).Error
}

// DeleteJurisdiction soft deletes a jurisdiction
func (r *TaxRepository) DeleteJurisdiction(ctx context.Context, jurisdictionID uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&models.TaxJurisdiction{}).
		Where("id = ?", jurisdictionID).
		Update("is_active", false).Error
}

// GetJurisdiction gets a jurisdiction by ID
func (r *TaxRepository) GetJurisdiction(ctx context.Context, jurisdictionID uuid.UUID) (*models.TaxJurisdiction, error) {
	var jurisdiction models.TaxJurisdiction
	err := r.db.WithContext(ctx).
		Preload("Parent").
		Preload("Children").
		Preload("TaxRates").
		First(&jurisdiction, "id = ?", jurisdictionID).Error
	if err != nil {
		return nil, err
	}
	return &jurisdiction, nil
}

// GlobalTenantID is the special tenant ID for global data accessible to all tenants
const GlobalTenantID = "global"

// ListJurisdictions lists all jurisdictions for a tenant (including global data)
func (r *TaxRepository) ListJurisdictions(ctx context.Context, tenantID string) ([]models.TaxJurisdiction, error) {
	var jurisdictions []models.TaxJurisdiction
	err := r.db.WithContext(ctx).
		Where("tenant_id IN ?", []string{tenantID, GlobalTenantID}).
		Preload("TaxRates").
		Order("type, name").
		Find(&jurisdictions).Error
	return jurisdictions, err
}

// CreateProductCategory creates a new product tax category
func (r *TaxRepository) CreateProductCategory(ctx context.Context, category *models.ProductTaxCategory) error {
	return r.db.WithContext(ctx).Create(category).Error
}

// UpdateProductCategory updates a product tax category
func (r *TaxRepository) UpdateProductCategory(ctx context.Context, category *models.ProductTaxCategory) error {
	category.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(category).Error
}

// DeleteProductCategory deletes a product tax category
func (r *TaxRepository) DeleteProductCategory(ctx context.Context, categoryID uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.ProductTaxCategory{}, "id = ?", categoryID).Error
}

// ListProductCategories lists all product tax categories (including global data)
func (r *TaxRepository) ListProductCategories(ctx context.Context, tenantID string) ([]models.ProductTaxCategory, error) {
	var categories []models.ProductTaxCategory
	err := r.db.WithContext(ctx).
		Where("tenant_id IN ?", []string{tenantID, GlobalTenantID}).
		Order("name").
		Find(&categories).Error
	return categories, err
}

// CreateExemptionCertificate creates a new exemption certificate
func (r *TaxRepository) CreateExemptionCertificate(ctx context.Context, cert *models.TaxExemptionCertificate) error {
	return r.db.WithContext(ctx).Create(cert).Error
}

// UpdateExemptionCertificate updates an exemption certificate
func (r *TaxRepository) UpdateExemptionCertificate(ctx context.Context, cert *models.TaxExemptionCertificate) error {
	cert.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Save(cert).Error
}

// GetExemptionCertificate gets an exemption certificate by ID
func (r *TaxRepository) GetExemptionCertificate(ctx context.Context, certID uuid.UUID) (*models.TaxExemptionCertificate, error) {
	var cert models.TaxExemptionCertificate
	err := r.db.WithContext(ctx).First(&cert, "id = ?", certID).Error
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

// ListExemptionCertificates lists exemption certificates for a customer
func (r *TaxRepository) ListExemptionCertificates(ctx context.Context, tenantID string, customerID uuid.UUID) ([]models.TaxExemptionCertificate, error) {
	var certs []models.TaxExemptionCertificate
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND customer_id = ?", tenantID, customerID).
		Order("created_at DESC").
		Find(&certs).Error
	return certs, err
}

// CreateTaxNexus creates a new tax nexus
func (r *TaxRepository) CreateTaxNexus(ctx context.Context, nexus *models.TaxNexus) error {
	return r.db.WithContext(ctx).Create(nexus).Error
}

// ListTaxNexus lists all tax nexus for a tenant
func (r *TaxRepository) ListTaxNexus(ctx context.Context, tenantID string) ([]models.TaxNexus, error) {
	var nexus []models.TaxNexus
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Find(&nexus).Error
	return nexus, err
}

// GetNexusByJurisdiction checks if tenant has nexus in a jurisdiction
func (r *TaxRepository) GetNexusByJurisdiction(ctx context.Context, tenantID string, jurisdictionID uuid.UUID) (*models.TaxNexus, error) {
	var nexus models.TaxNexus
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND jurisdiction_id = ? AND is_active = true", tenantID, jurisdictionID).
		Preload("Jurisdiction").
		First(&nexus).Error
	if err != nil {
		return nil, err
	}
	return &nexus, nil
}

// GetNexusByCountry checks if tenant has nexus in a country
func (r *TaxRepository) GetNexusByCountry(ctx context.Context, tenantID string, countryCode string) (*models.TaxNexus, error) {
	var nexus models.TaxNexus
	err := r.db.WithContext(ctx).
		Joins("JOIN tax_jurisdictions ON tax_jurisdictions.id = tax_nexus.jurisdiction_id").
		Where("tax_nexus.tenant_id = ? AND tax_jurisdictions.code = ? AND tax_jurisdictions.type = ? AND tax_nexus.is_active = true",
			tenantID, countryCode, models.JurisdictionTypeCountry).
		Preload("Jurisdiction").
		First(&nexus).Error
	if err != nil {
		return nil, err
	}
	return &nexus, nil
}

// GetJurisdictionByStateCode gets a jurisdiction by state code (for India GST, includes global data)
func (r *TaxRepository) GetJurisdictionByStateCode(ctx context.Context, tenantID string, stateCode string) (*models.TaxJurisdiction, error) {
	var jurisdiction models.TaxJurisdiction
	err := r.db.WithContext(ctx).
		Where("tenant_id IN ? AND state_code = ? AND is_active = true", []string{tenantID, GlobalTenantID}, stateCode).
		First(&jurisdiction).Error
	if err != nil {
		return nil, err
	}
	return &jurisdiction, nil
}

// GetTaxRatesByTypeAndSlab gets tax rates by type and GST slab (for India GST)
func (r *TaxRepository) GetTaxRatesByTypeAndSlab(ctx context.Context, jurisdictionID uuid.UUID, taxType models.TaxType, rate float64) ([]models.TaxRate, error) {
	var rates []models.TaxRate
	now := time.Now()
	err := r.db.WithContext(ctx).
		Where("jurisdiction_id = ? AND tax_type = ? AND rate = ? AND is_active = true", jurisdictionID, taxType, rate).
		Where("effective_from <= ?", now).
		Where("effective_to IS NULL OR effective_to >= ?", now).
		Order("priority ASC").
		Find(&rates).Error
	return rates, err
}

// GetCountryJurisdiction gets the country jurisdiction for a given country code (includes global data)
func (r *TaxRepository) GetCountryJurisdiction(ctx context.Context, tenantID string, countryCode string) (*models.TaxJurisdiction, error) {
	var jurisdiction models.TaxJurisdiction
	err := r.db.WithContext(ctx).
		Where("tenant_id IN ? AND code = ? AND type = ? AND is_active = true", []string{tenantID, GlobalTenantID}, countryCode, models.JurisdictionTypeCountry).
		First(&jurisdiction).Error
	if err != nil {
		return nil, err
	}
	return &jurisdiction, nil
}

// GetProductCategoryByHSN gets a product category by HSN code (India, includes global data)
func (r *TaxRepository) GetProductCategoryByHSN(ctx context.Context, tenantID string, hsnCode string) (*models.ProductTaxCategory, error) {
	var category models.ProductTaxCategory
	err := r.db.WithContext(ctx).
		Where("tenant_id IN ? AND hsn_code = ?", []string{tenantID, GlobalTenantID}, hsnCode).
		First(&category).Error
	if err != nil {
		return nil, err
	}
	return &category, nil
}

// GetProductCategoryBySAC gets a product category by SAC code (India services, includes global data)
func (r *TaxRepository) GetProductCategoryBySAC(ctx context.Context, tenantID string, sacCode string) (*models.ProductTaxCategory, error) {
	var category models.ProductTaxCategory
	err := r.db.WithContext(ctx).
		Where("tenant_id IN ? AND sac_code = ?", []string{tenantID, GlobalTenantID}, sacCode).
		First(&category).Error
	if err != nil {
		return nil, err
	}
	return &category, nil
}
