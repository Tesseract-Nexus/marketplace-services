package models

import (
	"time"

	"github.com/google/uuid"
)

// AdPaymentType represents the type of ad payment
type AdPaymentType string

const (
	AdPaymentTypeDirect    AdPaymentType = "DIRECT"    // Full budget upfront
	AdPaymentTypeSponsored AdPaymentType = "SPONSORED" // Commission-based
)

// AdPaymentStatus represents the status of an ad payment
type AdPaymentStatus string

const (
	AdPaymentPending    AdPaymentStatus = "PENDING"
	AdPaymentProcessing AdPaymentStatus = "PROCESSING"
	AdPaymentPaid       AdPaymentStatus = "PAID"
	AdPaymentFailed     AdPaymentStatus = "FAILED"
	AdPaymentRefunded   AdPaymentStatus = "REFUNDED"
	AdPaymentCancelled  AdPaymentStatus = "CANCELLED"
)

// AdBillingInvoiceStatus represents the status of a billing invoice
type AdBillingInvoiceStatus string

const (
	AdInvoicePending   AdBillingInvoiceStatus = "PENDING"
	AdInvoiceInvoiced  AdBillingInvoiceStatus = "INVOICED"
	AdInvoicePaid      AdBillingInvoiceStatus = "PAID"
	AdInvoiceOverdue   AdBillingInvoiceStatus = "OVERDUE"
	AdInvoiceCancelled AdBillingInvoiceStatus = "CANCELLED"
)

// AdLedgerEntryType represents the type of ledger entry
type AdLedgerEntryType string

const (
	AdLedgerPayment    AdLedgerEntryType = "PAYMENT"
	AdLedgerSpend      AdLedgerEntryType = "SPEND"
	AdLedgerRefund     AdLedgerEntryType = "REFUND"
	AdLedgerAdjustment AdLedgerEntryType = "ADJUSTMENT"
	AdLedgerCommission AdLedgerEntryType = "COMMISSION"
)

// AdCommissionTier represents a commission tier configuration
type AdCommissionTier struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID       string    `gorm:"type:varchar(255);not null;index:idx_ad_commission_tiers_tenant" json:"tenantId"`
	Name           string    `gorm:"type:varchar(100);not null" json:"name"`
	MinDays        int       `gorm:"not null" json:"minDays"`
	MaxDays        *int      `gorm:"" json:"maxDays,omitempty"`
	CommissionRate float64   `gorm:"type:decimal(5,4);not null" json:"commissionRate"` // e.g., 0.019 for 1.9%
	TaxInclusive   bool      `gorm:"default:true" json:"taxInclusive"`
	IsActive       bool      `gorm:"default:true;index:idx_ad_commission_tiers_active" json:"isActive"`
	Priority       int       `gorm:"default:0" json:"priority"`
	Description    string    `gorm:"type:text" json:"description,omitempty"`
	CreatedAt      time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt      time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
}

// TableName specifies the table name for AdCommissionTier
func (AdCommissionTier) TableName() string {
	return "ad_commission_tiers"
}

// AdCampaignPayment represents a payment for an ad campaign
type AdCampaignPayment struct {
	ID                   uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID             string          `gorm:"type:varchar(255);not null;index:idx_ad_campaign_payments_tenant" json:"tenantId"`
	VendorID             uuid.UUID       `gorm:"type:uuid;not null;index:idx_ad_campaign_payments_vendor" json:"vendorId"`
	CampaignID           uuid.UUID       `gorm:"type:uuid;not null;index:idx_ad_campaign_payments_campaign" json:"campaignId"`

	// Payment type
	PaymentType AdPaymentType   `gorm:"type:varchar(20);not null" json:"paymentType"`
	Status      AdPaymentStatus `gorm:"type:varchar(20);not null;default:'PENDING';index:idx_ad_campaign_payments_status" json:"status"`

	// Amounts
	BudgetAmount     float64 `gorm:"type:decimal(12,2);not null" json:"budgetAmount"`
	CommissionRate   float64 `gorm:"type:decimal(5,4)" json:"commissionRate,omitempty"`
	CommissionAmount float64 `gorm:"type:decimal(12,2);default:0" json:"commissionAmount"`
	TaxAmount        float64 `gorm:"type:decimal(12,2);default:0" json:"taxAmount"`
	TotalAmount      float64 `gorm:"type:decimal(12,2);not null" json:"totalAmount"`
	Currency         string  `gorm:"type:varchar(3);default:'USD'" json:"currency"`

	// Commission tier reference
	CommissionTierID *uuid.UUID `gorm:"type:uuid" json:"commissionTierId,omitempty"`

	// Campaign duration (for commission calculation)
	CampaignDays int `gorm:"" json:"campaignDays,omitempty"`

	// Link to payment transaction
	PaymentTransactionID *uuid.UUID `gorm:"type:uuid" json:"paymentTransactionId,omitempty"`

	// Gateway info
	GatewayType          GatewayType `gorm:"type:varchar(50)" json:"gatewayType,omitempty"`
	GatewayTransactionID string      `gorm:"type:varchar(255)" json:"gatewayTransactionId,omitempty"`

	// Timestamps
	PaidAt     *time.Time `json:"paidAt,omitempty"`
	RefundedAt *time.Time `json:"refundedAt,omitempty"`

	// Metadata
	Metadata JSONB `gorm:"type:jsonb" json:"metadata,omitempty"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP;index:idx_ad_campaign_payments_created" json:"createdAt"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`

	// Relationships
	PaymentTransaction *PaymentTransaction `gorm:"foreignKey:PaymentTransactionID" json:"paymentTransaction,omitempty"`
	CommissionTier     *AdCommissionTier   `gorm:"foreignKey:CommissionTierID" json:"commissionTier,omitempty"`
}

