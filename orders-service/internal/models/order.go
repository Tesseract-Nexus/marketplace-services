package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

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

// OrderStatus represents the overall lifecycle status of an order
type OrderStatus string

const (
	OrderStatusPlaced     OrderStatus = "PLACED"     // Order created, awaiting payment
	OrderStatusConfirmed  OrderStatus = "CONFIRMED"  // Payment successful, order accepted
	OrderStatusProcessing OrderStatus = "PROCESSING" // Being fulfilled/packed
	OrderStatusShipped    OrderStatus = "SHIPPED"    // Dispatched to carrier
	OrderStatusDelivered  OrderStatus = "DELIVERED"  // Successfully delivered
	OrderStatusCompleted  OrderStatus = "COMPLETED"  // Fully delivered (alias for DELIVERED)
	OrderStatusCancelled  OrderStatus = "CANCELLED"  // Cancelled before fulfillment
)

// PaymentStatus represents the payment/money flow status
type PaymentStatus string

const (
	PaymentStatusPending           PaymentStatus = "PENDING"            // Awaiting payment
	PaymentStatusPaid              PaymentStatus = "PAID"               // Payment received
	PaymentStatusFailed            PaymentStatus = "FAILED"             // Payment failed
	PaymentStatusPartiallyRefunded PaymentStatus = "PARTIALLY_REFUNDED" // Partial refund issued
	PaymentStatusRefunded          PaymentStatus = "REFUNDED"           // Fully refunded
)

// FulfillmentStatus represents the physical delivery/fulfillment status
type FulfillmentStatus string

const (
	FulfillmentStatusUnfulfilled    FulfillmentStatus = "UNFULFILLED"      // Not yet picked
	FulfillmentStatusProcessing     FulfillmentStatus = "PROCESSING"       // Being picked/packed
	FulfillmentStatusPacked         FulfillmentStatus = "PACKED"           // Ready for dispatch
	FulfillmentStatusDispatched     FulfillmentStatus = "DISPATCHED"       // Handed to carrier
	FulfillmentStatusInTransit      FulfillmentStatus = "IN_TRANSIT"       // With carrier, en route
	FulfillmentStatusOutForDelivery FulfillmentStatus = "OUT_FOR_DELIVERY" // Last mile delivery
	FulfillmentStatusDelivered      FulfillmentStatus = "DELIVERED"        // Successfully delivered
	FulfillmentStatusFailedDelivery FulfillmentStatus = "FAILED_DELIVERY"  // Delivery attempt failed
	FulfillmentStatusReturned       FulfillmentStatus = "RETURNED"         // Returned to warehouse
)

