package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Payment credential errors
var (
	ErrCredentialsNotConfigured = errors.New("payment credentials not configured")
	ErrInvalidProvider          = errors.New("invalid payment provider")
	ErrCredentialsFetchFailed   = errors.New("failed to fetch payment credentials")
)

// GatewayType represents the payment gateway provider
type GatewayType string

const (
	GatewayRazorpay  GatewayType = "RAZORPAY"
	GatewayPayU      GatewayType = "PAYU_INDIA"
	GatewayCashfree  GatewayType = "CASHFREE"
	GatewayPaytm     GatewayType = "PAYTM"
	GatewayStripe    GatewayType = "STRIPE"
	GatewayPayPal    GatewayType = "PAYPAL"
	GatewayPhonePe   GatewayType = "PHONEPE"
	GatewayBharatPay GatewayType = "BHARATPAY"
	GatewayAfterpay  GatewayType = "AFTERPAY"
	GatewayZip       GatewayType = "ZIP"
	GatewayLinkt     GatewayType = "LINKT"
)

// PaymentStatus represents the payment transaction status
type PaymentStatus string

const (
	PaymentPending    PaymentStatus = "PENDING"
	PaymentProcessing PaymentStatus = "PROCESSING"
	PaymentSucceeded  PaymentStatus = "SUCCEEDED"
	PaymentFailed     PaymentStatus = "FAILED"
	PaymentCanceled   PaymentStatus = "CANCELED"
	PaymentRefunded   PaymentStatus = "REFUNDED"
)

// PaymentMethodType represents the type of payment method
type PaymentMethodType string

const (
	MethodCard          PaymentMethodType = "CARD"
	MethodUPI           PaymentMethodType = "UPI"
	MethodNetBanking    PaymentMethodType = "NET_BANKING"
	MethodWallet        PaymentMethodType = "WALLET"
	MethodEMI           PaymentMethodType = "EMI"
	MethodPayLater      PaymentMethodType = "PAY_LATER"
	MethodBankAccount   PaymentMethodType = "BANK_ACCOUNT"
	MethodPayPal        PaymentMethodType = "PAYPAL"
	MethodApplePay      PaymentMethodType = "APPLE_PAY"
	MethodGooglePay     PaymentMethodType = "GOOGLE_PAY"
	MethodSEPA          PaymentMethodType = "SEPA"
	MethodIDeal         PaymentMethodType = "IDEAL"
	MethodKlarna        PaymentMethodType = "KLARNA"
	MethodRuPay         PaymentMethodType = "RUPAY"
	MethodCardlessEMI   PaymentMethodType = "CARDLESS_EMI"
)

// FeePayer represents who pays the platform fee
type FeePayer string

const (
	FeePayerMerchant FeePayer = "merchant"
	FeePayerCustomer FeePayer = "customer"
	FeePayerSplit    FeePayer = "split"
)

// RefundType represents the type of refund
type RefundType string

const (
	RefundTypeFull    RefundType = "full"
	RefundTypePartial RefundType = "partial"
)

// LedgerEntryType represents the type of platform fee ledger entry
type LedgerEntryType string

const (
	LedgerEntryCollection LedgerEntryType = "collection"
	LedgerEntryRefund     LedgerEntryType = "refund"
	LedgerEntryAdjustment LedgerEntryType = "adjustment"
	LedgerEntryPayout     LedgerEntryType = "payout"
)

// LedgerStatus represents the status of a ledger entry
type LedgerStatus string

const (
	LedgerStatusPending    LedgerStatus = "pending"
	LedgerStatusProcessing LedgerStatus = "processing"
	LedgerStatusCollected  LedgerStatus = "collected"
	LedgerStatusRefunded   LedgerStatus = "refunded"
	LedgerStatusFailed     LedgerStatus = "failed"
	LedgerStatusSettled    LedgerStatus = "settled"
)

// RefundStatus represents the refund transaction status
type RefundStatus string

const (
	RefundPending   RefundStatus = "PENDING"
	RefundSucceeded RefundStatus = "SUCCEEDED"
	RefundFailed    RefundStatus = "FAILED"
	RefundCanceled  RefundStatus = "CANCELED"
)