// TableName specifies the table name for AdCampaignPayment
func (AdCampaignPayment) TableName() string {
	return "ad_campaign_payments"
}

// AdBillingInvoice represents an invoice for ad billing
type AdBillingInvoice struct {
	ID            uuid.UUID              `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID      string                 `gorm:"type:varchar(255);not null;index:idx_ad_billing_invoices_tenant" json:"tenantId"`
	VendorID      uuid.UUID              `gorm:"type:uuid;not null;index:idx_ad_billing_invoices_vendor" json:"vendorId"`
	InvoiceNumber string                 `gorm:"type:varchar(50);not null;uniqueIndex" json:"invoiceNumber"`

	// Billing period
	PeriodStart time.Time              `gorm:"not null" json:"periodStart"`
	PeriodEnd   time.Time              `gorm:"not null" json:"periodEnd"`
	Status      AdBillingInvoiceStatus `gorm:"type:varchar(20);not null;default:'PENDING';index:idx_ad_billing_invoices_status" json:"status"`

	// Amounts
	TotalSpend       float64 `gorm:"type:decimal(12,2);not null" json:"totalSpend"`
	CommissionRate   float64 `gorm:"type:decimal(5,4);not null" json:"commissionRate"`
	CommissionAmount float64 `gorm:"type:decimal(12,2);not null" json:"commissionAmount"`
	TaxAmount        float64 `gorm:"type:decimal(12,2);default:0" json:"taxAmount"`
	TotalDue         float64 `gorm:"type:decimal(12,2);not null" json:"totalDue"`
	Currency         string  `gorm:"type:varchar(3);default:'USD'" json:"currency"`

	// Payment info
	DueDate              time.Time  `gorm:"not null;index:idx_ad_billing_invoices_due" json:"dueDate"`
	PaidAt               *time.Time `json:"paidAt,omitempty"`
	PaymentTransactionID *uuid.UUID `gorm:"type:uuid" json:"paymentTransactionId,omitempty"`

	// Line items breakdown
	LineItems JSONB `gorm:"type:jsonb" json:"lineItems,omitempty"`

	// Metadata
	Metadata JSONB `gorm:"type:jsonb" json:"metadata,omitempty"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`

	// Relationships
	PaymentTransaction *PaymentTransaction `gorm:"foreignKey:PaymentTransactionID" json:"paymentTransaction,omitempty"`
}

// TableName specifies the table name for AdBillingInvoice
func (AdBillingInvoice) TableName() string {
	return "ad_billing_invoices"
}

// AdRevenueLedger tracks all ad revenue for the platform
type AdRevenueLedger struct {
	ID                   uuid.UUID         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID             string            `gorm:"type:varchar(255);not null;index:idx_ad_revenue_ledger_tenant" json:"tenantId"`
	VendorID             uuid.UUID         `gorm:"type:uuid;not null;index:idx_ad_revenue_ledger_vendor" json:"vendorId"`
	CampaignID           uuid.UUID         `gorm:"type:uuid;not null;index:idx_ad_revenue_ledger_campaign" json:"campaignId"`

	// Entry type
	EntryType AdLedgerEntryType `gorm:"type:varchar(30);not null;index:idx_ad_revenue_ledger_type" json:"entryType"`

	// Amounts
	Amount       float64 `gorm:"type:decimal(12,2);not null" json:"amount"`
	Currency     string  `gorm:"type:varchar(3);default:'USD'" json:"currency"`

	// Running balance (per vendor)
	BalanceAfter float64 `gorm:"type:decimal(12,2);not null" json:"balanceAfter"`

	// References
	CampaignPaymentID    *uuid.UUID `gorm:"type:uuid" json:"campaignPaymentId,omitempty"`
	InvoiceID            *uuid.UUID `gorm:"type:uuid" json:"invoiceId,omitempty"`
	PaymentTransactionID *uuid.UUID `gorm:"type:uuid" json:"paymentTransactionId,omitempty"`

	// Description
	Description string `gorm:"type:text" json:"description,omitempty"`

	// Metadata
	Metadata JSONB `gorm:"type:jsonb" json:"metadata,omitempty"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP;index:idx_ad_revenue_ledger_created" json:"createdAt"`

	// Relationships
	CampaignPayment    *AdCampaignPayment  `gorm:"foreignKey:CampaignPaymentID" json:"campaignPayment,omitempty"`
	Invoice            *AdBillingInvoice   `gorm:"foreignKey:InvoiceID" json:"invoice,omitempty"`
	PaymentTransaction *PaymentTransaction `gorm:"foreignKey:PaymentTransactionID" json:"paymentTransaction,omitempty"`
}

