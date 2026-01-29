package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/Tesseract-Nexus/go-shared/cache"
	"vendor-service/internal/models"
	"gorm.io/gorm"
)

// Cache TTL constants for storefronts
const (
	StorefrontCacheTTL     = 15 * time.Minute // Individual storefront
	StorefrontSlugCacheTTL = 30 * time.Minute // Slug lookups (used frequently for routing)
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

	// Public resolution methods - returns storefront data including isActive status
	// These don't filter by is_active, allowing "Coming Soon" pages for unpublished stores
	ResolveBySlugForPublic(slug string) (*models.StorefrontResolutionData, error)
	ResolveByCustomDomainForPublic(domain string) (*models.StorefrontResolutionData, error)

	// Vendor-specific operations
	GetByVendorID(vendorID uuid.UUID) ([]models.Storefront, error)
	CountByVendorID(vendorID uuid.UUID) (int64, error)

	// Validation helpers (global - slugs and domains are globally unique)
	SlugExists(slug string) (bool, error)
	DomainExists(domain string) (bool, error)
}

type storefrontRepository struct {
	db    *gorm.DB
	redis *redis.Client
	cache *cache.CacheLayer
}

// NewStorefrontRepository creates a new storefront repository
func NewStorefrontRepository(db *gorm.DB, redisClient *redis.Client) StorefrontRepository {
	repo := &storefrontRepository{
		db:    db,
		redis: redisClient,
	}

	// Initialize CacheLayer with the existing Redis client
	if redisClient != nil {
		cacheConfig := cache.CacheConfig{
			L1Enabled:  true,
			L1MaxItems: 1000,
			L1TTL:      30 * time.Second,
			DefaultTTL: StorefrontCacheTTL,
			KeyPrefix:  "tesseract:storefronts:",
		}
		repo.cache = cache.NewCacheLayerFromClient(redisClient, cacheConfig)
	}

	return repo
}

// generateStorefrontCacheKey creates a cache key for storefront lookups
func generateStorefrontCacheKey(vendorID uuid.UUID, storefrontID uuid.UUID) string {
	return fmt.Sprintf("storefront:%s:%s", vendorID.String(), storefrontID.String())
}

// generateStorefrontSlugCacheKey creates a cache key for slug lookups
func generateStorefrontSlugCacheKey(slug string) string {
	return fmt.Sprintf("storefront:slug:%s", strings.ToLower(slug))
}

// invalidateStorefrontCaches invalidates all caches related to a storefront
func (r *storefrontRepository) invalidateStorefrontCaches(ctx context.Context, vendorID uuid.UUID, storefrontID uuid.UUID, slug string) {
	if r.cache == nil {
		return
	}
	_ = r.cache.Delete(ctx, generateStorefrontCacheKey(vendorID, storefrontID))
	if slug != "" {
		_ = r.cache.Delete(ctx, generateStorefrontSlugCacheKey(slug))
	}
	// Invalidate list caches for this vendor
	_ = r.cache.DeletePattern(ctx, fmt.Sprintf("storefront:list:%s:*", vendorID.String()))
}

func (r *storefrontRepository) Create(vendorID uuid.UUID, storefront *models.Storefront) error {
	// Ensure storefront is assigned to the correct vendor (tenant isolation)
	storefront.VendorID = vendorID
	storefront.CreatedAt = time.Now()
	storefront.UpdatedAt = time.Now()
	storefront.Slug = strings.ToLower(storefront.Slug)

	err := r.db.Create(storefront).Error
	if err == nil {
		r.invalidateStorefrontCaches(context.Background(), vendorID, storefront.ID, storefront.Slug)
	}
	return err
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

	// Get existing storefront to get slug for cache invalidation
	existing, _ := r.GetByID(vendorID, id)
	slug := ""
	if existing != nil {
		slug = existing.Slug
	}

	err := r.db.Model(&models.Storefront{}).
		Where("vendor_id = ? AND id = ?", vendorID, id).
		Updates(updateMap).Error
	if err == nil {
		r.invalidateStorefrontCaches(context.Background(), vendorID, id, slug)
	}
	return err
}