// DisputeStatus represents the dispute status
type DisputeStatus string

const (
	DisputeNeedsResponse DisputeStatus = "NEEDS_RESPONSE"
	DisputeUnderReview   DisputeStatus = "UNDER_REVIEW"
	DisputeWon           DisputeStatus = "WON"
	DisputeLost          DisputeStatus = "LOST"
	DisputeAccepted      DisputeStatus = "ACCEPTED"
)

// JSONB custom type for PostgreSQL
type JSONB map[string]interface{}

func (j JSONB) Value() (driver.Value, error) {
	return json.Marshal(j)
}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = make(map[string]interface{})
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

func (j JSONB) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}(j))
}

func (j *JSONB) UnmarshalJSON(data []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*j = JSONB(m)
	return nil
}

// StringArray custom type for PostgreSQL text[]
type StringArray []string

func (s StringArray) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return "{" + stringArrayJoin(s) + "}", nil
}

func stringArrayJoin(arr []string) string {
	result := ""
	for i, v := range arr {
		if i > 0 {
			result += ","
		}
		result += "\"" + v + "\""
	}
	return result
}

func (s *StringArray) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return s.parsePostgresArray(string(v))
	case string:
		return s.parsePostgresArray(v)
	}
	return nil
}

func (s *StringArray) parsePostgresArray(str string) error {
	// Handle empty array
	if str == "{}" || str == "" {
		*s = []string{}
		return nil
	}

	// Remove outer braces
	str = str[1 : len(str)-1]

	// Parse elements
	var result []string
	var current string
	inQuotes := false

	for _, char := range str {
		switch char {
		case '"':
			inQuotes = !inQuotes
		case ',':
			if !inQuotes {
				result = append(result, current)
				current = ""
			} else {
				current += string(char)
			}
		default:
			current += string(char)
		}
	}

	if current != "" {
		result = append(result, current)
	}

	*s = result
	return nil
}

// PaymentGatewayConfig represents a payment gateway configuration
type PaymentGatewayConfig struct {
	ID                     uuid.UUID   `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID               string      `gorm:"type:varchar(255);not null;index:idx_payment_gateways_tenant" json:"tenantId"`
	GatewayType            GatewayType `gorm:"type:varchar(50);not null" json:"gatewayType"`
	DisplayName            string      `gorm:"type:varchar(255);not null" json:"displayName"`
	IsEnabled              bool        `gorm:"default:true;index:idx_payment_gateways_enabled" json:"isEnabled"`
	IsTestMode             bool        `gorm:"default:true" json:"isTestMode"`

	// API Credentials
	APIKeyPublic           string      `gorm:"type:text" json:"apiKeyPublic"`
	APIKeySecret           string      `gorm:"type:text" json:"-"` // Never expose in JSON
	WebhookSecret          string      `gorm:"type:text" json:"-"` // Never expose in JSON

	// Configuration
	Config                 JSONB       `gorm:"type:jsonb" json:"config"`

	// Features
	SupportsPayments       bool        `gorm:"default:true" json:"supportsPayments"`
	SupportsRefunds        bool        `gorm:"default:true" json:"supportsRefunds"`
	SupportsSubscriptions  bool        `gorm:"default:false" json:"supportsSubscriptions"`

	// Platform Split (for fee collection)
	MerchantAccountID      string      `gorm:"type:varchar(255)" json:"merchantAccountId,omitempty"`
	PlatformAccountID      string      `gorm:"type:varchar(255)" json:"platformAccountId,omitempty"`
	SupportsPlatformSplit  bool        `gorm:"default:false" json:"supportsPlatformSplit"`

	// Geo-based Configuration
	SupportedCountries     StringArray `gorm:"type:text[]" json:"supportedCountries"`
	SupportedPaymentMethods StringArray `gorm:"type:text[]" json:"supportedPaymentMethods"`

	// Fee Structure
	FeeStructure           JSONB       `gorm:"type:jsonb;default:'{\"fixed_fee\": 0, \"percent_fee\": 0}'" json:"feeStructure"`

	// Limits
	MinimumAmount          float64     `gorm:"type:decimal(10,2)" json:"minimumAmount"`
	MaximumAmount          float64     `gorm:"type:decimal(10,2)" json:"maximumAmount"`

	// Display
	Priority               int         `gorm:"default:0" json:"priority"`
	Description            string      `gorm:"type:text" json:"description"`
	LogoURL                string      `gorm:"type:varchar(500)" json:"logoUrl,omitempty"`

	CreatedAt              time.Time   `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt              time.Time   `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`

	// Relationships
	Regions                []PaymentGatewayRegion `gorm:"foreignKey:GatewayConfigID" json:"regions,omitempty"`
}

