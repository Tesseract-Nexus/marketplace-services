package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CouponScope represents the scope of a coupon
type CouponScope string

const (
	ScopeApplication CouponScope = "APPLICATION"
	ScopeTenant      CouponScope = "TENANT"
	ScopeVendor      CouponScope = "VENDOR"
	ScopeCustom      CouponScope = "CUSTOM"
)

// DiscountType represents the type of discount
type DiscountType string

const (
	DiscountPercentage  DiscountType = "PERCENTAGE"
	DiscountFixed       DiscountType = "FIXED"
	DiscountBuyXGetY    DiscountType = "BUY_X_GET_Y"
	DiscountFreeShipping DiscountType = "FREE_SHIPPING"
)

// CouponStatus represents the status of a coupon
type CouponStatus string

const (
	StatusActive        CouponStatus = "ACTIVE"
	StatusInactive      CouponStatus = "INACTIVE"
	StatusExpired       CouponStatus = "EXPIRED"
	StatusFullyRedeemed CouponStatus = "FULLY_REDEEMED"
	StatusScheduled     CouponStatus = "SCHEDULED"
)

// CouponPriority represents the priority of a coupon
type CouponPriority string

const (
	PriorityLow    CouponPriority = "LOW"
	PriorityMedium CouponPriority = "MEDIUM"
	PriorityHigh   CouponPriority = "HIGH"
)

// PaymentMethod represents allowed payment methods
type PaymentMethod string

const (
	PaymentCreditCard     PaymentMethod = "CREDIT_CARD"
	PaymentDebitCard      PaymentMethod = "DEBIT_CARD"
	PaymentDigitalWallet  PaymentMethod = "DIGITAL_WALLET"
	PaymentBankTransfer   PaymentMethod = "BANK_TRANSFER"
	PaymentCashOnDelivery PaymentMethod = "CASH_ON_DELIVERY"
)

// CouponCombination represents how coupons can be combined
type CouponCombination string

const (
	CombinationNone     CouponCombination = "NONE"
	CombinationAny      CouponCombination = "ANY"
	CombinationSameType CouponCombination = "SAME_TYPE"
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

// Coupon represents a promotional coupon
type Coupon struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID    string    `json:"tenantId" gorm:"not null;index"`
	CreatedByID string    `json:"createdById" gorm:"not null"`
	UpdatedByID string    `json:"updatedById" gorm:"not null"`

	// Basic Information
	Code        string  `json:"code" gorm:"not null;uniqueIndex:idx_tenant_code"`
	Description *string `json:"description,omitempty"`
	DisplayText *string `json:"displayText,omitempty"`
	ImageURL    *string `json:"imageUrl,omitempty"`
	ThumbnailURL *string `json:"thumbnailUrl,omitempty"`

	// Scope and Status
	Scope    CouponScope    `json:"scope" gorm:"not null"`
	Status   CouponStatus   `json:"status" gorm:"not null;default:'ACTIVE'"`
	Priority CouponPriority `json:"priority" gorm:"not null;default:'MEDIUM'"`
	IsActive bool           `json:"isActive" gorm:"default:true"`

	// Discount Configuration
	DiscountType          DiscountType `json:"discountType" gorm:"not null"`
	DiscountValue         float64      `json:"discountValue" gorm:"not null"`
	MaxDiscount           *float64     `json:"maxDiscount,omitempty"`
	MinOrderValue         *float64     `json:"minOrderValue,omitempty"`
	MaxDiscountPerVendor  *float64     `json:"maxDiscountPerVendor,omitempty"`

	// Usage Limits
	MaxUsageCount     *int `json:"maxUsageCount,omitempty"`
	CurrentUsageCount int  `json:"currentUsageCount" gorm:"default:0"`
	MaxUsagePerUser   *int `json:"maxUsagePerUser,omitempty"`
	MaxUsagePerTenant *int `json:"maxUsagePerTenant,omitempty"`
	MaxUsagePerVendor *int `json:"maxUsagePerVendor,omitempty"`

	// Restrictions
	FirstTimeUserOnly bool `json:"firstTimeUserOnly" gorm:"default:false"`
	MinItemCount      *int `json:"minItemCount,omitempty"`
	MaxItemCount      *int `json:"maxItemCount,omitempty"`

	// Target Criteria (stored as JSON arrays)
	ExcludedTenants *JSON `json:"excludedTenants,omitempty" gorm:"type:jsonb"`
	ExcludedVendors *JSON `json:"excludedVendors,omitempty" gorm:"type:jsonb"`
	CategoryIDs     *JSON `json:"categoryIds,omitempty" gorm:"type:jsonb"`
	ProductIDs      *JSON `json:"productIds,omitempty" gorm:"type:jsonb"`
	UserGroupIDs    *JSON `json:"userGroupIds,omitempty" gorm:"type:jsonb"`
	CountryCodes    *JSON `json:"countryCodes,omitempty" gorm:"type:jsonb"`
	RegionCodes     *JSON `json:"regionCodes,omitempty" gorm:"type:jsonb"`

	// Time Restrictions
	ValidFrom   time.Time  `json:"validFrom" gorm:"not null"`
	ValidUntil  *time.Time `json:"validUntil,omitempty"`
	DaysOfWeek  *JSON      `json:"daysOfWeek,omitempty" gorm:"type:jsonb"` // [1,2,3,4,5] for Mon-Fri
	TimeWindows *JSON      `json:"timeWindows,omitempty" gorm:"type:jsonb"`

	// Payment and Stacking
	AllowedPaymentMethods *JSON             `json:"allowedPaymentMethods,omitempty" gorm:"type:jsonb"`
	StackableWithOther    bool              `json:"stackableWithOther" gorm:"default:false"`
	StackablePriority     int               `json:"stackablePriority" gorm:"default:0"`
	Combination           CouponCombination `json:"combination" gorm:"default:'NONE'"`

	// Metadata
	Metadata *JSON `json:"metadata,omitempty" gorm:"type:jsonb"`
	Tags     *JSON `json:"tags,omitempty" gorm:"type:jsonb"`

	// Audit Fields
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
	DeletedAt *gorm.DeletedAt `json:"deletedAt,omitempty" gorm:"index"`
}

