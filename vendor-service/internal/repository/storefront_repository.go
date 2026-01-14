package repository

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"vendor-service/internal/models"
	"gorm.io/gorm"
)

// StorefrontRepository defines the interface for storefront data operations
type StorefrontRepository interface {
	// Basic CRUD - All methods require vendorID for tenant isolation
	Create(vendorID uuid.UUID, storefront *models.Storefront) error
	GetByID(vendorID uuid.UUID, id uuid.UUID) (*models.Storefront, error)
	Update(vendorID uuid.UUID, id uuid.UUID, updates *models.UpdateStorefrontRequest) error
	Delete(vendorID uuid.UUID, id uuid.UUID, deletedBy string) error
	List(vendorID uuid.UUID, page, limit int) ([]models.Storefront, *models.PaginationInfo, error)

	// Resolution methods for tenant identification (public - no tenant context needed)
	GetBySlug(slug string) (*models.Storefront, error)
	GetByCustomDomain(domain string) (*models.Storefront, error)
	ResolveBySlug(slug string) (*models.StorefrontResolutionData, error)
	ResolveByCustomDomain(domain string) (*models.StorefrontResolutionData, error)

	// Vendor-specific operations
	GetByVendorID(vendorID uuid.UUID) ([]models.Storefront, error)
	CountByVendorID(vendorID uuid.UUID) (int64, error)

	// Validation helpers (global - slugs and domains are globally unique)
	SlugExists(slug string) (bool, error)
	DomainExists(domain string) (bool, error)
}

type storefrontRepository struct {
	db *gorm.DB
}

// NewStorefrontRepository creates a new storefront repository
func NewStorefrontRepository(db *gorm.DB) StorefrontRepository {
	return &storefrontRepository{db: db}
}

func (r *storefrontRepository) Create(vendorID uuid.UUID, storefront *models.Storefront) error {
	// Ensure storefront is assigned to the correct vendor (tenant isolation)
	storefront.VendorID = vendorID
	storefront.CreatedAt = time.Now()
	storefront.UpdatedAt = time.Now()
	storefront.Slug = strings.ToLower(storefront.Slug)

	return r.db.Create(storefront).Error
}

func (r *storefrontRepository) GetByID(vendorID uuid.UUID, id uuid.UUID) (*models.Storefront, error) {
	var storefront models.Storefront
	err := r.db.Where("vendor_id = ? AND id = ?", vendorID, id).
		Preload("Vendor").
		First(&storefront).Error

	if err != nil {
		return nil, err
	}

	return &storefront, nil
}

func (r *storefrontRepository) Update(vendorID uuid.UUID, id uuid.UUID, updates *models.UpdateStorefrontRequest) error {
	updateMap := make(map[string]interface{})

	if updates.Slug != nil {
		updateMap["slug"] = strings.ToLower(*updates.Slug)
	}
	if updates.Name != nil {
		updateMap["name"] = *updates.Name
	}
	if updates.CustomDomain != nil {
		updateMap["custom_domain"] = *updates.CustomDomain
	}
	if updates.IsActive != nil {
		updateMap["is_active"] = *updates.IsActive
	}
	if updates.IsDefault != nil {
		updateMap["is_default"] = *updates.IsDefault
	}
	if updates.ThemeConfig != nil {
		updateMap["theme_config"] = *updates.ThemeConfig
	}
	if updates.Settings != nil {
		updateMap["settings"] = *updates.Settings
	}
	if updates.LogoURL != nil {
		updateMap["logo_url"] = *updates.LogoURL
	}
	if updates.FaviconURL != nil {
		updateMap["favicon_url"] = *updates.FaviconURL
	}
	if updates.Description != nil {
		updateMap["description"] = *updates.Description
	}
	if updates.MetaTitle != nil {
		updateMap["meta_title"] = *updates.MetaTitle
	}
	if updates.MetaDesc != nil {
		updateMap["meta_desc"] = *updates.MetaDesc
	}

	updateMap["updated_at"] = time.Now()

	return r.db.Model(&models.Storefront{}).
		Where("vendor_id = ? AND id = ?", vendorID, id).
		Updates(updateMap).Error
}

func (r *storefrontRepository) Delete(vendorID uuid.UUID, id uuid.UUID, deletedBy string) error {
	return r.db.Model(&models.Storefront{}).
		Where("vendor_id = ? AND id = ?", vendorID, id).
		Updates(map[string]interface{}{
			"deleted_at": time.Now(),
			"updated_by": deletedBy,
		}).Error
}

