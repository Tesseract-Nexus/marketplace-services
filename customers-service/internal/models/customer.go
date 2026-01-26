package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// CustomerStatus represents customer status
type CustomerStatus string

const (
	CustomerStatusActive   CustomerStatus = "ACTIVE"
	CustomerStatusInactive CustomerStatus = "INACTIVE"
	CustomerStatusBlocked  CustomerStatus = "BLOCKED"
)

// CustomerType represents customer type
type CustomerType string

const (
	CustomerTypeRetail    CustomerType = "RETAIL"
	CustomerTypeWholesale CustomerType = "WHOLESALE"
	CustomerTypeVIP       CustomerType = "VIP"
)

// Customer represents the main customer entity
// Performance indexes: Composite indexes on tenant_id with frequently filtered columns
// for 5-20x query improvement on multi-tenant lookups and list queries
type Customer struct {
	ID         uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID   string         `json:"tenantId" gorm:"type:varchar(255);not null;index:idx_customers_tenant_id;index:idx_customers_tenant_email,unique;index:idx_customers_tenant_user;index:idx_customers_tenant_status;index:idx_customers_tenant_created"`
	UserID     *uuid.UUID     `json:"userId" gorm:"type:uuid;index;index:idx_customers_tenant_user"` // NULL for guest customers
	Email      string         `json:"email" gorm:"type:varchar(255);not null;index:idx_customers_tenant_email,unique"`
	FirstName  string         `json:"firstName" gorm:"type:varchar(100);not null"`
	LastName   string         `json:"lastName" gorm:"type:varchar(100);not null"`
	Phone      string         `json:"phone" gorm:"type:varchar(50)"`
	Status       CustomerStatus `json:"status" gorm:"type:varchar(20);default:'ACTIVE';index:idx_customers_tenant_status"`
	CustomerType CustomerType   `json:"customerType" gorm:"type:varchar(20);default:'RETAIL'"`

	// Lock/Unlock metadata
	LockReason   string     `json:"lockReason,omitempty" gorm:"type:text"`
	LockedAt     *time.Time `json:"lockedAt,omitempty"`
	LockedBy     *uuid.UUID `json:"lockedBy,omitempty" gorm:"type:uuid"`
	UnlockReason string     `json:"unlockReason,omitempty" gorm:"type:text"`
	UnlockedAt   *time.Time `json:"unlockedAt,omitempty"`
	UnlockedBy   *uuid.UUID `json:"unlockedBy,omitempty" gorm:"type:uuid"`

	// Analytics fields
	TotalOrders        int       `json:"totalOrders" gorm:"default:0"`
	TotalSpent         float64   `json:"totalSpent" gorm:"type:decimal(12,2);default:0"`
	AverageOrderValue  float64   `json:"averageOrderValue" gorm:"type:decimal(10,2);default:0"`
	LifetimeValue      float64   `json:"lifetimeValue" gorm:"type:decimal(12,2);default:0"`
	LastOrderDate      *time.Time `json:"lastOrderDate"`
	FirstOrderDate     *time.Time `json:"firstOrderDate"`

	// Location/Region
	Country     string `json:"country" gorm:"type:varchar(100)"` // Full country name (e.g., "Australia")
	CountryCode string `json:"countryCode" gorm:"type:varchar(2);index:idx_customers_tenant_country"` // ISO 2-letter code (e.g., "AU")

	// Engagement
	Tags             pq.StringArray `json:"tags" gorm:"type:text[]"`
	Notes            string         `json:"notes" gorm:"type:text"`
	MarketingOptIn   bool           `json:"marketingOptIn" gorm:"default:false"`

	// Email Verification
	EmailVerified              bool       `json:"emailVerified" gorm:"default:false"`
	VerificationToken          string     `json:"-" gorm:"type:varchar(255)"`
	VerificationTokenExpiresAt *time.Time `json:"-"`

	// Metadata
	CreatedAt time.Time      `json:"createdAt" gorm:"index:idx_customers_tenant_created,sort:desc"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Relationships (not stored in DB, loaded via joins)
	Addresses      []CustomerAddress      `json:"addresses,omitempty" gorm:"foreignKey:CustomerID;constraint:OnDelete:CASCADE"`
	PaymentMethods []CustomerPaymentMethod `json:"paymentMethods,omitempty" gorm:"foreignKey:CustomerID;constraint:OnDelete:CASCADE"`
	Segments       []CustomerSegment      `json:"segments,omitempty" gorm:"many2many:customer_segment_members"`
}

// AddressType represents address type
type AddressType string

const (
	AddressTypeShipping AddressType = "SHIPPING"
	AddressTypeBilling  AddressType = "BILLING"
	AddressTypeBoth     AddressType = "BOTH"
)

// CustomerAddress represents a customer address
// Note: Column names use GORM's default snake_case conversion (address_line1, not address_line_1)
type CustomerAddress struct {
	ID          uuid.UUID   `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	CustomerID  uuid.UUID   `json:"customerId" gorm:"type:uuid;not null"`
	TenantID    string      `json:"tenantId" gorm:"type:varchar(255);not null"`
	AddressType AddressType `json:"addressType" gorm:"type:varchar(20);default:'SHIPPING'"`
	IsDefault   bool        `json:"isDefault" gorm:"default:false"`
	Label       string      `json:"label" gorm:"type:varchar(50)"` // User-defined label (e.g., "Home", "Work")

	// Address fields
	FirstName    string `json:"firstName" gorm:"type:varchar(100)"`
	LastName     string `json:"lastName" gorm:"type:varchar(100)"`
	Company      string `json:"company" gorm:"type:varchar(255)"`
	AddressLine1 string `json:"addressLine1" gorm:"type:varchar(255);not null"`
	AddressLine2 string `json:"addressLine2" gorm:"type:varchar(255)"`
	City         string `json:"city" gorm:"type:varchar(100);not null"`
	State        string `json:"state" gorm:"type:varchar(100)"`
	PostalCode   string `json:"postalCode" gorm:"type:varchar(20);not null"`
	Country      string `json:"country" gorm:"type:varchar(2);not null"` // ISO 2-letter code
	Phone        string `json:"phone" gorm:"type:varchar(50)"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// PaymentType represents payment method type
type PaymentType string

const (
	PaymentTypeCard        PaymentType = "card"
	PaymentTypeBankAccount PaymentType = "bank_account"
	PaymentTypePayPal      PaymentType = "paypal"
	PaymentTypeUPI         PaymentType = "upi"
)

// CustomerPaymentMethod represents a saved payment method
type CustomerPaymentMethod struct {
	ID                      uuid.UUID   `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	CustomerID              uuid.UUID   `json:"customerId" gorm:"type:uuid;not null"`
	TenantID                string      `json:"tenantId" gorm:"type:varchar(255);not null"`
	PaymentGateway          string      `json:"paymentGateway" gorm:"type:varchar(50);not null"` // stripe, paypal
	GatewayPaymentMethodID  string      `json:"gatewayPaymentMethodId" gorm:"type:varchar(255);not null"`
	PaymentType             PaymentType `json:"paymentType" gorm:"type:varchar(20);not null"`
	CardBrand               string      `json:"cardBrand" gorm:"type:varchar(20)"`
	LastFour                string      `json:"lastFour" gorm:"type:varchar(4)"`
	ExpiryMonth             int         `json:"expiryMonth"`
	ExpiryYear              int         `json:"expiryYear"`
	IsDefault               bool        `json:"isDefault" gorm:"default:false"`
	IsActive                bool        `json:"isActive" gorm:"default:true"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// JSONB is a custom type for PostgreSQL JSONB fields
type JSONB json.RawMessage

// Value implements the driver.Valuer interface for JSONB
func (j JSONB) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return []byte(j), nil
}

// Scan implements the sql.Scanner interface for JSONB
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	switch v := value.(type) {
	case []byte:
		*j = JSONB(v)
		return nil
	case string:
		*j = JSONB([]byte(v))
		return nil
	default:
		return nil
	}
}

// MarshalJSON implements json.Marshaler
func (j JSONB) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return []byte(j), nil
}

// UnmarshalJSON implements json.Unmarshaler
func (j *JSONB) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		*j = nil
		return nil
	}
	*j = JSONB(data)
	return nil
}

// CustomerSegment represents a customer segment/group
type CustomerSegment struct {
	ID            uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID      string    `json:"tenantId" gorm:"type:varchar(255);not null"`
	Name          string    `json:"name" gorm:"type:varchar(100);not null"`
	Description   string    `json:"description" gorm:"type:text"`
	Rules         JSONB     `json:"rules" gorm:"type:jsonb"`
	IsDynamic     bool      `json:"isDynamic" gorm:"default:false"`
	IsActive      bool      `json:"isActive" gorm:"default:true"`
	CustomerCount int       `json:"customerCount" gorm:"default:0"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// CustomerNote represents a note/comment on a customer
type CustomerNote struct {
	ID         uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	CustomerID uuid.UUID `json:"customerId" gorm:"type:uuid;not null"`
	TenantID   string    `json:"tenantId" gorm:"type:varchar(255);not null"`
	Note       string    `json:"note" gorm:"type:text;not null"`
	CreatedBy  *uuid.UUID `json:"createdBy" gorm:"type:uuid"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// CommunicationType represents communication channel
type CommunicationType string

const (
	CommunicationTypeEmail CommunicationType = "email"
	CommunicationTypeSMS   CommunicationType = "sms"
	CommunicationTypeCall  CommunicationType = "call"
	CommunicationTypeChat  CommunicationType = "chat"
)

// CommunicationDirection represents direction of communication
type CommunicationDirection string

const (
	CommunicationDirectionInbound  CommunicationDirection = "inbound"
	CommunicationDirectionOutbound CommunicationDirection = "outbound"
)

// CommunicationStatus represents status of communication
type CommunicationStatus string

const (
	CommunicationStatusSent      CommunicationStatus = "sent"
	CommunicationStatusDelivered CommunicationStatus = "delivered"
	CommunicationStatusFailed    CommunicationStatus = "failed"
	CommunicationStatusOpened    CommunicationStatus = "opened"
	CommunicationStatusClicked   CommunicationStatus = "clicked"
)

// CustomerCommunication represents communication history
type CustomerCommunication struct {
	ID                uuid.UUID              `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	CustomerID        uuid.UUID              `json:"customerId" gorm:"type:uuid;not null"`
	TenantID          string                 `json:"tenantId" gorm:"type:varchar(255);not null"`
	CommunicationType CommunicationType      `json:"communicationType" gorm:"type:varchar(50);not null"`
	Direction         CommunicationDirection `json:"direction" gorm:"type:varchar(20);not null"`
	Subject           string                 `json:"subject" gorm:"type:varchar(255)"`
	Content           string                 `json:"content" gorm:"type:text"`
	Status            CommunicationStatus    `json:"status" gorm:"type:varchar(20);default:'sent'"`
	ExternalID        string                 `json:"externalId" gorm:"type:varchar(255)"`

	CreatedAt time.Time `json:"createdAt"`
}

// CustomerWishlistItem represents an item in customer's wishlist
type CustomerWishlistItem struct {
	ID         uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	CustomerID uuid.UUID  `json:"customerId" gorm:"type:uuid;not null;index"`
	TenantID   string     `json:"tenantId" gorm:"type:varchar(255);not null;index"`
	ProductID  string     `json:"productId" gorm:"type:varchar(255);not null"`
	ProductName string    `json:"productName" gorm:"type:varchar(255)"`
	ProductPrice float64  `json:"productPrice" gorm:"type:decimal(10,2)"`
	ProductImage string   `json:"productImage" gorm:"type:varchar(500)"`
	AddedAt    time.Time  `json:"addedAt" gorm:"default:CURRENT_TIMESTAMP"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

// CustomerCart represents a customer's shopping cart
type CustomerCart struct {
	ID                  uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	CustomerID          uuid.UUID  `json:"customerId" gorm:"type:uuid;not null;uniqueIndex:idx_customer_tenant_cart"`
	TenantID            string     `json:"tenantId" gorm:"type:varchar(255);not null;uniqueIndex:idx_customer_tenant_cart"`
	Items               JSONB      `json:"items" gorm:"type:jsonb;default:'[]'"`
	Subtotal            float64    `json:"subtotal" gorm:"type:decimal(12,2);default:0"`
	ItemCount           int        `json:"itemCount" gorm:"default:0"`
	LastItemChange      time.Time  `json:"lastItemChange" gorm:"default:CURRENT_TIMESTAMP"` // When items were last added/removed/modified
	ExpiresAt           *time.Time `json:"expiresAt" gorm:"type:timestamp"`                 // Cart expiration (90 days)
	LastValidatedAt     *time.Time `json:"lastValidatedAt" gorm:"type:timestamp"`           // Last product validation check
	HasUnavailableItems bool       `json:"hasUnavailableItems" gorm:"default:false"`        // Has items that are unavailable
	HasPriceChanges     bool       `json:"hasPriceChanges" gorm:"default:false"`            // Has items with price changes
	UnavailableCount    int        `json:"unavailableCount" gorm:"default:0"`               // Count of unavailable items
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

// CartItemStatus represents the availability status of a cart item
type CartItemStatus string

const (
	CartItemStatusAvailable    CartItemStatus = "AVAILABLE"
	CartItemStatusUnavailable  CartItemStatus = "UNAVAILABLE"  // Product deleted or unpublished
	CartItemStatusOutOfStock   CartItemStatus = "OUT_OF_STOCK" // Product exists but no inventory
	CartItemStatusLowStock     CartItemStatus = "LOW_STOCK"    // Quantity > available stock
	CartItemStatusPriceChanged CartItemStatus = "PRICE_CHANGED" // Price has changed since added
)

// CartItem represents an individual item in the cart (stored as JSONB)
type CartItem struct {
	ID              string         `json:"id"`
	ProductID       string         `json:"productId"`
	VariantID       string         `json:"variantId,omitempty"`
	Name            string         `json:"name"`
	Price           float64        `json:"price"`           // Current price (may differ from priceAtAdd)
	PriceAtAdd      float64        `json:"priceAtAdd"`      // Price when item was added to cart
	Quantity        int            `json:"quantity"`
	Image           string         `json:"image,omitempty"`
	Status          CartItemStatus `json:"status"`          // Availability status
	AvailableStock  int            `json:"availableStock"`  // Current available stock
	AddedAt         *time.Time     `json:"addedAt"`         // When item was added (for 90-day per-item expiration)
	LastValidatedAt *time.Time     `json:"lastValidatedAt"` // When item was last validated
}