// TableName specifies the table name for PaymentGatewayConfig
func (PaymentGatewayConfig) TableName() string {
	return "payment_gateway_configs"
}

// PaymentTransaction represents a payment transaction
type PaymentTransaction struct {
	ID                    uuid.UUID         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID              string            `gorm:"type:varchar(255);not null;index:idx_payment_transactions_tenant" json:"tenantId"`
	OrderID               uuid.UUID         `gorm:"type:uuid;not null;index:idx_payment_transactions_order" json:"orderId"`
	CustomerID            *uuid.UUID        `gorm:"type:uuid;index:idx_payment_transactions_customer" json:"customerId,omitempty"`

	// Gateway info
	GatewayConfigID       uuid.UUID         `gorm:"type:uuid;not null" json:"gatewayConfigId"`
	GatewayType           GatewayType       `gorm:"type:varchar(50);not null" json:"gatewayType"`
	GatewayTransactionID  string            `gorm:"type:varchar(255);index:idx_payment_transactions_gateway_id" json:"gatewayTransactionId,omitempty"`

	// Amount and Fees
	Amount                float64           `gorm:"type:decimal(12,2);not null" json:"amount"`
	GrossAmount           float64           `gorm:"type:decimal(12,2)" json:"grossAmount,omitempty"`
	PlatformFee           float64           `gorm:"type:decimal(12,2);default:0" json:"platformFee"`
	PlatformFeePercent    float64           `gorm:"type:decimal(5,4);default:0.0500" json:"platformFeePercent"`
	GatewayFee            float64           `gorm:"type:decimal(12,2);default:0" json:"gatewayFee"`
	GatewayTax            float64           `gorm:"type:decimal(12,2);default:0" json:"gatewayTax"`
	NetAmount             float64           `gorm:"type:decimal(12,2)" json:"netAmount,omitempty"`
	Currency              string            `gorm:"type:varchar(3);default:'USD'" json:"currency"`

	// Idempotency and Retry
	IdempotencyKey        string            `gorm:"type:varchar(255);index:idx_payment_transactions_idempotency" json:"idempotencyKey,omitempty"`
	RetryCount            int               `gorm:"default:0" json:"retryCount"`
	LastRetryAt           *time.Time        `json:"lastRetryAt,omitempty"`

	// Geo info
	CountryCode           string            `gorm:"type:varchar(2);index:idx_payment_transactions_country" json:"countryCode,omitempty"`

	// Status
	Status                PaymentStatus     `gorm:"type:varchar(50);not null;index:idx_payment_transactions_status" json:"status"`
	PaymentMethodType     PaymentMethodType `gorm:"type:varchar(50)" json:"paymentMethodType,omitempty"`

	// Card details (last 4 digits only)
	CardBrand             string            `gorm:"type:varchar(50)" json:"cardBrand,omitempty"`
	CardLastFour          string            `gorm:"type:varchar(4)" json:"cardLastFour,omitempty"`
	CardExpMonth          int               `json:"cardExpMonth,omitempty"`
	CardExpYear           int               `json:"cardExpYear,omitempty"`

	// Customer info
	BillingEmail          string            `gorm:"type:varchar(255)" json:"billingEmail,omitempty"`
	BillingName           string            `gorm:"type:varchar(255)" json:"billingName,omitempty"`
	BillingAddress        JSONB             `gorm:"type:jsonb" json:"billingAddress,omitempty"`

	// Processing
	ProcessedAt           *time.Time        `json:"processedAt,omitempty"`
	FailedAt              *time.Time        `json:"failedAt,omitempty"`
	FailureCode           string            `gorm:"type:varchar(100)" json:"failureCode,omitempty"`
	FailureMessage        string            `gorm:"type:text" json:"failureMessage,omitempty"`

	// Metadata
	Metadata              JSONB             `gorm:"type:jsonb" json:"metadata,omitempty"`

	CreatedAt             time.Time         `gorm:"default:CURRENT_TIMESTAMP;index:idx_payment_transactions_created" json:"createdAt"`
	UpdatedAt             time.Time         `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`

	// Relationships
	GatewayConfig         *PaymentGatewayConfig `gorm:"foreignKey:GatewayConfigID" json:"gatewayConfig,omitempty"`
	Refunds               []RefundTransaction   `gorm:"foreignKey:PaymentTransactionID" json:"refunds,omitempty"`
	FeeLedgerEntries      []PlatformFeeLedger   `gorm:"foreignKey:PaymentTransactionID" json:"feeLedgerEntries,omitempty"`
}

