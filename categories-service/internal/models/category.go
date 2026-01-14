package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CategoryStatusEnum represents the possible states of a category
type CategoryStatusEnum string

const (
	StatusDraft    CategoryStatusEnum = "DRAFT"
	StatusPending  CategoryStatusEnum = "PENDING"
	StatusApproved CategoryStatusEnum = "APPROVED"
	StatusRejected CategoryStatusEnum = "REJECTED"
)

// CategoryTier represents the tier level of a category
type CategoryTier string

const (
	TierBasic      CategoryTier = "BASIC"
	TierPremium    CategoryTier = "PREMIUM"
	TierEnterprise CategoryTier = "ENTERPRISE"
)

// JSON type for PostgreSQL JSONB
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

// CategoryImage represents a category image (mirrors ProductImage pattern)
type CategoryImage struct {
	ID       string  `json:"id"`
	URL      string  `json:"url"`
	AltText  *string `json:"altText,omitempty"`
	Position int     `json:"position"`
	Width    *int    `json:"width,omitempty"`
	Height   *int    `json:"height,omitempty"`
}

// CategoryMediaLimits defines upload limits for category media
var CategoryMediaLimits = struct {
	MaxImages          int   // Max images per category (3)
	MaxImageSizeBytes  int64 // Max size for images (10MB)
	MaxBannerSizeBytes int64 // Max size for banners (5MB)
}{
	MaxImages:          3,
	MaxImageSizeBytes:  10 * 1024 * 1024,  // 10MB
	MaxBannerSizeBytes: 5 * 1024 * 1024,   // 5MB
}