func (r *storefrontRepository) Delete(vendorID uuid.UUID, id uuid.UUID, deletedBy string) error {
	// Get existing storefront to get slug for cache invalidation
	existing, _ := r.GetByID(vendorID, id)
	slug := ""
	if existing != nil {
		slug = existing.Slug
	}

	err := r.db.Model(&models.Storefront{}).
		Where("vendor_id = ? AND id = ?", vendorID, id).
		Updates(map[string]interface{}{
			"deleted_at": time.Now(),
			"updated_by": deletedBy,
		}).Error
	if err == nil {
		r.invalidateStorefrontCaches(context.Background(), vendorID, id, slug)
	}
	return err
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
	ctx := context.Background()
	cacheKey := generateStorefrontSlugCacheKey(slug)

	// Try to get from cache first (slug lookups are very frequent for routing)
	if r.redis != nil {
		val, err := r.redis.Get(ctx, "tesseract:storefronts:"+cacheKey).Result()
		if err == nil {
			var storefront models.Storefront
			if err := json.Unmarshal([]byte(val), &storefront); err == nil {
				return &storefront, nil
			}
		}
	}

	// Query from database
	var storefront models.Storefront
	err := r.db.Where("slug = ? AND is_active = true", strings.ToLower(slug)).
		Preload("Vendor").
		First(&storefront).Error

	if err != nil {
		return nil, err
	}

	// Cache the result
	if r.redis != nil {
		data, marshalErr := json.Marshal(storefront)
		if marshalErr == nil {
			r.redis.Set(ctx, "tesseract:storefronts:"+cacheKey, data, StorefrontSlugCacheTTL)
		}
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
		TenantID:       storefront.Vendor.TenantID,
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
		IsActive:       storefront.IsActive,
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
		TenantID:       storefront.Vendor.TenantID,
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
		IsActive:       storefront.IsActive,
	}, nil
}

// ResolveBySlugForPublic returns tenant resolution data including isActive status
// This method does NOT filter by is_active, allowing storefronts to show "Coming Soon" pages
// for unpublished stores. It still verifies the vendor is active.
func (r *storefrontRepository) ResolveBySlugForPublic(slug string) (*models.StorefrontResolutionData, error) {
	var storefront models.Storefront
	err := r.db.Where("slug = ?", strings.ToLower(slug)).
		Preload("Vendor").
		First(&storefront).Error

	if err != nil {
		return nil, err
	}

	// Verify vendor is active - we don't want to expose storefronts for deactivated vendors
	if !storefront.Vendor.IsActive {
		return nil, gorm.ErrRecordNotFound
	}

	return &models.StorefrontResolutionData{
		StorefrontID:   storefront.ID,
		TenantID:       storefront.Vendor.TenantID,
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
		IsActive:       storefront.IsActive,
	}, nil
}

// ResolveByCustomDomainForPublic returns tenant resolution data including isActive status
// This method does NOT filter by is_active, allowing storefronts to show "Coming Soon" pages
// for unpublished stores. It still verifies the vendor is active.
func (r *storefrontRepository) ResolveByCustomDomainForPublic(domain string) (*models.StorefrontResolutionData, error) {
	var storefront models.Storefront
	err := r.db.Where("custom_domain = ?", strings.ToLower(domain)).
		Preload("Vendor").
		First(&storefront).Error

	if err != nil {
		return nil, err
	}

	// Verify vendor is active - we don't want to expose storefronts for deactivated vendors
	if !storefront.Vendor.IsActive {
		return nil, gorm.ErrRecordNotFound
	}

	return &models.StorefrontResolutionData{
		StorefrontID:   storefront.ID,
		TenantID:       storefront.Vendor.TenantID,
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
		IsActive:       storefront.IsActive,
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
