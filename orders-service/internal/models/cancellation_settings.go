package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CancellationSettings stores tenant-level cancellation policy configuration
// This model is the authoritative source for cancellation workflows across the orders system
type CancellationSettings struct {
	ID         uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID   string         `json:"tenantId" gorm:"type:varchar(255);not null;uniqueIndex:idx_cancellation_settings_tenant_storefront"`
	VendorID   string         `json:"vendorId,omitempty" gorm:"type:varchar(255);index:idx_cancellation_settings_vendor"`
	StorefrontID string       `json:"storefrontId,omitempty" gorm:"type:varchar(255);uniqueIndex:idx_cancellation_settings_tenant_storefront"`

	// Basic cancellation policy
	Enabled                   bool    `json:"enabled" gorm:"default:true"`
	RequireReason             bool    `json:"requireReason" gorm:"default:true"`
	AllowPartialCancellation  bool    `json:"allowPartialCancellation" gorm:"default:false"`

	// Fee configuration
	DefaultFeeType   string  `json:"defaultFeeType" gorm:"type:varchar(20);default:'percentage'"` // 'percentage' or 'fixed'
	DefaultFeeValue  float64 `json:"defaultFeeValue" gorm:"default:15"`

	// Refund configuration
	RefundMethod     string  `json:"refundMethod" gorm:"type:varchar(50);default:'original_payment'"` // 'original_payment', 'store_credit', 'either'
	AutoRefundEnabled bool   `json:"autoRefundEnabled" gorm:"default:true"`

	// Status-based restrictions (JSONB array of status strings)
	NonCancellableStatuses StatusList `json:"nonCancellableStatuses" gorm:"type:jsonb;default:'[\"SHIPPED\",\"DELIVERED\"]'"`

	// Time-based cancellation windows (JSONB)
	CancellationWindows CancellationWindowList `json:"windows" gorm:"type:jsonb;default:'[]'"`

	// Customer-facing cancellation reasons (JSONB array of strings)
	CancellationReasons StringList `json:"cancellationReasons" gorm:"type:jsonb;default:'[]'"`

	// Approval workflow
	RequireApprovalForPolicyChanges bool `json:"requireApprovalForPolicyChanges" gorm:"default:false"`

	// Customer-facing policy text (can contain HTML)
	PolicyText string `json:"policyText" gorm:"type:text"`

	// Audit fields
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
	CreatedBy string         `json:"createdBy,omitempty" gorm:"type:varchar(255)"`
	UpdatedBy string         `json:"updatedBy,omitempty" gorm:"type:varchar(255)"`
}

func (CancellationSettings) TableName() string {
	return "cancellation_settings"
}

// CancellationWindow defines a time-based cancellation fee window
type CancellationWindow struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	MaxHoursAfterOrder int     `json:"maxHoursAfterOrder"`
	FeeType            string  `json:"feeType"` // 'percentage' or 'fixed'
	FeeValue           float64 `json:"feeValue"`
	Description        string  `json:"description,omitempty"`
}

// CancellationWindowList is a custom type for JSONB storage of cancellation windows
type CancellationWindowList []CancellationWindow

// Value implements driver.Valuer for JSONB storage
func (c CancellationWindowList) Value() (driver.Value, error) {
	if c == nil {
		return json.Marshal([]CancellationWindow{})
	}
	return json.Marshal(c)
}

// Scan implements sql.Scanner for JSONB retrieval
func (c *CancellationWindowList) Scan(value interface{}) error {
	if value == nil {
		*c = []CancellationWindow{}
		return nil
	}
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, c)
	case string:
		return json.Unmarshal([]byte(v), c)
	}
	return nil
}

// StringList is a custom type for JSONB storage of string arrays
type StringList []string

// Value implements driver.Valuer for JSONB storage
func (s StringList) Value() (driver.Value, error) {
	if s == nil {
		return json.Marshal([]string{})
	}
	return json.Marshal(s)
}

// Scan implements sql.Scanner for JSONB retrieval
func (s *StringList) Scan(value interface{}) error {
	if value == nil {
		*s = []string{}
		return nil
	}
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, s)
	case string:
		return json.Unmarshal([]byte(v), s)
	}
	return nil
}

// StatusList is a custom type for JSONB storage of status strings
type StatusList []string

// Value implements driver.Valuer for JSONB storage
func (s StatusList) Value() (driver.Value, error) {
	if s == nil {
		return json.Marshal([]string{})
	}
	return json.Marshal(s)
}

