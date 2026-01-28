package repository

import (
	"context"
	"fmt"

	"orders-service/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CancellationSettingsRepository handles database operations for cancellation settings
type CancellationSettingsRepository struct {
	db *gorm.DB
}

// NewCancellationSettingsRepository creates a new repository instance
func NewCancellationSettingsRepository(db *gorm.DB) *CancellationSettingsRepository {
	return &CancellationSettingsRepository{db: db}
}

// GetByTenant retrieves cancellation settings for a tenant
func (r *CancellationSettingsRepository) GetByTenant(ctx context.Context, tenantID string) (*models.CancellationSettings, error) {
	var settings models.CancellationSettings
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND deleted_at IS NULL", tenantID).
		First(&settings).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Return nil without error for not found
		}
		return nil, fmt.Errorf("failed to get cancellation settings: %w", err)
	}

	return &settings, nil
}

// GetByTenantAndStorefront retrieves cancellation settings for a specific tenant-storefront combination
func (r *CancellationSettingsRepository) GetByTenantAndStorefront(ctx context.Context, tenantID, storefrontID string) (*models.CancellationSettings, error) {
	var settings models.CancellationSettings
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND storefront_id = ? AND deleted_at IS NULL", tenantID, storefrontID).
		First(&settings).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Try to find tenant-level settings (empty storefront_id)
			err = r.db.WithContext(ctx).
				Where("tenant_id = ? AND (storefront_id = '' OR storefront_id IS NULL) AND deleted_at IS NULL", tenantID).
				First(&settings).Error
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					return nil, nil
				}
				return nil, fmt.Errorf("failed to get cancellation settings: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to get cancellation settings: %w", err)
		}
	}

	return &settings, nil
}

// GetByID retrieves cancellation settings by ID
func (r *CancellationSettingsRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.CancellationSettings, error) {
	var settings models.CancellationSettings
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&settings).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("cancellation settings not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get cancellation settings: %w", err)
	}

	return &settings, nil
}

// Create creates new cancellation settings
func (r *CancellationSettingsRepository) Create(ctx context.Context, settings *models.CancellationSettings) error {
	if settings.ID == uuid.Nil {
		settings.ID = uuid.New()
	}
	return r.db.WithContext(ctx).Create(settings).Error
}

// Update updates existing cancellation settings
func (r *CancellationSettingsRepository) Update(ctx context.Context, settings *models.CancellationSettings) error {
	return r.db.WithContext(ctx).Save(settings).Error
}

// Upsert creates or updates cancellation settings for a tenant-storefront combination
func (r *CancellationSettingsRepository) Upsert(ctx context.Context, settings *models.CancellationSettings) error {
	// Try to find existing settings
	var existing models.CancellationSettings
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND storefront_id = ? AND deleted_at IS NULL", settings.TenantID, settings.StorefrontID).
		First(&existing).Error

	if err == nil {
		// Update existing
		settings.ID = existing.ID
		settings.CreatedAt = existing.CreatedAt
		settings.CreatedBy = existing.CreatedBy
		return r.db.WithContext(ctx).Save(settings).Error
	}

	if err == gorm.ErrRecordNotFound {
		// Create new
		if settings.ID == uuid.Nil {
			settings.ID = uuid.New()
		}
		return r.db.WithContext(ctx).Create(settings).Error
	}

	return fmt.Errorf("failed to upsert cancellation settings: %w", err)
}

// Delete soft-deletes cancellation settings
func (r *CancellationSettingsRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.CancellationSettings{}).
		Where("id = ?", id).
		Update("deleted_at", gorm.Expr("NOW()")).Error
}

// ListByVendor retrieves all cancellation settings for a vendor
func (r *CancellationSettingsRepository) ListByVendor(ctx context.Context, tenantID, vendorID string) ([]models.CancellationSettings, error) {
	var settings []models.CancellationSettings
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND vendor_id = ? AND deleted_at IS NULL", tenantID, vendorID).
		Find(&settings).Error

	if err != nil {
		return nil, fmt.Errorf("failed to list cancellation settings: %w", err)
	}

	return settings, nil
}

// ListByTenant retrieves all cancellation settings for a tenant
func (r *CancellationSettingsRepository) ListByTenant(ctx context.Context, tenantID string) ([]models.CancellationSettings, error) {
	var settings []models.CancellationSettings
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND deleted_at IS NULL", tenantID).
		Find(&settings).Error

	if err != nil {
		return nil, fmt.Errorf("failed to list cancellation settings: %w", err)
	}

	return settings, nil
}