// Order represents the main order entity
// Performance indexes: Composite indexes on tenant_id with frequently filtered columns
// for 10-30x query improvement on multi-tenant list/filter queries
// Vendor isolation: vendor_id for marketplace mode (Tenant -> Vendor -> Staff hierarchy)
type Order struct {
	ID                uuid.UUID         `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID          string            `json:"tenantId" gorm:"type:varchar(255);not null;index:idx_orders_tenant_id;index:idx_orders_tenant_status;index:idx_orders_tenant_customer;index:idx_orders_tenant_created;index:idx_orders_tenant_order_number,unique"`
	VendorID          string            `json:"vendorId,omitempty" gorm:"type:varchar(255);index:idx_orders_tenant_vendor;index:idx_orders_vendor_status;index:idx_orders_vendor_created"` // Vendor ID for marketplace isolation
	OrderNumber       string            `json:"orderNumber" gorm:"not null;index:idx_orders_tenant_order_number,unique"`
	CustomerID        uuid.UUID         `json:"customerId" gorm:"type:uuid;not null;index:idx_orders_tenant_customer"`
	Status            OrderStatus       `json:"status" gorm:"type:varchar(20);not null;default:'PLACED';index:idx_orders_tenant_status"`
	PaymentStatus     PaymentStatus     `json:"paymentStatus" gorm:"type:varchar(30);not null;default:'PENDING'"`
	FulfillmentStatus FulfillmentStatus `json:"fulfillmentStatus" gorm:"type:varchar(30);not null;default:'UNFULFILLED'"`
	Currency          string            `json:"currency" gorm:"type:varchar(3);not null;default:'USD'"`
	Subtotal          float64           `json:"subtotal" gorm:"type:decimal(10,2);not null"`
	TaxAmount         float64           `json:"taxAmount" gorm:"type:decimal(10,2);default:0"`
	ShippingCost      float64           `json:"shippingCost" gorm:"type:decimal(10,2);default:0"`
	DiscountAmount    float64           `json:"discountAmount" gorm:"type:decimal(10,2);default:0"`
	Total             float64           `json:"total" gorm:"type:decimal(10,2);not null"`
	Notes             string            `json:"notes" gorm:"type:text"`
	CreatedAt         time.Time         `json:"createdAt" gorm:"index:idx_orders_tenant_created,sort:desc"`
	UpdatedAt         time.Time         `json:"updatedAt"`
	DeletedAt         gorm.DeletedAt    `json:"-" gorm:"index"`

	// Order splitting fields
	ParentOrderID     *uuid.UUID        `json:"parentOrderId,omitempty" gorm:"type:uuid;index"`
	IsSplit           bool              `json:"isSplit" gorm:"default:false"`
	SplitReason       string            `json:"splitReason,omitempty" gorm:"type:varchar(50)"`

	// Tax breakdown (stored as JSONB for flexibility across different tax systems)
	TaxBreakdown JSONB `json:"taxBreakdown,omitempty" gorm:"type:jsonb"`

	// India GST specific fields
	CGST         float64 `json:"cgst,omitempty" gorm:"type:decimal(10,2);default:0"`         // Central GST amount
	SGST         float64 `json:"sgst,omitempty" gorm:"type:decimal(10,2);default:0"`         // State GST amount
	IGST         float64 `json:"igst,omitempty" gorm:"type:decimal(10,2);default:0"`         // Integrated GST (interstate)
	UTGST        float64 `json:"utgst,omitempty" gorm:"type:decimal(10,2);default:0"`        // Union Territory GST
	GSTCess      float64 `json:"gstCess,omitempty" gorm:"type:decimal(10,2);default:0"`      // GST Cess (luxury goods)
	IsInterstate bool    `json:"isInterstate,omitempty" gorm:"default:false"`                // True if interstate transaction
	CustomerGSTIN string `json:"customerGstin,omitempty" gorm:"type:varchar(15)"`            // Customer's GSTIN for B2B invoices

	// EU VAT specific fields
	VATAmount       float64 `json:"vatAmount,omitempty" gorm:"type:decimal(10,2);default:0"`
	IsReverseCharge bool    `json:"isReverseCharge,omitempty" gorm:"default:false"`            // EU B2B reverse charge
	CustomerVATNumber string `json:"customerVatNumber,omitempty" gorm:"type:varchar(50)"`     // Customer's VAT number

	// Idempotency key for duplicate order prevention (nullable, unique per tenant)
	IdempotencyKey *string `json:"idempotencyKey,omitempty" gorm:"type:varchar(255);index:idx_orders_tenant_idempotency_key,unique"`

	// Storefront host for building email URLs (custom domain or default subdomain)
	StorefrontHost string `json:"storefrontHost,omitempty" gorm:"type:varchar(255)"`

	// Receipt/Invoice tracking
	ReceiptNumber      string     `json:"receiptNumber,omitempty" gorm:"type:varchar(50);index:idx_orders_receipt_number"`
	InvoiceNumber      string     `json:"invoiceNumber,omitempty" gorm:"type:varchar(50);index:idx_orders_invoice_number"`
	ReceiptDocumentID  *uuid.UUID `json:"receiptDocumentId,omitempty" gorm:"type:uuid"`       // Reference to receipt_documents table
	ReceiptShortURL    string     `json:"receiptShortUrl,omitempty" gorm:"type:varchar(255)"` // Short URL for receipt download
	ReceiptGeneratedAt *time.Time `json:"receiptGeneratedAt,omitempty"`                       // When receipt was generated

	// Relationships
	Items     []OrderItem     `json:"items" gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE"`
	Customer  *OrderCustomer  `json:"customer" gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE"`
	Shipping  *OrderShipping  `json:"shipping" gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE"`
	Payment   *OrderPayment   `json:"payment" gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE"`
	Timeline  []OrderTimeline `json:"timeline" gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE"`
	Discounts []OrderDiscount `json:"discounts" gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE"`
}

