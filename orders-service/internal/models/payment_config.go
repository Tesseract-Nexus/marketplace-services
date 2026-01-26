package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// PaymentMethodType represents the type of payment method
type PaymentMethodType string

const (
	PaymentMethodTypeCard       PaymentMethodType = "card"
	PaymentMethodTypeWallet     PaymentMethodType = "wallet"
	PaymentMethodTypeBNPL       PaymentMethodType = "bnpl"
	PaymentMethodTypeUPI        PaymentMethodType = "upi"
	PaymentMethodTypeNetbanking PaymentMethodType = "netbanking"
	PaymentMethodTypeGateway    PaymentMethodType = "gateway"
	PaymentMethodTypeCOD        PaymentMethodType = "cod"
	PaymentMethodTypeBank       PaymentMethodType = "bank"
)

// PaymentMethod represents an available payment method in the system
// This is seeded reference data
type PaymentMethod struct {
	ID                    uuid.UUID         `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Code                  string            `json:"code" gorm:"type:varchar(50);not null;unique"`
	Name                  string            `json:"name" gorm:"type:varchar(100);not null"`
	Description           string            `json:"description" gorm:"type:text"`
	Provider              string            `json:"provider" gorm:"type:varchar(50);not null"`
	Type                  PaymentMethodType `json:"type" gorm:"type:varchar(30);not null"`
	SupportedRegions      pq.StringArray    `json:"supportedRegions" gorm:"type:text[];not null"`
	SupportedCurrencies   pq.StringArray    `json:"supportedCurrencies" gorm:"type:text[];not null"`
	IconURL               string            `json:"iconUrl" gorm:"type:text"`
	TransactionFeePercent float64           `json:"transactionFeePercent" gorm:"type:decimal(5,2);default:0"`
	TransactionFeeFixed   float64           `json:"transactionFeeFixed" gorm:"type:decimal(10,2);default:0"`
	DisplayOrder          int               `json:"displayOrder" gorm:"default:0"`
	IsActive              bool              `json:"isActive" gorm:"default:true"`
	CreatedAt             time.Time         `json:"createdAt"`
	UpdatedAt             time.Time         `json:"updatedAt"`
}

// TableName specifies the table name for PaymentMethod
func (PaymentMethod) TableName() string {
	return "payment_methods"
}

// PaymentConfigSettings represents non-sensitive settings for a payment config
type PaymentConfigSettings struct {
	// Display settings
	DisplayName     string `json:"displayName,omitempty"`
	CustomIconURL   string `json:"customIconUrl,omitempty"`
	CheckoutMessage string `json:"checkoutMessage,omitempty"`

	// Payment limits
	MinOrderAmount float64 `json:"minOrderAmount,omitempty"`
	MaxOrderAmount float64 `json:"maxOrderAmount,omitempty"`

	// Webhook settings
	WebhookURL string `json:"webhookUrl,omitempty"`

	// Provider-specific settings (non-sensitive)
	MerchantName     string `json:"merchantName,omitempty"`
	MerchantCategory string `json:"merchantCategory,omitempty"`
}

// Value implements driver.Valuer for PaymentConfigSettings
func (s PaymentConfigSettings) Value() (driver.Value, error) {
	return json.Marshal(s)
}

// Scan implements sql.Scanner for PaymentConfigSettings
func (s *PaymentConfigSettings) Scan(value interface{}) error {
	if value == nil {
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

// TenantPaymentConfig represents a tenant's configuration for a payment method
type TenantPaymentConfig struct {
	ID                uuid.UUID              `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID          string                 `json:"tenantId" gorm:"type:varchar(255);not null;index:idx_tenant_payment_configs_tenant"`
	PaymentMethodCode string                 `json:"paymentMethodCode" gorm:"type:varchar(50);not null"`
	IsEnabled         bool                   `json:"isEnabled" gorm:"default:false"`
	IsTestMode        bool                   `json:"isTestMode" gorm:"default:true"`
	DisplayOrder      int                    `json:"displayOrder" gorm:"default:0"`
	// EnabledRegions - which regions the tenant wants to enable this payment method for
	// If empty/nil, uses the payment method's default supported regions
	EnabledRegions    pq.StringArray         `json:"enabledRegions" gorm:"type:text[]"`
	// Encrypted credentials - stored as encrypted bytes
	CredentialsEncrypted []byte                 `json:"-" gorm:"type:bytea"`
	// Non-sensitive settings
	Settings          PaymentConfigSettings  `json:"settings" gorm:"type:jsonb;default:'{}'"`
	// Test connection tracking
	LastTestAt        *time.Time             `json:"lastTestAt"`
	LastTestSuccess   *bool                  `json:"lastTestSuccess"`
	LastTestMessage   string                 `json:"lastTestMessage,omitempty"`
	// Audit fields
	CreatedAt         time.Time              `json:"createdAt"`
	UpdatedAt         time.Time              `json:"updatedAt"`
	CreatedBy         string                 `json:"createdBy,omitempty"`
	UpdatedBy         string                 `json:"updatedBy,omitempty"`

	// Relationship
	PaymentMethod     *PaymentMethod         `json:"paymentMethod,omitempty" gorm:"foreignKey:PaymentMethodCode;references:Code"`
}

