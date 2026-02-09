package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProductStatus represents the status of a product
type ProductStatus string

const (
	ProductStatusDraft    ProductStatus = "DRAFT"
	ProductStatusPending  ProductStatus = "PENDING"
	ProductStatusActive   ProductStatus = "ACTIVE"
	ProductStatusInactive ProductStatus = "INACTIVE"
	ProductStatusArchived ProductStatus = "ARCHIVED"
	ProductStatusRejected ProductStatus = "REJECTED"
)

// InventoryStatus represents the inventory status of a product
type InventoryStatus string

const (
	InventoryStatusInStock      InventoryStatus = "IN_STOCK"
	InventoryStatusLowStock     InventoryStatus = "LOW_STOCK"
	InventoryStatusOutOfStock   InventoryStatus = "OUT_OF_STOCK"
	InventoryStatusBackOrder    InventoryStatus = "BACK_ORDER"
	InventoryStatusDiscontinued InventoryStatus = "DISCONTINUED"
)

// SyncStatus represents the synchronization status
type SyncStatus string

const (
	SyncStatusSynced   SyncStatus = "SYNCED"
	SyncStatusPending  SyncStatus = "PENDING"
	SyncStatusFailed   SyncStatus = "FAILED"
	SyncStatusConflict SyncStatus = "CONFLICT"
)

// JSON type for PostgreSQL JSONB (object/map)
type JSON map[string]interface{}

func (j JSON) Value() (driver.Value, error) {
	return json.Marshal(j)
}

func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSON)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// JSONArray type for PostgreSQL JSONB (array)
type JSONArray []interface{}

func (j JSONArray) Value() (driver.Value, error) {
	return json.Marshal(j)
}

func (j *JSONArray) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSONArray, 0)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// ProductAttribute represents a product attribute
type ProductAttribute struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

// ProductVariantAttribute represents a variant attribute
type ProductVariantAttribute struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

// ProductImage represents a product image
type ProductImage struct {
	ID       string  `json:"id"`
	URL      string  `json:"url"`
	AltText  *string `json:"altText,omitempty"`
	Position int     `json:"position"`
	Width    *int    `json:"width,omitempty"`
	Height   *int    `json:"height,omitempty"`
}

// ProductVideo represents a product promotional video
type ProductVideo struct {
	ID          string  `json:"id"`
	URL         string  `json:"url"`
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	ThumbnailURL *string `json:"thumbnailUrl,omitempty"`
	Duration    *int    `json:"duration,omitempty"` // Duration in seconds
	Size        *int64  `json:"size,omitempty"`     // Size in bytes
	Position    int     `json:"position"`
}

// MediaType constants for different media types
const (
	MediaTypeImage   = "image"
	MediaTypeVideo   = "video"
	MediaTypeLogo    = "logo"
	MediaTypeBanner  = "banner"
	MediaTypeIcon    = "icon"
	MediaTypeGallery = "gallery"
)

// MediaLimits defines upload limits for different media types
var MediaLimits = struct {
	MaxGalleryImages   int   // Max gallery images per product
	MaxCategoryImages  int   // Max images per category
	MaxVideos          int   // Max videos per product
	MaxImageSizeBytes  int64 // Max size for images (10MB)
	MaxLogoSizeBytes   int64 // Max size for logos (2MB)
	MaxBannerSizeBytes int64 // Max size for banners (5MB)
	MaxVideoSizeBytes  int64 // Max size for videos (100MB)
	LogoMaxWidth       int   // Max logo width
	LogoMaxHeight      int   // Max logo height
	BannerMaxWidth     int   // Max banner width
	BannerMaxHeight    int   // Max banner height
}{
	MaxGalleryImages:   12,                 // Updated from 7 to 12 for richer product galleries
	MaxCategoryImages:  3,                  // Max 3 images per category
	MaxVideos:          2,
	MaxImageSizeBytes:  10 * 1024 * 1024,   // 10MB
	MaxLogoSizeBytes:   2 * 1024 * 1024,    // 2MB
	MaxBannerSizeBytes: 5 * 1024 * 1024,    // 5MB
	MaxVideoSizeBytes:  100 * 1024 * 1024,  // 100MB
	LogoMaxWidth:       512,
	LogoMaxHeight:      512,
	BannerMaxWidth:     1920,
	BannerMaxHeight:    480,
}

