package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Storefront represents a storefront entity linked to a vendor
// One vendor (tenant) can have multiple storefronts
type Storefront struct {
	ID           uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	VendorID     uuid.UUID       `json:"vendorId" gorm:"type:uuid;not null;index"`
	Slug         string          `json:"slug" gorm:"uniqueIndex;not null;size:100"`
	Name         string          `json:"name" gorm:"not null;size:255"`
	CustomDomain *string         `json:"customDomain,omitempty" gorm:"uniqueIndex;size:255"`
	IsActive     bool            `json:"isActive" gorm:"default:false"`
	IsDefault    bool            `json:"isDefault" gorm:"default:false"`
	ThemeConfig  *JSON           `json:"themeConfig,omitempty" gorm:"type:jsonb;default:'{}'"`
	Settings     *JSON           `json:"settings,omitempty" gorm:"type:jsonb;default:'{}'"`
	LogoURL      *string         `json:"logoUrl,omitempty" gorm:"size:500"`
	FaviconURL   *string         `json:"faviconUrl,omitempty" gorm:"size:500"`
	Description  *string         `json:"description,omitempty"`
	MetaTitle    *string         `json:"metaTitle,omitempty" gorm:"size:100"`
	MetaDesc     *string         `json:"metaDescription,omitempty" gorm:"size:300"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
	DeletedAt    *gorm.DeletedAt `json:"deletedAt,omitempty" gorm:"index"`
	CreatedBy    *string         `json:"createdBy,omitempty"`
	UpdatedBy    *string         `json:"updatedBy,omitempty"`

	// Computed field - not stored in database
	// This is populated by the handler based on STOREFRONT_DOMAIN config
	// If customDomain is set, it uses that; otherwise, constructs from slug + domain
	StorefrontURL string `json:"storefrontUrl" gorm:"-"`

	// Relationship
	Vendor Vendor `json:"vendor,omitempty" gorm:"foreignKey:VendorID"`
}

// CreateStorefrontRequest represents a request to create a new storefront
type CreateStorefrontRequest struct {
	VendorID     uuid.UUID `json:"vendorId" binding:"required"`
	Slug         string    `json:"slug" binding:"required,min=3,max=100"`
	Name         string    `json:"name" binding:"required,min=1,max=255"`
	CustomDomain *string   `json:"customDomain,omitempty"`
	IsDefault    bool      `json:"isDefault"`
	ThemeConfig  *JSON     `json:"themeConfig,omitempty"`
	Settings     *JSON     `json:"settings,omitempty"`
	LogoURL      *string   `json:"logoUrl,omitempty"`
	FaviconURL   *string   `json:"faviconUrl,omitempty"`
	Description  *string   `json:"description,omitempty"`
	MetaTitle    *string   `json:"metaTitle,omitempty"`
	MetaDesc     *string   `json:"metaDescription,omitempty"`
}

// UpdateStorefrontRequest represents a request to update a storefront
type UpdateStorefrontRequest struct {
	Slug         *string `json:"slug,omitempty"`
	Name         *string `json:"name,omitempty"`
	CustomDomain *string `json:"customDomain,omitempty"`
	IsActive     *bool   `json:"isActive,omitempty"`
	IsDefault    *bool   `json:"isDefault,omitempty"`
	ThemeConfig  *JSON   `json:"themeConfig,omitempty"`
	Settings     *JSON   `json:"settings,omitempty"`
	LogoURL      *string `json:"logoUrl,omitempty"`
	FaviconURL   *string `json:"faviconUrl,omitempty"`
	Description  *string `json:"description,omitempty"`
	MetaTitle    *string `json:"metaTitle,omitempty"`
	MetaDesc     *string `json:"metaDescription,omitempty"`
}

// StorefrontResponse represents a single storefront response
type StorefrontResponse struct {
	Success bool        `json:"success"`
	Data    *Storefront `json:"data"`
	Message *string     `json:"message,omitempty"`
}

// StorefrontListResponse represents a list of storefronts response
type StorefrontListResponse struct {
	Success    bool            `json:"success"`
	Data       []Storefront    `json:"data"`
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

// StorefrontResolutionResponse is used for tenant resolution by slug/domain
type StorefrontResolutionResponse struct {
	Success bool                      `json:"success"`
	Data    *StorefrontResolutionData `json:"data"`
}

// StorefrontResolutionData contains tenant info for middleware
type StorefrontResolutionData struct {
	StorefrontID   uuid.UUID `json:"storefrontId"`
	TenantID       string    `json:"tenantId"`
	VendorID       uuid.UUID `json:"vendorId"`
	Slug           string    `json:"slug"`
	Name           string    `json:"name"`
	CustomDomain   *string   `json:"customDomain,omitempty"`
	ThemeConfig    *JSON     `json:"themeConfig,omitempty"`
	Settings       *JSON     `json:"settings,omitempty"`
	LogoURL        *string   `json:"logoUrl,omitempty"`
	FaviconURL     *string   `json:"faviconUrl,omitempty"`
	VendorName     string    `json:"vendorName"`
	VendorIsActive bool      `json:"vendorIsActive"`
	// IsActive indicates whether the storefront is published (visible to customers)
	IsActive bool `json:"isActive"`
	// Computed field - the public URL for this storefront
	StorefrontURL string `json:"storefrontUrl"`
}

// TableName returns the table name for the Storefront model
func (Storefront) TableName() string {
	return "storefronts"
}

// ComputeStorefrontURL computes and sets the StorefrontURL field based on config
// If customDomain is set, uses https://{customDomain}
// Otherwise, uses https://{slug}.{storefrontDomain}
func (s *Storefront) ComputeStorefrontURL(storefrontDomain string) {
	if s.CustomDomain != nil && *s.CustomDomain != "" {
		s.StorefrontURL = "https://" + *s.CustomDomain
	} else {
		s.StorefrontURL = "https://" + s.Slug + "." + storefrontDomain
	}
}

// ComputeStorefrontURLForSlug is a helper function to compute the URL without a Storefront instance
func ComputeStorefrontURLForSlug(slug string, customDomain *string, storefrontDomain string) string {
	if customDomain != nil && *customDomain != "" {
		return "https://" + *customDomain
	}
	return "https://" + slug + "." + storefrontDomain
}
