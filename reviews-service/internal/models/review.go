package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ReviewStatus represents the status of a review
type ReviewStatus string

const (
	ReviewStatusDraft    ReviewStatus = "DRAFT"
	ReviewStatusPending  ReviewStatus = "PENDING"
	ReviewStatusApproved ReviewStatus = "APPROVED"
	ReviewStatusRejected ReviewStatus = "REJECTED"
	ReviewStatusFlagged  ReviewStatus = "FLAGGED"
	ReviewStatusArchived ReviewStatus = "ARCHIVED"
)

// ReviewType represents the type of review
type ReviewType string

const (
	ReviewTypeProduct    ReviewType = "PRODUCT"
	ReviewTypeService    ReviewType = "SERVICE"
	ReviewTypeVendor     ReviewType = "VENDOR"
	ReviewTypeExperience ReviewType = "EXPERIENCE"
)

// VisibilityType represents the visibility of a review
type VisibilityType string

const (
	VisibilityPublic   VisibilityType = "PUBLIC"
	VisibilityPrivate  VisibilityType = "PRIVATE"
	VisibilityInternal VisibilityType = "INTERNAL"
)

// MediaType represents the type of media
type MediaType string

const (
	MediaTypeImage MediaType = "IMAGE"
	MediaTypeVideo MediaType = "VIDEO"
	MediaTypeAudio MediaType = "AUDIO"
	MediaTypeFile  MediaType = "FILE"
)

// ReactionType represents the type of reaction
type ReactionType string