func (r *storefrontRepository) List(vendorID uuid.UUID, page, limit int) ([]models.Storefront, *models.PaginationInfo, error) {
	var storefronts []models.Storefront
	var total int64

	// Always filter by vendorID for tenant isolation
	query := r.db.Model(&models.Storefront{}).Where("vendor_id = ?", vendorID)

	// Count total records
	if err := query.Count(&total).Error; err != nil {
		return nil, nil, err
	}

	// Apply pagination
	offset := (page - 1) * limit
	if err := query.Offset(offset).Limit(limit).
		Preload("Vendor").
		Order("created_at DESC").
		Find(&storefronts).Error; err != nil {
		return nil, nil, err
	}

	// Calculate pagination info
	totalPages := int((total + int64(limit) - 1) / int64(limit))
	pagination := &models.PaginationInfo{
		Page:        page,
		Limit:       limit,
		Total:       total,
		TotalPages:  totalPages,
		HasNext:     page < totalPages,
		HasPrevious: page > 1,
	}

	return storefronts, pagination, nil
}

func (r *storefrontRepository) GetBySlug(slug string) (*models.Storefront, error) {
	var storefront models.Storefront
	err := r.db.Where("slug = ? AND is_active = true", strings.ToLower(slug)).
		Preload("Vendor").
		First(&storefront).Error

	if err != nil {
		return nil, err
	}

	return &storefront, nil
}

func (r *storefrontRepository) GetByCustomDomain(domain string) (*models.Storefront, error) {
	var storefront models.Storefront
	err := r.db.Where("custom_domain = ? AND is_active = true", strings.ToLower(domain)).
		Preload("Vendor").
		First(&storefront).Error

	if err != nil {
		return nil, err
	}

	return &storefront, nil
}

// ResolveBySlug returns tenant resolution data for middleware use
func (r *storefrontRepository) ResolveBySlug(slug string) (*models.StorefrontResolutionData, error) {
	var storefront models.Storefront
	err := r.db.Where("slug = ? AND is_active = true", strings.ToLower(slug)).
		Preload("Vendor").
		First(&storefront).Error

	if err != nil {
		return nil, err
	}

	// Verify vendor is active
	if !storefront.Vendor.IsActive {
		return nil, gorm.ErrRecordNotFound
	}

	return &models.StorefrontResolutionData{
		StorefrontID:   storefront.ID,
		VendorID:       storefront.VendorID,
		Slug:           storefront.Slug,
		Name:           storefront.Name,
		CustomDomain:   storefront.CustomDomain,
		ThemeConfig:    storefront.ThemeConfig,
		Settings:       storefront.Settings,
		LogoURL:        storefront.LogoURL,
		FaviconURL:     storefront.FaviconURL,
		VendorName:     storefront.Vendor.Name,
		VendorIsActive: storefront.Vendor.IsActive,
	}, nil
}

// ResolveByCustomDomain returns tenant resolution data for middleware use
func (r *storefrontRepository) ResolveByCustomDomain(domain string) (*models.StorefrontResolutionData, error) {
	var storefront models.Storefront
	err := r.db.Where("custom_domain = ? AND is_active = true", strings.ToLower(domain)).
		Preload("Vendor").
		First(&storefront).Error

	if err != nil {
		return nil, err
	}

	// Verify vendor is active
	if !storefront.Vendor.IsActive {
		return nil, gorm.ErrRecordNotFound
	}

	return &models.StorefrontResolutionData{
		StorefrontID:   storefront.ID,
		VendorID:       storefront.VendorID,
		Slug:           storefront.Slug,
		Name:           storefront.Name,
		CustomDomain:   storefront.CustomDomain,
		ThemeConfig:    storefront.ThemeConfig,
		Settings:       storefront.Settings,
		LogoURL:        storefront.LogoURL,
		FaviconURL:     storefront.FaviconURL,
		VendorName:     storefront.Vendor.Name,
		VendorIsActive: storefront.Vendor.IsActive,
	}, nil
}

func (r *storefrontRepository) GetByVendorID(vendorID uuid.UUID) ([]models.Storefront, error) {
	var storefronts []models.Storefront
	err := r.db.Where("vendor_id = ?", vendorID).
		Order("created_at DESC").
		Find(&storefronts).Error

	return storefronts, err
}

func (r *storefrontRepository) CountByVendorID(vendorID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&models.Storefront{}).
		Where("vendor_id = ?", vendorID).
		Count(&count).Error

	return count, err
}

func (r *storefrontRepository) SlugExists(slug string) (bool, error) {
	var count int64
	err := r.db.Model(&models.Storefront{}).
		Where("slug = ?", strings.ToLower(slug)).
		Count(&count).Error

	return count > 0, err
}

func (r *storefrontRepository) DomainExists(domain string) (bool, error) {
	var count int64
	err := r.db.Model(&models.Storefront{}).
		Where("custom_domain = ?", strings.ToLower(domain)).
		Count(&count).Error

	return count > 0, err
}