// TableName specifies the table name for TenantPaymentConfig
func (TenantPaymentConfig) TableName() string {
	return "tenant_payment_configs"
}

// HasCredentials returns true if credentials are configured
func (c *TenantPaymentConfig) HasCredentials() bool {
	return len(c.CredentialsEncrypted) > 0
}

// PaymentConfigAuditLog represents an audit log entry for payment config changes
type PaymentConfigAuditLog struct {
	ID                uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TenantID          string    `json:"tenantId" gorm:"type:varchar(255);not null;index:idx_payment_config_audit_tenant"`
	PaymentMethodCode string    `json:"paymentMethodCode" gorm:"type:varchar(50);not null"`
	Action            string    `json:"action" gorm:"type:varchar(50);not null"` // enable, disable, configure, test
	UserID            string    `json:"userId,omitempty" gorm:"type:varchar(255)"`
	IPAddress         string    `json:"ipAddress,omitempty" gorm:"type:varchar(45)"`
	Changes           JSONB     `json:"changes,omitempty" gorm:"type:jsonb"` // Old and new values (credentials masked)
	CreatedAt         time.Time `json:"createdAt" gorm:"index:idx_payment_config_audit_created"`
}

// TableName specifies the table name for PaymentConfigAuditLog
func (PaymentConfigAuditLog) TableName() string {
	return "payment_config_audit_log"
}

// PaymentCredentials represents the structure of payment provider credentials
// This is encrypted before storage
type PaymentCredentials struct {
	// Stripe
	StripePublishableKey string `json:"stripePublishableKey,omitempty"`
	StripeSecretKey      string `json:"stripeSecretKey,omitempty"`
	StripeWebhookSecret  string `json:"stripeWebhookSecret,omitempty"`

	// PayPal
	PayPalClientID     string `json:"paypalClientId,omitempty"`
	PayPalClientSecret string `json:"paypalClientSecret,omitempty"`

	// Razorpay
	RazorpayKeyID       string `json:"razorpayKeyId,omitempty"`
	RazorpayKeySecret   string `json:"razorpayKeySecret,omitempty"`
	RazorpayWebhookSecret string `json:"razorpayWebhookSecret,omitempty"`

	// Afterpay
	AfterpayMerchantID string `json:"afterpayMerchantId,omitempty"`
	AfterpaySecretKey  string `json:"afterpaySecretKey,omitempty"`

	// Zip
	ZipMerchantID string `json:"zipMerchantId,omitempty"`
	ZipAPIKey     string `json:"zipApiKey,omitempty"`
}

