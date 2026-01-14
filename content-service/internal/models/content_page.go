package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ContentPageStatus represents the status of a content page
type ContentPageStatus string

const (
	ContentPageStatusDraft     ContentPageStatus = "DRAFT"
	ContentPageStatusPublished ContentPageStatus = "PUBLISHED"
	ContentPageStatusArchived  ContentPageStatus = "ARCHIVED"
)

// ContentPageType represents the type of content page
type ContentPageType string

const (
	ContentPageTypeStatic   ContentPageType = "STATIC"   // About Us, Contact, etc.
	ContentPageTypeBlog     ContentPageType = "BLOG"     // Blog post
	ContentPageTypeFAQ      ContentPageType = "FAQ"      // FAQ page
	ContentPageTypePolicy   ContentPageType = "POLICY"   // Policies (Privacy, Terms, etc.)
	ContentPageTypeLanding  ContentPageType = "LANDING"  // Landing page
	ContentPageTypeCustom   ContentPageType = "CUSTOM"   // Custom content
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

// ContentPage represents a content page entity
type ContentPage struct {
	ID          uuid.UUID         `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID    string            `json:"tenantId" gorm:"type:varchar(255);not null;index"`
	Type        ContentPageType   `json:"type" gorm:"type:varchar(50);not null;index"`
	Status      ContentPageStatus `json:"status" gorm:"type:varchar(20);not null;default:'DRAFT';index"`

	// Basic information
	Title       string            `json:"title" gorm:"type:varchar(500);not null"`
	Slug        string            `json:"slug" gorm:"type:varchar(255);not null;uniqueIndex:idx_tenant_slug"`
	Excerpt     *string           `json:"excerpt,omitempty" gorm:"type:text"`
	Content     string            `json:"content" gorm:"type:text;not null"`

	// SEO fields
	MetaTitle       *string `json:"metaTitle,omitempty" gorm:"type:varchar(255)"`
	MetaDescription *string `json:"metaDescription,omitempty" gorm:"type:varchar(500)"`
	MetaKeywords    *string `json:"metaKeywords,omitempty" gorm:"type:text"`
	OGImage         *string `json:"ogImage,omitempty" gorm:"type:varchar(500)"`

	// Featured image
	FeaturedImage    *string `json:"featuredImage,omitempty" gorm:"type:varchar(500)"`
	FeaturedImageAlt *string `json:"featuredImageAlt,omitempty" gorm:"type:varchar(255)"`

	// Author information
	AuthorID   *uuid.UUID `json:"authorId,omitempty" gorm:"type:uuid;index"`
	AuthorName *string    `json:"authorName,omitempty" gorm:"type:varchar(255)"`

	// Publishing
	PublishedAt *time.Time `json:"publishedAt,omitempty" gorm:"index"`
	ScheduledAt *time.Time `json:"scheduledAt,omitempty" gorm:"index"`

	// Organization
	CategoryID   *uuid.UUID `json:"categoryId,omitempty" gorm:"type:uuid;index"`
	CategoryName *string    `json:"categoryName,omitempty" gorm:"type:varchar(255)"`
	Tags         *JSON      `json:"tags,omitempty" gorm:"type:jsonb"`

	// Display options
	ShowInMenu      bool   `json:"showInMenu" gorm:"default:false"`
	MenuOrder       *int   `json:"menuOrder,omitempty"`
	ShowInFooter    bool   `json:"showInFooter" gorm:"default:false"`
	FooterOrder     *int   `json:"footerOrder,omitempty"`
	TemplateType    *string `json:"templateType,omitempty" gorm:"type:varchar(50)"`

	// Analytics
	ViewCount       int       `json:"viewCount" gorm:"default:0"`
	LastViewedAt    *time.Time `json:"lastViewedAt,omitempty"`

	// Settings
	AllowComments   bool   `json:"allowComments" gorm:"default:false"`
	IsFeatured      bool   `json:"isFeatured" gorm:"default:false;index"`
	IsProtected     bool   `json:"isProtected" gorm:"default:false"`
	RequiresAuth    bool   `json:"requiresAuth" gorm:"default:false"`

	// Metadata
	Metadata    *JSON  `json:"metadata,omitempty" gorm:"type:jsonb"`
	CustomCSS   *string `json:"customCss,omitempty" gorm:"type:text"`
	CustomJS    *string `json:"customJs,omitempty" gorm:"type:text"`

	// Audit fields
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
	DeletedAt *gorm.DeletedAt `json:"deletedAt,omitempty" gorm:"index"`
	CreatedBy *string         `json:"createdBy,omitempty"`
	UpdatedBy *string         `json:"updatedBy,omitempty"`

	// Relations
	Comments []ContentPageComment `json:"comments,omitempty" gorm:"foreignKey:PageID"`
}

// ContentPageComment represents a comment on a content page
type ContentPageComment struct {
	ID        uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID  string          `json:"tenantId" gorm:"type:varchar(255);not null;index"`
	PageID    uuid.UUID       `json:"pageId" gorm:"type:uuid;not null;index"`
	ParentID  *uuid.UUID      `json:"parentId,omitempty" gorm:"type:uuid;index"`

	// Comment details
	AuthorName  string  `json:"authorName" gorm:"type:varchar(255);not null"`
	AuthorEmail string  `json:"authorEmail" gorm:"type:varchar(255);not null"`
	Content     string  `json:"content" gorm:"type:text;not null"`
	Status      string  `json:"status" gorm:"type:varchar(20);not null;default:'PENDING'"`

	// Metadata
	IPAddress   *string `json:"ipAddress,omitempty" gorm:"type:varchar(45)"`
	UserAgent   *string `json:"userAgent,omitempty" gorm:"type:text"`

	// Audit
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
	DeletedAt *gorm.DeletedAt `json:"deletedAt,omitempty" gorm:"index"`
}

// ContentPageCategory represents a category for content pages
type ContentPageCategory struct {
	ID          uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID    string          `json:"tenantId" gorm:"type:varchar(255);not null;index"`
	Name        string          `json:"name" gorm:"type:varchar(255);not null"`
	Slug        string          `json:"slug" gorm:"type:varchar(255);not null;uniqueIndex:idx_tenant_category_slug"`
	Description *string         `json:"description,omitempty" gorm:"type:text"`
	ParentID    *uuid.UUID      `json:"parentId,omitempty" gorm:"type:uuid;index"`
	SortOrder   int             `json:"sortOrder" gorm:"default:0"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
	DeletedAt   *gorm.DeletedAt `json:"deletedAt,omitempty" gorm:"index"`
}

// TableName returns the table name for ContentPage
func (ContentPage) TableName() string {
	return "content_pages"
}

// TableName returns the table name for ContentPageComment
func (ContentPageComment) TableName() string {
	return "content_page_comments"
}

// TableName returns the table name for ContentPageCategory
func (ContentPageCategory) TableName() string {
	return "content_page_categories"
}

// Request/Response models

// CreateContentPageRequest represents a request to create a content page
type CreateContentPageRequest struct {
	Type             ContentPageType   `json:"type" binding:"required"`
	Status           *ContentPageStatus `json:"status,omitempty"`
	Title            string            `json:"title" binding:"required,min=1,max=500"`
	Slug             string            `json:"slug" binding:"required,min=1,max=255"`
	Excerpt          *string           `json:"excerpt,omitempty"`
	Content          string            `json:"content" binding:"required"`
	MetaTitle        *string           `json:"metaTitle,omitempty"`
	MetaDescription  *string           `json:"metaDescription,omitempty"`
	MetaKeywords     *string           `json:"metaKeywords,omitempty"`
	OGImage          *string           `json:"ogImage,omitempty"`
	FeaturedImage    *string           `json:"featuredImage,omitempty"`
	FeaturedImageAlt *string           `json:"featuredImageAlt,omitempty"`
	AuthorName       *string           `json:"authorName,omitempty"`
	CategoryID       *uuid.UUID        `json:"categoryId,omitempty"`
	Tags             *JSON             `json:"tags,omitempty"`
	ShowInMenu       *bool             `json:"showInMenu,omitempty"`
	MenuOrder        *int              `json:"menuOrder,omitempty"`
	ShowInFooter     *bool             `json:"showInFooter,omitempty"`
	FooterOrder      *int              `json:"footerOrder,omitempty"`
	TemplateType     *string           `json:"templateType,omitempty"`
	AllowComments    *bool             `json:"allowComments,omitempty"`
	IsFeatured       *bool             `json:"isFeatured,omitempty"`
	RequiresAuth     *bool             `json:"requiresAuth,omitempty"`
	ScheduledAt      *time.Time        `json:"scheduledAt,omitempty"`
	Metadata         *JSON             `json:"metadata,omitempty"`
	CustomCSS        *string           `json:"customCss,omitempty"`
	CustomJS         *string           `json:"customJs,omitempty"`
}

// UpdateContentPageRequest represents a request to update a content page
type UpdateContentPageRequest struct {
	Type             *ContentPageType   `json:"type,omitempty"`
	Status           *ContentPageStatus `json:"status,omitempty"`
	Title            *string            `json:"title,omitempty"`
	Slug             *string            `json:"slug,omitempty"`
	Excerpt          *string            `json:"excerpt,omitempty"`
	Content          *string            `json:"content,omitempty"`
	MetaTitle        *string            `json:"metaTitle,omitempty"`
	MetaDescription  *string            `json:"metaDescription,omitempty"`
	MetaKeywords     *string            `json:"metaKeywords,omitempty"`
	OGImage          *string            `json:"ogImage,omitempty"`
	FeaturedImage    *string            `json:"featuredImage,omitempty"`
	FeaturedImageAlt *string            `json:"featuredImageAlt,omitempty"`
	AuthorName       *string            `json:"authorName,omitempty"`
	CategoryID       *uuid.UUID         `json:"categoryId,omitempty"`
	Tags             *JSON              `json:"tags,omitempty"`
	ShowInMenu       *bool              `json:"showInMenu,omitempty"`
	MenuOrder        *int               `json:"menuOrder,omitempty"`
	ShowInFooter     *bool              `json:"showInFooter,omitempty"`
	FooterOrder      *int               `json:"footerOrder,omitempty"`
	TemplateType     *string            `json:"templateType,omitempty"`
	AllowComments    *bool              `json:"allowComments,omitempty"`
	IsFeatured       *bool              `json:"isFeatured,omitempty"`
	RequiresAuth     *bool              `json:"requiresAuth,omitempty"`
	ScheduledAt      *time.Time         `json:"scheduledAt,omitempty"`
	Metadata         *JSON              `json:"metadata,omitempty"`
	CustomCSS        *string            `json:"customCss,omitempty"`
	CustomJS         *string            `json:"customJs,omitempty"`
}

// SearchContentPagesRequest represents search parameters
type SearchContentPagesRequest struct {
	Query       *string              `json:"query,omitempty"`
	Type        []ContentPageType    `json:"type,omitempty"`
	Status      []ContentPageStatus  `json:"status,omitempty"`
	CategoryID  *uuid.UUID           `json:"categoryId,omitempty"`
	Tags        []string             `json:"tags,omitempty"`
	AuthorID    *uuid.UUID           `json:"authorId,omitempty"`
	IsFeatured  *bool                `json:"isFeatured,omitempty"`
	ShowInMenu  *bool                `json:"showInMenu,omitempty"`
	DateFrom    *time.Time           `json:"dateFrom,omitempty"`
	DateTo      *time.Time           `json:"dateTo,omitempty"`
	SortBy      *string              `json:"sortBy,omitempty"`
	SortOrder   *string              `json:"sortOrder,omitempty"`
	Page        int                  `json:"page"`
	Limit       int                  `json:"limit"`
}

// ContentPageStats represents content page statistics
type ContentPageStats struct {
	TotalPages      int64   `json:"totalPages"`
	PublishedPages  int64   `json:"publishedPages"`
	DraftPages      int64   `json:"draftPages"`
	TotalViews      int64   `json:"totalViews"`
	TotalComments   int64   `json:"totalComments"`
	PagesByType     map[string]int64 `json:"pagesByType"`
}

// Response models

type ContentPageResponse struct {
	Success bool         `json:"success"`
	Data    *ContentPage `json:"data,omitempty"`
	Message *string      `json:"message,omitempty"`
}

type ContentPageListResponse struct {
	Success    bool             `json:"success"`
	Data       []ContentPage    `json:"data"`
	Pagination *PaginationInfo  `json:"pagination,omitempty"`
}

type ContentPageStatsResponse struct {
	Success bool              `json:"success"`
	Data    *ContentPageStats `json:"data"`
}

type PaginationInfo struct {
	Page        int   `json:"page"`
	Limit       int   `json:"limit"`
	Total       int64 `json:"total"`
	TotalPages  int   `json:"totalPages"`
	HasNext     bool  `json:"hasNext"`
	HasPrevious bool  `json:"hasPrevious"`
}

type ErrorResponse struct {
	Success   bool   `json:"success"`
	Error     Error  `json:"error"`
	Timestamp string `json:"timestamp,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details *JSON  `json:"details,omitempty"`
}

type SuccessResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message *string     `json:"message,omitempty"`
}