const (
	ReactionTypeHelpful ReactionType = "helpful"
	ReactionTypeLike    ReactionType = "like"
	ReactionTypeDislike ReactionType = "dislike"
	ReactionTypeLove    ReactionType = "love"
	ReactionTypeAngry   ReactionType = "angry"
	ReactionTypeLaugh   ReactionType = "laugh"
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

// Rating represents a rating aspect
type Rating struct {
	ID       string  `json:"id"`
	Aspect   string  `json:"aspect"`
	Score    float64 `json:"score"`
	MaxScore int     `json:"maxScore"`
}

// Media represents review media
type Media struct {
	ID           string    `json:"id"`
	Type         MediaType `json:"type"`
	URL          string    `json:"url"`
	ThumbnailURL *string   `json:"thumbnailUrl,omitempty"`
	Caption      *string   `json:"caption,omitempty"`
	FileSize     *int      `json:"fileSize,omitempty"`
	Width        *int      `json:"width,omitempty"`
	Height       *int      `json:"height,omitempty"`
	Duration     *int      `json:"duration,omitempty"`
	UploadedAt   time.Time `json:"uploadedAt"`
}

// Comment represents a review comment
type Comment struct {
	ID         string     `json:"id"`
	UserID     string     `json:"userId"`
	UserName   string     `json:"userName"`
	Content    string     `json:"content"`
	IsInternal bool       `json:"isInternal"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  *time.Time `json:"updatedAt,omitempty"`
}

// Reaction represents a review reaction
type Reaction struct {
	ID        string       `json:"id"`
	UserID    string       `json:"userId"`
	Type      ReactionType `json:"type"`
	CreatedAt time.Time    `json:"createdAt"`
}

// Tag represents a review tag
type Tag struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Color *string `json:"color,omitempty"`
}

// Review represents a customer review
// VendorID enables vendor-specific review filtering in marketplace mode
type Review struct {
	ID               uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID         string          `json:"tenantId" gorm:"not null;index"`
	VendorID         string          `json:"vendorId,omitempty" gorm:"index:idx_reviews_tenant_vendor;index:idx_reviews_vendor_target;index:idx_reviews_vendor_status"` // Vendor isolation for marketplace
	ApplicationID    string          `json:"applicationId" gorm:"not null"`
	TargetID         string          `json:"targetId" gorm:"not null;index"`
	TargetType       string          `json:"targetType" gorm:"not null"`
	UserID           string          `json:"userId" gorm:"not null;index"`
	UserName         *string         `json:"userName,omitempty"`
	UserEmail        *string         `json:"userEmail,omitempty" gorm:"-"`
	Title            *string         `json:"title,omitempty"`
	Content          string          `json:"content" gorm:"not null"`
	Status           ReviewStatus    `json:"status" gorm:"not null;default:'PENDING'"`
	Type             ReviewType      `json:"type" gorm:"not null"`
	Visibility       VisibilityType  `json:"visibility" gorm:"not null;default:'PUBLIC'"`
	Ratings          *JSON           `json:"ratings,omitempty" gorm:"type:jsonb"`
	Comments         *JSON           `json:"comments,omitempty" gorm:"type:jsonb"`
	Reactions        *JSON           `json:"reactions,omitempty" gorm:"type:jsonb"`
	Media            *JSON           `json:"media,omitempty" gorm:"type:jsonb"`
	Tags             *JSON           `json:"tags,omitempty" gorm:"type:jsonb"`
	HelpfulCount     int             `json:"helpfulCount" gorm:"default:0"`
	ReportCount      int             `json:"reportCount" gorm:"default:0"`
	Featured         bool            `json:"featured" gorm:"default:false"`
	VerifiedPurchase bool            `json:"verifiedPurchase" gorm:"default:false"`
	Language         *string         `json:"language,omitempty"`
	IPAddress        *string         `json:"ipAddress,omitempty"`
	UserAgent        *string         `json:"userAgent,omitempty"`
	SpamScore        *float64        `json:"spamScore,omitempty"`
	SentimentScore   *float64        `json:"sentimentScore,omitempty"`
	ModerationNotes  *string         `json:"moderationNotes,omitempty"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
	PublishedAt      *time.Time      `json:"publishedAt,omitempty"`
	DeletedAt        *gorm.DeletedAt `json:"deletedAt,omitempty" gorm:"index"`
	CreatedBy        *string         `json:"createdBy,omitempty"`
	UpdatedBy        *string         `json:"updatedBy,omitempty"`
	Metadata         *JSON           `json:"metadata,omitempty" gorm:"type:jsonb"`
}

// CreateReviewRequest represents a request to create a new review
type CreateReviewRequest struct {
	TargetID         string          `json:"targetId" binding:"required"`
	TargetType       string          `json:"targetType" binding:"required"`
	Title            *string         `json:"title,omitempty"`
	Content          string          `json:"content" binding:"required"`
	Type             ReviewType      `json:"type" binding:"required"`
	Visibility       *VisibilityType `json:"visibility,omitempty"`
	Ratings          []Rating        `json:"ratings,omitempty"`
	Tags             []string        `json:"tags,omitempty"`
	VerifiedPurchase *bool           `json:"verifiedPurchase,omitempty"`
	Language         *string         `json:"language,omitempty"`
	Metadata         *JSON           `json:"metadata,omitempty"`
}

// StorefrontCreateReviewRequest represents a public storefront review submission (no JWT required)
type StorefrontCreateReviewRequest struct {
	TargetID      string     `json:"targetId" binding:"required"`
	TargetType    string     `json:"targetType" binding:"required"`
	Title         *string    `json:"title,omitempty"`
	Content       string     `json:"content" binding:"required"`
	Type          ReviewType `json:"type" binding:"required"`
	Ratings       []Rating   `json:"ratings,omitempty"`
	ReviewerName  string     `json:"reviewerName" binding:"required"`
	ReviewerEmail string     `json:"reviewerEmail" binding:"required,email"`
	Language      *string    `json:"language,omitempty"`
}

// UpdateReviewRequest represents a request to update a review
type UpdateReviewRequest struct {
	Title      *string         `json:"title,omitempty"`
	Content    *string         `json:"content,omitempty"`
	Visibility *VisibilityType `json:"visibility,omitempty"`
	Ratings    []Rating        `json:"ratings,omitempty"`
	Tags       []string        `json:"tags,omitempty"`
	Featured   *bool           `json:"featured,omitempty"`
	Metadata   *JSON           `json:"metadata,omitempty"`
}

// UpdateStatusRequest represents a request to update review status
type UpdateStatusRequest struct {
	Status          ReviewStatus `json:"status" binding:"required"`
	ModerationNotes *string      `json:"moderationNotes,omitempty"`
}

// BulkUpdateRequest represents a bulk update request
type BulkUpdateRequest struct {
	ReviewIDs       []string     `json:"reviewIds" binding:"required"`
	Status          ReviewStatus `json:"status" binding:"required"`
	ModerationNotes *string      `json:"moderationNotes,omitempty"`
}

// AddMediaRequest represents a request to add media
type AddMediaRequest struct {
	Type         MediaType `json:"type" binding:"required"`
	URL          string    `json:"url" binding:"required"`
	ThumbnailURL *string   `json:"thumbnailUrl,omitempty"`
	Caption      *string   `json:"caption,omitempty"`
	FileSize     *int      `json:"fileSize,omitempty"`
	Width        *int      `json:"width,omitempty"`
	Height       *int      `json:"height,omitempty"`
	Duration     *int      `json:"duration,omitempty"`
}

// AddReactionRequest represents a request to add a reaction
type AddReactionRequest struct {
	Type ReactionType `json:"type" binding:"required"`
}

// AddCommentRequest represents a request to add a comment
type AddCommentRequest struct {
	Content    string `json:"content" binding:"required"`
	IsInternal *bool  `json:"isInternal,omitempty"`
}

// UpdateCommentRequest represents a request to update a comment
type UpdateCommentRequest struct {
	Content string `json:"content" binding:"required"`
}

// SearchReviewsRequest represents a search request
type SearchReviewsRequest struct {
	VendorID   string         `json:"vendorId,omitempty"` // Vendor isolation for marketplace
	Query      *string        `json:"query,omitempty"`
	TargetType *string        `json:"targetType,omitempty"`
	TargetIDs  []string       `json:"targetIds,omitempty"`
	Status     []ReviewStatus `json:"status,omitempty"`
	Type       []ReviewType   `json:"type,omitempty"`
	UserID     *string        `json:"userId,omitempty"`
	Featured   *bool          `json:"featured,omitempty"`
	MinRating  *float64       `json:"minRating,omitempty"`
	MaxRating  *float64       `json:"maxRating,omitempty"`
	Tags       []string       `json:"tags,omitempty"`
	DateFrom   *time.Time     `json:"dateFrom,omitempty"`
	DateTo     *time.Time     `json:"dateTo,omitempty"`
	Language   *string        `json:"language,omitempty"`
	SortBy     *string        `json:"sortBy,omitempty"`
	SortOrder  *string        `json:"sortOrder,omitempty"`
	Page       int            `json:"page"`
	Limit      int            `json:"limit"`
}

// ReportReviewRequest represents a request to report a review
type ReportReviewRequest struct {
	Reason      string  `json:"reason" binding:"required"`
	Description *string `json:"description,omitempty"`
}

// ExportReviewsRequest represents an export request
type ExportReviewsRequest struct {
	Format       string                `json:"format" binding:"required"` // csv, xlsx, json
	Filters      *SearchReviewsRequest `json:"filters,omitempty"`
	IncludeMedia *bool                 `json:"includeMedia,omitempty"`
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

// ReviewResponse represents a single review response
type ReviewResponse struct {
	Success bool    `json:"success"`
	Data    *Review `json:"data"`
	Message *string `json:"message,omitempty"`
}

// ReviewListResponse represents a list of reviews response
type ReviewListResponse struct {
	Success    bool            `json:"success"`
	Data       []Review        `json:"data"`
	Pagination *PaginationInfo `json:"pagination"`
	Metadata   *JSON           `json:"metadata,omitempty"`
}

// ReviewsAnalyticsResponse represents analytics data
type ReviewsAnalyticsResponse struct {
	Success bool             `json:"success"`
	Data    ReviewsAnalytics `json:"data"`
	Message *string          `json:"message,omitempty"`
}

// ReviewsAnalytics represents review analytics data
type ReviewsAnalytics struct {
	Overview          ReviewsOverview     `json:"overview"`
	Distribution      ReviewsDistribution `json:"distribution"`
	Trends            ReviewsTrends       `json:"trends"`
	TopReviewed       []TopReviewedItem   `json:"topReviewed"`
	SentimentAnalysis SentimentAnalysis   `json:"sentimentAnalysis"`
}

// ReviewsOverview represents overview statistics
type ReviewsOverview struct {
	TotalReviews     int     `json:"totalReviews"`
	AverageRating    float64 `json:"averageRating"`
	FeaturedCount    int     `json:"featuredCount"`
	VerifiedCount    int     `json:"verifiedCount"`
	PendingCount     int     `json:"pendingCount"`
	FlaggedCount     int     `json:"flaggedCount"`
	ResponseRate     float64 `json:"responseRate"`
	SatisfactionRate float64 `json:"satisfactionRate"`
}

// ReviewsDistribution represents distribution data
type ReviewsDistribution struct {
	ByStatus   map[ReviewStatus]int `json:"byStatus"`
	ByType     map[ReviewType]int   `json:"byType"`
	ByRating   map[string]int       `json:"byRating"`
	ByLanguage map[string]int       `json:"byLanguage"`
}

// ReviewsTrends represents trend data
type ReviewsTrends struct {
	Daily   []TrendData `json:"daily"`
	Weekly  []TrendData `json:"weekly"`
	Monthly []TrendData `json:"monthly"`
}

// TrendData represents time-series data point
type TrendData struct {
	Date          string  `json:"date"`
	Count         int     `json:"count"`
	AverageRating float64 `json:"averageRating"`
}

// TopReviewedItem represents a top reviewed item
type TopReviewedItem struct {
	TargetID      string  `json:"targetId"`
	TargetType    string  `json:"targetType"`
	Name          string  `json:"name"`
	ReviewCount   int     `json:"reviewCount"`
	AverageRating float64 `json:"averageRating"`
}

// SentimentAnalysis represents sentiment analysis data
type SentimentAnalysis struct {
	Positive float64          `json:"positive"`
	Negative float64          `json:"negative"`
	Neutral  float64          `json:"neutral"`
	Themes   []SentimentTheme `json:"themes"`
}

// SentimentTheme represents a sentiment theme
type SentimentTheme struct {
	Theme     string   `json:"theme"`
	Sentiment float64  `json:"sentiment"`
	Frequency int      `json:"frequency"`
	Keywords  []string `json:"keywords"`
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

// TableName returns the table name for the Review model
func (Review) TableName() string {
	return "reviews"
}