// MaskSecrets returns a copy of credentials with secrets masked
func (c *PaymentCredentials) MaskSecrets() *PaymentCredentials {
	masked := &PaymentCredentials{}

	// Stripe - show last 4 chars of publishable key, mask secret
	if c.StripePublishableKey != "" {
		if len(c.StripePublishableKey) > 4 {
			masked.StripePublishableKey = "****" + c.StripePublishableKey[len(c.StripePublishableKey)-4:]
		} else {
			masked.StripePublishableKey = "****"
		}
	}
	if c.StripeSecretKey != "" {
		masked.StripeSecretKey = "****"
	}
	if c.StripeWebhookSecret != "" {
		masked.StripeWebhookSecret = "****"
	}

	// PayPal - show last 4 chars of client ID, mask secret
	if c.PayPalClientID != "" {
		if len(c.PayPalClientID) > 4 {
			masked.PayPalClientID = "****" + c.PayPalClientID[len(c.PayPalClientID)-4:]
		} else {
			masked.PayPalClientID = "****"
		}
	}
	if c.PayPalClientSecret != "" {
		masked.PayPalClientSecret = "****"
	}

	// Razorpay - show last 4 chars of key ID, mask secret
	if c.RazorpayKeyID != "" {
		if len(c.RazorpayKeyID) > 4 {
			masked.RazorpayKeyID = "****" + c.RazorpayKeyID[len(c.RazorpayKeyID)-4:]
		} else {
			masked.RazorpayKeyID = "****"
		}
	}
	if c.RazorpayKeySecret != "" {
		masked.RazorpayKeySecret = "****"
	}
	if c.RazorpayWebhookSecret != "" {
		masked.RazorpayWebhookSecret = "****"
	}

	// Afterpay
	if c.AfterpayMerchantID != "" {
		if len(c.AfterpayMerchantID) > 4 {
			masked.AfterpayMerchantID = "****" + c.AfterpayMerchantID[len(c.AfterpayMerchantID)-4:]
		} else {
			masked.AfterpayMerchantID = "****"
		}
	}
	if c.AfterpaySecretKey != "" {
		masked.AfterpaySecretKey = "****"
	}

	// Zip
	if c.ZipMerchantID != "" {
		if len(c.ZipMerchantID) > 4 {
			masked.ZipMerchantID = "****" + c.ZipMerchantID[len(c.ZipMerchantID)-4:]
		} else {
			masked.ZipMerchantID = "****"
		}
	}
	if c.ZipAPIKey != "" {
		masked.ZipAPIKey = "****"
	}

	return masked
}

// ==================== DTOs ====================

// PaymentMethodResponse represents a payment method with tenant config status
type PaymentMethodResponse struct {
	PaymentMethod
	// Tenant-specific fields
	IsConfigured    bool                   `json:"isConfigured"`
	IsEnabled       bool                   `json:"isEnabled"`
	IsTestMode      bool                   `json:"isTestMode"`
	EnabledRegions  []string               `json:"enabledRegions,omitempty"` // Regions tenant has enabled for this method
	LastTestAt      *time.Time             `json:"lastTestAt,omitempty"`
	LastTestSuccess *bool                  `json:"lastTestSuccess,omitempty"`
	ConfigID        *uuid.UUID             `json:"configId,omitempty"`
}

// UpdatePaymentConfigRequest represents a request to update payment config
type UpdatePaymentConfigRequest struct {
	IsEnabled      *bool                  `json:"isEnabled,omitempty"`
	IsTestMode     *bool                  `json:"isTestMode,omitempty"`
	DisplayOrder   *int                   `json:"displayOrder,omitempty"`
	EnabledRegions []string               `json:"enabledRegions,omitempty"` // Regions to enable for this method
	Credentials    *PaymentCredentials    `json:"credentials,omitempty"`
	Settings       *PaymentConfigSettings `json:"settings,omitempty"`
}

// TestPaymentConnectionResponse represents the result of testing a payment connection
type TestPaymentConnectionResponse struct {
	Success    bool      `json:"success"`
	Message    string    `json:"message"`
	TestedAt   time.Time `json:"testedAt"`
	Provider   string    `json:"provider"`
	IsTestMode bool      `json:"isTestMode"`
}

// EnabledPaymentMethod represents a payment method available at checkout
type EnabledPaymentMethod struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	Provider     string `json:"provider"`
	Type         string `json:"type"`
	IconURL      string `json:"iconUrl,omitempty"`
	DisplayOrder int    `json:"displayOrder"`
	// For BNPL methods - show installment info
	InstallmentInfo string `json:"installmentInfo,omitempty"`
}