// TableName specifies the table name for PaymentTransaction
func (PaymentTransaction) TableName() string {
	return "payment_transactions"
}

// RefundTransaction represents a refund transaction
type RefundTransaction struct {
	ID                   uuid.UUID    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID             string       `gorm:"type:varchar(255);not null;index:idx_refunds_tenant" json:"tenantId"`
	PaymentTransactionID uuid.UUID    `gorm:"type:uuid;not null;index:idx_refunds_payment" json:"paymentTransactionId"`

	// Gateway info
	GatewayRefundID      string       `gorm:"type:varchar(255)" json:"gatewayRefundId,omitempty"`

	// Amount
	Amount               float64      `gorm:"type:decimal(12,2);not null" json:"amount"`
	Currency             string       `gorm:"type:varchar(3);default:'USD'" json:"currency"`

	// Fee Refunds
	PlatformFeeRefund    float64      `gorm:"type:decimal(12,2);default:0" json:"platformFeeRefund"`
	GatewayFeeRefund     float64      `gorm:"type:decimal(12,2);default:0" json:"gatewayFeeRefund"`
	NetRefundAmount      float64      `gorm:"type:decimal(12,2)" json:"netRefundAmount,omitempty"`
	RefundType           RefundType   `gorm:"type:varchar(20);default:'full'" json:"refundType"`

	// Status
	Status               RefundStatus `gorm:"type:varchar(50);not null;index:idx_refunds_status" json:"status"`
	Reason               string       `gorm:"type:varchar(255)" json:"reason,omitempty"`

	// Processing
	ProcessedAt          *time.Time   `json:"processedAt,omitempty"`
	FailedAt             *time.Time   `json:"failedAt,omitempty"`
	FailureCode          string       `gorm:"type:varchar(100)" json:"failureCode,omitempty"`
	FailureMessage       string       `gorm:"type:text" json:"failureMessage,omitempty"`

	// Metadata
	Metadata             JSONB        `gorm:"type:jsonb" json:"metadata,omitempty"`
	Notes                string       `gorm:"type:text" json:"notes,omitempty"`

	CreatedBy            *uuid.UUID   `gorm:"type:uuid" json:"createdBy,omitempty"`
	CreatedAt            time.Time    `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt            time.Time    `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`

	// Relationships
	PaymentTransaction   *PaymentTransaction `gorm:"foreignKey:PaymentTransactionID" json:"paymentTransaction,omitempty"`
	FeeLedgerEntries     []PlatformFeeLedger `gorm:"foreignKey:RefundTransactionID" json:"feeLedgerEntries,omitempty"`
}

// TableName specifies the table name for RefundTransaction
func (RefundTransaction) TableName() string {
	return "refund_transactions"
}

// WebhookEvent represents a webhook event from a payment gateway
type WebhookEvent struct {
	ID                   uuid.UUID   `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID             string      `gorm:"type:varchar(255);index:idx_webhooks_gateway" json:"tenantId,omitempty"`
	GatewayType          GatewayType `gorm:"type:varchar(50);not null;index:idx_webhooks_gateway" json:"gatewayType"`
	EventID              string      `gorm:"type:varchar(255);not null" json:"eventId"`
	EventType            string      `gorm:"type:varchar(100);not null;index:idx_webhooks_type" json:"eventType"`

	// Payload
	Payload              JSONB       `gorm:"type:jsonb;not null" json:"payload"`

	// Processing
	Processed            bool        `gorm:"default:false;index:idx_webhooks_processed" json:"processed"`
	ProcessedAt          *time.Time  `json:"processedAt,omitempty"`
	ProcessingError      string      `gorm:"type:text" json:"processingError,omitempty"`
	RetryCount           int         `gorm:"default:0" json:"retryCount"`

	// Related entities
	PaymentTransactionID *uuid.UUID  `gorm:"type:uuid" json:"paymentTransactionId,omitempty"`

	CreatedAt            time.Time   `gorm:"default:CURRENT_TIMESTAMP;index:idx_webhooks_created" json:"createdAt"`
}

