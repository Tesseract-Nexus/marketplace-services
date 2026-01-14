package repository

import (
	"context"

	"github.com/google/uuid"
	"orders-service/internal/models"
	"gorm.io/gorm"
)

// ShippingMethodRepository handles database operations for shipping methods
type ShippingMethodRepository struct {
	db *gorm.DB
}

// NewShippingMethodRepository creates a new shipping method repository
func NewShippingMethodRepository(db *gorm.DB) *ShippingMethodRepository {
	return &ShippingMethodRepository{db: db}
}

// ListByTenant returns all active shipping methods for a tenant
func (r *ShippingMethodRepository) ListByTenant(ctx context.Context, tenantID string) ([]models.ShippingMethod, error) {
	var methods []models.ShippingMethod
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND is_active = ?", tenantID, true).
		Order("sort_order ASC, name ASC").
		Find(&methods).Error
	return methods, err
}

// ListAllByTenant returns all shipping methods for a tenant (including inactive)
func (r *ShippingMethodRepository) ListAllByTenant(ctx context.Context, tenantID string) ([]models.ShippingMethod, error) {
	var methods []models.ShippingMethod
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("sort_order ASC, name ASC").
		Find(&methods).Error
	return methods, err
}

// GetByID returns a shipping method by ID
func (r *ShippingMethodRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.ShippingMethod, error) {
	var method models.ShippingMethod
	err := r.db.WithContext(ctx).First(&method, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &method, nil
}

// Create creates a new shipping method
func (r *ShippingMethodRepository) Create(ctx context.Context, method *models.ShippingMethod) error {
	return r.db.WithContext(ctx).Create(method).Error
}

// Update updates a shipping method
func (r *ShippingMethodRepository) Update(ctx context.Context, method *models.ShippingMethod) error {
	return r.db.WithContext(ctx).Save(method).Error
}

// Delete soft deletes a shipping method
func (r *ShippingMethodRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.ShippingMethod{}, "id = ?", id).Error
}

// GetByCountry returns active shipping methods available for a specific country
func (r *ShippingMethodRepository) GetByCountry(ctx context.Context, tenantID string, countryCode string) ([]models.ShippingMethod, error) {
	var methods []models.ShippingMethod
	// Get methods where countries array is empty (all countries) or contains the specified country
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND is_active = ? AND (countries IS NULL OR cardinality(countries) = 0 OR ? = ANY(countries))", tenantID, true, countryCode).
		Order("sort_order ASC, base_rate ASC").
		Find(&methods).Error
	return methods, err
}

// SeedDefaultMethods creates default shipping methods for a tenant if none exist
func (r *ShippingMethodRepository) SeedDefaultMethods(ctx context.Context, tenantID string) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.ShippingMethod{}).Where("tenant_id = ?", tenantID).Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		return nil // Already has shipping methods
	}

	freeThreshold := float64(100)
	expressThreshold := float64(150)

	defaults := []models.ShippingMethod{
		{
			TenantID:              tenantID,
			Name:                  "Standard Shipping",
			Description:           "Delivery in 5-7 business days",
			EstimatedDaysMin:      5,
			EstimatedDaysMax:      7,
			BaseRate:              9.99,
			FreeShippingThreshold: &freeThreshold,
			IsActive:              true,
			SortOrder:             1,
		},
		{
			TenantID:              tenantID,
			Name:                  "Express Shipping",
			Description:           "Delivery in 2-3 business days",
			EstimatedDaysMin:      2,
			EstimatedDaysMax:      3,
			BaseRate:              19.99,
			FreeShippingThreshold: &expressThreshold,
			IsActive:              true,
			SortOrder:             2,
		},
		{
			TenantID:         tenantID,
			Name:             "Next Day Delivery",
			Description:      "Delivery by next business day",
			EstimatedDaysMin: 1,
			EstimatedDaysMax: 1,
			BaseRate:         29.99,
			IsActive:         true,
			SortOrder:        3,
		},
	}

	for _, method := range defaults {
		if err := r.db.WithContext(ctx).Create(&method).Error; err != nil {
			return err
		}
	}

	return nil
}