// CouponUsage represents a usage record of a coupon
type CouponUsage struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID  string    `json:"tenantId" gorm:"not null;index"`
	CouponID  uuid.UUID `json:"couponId" gorm:"not null;index"`
	UserID    string    `json:"userId" gorm:"not null"`
	OrderID   *string   `json:"orderId,omitempty"`
	VendorID  *string   `json:"vendorId,omitempty"`

	// Usage Details
	DiscountAmount    float64 `json:"discountAmount" gorm:"not null"`
	OrderValue        float64 `json:"orderValue" gorm:"not null"`
	PaymentMethod     *string `json:"paymentMethod,omitempty"`
	ApplicationSource *string `json:"applicationSource,omitempty"`

	// Metadata
	Metadata *JSON `json:"metadata,omitempty" gorm:"type:jsonb"`

	// Audit Fields
	UsedAt    time.Time `json:"usedAt" gorm:"not null"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// Relationships
	Coupon Coupon `json:"coupon,omitempty" gorm:"foreignKey:CouponID"`
}

// CreateCouponRequest represents a request to create a new coupon
type CreateCouponRequest struct {
	Code                  string             `json:"code" binding:"required"`
	Description           *string            `json:"description,omitempty"`
	DisplayText           *string            `json:"displayText,omitempty"`
	ImageURL              *string            `json:"imageUrl,omitempty"`
	ThumbnailURL          *string            `json:"thumbnailUrl,omitempty"`
	Scope                 CouponScope        `json:"scope" binding:"required"`
	Priority              *CouponPriority    `json:"priority,omitempty"`
	DiscountType          DiscountType       `json:"discountType" binding:"required"`
	DiscountValue         float64            `json:"discountValue" binding:"required,gt=0"`
	MaxDiscount           *float64           `json:"maxDiscount,omitempty"`
	MinOrderValue         *float64           `json:"minOrderValue,omitempty"`
	MaxDiscountPerVendor  *float64           `json:"maxDiscountPerVendor,omitempty"`
	MaxUsageCount         *int               `json:"maxUsageCount,omitempty"`
	MaxUsagePerUser       *int               `json:"maxUsagePerUser,omitempty"`
	MaxUsagePerTenant     *int               `json:"maxUsagePerTenant,omitempty"`
	MaxUsagePerVendor     *int               `json:"maxUsagePerVendor,omitempty"`
	FirstTimeUserOnly     *bool              `json:"firstTimeUserOnly,omitempty"`
	MinItemCount          *int               `json:"minItemCount,omitempty"`
	MaxItemCount          *int               `json:"maxItemCount,omitempty"`
	ExcludedTenants       []string           `json:"excludedTenants,omitempty"`
	ExcludedVendors       []string           `json:"excludedVendors,omitempty"`
	CategoryIDs           []string           `json:"categoryIds,omitempty"`
	ProductIDs            []string           `json:"productIds,omitempty"`
	UserGroupIDs          []string           `json:"userGroupIds,omitempty"`
	CountryCodes          []string           `json:"countryCodes,omitempty"`
	RegionCodes           []string           `json:"regionCodes,omitempty"`
	ValidFrom             time.Time          `json:"validFrom" binding:"required"`
	ValidUntil            *time.Time         `json:"validUntil,omitempty"`
	DaysOfWeek            []int              `json:"daysOfWeek,omitempty"`
	AllowedPaymentMethods []PaymentMethod    `json:"allowedPaymentMethods,omitempty"`
	StackableWithOther    *bool              `json:"stackableWithOther,omitempty"`
	StackablePriority     *int               `json:"stackablePriority,omitempty"`
	Combination           *CouponCombination `json:"combination,omitempty"`
	IsActive              *bool              `json:"isActive,omitempty"`
	Metadata              *JSON              `json:"metadata,omitempty"`
	Tags                  []string           `json:"tags,omitempty"`
}

// UpdateCouponRequest represents a request to update a coupon
type UpdateCouponRequest struct {
	Description           *string            `json:"description,omitempty"`
	DisplayText           *string            `json:"displayText,omitempty"`
	ImageURL              *string            `json:"imageUrl,omitempty"`
	ThumbnailURL          *string            `json:"thumbnailUrl,omitempty"`
	Priority              *CouponPriority    `json:"priority,omitempty"`
	Status                *CouponStatus      `json:"status,omitempty"`
	MaxDiscount           *float64           `json:"maxDiscount,omitempty"`
	MinOrderValue         *float64           `json:"minOrderValue,omitempty"`
	MaxDiscountPerVendor  *float64           `json:"maxDiscountPerVendor,omitempty"`
	MaxUsageCount         *int               `json:"maxUsageCount,omitempty"`
	MaxUsagePerUser       *int               `json:"maxUsagePerUser,omitempty"`
	MaxUsagePerTenant     *int               `json:"maxUsagePerTenant,omitempty"`
	MaxUsagePerVendor     *int               `json:"maxUsagePerVendor,omitempty"`
	FirstTimeUserOnly     *bool              `json:"firstTimeUserOnly,omitempty"`
	MinItemCount          *int               `json:"minItemCount,omitempty"`
	MaxItemCount          *int               `json:"maxItemCount,omitempty"`
	ExcludedTenants       []string           `json:"excludedTenants,omitempty"`
	ExcludedVendors       []string           `json:"excludedVendors,omitempty"`
	CategoryIDs           []string           `json:"categoryIds,omitempty"`
	ProductIDs            []string           `json:"productIds,omitempty"`
	UserGroupIDs          []string           `json:"userGroupIds,omitempty"`
	CountryCodes          []string           `json:"countryCodes,omitempty"`
	RegionCodes           []string           `json:"regionCodes,omitempty"`
	ValidUntil            *time.Time         `json:"validUntil,omitempty"`
	DaysOfWeek            []int              `json:"daysOfWeek,omitempty"`
	AllowedPaymentMethods []PaymentMethod    `json:"allowedPaymentMethods,omitempty"`
	StackableWithOther    *bool              `json:"stackableWithOther,omitempty"`
	StackablePriority     *int               `json:"stackablePriority,omitempty"`
	Combination           *CouponCombination `json:"combination,omitempty"`
	IsActive              *bool              `json:"isActive,omitempty"`
	Metadata              *JSON              `json:"metadata,omitempty"`
	Tags                  []string           `json:"tags,omitempty"`
}

// ValidateCouponRequest represents a request to validate a coupon
type ValidateCouponRequest struct {
	Code              string          `json:"code" binding:"required"`
	UserID            string          `json:"userId,omitempty"`
	OrderValue        float64         `json:"orderValue" binding:"required,gt=0"`
	PaymentMethod     *PaymentMethod  `json:"paymentMethod,omitempty"`
	CategoryIDs       []string        `json:"categoryIds,omitempty"`
	ProductIDs        []string        `json:"productIds,omitempty"`
	CountryCode       *string         `json:"countryCode,omitempty"`
	RegionCode        *string         `json:"regionCode,omitempty"`
	IsFirstTimeUser   *bool           `json:"isFirstTimeUser,omitempty"`
	AppliedCoupons    []string        `json:"appliedCoupons,omitempty"`
}

// ApplyCouponRequest represents a request to apply a coupon
type ApplyCouponRequest struct {
	UserID            string         `json:"userId" binding:"required"`
	OrderID           *string        `json:"orderId,omitempty"`
	VendorID          *string        `json:"vendorId,omitempty"`
	OrderValue        float64        `json:"orderValue" binding:"required,gt=0"`
	PaymentMethod     *PaymentMethod `json:"paymentMethod,omitempty"`
	ApplicationSource *string        `json:"applicationSource,omitempty"`
	Metadata          *JSON          `json:"metadata,omitempty"`
	// Optional customer info for notifications
	CustomerEmail *string `json:"customerEmail,omitempty"`
	CustomerName  *string `json:"customerName,omitempty"`
}

// CouponFilters represents filters for coupon queries
type CouponFilters struct {
	Codes           []string           `json:"codes,omitempty"`
	Scopes          []CouponScope      `json:"scopes,omitempty"`
	Statuses        []CouponStatus     `json:"statuses,omitempty"`
	Priorities      []CouponPriority   `json:"priorities,omitempty"`
	DiscountTypes   []DiscountType     `json:"discountTypes,omitempty"`
	CategoryIDs     []string           `json:"categoryIds,omitempty"`
	ProductIDs      []string           `json:"productIds,omitempty"`
	VendorIDs       []string           `json:"vendorIds,omitempty"`
	UserGroupIDs    []string           `json:"userGroupIds,omitempty"`
	CountryCodes    []string           `json:"countryCodes,omitempty"`
	RegionCodes     []string           `json:"regionCodes,omitempty"`
	IsActive        *bool              `json:"isActive,omitempty"`
	ValidFrom       *time.Time         `json:"validFrom,omitempty"`
	ValidUntil      *time.Time         `json:"validUntil,omitempty"`
	MinDiscountValue *float64          `json:"minDiscountValue,omitempty"`
	MaxDiscountValue *float64          `json:"maxDiscountValue,omitempty"`
	Tags            []string           `json:"tags,omitempty"`
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

// CouponResponse represents a single coupon response
type CouponResponse struct {
	Success bool    `json:"success"`
	Data    *Coupon `json:"data"`
	Message *string `json:"message,omitempty"`
}

// CouponListResponse represents a list of coupons response
type CouponListResponse struct {
	Success    bool            `json:"success"`
	Data       []Coupon        `json:"data"`
	Pagination *PaginationInfo `json:"pagination"`
	Metadata   *JSON           `json:"metadata,omitempty"`
}

// CouponValidationResponse represents a coupon validation response
type CouponValidationResponse struct {
	Success         bool     `json:"success"`
	Valid           bool     `json:"valid"`
	DiscountAmount  *float64 `json:"discountAmount,omitempty"`
	Message         *string  `json:"message,omitempty"`
	ReasonCode      *string  `json:"reasonCode,omitempty"`
	Coupon          *Coupon  `json:"coupon,omitempty"`
}

// CouponUsageResponse represents a coupon usage response
type CouponUsageResponse struct {
	Success       bool         `json:"success"`
	Data          *CouponUsage `json:"data"`
	Message       *string      `json:"message,omitempty"`
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

// TableName returns the table name for the Coupon model
func (Coupon) TableName() string {
	return "coupons"
}

// TableName returns the table name for the CouponUsage model
func (CouponUsage) TableName() string {
	return "coupon_usage"
}