// TableName specifies the table name for WebhookEvent
func (WebhookEvent) TableName() string {
	return "payment_webhook_events"
}

// SavedPaymentMethod represents a customer's saved payment method
type SavedPaymentMethod struct {
	ID                      uuid.UUID         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID                string            `gorm:"type:varchar(255);not null;index:idx_saved_payment_methods_tenant" json:"tenantId"`
	CustomerID              uuid.UUID         `gorm:"type:uuid;not null;index:idx_saved_payment_methods_customer" json:"customerId"`

	// Gateway info
	GatewayConfigID         uuid.UUID         `gorm:"type:uuid;not null;index:idx_saved_payment_methods_gateway" json:"gatewayConfigId"`
	GatewayType             GatewayType       `gorm:"type:varchar(50);not null" json:"gatewayType"`
	GatewayPaymentMethodID  string            `gorm:"type:varchar(255);not null;unique" json:"gatewayPaymentMethodId"`

	// Payment method details
	PaymentMethodType       PaymentMethodType `gorm:"type:varchar(50);not null" json:"paymentMethodType"`

	// Card info (for display only)
	CardBrand               string            `gorm:"type:varchar(50)" json:"cardBrand,omitempty"`
	CardLastFour            string            `gorm:"type:varchar(4)" json:"cardLastFour,omitempty"`
	CardExpMonth            int               `json:"cardExpMonth,omitempty"`
	CardExpYear             int               `json:"cardExpYear,omitempty"`

	// Bank account info (for display only)
	BankName                string            `gorm:"type:varchar(255)" json:"bankName,omitempty"`
	AccountLastFour         string            `gorm:"type:varchar(4)" json:"accountLastFour,omitempty"`

	// PayPal
	PayPalEmail             string            `gorm:"type:varchar(255)" json:"paypalEmail,omitempty"`

	// Status
	IsDefault               bool              `gorm:"default:false" json:"isDefault"`
	IsActive                bool              `gorm:"default:true" json:"isActive"`

	// Billing address
	BillingName             string            `gorm:"type:varchar(255)" json:"billingName,omitempty"`
	BillingEmail            string            `gorm:"type:varchar(255)" json:"billingEmail,omitempty"`
	BillingAddress          JSONB             `gorm:"type:jsonb" json:"billingAddress,omitempty"`

	// Verification
	IsVerified              bool              `gorm:"default:false" json:"isVerified"`
	VerifiedAt              *time.Time        `json:"verifiedAt,omitempty"`

	CreatedAt               time.Time         `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt               time.Time         `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`

	// Relationships
	GatewayConfig           *PaymentGatewayConfig `gorm:"foreignKey:GatewayConfigID" json:"gatewayConfig,omitempty"`
}

// TableName specifies the table name for SavedPaymentMethod
func (SavedPaymentMethod) TableName() string {
	return "saved_payment_methods"
}

// GatewayCustomer stores the mapping between internal customer IDs and gateway customer IDs
// This allows us to create a customer once in Stripe/Razorpay and reuse it for saved cards
type GatewayCustomer struct {
	ID                uuid.UUID   `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID          string      `gorm:"type:varchar(255);not null;index:idx_gateway_customers_tenant" json:"tenantId"`
	CustomerID        uuid.UUID   `gorm:"type:uuid;not null;index:idx_gateway_customers_customer" json:"customerId"`
	GatewayType       GatewayType `gorm:"type:varchar(50);not null;index:idx_gateway_customers_gateway" json:"gatewayType"`
	GatewayCustomerID string      `gorm:"type:varchar(255);not null" json:"gatewayCustomerId"` // e.g., Stripe's cus_xxx

	// Customer details synced with gateway
	Email     string `gorm:"type:varchar(255)" json:"email,omitempty"`
	Name      string `gorm:"type:varchar(255)" json:"name,omitempty"`
	Phone     string `gorm:"type:varchar(50)" json:"phone,omitempty"`

	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
}