// TableName specifies the table name for AdRevenueLedger
func (AdRevenueLedger) TableName() string {
	return "ad_revenue_ledger"
}

// AdVendorBalance represents the current balance for a vendor
type AdVendorBalance struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID       string    `gorm:"type:varchar(255);not null;index:idx_ad_vendor_balances_tenant" json:"tenantId"`
	VendorID       uuid.UUID `gorm:"type:uuid;not null;index:idx_ad_vendor_balances_vendor" json:"vendorId"`

	// Balance
	CurrentBalance float64 `gorm:"type:decimal(12,2);not null;default:0" json:"currentBalance"`
	TotalDeposited float64 `gorm:"type:decimal(12,2);not null;default:0" json:"totalDeposited"`
	TotalSpent     float64 `gorm:"type:decimal(12,2);not null;default:0" json:"totalSpent"`
	TotalRefunded  float64 `gorm:"type:decimal(12,2);not null;default:0" json:"totalRefunded"`
	Currency       string  `gorm:"type:varchar(3);default:'USD'" json:"currency"`

	// Status
	IsActive bool `gorm:"default:true" json:"isActive"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
}

// TableName specifies the table name for AdVendorBalance
func (AdVendorBalance) TableName() string {
	return "ad_vendor_balances"
}

// CommissionCalculation holds the result of commission calculation
type CommissionCalculation struct {
	TierID           uuid.UUID `json:"tierId"`
	TierName         string    `json:"tierName"`
	CampaignDays     int       `json:"campaignDays"`
	BudgetAmount     float64   `json:"budgetAmount"`
	CommissionRate   float64   `json:"commissionRate"`
	CommissionAmount float64   `json:"commissionAmount"`
	TaxInclusive     bool      `json:"taxInclusive"`
	TaxAmount        float64   `json:"taxAmount"`
	TotalAmount      float64   `json:"totalAmount"`
	Currency         string    `json:"currency"`
}

// CreateAdPaymentRequest represents a request to create an ad payment
type CreateAdPaymentRequest struct {
	TenantID     string    `json:"tenantId" binding:"required"`
	VendorID     uuid.UUID `json:"vendorId" binding:"required"`
	CampaignID   uuid.UUID `json:"campaignId" binding:"required"`
	BudgetAmount float64   `json:"budgetAmount" binding:"required,gt=0"`
	CampaignDays int       `json:"campaignDays" binding:"required,gt=0"`
	Currency     string    `json:"currency"`
}

// ProcessAdPaymentRequest represents a request to process an ad payment
type ProcessAdPaymentRequest struct {
	GatewayType GatewayType `json:"gatewayType" binding:"required"`
}

// AdPaymentIntentResponse represents the response when creating a payment intent
type AdPaymentIntentResponse struct {
	PaymentID        uuid.UUID              `json:"paymentId"`
	Status           AdPaymentStatus        `json:"status"`
	TotalAmount      float64                `json:"totalAmount"`
	Currency         string                 `json:"currency"`

	// Stripe specific
	StripeSessionID  string `json:"stripeSessionId,omitempty"`
	StripeSessionURL string `json:"stripeSessionUrl,omitempty"`
	StripePublicKey  string `json:"stripePublicKey,omitempty"`

	// Razorpay specific
	RazorpayOrderID string                 `json:"razorpayOrderId,omitempty"`
	Options         map[string]interface{} `json:"options,omitempty"`
}

// CalculateCommissionRequest represents a request to calculate commission
type CalculateCommissionRequest struct {
	CampaignDays int     `json:"campaignDays" binding:"required,gt=0"`
	BudgetAmount float64 `json:"budgetAmount" binding:"required,gt=0"`
}
