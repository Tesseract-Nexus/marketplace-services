package repository

import (
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"orders-service/internal/models"
)

// ReceiptSettingsRepository handles receipt settings data persistence
type ReceiptSettingsRepository struct {
	db *gorm.DB
}

// NewReceiptSettingsRepository creates a new receipt settings repository
func NewReceiptSettingsRepository(db *gorm.DB) *ReceiptSettingsRepository {
	return &ReceiptSettingsRepository{db: db}
}

// GetByTenantID retrieves receipt settings for a tenant
func (r *ReceiptSettingsRepository) GetByTenantID(tenantID string) (*models.ReceiptSettings, error) {
	var settings models.ReceiptSettings
	err := r.db.Where("tenant_id = ?", tenantID).First(&settings).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Not found, return nil without error
		}
		return nil, fmt.Errorf("failed to get receipt settings: %w", err)
	}
	return &settings, nil
}

// Create creates new receipt settings for a tenant
func (r *ReceiptSettingsRepository) Create(settings *models.ReceiptSettings) error {
	if settings.ID == uuid.Nil {
		settings.ID = uuid.New()
	}
	if err := r.db.Create(settings).Error; err != nil {
		return fmt.Errorf("failed to create receipt settings: %w", err)
	}
	return nil
}

// Update updates existing receipt settings
func (r *ReceiptSettingsRepository) Update(settings *models.ReceiptSettings) error {
	if err := r.db.Save(settings).Error; err != nil {
		return fmt.Errorf("failed to update receipt settings: %w", err)
	}
	return nil
}

// Upsert creates or updates receipt settings for a tenant using ON CONFLICT
func (r *ReceiptSettingsRepository) Upsert(settings *models.ReceiptSettings) error {
	if settings.ID == uuid.Nil {
		settings.ID = uuid.New()
	}
	// Use GORM's Clauses for atomic upsert to avoid TOCTOU race condition
	err := r.db.Where("tenant_id = ?", settings.TenantID).
		Assign(settings).
		FirstOrCreate(settings).Error
	if err != nil {
		return fmt.Errorf("failed to upsert receipt settings: %w", err)
	}
	return nil
}

// Delete soft-deletes receipt settings for a tenant
func (r *ReceiptSettingsRepository) Delete(tenantID string) error {
	if err := r.db.Where("tenant_id = ?", tenantID).Delete(&models.ReceiptSettings{}).Error; err != nil {
		return fmt.Errorf("failed to delete receipt settings: %w", err)
	}
	return nil
}
