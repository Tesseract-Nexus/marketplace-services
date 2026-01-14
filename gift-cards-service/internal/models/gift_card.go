package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GiftCardStatus represents the status of a gift card
type GiftCardStatus string

const (
	GiftCardStatusActive    GiftCardStatus = "ACTIVE"
	GiftCardStatusRedeemed  GiftCardStatus = "REDEEMED"
	GiftCardStatusExpired   GiftCardStatus = "EXPIRED"
	GiftCardStatusCancelled GiftCardStatus = "CANCELLED"
	GiftCardStatusSuspended GiftCardStatus = "SUSPENDED"
)

// TransactionType represents the type of gift card transaction
type TransactionType string

const (
	TransactionTypeIssue      TransactionType = "ISSUE"      // Initial creation/purchase
	TransactionTypeRedemption TransactionType = "REDEMPTION" // Used in order
	TransactionTypeRefund     TransactionType = "REFUND"     // Refund added back
	TransactionTypeAdjustment TransactionType = "ADJUSTMENT" // Manual adjustment
	TransactionTypeExpiry     TransactionType = "EXPIRY"     // Expired balance removal
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

// GiftCard represents a gift card entity
type GiftCard struct {
	ID                uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID          string          `json:"tenantId" gorm:"type:varchar(255);not null;index"`
	Code              string          `json:"code" gorm:"type:varchar(50);not null;uniqueIndex"`
	InitialBalance    float64         `json:"initialBalance" gorm:"type:decimal(10,2);not null"`
	CurrentBalance    float64         `json:"currentBalance" gorm:"type:decimal(10,2);not null"`
	CurrencyCode      string          `json:"currencyCode" gorm:"type:varchar(3);not null;default:'USD'"`
	Status            GiftCardStatus  `json:"status" gorm:"type:varchar(20);not null;default:'ACTIVE'"`

	// Recipient information
	RecipientEmail    *string         `json:"recipientEmail,omitempty" gorm:"type:varchar(255)"`
	RecipientName     *string         `json:"recipientName,omitempty" gorm:"type:varchar(255)"`
	SenderName        *string         `json:"senderName,omitempty" gorm:"type:varchar(255)"`
	Message           *string         `json:"message,omitempty" gorm:"type:text"`

	// Purchase information
	PurchasedBy       *uuid.UUID      `json:"purchasedBy,omitempty" gorm:"type:uuid;index"`
	PurchaseOrderID   *uuid.UUID      `json:"purchaseOrderId,omitempty" gorm:"type:uuid"`
	PurchaseDate      *time.Time      `json:"purchaseDate,omitempty"`

	// Expiration
	ExpiresAt         *time.Time      `json:"expiresAt,omitempty" gorm:"index"`

	// Usage tracking
	LastUsedAt        *time.Time      `json:"lastUsedAt,omitempty"`
	UsageCount        int             `json:"usageCount" gorm:"default:0"`

	// Metadata
	Metadata          *JSON           `json:"metadata,omitempty" gorm:"type:jsonb"`
	Notes             *string         `json:"notes,omitempty" gorm:"type:text"`

	// Audit fields
	CreatedAt         time.Time       `json:"createdAt"`
	UpdatedAt         time.Time       `json:"updatedAt"`
	DeletedAt         *gorm.DeletedAt `json:"deletedAt,omitempty" gorm:"index"`
	CreatedBy         *string         `json:"createdBy,omitempty"`
	UpdatedBy         *string         `json:"updatedBy,omitempty"`

	// Relations
	Transactions      []GiftCardTransaction `json:"transactions,omitempty" gorm:"foreignKey:GiftCardID"`
}

// GiftCardTransaction represents a transaction on a gift card
type GiftCardTransaction struct {
	ID            uuid.UUID       `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID      string          `json:"tenantId" gorm:"type:varchar(255);not null;index"`
	GiftCardID    uuid.UUID       `json:"giftCardId" gorm:"type:uuid;not null;index"`
	Type          TransactionType `json:"type" gorm:"type:varchar(20);not null"`
	Amount        float64         `json:"amount" gorm:"type:decimal(10,2);not null"`
	BalanceBefore float64         `json:"balanceBefore" gorm:"type:decimal(10,2);not null"`
	BalanceAfter  float64         `json:"balanceAfter" gorm:"type:decimal(10,2);not null"`

	// Related entities
	OrderID       *uuid.UUID      `json:"orderId,omitempty" gorm:"type:uuid;index"`
	UserID        *uuid.UUID      `json:"userId,omitempty" gorm:"type:uuid"`

	// Transaction details
	Description   *string         `json:"description,omitempty"`
	Metadata      *JSON           `json:"metadata,omitempty" gorm:"type:jsonb"`

	// Audit
	CreatedAt     time.Time       `json:"createdAt"`
	CreatedBy     *string         `json:"createdBy,omitempty"`
}

// TableName returns the table name for GiftCard
func (GiftCard) TableName() string {
	return "gift_cards"
}

// TableName returns the table name for GiftCardTransaction
func (GiftCardTransaction) TableName() string {
	return "gift_card_transactions"
}

// Request/Response models

// CreateGiftCardRequest represents a request to create a gift card
type CreateGiftCardRequest struct {
	InitialBalance float64    `json:"initialBalance" binding:"required,gt=0"`
	CurrencyCode   string     `json:"currencyCode,omitempty"`
	RecipientEmail *string    `json:"recipientEmail,omitempty"`
	RecipientName  *string    `json:"recipientName,omitempty"`
	SenderName     *string    `json:"senderName,omitempty"`
	Message        *string    `json:"message,omitempty"`
	ExpiresAt      *time.Time `json:"expiresAt,omitempty"`
	Metadata       *JSON      `json:"metadata,omitempty"`
}

// PurchaseGiftCardRequest represents a request to purchase a gift card
type PurchaseGiftCardRequest struct {
	Amount         float64    `json:"amount" binding:"required,gt=0"`
	Quantity       int        `json:"quantity" binding:"required,gt=0,lte=10"`
	RecipientEmail *string    `json:"recipientEmail,omitempty"`
	RecipientName  *string    `json:"recipientName,omitempty"`
	SenderName     *string    `json:"senderName,omitempty"`
	Message        *string    `json:"message,omitempty"`
	DeliveryDate   *time.Time `json:"deliveryDate,omitempty"`
}

// RedeemGiftCardRequest represents a request to redeem a gift card
type RedeemGiftCardRequest struct {
	Code   string  `json:"code" binding:"required"`
	Amount float64 `json:"amount" binding:"required,gt=0"`
}

// ApplyGiftCardRequest represents a request to apply a gift card to an order
type ApplyGiftCardRequest struct {
	Code    string     `json:"code" binding:"required"`
	OrderID *uuid.UUID `json:"orderId,omitempty"`
}

// CheckBalanceRequest represents a request to check gift card balance
type CheckBalanceRequest struct {
	Code string `json:"code" binding:"required"`
}

// SearchGiftCardsRequest represents search parameters
type SearchGiftCardsRequest struct {
	Query          *string           `json:"query,omitempty"`
	Status         []GiftCardStatus  `json:"status,omitempty"`
	PurchasedBy    *uuid.UUID        `json:"purchasedBy,omitempty"`
	RecipientEmail *string           `json:"recipientEmail,omitempty"`
	MinBalance     *float64          `json:"minBalance,omitempty"`
	MaxBalance     *float64          `json:"maxBalance,omitempty"`
	ExpiringBefore *time.Time        `json:"expiringBefore,omitempty"`
	CreatedFrom    *time.Time        `json:"createdFrom,omitempty"`
	CreatedTo      *time.Time        `json:"createdTo,omitempty"`
	SortBy         *string           `json:"sortBy,omitempty"`
	SortOrder      *string           `json:"sortOrder,omitempty"`
	Page           int               `json:"page"`
	Limit          int               `json:"limit"`
}

// GiftCardStats represents gift card statistics
type GiftCardStats struct {
	TotalCards       int64   `json:"totalCards"`
	ActiveCards      int64   `json:"activeCards"`
	RedeemedCards    int64   `json:"redeemedCards"`
	ExpiredCards     int64   `json:"expiredCards"`
	TotalValue       float64 `json:"totalValue"`
	RedeemedValue    float64 `json:"redeemedValue"`
	RemainingValue   float64 `json:"remainingValue"`
	AverageBalance   float64 `json:"averageBalance"`
	RedemptionRate   float64 `json:"redemptionRate"`
}

// Response models

type GiftCardResponse struct {
	Success bool      `json:"success"`
	Data    *GiftCard `json:"data,omitempty"`
	Message *string   `json:"message,omitempty"`
}

type GiftCardListResponse struct {
	Success    bool             `json:"success"`
	Data       []GiftCard       `json:"data"`
	Pagination *PaginationInfo  `json:"pagination,omitempty"`
}

type GiftCardStatsResponse struct {
	Success bool           `json:"success"`
	Data    *GiftCardStats `json:"data"`
}

type BalanceResponse struct {
	Success bool    `json:"success"`
	Data    *struct {
		Code           string          `json:"code"`
		Balance        float64         `json:"balance"`
		CurrencyCode   string          `json:"currencyCode"`
		Status         GiftCardStatus  `json:"status"`
		ExpiresAt      *time.Time      `json:"expiresAt,omitempty"`
	} `json:"data"`
	Message *string `json:"message,omitempty"`
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