// OrderItem represents an item in an order
type OrderItem struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	OrderID     uuid.UUID `json:"orderId" gorm:"type:uuid;not null"`
	VendorID    string    `json:"vendorId,omitempty" gorm:"type:varchar(255);index:idx_order_items_vendor"` // Vendor that fulfills this item
	ProductID   uuid.UUID `json:"productId" gorm:"type:uuid;not null"`
	ProductName string    `json:"productName" gorm:"not null"`
	SKU         string    `json:"sku" gorm:"not null"`
	Image       string    `json:"image,omitempty" gorm:"type:varchar(500)"` // Product image URL
	Quantity    int       `json:"quantity" gorm:"not null"`
	UnitPrice   float64   `json:"unitPrice" gorm:"type:decimal(10,2);not null"`
	TotalPrice  float64   `json:"totalPrice" gorm:"type:decimal(10,2);not null"`

	// Tax fields
	TaxAmount   float64 `json:"taxAmount" gorm:"type:decimal(10,2);default:0"`
	TaxRate     float64 `json:"taxRate" gorm:"type:decimal(5,2);default:0"`         // Tax rate percentage

	// India GST fields
	HSNCode     string  `json:"hsnCode,omitempty" gorm:"type:varchar(10)"`          // Harmonized System of Nomenclature (goods)
	SACCode     string  `json:"sacCode,omitempty" gorm:"type:varchar(10)"`          // Services Accounting Code
	GSTSlab     float64 `json:"gstSlab,omitempty" gorm:"type:decimal(5,2)"`         // GST slab (0, 5, 12, 18, 28)
	CGSTAmount  float64 `json:"cgstAmount,omitempty" gorm:"type:decimal(10,2);default:0"`
	SGSTAmount  float64 `json:"sgstAmount,omitempty" gorm:"type:decimal(10,2);default:0"`
	IGSTAmount  float64 `json:"igstAmount,omitempty" gorm:"type:decimal(10,2);default:0"`

	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// OrderCustomer represents customer information for an order
type OrderCustomer struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	OrderID   uuid.UUID `json:"orderId" gorm:"type:uuid;not null;unique"`
	FirstName string    `json:"firstName" gorm:"not null"`
	LastName  string    `json:"lastName" gorm:"not null"`
	Email     string    `json:"email" gorm:"not null"`
	Phone     string    `json:"phone"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// OrderShipping represents shipping information for an order
type OrderShipping struct {
	ID             uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	OrderID        uuid.UUID `json:"orderId" gorm:"type:uuid;not null;unique"`
	Method         string    `json:"method" gorm:"not null"`
	Carrier        string    `json:"carrier"`
	CourierServiceCode string `json:"courierServiceCode"` // Carrier-specific courier ID (e.g., Shiprocket courier_company_id)
	TrackingNumber string    `json:"trackingNumber"`
	Cost           float64   `json:"cost" gorm:"type:decimal(10,2);not null"`

	// Package dimensions for shipping (captured at checkout for accurate shipping)
	PackageWeight float64 `json:"packageWeight" gorm:"type:decimal(10,3);default:0"` // Weight in kg
	PackageLength float64 `json:"packageLength" gorm:"type:decimal(10,2);default:0"` // Length in cm
	PackageWidth  float64 `json:"packageWidth" gorm:"type:decimal(10,2);default:0"`  // Width in cm
	PackageHeight float64 `json:"packageHeight" gorm:"type:decimal(10,2);default:0"` // Height in cm

	// Shipping rate breakdown (for transparency in admin)
	BaseRate      float64 `json:"baseRate" gorm:"type:decimal(10,2);default:0"`      // Original carrier rate before markup
	MarkupAmount  float64 `json:"markupAmount" gorm:"type:decimal(10,2);default:0"`  // Markup amount applied
	MarkupPercent float64 `json:"markupPercent" gorm:"type:decimal(5,2);default:0"`  // Markup percentage applied (e.g., 10 for 10%)

	// Address fields
	Street      string `json:"street" gorm:"not null"`
	City        string `json:"city" gorm:"not null"`
	State       string `json:"state" gorm:"not null"`
	StateCode   string `json:"stateCode"`              // State code for tax determination (MH, KA, etc.)
	PostalCode  string `json:"postalCode" gorm:"not null"`
	Country     string `json:"country" gorm:"not null"`
	CountryCode string `json:"countryCode"`            // ISO 3166-1 alpha-2 (IN, US, GB, etc.)

	EstimatedDelivery *time.Time `json:"estimatedDelivery"`
	ActualDelivery    *time.Time `json:"actualDelivery"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