// TableName specifies the table name for GatewayCustomer
func (GatewayCustomer) TableName() string {
	return "gateway_customers"
}

// PaymentDispute represents a payment dispute/chargeback
type PaymentDispute struct {
	ID                   uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID             string          `gorm:"type:varchar(255);not null;index:idx_disputes_tenant" json:"tenantId"`
	PaymentTransactionID uuid.UUID       `gorm:"type:uuid;not null;index:idx_disputes_payment" json:"paymentTransactionId"`

	// Gateway info
	GatewayDisputeID     string          `gorm:"type:varchar(255);not null" json:"gatewayDisputeId"`

	// Dispute details
	Amount               float64         `gorm:"type:decimal(12,2);not null" json:"amount"`
	Currency             string          `gorm:"type:varchar(3);default:'USD'" json:"currency"`
	Reason               string          `gorm:"type:varchar(100)" json:"reason,omitempty"`
	Status               DisputeStatus   `gorm:"type:varchar(50);not null;index:idx_disputes_status" json:"status"`

	// Evidence
	Evidence             JSONB           `gorm:"type:jsonb" json:"evidence,omitempty"`
	EvidenceSubmittedAt  *time.Time      `json:"evidenceSubmittedAt,omitempty"`

	// Deadlines
	RespondBy            *time.Time      `json:"respondBy,omitempty"`

	// Resolution
	ResolvedAt           *time.Time      `json:"resolvedAt,omitempty"`
	Resolution           string          `gorm:"type:varchar(50)" json:"resolution,omitempty"`

	CreatedAt            time.Time       `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt            time.Time       `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`

	// Relationships
	PaymentTransaction   *PaymentTransaction `gorm:"foreignKey:PaymentTransactionID" json:"paymentTransaction,omitempty"`
}

// TableName specifies the table name for PaymentDispute
func (PaymentDispute) TableName() string {
	return "payment_disputes"
}

// PaymentSettings represents global payment settings per tenant
type PaymentSettings struct {
	ID                                uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID                          string    `gorm:"type:varchar(255);not null;unique;index:idx_payment_settings_tenant" json:"tenantId"`

	// Currency
	DefaultCurrency                   string    `gorm:"type:varchar(3);default:'USD'" json:"defaultCurrency"`
	SupportedCurrencies               []string  `gorm:"type:varchar(3)[]" json:"supportedCurrencies"`

	// Platform Fees (5% by default)
	PlatformFeeEnabled                bool      `gorm:"default:true" json:"platformFeeEnabled"`
	PlatformFeePercent                float64   `gorm:"type:decimal(5,4);default:0.0500" json:"platformFeePercent"`
	PlatformAccountID                 string    `gorm:"type:varchar(255)" json:"platformAccountId,omitempty"`
	FeePayer                          FeePayer  `gorm:"type:varchar(20);default:'merchant'" json:"feePayer"`
	MinimumPlatformFee                float64   `gorm:"type:decimal(10,2);default:0" json:"minimumPlatformFee"`
	MaximumPlatformFee                *float64  `gorm:"type:decimal(10,2)" json:"maximumPlatformFee,omitempty"`

	// Checkout
	EnableExpressCheckout             bool      `gorm:"default:true" json:"enableExpressCheckout"`
	CollectBillingAddress             bool      `gorm:"default:true" json:"collectBillingAddress"`
	CollectShippingAddress            bool      `gorm:"default:true" json:"collectShippingAddress"`

	// 3D Secure
	Enable3DSecure                    bool      `gorm:"default:true" json:"enable3dSecure"`
	Require3DSecureForAmountsAbove    float64   `gorm:"type:decimal(10,2)" json:"require3dSecureForAmountsAbove,omitempty"`

	// Fraud prevention
	EnableFraudDetection              bool      `gorm:"default:true" json:"enableFraudDetection"`
	AutoCancelRiskyPayments           bool      `gorm:"default:false" json:"autoCancelRiskyPayments"`

	// Failed payments
	MaxPaymentRetryAttempts           int       `gorm:"default:3" json:"maxPaymentRetryAttempts"`

	// Notifications
	SendPaymentReceipts               bool      `gorm:"default:true" json:"sendPaymentReceipts"`
	SendRefundNotifications           bool      `gorm:"default:true" json:"sendRefundNotifications"`

	CreatedAt                         time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt                         time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
}

// TableName specifies the table name for PaymentSettings
func (PaymentSettings) TableName() string {
	return "payment_settings"
}

// PlatformFeeLedger represents the ledger for platform fee collection and reconciliation
type PlatformFeeLedger struct {
	ID                   uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID             string          `gorm:"type:varchar(255);not null;index:idx_platform_fee_ledger_tenant" json:"tenantId"`
	PaymentTransactionID *uuid.UUID      `gorm:"type:uuid;index:idx_platform_fee_ledger_payment" json:"paymentTransactionId,omitempty"`
	RefundTransactionID  *uuid.UUID      `gorm:"type:uuid;index:idx_platform_fee_ledger_refund" json:"refundTransactionId,omitempty"`

	// Entry details
	EntryType            LedgerEntryType `gorm:"type:varchar(20);not null" json:"entryType"`
	Amount               float64         `gorm:"type:decimal(12,2);not null" json:"amount"`
	Currency             string          `gorm:"type:varchar(3);default:'USD'" json:"currency"`

	// Status tracking
	Status               LedgerStatus    `gorm:"type:varchar(20);default:'pending';index:idx_platform_fee_ledger_status" json:"status"`

	// Gateway transfer info
	GatewayType          GatewayType     `gorm:"type:varchar(50)" json:"gatewayType,omitempty"`
	GatewayTransferID    string          `gorm:"type:varchar(255)" json:"gatewayTransferId,omitempty"`
	GatewayPayoutID      string          `gorm:"type:varchar(255)" json:"gatewayPayoutId,omitempty"`

	// Settlement
	SettledAt            *time.Time      `json:"settledAt,omitempty"`
	SettlementBatchID    string          `gorm:"type:varchar(255)" json:"settlementBatchId,omitempty"`

	// Error tracking
	ErrorCode            string          `gorm:"type:varchar(100)" json:"errorCode,omitempty"`
	ErrorMessage         string          `gorm:"type:text" json:"errorMessage,omitempty"`

	CreatedAt            time.Time       `gorm:"default:CURRENT_TIMESTAMP;index:idx_platform_fee_ledger_created" json:"createdAt"`
	UpdatedAt            time.Time       `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`

	// Relationships
	PaymentTransaction   *PaymentTransaction `gorm:"foreignKey:PaymentTransactionID" json:"paymentTransaction,omitempty"`
	RefundTransaction    *RefundTransaction  `gorm:"foreignKey:RefundTransactionID" json:"refundTransaction,omitempty"`
}

// TableName specifies the table name for PlatformFeeLedger
func (PlatformFeeLedger) TableName() string {
	return "platform_fee_ledger"
}

// PaymentGatewayRegion represents country-specific configuration for payment gateways
type PaymentGatewayRegion struct {
	ID               uuid.UUID   `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	GatewayConfigID  uuid.UUID   `gorm:"type:uuid;not null;index:idx_gateway_regions_gateway" json:"gatewayConfigId"`
	CountryCode      string      `gorm:"type:varchar(2);not null;index:idx_gateway_regions_country" json:"countryCode"`
	IsPrimary        bool        `gorm:"default:false" json:"isPrimary"`
	Priority         int         `gorm:"default:0" json:"priority"`
	Enabled          bool        `gorm:"default:true;index:idx_gateway_regions_enabled" json:"enabled"`

	// Region-specific settings
	SupportedMethods StringArray `gorm:"type:text[]" json:"supportedMethods,omitempty"`
	MinAmount        *float64    `gorm:"type:decimal(10,2)" json:"minAmount,omitempty"`
	MaxAmount        *float64    `gorm:"type:decimal(10,2)" json:"maxAmount,omitempty"`
	Currency         string      `gorm:"type:varchar(3)" json:"currency,omitempty"`

	CreatedAt        time.Time   `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt        time.Time   `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`

	// Relationships
	GatewayConfig    *PaymentGatewayConfig `gorm:"foreignKey:GatewayConfigID" json:"gatewayConfig,omitempty"`
}

// TableName specifies the table name for PaymentGatewayRegion
func (PaymentGatewayRegion) TableName() string {
	return "payment_gateway_regions"
}

// PaymentGatewayTemplate represents pre-configured templates for easy gateway setup
type PaymentGatewayTemplate struct {
	ID                      uuid.UUID   `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	GatewayType             GatewayType `gorm:"type:varchar(50);not null;unique" json:"gatewayType"`
	DisplayName             string      `gorm:"type:varchar(255);not null" json:"displayName"`
	Description             string      `gorm:"type:text" json:"description,omitempty"`
	LogoURL                 string      `gorm:"type:varchar(500)" json:"logoUrl,omitempty"`

	// Supported features
	SupportsPayments        bool        `gorm:"default:true" json:"supportsPayments"`
	SupportsRefunds         bool        `gorm:"default:true" json:"supportsRefunds"`
	SupportsSubscriptions   bool        `gorm:"default:false" json:"supportsSubscriptions"`
	SupportsPlatformSplit   bool        `gorm:"default:false" json:"supportsPlatformSplit"`

	// Regional support
	SupportedCountries      StringArray `gorm:"type:text[];not null" json:"supportedCountries"`
	SupportedPaymentMethods StringArray `gorm:"type:text[];not null" json:"supportedPaymentMethods"`

	// Default configuration
	DefaultConfig           JSONB       `gorm:"type:jsonb" json:"defaultConfig,omitempty"`

	// Required credentials
	RequiredCredentials     StringArray `gorm:"type:text[];default:'{\"api_key_public\",\"api_key_secret\"}'" json:"requiredCredentials"`

	// Documentation
	SetupInstructions       string      `gorm:"type:text" json:"setupInstructions,omitempty"`
	DocumentationURL        string      `gorm:"type:varchar(500)" json:"documentationUrl,omitempty"`

	// Display
	Priority                int         `gorm:"default:0" json:"priority"`
	IsActive                bool        `gorm:"default:true" json:"isActive"`

	CreatedAt               time.Time   `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt               time.Time   `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
}

// TableName specifies the table name for PaymentGatewayTemplate
func (PaymentGatewayTemplate) TableName() string {
	return "payment_gateway_templates"
}

// FeeCalculation represents the result of fee calculation
type FeeCalculation struct {
	GrossAmount     float64 `json:"grossAmount"`
	PlatformFee     float64 `json:"platformFee"`
	PlatformPercent float64 `json:"platformPercent"`
	GatewayFee      float64 `json:"gatewayFee"`
	TaxAmount       float64 `json:"taxAmount"`
	NetAmount       float64 `json:"netAmount"`
}

// PaymentMethodOption represents an available payment method for checkout
type PaymentMethodOption struct {
	ID          string            `json:"id"`
	Type        PaymentMethodType `json:"type"`
	DisplayName string            `json:"displayName"`
	GatewayID   string            `json:"gatewayId"`
	GatewayType GatewayType       `json:"gatewayType"`
	Icon        string            `json:"icon,omitempty"`
	Description string            `json:"description,omitempty"`
	Priority    int               `json:"priority"`
}