// DefaultMediaURLs provides fallback images from Unsplash
var DefaultMediaURLs = struct {
	ProductImage    string
	ProductLogo     string
	ProductBanner   string
	CategoryIcon    string
	CategoryBanner  string
	WarehouseLogo   string
}{
	ProductImage:    "https://images.unsplash.com/photo-1505740420928-5e560c06d30e?w=800&q=80",
	ProductLogo:     "https://images.unsplash.com/photo-1560472355-536de3962603?w=200&q=80",
	ProductBanner:   "https://images.unsplash.com/photo-1441986300917-64674bd600d8?w=1920&q=80",
	CategoryIcon:    "https://images.unsplash.com/photo-1472851294608-062f824d29cc?w=200&q=80",
	CategoryBanner:  "https://images.unsplash.com/photo-1441984904996-e0b6ba687e04?w=1920&q=80",
	WarehouseLogo:   "https://images.unsplash.com/photo-1586528116311-ad8dd3c8310d?w=200&q=80",
}

// Dimensions represents product dimensions
type Dimensions struct {
	Length string `json:"length"`
	Width  string `json:"width"`
	Height string `json:"height"`
	Unit   string `json:"unit"`
}

// Product represents a product entity
// Performance indexes: Composite indexes on tenant_id with frequently filtered columns
// for 20-50x query improvement on multi-tenant queries
type Product struct {
	ID                uuid.UUID         `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID          string            `json:"tenantId" gorm:"not null;index:idx_products_tenant_id;index:idx_products_tenant_status;index:idx_products_tenant_category;index:idx_products_tenant_vendor;index:idx_products_tenant_inventory;index:idx_products_tenant_sku,unique;index:idx_products_tenant_slug,unique"`
	VendorID          string            `json:"vendorId" gorm:"not null;index;index:idx_products_tenant_vendor"`
	CategoryID        string            `json:"categoryId" gorm:"not null;index;index:idx_products_tenant_category"`
	WarehouseID       *string           `json:"warehouseId,omitempty" gorm:"index"`
	SupplierID        *string           `json:"supplierId,omitempty" gorm:"index"`
	CreatedByID       *string           `json:"createdById,omitempty"`
	Name              string            `json:"name" gorm:"not null"`
	Slug              *string           `json:"slug,omitempty" gorm:"index:idx_products_tenant_slug,unique"`
	SKU               string            `json:"sku" gorm:"not null;index:idx_products_tenant_sku,unique"`
	Brand             *string           `json:"brand,omitempty" gorm:"index"`
	Description       *string           `json:"description,omitempty"`
	Price             string            `json:"price" gorm:"not null"`
	ComparePrice      *string           `json:"comparePrice,omitempty"`
	CostPrice         *string           `json:"costPrice,omitempty"`
	Status            ProductStatus     `json:"status" gorm:"not null;default:'DRAFT';index:idx_products_tenant_status"`
	InventoryStatus   *InventoryStatus  `json:"inventoryStatus,omitempty" gorm:"index:idx_products_tenant_inventory"`
	Quantity          *int              `json:"quantity,omitempty"`
	MinOrderQty       *int              `json:"minOrderQty,omitempty"`
	MaxOrderQty       *int              `json:"maxOrderQty,omitempty"`
	LowStockThreshold *int              `json:"lowStockThreshold,omitempty"`
	Weight            *string           `json:"weight,omitempty"`
	Dimensions        *JSON             `json:"dimensions,omitempty" gorm:"type:jsonb"`
	SearchKeywords    *string           `json:"searchKeywords,omitempty"`
	SearchVector      *string           `json:"-" gorm:"type:tsvector;index:,type:gin"`
	AverageRating     *float64          `json:"averageRating,omitempty"`
	ReviewCount       *int              `json:"reviewCount,omitempty"`
	Tags              *JSON             `json:"tags,omitempty" gorm:"type:jsonb"`
	CurrencyCode      *string           `json:"currencyCode,omitempty"`
	SyncStatus        *SyncStatus       `json:"syncStatus,omitempty"`
	SyncedAt          *time.Time        `json:"syncedAt,omitempty"`
	Version           *int              `json:"version,omitempty" gorm:"default:1"`
	OfflineID         *string           `json:"offlineId,omitempty"`
	Localizations     *JSON             `json:"localizations,omitempty" gorm:"type:jsonb"`
	Attributes        *JSON             `json:"attributes,omitempty" gorm:"type:jsonb"`
	Images            *JSONArray        `json:"images,omitempty" gorm:"type:jsonb"`
	// Media fields for storefront display
	LogoURL           *string           `json:"logoUrl,omitempty" gorm:"column:logo_url"`           // Small product logo/icon (512x512 max)
	BannerURL         *string           `json:"bannerUrl,omitempty" gorm:"column:banner_url"`       // Product banner (1920x480)
	Videos            *JSONArray        `json:"videos,omitempty" gorm:"type:jsonb"`                 // Promotional videos (max 2)
	Variants          []*ProductVariant `json:"variants,omitempty" gorm:"foreignKey:ProductID;constraint:OnDelete:CASCADE"`
	CreatedAt         time.Time         `json:"createdAt"`
	UpdatedAt         time.Time         `json:"updatedAt"`
	DeletedAt         *gorm.DeletedAt   `json:"deletedAt,omitempty" gorm:"index"`
	CreatedBy         *string           `json:"createdBy,omitempty"`
	UpdatedBy         *string           `json:"updatedBy,omitempty"`
	Metadata          *JSON             `json:"metadata,omitempty" gorm:"type:jsonb"`
	// Denormalized names for display (updated on create/update)
	WarehouseName *string `json:"warehouseName,omitempty" gorm:"index"`
	SupplierName  *string `json:"supplierName,omitempty" gorm:"index"`
	// SEO metadata
	SeoTitle       *string    `json:"seoTitle,omitempty" gorm:"column:seo_title;type:text"`
	SeoDescription *string    `json:"seoDescription,omitempty" gorm:"column:seo_description;type:text"`
	SeoKeywords    *JSONArray `json:"seoKeywords,omitempty" gorm:"column:seo_keywords;type:jsonb"`
	OgImage        *string    `json:"ogImage,omitempty" gorm:"column:og_image;type:text"`
}

// ProductVariant represents a product variant
type ProductVariant struct {
	ID                uuid.UUID        `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ProductID         uuid.UUID        `json:"productId" gorm:"type:uuid;not null;index"`
	SKU               string           `json:"sku" gorm:"not null;unique"`
	Name              string           `json:"name" gorm:"not null"`
	Price             string           `json:"price" gorm:"not null"`
	ComparePrice      *string          `json:"comparePrice,omitempty"`
	CostPrice         *string          `json:"costPrice,omitempty"`
	Quantity          int              `json:"quantity" gorm:"not null;default:0"`
	LowStockThreshold *int             `json:"lowStockThreshold,omitempty"`
	Weight            *string          `json:"weight,omitempty"`
	Dimensions        *JSON            `json:"dimensions,omitempty" gorm:"type:jsonb"`
	InventoryStatus   *InventoryStatus `json:"inventoryStatus,omitempty"`
	SyncStatus        *SyncStatus      `json:"syncStatus,omitempty"`
	Version           *int             `json:"version,omitempty" gorm:"default:1"`
	OfflineID         *string          `json:"offlineId,omitempty"`
	Images            *JSON            `json:"images,omitempty" gorm:"type:jsonb"`
	Attributes        *JSON            `json:"attributes,omitempty" gorm:"type:jsonb"`
	CreatedAt         time.Time        `json:"createdAt"`
	UpdatedAt         time.Time        `json:"updatedAt"`
	DeletedAt         *gorm.DeletedAt  `json:"deletedAt,omitempty" gorm:"index"`
}