// Category represents a product category
type Category struct {
	ID            uuid.UUID          `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID      string             `json:"tenantId" gorm:"not null;index"`
	CreatedByID   string             `json:"createdById" gorm:"not null"`
	UpdatedByID   string             `json:"updatedById" gorm:"not null"`
	Name          string             `json:"name" gorm:"not null"`
	Slug          string             `json:"slug" gorm:"not null;uniqueIndex:idx_tenant_slug"`
	Description   *string            `json:"description,omitempty"`
	ImageURL      *string            `json:"imageUrl,omitempty"`
	BannerURL     *string            `json:"bannerUrl,omitempty"`
	Images        *JSONArray         `json:"images,omitempty" gorm:"type:jsonb"` // Gallery images (max 3)
	ParentID      *uuid.UUID         `json:"parentId,omitempty" gorm:"index"`
	Level         int                `json:"level" gorm:"not null;default:0"`
	Position      int                `json:"position" gorm:"not null;default:1"`
	IsActive      bool               `json:"isActive" gorm:"default:true"`
	Status        CategoryStatusEnum `json:"status" gorm:"not null;default:'DRAFT'"`
	Tier          *CategoryTier      `json:"tier,omitempty"`
	Tags          *JSON              `json:"tags,omitempty" gorm:"type:jsonb"`
	SeoTitle      *string            `json:"seoTitle,omitempty"`
	SeoDescription *string           `json:"seoDescription,omitempty"`
	SeoKeywords   *JSON              `json:"seoKeywords,omitempty" gorm:"type:jsonb"`
	Metadata      *JSON              `json:"metadata,omitempty" gorm:"type:jsonb"`
	CreatedAt     time.Time          `json:"createdAt"`
	UpdatedAt     time.Time          `json:"updatedAt"`
	DeletedAt     *gorm.DeletedAt    `json:"deletedAt,omitempty" gorm:"index"`

	// Relationships
	Parent   *Category  `json:"parent,omitempty" gorm:"foreignKey:ParentID"`
	Children []Category `json:"children,omitempty" gorm:"foreignKey:ParentID"`
}

// CategoryAudit represents audit log for category changes
type CategoryAudit struct {
	ID            uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	CategoryID    uuid.UUID `json:"categoryId" gorm:"not null;index"`
	UserID        string    `json:"userId" gorm:"not null"`
	Action        string    `json:"action" gorm:"not null"`
	FieldsChanged *JSON     `json:"fieldsChanged,omitempty" gorm:"type:jsonb"`
	OldValues     *JSON     `json:"oldValues,omitempty" gorm:"type:jsonb"`
	NewValues     *JSON     `json:"newValues,omitempty" gorm:"type:jsonb"`
	Timestamp     time.Time `json:"timestamp" gorm:"not null"`
	IPAddress     *string   `json:"ipAddress,omitempty"`
}

// CategoryPosition represents position update request
type CategoryPosition struct {
	CategoryID uuid.UUID  `json:"categoryId" gorm:"not null"`
	Position   int        `json:"position" gorm:"not null"`
	ParentID   *uuid.UUID `json:"parentId,omitempty"`
}

// CreateCategoryRequest represents a request to create a new category
type CreateCategoryRequest struct {
	Name           string          `json:"name" binding:"required"`
	Slug           *string         `json:"slug,omitempty"`
	Description    *string         `json:"description,omitempty"`
	ImageURL       *string         `json:"imageUrl,omitempty"`
	BannerURL      *string         `json:"bannerUrl,omitempty"`
	Images         []CategoryImage `json:"images,omitempty"` // Gallery images (max 3)
	ParentID       *uuid.UUID      `json:"parentId,omitempty"`
	Position       *int            `json:"position,omitempty"`
	IsActive       *bool           `json:"isActive,omitempty"`
	Tier           *CategoryTier   `json:"tier,omitempty"`
	Tags           []string        `json:"tags,omitempty"`
	SeoTitle       *string         `json:"seoTitle,omitempty"`
	SeoDescription *string         `json:"seoDescription,omitempty"`
	SeoKeywords    []string        `json:"seoKeywords,omitempty"`
	Metadata       *JSON           `json:"metadata,omitempty"`
}

// UpdateCategoryRequest represents a request to update a category
type UpdateCategoryRequest struct {
	Name           *string             `json:"name,omitempty"`
	Slug           *string             `json:"slug,omitempty"`
	Description    *string             `json:"description,omitempty"`
	ImageURL       *string             `json:"imageUrl,omitempty"`
	BannerURL      *string             `json:"bannerUrl,omitempty"`
	Images         []CategoryImage     `json:"images,omitempty"` // Gallery images (max 3)
	ParentID       *uuid.UUID          `json:"parentId,omitempty"`
	Position       *int                `json:"position,omitempty"`
	IsActive       *bool               `json:"isActive,omitempty"`
	Status         *CategoryStatusEnum `json:"status,omitempty"`
	Tier           *CategoryTier       `json:"tier,omitempty"`
	Tags           []string            `json:"tags,omitempty"`
	SeoTitle       *string             `json:"seoTitle,omitempty"`
	SeoDescription *string             `json:"seoDescription,omitempty"`
	SeoKeywords    []string            `json:"seoKeywords,omitempty"`
	Metadata       *JSON               `json:"metadata,omitempty"`
}

// UpdateCategoryStatusRequest represents a request to update category status
type UpdateCategoryStatusRequest struct {
	Status CategoryStatusEnum `json:"status" binding:"required"`
}

// ReorderCategoryRequest represents a request to reorder categories
type ReorderCategoryRequest struct {
	CategoryID uuid.UUID  `json:"categoryId" binding:"required"`
	Position   int        `json:"position" binding:"required"`
	ParentID   *uuid.UUID `json:"parentId,omitempty"`
}

// CategoryFilters represents filters for category queries
type CategoryFilters struct {
	Names       []string             `json:"names,omitempty"`
	Slugs       []string             `json:"slugs,omitempty"`
	Statuses    []CategoryStatusEnum `json:"statuses,omitempty"`
	Tiers       []CategoryTier       `json:"tiers,omitempty"`
	ParentIDs   []uuid.UUID          `json:"parentIds,omitempty"`
	Levels      []int                `json:"levels,omitempty"`
	IsActive    *bool                `json:"isActive,omitempty"`
	Tags        []string             `json:"tags,omitempty"`
	HasChildren *bool                `json:"hasChildren,omitempty"`
}

// PaginationInfo represents pagination information
type PaginationInfo struct {
	Page        int   `json:"page"`
	Limit       int   `json:"limit"`
	Total       int64 `json:"total"`
	TotalPages  int   `json:"totalPages"`
	HasNext     bool  `json:"hasNext"`
	HasPrevious bool  `json:"hasPrevious"`
}

// CategoryResponse represents a single category response
type CategoryResponse struct {
	Success bool      `json:"success"`
	Data    *Category `json:"data"`
	Message *string   `json:"message,omitempty"`
}

// CategoryListResponse represents a list of categories response
type CategoryListResponse struct {
	Success    bool            `json:"success"`
	Data       []Category      `json:"data"`
	Pagination *PaginationInfo `json:"pagination"`
	Metadata   *JSON           `json:"metadata,omitempty"`
}

// CategoryTreeResponse represents hierarchical category tree response
type CategoryTreeResponse struct {
	Success bool      `json:"success"`
	Data    []Category `json:"data"`
	Message *string   `json:"message,omitempty"`
}

// CategoryAuditResponse represents category audit response
type CategoryAuditResponse struct {
	Success    bool            `json:"success"`
	Data       []CategoryAudit `json:"data"`
	Pagination *PaginationInfo `json:"pagination"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Success   bool   `json:"success"`
	Error     Error  `json:"error"`
	Timestamp string `json:"timestamp,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}

// Error represents error details
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
	Details *JSON  `json:"details,omitempty"`
}

// ============================================================================
// Bulk Operation Models
// ============================================================================

// BulkCreateCategoryItem represents a single category in bulk create request
type BulkCreateCategoryItem struct {
	Name           string        `json:"name" binding:"required"`
	Slug           *string       `json:"slug,omitempty"`
	Description    *string       `json:"description,omitempty"`
	ImageURL       *string       `json:"imageUrl,omitempty"`
	BannerURL      *string       `json:"bannerUrl,omitempty"`
	ParentID       *uuid.UUID    `json:"parentId,omitempty"`
	Position       *int          `json:"position,omitempty"`
	IsActive       *bool         `json:"isActive,omitempty"`
	Tier           *CategoryTier `json:"tier,omitempty"`
	Tags           []string      `json:"tags,omitempty"`
	SeoTitle       *string       `json:"seoTitle,omitempty"`
	SeoDescription *string       `json:"seoDescription,omitempty"`
	SeoKeywords    []string      `json:"seoKeywords,omitempty"`
	Metadata       *JSON         `json:"metadata,omitempty"`
	// ExternalID allows client to track items in response
	ExternalID     *string       `json:"externalId,omitempty"`
}

// BulkCreateCategoriesRequest represents bulk create request
type BulkCreateCategoriesRequest struct {
	Categories []BulkCreateCategoryItem `json:"categories" binding:"required,min=1,max=100"`
	// SkipDuplicates - if true, skip items with duplicate slugs instead of failing
	SkipDuplicates bool `json:"skipDuplicates,omitempty"`
}

// BulkCreateResultItem represents result for a single item
type BulkCreateResultItem struct {
	Index      int       `json:"index"`
	ExternalID *string   `json:"externalId,omitempty"`
	Success    bool      `json:"success"`
	Category   *Category `json:"category,omitempty"`
	Error      *Error    `json:"error,omitempty"`
}

// BulkCreateCategoriesResponse represents bulk create response
type BulkCreateCategoriesResponse struct {
	Success      bool                   `json:"success"`
	TotalCount   int                    `json:"totalCount"`
	SuccessCount int                    `json:"successCount"`
	FailedCount  int                    `json:"failedCount"`
	Results      []BulkCreateResultItem `json:"results"`
}

// BulkDeleteCategoriesRequest represents bulk delete request
type BulkDeleteCategoriesRequest struct {
	IDs []uuid.UUID `json:"ids" binding:"required,min=1,max=100"`
}

// BulkDeleteCategoriesResponse represents bulk delete response
type BulkDeleteCategoriesResponse struct {
	Success      bool     `json:"success"`
	TotalCount   int      `json:"totalCount"`
	DeletedCount int      `json:"deletedCount"`
	FailedIDs    []string `json:"failedIds,omitempty"`
}

// BulkUpdateCategoryStatusRequest represents bulk status update request
type BulkUpdateCategoryStatusRequest struct {
	IDs    []uuid.UUID        `json:"ids" binding:"required,min=1,max=100"`
	Status CategoryStatusEnum `json:"status" binding:"required"`
}

// BulkUpdateCategoryStatusResponse represents bulk status update response
type BulkUpdateCategoryStatusResponse struct {
	Success      bool     `json:"success"`
	TotalCount   int      `json:"totalCount"`
	UpdatedCount int      `json:"updatedCount"`
	FailedIDs    []string `json:"failedIds,omitempty"`
	Message      string   `json:"message,omitempty"`
}

// TableName returns the table name for the Category model
func (Category) TableName() string {
	return "categories"
}

// TableName returns the table name for the CategoryAudit model
func (CategoryAudit) TableName() string {
	return "category_audit"
}