// OrderPayment represents payment information for an order
type OrderPayment struct {
	ID            uuid.UUID     `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	OrderID       uuid.UUID     `json:"orderId" gorm:"type:uuid;not null;unique"`
	Method        string        `json:"method" gorm:"not null"`
	Status        PaymentStatus `json:"status" gorm:"type:varchar(20);not null;default:'PENDING'"`
	Amount        float64       `json:"amount" gorm:"type:decimal(10,2);not null"`
	Currency      string        `json:"currency" gorm:"type:varchar(3);not null;default:'USD'"`
	TransactionID string        `json:"transactionId"`
	ProcessedAt   *time.Time    `json:"processedAt"`
	CreatedAt     time.Time     `json:"createdAt"`
	UpdatedAt     time.Time     `json:"updatedAt"`
}

// OrderTimeline represents timeline events for an order
type OrderTimeline struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	OrderID     uuid.UUID `json:"orderId" gorm:"type:uuid;not null"`
	Event       string    `json:"event" gorm:"not null"`
	Description string    `json:"description" gorm:"not null"`
	Timestamp   time.Time `json:"timestamp" gorm:"not null"`
	CreatedBy   string    `json:"createdBy"`
	CreatedAt   time.Time `json:"createdAt"`
}

// OrderDiscount represents discounts applied to an order
type OrderDiscount struct {
	ID           uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	OrderID      uuid.UUID  `json:"orderId" gorm:"type:uuid;not null"`
	CouponID     *uuid.UUID `json:"couponId" gorm:"type:uuid"`
	CouponCode   string     `json:"couponCode"`
	DiscountType string     `json:"discountType" gorm:"not null"` // percentage, fixed
	Amount       float64    `json:"amount" gorm:"type:decimal(10,2);not null"`
	Description  string     `json:"description"`
	CreatedAt    time.Time  `json:"createdAt"`
}

// OrderRefund represents refund information for an order
type OrderRefund struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	OrderID     uuid.UUID  `json:"orderId" gorm:"type:uuid;not null;index:idx_order_refunds_order"`
	Amount      float64    `json:"amount" gorm:"type:decimal(10,2);not null"`
	Reason      string     `json:"reason" gorm:"not null"`
	Status      string     `json:"status" gorm:"not null;default:'PENDING';index:idx_order_refunds_status"`
	ProcessedAt *time.Time `json:"processedAt"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// BeforeCreate hook to generate order number
func (o *Order) BeforeCreate(tx *gorm.DB) (err error) {
	if o.OrderNumber == "" {
		o.OrderNumber = generateOrderNumber()
	}
	return
}

// generateOrderNumber creates a unique order number
func generateOrderNumber() string {
	timestamp := time.Now().Unix()
	return fmt.Sprintf("ORD-%d", timestamp)
}

// SplitType represents the reason for order splitting
type SplitType string

const (
	SplitTypePartialFulfillment SplitType = "PARTIAL_FULFILLMENT" // Some items available, others not
	SplitTypeBackorder          SplitType = "BACKORDER"           // Out of stock items
	SplitTypeMultiWarehouse     SplitType = "MULTI_WAREHOUSE"     // Items from different warehouses
	SplitTypeManual             SplitType = "MANUAL"              // Manual split by admin
)

// OrderSplit represents a split operation record
type OrderSplit struct {
	ID              uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID        string     `json:"tenantId" gorm:"type:varchar(255);not null;index"`
	OriginalOrderID uuid.UUID  `json:"originalOrderId" gorm:"type:uuid;not null;index"`
	NewOrderID      uuid.UUID  `json:"newOrderId" gorm:"type:uuid;not null;index"`
	SplitType       SplitType  `json:"splitType" gorm:"type:varchar(30);not null"`
	ItemIDs         JSONB      `json:"itemIds" gorm:"type:jsonb"` // Array of item IDs moved to new order
	Reason          string     `json:"reason" gorm:"type:text"`
	CreatedBy       *uuid.UUID `json:"createdBy,omitempty" gorm:"type:uuid"`
	CreatedAt       time.Time  `json:"createdAt"`
}

// SplitOrderRequest represents a request to split an order
type SplitOrderRequest struct {
	ItemIDs   []uuid.UUID `json:"itemIds" binding:"required"`   // Items to move to new order
	SplitType SplitType   `json:"splitType" binding:"required"` // Reason for split
	Reason    string      `json:"reason"`                       // Optional detailed reason
}