// Category represents a product category
type Category struct {
	ID          uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID    string          `json:"tenantId" gorm:"column:tenant_id;not null;index"`
	CreatedByID string          `json:"createdById" gorm:"column:created_by_id;not null"`
	UpdatedByID string          `json:"updatedById" gorm:"column:updated_by_id;not null"`
	Name        string          `json:"name" gorm:"not null"`
	Slug        string          `json:"slug" gorm:"not null"`
	Description *string         `json:"description,omitempty"`
	ImageURL    *string         `json:"imageUrl,omitempty" gorm:"column:image_url"`      // Category icon/thumbnail
	BannerURL   *string         `json:"bannerUrl,omitempty" gorm:"column:banner_url"`    // Category banner for storefront
	ParentID    *uuid.UUID      `json:"parentId,omitempty" gorm:"column:parent_id"`
	Level       int             `json:"level" gorm:"not null;default:0"`
	Position    int             `json:"position" gorm:"not null;default:1"`
	IsActive    *bool           `json:"isActive" gorm:"column:is_active;default:true"`
	Status      string          `json:"status" gorm:"not null;default:'ACTIVE'"`
	// SEO metadata
	SeoTitle       *string    `json:"seoTitle,omitempty" gorm:"column:seo_title;type:text"`
	SeoDescription *string    `json:"seoDescription,omitempty" gorm:"column:seo_description;type:text"`
	SeoKeywords    *JSONArray `json:"seoKeywords,omitempty" gorm:"column:seo_keywords;type:jsonb"`
	CreatedAt   time.Time       `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt   time.Time       `json:"updatedAt" gorm:"column:updated_at"`
	DeletedAt   *gorm.DeletedAt `json:"deletedAt,omitempty" gorm:"column:deleted_at;index"`
}

// Vendor represents a vendor entity (read-only for import lookup)
type Vendor struct {
	ID       uuid.UUID `json:"id" gorm:"type:uuid;primary_key"`
	TenantID string    `json:"tenantId" gorm:"column:tenant_id;not null;index"`
	Name     string    `json:"name" gorm:"not null"`
}

// TableName specifies the table name for Vendor
func (Vendor) TableName() string {
	return "vendors"
}

// CreateProductRequest represents a request to create a new product
type CreateProductRequest struct {
	Name              string             `json:"name" binding:"required"`
	Slug              *string            `json:"slug,omitempty"`
	SKU               string             `json:"sku" binding:"required"`
	Description       *string            `json:"description,omitempty"`
	Price             string             `json:"price" binding:"required"`
	ComparePrice      *string            `json:"comparePrice,omitempty"`
	CostPrice         *string            `json:"costPrice,omitempty"`
	VendorID          string             `json:"vendorId" binding:"required"`
	CategoryID        string             `json:"categoryId" binding:"required"`
	// Optional warehouse - use ID or Name (Name will auto-create if not exists)
	WarehouseID   *string `json:"warehouseId,omitempty"`
	WarehouseName *string `json:"warehouseName,omitempty"`
	// Optional supplier - use ID or Name (Name will auto-create if not exists)
	SupplierID   *string `json:"supplierId,omitempty"`
	SupplierName *string `json:"supplierName,omitempty"`
	Quantity          *int               `json:"quantity,omitempty"`
	MinOrderQty       *int               `json:"minOrderQty,omitempty"`
	MaxOrderQty       *int               `json:"maxOrderQty,omitempty"`
	LowStockThreshold *int               `json:"lowStockThreshold,omitempty"`
	Weight            *string            `json:"weight,omitempty"`
	Dimensions        *JSON              `json:"dimensions,omitempty"`
	SearchKeywords    *string            `json:"searchKeywords,omitempty"`
	Tags              []string           `json:"tags,omitempty"`
	CurrencyCode      *string            `json:"currencyCode,omitempty"`
	Attributes        []ProductAttribute `json:"attributes,omitempty"`
	Images            []ProductImage     `json:"images,omitempty"` // Gallery images (max 7)
	LogoURL           *string            `json:"logoUrl,omitempty"`   // Product logo (512x512 max)
	BannerURL         *string            `json:"bannerUrl,omitempty"` // Product banner (1920x480)
	Videos            []ProductVideo     `json:"videos,omitempty"`    // Promotional videos (max 2)
	// SEO metadata
	SeoTitle       *string  `json:"seoTitle,omitempty"`
	SeoDescription *string  `json:"seoDescription,omitempty"`
	SeoKeywords    []string `json:"seoKeywords,omitempty"`
	OgImage        *string  `json:"ogImage,omitempty"`
}

// UpdateProductRequest represents a request to update a product
type UpdateProductRequest struct {
	Name              *string            `json:"name,omitempty"`
	Slug              *string            `json:"slug,omitempty"`
	SKU               *string            `json:"sku,omitempty"`
	Brand             *string            `json:"brand,omitempty"`
	Description       *string            `json:"description,omitempty"`
	Price             *string            `json:"price,omitempty"`
	ComparePrice      *string            `json:"comparePrice,omitempty"`
	CostPrice         *string            `json:"costPrice,omitempty"`
	VendorID          *string            `json:"vendorId,omitempty"`
	CategoryID        *string            `json:"categoryId,omitempty"`
	// Optional warehouse - use ID or Name (Name will auto-create if not exists)
	WarehouseID   *string `json:"warehouseId,omitempty"`
	WarehouseName *string `json:"warehouseName,omitempty"`
	// Optional supplier - use ID or Name (Name will auto-create if not exists)
	SupplierID   *string `json:"supplierId,omitempty"`
	SupplierName *string `json:"supplierName,omitempty"`
	Quantity          *int               `json:"quantity,omitempty"`
	MinOrderQty       *int               `json:"minOrderQty,omitempty"`
	MaxOrderQty       *int               `json:"maxOrderQty,omitempty"`
	LowStockThreshold *int               `json:"lowStockThreshold,omitempty"`
	Weight            *string            `json:"weight,omitempty"`
	Dimensions        *JSON              `json:"dimensions,omitempty"`
	SearchKeywords    *string            `json:"searchKeywords,omitempty"`
	CurrencyCode      *string            `json:"currencyCode,omitempty"`
	Tags              []string           `json:"tags,omitempty"`
	Attributes        []ProductAttribute `json:"attributes,omitempty"`
	Images            []ProductImage     `json:"images,omitempty"` // Gallery images (max 7)
	LogoURL           *string            `json:"logoUrl,omitempty"`   // Product logo (512x512 max)
	BannerURL         *string            `json:"bannerUrl,omitempty"` // Product banner (1920x480)
	Videos            []ProductVideo     `json:"videos,omitempty"`    // Promotional videos (max 2)
	// SEO metadata
	SeoTitle       *string  `json:"seoTitle,omitempty"`
	SeoDescription *string  `json:"seoDescription,omitempty"`
	SeoKeywords    []string `json:"seoKeywords,omitempty"`
	OgImage        *string  `json:"ogImage,omitempty"`
}

// UpdateProductStatusRequest represents a request to update product status
type UpdateProductStatusRequest struct {
	Status ProductStatus `json:"status" binding:"required"`
	Notes  *string       `json:"notes,omitempty"`
}

// CreateProductVariantRequest represents a request to create a product variant
type CreateProductVariantRequest struct {
	SKU               string                    `json:"sku" binding:"required"`
	Name              string                    `json:"name" binding:"required"`
	Price             string                    `json:"price" binding:"required"`
	ComparePrice      *string                   `json:"comparePrice,omitempty"`
	CostPrice         *string                   `json:"costPrice,omitempty"`
	Quantity          *int                      `json:"quantity,omitempty"`
	LowStockThreshold *int                      `json:"lowStockThreshold,omitempty"`
	Weight            *string                   `json:"weight,omitempty"`
	Dimensions        *JSON                     `json:"dimensions,omitempty"`
	Attributes        []ProductVariantAttribute `json:"attributes,omitempty"`
}

// UpdateProductVariantRequest represents a request to update a product variant
type UpdateProductVariantRequest struct {
	Name              *string                   `json:"name,omitempty"`
	Price             *string                   `json:"price,omitempty"`
	ComparePrice      *string                   `json:"comparePrice,omitempty"`
	CostPrice         *string                   `json:"costPrice,omitempty"`
	LowStockThreshold *int                      `json:"lowStockThreshold,omitempty"`
	Weight            *string                   `json:"weight,omitempty"`
	Dimensions        *JSON                     `json:"dimensions,omitempty"`
	Attributes        []ProductVariantAttribute `json:"attributes,omitempty"`
}

// UpdateInventoryRequest represents a request to update inventory
type UpdateInventoryRequest struct {
	Quantity        int              `json:"quantity" binding:"required"`
	InventoryStatus *InventoryStatus `json:"inventoryStatus,omitempty"`
	Reason          *string          `json:"reason,omitempty"`
}

// InventoryAdjustmentRequest represents an inventory adjustment
type InventoryAdjustmentRequest struct {
	Adjustment int     `json:"adjustment" binding:"required"`
	Reason     string  `json:"reason" binding:"required"`
	Notes      *string `json:"notes,omitempty"`
}

// BulkInventoryItem represents a single product in bulk inventory operations
type BulkInventoryItem struct {
	ProductID string `json:"productId" binding:"required"`
	Quantity  int    `json:"quantity" binding:"required,min=1"`
}

// BulkInventoryRequest for bulk inventory operations (deduct/restore)
type BulkInventoryRequest struct {
	Items  []BulkInventoryItem `json:"items" binding:"required,dive"`
	Reason string              `json:"reason" binding:"required"`
}

// StockCheckItem represents a single product stock check request
type StockCheckItem struct {
	ProductID string `json:"productId" binding:"required"`
	Quantity  int    `json:"quantity" binding:"required,min=1"`
}

// StockCheckRequest for checking stock availability
type StockCheckRequest struct {
	Items []StockCheckItem `json:"items" binding:"required,dive"`
}

// StockCheckResult represents stock availability for a single product
type StockCheckResult struct {
	ProductID   string `json:"productId"`
	Available   bool   `json:"available"`
	InStock     int    `json:"inStock"`
	Requested   int    `json:"requested"`
	ProductName string `json:"productName,omitempty"`
}

// StockCheckResponse for stock check results
type StockCheckResponse struct {
	Success    bool               `json:"success"`
	AllInStock bool               `json:"allInStock"`
	Results    []StockCheckResult `json:"results"`
	Message    *string            `json:"message,omitempty"`
}

// AddImageRequest represents a request to add an image
type AddImageRequest struct {
	URL      string  `json:"url" binding:"required"`
	AltText  *string `json:"altText,omitempty"`
	Position *int    `json:"position,omitempty"`
	Width    *int    `json:"width,omitempty"`
	Height   *int    `json:"height,omitempty"`
}

// UpdateImageRequest represents a request to update an image
type UpdateImageRequest struct {
	AltText  *string `json:"altText,omitempty"`
	Position *int    `json:"position,omitempty"`
}

// SearchProductsRequest represents a search request
type SearchProductsRequest struct {
	Query           *string            `json:"query,omitempty"`
	CategoryID      *string            `json:"categoryId,omitempty"`
	VendorID        *string            `json:"vendorId,omitempty"`
	Brands          []string           `json:"brands,omitempty"`
	Status          []ProductStatus    `json:"status,omitempty"`
	InventoryStatus []InventoryStatus  `json:"inventoryStatus,omitempty"`
	MinPrice        *string            `json:"minPrice,omitempty"`
	MaxPrice        *string            `json:"maxPrice,omitempty"`
	MinRating       *float64           `json:"minRating,omitempty"`
	Tags            []string           `json:"tags,omitempty"`
	Attributes      map[string][]string `json:"attributes,omitempty"` // e.g., {"color": ["red", "blue"], "size": ["M", "L"]}
	DateFrom        *time.Time         `json:"dateFrom,omitempty"`
	DateTo          *time.Time         `json:"dateTo,omitempty"`
	UpdatedAfter    *time.Time         `json:"updatedAfter,omitempty"`
	IncludeVariants *bool              `json:"includeVariants,omitempty"`
	SortBy          *string            `json:"sortBy,omitempty"`
	SortOrder       *string            `json:"sortOrder,omitempty"`
	Page            int                `json:"page"`
	Limit           int                `json:"limit"`
}

// BulkUpdateRequest represents a bulk update request
type BulkUpdateRequest struct {
	ProductIDs []string      `json:"productIds" binding:"required"`
	Status     ProductStatus `json:"status" binding:"required"`
	Notes      *string       `json:"notes,omitempty"`
}

// ============================================================================
// Bulk Create Models - Consistent pattern for all services
// ============================================================================

// BulkCreateProductItem represents a single product in bulk create request
type BulkCreateProductItem struct {
	Name              string             `json:"name" binding:"required"`
	Slug              *string            `json:"slug,omitempty"`
	SKU               string             `json:"sku" binding:"required"`
	Description       *string            `json:"description,omitempty"`
	Price             string             `json:"price" binding:"required"`
	ComparePrice      *string            `json:"comparePrice,omitempty"`
	CostPrice         *string            `json:"costPrice,omitempty"`
	VendorID          string             `json:"vendorId" binding:"required"`
	CategoryID        string             `json:"categoryId" binding:"required"`
	// Optional warehouse - use ID or Name (Name will auto-create if not exists)
	WarehouseID   *string `json:"warehouseId,omitempty"`
	WarehouseName *string `json:"warehouseName,omitempty"`
	// Optional supplier - use ID or Name (Name will auto-create if not exists)
	SupplierID   *string `json:"supplierId,omitempty"`
	SupplierName *string `json:"supplierName,omitempty"`
	Brand             *string            `json:"brand,omitempty"`
	Quantity          *int               `json:"quantity,omitempty"`
	MinOrderQty       *int               `json:"minOrderQty,omitempty"`
	MaxOrderQty       *int               `json:"maxOrderQty,omitempty"`
	LowStockThreshold *int               `json:"lowStockThreshold,omitempty"`
	Weight            *string            `json:"weight,omitempty"`
	Dimensions        *JSON              `json:"dimensions,omitempty"`
	SearchKeywords    *string            `json:"searchKeywords,omitempty"`
	Tags              []string           `json:"tags,omitempty"`
	CurrencyCode      *string            `json:"currencyCode,omitempty"`
	Attributes        []ProductAttribute `json:"attributes,omitempty"`
	Images            []ProductImage     `json:"images,omitempty"`
	// SEO metadata
	SeoTitle       *string  `json:"seoTitle,omitempty"`
	SeoDescription *string  `json:"seoDescription,omitempty"`
	SeoKeywords    []string `json:"seoKeywords,omitempty"`
	OgImage        *string  `json:"ogImage,omitempty"`
	// ExternalID allows client to track items in response
	ExternalID *string `json:"externalId,omitempty"`
}

// BulkCreateProductsRequest represents bulk create request for products
type BulkCreateProductsRequest struct {
	Products       []BulkCreateProductItem `json:"products" binding:"required,min=1,max=100"`
	SkipDuplicates bool                    `json:"skipDuplicates,omitempty"`
}

// BulkCreateResultItem represents result for a single item (generic pattern)
type BulkCreateResultItem struct {
	Index      int         `json:"index"`
	ExternalID *string     `json:"externalId,omitempty"`
	Success    bool        `json:"success"`
	Data       interface{} `json:"data,omitempty"`
	Error      *Error      `json:"error,omitempty"`
}

// BulkCreateProductsResponse represents bulk create response for products
type BulkCreateProductsResponse struct {
	Success      bool                   `json:"success"`
	TotalCount   int                    `json:"totalCount"`
	SuccessCount int                    `json:"successCount"`
	FailedCount  int                    `json:"failedCount"`
	Results      []BulkCreateResultItem `json:"results"`
}

// BulkDeleteProductsRequest represents bulk delete request for products
type BulkDeleteProductsRequest struct {
	IDs []uuid.UUID `json:"ids" binding:"required,min=1,max=100"`
}

// BulkDeleteProductsResponse represents bulk delete response for products
type BulkDeleteProductsResponse struct {
	Success      bool     `json:"success"`
	TotalCount   int      `json:"totalCount"`
	DeletedCount int      `json:"deletedCount"`
	FailedIDs    []string `json:"failedIds,omitempty"`
}

// ExportProductsRequest represents an export request
type ExportProductsRequest struct {
	Format          string                 `json:"format" binding:"required"` // csv, xlsx, json
	Filters         *SearchProductsRequest `json:"filters,omitempty"`
	IncludeVariants *bool                  `json:"includeVariants,omitempty"`
	IncludeImages   *bool                  `json:"includeImages,omitempty"`
}

// CreateCategoryRequest represents a request to create a category
type CreateCategoryRequest struct {
	Name        string  `json:"name" binding:"required"`
	Slug        *string `json:"slug,omitempty"`
	Description *string `json:"description,omitempty"`
	ParentID    *string `json:"parentId,omitempty"`
	ImageURL    *string `json:"imageUrl,omitempty"`  // Category icon/thumbnail
	BannerURL   *string `json:"bannerUrl,omitempty"` // Category banner for storefront
	SortOrder   *int    `json:"sortOrder,omitempty"`
	// SEO metadata
	SeoTitle       *string  `json:"seoTitle,omitempty"`
	SeoDescription *string  `json:"seoDescription,omitempty"`
	SeoKeywords    []string `json:"seoKeywords,omitempty"`
}

// UpdateCategoryRequest represents a request to update a category
type UpdateCategoryRequest struct {
	Name        *string `json:"name,omitempty"`
	Slug        *string `json:"slug,omitempty"`
	Description *string `json:"description,omitempty"`
	ParentID    *string `json:"parentId,omitempty"`
	ImageURL    *string `json:"imageUrl,omitempty"`  // Category icon/thumbnail
	BannerURL   *string `json:"bannerUrl,omitempty"` // Category banner for storefront
	IsActive    *bool   `json:"isActive,omitempty"`
	SortOrder   *int    `json:"sortOrder,omitempty"`
	// SEO metadata
	SeoTitle       *string  `json:"seoTitle,omitempty"`
	SeoDescription *string  `json:"seoDescription,omitempty"`
	SeoKeywords    []string `json:"seoKeywords,omitempty"`
}

// Response types
type PaginationInfo struct {
	Page        int   `json:"page"`
	Limit       int   `json:"limit"`
	Total       int64 `json:"total"`
	TotalPages  int   `json:"totalPages"`
	HasNext     bool  `json:"hasNext"`
	HasPrevious bool  `json:"hasPrevious"`
}

type ProductResponse struct {
	Success bool     `json:"success"`
	Data    *Product `json:"data"`
	Message *string  `json:"message,omitempty"`
}

type ProductListResponse struct {
	Success    bool            `json:"success"`
	Data       []Product       `json:"data"`
	Pagination *PaginationInfo `json:"pagination"`
	Metadata   *JSON           `json:"metadata,omitempty"`
}

type ProductVariantResponse struct {
	Success bool            `json:"success"`
	Data    *ProductVariant `json:"data"`
	Message *string         `json:"message,omitempty"`
}

type ProductVariantListResponse struct {
	Success    bool             `json:"success"`
	Data       []ProductVariant `json:"data"`
	Pagination *PaginationInfo  `json:"pagination"`
}

type CategoryResponse struct {
	Success bool      `json:"success"`
	Data    *Category `json:"data"`
	Message *string   `json:"message,omitempty"`
}

type CategoryListResponse struct {
	Success    bool            `json:"success"`
	Data       []Category      `json:"data"`
	Pagination *PaginationInfo `json:"pagination"`
}

// ProductsAnalyticsResponse represents analytics data
type ProductsAnalyticsResponse struct {
	Success bool              `json:"success"`
	Data    ProductsAnalytics `json:"data"`
	Message *string           `json:"message,omitempty"`
}

type ProductsAnalytics struct {
	Overview     ProductsOverview     `json:"overview"`
	Distribution ProductsDistribution `json:"distribution"`
	Trends       ProductsTrends       `json:"trends"`
	TopProducts  []TopProduct         `json:"topProducts"`
}

type ProductsOverview struct {
	TotalProducts  int     `json:"totalProducts"`
	ActiveProducts int     `json:"activeProducts"`
	DraftProducts  int     `json:"draftProducts"`
	OutOfStock     int     `json:"outOfStock"`
	LowStock       int     `json:"lowStock"`
	TotalVariants  int     `json:"totalVariants"`
	AveragePrice   float64 `json:"averagePrice"`
	TotalInventory int64   `json:"totalInventory"`
}

type ProductsDistribution struct {
	ByStatus    map[ProductStatus]int   `json:"byStatus"`
	ByCategory  map[string]int          `json:"byCategory"`
	ByInventory map[InventoryStatus]int `json:"byInventory"`
}

type ProductsTrends struct {
	Daily   []TrendData `json:"daily"`
	Weekly  []TrendData `json:"weekly"`
	Monthly []TrendData `json:"monthly"`
}

type TrendData struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type TopProduct struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	SKU        string  `json:"sku"`
	ViewCount  int     `json:"viewCount"`
	OrderCount int     `json:"orderCount"`
	Revenue    float64 `json:"revenue"`
}

// SearchAnalytics represents search query tracking
type SearchAnalytics struct {
	ID            uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID      string     `json:"tenantId" gorm:"not null;index"`
	Query         string     `json:"query" gorm:"not null;index"`
	ResultsCount  int        `json:"resultsCount"`
	Filters       *JSON      `json:"filters,omitempty" gorm:"type:jsonb"`
	UserID        *uuid.UUID `json:"userId,omitempty" gorm:"type:uuid"`
	SessionID     *string    `json:"sessionId,omitempty"`
	IPAddress     *string    `json:"ipAddress,omitempty"`
	UserAgent     *string    `json:"userAgent,omitempty"`
	ClickedResult *uuid.UUID `json:"clickedResult,omitempty" gorm:"type:uuid"` // Product ID that was clicked
	CreatedAt     time.Time  `json:"createdAt"`
}

type ErrorResponse struct {
	Success   bool   `json:"success"`
	Error     Error  `json:"error"`
	Timestamp string `json:"timestamp,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
	Details *JSON  `json:"details,omitempty"`
}

type SuccessResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message *string     `json:"message,omitempty"`
}

// TableName returns the table name for the Product model
func (Product) TableName() string {
	return "products"
}

// TableName returns the table name for the ProductVariant model
func (ProductVariant) TableName() string {
	return "product_variants"
}

// TableName returns the table name for the Category model
func (Category) TableName() string {
	return "categories"
}

// TableName returns the table name for the SearchAnalytics model
func (SearchAnalytics) TableName() string {
	return "search_analytics"
}