// Scan implements sql.Scanner for JSONB retrieval
func (s *StatusList) Scan(value interface{}) error {
	if value == nil {
		*s = []string{}
		return nil
	}
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, s)
	case string:
		return json.Unmarshal([]byte(v), s)
	}
	return nil
}

// CreateCancellationSettingsRequest is the request body for creating cancellation settings
type CreateCancellationSettingsRequest struct {
	Enabled                   *bool              `json:"enabled,omitempty"`
	RequireReason             *bool              `json:"requireReason,omitempty"`
	AllowPartialCancellation  *bool              `json:"allowPartialCancellation,omitempty"`
	DefaultFeeType            string             `json:"defaultFeeType,omitempty"`
	DefaultFeeValue           *float64           `json:"defaultFeeValue,omitempty"`
	RefundMethod              string             `json:"refundMethod,omitempty"`
	AutoRefundEnabled         *bool              `json:"autoRefundEnabled,omitempty"`
	NonCancellableStatuses    []string           `json:"nonCancellableStatuses,omitempty"`
	CancellationWindows       []CancellationWindow `json:"windows,omitempty"`
	CancellationReasons       []string           `json:"cancellationReasons,omitempty"`
	RequireApprovalForPolicyChanges *bool        `json:"requireApprovalForPolicyChanges,omitempty"`
	PolicyText                string             `json:"policyText,omitempty"`
}

// UpdateCancellationSettingsRequest is the request body for updating cancellation settings
type UpdateCancellationSettingsRequest struct {
	Enabled                   *bool              `json:"enabled,omitempty"`
	RequireReason             *bool              `json:"requireReason,omitempty"`
	AllowPartialCancellation  *bool              `json:"allowPartialCancellation,omitempty"`
	DefaultFeeType            string             `json:"defaultFeeType,omitempty"`
	DefaultFeeValue           *float64           `json:"defaultFeeValue,omitempty"`
	RefundMethod              string             `json:"refundMethod,omitempty"`
	AutoRefundEnabled         *bool              `json:"autoRefundEnabled,omitempty"`
	NonCancellableStatuses    []string           `json:"nonCancellableStatuses,omitempty"`
	CancellationWindows       []CancellationWindow `json:"windows,omitempty"`
	CancellationReasons       []string           `json:"cancellationReasons,omitempty"`
	RequireApprovalForPolicyChanges *bool        `json:"requireApprovalForPolicyChanges,omitempty"`
	PolicyText                string             `json:"policyText,omitempty"`
}

// CancellationSettingsResponse is the API response wrapper
type CancellationSettingsResponse struct {
	Success bool                  `json:"success"`
	Data    *CancellationSettings `json:"data,omitempty"`
	Message string                `json:"message,omitempty"`
	Error   string                `json:"error,omitempty"`
}

// DefaultCancellationSettings returns the default settings for a new tenant
func DefaultCancellationSettings(tenantID, storefrontID string) *CancellationSettings {
	return &CancellationSettings{
		ID:           uuid.New(),
		TenantID:     tenantID,
		StorefrontID: storefrontID,
		Enabled:      true,
		RequireReason: true,
		AllowPartialCancellation: false,
		DefaultFeeType: "percentage",
		DefaultFeeValue: 15,
		RefundMethod: "original_payment",
		AutoRefundEnabled: true,
		NonCancellableStatuses: StatusList{"SHIPPED", "DELIVERED"},
		CancellationWindows: CancellationWindowList{
			{ID: uuid.New().String(), Name: "Free cancellation", MaxHoursAfterOrder: 6, FeeType: "percentage", FeeValue: 0, Description: "Cancel within 6 hours at no charge."},
			{ID: uuid.New().String(), Name: "Low fee", MaxHoursAfterOrder: 24, FeeType: "percentage", FeeValue: 3, Description: "A small processing fee applies within 24 hours."},
			{ID: uuid.New().String(), Name: "Pre-delivery", MaxHoursAfterOrder: 72, FeeType: "percentage", FeeValue: 10, Description: "10% fee for cancellations before delivery."},
		},
		CancellationReasons: StringList{
			"I changed my mind",
			"Found a better price elsewhere",
			"Ordered by mistake",
			"Shipping is taking too long",
			"Payment issue",
			"Other reason",
		},
		RequireApprovalForPolicyChanges: false,
		PolicyText: "",
	}
}
